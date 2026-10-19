package app

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestMaintenanceQuietAcknowledgementAndSafeStatus(t *testing.T) {
	a := monitorFixture(t)
	var deliveries atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" {
			deliveries.Add(1)
		}
		w.WriteHeader(503)
	}))
	defer server.Close()
	a.trusted[server.URL] = true
	now := time.Now().UTC()
	_, err := a.H.DB.Exec("INSERT INTO monitor_checks(id,name,kind,target,threshold,quiet,public,maintenance) VALUES('test','Safe service','http',?,1,300,1,?)", server.URL, now.Add(time.Hour).Format(time.RFC3339))
	if err != nil {
		t.Fatal(err)
	}
	a.H.DB.Exec("INSERT INTO monitor_settings VALUES('webhook',?)", a.encrypt(server.URL))
	c := Check{ID: "test"}
	if err = a.sample(context.Background(), c, now); err != nil {
		t.Fatal(err)
	}
	var count int
	a.H.DB.QueryRow("SELECT COUNT(*) FROM monitor_incidents").Scan(&count)
	if count != 0 {
		t.Fatal("maintenance created outage")
	}
	var planned bool
	a.H.DB.QueryRow("SELECT maintenance FROM monitor_samples LIMIT 1").Scan(&planned)
	if !planned {
		t.Fatal("maintenance sample counted as ordinary uptime")
	}
	a.H.DB.Exec("UPDATE monitor_checks SET maintenance='' WHERE id='test'")
	for _, seconds := range []int{1, 10, 60} {
		if err = a.sample(context.Background(), c, now.Add(time.Duration(seconds)*time.Second)); err != nil {
			t.Fatal(err)
		}
	}
	if deliveries.Load() != 1 {
		t.Fatal("failed webhook ignored quiet period", deliveries.Load())
	}
	a.H.DB.Exec("UPDATE monitor_incidents SET ack=1")
	a.sample(context.Background(), c, now.Add(10*time.Minute))
	if deliveries.Load() != 1 {
		t.Fatal("acknowledged incident alerted")
	}
	a.H.DB.Exec("INSERT INTO monitor_settings VALUES('public','true')")
	r := httptest.NewRequest("GET", a.H.Config.Origin+"/status", nil)
	w := httptest.NewRecorder()
	a.H.Handler().ServeHTTP(w, r)
	if w.Code != 200 || !strings.Contains(w.Body.String(), "Safe service") || strings.Contains(w.Body.String(), server.URL) || strings.Contains(w.Body.String(), "webhook") {
		t.Fatal("unsafe public status", w.Code, w.Body.String())
	}
	r = httptest.NewRequest("GET", a.H.Config.Origin+"/api/checks", nil)
	w = httptest.NewRecorder()
	a.H.Handler().ServeHTTP(w, r)
	if w.Code != 401 {
		t.Fatal("anonymous infrastructure", w.Code)
	}
}
