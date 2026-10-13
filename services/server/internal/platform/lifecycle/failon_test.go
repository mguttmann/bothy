// SPDX-License-Identifier: Apache-2.0

package lifecycle_test

import (
	"context"
	"errors"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/platform/lifecycle"
)

// fakeRuntime stands in for the job runtime of platform/jobs: Start begins its work, Stop ends
// it, and Failed delivers an error when the work ends without Stop (a lost River client).
type fakeRuntime struct {
	failed chan error
}

func newFakeRuntime() *fakeRuntime { return &fakeRuntime{failed: make(chan error, 1)} }

// die ends the work without Stop.
func (f *fakeRuntime) die() {
	f.failed <- errors.New("jobs: the River client stopped unexpectedly")
	close(f.failed)
}

// stop ends the work through Stop: Failed closes without a value.
func (f *fakeRuntime) stop(context.Context) error {
	close(f.failed)
	return nil
}

// TestFailed_UnexpectedStopEndsProcess: a runtime whose Failed channel delivers an error ends
// the process with exit code 1 and the error, after the stop steps ran (02 "Panics").
func TestFailed_UnexpectedStopEndsProcess(t *testing.T) {
	t.Parallel()
	h := newHarness(t, time.Minute)
	rt := newFakeRuntime()
	h.r.Add(&component{name: "jobs", rec: h.rec, start: func(context.Context) error {
		h.r.FailOn("jobs", rt.failed)
		return nil
	}})
	h.r.OnStop(lifecycle.StopStep{Name: "jobs", Phase: lifecycle.PhaseJobs, Run: func(ctx context.Context) error {
		h.rec.add("stop jobs")
		return nil
	}})
	rc, readyCh := ready(h.rec)
	h.r.Add(rc)
	done := h.runAsync(context.Background())
	waitCh(t, readyCh)
	rt.die()
	if code := wait(t, done); code != lifecycle.ExitFailure {
		t.Fatalf("exit = %d, want 1", code)
	}
	equal(t, h.rec.got(), []string{"start jobs", "start ready", "stop jobs"})
	if err := h.r.Err(); err == nil || !strings.Contains(err.Error(), "jobs: ") || !strings.Contains(err.Error(), "unexpectedly") {
		t.Fatalf("Err() = %v, want the runtime's error named after the component", err)
	}
}

// TestFailOnClosedWithoutValueIsQuiet: a channel closed by an orderly Stop is no failure.
func TestFailOnClosedWithoutValueIsQuiet(t *testing.T) {
	t.Parallel()
	h := newHarness(t, time.Minute)
	rt := newFakeRuntime()
	h.r.FailOn("jobs", rt.failed)
	h.r.OnStop(lifecycle.StopStep{Name: "jobs", Phase: lifecycle.PhaseJobs, Run: rt.stop})
	rc, readyCh := ready(h.rec)
	h.r.Add(rc)
	done := h.runAsync(context.Background())
	waitCh(t, readyCh)
	h.sigs <- syscall.SIGTERM
	if code := wait(t, done); code != lifecycle.ExitOK {
		t.Fatalf("exit = %d, want 0; Err = %v", code, h.r.Err())
	}
}
