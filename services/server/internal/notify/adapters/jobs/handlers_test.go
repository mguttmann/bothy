// SPDX-License-Identifier: Apache-2.0

package jobs

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/notify/app"
	"github.com/OWNER/bothy/services/server/internal/notify/domain"
	"github.com/OWNER/bothy/services/server/internal/platform/jobs"
)

// recordingUseCases records the calls of the handlers and answers with err.
type recordingUseCases struct {
	err   error
	sent  []app.PushJob
	flush []string
	syncs []int64
}

func (r *recordingUseCases) Send(_ context.Context, job app.PushJob) error {
	r.sent = append(r.sent, job)
	return r.err
}

func (r *recordingUseCases) Flush(_ context.Context, user, thread ids.ID, end time.Time) error {
	r.flush = append(r.flush, user.String()+"/"+thread.String()+"/"+end.Format(time.RFC3339Nano))
	return r.err
}

func (r *recordingUseCases) BadgeSync(_ context.Context, _ ids.ID, afterSeq int64) error {
	r.syncs = append(r.syncs, afterSeq)
	return r.err
}

// TestHandlers_RunTheUseCases: each handler maps its arguments onto its use case and returns the
// use case's error; the two markers do nothing.
func TestHandlers_RunTheUseCases(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	user, thread := ids.New(), ids.New()
	end := time.Date(2026, 10, 21, 9, 30, 0, 0, time.UTC)
	uc := &recordingUseCases{}
	job := app.PushJob{NotificationID: ids.New(), RegistrationID: ids.New(), RecipientID: user, ThreadID: thread,
		Kind: domain.KindMention, DueAt: end, Attempt: 2}
	args, err := pushArgs(job)
	if err != nil {
		t.Fatal(err)
	}
	if err := (sendHandler{uc}).Work(ctx, jobs.Job[PushSend]{Args: args}); err != nil {
		t.Fatalf("send: %v", err)
	}
	if len(uc.sent) != 1 || uc.sent[0].RecipientID != user || uc.sent[0].Kind != domain.KindMention || !uc.sent[0].DueAt.Equal(end) {
		t.Fatalf("sent %+v", uc.sent)
	}
	if err := (flushHandler{uc}).Work(ctx, jobs.Job[CoalesceFlush]{Args: CoalesceFlush{UserID: user, ThreadID: thread, EndUnixMicro: end.UnixMicro()}}); err != nil {
		t.Fatalf("flush: %v", err)
	}
	if want := user.String() + "/" + thread.String() + "/" + end.Format(time.RFC3339Nano); len(uc.flush) != 1 || uc.flush[0] != want {
		t.Fatalf("flush %v, want %s", uc.flush, want)
	}
	if err := (badgeHandler{uc}).Work(ctx, jobs.Job[BadgeSync]{Args: BadgeSync{UserID: user, AfterSeq: 42}}); err != nil {
		t.Fatalf("badge: %v", err)
	}
	if len(uc.syncs) != 1 || uc.syncs[0] != 42 {
		t.Fatalf("syncs %v", uc.syncs)
	}
	if err := (nothing[CoalesceWindow]{}).Work(ctx, jobs.Job[CoalesceWindow]{}); err != nil {
		t.Fatalf("window marker: %v", err)
	}

	failing := &recordingUseCases{err: errors.New("boom")}
	if err := (sendHandler{failing}).Work(ctx, jobs.Job[PushSend]{Args: args}); err == nil {
		t.Error("send swallowed the use case error")
	}
	if err := (flushHandler{failing}).Work(ctx, jobs.Job[CoalesceFlush]{}); err == nil {
		t.Error("flush swallowed the use case error")
	}
	if err := (badgeHandler{failing}).Work(ctx, jobs.Job[BadgeSync]{}); err == nil {
		t.Error("badge swallowed the use case error")
	}
}

// TestHandlers_RefuseWithoutUseCasesOrKind: a registry built without use cases fails each job
// with an internal error, and a stored kind code outside the closed set is refused.
func TestHandlers_RefuseWithoutUseCasesOrKind(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	for name, err := range map[string]error{
		"send":  (sendHandler{}).Work(ctx, jobs.Job[PushSend]{}),
		"flush": (flushHandler{}).Work(ctx, jobs.Job[CoalesceFlush]{}),
		"badge": (badgeHandler{}).Work(ctx, jobs.Job[BadgeSync]{}),
	} {
		if errs.KindOf(err) != errs.KindInternal {
			t.Errorf("%s without use cases = %v, want kind internal", name, err)
		}
	}
	if err := (sendHandler{&recordingUseCases{}}).Work(ctx, jobs.Job[PushSend]{Args: PushSend{KindCode: 99}}); err == nil {
		t.Error("an unknown kind code was accepted")
	}
	if _, err := pushArgs(app.PushJob{Kind: "unknown"}); err == nil {
		t.Error("an unknown kind was stored")
	}
}

// TestScheduler_UnboundIsInternal: before Bind every method of the Scheduler is an internal
// error and inserts nothing.
func TestScheduler_UnboundIsInternal(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s := NewScheduler(nil)
	u, th := ids.New(), ids.New()
	now := time.Now()
	_, _, e1 := s.WindowEnd(ctx, u, th)
	_, _, e2 := s.OpenWindow(ctx, u, th, now)
	_, e3 := s.Collect(ctx, u, th, now)
	e4 := s.Send(ctx, app.PushJob{Kind: domain.KindMessage}, now)
	_, _, e5 := s.BadgeSyncPending(ctx, u)
	_, _, e6 := s.HoldEnd(ctx, u)
	e7 := s.ScheduleBadgeSync(ctx, u, 1, now)
	e8 := s.Hold(ctx, u, now)
	for i, err := range []error{e1, e2, e3, e4, e5, e6, e7, e8} {
		if errs.KindOf(err) != errs.KindInternal {
			t.Errorf("method %d unbound = %v, want kind internal", i+1, err)
		}
	}
}

// TestRegister_RefusesASecondRegistration: Register on a registry that holds the kinds already
// returns the error of jobs.Register.
func TestRegister_RefusesASecondRegistration(t *testing.T) {
	t.Parallel()
	reg := jobs.NewRegistry()
	if err := Register(reg, &recordingUseCases{}); err != nil {
		t.Fatal(err)
	}
	if err := Register(reg, &recordingUseCases{}); err == nil {
		t.Fatal("the kinds of notify registered twice")
	}
}
