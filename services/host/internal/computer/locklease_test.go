// SPDX-License-Identifier: Apache-2.0

package computer

import (
	"context"
	"net"
	"sync/atomic"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/host/internal/confirm"
	"github.com/OWNER/bothy/services/host/internal/control"
	"github.com/OWNER/bothy/services/host/internal/frame"
	"github.com/OWNER/bothy/services/host/internal/platform/testkit"
)

// gateProc is a captured process whose checks block while hold is set.
type gateProc struct {
	hold    atomic.Bool
	waiting atomic.Bool
	release chan struct{}
}

func (g *gateProc) Current() error {
	if g.hold.Load() {
		g.waiting.Store(true)
		<-g.release
	}
	return nil
}
func (g *gateProc) ExePath() (string, error) { return helperPath, nil }
func (g *gateProc) Close() error             { return nil }

// TestEndDuringHandshakeFreesLock: a session that ends while the checks of message 1 run (here
// computer.close during the peer check of step 9) leaves the controller lock free, so the next
// open is not busy; the first session reports no computer.started (review finding of CU-03).
func TestEndDuringHandshakeFreesLock(t *testing.T) {
	r := newRig(t)
	r.h.o.Rule = rule(nil, "", nil, map[string]bool{helperPath: true})
	g := &gateProc{release: make(chan struct{})}
	r.h.o.Bind = func(net.Conn, confirm.Peer, time.Time) (Process, error) { return g, nil }
	_ = r.connect(hello())
	r.waitState(func(s HelperState) bool { return s.Connected })
	r.m.Open(context.Background(), openOp(sidA), r.n)
	g.hold.Store(true)
	done := make(chan bool, 1)
	go func() {
		_, ok := r.handshake(sidA, nil)
		done <- ok
	}()
	deadline := testkit.Deadline()
	for !g.waiting.Load() && time.Now().Before(deadline) {
		time.Sleep(2 * time.Millisecond)
	}
	if !g.waiting.Load() {
		t.Fatal("the check of message 1 never ran")
	}
	r.m.Close(context.Background(), control.ComputerClose{SessionID: sidA, Reason: "run_ended"})
	g.hold.Store(false)
	close(g.release)
	if <-done {
		t.Error("message 2 for an ended session")
	}
	time.Sleep(20 * time.Millisecond)
	r.m.lock.mu.Lock()
	holder := r.m.lock.holder
	r.m.lock.mu.Unlock()
	if holder != "" {
		t.Fatalf("controller lock held by %s after the end", holder)
	}
	_, st := r.open(sidB)
	if st.State != frame.StateActive {
		t.Errorf("next session %+v", st)
	}
	r.n.mu.Lock()
	defer r.n.mu.Unlock()
	for _, s := range r.n.started {
		if s.SessionID == sidA {
			t.Error("computer.started for the ended session")
		}
	}
}
