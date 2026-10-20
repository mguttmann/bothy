// SPDX-License-Identifier: Apache-2.0

package multinode

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/terminal/domain"
	"github.com/OWNER/bothy/services/server/test/harness"
	"github.com/OWNER/bothy/services/server/test/terminal/fakehost"
)

// pgNotify sends a payload on the channel ephemeral as a forger with database access would.
func (c *cluster) pgNotify(payload string) {
	c.t.Helper()
	if _, err := c.n(nodeA).srv.AdminDB().Exec(context.Background(), `SELECT pg_notify('ephemeral', $1)`, payload); err != nil {
		c.t.Fatal(err)
	}
}

// TestCrossNode_SessionOpenOnlyAfterCommit: control on B; a wrapper around the NOTIFY signals of A
// stages terminal.session_open and then fails the unit: the open answers an error, no row exists,
// the Fake-Host gets no session.open and a test connection with its own LISTEN receives nothing.
func TestCrossNode_SessionOpenOnlyAfterCommit(t *testing.T) {
	t.Parallel()
	c := newCluster(t, "multi")
	h := c.connect(nodeB, c.key, false)
	c.n(nodeA).signals.set(false, true)
	before := len(c.listen.kinds())
	r := c.open(nodeA, c.alex)
	if r.Status < 400 {
		t.Fatalf("the open answered %d", r.Status)
	}
	time.Sleep(500 * time.Millisecond)
	if n := c.count(`SELECT count(*) FROM terminal_sessions`); n != 0 {
		t.Fatalf("%d sessions", n)
	}
	if len(h.Opens()) != 0 {
		t.Fatal("the Fake-Host got session.open")
	}
	for _, k := range c.listen.kinds()[before:] {
		if k == domain.SignalSessionOpen {
			t.Fatal("a rolled-back signal was delivered")
		}
	}
}

// TestCrossNode_SignalsCarryOnlyIDs: a test connection with LISTEN ephemeral records every payload
// of an opening, a pairing, an attach, closes and a host transition; each decodes strictly into its
// kind, and none holds the session token, its jti, the profile, the host name, the session user,
// a person ID or a device ID (canary search).
func TestCrossNode_SignalsCarryOnlyIDs(t *testing.T) {
	t.Parallel()
	c := newCluster(t, "multi")
	c.connect(nodeB, c.key, false)
	g := c.opened(nodeA, c.alex)
	d := c.device(c.alex, g.Session.ID)
	c.handshake(d, g.SessionToken)
	c.waitState(g.Session.ID, "active")
	g2 := c.attach(nodeB, c.alex, g.Session.ID)
	c.closeSession(g.Session.ID)
	g3 := c.opened(nodeB, c.alex)
	c.must(nodeA, c.robin, http.MethodPost, "/terminal/sessions/"+g3.Session.ID+"/close", map[string]any{"reason": "owner check"}, http.StatusOK)
	c.setState(nodeA, c.host, "disabled")
	c.eventually("a host_close", func() bool {
		for _, k := range c.listen.kinds() {
			if k == domain.SignalHostClose {
				return true
			}
		}
		return false
	})
	canaries := []string{g.SessionToken, g2.SessionToken, g3.SessionToken, jti(t, g.SessionToken), jti(t, g2.SessionToken), "shell", "lab-1",
		"bothy-s-alex", c.alex.PrincipalID, c.robin.PrincipalID, c.alex.DeviceID, c.robin.DeviceID}
	payloads := c.listen.all()
	if len(payloads) == 0 {
		t.Fatal("no signal recorded")
	}
	for _, p := range payloads {
		if _, ours, err := domain.DecodeSignal([]byte(p)); err != nil || !ours {
			t.Fatalf("a payload that is no strict terminal signal: %v %v", ours, err)
		}
		for _, canary := range canaries {
			if canary != "" && strings.Contains(p, canary) {
				t.Fatalf("a signal carries a secret or a name (canary %d)", len(canary))
			}
		}
	}
}

// jti returns the jti claim of a compact JWS without checking it.
func jti(t *testing.T, tok string) string {
	t.Helper()
	parts := strings.Split(tok, ".")
	if len(parts) != 3 {
		t.Fatal("no compact JWS")
	}
	b, _ := base64.RawURLEncoding.DecodeString(parts[1])
	var claims struct {
		JTI string `json:"jti"`
	}
	_ = json.Unmarshal(b, &claims)
	return claims.JTI
}

