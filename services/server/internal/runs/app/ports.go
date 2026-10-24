// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"time"

	"github.com/OWNER/bothy/services/server/internal/billing"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/runs/domain"
)

// RunRow is one row of runs as this module reads it. Zero values are NULL, except DurationMS,
// which HasDuration marks, and the JSON columns, which hold the stored object ("{}" at least).
type RunRow struct {
	ID, ThreadID, AgentID ids.ID
	// AgentSessionID is runs.agent_session_id: the agent_sessions row the run attached at its
	// start or at the first agent_session.report (M2-12); zero for NULL, always for a guest run.
	AgentSessionID ids.ID
	// TriggerKind is message, command or interaction.
	TriggerKind string
	// TriggeringMessageID, TriggeringCommandID and TriggeringInteractionID name the trigger.
	TriggeringMessageID, TriggeringCommandID, TriggeringInteractionID ids.ID
	// TriggeredBy is the person who triggered.
	TriggeredBy ids.ID
	State       domain.State
	// StateReason is a reason of 09 5.2; "" for NULL.
	StateReason string
	Version     int64
	// BillingModeExpected is never empty; BillingModeUsed "" for NULL.
	BillingModeExpected, BillingModeUsed string
	// InvocationJTI is runs.invocation_jti; RenewedFromJTI runs.renewed_from_jti and StartedJTI
	// runs.started_jti (zero for NULL).
	InvocationJTI, RenewedFromJTI, StartedJTI ids.ID
	// BillingEvidence is the stored object billing_evidence.v1 ("{}" at least).
	BillingEvidence []byte
	// Model, StatusLine, LastTextExcerpt and Summary are "" for NULL.
	Model, StatusLine, LastTextExcerpt, Summary string
	// Plan and Metrics are the stored JSON objects.
	Plan, Metrics []byte
	// CostMicros, Currency and CostIsEstimate are the cost columns.
	CostMicros     int64
	Currency       string
	CostIsEstimate bool
	// QueuedAt is never zero; StartedAt and FinishedAt are zero for NULL.
	QueuedAt, StartedAt, FinishedAt time.Time
	// InvocationHeldUntil is runs.invocation_held_until (R-DM-44, M3-07): the release time of a
	// held agent.invocation; zero for NULL (never held, or released).
	InvocationHeldUntil time.Time
	// DurationMS is runs.duration_ms when HasDuration.
	DurationMS  int64
	HasDuration bool
}

// StepRow is one row of run_steps. Zero values are NULL.
type StepRow struct {
	ID, RunID ids.ID
	// Ordinal is given by AppendStep; ignored on input.
	Ordinal int
	// ParentStepID is zero for none.
	ParentStepID ids.ID
	// Kind, Name, Title and Status are the column values.
	Kind, Name, Title, Status string
	// Detail is the stored JSON object; nil or empty means "{}".
	Detail []byte
	// StartedAt is the start; FinishedAt zero for NULL.
	StartedAt, FinishedAt time.Time
}

// NewRun is the input of Repository.InsertRun: a run in queued with version 1.
type NewRun struct {
	ID, ThreadID, AgentID, TriggeredBy ids.ID
	// TriggerKind is message, command or interaction; TriggeringMessageID is set for message,
	// TriggeringCommandID for command (M2-16) and TriggeringInteractionID for interaction (M2-15).
	TriggerKind                                                       string
	TriggeringMessageID, TriggeringCommandID, TriggeringInteractionID ids.ID
	// BillingModeExpected is the agent's billing mode at the acceptance (ruling D-M2-05-5).
	BillingModeExpected string
	// InvocationJTI is a fresh UUIDv7 that no token carries (ruling D-M2-05-4).
	InvocationJTI ids.ID
	// QueuedAt is the acceptance time.
	QueuedAt time.Time
	// InvocationHeldUntil is the release time of a held agent.invocation (M3-07); zero stores NULL
	// (not held).
	InvocationHeldUntil time.Time
}

