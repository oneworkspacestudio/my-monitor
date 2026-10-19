package app

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"github.com/oneworkspacestudio/one-workspace/pkg/apphost"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Check struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	Kind        string  `json:"kind"`
	Target      string  `json:"target"`
	Enabled     bool    `json:"enabled"`
	Public      bool    `json:"public"`
	Timeout     int     `json:"timeout"`
	Interval    int     `json:"interval"`
	Threshold   int     `json:"threshold"`
	Failures    int     `json:"failures"`
	State       string  `json:"state"`
	Last        string  `json:"last"`
	Latency     int64   `json:"latency"`
	TLSExpiry   string  `json:"tlsExpiry"`
	Maintenance string  `json:"maintenance"`
	Quiet       int     `json:"quiet"`
	Managed     bool    `json:"managed"`
	Uptime      float64 `json:"uptime"`
}
type Registry struct {
	Connected string                                  `json:"connected"`
	Apps      []struct{ ID, Name, URL, State string } `json:"apps"`
	Update    string                                  `json:"update"`
	Backup    string                                  `json:"backup"`
}
type App struct {
	H           *apphost.Host
	mu          sync.RWMutex
	registry    Registry
	trusted     map[string]bool
	key         []byte
	samplesLock sync.Map
	resolver    func(context.Context, string) ([]net.IPAddr, error)
}

func New(h *apphost.Host) (*App, error) {
	if e := h.Migrate(1, `CREATE TABLE monitor_checks(id TEXT PRIMARY KEY,name TEXT NOT NULL,kind TEXT NOT NULL,target TEXT NOT NULL,enabled INTEGER NOT NULL DEFAULT 1,public INTEGER NOT NULL DEFAULT 0,interval INTEGER NOT NULL DEFAULT 60,threshold INTEGER NOT NULL DEFAULT 3,failures INTEGER NOT NULL DEFAULT 0,state TEXT NOT NULL DEFAULT 'pending',last TEXT NOT NULL DEFAULT '',latency INTEGER NOT NULL DEFAULT 0,tls_expiry TEXT NOT NULL DEFAULT '',maintenance TEXT NOT NULL DEFAULT '',quiet INTEGER NOT NULL DEFAULT 300,managed INTEGER NOT NULL DEFAULT 0,timeout INTEGER NOT NULL DEFAULT 8); CREATE TABLE monitor_samples(id INTEGER PRIMARY KEY,check_id TEXT NOT NULL,at TEXT NOT NULL,ok INTEGER NOT NULL,latency INTEGER NOT NULL,detail TEXT NOT NULL,maintenance INTEGER NOT NULL DEFAULT 0); CREATE INDEX sample_check ON monitor_samples(check_id,at); CREATE TABLE monitor_incidents(id TEXT PRIMARY KEY,check_id TEXT NOT NULL,opened TEXT NOT NULL,closed TEXT NOT NULL DEFAULT '',ack INTEGER NOT NULL DEFAULT 0,last_alert TEXT NOT NULL DEFAULT '',last_delivery TEXT NOT NULL DEFAULT ''); CREATE TABLE monitor_settings(key TEXT PRIMARY KEY,value TEXT NOT NULL);`); e != nil {
		return nil, e
	}
	keypath := filepath.Join(h.Config.Data, "secret.key")
	key, e := os.ReadFile(keypath)
	if os.IsNotExist(e) {
		key = make([]byte, 32)
		if _, e = rand.Read(key); e != nil {
			return nil, e
		}
		e = os.WriteFile(keypath, key, 0600)
	}
	if e != nil || len(key) != 32 {
		return nil, errors.New("monitor secret key unavailable")
	}
	a := &App{H: h, key: key, trusted: map[string]bool{}, resolver: net.DefaultResolver.LookupIPAddr}
	h.Mux.HandleFunc("GET /api/checks", h.Owner(a.checks))
	h.Mux.HandleFunc("POST /api/checks", h.Owner(a.saveCheck))
	h.Mux.HandleFunc("PUT /api/checks/{id}", h.Owner(a.saveCheck))
	h.Mux.HandleFunc("DELETE /api/checks/{id}", h.Owner(a.deleteCheck))
	h.Mux.HandleFunc("POST /api/checks/{id}/run", h.Owner(a.manual))
	h.Mux.HandleFunc("GET /api/samples/{id}", h.Owner(a.samples))
	h.Mux.HandleFunc("GET /api/incidents", h.Owner(a.incidents))
	h.Mux.HandleFunc("PUT /api/incidents/{id}/ack", h.Owner(a.ack))
	h.Mux.HandleFunc("GET /api/overview", h.Owner(a.overview))
	h.Mux.HandleFunc("PUT /api/settings", h.Owner(a.settings))
	h.Mux.HandleFunc("GET /status", a.publicStatus)
	return a, nil
}
func (a *App) Run(ctx context.Context) error {
	a.discover(ctx)
	tick := time.NewTicker(15 * time.Second)
	defer tick.Stop()
	cycles := 0
	for {
		a.H.Freeze.RLock()
		e := a.pass(ctx, time.Now())
		a.H.Freeze.RUnlock()
		if e != nil {
			a.H.Log.Warn("monitor sampling deferred")
		}
		select {
		case <-ctx.Done():
			return nil
		case <-tick.C:
		}
		cycles++
		a.mu.RLock()
		connected := len(a.trusted) > 0
		a.mu.RUnlock()
		if cycles%4 == 0 || !connected {
			a.discover(ctx)
		}
	}
}
func (a *App) discover(ctx context.Context) {
	if a.H.Config.RegistryURL == "" {
		return
	}
	u, e := url.Parse(a.H.Config.RegistryURL)
	if e != nil || u.Scheme != "http" || !(u.Hostname() == "127.0.0.1" || u.Hostname() == "localhost") || u.User != nil {
		return
	}
	client := &http.Client{Timeout: 5 * time.Second, Transport: &http.Transport{Proxy: nil}, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	r, _ := http.NewRequestWithContext(ctx, "GET", u.String(), nil)
	r.Header.Set("Authorization", "Bearer "+a.H.Config.RegistryToken)
	resp, e := client.Do(r)
	if e != nil {
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return
	}
	var reg Registry
	if json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&reg) != nil {
		return
	}
	reg.Connected = time.Now().UTC().Format(time.RFC3339)
	trusted := map[string]bool{}
	for _, v := range reg.Apps {
		u, e := url.Parse(v.URL)
		if e != nil || u.Scheme != "http" || u.User != nil || u.Hostname() != "127.0.0.1" || strings.ContainsAny(v.ID, "/\\\r\n") || len(v.ID) > 60 {
			continue
		}
		trusted[v.URL] = true
		_, e = a.H.DB.Exec("INSERT INTO monitor_checks(id,name,kind,target,managed)VALUES(?,?,'http',?,1) ON CONFLICT(id)DO UPDATE SET name=excluded.name,target=excluded.target", "app-"+v.ID, v.Name, v.URL)
		if e != nil {
			a.H.Log.Warn("registry check deferred")
		}
	}
	a.mu.Lock()
	a.registry = reg
	a.trusted = trusted
	a.mu.Unlock()
}

