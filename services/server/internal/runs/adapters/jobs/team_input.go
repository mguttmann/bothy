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

// KindTeamInput is the kind of the job that copies an owner's message in a team channel to every
// member, or mirrors a member's final reply into the team channel and hands it to the members it
// mentions (docs/spec/09-agents.md 3.9.3, 3.9.4; FEAT-TEAM-CHANNEL-SERVER). Scheduled by the
// acceptance of the message in its own unit of work, so the copying never holds the locks of the
// message's thread and run.
const KindTeamInput = "runs.team_input"

// TeamInput are the arguments of the job (02 12.1: IDs only): the message. The run a member's
// reply answers is read from run_answers (red team round 7), never passed along.
type TeamInput struct {
	MessageID ids.ID `json:"message_id"`
}

// Kind returns KindTeamInput.
func (TeamInput) Kind() string { return KindTeamInput }

// TeamInputer is the use case the job runs (app.Service.TeamInput).
type TeamInputer interface {
	// TeamInput copies or mirrors the message; its error makes River retry the job.
	TeamInput(ctx context.Context, messageID ids.ID) error
}

// TeamInputHandler works the job once; the use case is idempotent (team_mirrors and team_inputs
// record what reached whom), so a retry is safe (02 12.4 rule 1).
type TeamInputHandler struct {
	Inputer TeamInputer
}

// Work runs the use case for the message of the job.
func (h TeamInputHandler) Work(ctx context.Context, j jobs.Job[TeamInput]) error {
	return h.Inputer.TeamInput(ctx, j.Args.MessageID)
}

// RegisterTeamInput adds the handler on the queue default: it writes messages, runs and events, so
// it pauses in maintenance mode with every queue except the deliveries (ADR 0065 point 4). d may be
// nil only where no job runs (tests of the registration). The error is the rejected registration.
func RegisterTeamInput(reg *jobs.Registry, d TeamInputer) error {
	return jobs.Register[TeamInput](reg, TeamInputHandler{Inputer: d}, jobs.HandlerOptions{Queue: jobs.QueueDefault})
}

// TeamScheduler implements app.TeamScheduler over jobs.Inserter.InsertTx. internal/wire binds the
// inserter once the job runtime exists (Bind); a schedule before that is an internal error. It is
// safe for concurrent use.
type TeamScheduler struct {
	mu  stdsync.RWMutex
	ins jobs.Inserter
}

// Compile-time check: TeamScheduler is the outgoing port of the use cases.
var _ app.TeamScheduler = (*TeamScheduler)(nil)

// Bind sets the inserter of the job runtime.
func (s *TeamScheduler) Bind(ins jobs.Inserter) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ins = ins
}

// ScheduleTeamInput inserts runs.team_input of the message in the ambient unit of work, to run as
// soon as it commits.
func (s *TeamScheduler) ScheduleTeamInput(ctx context.Context, messageID ids.ID) error {
	return s.ScheduleTeamInputAt(ctx, messageID, time.Time{})
}

// ScheduleTeamInputAt inserts runs.team_input of the message in the ambient unit of work, to run
// at at (the zero time: as soon as it commits); the job waiting for the files of its message
// schedules itself again this way (FIX-TEAM-CHANNEL-ATTACHMENTS D-TCAT-4).
func (s *TeamScheduler) ScheduleTeamInputAt(ctx context.Context, messageID ids.ID, at time.Time) error {
	s.mu.RLock()
	ins := s.ins
	s.mu.RUnlock()
	if ins == nil {
		return errs.New(errs.CodeInternalError, "runs: the team scheduler has no job runtime")
	}
	if at.IsZero() {
		return ins.InsertTx(ctx, TeamInput{MessageID: messageID}, nil)
	}
	return ins.InsertTx(ctx, TeamInput{MessageID: messageID}, &jobs.InsertOptions{ScheduledAt: at})
}
