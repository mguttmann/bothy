// SPDX-License-Identifier: Apache-2.0

package testkit_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/platform/testkit"
)

// recorder is a testing.TB that records Fatalf and Skip instead of ending the goroutine.
type recorder struct {
	testing.TB
	failed  string
	skipped string
}

func (r *recorder) Helper() {}

func (r *recorder) Fatalf(format string, args ...any) { r.failed = fmt.Sprintf(format, args...) }

func (r *recorder) Skip(args ...any) { r.skipped = fmt.Sprint(args...) }

func TestPatienceStaysBelowTheBinaryTimeLimit(t *testing.T) {
	t.Parallel()
	// quality.test.go_timeout is 20m; a failing wait must report itself well before that.
	if testkit.Patience < time.Minute || testkit.Patience > 5*time.Minute {
		t.Fatalf("Patience = %s, want between one and five minutes", testkit.Patience)
	}
	if d, err := time.ParseDuration(testkit.PatienceSetting); err != nil || d != testkit.Patience {
		t.Fatalf("PatienceSetting %q = %v, %v; want Patience", testkit.PatienceSetting, d, err)
	}
	if d := time.Until(testkit.Deadline()); d < testkit.Patience-time.Second || d > testkit.Patience {
		t.Fatalf("Deadline is %s away, want Patience", d)
	}
}

func TestUntilReturnsOnceTheConditionHolds(t *testing.T) {
	t.Parallel()
	var calls atomic.Int32
	testkit.Until(t, "the third check", func() bool { return calls.Add(1) == 3 })
	if calls.Load() != 3 {
		t.Fatalf("cond ran %d times, want 3", calls.Load())
	}
}

func TestUntilFailsWithWhatAfterThePatience(t *testing.T) {
	t.Parallel()
	r := &recorder{TB: t}
	start := time.Now()
	testkit.UntilWithin(r, 50*time.Millisecond, "the impossible", func() bool { return false })
	if !strings.Contains(r.failed, "the impossible did not happen within 50ms") {
		t.Fatalf("failure %q", r.failed)
	}
	if time.Since(start) < 50*time.Millisecond {
		t.Fatal("Until gave up before its patience")
	}
}

func TestReceiveReturnsTheValue(t *testing.T) {
	t.Parallel()
	ch := make(chan int, 1)
	ch <- 7
	if got := testkit.Receive(t, "the value", ch); got != 7 {
		t.Fatalf("Receive = %d, want 7", got)
	}
}

func TestReceiveFailsWithWhatAfterThePatience(t *testing.T) {
	t.Parallel()
	r := &recorder{TB: t}
	if got := testkit.ReceiveWithin(r, 20*time.Millisecond, "the reply", make(chan string)); got != "" {
		t.Fatalf("Receive = %q, want the zero value", got)
	}
	if !strings.Contains(r.failed, "the reply did not arrive within 20ms") {
		t.Fatalf("failure %q", r.failed)
	}
}

func TestContextEndsWithTheTest(t *testing.T) {
	t.Parallel()
	var ctx context.Context
	t.Run("inner", func(t *testing.T) {
		ctx = testkit.Context(t)
		deadline, ok := ctx.Deadline()
		if !ok || time.Until(deadline) > testkit.Patience {
			t.Fatalf("deadline %v, %v; want one within Patience", deadline, ok)
		}
		if ctx.Err() != nil {
			t.Fatal(ctx.Err())
		}
	})
	if !errors.Is(ctx.Err(), context.Canceled) {
		t.Fatalf("after the test: %v, want canceled", ctx.Err())
	}
}

func TestIntegrationSkipsOnlyUnderShort(t *testing.T) {
	t.Parallel()
	r := &recorder{TB: t}
	testkit.Integration(r)
	if testing.Short() != (r.skipped != "") {
		t.Fatalf("short %v, skipped %q", testing.Short(), r.skipped)
	}
}
