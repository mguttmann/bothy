// SPDX-License-Identifier: Apache-2.0

package wire

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/retention"
	"github.com/OWNER/bothy/services/server/internal/platform/jobs"
)

// countingParticipant removes left rows in batches and records the cutoffs it saw.
type countingParticipant struct {
	class retention.Class
	left  int
	err   error
}

func (c *countingParticipant) Class() retention.Class { return c.class }
func (c *countingParticipant) Tables() []string       { return []string{"example_" + string(c.class)} }
func (c *countingParticipant) Purge(_ context.Context, _ time.Time, batch int) (int, bool, error) {
	if c.err != nil {
		return 0, false, c.err
	}
	n := min(batch, c.left)
	c.left -= n
	return n, c.left > 0, nil
}

// TestRetentionJobs (M7-05): the runner refuses a missing registry and settings that are not
// positive, runs only the participants of the asked class until they are done, and the handler of
// a class job runs it; a handler without a runner fails.
func TestRetentionJobs(t *testing.T) {
	t.Parallel()
	cfg := RetentionConfig{BatchSize: 2, JobInterval: time.Hour}
	if _, err := NewRetentionJobs(nil, cfg, nil); err == nil {
		t.Error("a nil registry was accepted")
	}
	if _, err := NewRetentionJobs(retention.NewRegistry(), RetentionConfig{}, nil); err == nil {
		t.Error("zero batch size and interval were accepted")
	}
	reg := retention.NewRegistry()
	security := &countingParticipant{class: retention.ClassSecurity, left: 5}
	cost := &countingParticipant{class: retention.ClassCost, left: 3}
	for _, p := range []*countingParticipant{security, cost} {
		if err := reg.Register(p, 24*time.Hour); err != nil {
			t.Fatal(err)
		}
	}
	r, err := NewRetentionJobs(reg, cfg, clock.NewFake(time.Date(2026, 10, 24, 12, 0, 0, 0, time.UTC)))
	if err != nil {
		t.Fatal(err)
	}
	res, err := r.Run(context.Background(), retention.ClassSecurity)
	if err != nil || res.Deleted != 5 || security.left != 0 || cost.left != 3 {
		t.Fatalf("Run(security) = %+v, %v; security left %d, cost left %d", res, err, security.left, cost.left)
	}
	if err := (retentionHandler[retention.CostJob]{jobs: r, class: retention.ClassCost}).Work(context.Background(), jobs.Job[retention.CostJob]{}); err != nil || cost.left != 0 {
		t.Fatalf("Work(cost): %v, left %d", err, cost.left)
	}
	boom := errors.New("boom")
	failing := retention.NewRegistry()
	if err := failing.Register(&countingParticipant{class: retention.ClassAudit, err: boom}, time.Hour); err != nil {
		t.Fatal(err)
	}
	rf, err := NewRetentionJobs(failing, cfg, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := (retentionHandler[retention.AuditJob]{jobs: rf, class: retention.ClassAudit}).Work(context.Background(), jobs.Job[retention.AuditJob]{}); !errors.Is(err, boom) {
		t.Fatalf("Work of a failing participant: %v", err)
	}
	if err := (retentionHandler[retention.AuditJob]{class: retention.ClassAudit}).Work(context.Background(), jobs.Job[retention.AuditJob]{}); err == nil {
		t.Error("a handler without a runner worked")
	}
	if err := registerRetentionJobs(jobs.NewRegistry(), r, 0); err == nil {
		t.Error("a zero interval was accepted")
	}
	jr := jobs.NewRegistry()
	if err := registerRetentionJobs(jr, r, time.Hour); err != nil {
		t.Fatal(err)
	}
	for _, c := range retention.JobClasses() {
		if q, ok := jr.Queue(retention.JobKind(c)); !ok || q != jobs.QueueMaintenance {
			t.Errorf("%s: queue %q, %v", retention.JobKind(c), q, ok)
		}
	}
	if n := len(jr.Periodic()); n != len(retention.JobClasses()) {
		t.Errorf("%d periodic jobs, want one per class", n)
	}
}

// TestRetentionContentPeriod: none is no period, a duration of at least one day is the period,
// and every other text is refused.
func TestRetentionContentPeriod(t *testing.T) {
	t.Parallel()
	if d := (RetentionConfig{Content: "none"}).ContentPeriod(); d != 0 {
		t.Errorf("none: %v", d)
	}
	if d := (RetentionConfig{Content: "730d"}).ContentPeriod(); d != 730*24*time.Hour {
		t.Errorf("730d: %v", d)
	}
	for _, bad := range []string{"", "12m", "forever", "-1d"} {
		if _, err := contentPeriod(bad); err == nil {
			t.Errorf("%q accepted", bad)
		}
	}
}
