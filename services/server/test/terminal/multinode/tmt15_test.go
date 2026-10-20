// SPDX-License-Identifier: Apache-2.0

package multinode

import (
	"bytes"
	"crypto/rand"
	"strings"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/terminal/domain"
	"github.com/OWNER/bothy/services/server/test/terminal/fakehost"
)

// other returns the other node.
func other(n string) string {
	if n == nodeA {
		return nodeB
	}
	return nodeA
}

// TestTMT15_TwoNodesWithHashRouting (TM-T15, first part): edge hash, control connection on B.
// Sessions opened through A until one hashes to A and one to B: the Fake-Host gets exactly one
// session.open on B with the session's data path; both stream paths meet on the hash node; the
// handshake and later frames pass byte for byte; session.started makes the row active; the token
// issued on A verifies with the key set B sent in hello; no 4102, no unpaired count, /readyz 200.
// Then the device leaves, an attach runs through the other node, the new device pairs on the same
// hash node, and an older device still open there gets 4103.
func TestTMT15_TwoNodesWithHashRouting(t *testing.T) {
	t.Parallel()
	c := newCluster(t, "multi")
	h := c.connect(nodeB, c.key, false)
	seen := map[string]bool{}
	for i := 0; i < 16 && len(seen) < 2; i++ {
		g := c.opened(nodeA, c.alex)
		hn := hashNode(g.Session.ID)
		if seen[hn] {
			c.closeSession(g.Session.ID)
			continue
		}
		seen[hn] = true
		c.runHashSession(h, g, hn)
	}
	if len(seen) != 2 {
		t.Fatalf("hash nodes %v", seen)
	}
	for _, n := range []string{nodeA, nodeB} {
		if c.n(n).metrics.of(domain.SideClient)+c.n(n).metrics.of(domain.SideHost) != 0 {
			t.Fatalf("node %s counted an unpaired side", n)
		}
	}
	if !c.ready() {
		t.Fatal("a node is not ready")
	}
}

// closeSession closes a session as Alex through A.
func (c *cluster) closeSession(session string) {
	c.t.Helper()
	c.must(nodeA, c.alex, "POST", "/terminal/sessions/"+session+"/close", map[string]any{"reason": "done"}, 200)
	c.waitState(session, "closed:client_closed")
}

// runHashSession runs the checks of one session hashed to node hn.
func (c *cluster) runHashSession(h *fakehost.Host, g grantBody, hn string) {
	c.t.Helper()
	ctx := c.ctx()
	sid := ids.MustParse(g.Session.ID)
	if err := h.WaitOpens(ctx, sid, 1); err != nil {
		c.t.Fatal(err)
	}
	opens := 0
	for _, o := range h.Opens() {
		if ids.ID(o.SessionId) == sid {
			opens++
			if o.DataPath != "/api/v1/hosts/sessions/"+g.Session.ID+"/stream" {
				c.t.Fatalf("data_path %s", o.DataPath)
			}
		}
	}
	if opens != 1 {
		c.t.Fatalf("%d session.open, want 1", opens)
	}
	d := c.device(c.alex, g.Session.ID)
	c.handshake(d, g.SessionToken)
	c.waitState(g.Session.ID, "active")
	// The Fake-Host on B verified the token issued on A with the key set B sent in hello before it
	// answered message 2 (fakehost Accept; a refusal closes with 4104).
	for _, n := range []int{1, 1000, 65535} {
		m := make([]byte, n)
		_, _ = rand.Read(m)
		if err := d.Send(ctx, m); err != nil {
			c.t.Fatal(err)
		}
		back, err := d.Recv(ctx)
		if err != nil || !bytes.Equal(back, m) {
			c.t.Fatalf("%d bytes: echo differs (%v)", n, err)
		}
	}
	// The device leaves; the session detaches on the hash node.
	_ = d.Close()
	c.waitState(g.Session.ID, "detached")
	// Attach through the other node with the first device, while a second device holds an older
	// stream: that stream gets 4103 on the hash node.
	g2 := c.attach(other(hn), c.alex2, g.Session.ID)
	d2 := c.device(c.alex2, g.Session.ID)
	c.handshake(d2, g2.SessionToken)
	c.waitState(g.Session.ID, "active")
	g3 := c.attach(other(hn), c.alex, g.Session.ID)
	c.closeCode("older device", func() (int, string) { return d2.CloseCode(ctx) }, int(domain.StreamReplaced), "")
	d3 := c.device(c.alex, g.Session.ID)
	c.handshake(d3, g3.SessionToken)
	c.waitState(g.Session.ID, "active")
	c.closeSession(g.Session.ID)
	c.closeCode("closed device", func() (int, string) { return d3.CloseCode(ctx) }, int(domain.StreamTerminalClosed), "client_closed")
}

