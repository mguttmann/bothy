// SPDX-License-Identifier: Apache-2.0

// Package jobs is the adapter of the apps module to platform/jobs (M2-26): the job
// apps.webhook_deliver on the queue delivery, which runs one delivery attempt of a subscription
// (docs/spec/02-architektur.md "Job-Katalog"), and the DeliveryScheduler of the use cases over
// jobs.Inserter.InsertTx.
//
// Example:
//
//	sched := &jobs.Scheduler{}
//	svc, _ := app.NewWebhooks(cfg, app.WebhookDeps{Scheduler: sched, ...})
//	_ = jobs.Register(reg, svc) // every role
//	sched.Bind(runtime.Inserter()) // after the runtime of the role exists
package jobs

import (
	"context"
	stdsync "sync"
	"time"

	"github.com/OWNER/bothy/services/server/internal/apps/app"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/jobs"
)

// KindWebhookDeliver is the kind of the job.
const KindWebhookDeliver = "apps.webhook_deliver"

// WebhookDeliver are the arguments of the job: only the subscription (02 "Einsatz": IDs only).
type WebhookDeliver struct {
	SubscriptionID ids.ID `json:"subscription_id"`
}

// Kind returns KindWebhookDeliver.
func (WebhookDeliver) Kind() string { return KindWebhookDeliver }

// Deliverer is the use case the job runs (app.WebhookService.Deliver).
type Deliverer interface {
	// Deliver runs one attempt of the subscription; its error makes River retry the job.
	Deliver(ctx context.Context, subscriptionID ids.ID) error
}

// Handler works the job: one attempt. The attempt is idempotent through the claimed attempt and
// the lease, so a retry after an error is safe.
type Handler struct {
	Deliverer Deliverer
}

// Work runs the attempt once.
func (h Handler) Work(ctx context.Context, j jobs.Job[WebhookDeliver]) error {
	return h.Deliverer.Deliver(ctx, j.Args.SubscriptionID)
}

// Register adds the handler on the queue delivery. Every role builds the same registry: the api
// role inserts the job (redeliverWebhook, subscription changes), worker and all run it. d may be
// nil only where no job runs (tests of the registration). The error is the rejected registration.
func Register(reg *jobs.Registry, d Deliverer) error {
	return jobs.Register[WebhookDeliver](reg, Handler{Deliverer: d}, jobs.HandlerOptions{Queue: jobs.QueueDelivery})
}

// Scheduler implements app.DeliveryScheduler over jobs.Inserter.InsertTx. The use cases are built
// before the job runtime of the role, so internal/wire binds the inserter once the runtime exists
// (Bind); a Schedule before that is an internal error. It is safe for concurrent use.
type Scheduler struct {
	mu  stdsync.RWMutex
	ins jobs.Inserter
}

// Compile-time check: Scheduler is the outgoing port of the use cases.
var _ app.DeliveryScheduler = (*Scheduler)(nil)

// Bind sets the inserter of the job runtime.
func (s *Scheduler) Bind(ins jobs.Inserter) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ins = ins
}

// Schedule inserts apps.webhook_deliver of the subscription in the ambient unit of work, to run at
// or after at.
func (s *Scheduler) Schedule(ctx context.Context, subscriptionID ids.ID, at time.Time) error {
	s.mu.RLock()
	ins := s.ins
	s.mu.RUnlock()
	if ins == nil {
		return errs.New(errs.CodeInternalError, "apps: the delivery scheduler has no job runtime")
	}
	return ins.InsertTx(ctx, WebhookDeliver{SubscriptionID: subscriptionID}, &jobs.InsertOptions{ScheduledAt: at})
}