// setState sets the state of host through node as Robin with a step-up.
func (c *cluster) setState(node string, host ids.ID, state string) {
	c.t.Helper()
	get := c.must(node, c.robin, http.MethodGet, "/hosts/"+host.String(), nil, http.StatusOK)
	r := req(node, http.MethodPost, "/hosts/"+host.String()+"/state", map[string]any{"state": state, "reason": "terminal test"})
	r.Header.Set("If-Match", get.Header.Get("ETag"))
	resp, err := harness.DoWithStepUp(c.t.Context(), c.robin, r)
	if err != nil || resp.Status != http.StatusOK {
		c.t.Fatalf("setHostState %s: %v %d %s", state, err, resp.Status, resp.Body)
	}
}

// repair issues a new pairing code of host through node.
func (c *cluster) repair(node string, host ids.ID) {
	c.t.Helper()
	resp, err := harness.DoWithStepUp(c.t.Context(), c.robin, req(node, http.MethodPost, "/hosts/"+host.String()+"/pairing-code",
		map[string]any{"reason": "terminal test"}))
	if err != nil || resp.Status >= 300 {
		c.t.Fatalf("issueHostPairingCode: %v %d %s", err, resp.Status, resp.Body)
	}
}

// TestCrossNode_ForgedSignalOpensNothing: terminal.session_open for a random session, for a closed
// session and for a session of another host, and malformed payloads: the Fake-Host gets no
// session.open beyond the real one; each malformed payload gives one "terminal signal dropped" on
// every node; typing.started gives nothing.
func TestCrossNode_ForgedSignalOpensNothing(t *testing.T) {
	t.Parallel()
	c := newCluster(t, "multi")
	h := c.connect(nodeB, c.key, false)
	ctx := c.ctx()
	open := c.opened(nodeA, c.alex)
	closed := c.opened(nodeA, c.alex)
	for _, g := range []grantBody{open, closed} {
		if err := h.WaitOpens(ctx, ids.MustParse(g.Session.ID), 1); err != nil {
			t.Fatal(err)
		}
	}
	c.closeSession(closed.Session.ID)
	baseline := len(h.Opens())
	enc := func(host, session string) string {
		return `{"kind":"terminal.session_open","host_id":"` + host + `","session_id":"` + session + `"}`
	}
	c.pgNotify(enc(c.host.String(), ids.New().String()))
	c.pgNotify(enc(c.host.String(), closed.Session.ID))
	c.pgNotify(enc(ids.New().String(), open.Session.ID))
	malformed := []string{`{"kind":"terminal.session_open","host_id":"x"}`, `{"kind":"terminal.unknown","host_id":"` + c.host.String() + `"}`, `not json`}
	for _, p := range malformed {
		c.pgNotify(p)
	}
	c.pgNotify(`{"kind":"typing.started","thread_id":"` + ids.New().String() + `"}`)
	for _, n := range []string{nodeA, nodeB} {
		c.eventually("the dropped records of "+n, func() bool {
			return len(c.n(n).logs.records("terminal signal dropped")) >= len(malformed)
		})
	}
	time.Sleep(500 * time.Millisecond)
	for _, n := range []string{nodeA, nodeB} {
		if got := len(c.n(n).logs.records("terminal signal dropped")); got != len(malformed) {
			t.Fatalf("node %s: %d dropped records, want %d", n, got, len(malformed))
		}
	}
	if len(h.Opens()) != baseline {
		t.Fatalf("a forged signal sent session.open (%d, want %d)", len(h.Opens()), baseline)
	}
}

