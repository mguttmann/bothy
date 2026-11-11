// SPDX-License-Identifier: Apache-2.0

package install

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// fakeLaunchd is launchd as the plans meet it; TestReloadOverARunningLaunchdJob holds the same
// against the real one. bootout returns at once while the job still ends, print knows the label
// until the job is gone, and a bootstrap of a label that is still known fails with exit code 5.
type fakeLaunchd struct {
	fakeOps
	// ends is how long a job needs to end after bootout.
	ends time.Duration
	// jobs maps a service target ("gui/501/bothy-host") to the time it is gone; the zero time is
	// a loaded job.
	jobs map[string]time.Time
}

func newFakeLaunchd(ends time.Duration) *fakeLaunchd {
	return &fakeLaunchd{ends: ends, jobs: map[string]time.Time{},
		fakeOps: fakeOps{homes: map[string]string{"bothy-s-alex": "/home/bothy-s-alex", "bothy-s-sam": "/home/bothy-s-sam"}}}
}

func (l *fakeLaunchd) known(target string) bool {
	gone, ok := l.jobs[target]
	if ok && !gone.IsZero() && !time.Now().Before(gone) {
		delete(l.jobs, target)
		return false
	}
	return ok
}

func (l *fakeLaunchd) Run(ctx context.Context, argv []string) error {
	_ = l.fakeOps.Run(ctx, argv)
	if argv[0] != launchctl {
		return nil
	}
	switch argv[1] {
	case "bootout":
		if !l.known(argv[2]) {
			return errors.New("exit code 3")
		}
		l.jobs[argv[2]] = time.Now().Add(l.ends)
	case "print", "kickstart":
		if !l.known(argv[len(argv)-1]) {
			return errors.New("exit code 113")
		}
	case "bootstrap":
		target := argv[2] + "/" + strings.TrimSuffix(filepath.Base(argv[3]), ".plist")
		if l.known(target) {
			return errors.New("exit code 5")
		}
		l.jobs[target] = time.Time{}
	}
	return nil
}

// shortUnloadPause sets the pause between two looks of an until-gone step for the test.
func shortUnloadPause(t *testing.T, pause time.Duration) {
	t.Helper()
	old := unloadPause
	unloadPause = pause
	t.Cleanup(func() { unloadPause = old })
}

// TestInstallOverARunningService: "bothy-host install" over a running installation on macOS (the
// update route of the release binary) loads every service again. launchctl bootout returns while
// launchd still ends the job, and until the job is gone a bootstrap of its label fails with exit
// code 5: install ended there and left the service unloaded. Both plans, the LaunchAgent of the
// owner account mode and the LaunchDaemons, wait for the unload before they load.
func TestInstallOverARunningService(t *testing.T) {
	shortUnloadPause(t, time.Millisecond)
	owner, err := OwnerAccount(ownerTarget("darwin"))
	if err != nil {
		t.Fatal(err)
	}
	system, err := Install(target("darwin"), people)
	if err != nil {
		t.Fatal(err)
	}
	for name, steps := range map[string][]Step{"owner account": owner, "system": system} {
		l := newFakeLaunchd(5 * time.Millisecond)
		if err := Apply(context.Background(), l, steps); err != nil {
			t.Fatalf("%s: first install: %v", name, err)
		}
		first := len(l.jobs)
		if first == 0 {
			t.Fatalf("%s: the first install loaded no service", name)
		}
		if err := Apply(context.Background(), l, steps); err != nil {
			t.Fatalf("%s: install over the running installation: %v", name, err)
		}
		if len(l.jobs) != first {
			t.Errorf("%s: %d services loaded after the second install, %d after the first", name, len(l.jobs), first)
		}
		for target, gone := range l.jobs {
			if !gone.IsZero() {
				t.Errorf("%s: %s is not loaded after the second install", name, target)
			}
		}
	}
}

// TestUntilGoneGivesUp: a service that never unloads ends install after the bound with an error
// that names the step, and nothing after the step runs (no bootstrap over the loaded label).
func TestUntilGoneGivesUp(t *testing.T) {
	shortUnloadPause(t, time.Microsecond)
	steps := reload("gui/501", "bothy-host", "/tmp/bothy-host.plist")
	l := newFakeLaunchd(time.Hour)
	if err := Apply(context.Background(), l, steps); err != nil {
		t.Fatal(err)
	}
	l.log = nil
	err := Apply(context.Background(), l, steps)
	if !errors.Is(err, errNotGone) || err.Error() != "install: until-gone launchctl print: the service did not unload" {
		t.Fatalf("error %v", err)
	}
	if len(l.log) != 1+unloadLooks || l.log[0] != "run /bin/launchctl bootout gui/501/bothy-host" || l.log[len(l.log)-1] != "run /bin/launchctl print gui/501/bothy-host" {
		t.Errorf("%d steps ran, the last %q", len(l.log), l.log[len(l.log)-1])
	}
}

// TestUntilGoneEndsWithTheContext: the wait for an unload ends when the command is interrupted.
func TestUntilGoneEndsWithTheContext(t *testing.T) {
	steps := reload("gui/501", "bothy-host", "/tmp/bothy-host.plist")
	l := newFakeLaunchd(time.Hour)
	if err := Apply(context.Background(), l, steps); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if err := Apply(ctx, l, steps); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("error %v", err)
	}
}
