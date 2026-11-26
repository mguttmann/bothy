// SPDX-License-Identifier: Apache-2.0

package computer

import (
	"context"
	"errors"
	"net"
	"sync/atomic"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/host/internal/confirm"
)

// TestPeerExecAfterConnectAndPIDReuse: the peer rule checks the process the connection captured;
// an exec of another image after the connect (the same PID and start time, another PID version)
// and a process that took over the PID of an exited peer are both refused, at the connection and
// at every later check (27 9.4, D-CU-03-12).
func TestPeerExecAfterConnectAndPIDReuse(t *testing.T) {
	procs := procTable{10: helperPath}
	r := rule(procs, "", nil, map[string]bool{helperPath: true})
	peer := confirm.Peer{UID: uidAlex, PID: 10}
	p := procs.proc(10)
	if err := r.Check(context.Background(), peer, p); err != nil {
		t.Fatalf("genuine helper: %v", err)
	}
	*p.now = 2 // the connector exec'd the helper image, or exited and another process took the PID
	if err := r.Check(context.Background(), peer, p); !errors.Is(err, ErrPeerRefused) {
		t.Errorf("changed process: %v", err)
	}
	if err := r.Check(context.Background(), peer, nil); !errors.Is(err, ErrPeerRefused) {
		t.Errorf("no captured process: %v", err)
	}
}

// changingProc is a captured process that turns into another one after a number of checks.
type changingProc struct {
	calls atomic.Int64
	after int64
}

func (c *changingProc) Current() error {
	if c.calls.Add(1) > c.after {
		return errors.New("exec after connect")
	}
	return nil
}
func (c *changingProc) ExePath() (string, error) { return helperPath, c.Current() }
func (c *changingProc) Close() error             { return nil }

// TestHelperRecheckAfterHello: a peer whose process changes while the host service waits for
// hello is refused after hello; the helper stays absent and the refusal is logged.
func TestHelperRecheckAfterHello(t *testing.T) {
	r := newRig(t)
	r.h.o.Rule = rule(nil, "", nil, map[string]bool{helperPath: true})
	cp := &changingProc{after: 3} // the checks before hello pass, the one after it fails
	r.h.o.Bind = func(net.Conn, confirm.Peer, time.Time) (Process, error) { return cp, nil }
	r.connect(hello())
	deadline := time.Now().Add(wait)
	for time.Now().Before(deadline) && cp.calls.Load() <= cp.after {
		time.Sleep(5 * time.Millisecond)
	}
	time.Sleep(50 * time.Millisecond)
	if st := r.h.State(); st.Connected || !st.Refused {
		t.Fatalf("helper state after its process changed: %+v", st)
	}
}

// TestRecheckRefusalReportsRefused: a helper that a later check refuses is reported as refused,
// not absent.
func TestRecheckRefusalReportsRefused(t *testing.T) {
	r := newRig(t)
	r.h.o.Rule = rule(nil, "", nil, map[string]bool{helperPath: true})
	cp := &changingProc{after: 6} // the checks before and after hello pass (three calls each), the next fails
	r.h.o.Bind = func(net.Conn, confirm.Peer, time.Time) (Process, error) { return cp, nil }
	r.connect(hello())
	r.waitState(func(s HelperState) bool { return s.Connected })
	if err := r.h.Recheck(context.Background()); !errors.Is(err, ErrPeerRefused) {
		t.Fatalf("recheck %v", err)
	}
	r.waitState(func(s HelperState) bool { return !s.Connected && s.Refused })
}
