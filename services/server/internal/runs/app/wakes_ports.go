// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/runs/domain"
)

// WakeConfig holds the configuration of the wake-ups (09 21, stage AR), bound in internal/wire
// next to Config. Every key defaults to the value of 09 21; none switches a behaviour on: wake-ups
// exist only after a runner that reports wakes asks for them.
type WakeConfig struct {
	// MaxPending is run.wake_max_pending: pending wake-ups per thread.
	MaxPending int `config:"run.wake_max_pending"`
	// MaxWait is run.wake_max_wait: the longest window from creation (usage_limit: from
	// not_before) to the deadline.
	MaxWait time.Duration `config:"run.wake_max_wait"`
	// MinDelay is run.wake_min_delay: the earliest fire and first check after the creation.
	MinDelay time.Duration `config:"run.wake_min_delay"`
	// UsageWakeMax is run.usage_wake_max: usage-limit wake-ups per run, every state counted.
	UsageWakeMax int `config:"run.usage_wake_max"`
	// LabelMaxChars is run.wake_label_max_chars.
	LabelMaxChars int `config:"run.wake_label_max_chars"`
	// ChainMax is run.autonomous_chain_max; 0 means no chain cap.
	ChainMax int `config:"run.autonomous_chain_max"`
	// IdleMax is run.autonomous_idle_max; at least 1.
	IdleMax int `config:"run.autonomous_idle_max"`
}

// WakeRow is one row of run_wakes (03 4.8). Zero values are NULL, except the text columns.
type WakeRow struct {
	ID, AgentID, ThreadID, SourceRunID, TriggeredBy ids.ID
	Kind                                            domain.WakeKind
	Reason                                          domain.WakeReason
	State                                           domain.WakeState
	// StateReason is "" for NULL.
	StateReason string
	// Condition is the stored run_wake_condition.v1.
	Condition []byte
	Label     string
	// Outage is 0 for NULL.
	Outage int
	// NotBefore, Deadline and CreatedAt are never zero; EndedAt is zero while pending.
	NotBefore, Deadline, CreatedAt, EndedAt time.Time
	ResultText                              string
	// FiredRunID and FiredIntoRunID are zero for NULL.
	FiredRunID, FiredIntoRunID ids.ID
	Version                    int64
}

// NewWake is the input of WakeRepository.InsertWake: a wake-up in State (pending, or refused for a
// deferred request that the checks refused) with version 1.
type NewWake struct {
	ID, AgentID, ThreadID, SourceRunID, TriggeredBy ids.ID
	Kind                                            domain.WakeKind
	Reason                                          domain.WakeReason
	State                                           domain.WakeState
	StateReason                                     string
	Condition                                       []byte
	Label                                           string
	Outage                                          int
	NotBefore, Deadline, CreatedAt                  time.Time
}

// WakeWrite is the input of WakeRepository.UpdateWake: the new state and the columns that change
// with it, a compare-and-swap on version. Zero values keep FiredRunID, FiredIntoRunID and
// ResultText; ClearFired clears both fired columns (a skipped wake input).
type WakeWrite struct {
	ID      ids.ID
	Version int64
	State   domain.WakeState
	// StateReason is stored as given ("" stores NULL).
	StateReason string
	// At is ended_at for an end state; a pending state stores NULL.
	At time.Time
	// NotBefore and Deadline replace the window when not zero (a repeated usage_limit request).
	NotBefore, Deadline time.Time
	ResultText          string
	// FiredRunID and FiredIntoRunID are set by a fire; ClearFired clears them.
	FiredRunID, FiredIntoRunID ids.ID
	ClearFired                 bool
}

// WakeKey is the keyset position (created_at DESC, id DESC) of the thread's list.
type WakeKey struct {
	CreatedAt time.Time
	ID        ids.ID
}

