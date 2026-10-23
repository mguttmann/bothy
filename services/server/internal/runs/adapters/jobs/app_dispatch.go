// SPDX-License-Identifier: Apache-2.0

package jobs

import (
	"context"
	stdsync "sync"

	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/jobs"
	"github.com/OWNER/bothy/services/server/internal/runs/app"
)

// KindAppDispatch is the kind of the job that hands an app its command or interaction after the
// commit (brief M2-25 design note 8; 02 sections 8.5 and 12.3).
const KindAppDispatch = app.JobKindAppDispatch

// AppDispatch are the arguments of the job: IDs only (02 12.1). The kind follows from the ID that
// is set: CommandID for a command, InteractionID for an interaction.
type AppDispatch struct {
	// EventID is the stored command.invoked or interaction.created; AppID its audience.
	EventID ids.ID `json:"event_id"`
	AppID   ids.ID `json:"app_id"`
	// PersonID is the invoking principal of the re-check; PersonInstallationID the installation
	// of an invoking app (absent for a person or an agent).
	PersonID             ids.ID `json:"person_id"`
	PersonInstallationID ids.ID `json:"person_installation_id,omitzero"`
	// ChannelID and ThreadID are the place; ThreadID is absent without a target thread.
	ChannelID ids.ID `json:"channel_id"`
	ThreadID  ids.ID `json:"thread_id,omitzero"`
	// CommandID is set for a command, InteractionID for an interaction.
	CommandID     ids.ID `json:"command_id,omitzero"`
	InteractionID ids.ID `json:"interaction_id,omitzero"`
}

// Kind returns KindAppDispatch.
func (AppDispatch) Kind() string { return KindAppDispatch }

// AppDispatcher is the use case the job runs (app.Service.DispatchToApp).
type AppDispatcher interface {
	// DispatchToApp works one job; it returns nil after the request to the app (no repetition)
	// and an error only for a fault before it or on shutdown.
	DispatchToApp(ctx context.Context, j app.DispatchJob) error
}

// AppDispatchHandler works the job once.
type AppDispatchHandler struct {
	Dispatcher AppDispatcher
}

// Work runs the use case for the job's arguments.
func (h AppDispatchHandler) Work(ctx context.Context, j jobs.Job[AppDispatch]) error {
	a := j.Args
	kind := app.AppDispatchCommand
	if !a.InteractionID.IsZero() {
		kind = app.AppDispatchInteraction
	}
	return h.Dispatcher.DispatchToApp(ctx, app.DispatchJob{
		Kind: kind, EventID: a.EventID, AppID: a.AppID, PersonID: a.PersonID, PersonInstallationID: a.PersonInstallationID,
		ChannelID: a.ChannelID, ThreadID: a.ThreadID, CommandID: a.CommandID, InteractionID: a.InteractionID,
	})
}

// RegisterAppDispatch adds the handler on the queue default (D-M2-25: the job may write a reply
// message, so it pauses in maintenance mode with every queue except the deliveries of ADR 0065
// point 4). Every role builds the same registry: the api role inserts the job, worker and all run
// it. d may be nil only where no job runs (tests of the registration).
func RegisterAppDispatch(reg *jobs.Registry, d AppDispatcher) error {
	return jobs.Register[AppDispatch](reg, AppDispatchHandler{Dispatcher: d}, jobs.HandlerOptions{Queue: jobs.QueueDefault})
}

// DispatchScheduler implements app.DispatchScheduler over jobs.Inserter.InsertTx. The use
// cases are built before the job runtime of the role, so internal/wire binds the inserter once the
// runtime exists (Bind); a schedule before that is an internal error. It is safe for concurrent
// use.
type DispatchScheduler struct {
	mu  stdsync.RWMutex
	ins jobs.Inserter
}

// Compile-time check: DispatchScheduler is the outgoing port of the use cases.
var _ app.DispatchScheduler = (*DispatchScheduler)(nil)

// Bind sets the inserter of the job runtime.
func (s *DispatchScheduler) Bind(ins jobs.Inserter) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ins = ins
}

// ScheduleAppDispatch inserts runs.app_dispatch in the ambient unit of work, to run at once.
func (s *DispatchScheduler) ScheduleAppDispatch(ctx context.Context, j app.DispatchJob) error {
	s.mu.RLock()
	ins := s.ins
	s.mu.RUnlock()
	if ins == nil {
		return errs.New(errs.CodeInternalError, "runs: the app dispatch scheduler has no job runtime")
	}
	return ins.InsertTx(ctx, AppDispatch{
		EventID: j.EventID, AppID: j.AppID, PersonID: j.PersonID, PersonInstallationID: j.PersonInstallationID, ChannelID: j.ChannelID,
		ThreadID: j.ThreadID, CommandID: j.CommandID, InteractionID: j.InteractionID,
	}, nil)
}
