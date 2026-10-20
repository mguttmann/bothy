// SPDX-License-Identifier: Apache-2.0

package integrations_test

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/wire"
	"github.com/OWNER/bothy/services/server/test/harness"
)

// fixtureDir holds the Alertmanager payloads and their expected messages (golden files of
// internal/apps/domain/incoming).
var fixtureDir = filepath.Join("..", "..", "internal", "apps", "domain", "incoming", "testdata", "alertmanager")

// fixture returns the payload and the expected message of case name; the golden file ends with
// the one line break the end-of-file rule of the repository adds, which the message does not have.
func fixture(t *testing.T, name string) (payload []byte, want string) {
	t.Helper()
	payload, err := os.ReadFile(filepath.Join(fixtureDir, name+".json")) //nolint:gosec // G304: a fixture of the repository, not input.
	if err != nil {
		t.Fatal(err)
	}
	md, err := os.ReadFile(filepath.Join(fixtureDir, name+".md")) //nolint:gosec // G304: a fixture of the repository, not input.
	if err != nil {
		t.Fatal(err)
	}
	return payload, strings.TrimSuffix(string(md), "\n")
}

// TestAlertmanager_ExamplePayloadGivesExpectedMessage (11 10.3, plan row): the firing fixture
// posted to the path URL gives 201 and a stored content_md byte-identical to its golden file; the
// resolved fixture gives a second message; both origin app; the same on the internal listener;
// with a fixed thread both land there.
func TestAlertmanager_ExamplePayloadGivesExpectedMessage(t *testing.T) {
	t.Parallel()
	h := newHW(t, options{})
	k := h.create(h.tracker, h.eskalation, "")
	firing, wantFiring := fixture(t, "firing_two_alerts")
	resolved, wantResolved := fixture(t, "resolved_one")
	check := func(r reply, want string) string {
		t.Helper()
		if r.status != http.StatusCreated {
			t.Fatalf("%d %s", r.status, r.body)
		}
		msg, thread := r.result(t)
		if m := h.message(msg); m.content != want || m.origin != "app" || m.threadID != thread {
			t.Fatalf("stored %q (origin %s), want %q", m.content, m.origin, want)
		}
		return thread
	}
	public := h.s.APIBase + "/hooks/" + k.id + "/" + k.token
	if th := check(h.send(public, firing, nil), wantFiring); th != h.eskalationMain {
		t.Fatalf("firing landed in %s, want the main thread", th)
	}
	check(h.send(public, resolved, nil), wantResolved)
	internal := h.s.InternalURL + "/api/v1/hooks/" + k.id + "/" + k.token
	check(h.send(internal, firing, nil), wantFiring)

	fixed := h.create(h.tracker, h.eskalation, h.w.Threads["Support/Eskalation"])
	for _, p := range [][]byte{firing, resolved} {
		if th := check(h.send(h.s.APIBase+"/hooks/"+fixed.id+"/"+fixed.token, p, nil), map[bool]string{true: wantFiring, false: wantResolved}[string(p) == string(firing)]); th != h.w.Threads["Support/Eskalation"] {
			t.Fatalf("with a fixed thread the alert landed in %s", th)
		}
	}
}

// TestInternalListener_IncomingWebhook (16 4.2, design note 12): both post operations answer 201
// on server.internal_listen_addr in the roles all, api and worker; createMessage is not served
// there; failed tokens sent with changing X-Forwarded-For values all count against rl.public_auth
// of the peer address.
func TestInternalListener_IncomingWebhook(t *testing.T) {
	t.Parallel()
	h := newHW(t, options{})
	k := h.create(h.tracker, h.eskalation, "")
	nodes := map[string]*harness.Server{"all": h.s}
	for _, role := range []wire.Role{wire.RoleAPI, wire.RoleWorker} {
		cfg := harness.RaisedRateLimits()
		if role == wire.RoleAPI {
			cfg["ratelimit.rl.public_auth.per_minute"], cfg["ratelimit.rl.public_auth.burst"] = "1", "3"
		}
		nodes[string(role)] = harness.StartNode(t, h.s, harness.Options{EgressDenyAll: true, Role: role, Config: cfg})
	}
	bearer := http.Header{}
	bearer.Set("Authorization", "Bearer "+k.token)
	for role, n := range nodes {
		base := n.InternalURL + "/api/v1/hooks/" + k.id
		if r := h.send(base+"/"+k.token, marshal(t, native("from "+role)), nil); r.status != http.StatusCreated {
			t.Fatalf("%s path: %d %s", role, r.status, r.body)
		}
		if r := h.send(base, marshal(t, native("bearer from "+role)), bearer); r.status != http.StatusCreated {
			t.Fatalf("%s bearer: %d %s", role, r.status, r.body)
		}
		if r := h.send(n.InternalURL+"/api/v1/threads/"+h.eskalationMain+"/messages", marshal(t, native("x", "client_msg_id", newID())), nil); r.status != http.StatusNotFound {
			t.Fatalf("%s: createMessage on the internal listener answers %d", role, r.status)
		}
	}
	api := nodes[string(wire.RoleAPI)]
	for _, ip := range []string{"192.0.2.21", "192.0.2.22", "192.0.2.23"} {
		hdr := http.Header{}
		hdr.Set("X-Forwarded-For", ip)
		if r := h.send(api.InternalURL+"/api/v1/hooks/"+k.id+"/bothy_whi_wrong", marshal(t, native("x")), hdr); r.status != http.StatusNotFound {
			t.Fatalf("wrong token from %s: %d %s", ip, r.status, r.body)
		}
	}
	hdr := http.Header{}
	hdr.Set("X-Forwarded-For", "192.0.2.24")
	if r := h.send(api.InternalURL+"/api/v1/hooks/"+k.id+"/"+k.token, marshal(t, native("x")), hdr); r.status != http.StatusTooManyRequests {
		t.Fatalf("after three failures of the peer: %d %s", r.status, r.body)
	}
}