// TransitionWrite is the input of Repository.TransitionRun: the compare-and-swap on state and
// version of 03 runs.
type TransitionWrite struct {
	ID ids.ID
	// From and Version are the expected current state and version.
	From    domain.State
	Version int64
	// To is the new state; Reason the stored state_reason ("" for NULL).
	To     domain.State
	Reason string
	// At is the time of the change: started_at = COALESCE(started_at, At) for running, finished_at
	// = At and duration_ms = At - started_at (NULL when never started) for an end state.
	At time.Time
	// Columns stored with the same swap (M2-09); zero values keep the column. StartedJTI also
	// clears renewed_from_jti (a start, 09 4.5.1).
	StartedJTI                                  ids.ID
	BillingModeUsed, Model, StatusLine, Summary string
	// BillingEvidence and Metrics are JSON objects; nil keeps the column.
	BillingEvidence, Metrics []byte
	// Cost are the cost columns of a finish that booked costs (M2-22); nil keeps them.
	Cost *CostColumns
}

// DeliveryWrite is the input of Repository.RecordDelivery: the columns an allowed delivery
// stores (M2-08 design note 3 h). JTI replaces invocation_jti and sets renewed_from_jti to
// RenewedFromJTI (NULL when zero, so a delivery clears it); a zero JTI keeps both (a delivery
// that may not start the run).
type DeliveryWrite struct {
	ID ids.ID
	// JTI is the jti of the new token when the matrix records it; zero otherwise.
	JTI ids.ID
	// RenewedFromJTI is the jti a renewal replaced or kept (M2-09 design note 4 e); zero for a
	// delivery.
	RenewedFromJTI ids.ID
	// BillingModeExpected is the configured agents.billing_mode now.
	BillingModeExpected string
}

// PatchWrite is the input of Repository.WritePatch: the run and the change domain.Apply computed.
type PatchWrite struct {
	ID     ids.ID
	Change domain.Change
}

// RunKey is the keyset position (queued_at DESC, id DESC) of the run lists.
type RunKey struct {
	QueuedAt time.Time
	ID       ids.ID
}

// ThreadRunsRead is the input of Repository.ThreadRuns.
type ThreadRunsRead struct {
	ThreadID ids.ID
	// After is the keyset position the page starts after; nil for the first page.
	After *RunKey
	Limit int
}

// ActiveRunsRead is the input of Repository.ActiveRuns.
type ActiveRunsRead struct {
	// AgentID keeps one agent; zero for all.
	AgentID ids.ID
	// After is the keyset position the batch starts after; nil for the first batch.
	After *RunKey
	Limit int
}

// StepsRead is the input of Repository.Steps: the steps of RunID with ordinal above AfterOrdinal
// (0 for the first page), ascending, at most Limit.
type StepsRead struct {
	RunID        ids.ID
	AfterOrdinal int
	Limit        int
}

// ActiveCount is one group of the gauge bothy.run.active: the number of active runs of one agent
// in one state.
type ActiveCount struct {
	AgentID ids.ID
	State   domain.State
	Count   int64
}

