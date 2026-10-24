// SPDX-License-Identifier: Apache-2.0

// Package jobs is the adapter of the billing module to platform/jobs (M2-28): the job
// billing.reconcile on the queue default, which applies the reconciled entries of one request to
// the runs and agent sessions (docs/spec/02-architecture.md section 12.3, never periodic), and the
// ReconcileScheduler of the use cases over jobs.Inserter.InsertTx.
//
// Example:
//
//	sched := &jobs.Scheduler{}
//	svc, _ := app.New(cfg, app.Deps{Scheduler: sched, ...})
//	_ = jobs.Register(reg, svc) // every role
//	sched.Bind(runtime.Inserter()) // after the runtime of the role exists
package jobs

import (
	"context"
	stdsync "sync"

	"github.com/OWNER/bothy/services/server/internal/billing/app"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/jobs"
)

// KindReconcile is the kind of the job.
const KindReconcile = "billing.reconcile"

// Reconcile are the arguments of the job: only the run IDs of the request, ascending (02 section
// 12.1: IDs only).
type Reconcile struct {
	RunIDs []ids.ID `json:"run_ids"`
}

// Kind returns KindReconcile.
func (Reconcile) Kind() string { return KindReconcile }

// Applier is the use case the job runs (app.Service.ApplyReconciliation).
type Applier interface {
	// ApplyReconciliation applies the valid totals of the runs; its error makes River retry.
	ApplyReconciliation(ctx context.Context, runIDs []ids.ID) error
}

// Handler works the job. The use case reads its state again on every run, so a retry or a
// repeated job has no second effect (02 section 12.4 rule 1).
type Handler struct {
	Applier Applier
}

// Work applies the reconciliation of the job's runs once.
func (h Handler) Work(ctx context.Context, j jobs.Job[Reconcile]) error {
	return h.Applier.ApplyReconciliation(ctx, j.Args.RunIDs)
}

// Register adds the handler on the queue default with River's defaults and no periodic entry
// (02 section 12.3: never periodic). Every role builds the same registry: the api role inserts the
// job, worker and all run it. a may be nil only where no job runs (tests of the registration).
// The error is the rejected registration.
func Register(reg *jobs.Registry, a Applier) error {
	return jobs.Register[Reconcile](reg, Handler{Applier: a}, jobs.HandlerOptions{Queue: jobs.QueueDefault})
}

// Scheduler implements app.ReconcileScheduler over jobs.Inserter.InsertTx. The use cases are built
// before the job runtime of the role, so internal/wire binds the inserter once the runtime exists
// (Bind); a Schedule before that is an internal error. It is safe for concurrent use.
type Scheduler struct {
	mu  stdsync.RWMutex
	ins jobs.Inserter
}

// Compile-time check: Scheduler is the outgoing port of the use cases.
var _ app.ReconcileScheduler = (*Scheduler)(nil)

// Bind sets the inserter of the job runtime.
func (s *Scheduler) Bind(ins jobs.Inserter) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ins = ins
}

// Schedule inserts billing.reconcile with the run IDs in the ambient unit of work, so a rolled
// back request inserts no job.
func (s *Scheduler) Schedule(ctx context.Context, runIDs []ids.ID) error {
	s.mu.RLock()
	ins := s.ins
	s.mu.RUnlock()
	if ins == nil {
		return errs.New(errs.CodeInternalError, "billing: the reconciliation scheduler has no job runtime")
	}
	return ins.InsertTx(ctx, Reconcile{RunIDs: runIDs}, nil)
}
