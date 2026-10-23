// SPDX-License-Identifier: Apache-2.0

// Package jobs is the job adapter of the notify module (docs/spec/02-architecture.md "Job catalogue",
// brief M3-19 design notes 9, 11 and 16): the arguments and handlers of notify.push_send,
// notify.coalesce_window, notify.coalesce_flush, notify.badge_sync and notify.badge_sync_hold on
// the queue delivery, the Scheduler of notify/app over platform/jobs, and the span push.send
// around every provider request.
package jobs

import (
	"context"
	stdsync "sync"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/notify/app"
	"github.com/OWNER/bothy/services/server/internal/notify/domain"
	"github.com/OWNER/bothy/services/server/internal/platform/jobs"
	"github.com/OWNER/bothy/services/server/internal/platform/otel/attr"
)

// instrumentationName is the scope of the span push.send.
const instrumentationName = "github.com/OWNER/bothy/services/server/internal/notify/adapters/jobs"

// SpanName is the CLIENT span of one provider request (docs/observability/signals.md 5).
const SpanName = "push.send"

// The job kinds of notify (02 12.3).
const (
	// KindPushSend sends one hint to one registration.
	KindPushSend = "notify.push_send"
	// KindCoalesceWindow marks an open bundling window of (recipient, thread).
	KindCoalesceWindow = "notify.coalesce_window"
	// KindCoalesceFlush flushes the collected hints of a window at its end.
	KindCoalesceFlush = "notify.coalesce_flush"
	// KindBadgeSync sends the badge sync of a person.
	KindBadgeSync = "notify.badge_sync"
	// KindBadgeSyncHold marks the interval after a badge sync.
	KindBadgeSyncHold = "notify.badge_sync_hold"
)

// PushSend are the arguments of notify.push_send (design note 11 a): only IDs and int64. KindCode
// is domain.KindCode (1 to 8 in the order of NotificationKind, 9 the badge sync); the brief's
// field name Kind would clash with the method Kind of jobs.Args. Quiet is 0 or 1, DueAt t0 of 13 9
// in Unix microseconds. Absent IDs are left out of the stored JSON (a zero ID is no UUIDv7).
type PushSend struct {
	NotificationID ids.ID   `json:"notification_id"`
	RegistrationID ids.ID   `json:"registration_id"`
	RecipientID    ids.ID   `json:"recipient_id"`
	ThreadID       ids.ID   `json:"thread_id,omitzero"`
	MessageID      ids.ID   `json:"message_id,omitzero"`
	TargetID       ids.ID   `json:"target_id,omitzero"`
	Cleared        []ids.ID `json:"cleared,omitempty"`
	KindCode       int64    `json:"kind"`
	Quiet          int64    `json:"quiet"`
	Count          int64    `json:"count"`
	DueAtUnixMicro int64    `json:"due_at"`
	Attempt        int64    `json:"attempt"`
}

// Kind implements jobs.Args.
func (PushSend) Kind() string { return KindPushSend }

// CoalesceWindow are the arguments of notify.coalesce_window: one marker per (recipient, thread)
// and window. EndUnixMicro is the window end; it is part of the arguments (beyond the brief's two
// IDs, docs/DECISIONS.md D-M3-19-7), so the flush at the end of a window opens the next window
// even while River still runs the marker of the ending one; the open window is the latest pending
// marker of (recipient, thread).
type CoalesceWindow struct {
	UserID       ids.ID `json:"user_id"`
	ThreadID     ids.ID `json:"thread_id"`
	EndUnixMicro int64  `json:"end"`
}

// Kind implements jobs.Args.
func (CoalesceWindow) Kind() string { return KindCoalesceWindow }

// CoalesceFlush are the arguments of notify.coalesce_flush. EndUnixMicro is the window end; it is
// part of the arguments (beyond the brief's two IDs, docs/DECISIONS.md D-M3-19-7), so the flush of
// the next window is a different job even while the flush of the previous window still runs, and
// the flush hint has the window end as t0 without another read.
type CoalesceFlush struct {
	UserID       ids.ID `json:"user_id"`
	ThreadID     ids.ID `json:"thread_id"`
	EndUnixMicro int64  `json:"end"`
}