// Repository is the outgoing port of the run rows; adapters/postgres implements it over runs and
// run_steps. Reads join the ambient transaction of ctx; writes need one. The queries follow the
// visibility rule of D-M2-05-9: they read the rows of one thread or run whose visibility the use
// case checked first, the active runs whose threads the use case resolves and filters, or (the
// gauge) all active runs without a principal. Errors are kernel/errs errors.
type Repository interface {
	// InsertRun inserts r in queued with ON CONFLICT on the partial unique index of the active
	// run of a thread DO NOTHING; inserted is false when the thread has an active run (R-DM-14).
	InsertRun(ctx context.Context, r NewRun) (row RunRow, inserted bool, err error)
	// Run reads the run id; found is false for an unknown id.
	Run(ctx context.Context, id ids.ID) (row RunRow, found bool, err error)
	// ActiveRunOfThread reads the run in queued, running or waiting_input of the thread.
	ActiveRunOfThread(ctx context.Context, threadID ids.ID) (row RunRow, found bool, err error)
	// LockRun reads the run id with its row lock for the rest of the unit of work (SELECT ... FOR
	// UPDATE, M2-08); found is false for an unknown id. It needs a unit of work.
	LockRun(ctx context.Context, id ids.ID) (row RunRow, found bool, err error)
	// LockRunForAnswer reads thread, agent and state of the run id with a shared row lock for the
	// rest of the unit of work (SELECT ... FOR SHARE, M2-19 design note 7): an end of the run
	// waits for the commit of the answer, or the answer waits and sees the end state. found is
	// false for an unknown id. It needs a unit of work.
	LockRunForAnswer(ctx context.Context, id ids.ID) (run domain.AnswerRun, found bool, err error)
	// RecordDelivery stores the columns of an allowed delivery and raises version by one; the
	// caller holds the row lock and calls it only when a column changes. Errors: not_found for an
	// unknown run.
	RecordDelivery(ctx context.Context, w DeliveryWrite) (RunRow, error)
	// TransitionRun runs the compare-and-swap of w; changed is false when no row has the id, From
	// and Version (the caller reads the row to tell a missing run from a concurrent change).
	TransitionRun(ctx context.Context, w TransitionWrite) (row RunRow, changed bool, err error)
	// AppendStep inserts s with the next ordinal of its run (one more than the highest, 1 for the
	// first) and returns it as stored. The caller holds the row lock of the run (TransitionRun).
	AppendStep(ctx context.Context, s StepRow) (StepRow, error)
	// StepCounts counts the steps of each of runIDs in one statement; a run without steps is
	// absent.
	StepCounts(ctx context.Context, runIDs []ids.ID) (map[ids.ID]int, error)
	// Steps reads one page of steps of a run by ordinal.
	Steps(ctx context.Context, q StepsRead) ([]StepRow, error)
	// AllSteps reads every step of a run by ordinal.
	AllSteps(ctx context.Context, runID ids.ID) ([]StepRow, error)
	// ThreadRuns reads the runs of one thread in keyset order (queued_at DESC, id DESC).
	ThreadRuns(ctx context.Context, q ThreadRunsRead) ([]RunRow, error)
	// ActiveRuns reads the runs in queued, running or waiting_input in keyset order (queued_at
	// DESC, id DESC), optionally of one agent.
	ActiveRuns(ctx context.Context, q ActiveRunsRead) ([]RunRow, error)
	// ActiveCounts groups the active runs by agent and state for the gauge bothy.run.active.
	ActiveCounts(ctx context.Context) ([]ActiveCount, error)
	// LockActiveRunsByStarter locks the active runs triggered by one of principalIDs in ascending
	// id (R-SY-11, M2-09). It needs a unit of work.
	LockActiveRunsByStarter(ctx context.Context, principalIDs []ids.ID) ([]RunRow, error)
	// LockActiveRunsByAgent locks the active runs of agentID in ascending id (09 2.1, M2-09). It
	// needs a unit of work.
	LockActiveRunsByAgent(ctx context.Context, agentID ids.ID) ([]RunRow, error)
	// PatchSteps reads every step of a run in ordinal order for a patch (M2-10): the caller holds
	// the row lock of the run (LockRun). Detail is loaded only for the summary step
	// (domain.IsSummaryStep), nil for every other step, so a run with many large details is not
	// read in full on every patch.
	PatchSteps(ctx context.Context, runID ids.ID) ([]StepRow, error)
	// WritePatch stores the change of one patch in the lock order of 03 (M2-10 design note 1 f):
	// one UPDATE of the run row with version = version + 1 and stall_warned_at = NULL, then the
	// deletes, updates and inserts of run_steps; it returns the run row as stored. The caller holds
	// the row lock. Errors: not_found for an unknown run, internal outside a unit of work.
	WritePatch(ctx context.Context, w PatchWrite) (RunRow, error)
	// ActiveRunsOfThreads reads the active runs of threadIDs (at most one per thread, R-DM-14) for
	// the delivery reads of runs.LiveBox, whose recipients the gateway admitted.
	ActiveRunsOfThreads(ctx context.Context, threadIDs []ids.ID) ([]RunRow, error)
	// LockActiveRunsByThreads locks the active runs of the threads in ascending id (M8-03: a
	// channel became read-only). It needs a unit of work.
	LockActiveRunsByThreads(ctx context.Context, threadIDs []ids.ID) ([]RunRow, error)
	// OrphanCandidates reads the runs in running or waiting_input, ascending by id, for the job
	// runs.stall_check (M2-09); a thread has at most one (R-DM-14).
	OrphanCandidates(ctx context.Context) ([]RunRow, error)
	// LockActiveRunOfThread reads the run in queued, running or waiting_input of the thread with
	// its row lock for the rest of the unit of work (M2-11 design note 3 a): a run that ended while
	// the statement waited is not found. It needs a unit of work.
	LockActiveRunOfThread(ctx context.Context, threadID ids.ID) (row RunRow, found bool, err error)
	// RaiseRunVersion raises runs.version by one for a stored change of the run document outside a
	// patch (a follow-up accepted, refused or deleted, M2-11 design note 10) and returns the new
	// version. The caller holds the row lock. Errors: not_found for an unknown run, internal
	// outside a unit of work.
	RaiseRunVersion(ctx context.Context, id ids.ID) (int64, error)
	// LockHeldRunsOfMessage locks the queued runs triggered by messageID whose agent.invocation is
	// held (invocation_held_until set), in ascending id, and returns their ids (M3-07, the
	// subscriber of files.FileFinalized). It needs a unit of work.
	LockHeldRunsOfMessage(ctx context.Context, messageID ids.ID) ([]ids.ID, error)
	// ReleaseHeldInvocation clears invocation_held_until of a queued held run and raises version
	// by one (M3-07); released is false when the run is not queued or not held, which leaves it
	// unchanged. The caller holds the row lock. Errors: internal outside a unit of work.
	ReleaseHeldInvocation(ctx context.Context, id ids.ID) (released bool, err error)
	// SetRunSession writes runs.agent_session_id of runID and nothing else: no version change and
	// no event (M2-12 design notes 10 and 12). It needs a unit of work. Errors: not_found for an
	// unknown run.
	SetRunSession(ctx context.Context, runID, sessionID ids.ID) error
	// SetCostTotal writes cost_micros, currency and cost_is_estimate of the run t.RunID and raises
	// version by one when one of them differs (M2-28 design note 7); changed is false for an equal
	// or unknown run, which stays untouched. It needs a unit of work.
	SetCostTotal(ctx context.Context, t billing.RunCostTotal) (changed bool, err error)
	// SessionUsed reports whether some run of threadID has runs.agent_session_id = sessionID, the
	// difference of resume and bind (09 9.1, M2-12 design note 9). A session row belongs to one
	// thread, so the thread bounds the lookup.
	SessionUsed(ctx context.Context, threadID, sessionID ids.ID) (bool, error)
}