// TestTMT15_WithoutHashRouting (TM-T15, second part; PLAN row M2-47): edge split, open through A,
// the device stream on A and the host data connection on B. Before the lone timers both stay open;
// then the device closes with 4102 on A and the host data connection with 4102 on B, each node
// counts its side once, the row is closed open_failed open_timeout with one terminal.terminate and
// one event of the end, no byte crossed, /readyz stays 200, and each node logs one record
// "terminal relay side unpaired" with side and code and without any ID.
func TestTMT15_WithoutHashRouting(t *testing.T) {
	t.Parallel()
	c := newCluster(t, "multi")
	h := c.connect(nodeB, c.key, false)
	c.edge.setMode(modeSplit)
	g := c.opened(nodeA, c.alex)
	ctx := c.ctx()
	sid := ids.MustParse(g.Session.ID)
	d := c.device(c.alex, g.Session.ID)
	dc, err := h.WaitData(ctx, sid)
	if err != nil {
		t.Fatal(err)
	}
	if err := d.Sync(ctx); err != nil {
		t.Fatal(err)
	}
	if err := dc.Sync(ctx); err != nil {
		t.Fatal(err)
	}
	// Message 1 of the device never reaches the Fake-Host on the other node.
	if err := d.Send(ctx, []byte(g.SessionToken)); err != nil {
		t.Fatal(err)
	}
	if d.Ended() || !c.ready() {
		t.Fatal("before open_timeout: the device ended or a node is not ready")
	}
	c.n(nodeA).timers.Fire()
	c.n(nodeB).timers.Fire()
	c.closeCode("device on A", func() (int, string) { return d.CloseCode(ctx) }, int(domain.StreamHostUnavailable), "")
	c.closeCode("host data on B", func() (int, string) { return dc.CloseCode(ctx) }, int(domain.StreamHostUnavailable), "")
	c.waitState(g.Session.ID, "closed:open_failed:open_timeout")
	if c.n(nodeA).metrics.of(domain.SideClient) != 1 || c.n(nodeB).metrics.of(domain.SideHost) != 1 ||
		c.n(nodeA).metrics.of(domain.SideHost) != 0 || c.n(nodeB).metrics.of(domain.SideClient) != 0 {
		t.Fatal("bothy.terminal.relay.unpaired per node and side")
	}
	c.eventually("one end", func() bool {
		return c.count(`SELECT count(*) FROM audit_events WHERE action = 'terminal.terminate' AND resource_id = $1`, g.Session.ID) == 1
	})
	if n := c.count(`SELECT count(*) FROM event_log WHERE type = 'terminal_session.changed' AND resource_id = $1 AND payload->>'state' = 'closed'`,
		g.Session.ID); n != 1 {
		t.Fatalf("%d events of the end", n)
	}
	if len(h.Tokens()) != 0 {
		t.Fatal("a byte of the device reached the Fake-Host")
	}
	if !c.ready() {
		t.Fatal("a node is not ready after the timeout")
	}
	for name, side := range map[string]string{nodeA: "client", nodeB: "host"} {
		recs := c.n(name).logs.records("terminal relay side unpaired")
		if len(recs) != 1 || recs[0]["bothy.terminal.side"] != side || recs[0]["bothy.close.code"] != float64(4102) {
			t.Fatalf("node %s records %v", name, recs)
		}
		for _, secret := range []string{g.Session.ID, g.SessionToken, c.alex.PrincipalID, c.alex.DeviceID, c.host.String()} {
			if strings.Contains(c.n(name).logs.String(), `"terminal relay side unpaired"`) && containsRecord(recs, secret) {
				t.Fatalf("node %s logs an ID", name)
			}
		}
	}
}

// containsRecord reports whether one of recs holds s in a value.
func containsRecord(recs []map[string]any, s string) bool {
	for _, r := range recs {
		for _, v := range r {
			if str, ok := v.(string); ok && strings.Contains(str, s) {
				return true
			}
		}
	}
	return false
}

// TestTMT15_WithoutHashRouting_Attach: a session runs with hash; the edge switches to split; the
// device leaves and attaches; the new device lands on A and gets 4102 at its lone timer, A counts
// client, the session stays detached and the host data connection of the started session stays
// open on B; with hash again the next attach pairs.
func TestTMT15_WithoutHashRouting_Attach(t *testing.T) {
	t.Parallel()
	c := newCluster(t, "multi")
	h := c.connect(nodeB, c.key, false)
	var g grantBody
	for i := 0; i < 16; i++ {
		g = c.opened(nodeA, c.alex)
		if hashNode(g.Session.ID) == nodeB {
			break
		}
		c.closeSession(g.Session.ID)
	}
	if hashNode(g.Session.ID) != nodeB {
		t.Fatal("no session hashed to B")
	}
	ctx := c.ctx()
	sid := ids.MustParse(g.Session.ID)
	d := c.device(c.alex, g.Session.ID)
	c.handshake(d, g.SessionToken)
	c.waitState(g.Session.ID, "active")
	c.edge.setMode(modeSplit)
	_ = d.Close()
	c.waitState(g.Session.ID, "detached")
	old := h.Data(sid)
	c.attach(nodeA, c.alex, g.Session.ID)
	if err := h.WaitOpens(ctx, sid, 2); err != nil {
		t.Fatal(err)
	}
	var dc *fakehost.DataConn
	c.eventually("the new data connection", func() bool {
		dc = h.Data(sid)
		return dc != nil && dc != old
	})
	d2 := c.device(c.alex, g.Session.ID)
	if err := d2.Sync(ctx); err != nil {
		t.Fatal(err)
	}
	if err := dc.Sync(ctx); err != nil {
		t.Fatal(err)
	}
	c.n(nodeA).timers.Fire()
	c.n(nodeB).timers.Fire()
	c.closeCode("device on A", func() (int, string) { return d2.CloseCode(ctx) }, int(domain.StreamHostUnavailable), "")
	if c.n(nodeA).metrics.of(domain.SideClient) != 1 {
		t.Fatal("A did not count the client side")
	}
	if c.state(g.Session.ID) != "detached" {
		t.Fatalf("session %s, want detached", c.state(g.Session.ID))
	}
	if err := dc.Sync(ctx); err != nil {
		t.Fatalf("the host data connection on B closed: %v", err)
	}
	c.edge.setMode(modeHash)
	g3 := c.attach(nodeA, c.alex, g.Session.ID)
	d3 := c.device(c.alex, g.Session.ID)
	c.handshake(d3, g3.SessionToken)
	c.waitState(g.Session.ID, "active")
}
