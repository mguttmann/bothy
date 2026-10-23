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

func TestTimingBound(t *testing.T) {
	want := 20 * time.Millisecond
	if testkit.RaceEnabled {
		want *= 5
	}
	if got := testkit.TimingBound(20 * time.Millisecond); got != want {
		t.Fatalf("TimingBound(20ms) = %v, want %v (race detector %v)", got, want, testkit.RaceEnabled)
	}
}

// TestPatientGatewayPutsEveryWindowBeyondPatience: with the windows of PatientGateway a
// connection that sends no heartbeats, or no identify, is not closed before Patience has passed,
// whatever the heartbeat interval; the miss limit stays within the bounds of its key (0 to 100).
func TestPatientGatewayPutsEveryWindowBeyondPatience(t *testing.T) {
	t.Parallel()
	for _, interval := range []time.Duration{2 * time.Second, 7 * time.Second, 30 * time.Second, testkit.Patience} {
		n := testkit.HeartbeatMissLimit(interval)
		if deadline := interval * time.Duration(n+1); deadline < testkit.Patience || n < 0 || n > 100 {
			t.Errorf("interval %s: miss limit %d, heartbeat deadline %s; want one beyond Patience", interval, n, deadline)
		}
		w := testkit.PatientGateway(interval)
		want := map[string]string{
			"sync.heartbeat_interval":   fmt.Sprintf("%dms", interval.Milliseconds()),
			"sync.heartbeat_miss_limit": fmt.Sprint(n),
			"sync.identify_timeout":     testkit.PatienceSetting,
		}
		if fmt.Sprint(w) != fmt.Sprint(want) {
			t.Errorf("interval %s: %v, want %v", interval, w, want)
		}
	}
	defer func() {
		if recover() == nil {
			t.Error("an interval that needs a miss limit above 100 was accepted")
		}
	}()
	testkit.HeartbeatMissLimit(time.Second)
}

// TestPatientDatabasePutsEveryDeadlineAtPatience: the database deadlines of PatientDatabase are
// Patience each.
func TestPatientDatabasePutsEveryDeadlineAtPatience(t *testing.T) {
	t.Parallel()
	want := map[string]string{
		"database.acquire_timeout":   testkit.PatienceSetting,
		"database.statement_timeout": testkit.PatienceSetting,
		"database.lock_timeout":      testkit.PatienceSetting,
	}
	if got := testkit.PatientDatabase(); fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("PatientDatabase() = %v, want %v", got, want)
	}
}
