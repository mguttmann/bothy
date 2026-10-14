// SPDX-License-Identifier: Apache-2.0

package httpapi_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/httpserver/httpservertest"
	"github.com/OWNER/bothy/services/server/test/terminalworld"
)

// TestHostCanary_SECT10 (SEC-T10, Design note 13 of M2-39; services/server/test/canary of M1-69
// is not on the base branch): a run-unique host name goes through create, rename, get, list, a
// state change and a duplicate create (409) over the real HTTP chain. The name never appears in
// the captured logs, the recorded spans, the audit details, a problem body or an event row other
// than the host.updated payloads of that host; the pairing code of the run is a second canary,
// found nowhere, also not in host.updated and idempotency_keys.
func TestHostCanary_SECT10(t *testing.T) {
	t.Parallel()
	w := terminalworld.New(t)
	canary := "Canary " + strings.ReplaceAll(ids.New().String()[24:], "-", "")
	m := w.Want(w.Do("robin", http.MethodPost, terminalworld.HostsPath, `{"name":"`+canary+`","reason":"Canary run"}`, terminalworld.Key()...),
		http.StatusCreated, "")
	host := ids.MustParse(m["host_id"].(string))
	code := m["pairing_code"].(string)
	renamed := canary + " two"
	w.Want(w.Do("robin", http.MethodPatch, terminalworld.HostPath(host), `{"name":"`+renamed+`","reason":"Rename"}`, w.Keyed(host)...), http.StatusOK, "")
	if got := w.Want(w.Do("robin", http.MethodGet, terminalworld.HostPath(host), ""), http.StatusOK, ""); got["name"] != renamed {
		t.Fatalf("get %v", got)
	}
	found := false
	for _, id := range w.ListIDs("robin", 2) {
		found = found || id == host
	}
	if !found {
		t.Fatal("the canary host is not listed")
	}
	w.Want(w.Do("robin", http.MethodPost, terminalworld.HostPath(host)+"/state", `{"state":"retired","reason":"Gone"}`, w.Keyed(host)...),
		http.StatusConflict, "invalid_state_transition")
	dup := w.Do("robin", http.MethodPost, terminalworld.HostsPath, `{"name":"`+renamed+`","reason":"Twin"}`, terminalworld.Key()...)
	w.Want(dup, http.StatusConflict, "conflict")
	for _, s := range []string{canary, code} {
		if strings.Contains(string(dup.Body), s) {
			t.Errorf("a problem body carries a canary: %s", dup.Body)
		}
	}
	for _, s := range []string{canary, code} {
		if n := w.Count(`SELECT count(*) FROM event_log WHERE payload::text LIKE '%' || $1 || '%'
			AND NOT (type = 'host.updated' AND resource_id = $2)`, s, host.String()); n != 0 {
			t.Errorf("a canary in %d event rows outside the host.updated of its host", n)
		}
		if n := w.Count(`SELECT count(*) FROM audit_events WHERE details::text LIKE '%' || $1 || '%'`, s); n != 0 {
			t.Errorf("a canary in %d audit rows", n)
		}
		for _, sp := range httpservertest.Spans().Ended() {
			if strings.Contains(sp.Name(), s) {
				t.Errorf("a canary in the span name %q", sp.Name())
			}
			for _, a := range sp.Attributes() {
				if strings.Contains(a.Value.String(), s) {
					t.Errorf("a canary in the span attribute %s", a.Key)
				}
			}
		}
	}
	w.AssertNowhere(code, "pairing code")
	if n := w.Count(`SELECT count(*) FROM event_log WHERE type = 'host.updated' AND payload::text LIKE '%' || $1 || '%'`, code); n != 0 {
		t.Errorf("the pairing code is in %d host.updated rows", n)
	}
	for _, r := range append(w.H.Logs.Records(), w.Logs.Records()...) {
		for _, v := range r.Attrs {
			if strings.Contains(v, canary) {
				t.Errorf("the host name is in the log record %q", r.Message)
			}
		}
		if strings.Contains(r.Message, canary) {
			t.Errorf("the host name is in the log message %q", r.Message)
		}
	}
}