// Kind implements jobs.Args.
func (CoalesceFlush) Kind() string { return KindCoalesceFlush }

// BadgeSync are the arguments of notify.badge_sync: unique per person and scheduled time
// (AtUnixMicro, beyond the brief's two fields, D-M3-19-15), so a follow-up sync inserts while
// River still runs the previous one; the consumer keeps one waiting sync per person by reading
// the latest pending one. AfterSeq is the seq before the read that lowered the badge.
type BadgeSync struct {
	UserID      ids.ID `json:"user_id" river:"unique"`
	AfterSeq    int64  `json:"after_seq"`
	AtUnixMicro int64  `json:"at" river:"unique"`
}

// Kind implements jobs.Args.
func (BadgeSync) Kind() string { return KindBadgeSync }

// BadgeSyncHold are the arguments of notify.badge_sync_hold. EndUnixMicro is the end of the
// interval (D-M3-19-15), so a deferred sync inserts the next hold even while River still runs the
// hold that ends at the same moment; the running interval is the latest pending hold of the person.
type BadgeSyncHold struct {
	UserID       ids.ID `json:"user_id"`
	EndUnixMicro int64  `json:"end"`
}

// Kind implements jobs.Args.
func (BadgeSyncHold) Kind() string { return KindBadgeSyncHold }

// Scheduler implements app.Scheduler over the inserter of the job runtime; every method runs in
// the ambient unit of work of ctx. internal/wire builds the push path before the job runtime
// exists, so the inserter is bound later (Bind); before, every method is an internal error.
type Scheduler struct {
	mu  stdsync.RWMutex
	ins jobs.Inserter
}

// NewScheduler returns the Scheduler over ins; nil leaves it unbound until Bind.
func NewScheduler(ins jobs.Inserter) *Scheduler { return &Scheduler{ins: ins} }

// Bind sets the inserter of the job runtime of the process.
func (s *Scheduler) Bind(ins jobs.Inserter) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ins = ins
}

// inserter returns the bound inserter; an internal error before Bind.
func (s *Scheduler) inserter() (jobs.Inserter, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.ins == nil {
		return nil, errs.New(errs.CodeInternalError, "notify: the scheduler has no job runtime")
	}
	return s.ins, nil
}

// Compile-time check.
var _ app.Scheduler = (*Scheduler)(nil)

// WindowEnd implements app.Scheduler.
func (s *Scheduler) WindowEnd(ctx context.Context, user, thread ids.ID) (time.Time, bool, error) {
	ins, err := s.inserter()
	if err != nil {
		return time.Time{}, false, err
	}
	return ins.LatestPendingTx(ctx, CoalesceWindow{UserID: user, ThreadID: thread}, "user_id", "thread_id")
}

// OpenWindow implements app.Scheduler.
func (s *Scheduler) OpenWindow(ctx context.Context, user, thread ids.ID, end time.Time) (bool, time.Time, error) {
	ins, err := s.inserter()
	if err != nil {
		return false, time.Time{}, err
	}
	res, err := ins.InsertUniqueTx(ctx, CoalesceWindow{UserID: user, ThreadID: thread, EndUnixMicro: end.UnixMicro()},
		&jobs.InsertOptions{ScheduledAt: end})
	return res.Inserted, res.ScheduledAt, err
}

// Collect implements app.Scheduler.
func (s *Scheduler) Collect(ctx context.Context, user, thread ids.ID, end time.Time) (bool, error) {
	ins, err := s.inserter()
	if err != nil {
		return false, err
	}
	res, err := ins.InsertUniqueTx(ctx, CoalesceFlush{UserID: user, ThreadID: thread, EndUnixMicro: end.UnixMicro()},
		&jobs.InsertOptions{ScheduledAt: end})
	return res.Inserted, err
}