// WakeRepository is the outgoing port of run_wakes and run_autonomy (stage AR); adapters/postgres
// implements it next to Repository. Reads join the ambient transaction; writes and locking reads
// need one. The lock order is runs before run_wakes before run_autonomy (03 6.3). Errors are
// kernel/errs errors.
type WakeRepository interface {
	// InsertWake inserts w with ON CONFLICT on the idempotency keys of 03 run_wakes DO NOTHING;
	// inserted is false when a wake-up of the same source run and reason (recover) or outage
	// exists, which the caller then reads.
	InsertWake(ctx context.Context, w NewWake) (row WakeRow, inserted bool, err error)
	// LockWake reads the wake-up id with its row lock; found is false for an unknown id.
	LockWake(ctx context.Context, id ids.ID) (row WakeRow, found bool, err error)
	// WakeByOutage locks the wake-up of the source run's outage; found is false for none.
	WakeByOutage(ctx context.Context, sourceRunID ids.ID, outage int) (row WakeRow, found bool, err error)
	// WakeBySourceReason locks the wake-up of run.wake.recover of the source run and reason.
	WakeBySourceReason(ctx context.Context, sourceRunID ids.ID, reason domain.WakeReason) (row WakeRow, found bool, err error)
	// UpdateWake runs the compare-and-swap of w and raises the version by one; changed is false
	// when the version no longer matches.
	UpdateWake(ctx context.Context, w WakeWrite) (row WakeRow, changed bool, err error)
	// CountPendingWakes counts the pending wake-ups of the thread.
	CountPendingWakes(ctx context.Context, threadID ids.ID) (int, error)
	// CountUsageWakes counts the usage_limit wake-ups of the source run in every state.
	CountUsageWakes(ctx context.Context, sourceRunID ids.ID) (int, error)
	// PendingWakesOfAgent reads the pending wake-ups of the agent, ascending by id.
	PendingWakesOfAgent(ctx context.Context, agentID ids.ID) ([]WakeRow, error)
	// ThreadWakes reads one page of the thread's wake-ups, newest first.
	ThreadWakes(ctx context.Context, threadID ids.ID, after *WakeKey, limit int) ([]WakeRow, error)
	// ReadyWakesOfThread returns the pending wake-ups of the thread that could fire at now: an at
	// wake-up whose not_before passed, a pr_checks or command wake-up whose fire the runner
	// reported (a result text, or a returned wake input), ascending by not_before.
	ReadyWakesOfThread(ctx context.Context, threadID ids.ID, now time.Time) ([]ids.ID, error)
	// PendingWakeCounts counts the pending wake-ups by kind for the gauge bothy.run.wake.pending.
	PendingWakeCounts(ctx context.Context) (map[domain.WakeKind]int64, error)
	// LockAutonomy reads the counts of the thread and person with the row lock; found is false
	// for none.
	LockAutonomy(ctx context.Context, threadID, personID ids.ID) (a domain.Autonomy, found bool, err error)
	// SaveAutonomy inserts or replaces the counts of the thread and person.
	SaveAutonomy(ctx context.Context, threadID, personID ids.ID, a domain.Autonomy) error
	// ResetAutonomy sets both counts of the thread and person to 0 and clears ended_reason; a
	// missing row stays missing.
	ResetAutonomy(ctx context.Context, threadID, personID ids.ID) error
}

// WakeScheduler inserts the job runs.wake_due of a wake-up in the unit of work of ctx, to run at
// or after at (adapters/jobs over jobs.Inserter.InsertTx).
type WakeScheduler interface {
	ScheduleWakeDue(ctx context.Context, wakeID ids.ID, at time.Time) error
}

// WakeNotes writes the system note of a wake-up end other than fired and of an ended chain (09
// 5.8.4) in the caller's unit of work: a message with origin system in the thread, authored by
// the agent, that names the label and the reason (messaging.Messages.AppendSystemNote). It
// returns the id of the note.
type WakeNotes interface {
	AppendSystemNote(ctx context.Context, threadID, authorID ids.ID, text string) (ids.ID, error)
}

// Owners answers whether a person is an active owner of the instance at this moment
// (authorize.Owners of authz): the autonomy rule, pr_checks and its checks (07 2.14).
type Owners interface {
	IsActiveOwner(ctx context.Context, personID ids.ID) (bool, error)
}

// WakeMetrics records the signals of wake-ups (signals.md 4.3, 17 section 3): every end by state
// and reason, and every cap a chain reached. nil records nothing.
type WakeMetrics interface {
	WakeEnded(ctx context.Context, state domain.WakeState, reason domain.WakeReason)
	AutonomyCapReached(ctx context.Context, code string)
}

// noWakeMetrics records nothing.
type noWakeMetrics struct{}

// WakeEnded implements WakeMetrics without effect.
func (noWakeMetrics) WakeEnded(context.Context, domain.WakeState, domain.WakeReason) {}

// AutonomyCapReached implements WakeMetrics without effect.
func (noWakeMetrics) AutonomyCapReached(context.Context, string) {}

// WakeDeps are the collaborators of the wake-ups; Bind them with BindWakes. Without them every
// wake-up operation answers internal_error and nothing else changes (stage AR stays off).
type WakeDeps struct {
	Repo     WakeRepository
	Schedule WakeScheduler
	Notes    WakeNotes
	Owners   Owners
	// Metrics may be nil.
	Metrics WakeMetrics
}