// FollowupRow is one row of run_followups (M2-11).
type FollowupRow struct {
	RunID, MessageID ids.ID
	State            domain.FollowupState
}

// FollowupRepository is the outgoing port of the run_followups rows (M2-11); adapters/postgres
// implements it next to Repository. Every read names the rows of one run, or one message, whose
// run the use case authorized or changes in the same unit of work (07 section 7.4 rule 1). Writes
// and locking reads need the unit of work; the caller holds the row lock of the run first (03
// section 6.3). Errors are kernel/errs errors.
type FollowupRepository interface {
	// InsertFollowup inserts the follow-up messageID of runID in queued.
	InsertFollowup(ctx context.Context, runID, messageID ids.ID) error
	// LockOpenFollowups locks the follow-ups of runID in queued or delivered in ascending
	// message_id.
	LockOpenFollowups(ctx context.Context, runID ids.ID) ([]FollowupRow, error)
	// RunFollowups reads every follow-up of runID in ascending message_id (the run document).
	RunFollowups(ctx context.Context, runID ids.ID) ([]FollowupRow, error)
	// FollowupStatesOf reads the follow-ups of runID among messageIDs.
	FollowupStatesOf(ctx context.Context, runID ids.ID, messageIDs []ids.ID) ([]FollowupRow, error)
	// LockFollowup reads the follow-up messageID of runID with its row lock; found is false
	// when the message is no follow-up of the run.
	LockFollowup(ctx context.Context, runID, messageID ids.ID) (row FollowupRow, found bool, err error)
	// SetFollowupStates stores state for the follow-ups messageIDs of runID.
	SetFollowupStates(ctx context.Context, runID ids.ID, messageIDs []ids.ID, state domain.FollowupState) error
	// MoveFollowups moves the follow-ups messageIDs of fromRunID to toRunID in queued.
	MoveFollowups(ctx context.Context, fromRunID, toRunID ids.ID, messageIDs []ids.ID) error
	// FollowupOfMessage reads the follow-up of messageID without a lock; found is false when the
	// message is no follow-up.
	FollowupOfMessage(ctx context.Context, messageID ids.ID) (row FollowupRow, found bool, err error)
}

// RunBindings is the outgoing port of the acting-for resolution (07 guard rule SR9, M2-14 Design
// note 3): one run by primary key with the columns that bind a request to its triggering person.
// It is separate from Repository because it runs during the authentication of a request, before
// any unit of work and without a lock (Design note 3g), and reads runs without the visible-channel
// predicate (Design note 3f); adapters/postgres implements both.
type RunBindings interface {
	// RunBinding reads the run id; found is false for an unknown id. Errors are kernel/errs
	// errors (internal for a failed read).
	RunBinding(ctx context.Context, id ids.ID) (binding domain.RunBinding, found bool, err error)
}
