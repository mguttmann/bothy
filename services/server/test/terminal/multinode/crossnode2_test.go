// SPDX-License-Identifier: Apache-2.0

package multinode

import (
	"context"
	"net/http"
	"strings"
	gosync "sync"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/terminal/adapters/hosttest"
	"github.com/OWNER/bothy/services/server/internal/terminal/domain"
	"github.com/OWNER/bothy/services/server/test/harness"
	"github.com/OWNER/bothy/services/server/test/terminal/fakehost"
)

// newHost pairs and confirms another host of Alex with a grant terminal; it returns key and id.
func (c *cluster) newHost(name string) (hosttest.Key, ids.ID) {
	c.t.Helper()
	key, id := c.pairedHost(name, c.alex.PrincipalID)
	c.grant(c.alex.PrincipalID, id)
	return key, id
}

// controlClose waits for the close code of the control connection of h.
func (c *cluster) controlClose(what string, h *fakehost.Host, want int) {
	c.t.Helper()
	code, err := h.ControlClose(c.ctx())
	if err != nil || code != want {
		c.t.Fatalf("%s: control connection closed with %d (%v), want %d", what, code, err, want)
	}
}

// TestCrossNode_HostTransitionsCloseEverywhere: control connections on B, a session relayed on A.
// setHostState to disabled through A closes the control connection on B with 4111 and the device
// stream and data connection on A with 4100 and host_disabled; a second host retired through A
// closes its control connection on B with 4111; a re-pairing of a third host through A closes its
// control connection on B with 4110: the codes of M2-43's single-node path.
func TestCrossNode_HostTransitionsCloseEverywhere(t *testing.T) {
	t.Parallel()
	c := newCluster(t, "multi")
	ctx := c.ctx()
	h := c.connect(nodeB, c.key, false)
	_, d, dc := c.relayed(h)
	c.setState(nodeA, c.host, "disabled")
	c.controlClose("disabled", h, int(domain.CodeHostDisabled))
	c.closeCode("device", func() (int, string) { return d.CloseCode(ctx) }, int(domain.StreamTerminalClosed), "host_disabled")
	c.closeCode("host data", func() (int, string) { return dc.CloseCode(ctx) }, int(domain.StreamTerminalClosed), "host_disabled")

	key2, host2 := c.newHost("lab-2")
	h2 := c.connectHost(nodeB, host2, key2, false)
	c.setState(nodeA, host2, "retired")
	c.controlClose("retired", h2, int(domain.CodeHostDisabled))

	key3, host3 := c.newHost("lab-3")
	h3 := c.connectHost(nodeB, host3, key3, false)
	c.repair(nodeA, host3)
	c.controlClose("re-paired", h3, int(domain.CodeHostUnpaired))
}

