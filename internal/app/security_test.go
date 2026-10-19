package app

import (
	"context"
	"github.com/oneworkspacestudio/one-workspace/pkg/apphost"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"
)

func monitorFixture(t *testing.T) *App {
	h, e := apphost.New(apphost.Config{ID: "monitor", Name: "My Monitor", Data: filepath.Join(t.TempDir(), "data"), Origin: "http://localhost:29010", Listen: "127.0.0.1:29010"}, nil)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { h.Close() })
	a, e := New(h)
	if e != nil {
		t.Fatal(e)
	}
	return a
}
func TestSSRFAllResolvedAddressesAndRebinding(t *testing.T) {
	a := monitorFixture(t)
	cases := []string{"127.0.0.1", "10.0.0.1", "169.254.169.254", "::1", "fc00::1", "100.64.0.1", "0.0.0.0", "224.0.0.1", "0.1.2.3", "240.0.0.1", "198.18.0.1", "64:ff9b::7f00:1", "2002:7f00:1::", "2001::1", "2001:db8::1", "fec0::1"}
	for _, ip := range cases {
		a.resolver = func(context.Context, string) ([]net.IPAddr, error) { return []net.IPAddr{{IP: net.ParseIP(ip)}}, nil }
		if _, _, e := a.endpoint(context.Background(), "http", "https://fixture.test"); e == nil {
			t.Fatal("SSRF allowed", ip)
		}
	}
	a.resolver = func(context.Context, string) ([]net.IPAddr, error) {
		return []net.IPAddr{{IP: net.ParseIP("8.8.8.8")}, {IP: net.ParseIP("127.0.0.1")}}, nil
	}
	if _, _, e := a.endpoint(context.Background(), "http", "https://fixture.test"); e == nil {
		t.Fatal("mixed DNS allowed")
	}
	a.resolver = net.DefaultResolver.LookupIPAddr
	target := "http://127.0.0.1:29010/healthz"
	a.trusted[target] = true
	if _, _, e := a.endpoint(context.Background(), "http", target); e != nil {
		t.Fatal(e)
	}
	if _, _, e := a.endpoint(context.Background(), "http", target+"/admin"); e == nil {
		t.Fatal("registry trust widened")
	}
	for _, s := range []string{"http://u:p@example.com", "file:///C:/Windows", "https://example.com/#x"} {
		if _, _, e := a.endpoint(context.Background(), "http", s); e == nil {
			t.Fatal("invalid url", s)
		}
	}
}
func TestPublicStatusOptInSecretProtectionAndIncidentLifecycle(t *testing.T) {
	a := monitorFixture(t)
	r := httptest.NewRequest("GET", a.H.Config.Origin+"/status", nil)
	w := httptest.NewRecorder()
	a.H.Handler().ServeHTTP(w, r)
	if w.Code != 404 {
		t.Fatal("status public by default")
	}
	value := "https://example.test/webhook?secret=fixture"
	encrypted := a.encrypt(value)
	if encrypted == value || a.decrypt(encrypted) != value || a.decrypt(encrypted+"x") != "" {
		t.Fatal("secret encryption")
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(503) }))
	defer server.Close()
	a.trusted[server.URL] = true
	c := Check{ID: "fixture", Name: "Fixture", Kind: "http", Target: server.URL, Threshold: 2, Quiet: 60}
	a.H.DB.Exec("INSERT INTO monitor_checks(id,name,kind,target,threshold)VALUES(?,?,?, ?,2)", c.ID, c.Name, c.Kind, c.Target)
	for i := 0; i < 3; i++ {
		if e := a.sample(context.Background(), c, time.Now().Add(time.Duration(i)*time.Minute)); e != nil {
			t.Fatal(e)
		}
		c.Failures++
	}
	var n int
	a.H.DB.QueryRow("SELECT COUNT(*) FROM monitor_incidents WHERE closed=''").Scan(&n)
	if n != 1 {
		t.Fatal("incident duplicate", n)
	}
	server.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) })
	if e := a.sample(context.Background(), c, time.Now().Add(4*time.Minute)); e != nil {
		t.Fatal(e)
	}
	a.H.DB.QueryRow("SELECT COUNT(*) FROM monitor_incidents WHERE closed=''").Scan(&n)
	if n != 0 {
		t.Fatal("incident did not recover")
	}
}
