// SPDX-License-Identifier: Apache-2.0

// Package jobs is the adapter of the module privacy to platform/jobs (docs/spec/02-architecture.md
// 12.3, brief M7-06): the jobs privacy.export and privacy.erase on the queue privacy, inserted in
// the unit of work of the approval, and the periodic jobs privacy.export_expiry and
// privacy.request_reminder on the queue maintenance.
//
// Example:
//
//	err := jobs.Register(reg, svc, interval, batch)
//	q := jobs.NewQueue() // app.Jobs
//	q.Bind(inserter)
package jobs

import (
	"context"
	"errors"
	"sync/atomic"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/jobs"
	"github.com/OWNER/bothy/services/server/internal/privacy/app"
)

// The kinds of the jobs of the module.
const (
	KindExport   = "privacy.export"
	KindErase    = "privacy.erase"
	KindExpiry   = "privacy.export_expiry"
	KindReminder = "privacy.request_reminder"
)

// MaxAttempts is how often privacy.export and privacy.erase run before the request is marked
// failed; River's backoff spaces the attempts.
const MaxAttempts = 10

// codeFailed is the error code a request records after the last attempt (no text, no content).
const codeFailed = "job_failed"

// Export are the arguments of privacy.export: the request only (02 "Rules for jobs").
type Export struct {
	Request ids.ID `json:"request"`
}

// Kind returns KindExport.
func (Export) Kind() string { return KindExport }

// Erase are the arguments of privacy.erase.
type Erase struct {
	Request ids.ID `json:"request"`
}

// Kind returns KindErase.
func (Erase) Kind() string { return KindErase }

// Expiry are the arguments of privacy.export_expiry; none, every run reads the clock.
type Expiry struct{}

// Kind returns KindExpiry.
func (Expiry) Kind() string { return KindExpiry }

// Reminder are the arguments of privacy.request_reminder; none.
type Reminder struct{}

// Kind returns KindReminder.
func (Reminder) Kind() string { return KindReminder }

// UseCases are the use cases the jobs run (app.Service).
type UseCases interface {
	// RunExport and RunErase run one request.
	RunExport(ctx context.Context, id ids.ID) error
	RunErase(ctx context.Context, id ids.ID) error
	// Fail ends a running request as failed with an error code.
	Fail(ctx context.Context, id ids.ID, code string) error
	// ExpirePackages and Remind run one round in batches.
	ExpirePackages(ctx context.Context, batch int) (int, error)
	Remind(ctx context.Context, batch int) (int, error)
}

// errNoUseCases is returned by a handler in a process without the use cases.
var errNoUseCases = errors.New("privacy: the job runs without its use cases")

// requestHandler runs privacy.export or privacy.erase.
type requestHandler[A jobs.Args] struct {
	uc  UseCases
	run func(UseCases, context.Context, ids.ID) error
	id  func(A) ids.ID
}

// Work runs the request; after the last failed attempt the request is marked failed, so it
// never stays running.
func (h requestHandler[A]) Work(ctx context.Context, job jobs.Job[A]) error {
	if h.uc == nil {
		return errNoUseCases
	}
	err := h.run(h.uc, ctx, h.id(job.Args))
	if err != nil && job.Attempt >= MaxAttempts {
		return errors.Join(err, h.uc.Fail(ctx, h.id(job.Args), codeFailed))
	}
	return err
}

// roundHandler runs a periodic round.
type roundHandler[A jobs.Args] struct {
	uc    UseCases
	batch int
	run   func(UseCases, context.Context, int) (int, error)
}

// Work runs one round; a retry is harmless.
func (h roundHandler[A]) Work(ctx context.Context, _ jobs.Job[A]) error {
	if h.uc == nil {
		return errNoUseCases
	}
	_, err := h.run(h.uc, ctx, h.batch)
	return err
}

// Register adds privacy.export and privacy.erase on the queue privacy and the periodic
// privacy.export_expiry and privacy.request_reminder on the queue maintenance every interval
// (retention.job_interval) in batches of batch requests (retention.batch_size). uc may be nil
// where no job runs (tests of the registration). The error is the first rejected registration.
func Register(reg *jobs.Registry, uc UseCases, interval time.Duration, batch int) error {
	if interval <= 0 || batch < 1 {
		return errors.New("privacy: retention.job_interval and retention.batch_size must be positive")
	}
	// An export or a deletion bounds itself through its steps; River sets no timeout of its own.
	opts := jobs.HandlerOptions{Queue: jobs.QueuePrivacy, MaxAttempts: MaxAttempts, Timeout: -1}
	if err := jobs.Register[Export](reg, requestHandler[Export]{uc: uc, run: UseCases.RunExport, id: func(a Export) ids.ID { return a.Request }}, opts); err != nil {
		return err
	}
	if err := jobs.Register[Erase](reg, requestHandler[Erase]{uc: uc, run: UseCases.RunErase, id: func(a Erase) ids.ID { return a.Request }}, opts); err != nil {
		return err
	}
	maint := jobs.HandlerOptions{Queue: jobs.QueueMaintenance}
	if err := jobs.Register[Expiry](reg, roundHandler[Expiry]{uc: uc, batch: batch, run: UseCases.ExpirePackages}, maint); err != nil {
		return err
	}
	if err := jobs.Register[Reminder](reg, roundHandler[Reminder]{uc: uc, batch: batch, run: UseCases.Remind}, maint); err != nil {
		return err
	}
	if err := reg.AddPeriodic(jobs.PeriodicJob{Args: Expiry{}, Interval: interval, RunOnStart: true}); err != nil {
		return err
	}
	return reg.AddPeriodic(jobs.PeriodicJob{Args: Reminder{}, Interval: interval, RunOnStart: true})
}

// Queue implements app.Jobs over the inserter of the job runtime, bound once the runtime exists
// (the jobs step runs after the step that builds the use cases). It is safe for concurrent use.
type Queue struct {
	ins atomic.Pointer[jobs.Inserter]
}

// Compile-time check.
var _ app.Jobs = (*Queue)(nil)

// NewQueue returns an unbound queue; until Bind every insert is an error.
func NewQueue() *Queue { return &Queue{} }

// Bind sets the inserter; a second call replaces the first.
func (q *Queue) Bind(ins jobs.Inserter) { q.ins.Store(&ins) }

// inserter returns the bound inserter.
func (q *Queue) inserter() (jobs.Inserter, error) {
	if p := q.ins.Load(); p != nil && *p != nil {
		return *p, nil
	}
	return nil, errors.New("privacy: no job runtime bound")
}

// EnqueueExport implements app.Jobs.
func (q *Queue) EnqueueExport(ctx context.Context, id ids.ID) error {
	ins, err := q.inserter()
	if err != nil {
		return err
	}
	return ins.InsertTx(ctx, Export{Request: id}, nil)
}

// EnqueueErase implements app.Jobs.
func (q *Queue) EnqueueErase(ctx context.Context, id ids.ID) error {
	ins, err := q.inserter()
	if err != nil {
		return err
	}
	return ins.InsertTx(ctx, Erase{Request: id}, nil)
}