// TestCrossNode_DuplicateControlConnection (T-TM-05, 24 3.4 across nodes).
func TestCrossNode_DuplicateControlConnection(t *testing.T) {
	t.Parallel()
	c := newCluster(t, "multi")
	t.Run("older_answers", func(t *testing.T) {
		key, host := c.newHost("dup-1")
		h1 := c.connectHost(nodeA, host, key, false)
		g := c.openOnHost(nodeA, host)
		h2 := c.connectHost(nodeB, host, key, false)
		c.controlClose("older on A", h1, int(domain.CodeHostDuplicate))
		c.controlClose("newer on B", h2, int(domain.CodeHostDuplicate))
		c.eventually("disabled duplicate_connection", func() bool {
			return c.count(`SELECT count(*) FROM hosts WHERE id = $1 AND state = 'disabled' AND disabled_reason = 'duplicate_connection'`, host.String()) == 1
		})
		c.waitState(g.Session.ID, "closed:host_disabled")
		if n := c.count(`SELECT count(*) FROM audit_events WHERE action = 'host.connect' AND decision = 'deny' AND resource_id = $1
			AND details->>'code' = 'duplicate_connection'`, host.String()); n != 1 {
			t.Fatalf("%d audit entries of the duplicate", n)
		}
		if n := c.count(`SELECT count(*) FROM event_log WHERE type = 'host.updated' AND resource_id = $1
			AND payload::text LIKE '%duplicate_connection%'`, host.String()); n < 1 {
			t.Fatal("no host.updated of the disabled host")
		}
	})
	t.Run("older_silent", func(t *testing.T) {
		key, host := c.newHost("dup-2")
		h1 := c.connectHost(nodeA, host, key, false)
		h1.Control.StopHeartbeat()
		h1.Control.Pause()
		gone := c.countKind(domain.SignalHostGone)
		h2 := c.connectHost(nodeB, host, key, false)
		c.eventually("host_gone of the older connection", func() bool { return c.countKind(domain.SignalHostGone) > gone })
		h1.Control.Unpause()
		c.controlClose("older on A", h1, int(domain.CodeHeartbeatTimeout))
		time.Sleep(2 * probeTimeout)
		if h2.ControlEnded() {
			t.Fatal("the newer connection on B ended")
		}
		if c.count(`SELECT count(*) FROM hosts WHERE id = $1 AND state = 'active'`, host.String()) != 1 {
			t.Fatal("the host left active")
		}
	})
	t.Run("probation_defers_open", func(t *testing.T) {
		key, host := c.newHost("dup-3")
		h1 := c.connectHost(nodeA, host, key, false)
		h1.Control.StopHeartbeat()
		h1.Control.Pause()
		// A paused client cannot finish its close handshake at the cleanup.
		defer h1.Control.Unpause()
		h1.Default(fakehost.Script{Kind: fakehost.Ignore})
		h2 := c.connectHost(nodeB, host, key, false)
		h2.Default(fakehost.Script{Kind: fakehost.Ignore})
		connected := time.Now()
		r := c.openOn(nodeA, c.alex, host)
		if r.Status != http.StatusCreated {
			t.Fatalf("open: %d %s", r.Status, r.Body)
		}
		var g grantBody
		r.JSON(t, &g)
		if err := h2.WaitOpens(c.ctx(), ids.MustParse(g.Session.ID), 1); err != nil {
			t.Fatal(err)
		}
		if waited := time.Since(connected); waited < 2*probeTimeout-200*time.Millisecond {
			t.Fatalf("session.open reached the new connection after %s, before the end of its probation", waited)
		}
		// With the older connection answering, the newer one never gets a session.open.
		key4, host4 := c.newHost("dup-4")
		c.connectHost(nodeA, host4, key4, false)
		h4 := c.connectHost(nodeB, host4, key4, false)
		h4.Default(fakehost.Script{Kind: fakehost.Ignore})
		c.openOn(nodeA, c.alex, host4)
		c.controlClose("newer of dup-4", h4, int(domain.CodeHostDuplicate))
		if len(h4.Opens()) != 0 {
			t.Fatal("the newer connection got session.open during its probation")
		}
	})
	t.Run("announcement_lost", func(t *testing.T) {
		key, host := c.newHost("dup-5")
		c.connectHost(nodeA, host, key, false)
		a := c.n(nodeA)
		a.gate.close()
		c.terminateListeners()
		c.eventually("A deaf", func() bool { return len(a.logs.records("terminal ephemeral listener lost")) > 0 })
		c.connectHost(nodeB, host, key, false)
		time.Sleep(500 * time.Millisecond)
		if c.count(`SELECT count(*) FROM hosts WHERE id = $1 AND state = 'active'`, host.String()) != 1 {
			t.Fatal("the duplicate was decided while A was deaf")
		}
		a.gate.release()
		c.eventually("disabled after the reconnect", func() bool {
			return c.count(`SELECT count(*) FROM hosts WHERE id = $1 AND state = 'disabled' AND disabled_reason = 'duplicate_connection'`, host.String()) == 1
		})
	})
}

// countKind returns the number of recorded signals of kind.
func (c *cluster) countKind(kind domain.SignalKind) int {
	n := 0
	for _, k := range c.listen.kinds() {
		if k == kind {
			n++
		}
	}
	return n
}

// openOnHost opens a session of Alex on host through node and returns the grant.
func (c *cluster) openOnHost(node string, host ids.ID) grantBody {
	c.t.Helper()
	r := c.openOn(node, c.alex, host)
	if r.Status != http.StatusCreated {
		c.t.Fatalf("open: %d %s", r.Status, r.Body)
	}
	var g grantBody
	r.JSON(c.t, &g)
	return g
}

// terminateListeners ends the LISTEN ephemeral backends of the cluster's database.
func (c *cluster) terminateListeners() {
	c.t.Helper()
	if _, err := c.n(nodeA).srv.AdminDB().Exec(context.Background(), `SELECT pg_terminate_backend(pid) FROM pg_stat_activity
		WHERE application_name = 'bothy-terminal-ephemeral' AND datname = current_database()`); err != nil {
		c.t.Fatal(err)
	}
}

