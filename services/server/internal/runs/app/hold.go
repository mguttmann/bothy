// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"log/slog"
	"time"

	"github.com/OWNER/bothy/services/server/internal/files"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/reqmeta"
	"github.com/OWNER/bothy/services/server/internal/messaging"
	"github.com/OWNER/bothy/services/server/internal/runs"
	"github.com/OWNER/bothy/services/server/internal/runs/domain"
)

// msgHoldTimeout is the Info line of a release by runs.invocation_hold_timeout; it carries only
// bothy.run.id (12 section 14). The window was either key (domain.HoldWindow), so the line names
// both.
const msgHoldTimeout = "runs: the hold window (file.invocation_hold_max or voice.transcript_hold_max) passed; " +
	"the held agent.invocation is released with the attachments and the transcript in their current state"

// invokeOrHold opens the run of cmd in the caller's unit of work and writes its first
// agent.invocation, or holds it (04 15.3 point 1, 12 10, R-FI-09; M3-07 design note 11). It is the
// last step of every path that opens a run with a triggering message: the acceptance and the
// follow-on run of M2-11. mayHold is false for a message created without attachments, which costs
// no files call (M1-56 D9); the follow-on run passes true. When the triggering message is pending
// (files.Attachments.Pending: an attachment not final, or the server transcript of a voice message
// pending, R-DM-47), Open stores invocation_held_until = one clock reading plus the window of
// domain.HoldWindow (voice.transcript_hold_max when only the transcript is pending,
// file.invocation_hold_max otherwise; M3-09 design note 13), no agent.invocation is written and
// runs.invocation_hold_timeout is scheduled at that time; the release follows by
// ReleaseInvocation. Everything else of Open (anchor, run.state_changed, trigger_state received,
// runs.queued_expiry) is the same either way.
//
// Errors: those of Open (among them *ActiveRunExistsError, nothing written), of files, of the
// scheduler and of the appender.
func (s *Service) invokeOrHold(ctx context.Context, cmd OpenCmd, mayHold bool) (runs.RunSummary, error) {
	reason := files.PendingNone
	if mayHold && s.d.Files != nil && !cmd.TriggeringMessageID.IsZero() {
		// The attachments were bound earlier in this unit of work and their file rows are locked
		// by it, or the message is older (the follow-on run); either way the answer is current.
		var err error
		if reason, err = s.d.Files.Pending(ctx, cmd.TriggeringMessageID); err != nil {
			return runs.RunSummary{}, err
		}
	}
	window := domain.HoldWindow(reason == files.PendingTranscript, s.cfg.InvocationHoldMax, s.cfg.TranscriptHoldMax)
	cmd.InvocationHeldUntil = domain.HoldUntil(s.now(), reason != files.PendingNone, window)
	sum, err := s.Open(ctx, cmd)
	if err != nil {
		return runs.RunSummary{}, err
	}
	if cmd.InvocationHeldUntil.IsZero() {
		return sum, s.emitInvocation(ctx, sum, cmd.TriggeringMessageID, cmd.BillingModeExpected, false)
	}
	return sum, s.scheduleHold(ctx, sum.ID, cmd.InvocationHeldUntil)
}

// scheduleHold inserts runs.invocation_hold_timeout for the run at the release time at; without a
// scheduler nothing is inserted (Deps.Holds).
func (s *Service) scheduleHold(ctx context.Context, runID ids.ID, at time.Time) error {
	if s.d.Holds == nil {
		return nil
	}
	return s.d.Holds.ScheduleInvocationHoldTimeout(ctx, runID, at)
}

// ReleaseInvocation writes the held agent.invocation of the run inside the ambient unit of work
// (04 15.3 point 1, D-K-G6; M3-07 design note 12). The run row is locked first; for ReleaseFinal
// Pending of its triggering message is read after the lock, so a finalization committed before
// the lock was granted is seen. domain.ReleaseVerdict decides: only a queued run whose
// invocation_held_until is set is released, for ReleaseFinal only when nothing is pending
// (files.PendingNone: every attachment final and no server transcript pending, M3-09), for
// ReleaseTimeout also with a pending transcript. The
// release clears the column and raises runs.version by one (no run.state_changed), then appends
// agent.invocation with the facts of now (the scope of the run's thread, the agent's billing mode,
// the channel's settings_version, triggered_by) and without actor and correlation_id. It never
// authorizes (runs.Delivery.Prepare checks agent.invoke before the hand-over) and never touches
// trigger_state. Both causes serialize on the run row and the first release clears the column, so
// a run gets exactly one agent.invocation; a run that ended while held is never released. A
// release by ReleaseTimeout logs one Info line with bothy.run.id.
//
// Errors: internal outside a unit of work; those of the database, files and the appender. An
// unknown run is nothing.
//
// Example:
//
//	err := uow.Do(ctx, func(ctx context.Context) error { return svc.ReleaseInvocation(ctx, runID, domain.ReleaseTimeout) })
func (s *Service) ReleaseInvocation(ctx context.Context, runID ids.ID, cause domain.ReleaseCause) error {
	if err := needUnit(ctx, "ReleaseInvocation"); err != nil {
		return err
	}
	row, found, err := s.d.Repo.LockRun(ctx, runID)
	if err != nil || !found {
		return err
	}
	pending := false
	if cause == domain.ReleaseFinal && domain.ReleaseVerdict(row.State, row.InvocationHeldUntil, cause, false) && s.d.Files != nil {
		reason, err := s.d.Files.Pending(ctx, row.TriggeringMessageID)
		if err != nil {
			return err
		}
		pending = reason != files.PendingNone
	}
	if !domain.ReleaseVerdict(row.State, row.InvocationHeldUntil, cause, pending) {
		return nil
	}
	released, err := s.d.Repo.ReleaseHeldInvocation(ctx, row.ID)
	if err != nil || !released {
		return err
	}
	mode := row.BillingModeExpected
	facts, err := s.d.Agents.RunFacts(ctx, []ids.ID{row.AgentID})
	if err != nil {
		return err
	}
	if m := facts[row.AgentID].BillingMode; m != "" {
		mode = m
	}
	sum := runs.RunSummary{ID: row.ID, ThreadID: row.ThreadID, AgentID: row.AgentID, TriggeredBy: row.TriggeredBy}
	if err := s.emitInvocation(ctx, sum, row.TriggeringMessageID, mode, true); err != nil {
		return err
	}
	if cause == domain.ReleaseTimeout {
		s.d.Log.LogAttrs(ctx, slog.LevelInfo, msgHoldTimeout, slog.String(logKeyRunID, row.ID.String()))
	}
	return nil
}