const checkColumns = "id,name,kind,target,enabled,public,interval,threshold,failures,state,last,latency,tls_expiry,maintenance,quiet,managed,timeout"

func scanCheck(row interface{ Scan(...any) error }) (c Check, e error) {
	e = row.Scan(&c.ID, &c.Name, &c.Kind, &c.Target, &c.Enabled, &c.Public, &c.Interval, &c.Threshold, &c.Failures, &c.State, &c.Last, &c.Latency, &c.TLSExpiry, &c.Maintenance, &c.Quiet, &c.Managed, &c.Timeout)
	return
}
func (a *App) all() ([]Check, error) {
	rows, e := a.H.DB.Query("SELECT " + checkColumns + " FROM monitor_checks ORDER BY name")
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []Check{}
	for rows.Next() {
		c, e := scanCheck(rows)
		if e != nil {
			return nil, e
		}
		out = append(out, c)
	}
	return out, rows.Err()
}
func (a *App) checks(w http.ResponseWriter, r *http.Request) {
	out, e := a.all()
	if e != nil {
		a.H.DBError(w, e)
		return
	}
	for i := range out {
		var n, ok int
		a.H.DB.QueryRow("SELECT COUNT(*),COALESCE(SUM(ok),0) FROM monitor_samples WHERE check_id=? AND maintenance=0 AND at>?", out[i].ID, time.Now().Add(-24*time.Hour).UTC().Format(time.RFC3339)).Scan(&n, &ok)
		if n > 0 {
			out[i].Uptime = float64(ok) * 100 / float64(n)
		}
	}
	apphost.JSON(w, 200, out)
}
func (a *App) saveCheck(w http.ResponseWriter, r *http.Request) {
	var c Check
	if !apphost.Decode(w, r, &c) {
		return
	}
	if c.Name == "" || len(c.Name) > 100 || len(c.Target) > 2000 || c.Timeout < 1 || c.Timeout > 60 || c.Interval < 15 || c.Interval > 86400 || c.Threshold < 1 || c.Threshold > 20 || c.Quiet < 60 || c.Quiet > 86400 {
		apphost.Problem(w, 400, "Check name, interval (15–86400s), threshold (1–20) and alert quiet period (60–86400s)")
		return
	}
	if c.Maintenance != "" {
		if _, e := time.Parse(time.RFC3339, c.Maintenance); e != nil {
			apphost.Problem(w, 400, "Maintenance must include a timezone offset")
			return
		}
	}
	managed := false
	if id := r.PathValue("id"); id != "" {
		var old Check
		var e error
		old, e = scanCheck(a.H.DB.QueryRow("SELECT "+checkColumns+" FROM monitor_checks WHERE id=?", id))
		if e != nil {
			apphost.Problem(w, 404, "Check not found")
			return
		}
		c.ID = id
		managed = old.Managed
		if managed && (old.Target != c.Target || old.Kind != c.Kind) {
			apphost.Problem(w, 400, "Discovered check targets are managed by Workspace")
			return
		}
	} else {
		c.ID = apphost.ID()
		var n int
		a.H.DB.QueryRow("SELECT COUNT(*) FROM monitor_checks").Scan(&n)
		if n >= 200 {
			apphost.Problem(w, 413, "Limit of 200 monitors reached")
			return
		}
	}
	if c.Kind != "http" && c.Kind != "tcp" && c.Kind != "process" && c.Kind != "service" {
		apphost.Problem(w, 400, "Choose HTTP, TCP, discovered process or Windows service")
		return
	}
	if c.Kind == "process" {
		a.mu.RLock()
		found := false
		for _, v := range a.registry.Apps {
			if v.ID == c.Target {
				found = true
			}
		}
		a.mu.RUnlock()
		if !found {
			apphost.Problem(w, 400, "Choose a discovered Workspace process ID")
			return
		}
	} else if c.Kind == "service" {
		if !validService(c.Target) {
			apphost.Problem(w, 400, "Enter a Windows service name")
			return
		}
	} else if _, _, e := a.endpoint(r.Context(), c.Kind, c.Target); e != nil {
		apphost.Problem(w, 400, "Target is not allowed: use a public HTTP/TCP address or a discovered Workspace check")
		return
	}
	_, e := a.H.DB.Exec("INSERT INTO monitor_checks(id,name,kind,target,enabled,public,interval,threshold,maintenance,quiet,managed,timeout)VALUES(?,?,?,?,?,?,?,?,?,?,?,?) ON CONFLICT(id)DO UPDATE SET name=excluded.name,enabled=excluded.enabled,public=excluded.public,interval=excluded.interval,threshold=excluded.threshold,maintenance=excluded.maintenance,quiet=excluded.quiet,timeout=excluded.timeout", c.ID, c.Name, c.Kind, c.Target, c.Enabled, c.Public, c.Interval, c.Threshold, c.Maintenance, c.Quiet, managed, c.Timeout)
	if e != nil {
		a.H.DBError(w, e)
		return
	}
	a.H.Audit(apphost.Current(r).User.ID, "monitor-config", c.ID)
	apphost.JSON(w, 200, c)
}
func (a *App) deleteCheck(w http.ResponseWriter, r *http.Request) {
	_, e := a.H.DB.Exec("DELETE FROM monitor_checks WHERE id=? AND managed=0", r.PathValue("id"))
	if e != nil {
		a.H.DBError(w, e)
		return
	}
	apphost.JSON(w, 200, map[string]bool{"ok": true})
}
func (a *App) manual(w http.ResponseWriter, r *http.Request) {
	c, e := scanCheck(a.H.DB.QueryRow("SELECT "+checkColumns+" FROM monitor_checks WHERE id=?", r.PathValue("id")))
	if e != nil {
		apphost.Problem(w, 404, "Check not found")
		return
	}
	if e = a.sample(r.Context(), c, time.Now()); e != nil {
		a.H.DBError(w, e)
		return
	}
	apphost.JSON(w, 200, map[string]bool{"ok": true})
}
func (a *App) samples(w http.ResponseWriter, r *http.Request) {
	rows, e := a.H.DB.Query("SELECT at,ok,latency,detail FROM monitor_samples WHERE check_id=? ORDER BY id DESC LIMIT 240", r.PathValue("id"))
	if e != nil {
		a.H.DBError(w, e)
		return
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var at, detail string
		var ok bool
		var latency int64
		rows.Scan(&at, &ok, &latency, &detail)
		out = append(out, map[string]any{"at": at, "ok": ok, "latency": latency, "detail": detail})
	}
	apphost.JSON(w, 200, out)
}
func (a *App) incidents(w http.ResponseWriter, r *http.Request) {
	rows, e := a.H.DB.Query("SELECT i.id,c.name,i.opened,i.closed,i.ack,i.last_delivery FROM monitor_incidents i JOIN monitor_checks c ON c.id=i.check_id ORDER BY opened DESC LIMIT 200")
	if e != nil {
		a.H.DBError(w, e)
		return
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var id, name, opened, closed string
		var ack bool
		var delivery string
		rows.Scan(&id, &name, &opened, &closed, &ack, &delivery)
		out = append(out, map[string]any{"id": id, "name": name, "opened": opened, "closed": closed, "ack": ack, "delivery": delivery})
	}
	apphost.JSON(w, 200, out)
}
func (a *App) ack(w http.ResponseWriter, r *http.Request) {
	a.H.DB.Exec("UPDATE monitor_incidents SET ack=1 WHERE id=?", r.PathValue("id"))
	apphost.JSON(w, 200, map[string]bool{"ok": true})
}
func (a *App) overview(w http.ResponseWriter, r *http.Request) {
	a.mu.RLock()
	reg := a.registry
	a.mu.RUnlock()
	var public string
	a.H.DB.QueryRow("SELECT value FROM monitor_settings WHERE key='public'").Scan(&public)
	apphost.JSON(w, 200, map[string]any{"metrics": metrics(a.H.Config.Data), "registry": reg, "public": public == "true", "retentionDays": 30, "webhookConfigured": a.setting("webhook") != ""})
}
func (a *App) setting(k string) string {
	var v string
	a.H.DB.QueryRow("SELECT value FROM monitor_settings WHERE key=?", k).Scan(&v)
	return v
}
func (a *App) settings(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Public  bool    `json:"public"`
		Webhook *string `json:"webhook"`
	}
	if !apphost.Decode(w, r, &in) {
		return
	}
	if in.Webhook != nil {
		value := *in.Webhook
		if value != "" {
			u, e := url.Parse(value)
			if e != nil || u.Scheme != "https" {
				apphost.Problem(w, 400, "Use a public HTTPS webhook URL")
				return
			}
			if _, _, e = a.endpoint(r.Context(), "http", value); e != nil {
				apphost.Problem(w, 400, "Webhook target is not allowed")
				return
			}
			value = a.encrypt(value)
		}
		a.H.DB.Exec("INSERT INTO monitor_settings VALUES('webhook',?) ON CONFLICT(key)DO UPDATE SET value=excluded.value", value)
	}
	_, e := a.H.DB.Exec("INSERT INTO monitor_settings VALUES('public',?) ON CONFLICT(key)DO UPDATE SET value=excluded.value", strconv.FormatBool(in.Public))
	if e != nil {
		a.H.DBError(w, e)
		return
	}
	apphost.JSON(w, 200, map[string]bool{"ok": true})
}
func (a *App) publicStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if a.setting("public") != "true" {
		apphost.Problem(w, 404, "Status page is not published")
		return
	}
	checks, e := a.all()
	if e != nil {
		apphost.Problem(w, 503, "Status temporarily unavailable")
		return
	}
	out := []map[string]string{}
	for _, c := range checks {
		if c.Public {
			state := c.State
			if until, e := time.Parse(time.RFC3339, c.Maintenance); e == nil && until.After(time.Now()) {
				state = "maintenance"
			}
			out = append(out, map[string]string{"name": c.Name, "state": state})
		}
	}
	apphost.JSON(w, 200, map[string]any{"services": out, "updated": time.Now().UTC().Format(time.RFC3339)})
}
func (a *App) encrypt(value string) string {
	block, _ := aes.NewCipher(a.key)
	gcm, _ := cipher.NewGCM(block)
	n := make([]byte, gcm.NonceSize())
	if _, e := rand.Read(n); e != nil {
		return ""
	}
	return base64.RawStdEncoding.EncodeToString(gcm.Seal(n, n, []byte(value), nil))
}
func (a *App) decrypt(value string) string {
	b, e := base64.RawStdEncoding.DecodeString(value)
	if e != nil {
		return ""
	}
	block, _ := aes.NewCipher(a.key)
	gcm, _ := cipher.NewGCM(block)
	if len(b) < gcm.NonceSize() {
		return ""
	}
	plain, e := gcm.Open(nil, b[:gcm.NonceSize()], b[gcm.NonceSize():], nil)
	if e != nil {
		return ""
	}
	return string(plain)
}
