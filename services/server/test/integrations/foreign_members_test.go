// SPDX-License-Identifier: Apache-2.0

package integrations_test

import (
	"encoding/json"
	"fmt"
	"math/rand/v2"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
)

// foreignValue returns a random JSON value of any type for a member the contract does not name.
func foreignValue(r *rand.Rand) any {
	switch r.IntN(6) {
	case 0:
		return "text"
	case 1:
		return r.IntN(100)
	case 2:
		return map[string]any{"team": "ops"}
	case 3:
		return []any{"a", 1}
	case 4:
		return nil
	default:
		return true
	}
}

// foreignName returns a random member name no format defines.
func foreignName(r *rand.Rand) string {
	return []string{"routeLabels", "notification_reason", "futureMember", "x-extra", "a/b~c"}[r.IntN(5)] + fmt.Sprint(r.IntN(3))
}

// TestAlertmanager_ForeignMembersAreIgnored (11 8.2): the payloads Alertmanager v0.34.1 really
// sends (routeLabels, notification_reason) give 201 and their golden message; so does every
// fixture with random members the contract does not name added at the top and in each alert
// (seeded), because Alertmanager drops a notification answered with 400. A native body with such
// a member still gives 400 unknown_member at that member.
func TestAlertmanager_ForeignMembersAreIgnored(t *testing.T) {
	t.Parallel()
	h := newHW(t, options{})
	k := h.create(h.tracker, h.eskalation, "")
	url := h.s.InternalURL + "/api/v1/hooks/" + k.id + "/" + k.token
	post := func(name string, raw []byte, want string) {
		t.Helper()
		r := h.send(url, raw, nil)
		if r.status != http.StatusCreated {
			t.Fatalf("%s: %d %s", name, r.status, r.body)
		}
		msg, _ := r.result(t)
		if m := h.message(msg); m.content != want || m.origin != "app" {
			t.Fatalf("%s: stored %q (origin %s), want %q", name, m.content, m.origin, want)
		}
	}
	for _, name := range []string{"recorded_v0_34_1_firing", "recorded_v0_34_1_resolved"} {
		raw, want := fixture(t, name)
		post(name, raw, want)
	}
	files, err := filepath.Glob(filepath.Join(fixtureDir, "*.json"))
	if err != nil || len(files) == 0 {
		t.Fatalf("no fixtures: %v", err)
	}
	r := rand.New(rand.NewPCG(8, 2)) //nolint:gosec // G404: reproducible test data from a fixed seed, not security
	for _, f := range files {
		name := strings.TrimSuffix(filepath.Base(f), ".json")
		raw, want := fixture(t, name)
		var body map[string]any
		if err := json.Unmarshal(raw, &body); err != nil {
			t.Fatal(err)
		}
		for range 1 + r.IntN(3) {
			body[foreignName(r)] = foreignValue(r)
		}
		alerts, _ := body["alerts"].([]any)
		for _, a := range alerts {
			if m, ok := a.(map[string]any); ok && r.IntN(2) == 0 {
				m[foreignName(r)] = foreignValue(r)
			}
		}
		post(name+" with foreign members", marshal(t, body), want)
	}
	for range 4 {
		member := foreignName(r)
		res := h.send(url, marshal(t, map[string]any{"content_md": "x", member: foreignValue(r)}), nil)
		pointer := "/" + strings.ReplaceAll(strings.ReplaceAll(member, "~", "~0"), "/", "~1")
		if _, fc, ptr, _, _ := res.problem(); res.status != http.StatusBadRequest || fc != "unknown_member" || ptr != pointer {
			t.Fatalf("native with %q: %d %s", member, res.status, res.body)
		}
	}
}