// TestCrossNode_SingleModeSendsNoNotify: one node with cluster.mode single: open, attach and close
// work as M2-44's tests expect; the test listener receives nothing and no backend runs with the
// application name of the terminal listener.
func TestCrossNode_SingleModeSendsNoNotify(t *testing.T) {
	t.Parallel()
	c := newCluster(t, "single")
	c.connect(nodeA, c.key, false)
	g := c.opened(nodeA, c.alex)
	d := c.device(c.alex, g.Session.ID)
	c.handshake(d, g.SessionToken)
	c.waitState(g.Session.ID, "active")
	_ = d.Close()
	c.waitState(g.Session.ID, "detached")
	g2 := c.attach(nodeA, c.alex, g.Session.ID)
	d2 := c.device(c.alex, g.Session.ID)
	c.handshake(d2, g2.SessionToken)
	c.waitState(g.Session.ID, "active")
	c.closeSession(g.Session.ID)
	time.Sleep(300 * time.Millisecond)
	if p := c.listen.all(); len(p) != 0 {
		t.Fatalf("%d notifications with one node", len(p))
	}
	if n := c.count(`SELECT count(*) FROM pg_stat_activity WHERE application_name = 'bothy-terminal-ephemeral' AND datname = current_database()`); n != 0 {
		t.Fatalf("%d listener backends with one node", n)
	}
}

// hashedTo opens sessions through A until one hashes to node; the others are closed.
func (c *cluster) hashedTo(node string, host ids.ID) grantBody {
	c.t.Helper()
	for range 16 {
		r := c.openOn(nodeA, c.alex, host)
		if r.Status != http.StatusCreated {
			c.t.Fatalf("open: %d %s", r.Status, r.Body)
		}
		var g grantBody
		r.JSON(c.t, &g)
		if hashNode(g.Session.ID) == node {
			return g
		}
		c.closeSession(g.Session.ID)
	}
	c.t.Fatalf("no session hashed to %s", node)
	return grantBody{}
}

// relayed opens a session relayed on A (hash node A) of the host whose control connection h holds
// on B and runs the handshake; it returns the grant, the device and the host data connection.
func (c *cluster) relayed(h *fakehost.Host) (grantBody, *fakehost.Device, *fakehost.DataConn) {
	c.t.Helper()
	g := c.hashedTo(nodeA, h.ID)
	d := c.device(c.alex, g.Session.ID)
	c.handshake(d, g.SessionToken)
	c.waitState(g.Session.ID, "active")
	dc := h.Data(ids.MustParse(g.Session.ID))
	if dc == nil {
		c.t.Fatal("no data connection")
	}
	return g, d, dc
}

// TestCrossNode_CloseReachesRelayAndControl (T-TM-11): control on B, relay on A. Each end reaches
// the device stream and the host data connection on A with 4100 and its reason, and session.close
// on B: a close by the person through B, a close by Robin through A, session.ended of the Fake-Host
// on B, and an end committed in a unit of instance C in role worker.
func TestCrossNode_CloseReachesRelayAndControl(t *testing.T) {
	t.Parallel()
	c := newCluster(t, "multi")
	h := c.connect(nodeB, c.key, false)
	ctx := c.ctx()
	check := func(name string, end func(session string), reason string) {
		g, d, dc := c.relayed(h)
		end(g.Session.ID)
		c.closeCode(name+" device", func() (int, string) { return d.CloseCode(ctx) }, int(domain.StreamTerminalClosed), reason)
		c.closeCode(name+" host data", func() (int, string) { return dc.CloseCode(ctx) }, int(domain.StreamTerminalClosed), reason)
		if got, err := h.WaitClose(ctx, ids.MustParse(g.Session.ID)); err != nil || got != reason {
			c.t.Fatalf("%s: session.close %q %v", name, got, err)
		}
	}
	check("person_close", func(s string) {
		c.must(nodeB, c.alex, http.MethodPost, "/terminal/sessions/"+s+"/close", map[string]any{"reason": "done"}, http.StatusOK)
	}, "client_closed")
	check("owner_close", func(s string) {
		c.must(nodeA, c.robin, http.MethodPost, "/terminal/sessions/"+s+"/close", map[string]any{"reason": "owner check"}, http.StatusOK)
	}, "closed_by_owner")
	check("host_reported_end", func(s string) {
		if err := h.Send("session.ended", map[string]any{"session_id": s, "close_reason": "process_exited", "exit_code": 0}); err != nil {
			c.t.Fatal(err)
		}
	}, "process_exited")
	_, svc := c.worker()
	check("worker_end", func(s string) {
		if err := svc.RelayOpenFailed(context.Background(), ids.MustParse(s), ""); err != nil {
			c.t.Fatal(err)
		}
	}, "open_failed")
}
