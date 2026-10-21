package app

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestManagedProbeUsesSupervisorAuthorityWithoutTrustingUserTargets(t *testing.T) {
	for _, tc := range []struct {
		name                     string
		managed, trusted, wantUp bool
	}{
		{"registered app", true, true, true},
		{"ordinary user check", false, true, false},
		{"unregistered local target", true, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := monitorFixture(t)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if !strings.HasPrefix(r.Host, "localhost:") {
					http.Error(w, "invalid host", 403)
					return
				}
				fmt.Fprint(w, `{"status":"ok"}`)
			}))
			defer server.Close()
			target := server.URL + "/healthz"
			a.trusted[target] = tc.trusted
			c := Check{ID: "app-accounts", Name: "MyAccounts", Kind: "http", Target: target, Managed: tc.managed, Threshold: 1, Timeout: 2}
			if _, err := a.H.DB.Exec("INSERT INTO monitor_checks(id,name,kind,target,managed,threshold,timeout)VALUES(?,?,?, ?,?,1,2)", c.ID, c.Name, c.Kind, c.Target, c.Managed); err != nil {
				t.Fatal(err)
			}
			if err := a.sample(context.Background(), c, time.Now()); err != nil {
				t.Fatal(err)
			}
			var state, detail string
			a.H.DB.QueryRow("SELECT state FROM monitor_checks WHERE id=?", c.ID).Scan(&state)
			a.H.DB.QueryRow("SELECT detail FROM monitor_samples WHERE check_id=? ORDER BY id DESC LIMIT 1", c.ID).Scan(&detail)
			if (state == "up") != tc.wantUp {
				t.Fatalf("state=%s detail=%s", state, detail)
			}
			if tc.wantUp && detail != "HTTP 200" {
				t.Fatal("healthy result was not backed by HTTP", detail)
			}
		})
	}
}