// TestCrossNode_HostUnavailableParity (24 6.1 point 2, 13): without a control connection anywhere
// both nodes answer 409 host_unavailable; with the control connection on B an open through A gives
// 201; after the Fake-Host left, 409 again; a node B that vanishes without terminal.host_gone (its
// signals muted, like a crash) counts on A until the presence window passed, and a session opened
// in that window ends open_failed open_timeout.
func TestCrossNode_HostUnavailableParity(t *testing.T) {
	t.Parallel()
	c := newCluster(t, "multi")
	for _, n := range []string{nodeA, nodeB} {
		if r := c.open(n, c.alex); r.Status != http.StatusConflict || !hasCode(r, "host_unavailable") {
			t.Fatalf("no connection, through %s: %d %s", n, r.Status, r.Body)
		}
	}
	h := c.connect(nodeB, c.key, false)
	c.eventually("201 through A", func() bool { return c.openStatus(nodeA) == http.StatusCreated })
	h.Close()
	c.eventually("409 through A", func() bool { return c.openStatus(nodeA) == http.StatusConflict })

	h2 := c.connect(nodeB, c.key, false)
	c.eventually("201 through A", func() bool { return c.openStatus(nodeA) == http.StatusCreated })
	c.n(nodeB).signals.set(true, false)
	h2.Drop()
	r := c.open(nodeA, c.alex)
	if r.Status != http.StatusCreated {
		t.Fatalf("within the window: %d %s", r.Status, r.Body)
	}
	var g grantBody
	r.JSON(t, &g)
	c.eventually("409 after the window", func() bool { return c.openStatus(nodeA) == http.StatusConflict })
	c.n(nodeA).timers.Fire()
	c.waitState(g.Session.ID, "closed:open_failed:open_timeout")
}

// openStatus opens through node and returns the status.
func (c *cluster) openStatus(node string) int { return c.open(node, c.alex).Status }

// hasCode reports a problem body with code.
func hasCode(r harness.Response, code string) bool {
	return strings.Contains(string(r.Body), `"code":"`+code+`"`)
}

// TestCrossNode_ListenerLossReconciles (T-TM-11): control and relay on A; A's listener backend ends
// and its reconnect waits; the host is disabled through B: A's device stream and data connection
// still close with 4100 host_disabled through the log tail, A's control connection stays open and
// /readyz of A answers 200; after the reconnect A closes the control connection with 4111; A logs
// one "listener lost" and one "listener restored".
func TestCrossNode_ListenerLossReconciles(t *testing.T) {
	t.Parallel()
	// A long heartbeat interval: the test stops the heartbeats of the Fake-Host, whose recheck would
	// otherwise close the control connection by itself (M2-43).
	c := newClusterWith(t, "multi", map[string]string{"terminal.host_heartbeat_interval": "30s"})
	ctx := c.ctx()
	h := c.connect(nodeA, c.key, false)
	g := c.hashedTo(nodeA, c.host)
	d := c.device(c.alex, g.Session.ID)
	c.handshake(d, g.SessionToken)
	c.waitState(g.Session.ID, "active")
	dc := h.Data(ids.MustParse(g.Session.ID))
	h.Control.StopHeartbeat()
	a := c.n(nodeA)
	a.gate.close()
	c.terminateListeners()
	c.eventually("A deaf", func() bool { return len(a.logs.records("terminal ephemeral listener lost")) == 1 })
	c.setState(nodeB, c.host, "disabled")
	c.closeCode("device", func() (int, string) { return d.CloseCode(ctx) }, int(domain.StreamTerminalClosed), "host_disabled")
	c.closeCode("host data", func() (int, string) { return dc.CloseCode(ctx) }, int(domain.StreamTerminalClosed), "host_disabled")
	time.Sleep(500 * time.Millisecond)
	if h.ControlEnded() {
		t.Fatal("the control connection on A closed while A was deaf")
	}
	if !c.ready() {
		t.Fatal("A is not ready while its listener is down")
	}
	a.gate.release()
	c.controlClose("after the reconnect", h, int(domain.CodeHostDisabled))
	c.eventually("restored", func() bool { return len(a.logs.records("terminal ephemeral listener restored")) == 1 })
	if n := len(a.logs.records("terminal ephemeral listener lost")); n != 1 {
		t.Fatalf("%d lost records", n)
	}
}

// TestCrossNode_StepUpConsumedOnce (ID-T28 across nodes): Alex confirms one fresh step-up; two
// concurrent openings with it, one through A and one through B, give exactly one 201 and one 401
// step_up_required, and one row.
func TestCrossNode_StepUpConsumedOnce(t *testing.T) {
	t.Parallel()
	c := newCluster(t, "multi")
	c.connect(nodeB, c.key, false)
	c.eventually("the presence on A", func() bool { return c.openStatus(nodeA) == http.StatusCreated })
	before := c.count(`SELECT count(*) FROM terminal_sessions`)
	if err := c.alex.StepUp(c.t.Context()); err != nil {
		t.Fatal(err)
	}
	var wg gosync.WaitGroup
	statuses := make([]int, 2)
	for i, n := range []string{nodeA, nodeB} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			statuses[i] = c.alex.Do(context.Background(), req(n, http.MethodPost, "/hosts/"+c.host.String()+"/terminal/sessions", openBody())).Status
		}()
	}
	wg.Wait()
	got := map[int]int{}
	for _, s := range statuses {
		got[s]++
	}
	if got[http.StatusCreated] != 1 || got[http.StatusUnauthorized] != 1 {
		t.Fatalf("statuses %v", statuses)
	}
	if n := c.count(`SELECT count(*) FROM terminal_sessions`) - before; n != 1 {
		t.Fatalf("%d rows", n)
	}
}
