package app

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

var publicIPv6 = netip.MustParsePrefix("2000::/3")

var nonPublicPrefixes = []netip.Prefix{
	netip.MustParsePrefix("0.0.0.0/8"), netip.MustParsePrefix("100.64.0.0/10"),
	netip.MustParsePrefix("192.0.0.0/24"), netip.MustParsePrefix("192.0.2.0/24"),
	netip.MustParsePrefix("198.18.0.0/15"), netip.MustParsePrefix("198.51.100.0/24"),
	netip.MustParsePrefix("203.0.113.0/24"), netip.MustParsePrefix("240.0.0.0/4"),
	netip.MustParsePrefix("64:ff9b::/96"), netip.MustParsePrefix("64:ff9b:1::/48"),
	netip.MustParsePrefix("2002::/16"), netip.MustParsePrefix("2001::/32"),
	netip.MustParsePrefix("2001:db8::/32"),
}

func forbidden(ip net.IP) bool {
	if ip == nil || ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsUnspecified() || ip.IsMulticast() || !ip.IsGlobalUnicast() {
		return true
	}
	addr, ok := netip.AddrFromSlice(ip)
	if !ok {
		return true
	}
	addr = addr.Unmap()
	if addr.Is6() && !publicIPv6.Contains(addr) {
		return true
	}
	for _, prefix := range nonPublicPrefixes {
		if prefix.Contains(addr) {
			return true
		}
	}
	return false
}
func (a *App) endpoint(ctx context.Context, kind, target string) (string, []net.IPAddr, error) {
	host, port := "", ""
	if kind == "http" {
		u, e := url.Parse(target)
		if e != nil || u.User != nil || u.Fragment != "" || u.Host == "" || u.Scheme != "http" && u.Scheme != "https" || strings.ContainsAny(target, "\r\n\\") {
			return "", nil, errors.New("invalid URL")
		}
		host = u.Hostname()
		port = u.Port()
		if port == "" {
			port = "80"
			if u.Scheme == "https" {
				port = "443"
			}
		}
	} else {
		var e error
		host, port, e = net.SplitHostPort(target)
		if e != nil {
			return "", nil, e
		}
	}
	p, e := strconv.Atoi(port)
	if e != nil || p < 1 || p > 65535 {
		return "", nil, errors.New("invalid port")
	}
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	ips, e := a.resolver(ctx, host)
	if e != nil || len(ips) == 0 {
		return "", nil, errors.New("DNS unavailable")
	}
	a.mu.RLock()
	trusted := a.trusted[target]
	a.mu.RUnlock()
	for _, ip := range ips {
		if forbidden(ip.IP) && !(trusted && ip.IP.Equal(net.ParseIP("127.0.0.1"))) {
			return "", nil, errors.New("private, local or metadata target")
		}
	}
	return port, ips, nil
}
func (a *App) client(ctx context.Context, target string) (*http.Client, error) {
	port, ips, e := a.endpoint(ctx, "http", target)
	if e != nil {
		return nil, e
	}
	dial := func(ctx context.Context, network, ignored string) (net.Conn, error) {
		var last error
		for _, ip := range ips {
			conn, e := (&net.Dialer{Timeout: 4 * time.Second}).DialContext(ctx, "tcp", net.JoinHostPort(ip.IP.String(), port))
			if e == nil {
				return conn, nil
			}
			last = e
		}
		return nil, last
	}
	return &http.Client{Timeout: 60 * time.Second, Transport: &http.Transport{Proxy: nil, DialContext: dial, TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12}, DisableKeepAlives: true}, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}, nil
}
func (a *App) pass(ctx context.Context, at time.Time) error {
	checks, e := a.all()
	if e != nil {
		return e
	}
	for _, c := range checks {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		last, _ := time.Parse(time.RFC3339, c.Last)
		a.mu.RLock()
		registered := a.trusted[c.Target]
		update := a.registry.Update
		a.mu.RUnlock()
		if c.Managed && !registered {
			a.H.DB.Exec("UPDATE monitor_checks SET state='discovering',failures=0 WHERE id=?", c.ID)
			continue
		}
		if c.Managed && update != "" && update != "Idle" && update != "complete" && update != "rolled-back" {
			a.H.DB.Exec("UPDATE monitor_checks SET state='maintenance',failures=0 WHERE id=?", c.ID)
			continue
		}
		if c.Enabled && at.Sub(last) >= time.Duration(c.Interval)*time.Second {
			if e = a.sample(ctx, c, at); e != nil {
				return e
			}
		}
	}
	cut := at.Add(-30 * 24 * time.Hour).UTC().Format(time.RFC3339)
	_, e = a.H.DB.Exec("DELETE FROM monitor_samples WHERE at<?", cut)
	return e
}
func (a *App) sample(ctx context.Context, c Check, at time.Time) error {
	value, _ := a.samplesLock.LoadOrStore(c.ID, &sync.Mutex{})
	lock := value.(*sync.Mutex)
	lock.Lock()
	defer lock.Unlock()
	if fresh, e := scanCheck(a.H.DB.QueryRow("SELECT "+checkColumns+" FROM monitor_checks WHERE id=?", c.ID)); e == nil {
		c = fresh
	} else {
		return nil
	}
	timeout := c.Timeout
	if timeout == 0 {
		timeout = 8
	}
	ctx, cancel := context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
	defer cancel()
	start := time.Now()
	ok := false
	detail := "Connection failed"
	expiry := ""
	switch c.Kind {
	case "http":
		client, e := a.client(ctx, c.Target)
		if e == nil {
			req, _ := http.NewRequestWithContext(ctx, "GET", c.Target, nil)
			res, e := client.Do(req)
			if e == nil {
				io.Copy(io.Discard, io.LimitReader(res.Body, 4096))
				res.Body.Close()
				ok = res.StatusCode >= 200 && res.StatusCode < 400
				detail = "HTTP " + strconv.Itoa(res.StatusCode)
				if res.TLS != nil && len(res.TLS.PeerCertificates) > 0 {
					expiry = res.TLS.PeerCertificates[0].NotAfter.UTC().Format(time.RFC3339)
					if res.TLS.PeerCertificates[0].NotAfter.Before(at.Add(14 * 24 * time.Hour)) {
						ok = false
						detail = "TLS certificate expires within 14 days"
					}
				}
			}
		} else {
			detail = "Target not allowed or DNS unavailable"
		}
	case "tcp":
		port, ips, e := a.endpoint(ctx, "tcp", c.Target)
		if e == nil {
			for _, ip := range ips {
				conn, e := (&net.Dialer{Timeout: 5 * time.Second}).DialContext(ctx, "tcp", net.JoinHostPort(ip.IP.String(), port))
				if e == nil {
					conn.Close()
					ok = true
					detail = "TCP connected"
					break
				}
			}
		}
	case "process":
		a.mu.RLock()
		for _, v := range a.registry.Apps {
			if v.ID == c.Target {
				ok = v.State == "running"
				detail = "Supervisor state: " + v.State
			}
		}
		a.mu.RUnlock()
	case "service":
		ok = serviceRunning(c.Target)
		detail = "Windows service state checked"
	}
	if parentErr := ctx.Err(); parentErr != nil && errors.Is(parentErr, context.Canceled) {
		return nil
	}
	latency := time.Since(start).Milliseconds()
	failures := 0
	state := "up"
	if !ok {
		failures = c.Failures + 1
		state = "degraded"
		if failures >= c.Threshold {
			state = "down"
		}
	}
	maintenance, _ := time.Parse(time.RFC3339, c.Maintenance)
	planned := maintenance.After(at)
	if planned {
		state = "maintenance"
		failures = 0
		detail = "Planned maintenance: " + detail
	}
	stamp := at.UTC().Format(time.RFC3339)
	tx, e := a.H.DB.Begin()
	if e != nil {
		return e
	}
	defer tx.Rollback()
	if _, e = tx.Exec("INSERT INTO monitor_samples(check_id,at,ok,latency,detail,maintenance)VALUES(?,?,?,?,?,?)", c.ID, stamp, ok, latency, detail, planned); e != nil {
		return e
	}
	if _, e = tx.Exec("UPDATE monitor_checks SET failures=?,state=?,last=?,latency=?,tls_expiry=? WHERE id=?", failures, state, stamp, latency, expiry, c.ID); e != nil {
		return e
	}
	var incident, last string
	var ack bool
	tx.QueryRow("SELECT id,last_alert,ack FROM monitor_incidents WHERE check_id=? AND closed='' LIMIT 1", c.ID).Scan(&incident, &last, &ack)
	if state == "down" && incident == "" {
		incident = appID()
		_, e = tx.Exec("INSERT INTO monitor_incidents(id,check_id,opened)VALUES(?,?,?)", incident, c.ID, stamp)
	}
	if ok && !planned && incident != "" {
		_, e = tx.Exec("UPDATE monitor_incidents SET closed=? WHERE id=?", stamp, incident)
	}
	if e != nil {
		return e
	}
	if e = tx.Commit(); e != nil {
		return e
	}
	lastAlert, _ := time.Parse(time.RFC3339, last)
	if incident != "" && !ok && state == "down" && !ack && !maintenance.After(at) && at.Sub(lastAlert) >= time.Duration(c.Quiet)*time.Second {
		a.H.DB.Exec("UPDATE monitor_incidents SET last_alert=? WHERE id=?", stamp, incident)
		delivery := "In-app incident only"
		if a.setting("webhook") != "" {
			delivery = "Webhook failed; retry after quiet period"
			if a.alert(ctx, c.Name, state) {
				delivery = "Webhook delivered"
			}
		}
		a.H.DB.Exec("UPDATE monitor_incidents SET last_delivery=? WHERE id=?", delivery, incident)
	}
	return nil
}
func (a *App) alert(ctx context.Context, name, state string) bool {
	target := a.decrypt(a.setting("webhook"))
	if target == "" {
		return false
	}
	client, e := a.client(ctx, target)
	if e != nil {
		return false
	}
	payload, _ := json.Marshal(map[string]string{"app": "My Monitor", "name": name, "state": state})
	req, _ := http.NewRequestWithContext(ctx, "POST", target, bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	res, e := client.Do(req)
	if e != nil {
		return false
	}
	defer res.Body.Close()
	return res.StatusCode >= 200 && res.StatusCode < 300
}