// Send implements app.Scheduler.
func (s *Scheduler) Send(ctx context.Context, job app.PushJob, at time.Time) error {
	ins, err := s.inserter()
	if err != nil {
		return err
	}
	args, err := pushArgs(job)
	if err != nil {
		return err
	}
	return ins.InsertTx(ctx, args, &jobs.InsertOptions{ScheduledAt: at})
}

// BadgeSyncPending implements app.Scheduler.
func (s *Scheduler) BadgeSyncPending(ctx context.Context, user ids.ID) (time.Time, bool, error) {
	ins, err := s.inserter()
	if err != nil {
		return time.Time{}, false, err
	}
	return ins.LatestPendingTx(ctx, BadgeSync{UserID: user}, "user_id")
}

// HoldEnd implements app.Scheduler.
func (s *Scheduler) HoldEnd(ctx context.Context, user ids.ID) (time.Time, bool, error) {
	ins, err := s.inserter()
	if err != nil {
		return time.Time{}, false, err
	}
	return ins.LatestPendingTx(ctx, BadgeSyncHold{UserID: user}, "user_id")
}

// ScheduleBadgeSync implements app.Scheduler.
func (s *Scheduler) ScheduleBadgeSync(ctx context.Context, user ids.ID, afterSeq int64, at time.Time) error {
	ins, err := s.inserter()
	if err != nil {
		return err
	}
	_, err = ins.InsertUniqueTx(ctx, BadgeSync{UserID: user, AfterSeq: afterSeq, AtUnixMicro: at.UnixMicro()},
		&jobs.InsertOptions{ScheduledAt: at})
	return err
}

// Hold implements app.Scheduler.
func (s *Scheduler) Hold(ctx context.Context, user ids.ID, at time.Time) error {
	ins, err := s.inserter()
	if err != nil {
		return err
	}
	_, err = ins.InsertUniqueTx(ctx, BadgeSyncHold{UserID: user, EndUnixMicro: at.UnixMicro()}, &jobs.InsertOptions{ScheduledAt: at})
	return err
}

// pushArgs maps a PushJob onto the stored arguments.
func pushArgs(j app.PushJob) (PushSend, error) {
	code, err := domain.KindCode(j.Kind)
	if err != nil {
		return PushSend{}, err
	}
	var quiet int64
	if j.Quiet {
		quiet = 1
	}
	return PushSend{
		NotificationID: j.NotificationID, RegistrationID: j.RegistrationID, RecipientID: j.RecipientID, ThreadID: j.ThreadID,
		MessageID: j.MessageID, TargetID: j.TargetID, Cleared: j.Cleared, KindCode: code, Quiet: quiet, Count: j.Count,
		DueAtUnixMicro: j.DueAt.UnixMicro(), Attempt: j.Attempt,
	}, nil
}

// pushJob maps stored arguments back onto a PushJob.
func pushJob(a PushSend) (app.PushJob, error) {
	kind, err := domain.KindFromCode(a.KindCode)
	if err != nil {
		return app.PushJob{}, err
	}
	return app.PushJob{
		NotificationID: a.NotificationID, RegistrationID: a.RegistrationID, RecipientID: a.RecipientID, ThreadID: a.ThreadID,
		MessageID: a.MessageID, TargetID: a.TargetID, Cleared: a.Cleared, Kind: kind, Quiet: a.Quiet == 1, Count: a.Count,
		DueAt: time.UnixMicro(a.DueAtUnixMicro).UTC(), Attempt: a.Attempt,
	}, nil
}

// UseCases are the use cases the handlers run (implemented by *app.Push).
type UseCases interface {
	Send(ctx context.Context, job app.PushJob) error
	Flush(ctx context.Context, user, thread ids.ID, end time.Time) error
	BadgeSync(ctx context.Context, user ids.ID, afterSeq int64) error
}

