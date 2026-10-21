// SPDX-License-Identifier: Apache-2.0

package jobs

import (
	"context"
	stdsync "sync"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/jobs"
	"github.com/OWNER/bothy/services/server/internal/runs/app"
)

// KindComponentExpiry is the kind of the job that expires the components of one card (M2-15
// design note 15 a; 02 12.3): scheduled per card message at its earliest expires_at instead of a
// periodic job, whose interval component.default_ttl would leave components active for up to a
// whole period.
const KindComponentExpiry = "runs.component_expiry"

// ComponentExpiry are the arguments of the job: only the card message (02 12.1: IDs only).
type ComponentExpiry struct {
	MessageID ids.ID `json:"message_id"`
}

// Kind returns KindComponentExpiry.
func (ComponentExpiry) Kind() string { return KindComponentExpiry }

// ComponentExpirer is the use case the job runs (app.Service.ExpireComponents).
type ComponentExpirer interface {
	// ExpireComponents expires the due active components of the card and schedules the job again
	// for the next expires_at; its error makes River retry the job.
	ExpireComponents(ctx context.Context, messageID ids.ID) error
}

// ComponentExpiryHandler works the job once; the use case locks the card and is idempotent, so a
// retry is safe (02 12.4 rule 1).
type ComponentExpiryHandler struct {
	Expirer ComponentExpirer
}

// Work runs the use case for the card of the job.
func (h ComponentExpiryHandler) Work(ctx context.Context, j jobs.Job[ComponentExpiry]) error {
	return h.Expirer.ExpireComponents(ctx, j.Args.MessageID)
}

// RegisterComponentExpiry adds the handler on the queue maintenance. Every role builds the same
// registry: the api role inserts the job with a card, worker and all run it. e may be nil only
// where no job runs (tests of the registration). The error is the rejected registration.
func RegisterComponentExpiry(reg *jobs.Registry, e ComponentExpirer) error {
	return jobs.Register[ComponentExpiry](reg, ComponentExpiryHandler{Expirer: e}, jobs.HandlerOptions{Queue: jobs.QueueMaintenance})
}

// ComponentScheduler implements app.ComponentExpiryScheduler over jobs.Inserter.InsertTx. The use
// cases are built before the job runtime of the role, so internal/wire binds the inserter once the
// runtime exists (Bind); a schedule before that is an internal error. It is safe for concurrent
// use.
type ComponentScheduler struct {
	mu  stdsync.RWMutex
	ins jobs.Inserter
}

// Compile-time check: ComponentScheduler is the outgoing port of the use cases.
var _ app.ComponentExpiryScheduler = (*ComponentScheduler)(nil)

// Bind sets the inserter of the job runtime.
func (s *ComponentScheduler) Bind(ins jobs.Inserter) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ins = ins
}

// ScheduleComponentExpiry inserts runs.component_expiry of the card in the ambient unit of work, to
// run at or after at.
func (s *ComponentScheduler) ScheduleComponentExpiry(ctx context.Context, messageID ids.ID, at time.Time) error {
	s.mu.RLock()
	ins := s.ins
	s.mu.RUnlock()
	if ins == nil {
		return errs.New(errs.CodeInternalError, "runs: the component expiry scheduler has no job runtime")
	}
	return ins.InsertTx(ctx, ComponentExpiry{MessageID: messageID}, &jobs.InsertOptions{ScheduledAt: at})
}
