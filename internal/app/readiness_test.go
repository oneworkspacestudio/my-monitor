package app

import "testing"

func TestManagedSetupReadiness(t *testing.T) {
	for _, tc := range []struct {
		body       string
		incomplete bool
	}{
		{`{"status":"ok","configured":false}`, true},
		{`{"status":"ok","configured":true}`, false},
		{`{"status":"ok"}`, false}, {`OK`, false}, {`{"configured":null}`, false},
	} {
		if got := managedSetupIncomplete([]byte(tc.body)); got != tc.incomplete {
			t.Errorf("%s: %v", tc.body, got)
		}
	}
}
