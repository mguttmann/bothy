// SPDX-License-Identifier: Apache-2.0

package postgres_test

import (
	"net/http"
	"strings"
	"sync"
	"testing"
)

// TestInstallApp_TextsTheDatabaseRefuses (red team M2-24): a manifest whose JSON holds the escape
// of NUL or of an unpaired surrogate is refused with 400 validation_failed, and validateManifest
// reports it, instead of an install that fails in the database with 500.
func TestInstallApp_TextsTheDatabaseRefuses(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	o := w.RobinPhone.StepUp()
	base := `{"schema_version":1,"slug":"tracker","name":"Tracker","description":"DESC","publisher":"Example Org",` +
		`"version":"1.0.0","min_api_version":"0.1","scopes":{"required":["messages:write","commands:write"],"optional":[]},` +
		`"delivery":{"mode":"webhook","events_url":"https://` + allowedHost + `/e","interactions_url":"https://` + allowedHost + `/i"},` +
		`"data_egress":"Command options go to the help desk.",` +
		`"commands":[{"name":"issue","description":"Create an issue","options":[{"name":"prio","description":"Priority","type":"choice","required":false,` +
		`"choices":[{"name":"High","value":"VALUE"}]}]}]}`
	for _, c := range []struct{ name, desc, value string }{
		{"lone high surrogate", `Mirrors \ud800 tickets.`, "high"},
		{"lone low surrogate", `Mirrors \udfff tickets.`, "high"},
		{"nul in a choice", "Mirrors tickets.", `hi\u0000gh`},
	} {
		raw := strings.Replace(strings.Replace(base, "DESC", c.desc, 1), "VALUE", c.value, 1)
		body := `{"manifest":` + raw + `,"granted_scopes":["messages:write","commands:write"],"reason":"Sync tickets"}`
		r := o.Call(http.MethodPost, "/api/v1/apps", body, idem()...)
		if r.StatusCode != http.StatusBadRequest {
			t.Errorf("%s: install status %d, want 400: %s", c.name, r.StatusCode, r.Body)
		}
		v := w.Want(o.Call(http.MethodPost, "/api/v1/app-manifests/validate", raw), http.StatusOK, "")
		if v["valid"] != false {
			t.Errorf("%s: validateManifest %v", c.name, v)
		}
	}
	// A backslash before the u is text, and a pair of surrogates is one character.
	raw := strings.Replace(strings.Replace(base, "DESC", `Mirrors \\u0000 and 😀 tickets.`, 1), "VALUE", "high", 1)
	w.Want(o.Call(http.MethodPost, "/api/v1/apps", `{"manifest":`+raw+`,"granted_scopes":["messages:write","commands:write"],"reason":"Sync tickets"}`, idem()...),
		http.StatusCreated, "")
}

// TestUpgrade_LongCommandName (red team M2-24): an upgrade that adds and then removes a command
// whose name is longer than an audit code answers 200, not 500.
func TestUpgrade_LongCommandName(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	o := w.RobinPhone.StepUp()
	long := strings.Repeat("a", 70)
	withCommand := func(version, name string) map[string]any {
		return manifest("tracker", version, []string{"messages:write", "commands:write"}, nil, func(m map[string]any) {
			if name != "" {
				m["commands"] = []any{map[string]any{"name": name, "description": "Create an issue"}}
			}
			m["delivery"] = map[string]any{"mode": "webhook", "events_url": "https://" + allowedHost + "/events", "interactions_url": "https://" + allowedHost + "/interactions"}
			m["data_egress"] = "Command options go to the help desk."
		})
	}
	app := w.Install(o, withCommand("1.0.0", ""), []string{"messages:write", "commands:write"})
	path := "/api/v1/apps/" + app["id"].(string) + "/manifest"
	granted := []string{"messages:write", "commands:write"}
	w.Want(o.Call(http.MethodPut, path, installBody(withCommand("1.1.0", long), granted, nil)), http.StatusOK, "")
	w.Want(o.Call(http.MethodPut, path, installBody(withCommand("1.2.0", ""), granted, nil)), http.StatusOK, "")
	for _, r := range w.Allows("app.install") {
		if strings.Contains(r.Raw, long) {
			t.Errorf("audit holds the long name: %s", r.Raw)
		}
	}
}

// TestSuspend_ConcurrentKeyed (red team M2-24): parallel suspends with their own Idempotency-Key
// each answer 204; the ones that lose the race finish without effect instead of 500.
func TestSuspend_ConcurrentKeyed(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	o := w.RobinPhone.StepUp()
	app := w.Install(o, trackerManifest("1.0.0"), []string{"messages:write"})
	path := "/api/v1/apps/" + app["id"].(string) + "/suspend"
	const n = 4
	statuses := make([]int, n)
	var wg sync.WaitGroup
	for i := range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			statuses[i] = o.Call(http.MethodPost, path, "", idem()...).StatusCode
		}()
	}
	wg.Wait()
	for i, s := range statuses {
		if s != http.StatusNoContent {
			t.Errorf("call %d: status %d, want 204 (%v)", i, s, statuses)
		}
	}
	if n := w.Count(`SELECT count(*) FROM app_installations WHERE app_id = $1 AND state = 'suspended'`, app["id"]); n != 1 {
		t.Errorf("%d suspended installations", n)
	}
}
