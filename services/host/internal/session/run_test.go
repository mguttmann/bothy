// SPDX-License-Identifier: Apache-2.0

package session

import (
	"testing"

	"github.com/OWNER/bothy/services/host/internal/control"
	"github.com/OWNER/bothy/services/host/internal/doctor"
	"github.com/OWNER/bothy/services/host/internal/frame"
)

// TestOutputOtherThanDataIgnored: helper frames that do not decode or are no data frames neither
// reach the device nor end the session.
func TestOutputOtherThanDataIgnored(t *testing.T) {
	r := newPipeRig(t)
	dev, dc := r.open()
	if err := r.helper.sendRaw(sessID, []byte{0xff, 0x00}); err != nil {
		t.Fatal(err)
	}
	r.helper.output(t, sessID, frame.ReplayEnd())
	r.helper.output(t, sessID, frame.Data([]byte("visible")))
	if f := r.next(dc, dev); f.Type != frame.TypeData || string(f.Data) != "visible" {
		t.Errorf("first frame on the device %v %q", f.Type, f.Data)
	}
	if r.mgr.Active() != 1 {
		t.Error("ignored output ended the session")
	}
}

// TestExitWhileDetached: a process that exits while no device is attached ends the session with
// process_exited and its exit code.
func TestExitWhileDetached(t *testing.T) {
	r := newPipeRig(t)
	_, dc := r.open()
	r.detach(dc)
	r.helper.exited(t, sessID, 3)
	e := recv(t, r.ctx, "session.ended", r.note.ended)
	wantEnded(t, e, ReasonProcessExited, "")
	if e.ExitCode == nil || *e.ExitCode != 3 {
		t.Errorf("exit code %v", e.ExitCode)
	}
}

// TestBrokenChannelEndsItsSessions: when the session channel of a helper breaks, its sessions end
// with process_exited without an exit code (D-M2-45 ruling).
func TestBrokenChannelEndsItsSessions(t *testing.T) {
	r := newPipeRig(t)
	r.open()
	if err := r.helper.helperEnd().Close(); err != nil {
		t.Fatal(err)
	}
	e := recv(t, r.ctx, "session.ended", r.note.ended)
	wantEnded(t, e, ReasonProcessExited, "")
	if e.ExitCode != nil {
		t.Errorf("exit code %d for a broken channel", *e.ExitCode)
	}
	if r.mgr.Active() != 0 {
		t.Error("the session still runs")
	}
}

// TestEnforceEndsTheSessionsOfTheUser: a periodic run that finds HD11 at a place of the session
// user ends that user's sessions with host_policy_violation/shared_location.
func TestEnforceEndsTheSessionsOfTheUser(t *testing.T) {
	r := newPipeRig(t)
	r.open()
	res := pipeResult()
	for i := range res.Checks {
		if res.Checks[i].Check == doctor.HD11 {
			res.Checks[i].Entries = []doctor.Entry{{Status: doctor.Fail, Code: doctor.CodeSharedLocation, SessionUser: alexOSU}}
		}
	}
	r.runner.set(res)
	if p := r.gate.RunOnce(r.ctx); p.Host != "" || p.Refusal(alexOSU) != DetailSharedLocation {
		t.Fatalf("policy %+v", p)
	}
	wantEnded(t, recv(t, r.ctx, "session.ended", r.note.ended), ReasonHostPolicyViolation, DetailSharedLocation)
}

// TestEndKeptUntilTheNextReady: an end that cannot be reported stays among the live sessions and
// goes out on the next ready.
func TestEndKeptUntilTheNextReady(t *testing.T) {
	r := newPipeRig(t)
	r.open()
	if got := r.mgr.Live(); len(got) != 1 || got[0] != sessID {
		t.Fatalf("live %v", got)
	}
	r.note.down.Store(true)
	r.helper.exited(t, sessID, 0)
	until(t, "the end kept", func() bool {
		live := r.mgr.Live()
		return r.mgr.Active() == 0 && len(live) == 1 && live[0] == sessID
	})
	n := newNotes()
	r.mgr.Resend(r.ctx, n)
	wantEnded(t, recv(t, r.ctx, "resent session.ended", n.ended), ReasonProcessExited, "")
	if got := r.mgr.Live(); len(got) != 0 {
		t.Errorf("live after the resend %v", got)
	}
}

var _ control.Notifier = (*notes)(nil)