// InvocationHoldTimeout is the use case of the job runs.invocation_hold_timeout (M3-07 design note
// 13 b) in one unit of work: the run is locked; not queued or not held, nothing happens; before
// the stored release time (clock skew between nodes) the job is scheduled again at that time;
// otherwise ReleaseInvocation with ReleaseTimeout. Repeated execution has no second effect.
// Errors: those of the database and the scheduler, which make River retry the job.
func (s *Service) InvocationHoldTimeout(ctx context.Context, runID ids.ID) error {
	return s.d.UoW.Do(ctx, func(ctx context.Context) error {
		row, found, err := s.d.Repo.LockRun(ctx, runID)
		if err != nil || !found || row.State != domain.StateQueued || row.InvocationHeldUntil.IsZero() {
			return err
		}
		if s.now().Before(row.InvocationHeldUntil) {
			return s.scheduleHold(ctx, row.ID, row.InvocationHeldUntil)
		}
		return s.ReleaseInvocation(ctx, row.ID, domain.ReleaseTimeout)
	})
}

// OnFileFinalized is the in-transaction subscriber of files.FileFinalized (02 section 4.3 row
// runs, M3-07 design note 13 a). It runs in the finalizing unit of work, so the final state of the
// file, its file.updated and a released agent.invocation commit together: for a file attached to
// a message, the queued held runs of that message are locked in ascending id and each gets
// ReleaseInvocation with ReleaseFinal. An unattached file, an unbound acceptor and a service
// without files do nothing. Errors: those of the database, files and the appender, which veto the
// unit of work.
func (a *Acceptor) OnFileFinalized(ctx context.Context, ev files.FileFinalized) error {
	s := a.svc.Load()
	if s == nil || s.d.Files == nil {
		return nil
	}
	return s.onFileFinalized(ctx, ev)
}

// onFileFinalized is OnFileFinalized with the bound use cases.
func (s *Service) onFileFinalized(ctx context.Context, ev files.FileFinalized) error {
	ref, ok, err := s.d.Files.MessageOf(ctx, ev.FileID)
	if err != nil || !ok {
		return err
	}
	held, err := s.d.Repo.LockHeldRunsOfMessage(ctx, ref.MessageID)
	if err != nil {
		return err
	}
	for _, id := range held {
		if err := s.ReleaseInvocation(ctx, id, domain.ReleaseFinal); err != nil {
			return err
		}
	}
	return nil
}

// checkFilesOut enforces the owner's switch supports_files_out (09 section 2.2, M3-07 design note
// 15): a message of an agent with attachments is refused with feature_disabled when the switch
// is false, which vetoes the unit of work of the message, so neither message nor attachment rows
// stay; a missing key allows. The adapter half (heartbeat capabilities.files_out) is not
// evaluated here.
func (s *Service) checkFilesOut(ctx context.Context, ev messaging.MessagePosted) error {
	if ev.Origin != originAgent || !ev.HasAttachments {
		return nil
	}
	facts, err := s.d.Agents.RunFacts(ctx, []ids.ID{ev.AuthorID})
	if err != nil {
		return err
	}
	if p := facts[ev.AuthorID].FilesOutPermitted; p != nil && !*p {
		return errs.New(errs.CodeFeatureDisabled, "the owner of this agent has switched off sending files")
	}
	return nil
}

// uncorrelated returns ctx without the correlation the appender attaches: the request correlation
// id of kernel/reqmeta and, through Deps.Uncorrelated, the trace span of platform/otel.
func (s *Service) uncorrelated(ctx context.Context) context.Context {
	ctx = reqmeta.WithCorrelationID(ctx, "")
	if s.d.Uncorrelated != nil {
		ctx = s.d.Uncorrelated(ctx)
	}
	return ctx
}
