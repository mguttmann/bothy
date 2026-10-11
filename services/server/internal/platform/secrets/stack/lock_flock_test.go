// SPDX-License-Identifier: Apache-2.0

//go:build android || darwin || dragonfly || freebsd || illumos || ios || linux || netbsd || openbsd

package stack_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/platform/secrets/stack"
)

const busyMessage = "another secrets init is running on "

// TestInitRefusesWhileTargetIsLocked: a second run must not remove the temp files of a run that
// is still writing, nor generate a value over one the other run has just written; it stops with
// a runtime error before it reads or writes anything.
func TestInitRefusesWhileTargetIsLocked(t *testing.T) {
	t.Parallel()
	target, source := dirs(t)
	mustInit(t, opts(target, source))
	remove(t, target, "_store/token_pepper")
	put(t, target, "_store/.bothy-tmp-running", "EXAMPLE-partial", 0o600)
	before := snapshot(t, target)

	held, err := os.Open(filepath.Clean(target))
	if err != nil {
		t.Fatal(err)
	}
	if err := syscall.Flock(int(held.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		t.Fatal(err)
	}
	_, err = stack.Init(t.Context(), opts(target, source))
	if err == nil {
		t.Fatal("Init ran while another run held the target")
	}
	if !strings.Contains(err.Error(), busyMessage+target) || errors.Is(err, stack.ErrUsage) {
		t.Errorf("error %q does not report the other run as a runtime error", err)
	}
	diffSnapshots(t, before, snapshot(t, target))

	if err := held.Close(); err != nil {
		t.Fatal(err)
	}
	r := mustInit(t, opts(target, source))
	if got := statuses(r)["token_pepper"]; !strings.HasPrefix(got, "generated ") {
		t.Errorf("token_pepper after the lock was released: %q, want generated", got)
	}
}

// TestInitConcurrentRunsKeepOneValue: runs started together on the same target either finish or
// stop because another run holds it; at least one finishes, and every service copy of a generated
// secret equals the one value in _store.
func TestInitConcurrentRunsKeepOneValue(t *testing.T) {
	t.Parallel()
	for _, fresh := range []bool{true, false} {
		name := "existing target missing a secret"
		if fresh {
			name = "fresh target"
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			target, source := dirs(t)
			if !fresh {
				mustInit(t, opts(target, source))
				for _, rel := range []string{"_store/token_pepper", "server/token_pepper"} {
					remove(t, target, rel)
				}
			}
			const runs = 8
			var (
				wg    sync.WaitGroup
				start = make(chan struct{})
				errs  = make([]error, runs)
			)
			for i := range runs {
				wg.Go(func() {
					<-start
					_, errs[i] = stack.Init(t.Context(), opts(target, source))
				})
			}
			close(start)
			wg.Wait()
			done := 0
			for _, err := range errs {
				switch {
				case err == nil:
					done++
				case !strings.Contains(err.Error(), busyMessage):
					t.Errorf("concurrent run failed with %v, want success or %q", err, busyMessage)
				}
			}
			if done == 0 {
				t.Fatal("no concurrent run finished")
			}
			t.Logf("%d of %d concurrent runs finished, the others found the target held", done, runs)
			for service, names := range freshServiceFiles {
				for _, name := range names {
					if got, want := read(t, target, service+"/"+name), read(t, target, "_store/"+name); got != want {
						t.Errorf("%s/%s differs from _store/%s", service, name, name)
					}
				}
			}
			for _, rel := range files(t, filepath.Join(target, "_store")) {
				if strings.Contains(rel, ".bothy-tmp-") {
					t.Errorf("temp file %s left behind", rel)
				}
			}
		})
	}
}