// Register registers the five job kinds of notify on the queue delivery (02 12.2: they are part
// of push, so they keep running in maintenance mode). uc may be nil where no job runs (a
// registry of a test); a job of such a registry fails with an internal error.
//
// Errors: those of jobs.Register (a kind registered twice, a bad argument type).
func Register(reg *jobs.Registry, uc UseCases) error {
	o := jobs.HandlerOptions{Queue: jobs.QueueDelivery}
	if err := jobs.Register[PushSend](reg, sendHandler{uc}, o); err != nil {
		return err
	}
	if err := jobs.Register[CoalesceWindow](reg, nothing[CoalesceWindow]{}, o); err != nil {
		return err
	}
	if err := jobs.Register[CoalesceFlush](reg, flushHandler{uc}, o); err != nil {
		return err
	}
	if err := jobs.Register[BadgeSync](reg, badgeHandler{uc}, o); err != nil {
		return err
	}
	return jobs.Register[BadgeSyncHold](reg, nothing[BadgeSyncHold]{}, o)
}

// errNoUseCases is the error of a job of a registry built without the use cases.
var errNoUseCases = errs.New(errs.CodeInternalError, "notify: the jobs of notify have no use cases")

// sendHandler runs notify.push_send.
type sendHandler struct{ uc UseCases }

// Work implements jobs.Handler.
func (h sendHandler) Work(ctx context.Context, job jobs.Job[PushSend]) error {
	if h.uc == nil {
		return errNoUseCases
	}
	j, err := pushJob(job.Args)
	if err != nil {
		return err
	}
	return h.uc.Send(ctx, j)
}

// flushHandler runs notify.coalesce_flush.
type flushHandler struct{ uc UseCases }

// Work implements jobs.Handler.
func (h flushHandler) Work(ctx context.Context, job jobs.Job[CoalesceFlush]) error {
	if h.uc == nil {
		return errNoUseCases
	}
	return h.uc.Flush(ctx, job.Args.UserID, job.Args.ThreadID, time.UnixMicro(job.Args.EndUnixMicro).UTC())
}

// badgeHandler runs notify.badge_sync.
type badgeHandler struct{ uc UseCases }

// Work implements jobs.Handler.
func (h badgeHandler) Work(ctx context.Context, job jobs.Job[BadgeSync]) error {
	if h.uc == nil {
		return errNoUseCases
	}
	return h.uc.BadgeSync(ctx, job.Args.UserID, job.Args.AfterSeq)
}

// nothing is the handler of the markers notify.coalesce_window and notify.badge_sync_hold: their
// existence is their meaning, so the run does nothing.
type nothing[A jobs.Args] struct{}

// Work implements jobs.Handler.
func (nothing[A]) Work(context.Context, jobs.Job[A]) error { return nil }

// Traced wraps each provider of ps with the span push.send (kind CLIENT) carrying
// bothy.push.provider and, after the answer, bothy.outcome; tp nil means the global provider.
func Traced(ps []app.PushProvider, tp trace.TracerProvider) []app.PushProvider {
	if tp == nil {
		tp = otel.GetTracerProvider()
	}
	out := make([]app.PushProvider, 0, len(ps))
	for _, p := range ps {
		out = append(out, tracedProvider{inner: p, tracer: tp.Tracer(instrumentationName)})
	}
	return out
}

// tracedProvider is one provider inside the span push.send.
type tracedProvider struct {
	inner  app.PushProvider
	tracer trace.Tracer
}

// Name implements app.PushProvider.
func (t tracedProvider) Name() string { return t.inner.Name() }

// Send implements app.PushProvider.
func (t tracedProvider) Send(ctx context.Context, reg app.Registration, n app.Notification) (domain.SendResult, error) {
	ctx, span := t.tracer.Start(ctx, SpanName, trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(attribute.String(attr.PushProvider, t.inner.Name())))
	defer span.End()
	res, err := t.inner.Send(ctx, reg, n)
	if err != nil {
		span.SetStatus(codes.Error, "")
		return res, err
	}
	span.SetAttributes(attribute.String(attr.Outcome, outcomeOf(res.Class)))
	return res, nil
}

// outcomeOf maps a result class onto bothy.outcome as domain.ApplyResult does.
func outcomeOf(c domain.ResultClass) string {
	return domain.ApplyResult(domain.RegistrationState{}, domain.SendResult{Class: c, DisableReason: domain.DisableTokenInvalid}, time.Time{}, 1<<30).Outcome
}
