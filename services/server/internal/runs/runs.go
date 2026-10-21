// SPDX-License-Identifier: Apache-2.0

package runs

import (
	"context"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// Register declares the durable events runs appends (M1-25, docs/spec/05-aktionen.md section 6):
// run.state_changed (M2-05) and agent.invocation (M2-08). internal/wire lists it in
// moduleRegistrations.
func Register(r *authorize.Registry) {
	r.Event("runs", authorize.EventRunStateChanged)
	r.Event("runs", authorize.EventAgentInvocation)
}

// Money is an amount with its currency (schema Money): AmountMicros in millionths of the unit,
// IsEstimate while the adapter has not confirmed the amount.
type Money struct {
	AmountMicros int64
	Currency     string
	IsEstimate   bool
}

// RunSummary is a run as sidebars, "Running now", the snapshot and run.state_changed show it
// (schema RunSummary). Zero values are absent members, except DurationMS, which HasDuration marks.
type RunSummary struct {
	ID, ThreadID, AgentID, TriggeredBy ids.ID
	// ChannelID is the channel of the run's thread now (a run follows its thread, M1-62).
	ChannelID ids.ID
	// State is a value of schema RunState; StateReason the reason of failed and cancelled.
	State, StateReason string
	// Version is runs.version, the ETag of the run.
	Version int64
	// BillingModeExpected is the configured mode at the last delivery; BillingModeUsed the mode
	// the adapter reported ("" before the start).
	BillingModeExpected, BillingModeUsed string
	// StatusLine and Summary are the live box lines; "" for none.
	StatusLine, Summary string
	// Cost is the stored cost; amounts per billing mode belong to M2-22.
	Cost Money
	// StepCount counts the run_steps rows of the run.
	StepCount int
	// DurationMS is runs.duration_ms; HasDuration reports that it is set (a started run that ended).
	DurationMS  int64
	HasDuration bool
	// QueuedAt is the acceptance; StartedAt and FinishedAt are zero while unset.
	QueuedAt, StartedAt, FinishedAt time.Time
}

// RunStep is one step of a run (schema RunStep). Zero values are absent members.
type RunStep struct {
	ID ids.ID
	// Ordinal is the position in the run, given by the server.
	Ordinal int
	// ParentStepID nests a step; zero for none.
	ParentStepID ids.ID
	// Kind is tool_call, status, subagent, workflow or background_job; Status pending, running,
	// completed, failed or cancelled.
	Kind, Name, Title, Status string
	// Detail is the object run_step_detail.v1; never nil.
	Detail map[string]any
	// StartedAt is the start; FinishedAt is zero while the step runs.
	StartedAt, FinishedAt time.Time
}

// Run is the run document run_document.v1 (schema Run), the content of the live box.
type Run struct {
	RunSummary
	// AgentSessionID is the agent session (M2-12); zero for none.
	AgentSessionID ids.ID
	// TriggerKind is message, command or interaction.
	TriggerKind string
	// TriggeringMessageID, TriggeringCommandID and TriggeringInteractionID name the trigger; zero
	// for none (at most one is set).
	TriggeringMessageID, TriggeringCommandID, TriggeringInteractionID ids.ID
	// Model and LastTextExcerpt are "" while unset.
	Model, LastTextExcerpt string
	// Metrics is the object run_metrics.v1 as stored; never nil.
	Metrics map[string]any
	// Plan is the object run_plan.v1; nil while it has no items (the member is absent then).
	Plan map[string]any
	// Steps are all steps of the run in ordinal order; never nil. The REST form keys them by ID.
	Steps []RunStep
}

// RunSummaryPage is one page of runs; NextCursor is "" on the last page. There is no total
// (06 section 6 point 4).
type RunSummaryPage struct {
	Items      []RunSummary
	NextCursor string
}

// RunStepPage is one page of steps by ordinal; NextCursor is "" on the last page.
type RunStepPage struct {
	Items      []RunStep
	NextCursor string
}

// ThreadRunsQuery is the query of Runs.ListByThread (listThreadRuns).
type ThreadRunsQuery struct {
	ThreadID ids.ID
	// Cursor is next_cursor of the previous page; "" for the first.
	Cursor string
	// Limit is the page size; 0 means api.page_size_default, larger values are capped at
	// api.page_size_max.
	Limit int
}

// RunStepsQuery is the query of Runs.ListSteps (listRunSteps).
type RunStepsQuery struct {
	RunID  ids.ID
	Cursor string
	Limit  int
}

// ActiveRunsQuery is the query of Runs.ListActive (listActiveRuns).
type ActiveRunsQuery struct {
	// AgentID keeps the runs of one agent; zero for all.
	AgentID ids.ID
	Cursor  string
	Limit   int
}

// Runs is the read side of the runs module for handlers and other modules (M2-05). The caller is
// the principal of ctx (kernel/actor). Errors are kernel/errs errors; an unknown and an invisible
// run or thread give the same 404 not_found (02 section 9.2), and a foreign cursor 400
// cursor_invalid. State changes go through runs/app (Open, Transition), never through this port:
// no other module changes run states.
type Runs interface {
	// Get returns the run document for the actor in ctx (getRun, run.read); an unknown or invisible
	// run gives the same not_found error.
	//
	// Example:
	//
	//	run, err := rs.Get(ctx, runID)
	Get(ctx context.Context, runID ids.ID) (Run, error)
	// ListByThread pages the runs of one thread, newest first (listThreadRuns, run.read).
	ListByThread(ctx context.Context, q ThreadRunsQuery) (RunSummaryPage, error)
	// ListSteps pages the steps of one run by ordinal (listRunSteps, run.read).
	ListSteps(ctx context.Context, q RunStepsQuery) (RunStepPage, error)
	// ListActive pages "Running now" (listActiveRuns, run.overview.read): runs in queued, running
	// or waiting_input the actor may read with run.read, newest first.
	ListActive(ctx context.Context, q ActiveRunsQuery) (RunSummaryPage, error)
	// ActiveForActor lists all active runs the actor may read, for the snapshot (04 section 6.3
	// step 4); the snapshot checked sync.snapshot.read, so run.overview.read is not checked here.
	ActiveForActor(ctx context.Context) ([]RunSummary, error)
}
