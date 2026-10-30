// SPDX-License-Identifier: Apache-2.0

package runs

import (
	"context"
	"encoding/json"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// Register declares the events runs emits and the ops it owns (M1-25, docs/spec/05-actions.md
// sections 6 and 7): the durable run.state_changed (M2-05), agent.invocation (M2-08),
// command.invoked (M2-16), run.cancel_requested (M2-09) and interaction.created (M2-15), the
// ephemeral command.autocomplete_requested and the gateway op command.autocomplete_result (M2-17),
// whose handler the gateway runs on CommandExec, and the gateway op run.update it serves (M2-10).
// internal/wire lists it in moduleRegistrations.
func Register(r *authorize.Registry) {
	r.Event("runs", authorize.EventRunStateChanged)
	r.Event("runs", authorize.EventAgentInvocation)
	r.Event("runs", authorize.EventCommandInvoked)
	r.Event("runs", authorize.EventRunCancelRequested)
	r.Event("runs", authorize.EventInteractionCreated)
	r.Event("runs", authorize.EventCommandAutocompleteRequested)
	r.Op("runs", authorize.OpCommandAutocompleteResult)
	// The live-box op of the adapter (M2-10); run.updated is ephemeral and needs no declaration.
	r.Op("runs", authorize.OpRunUpdate)
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
	// StallWarnedAt is the stall notice of runs.stall_check (09 5.5, M2-51); zero while unset.
	StallWarnedAt time.Time
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
	// Followups are the follow-ups of the run in ascending message id (M2-11); never nil. The
	// REST form keys them by message id (schema RunFollowup).
	Followups []Followup
}

// Followup is one follow-up of a run in the run document (schema RunFollowup): the message and
// its state queued, delivered, absorbed, deferred or discarded (09 section 7).
type Followup struct {
	MessageID ids.ID
	State     string
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

// BillingEvidence is billing_evidence.v1 as an adapter reports it at the start (schema
// BillingEvidence): codes only, never a key value or prefix.
type BillingEvidence struct {
	Provider, KeyType, Method string
}

// StartRun is the input of Runs.Start (startRun) and Runs.Resume (resumeRun), schema RunStart.
type StartRun struct {
	RunID ids.ID
	// Token is the compact invocation token; it is never logged, stored or echoed.
	Token string
	// BillingModeUsed and ToolClassUsed are the mode and the tool class the runtime uses; Model
	// the model ("" for none).
	BillingModeUsed, ToolClassUsed, Model string
	// BillingEvidence is nil when absent.
	BillingEvidence *BillingEvidence
	// ExternalSessionRef is the reference of the external session the runtime uses, nil when
	// absent. At startRun of an operator run it attaches the current row of the agent or creates
	// one with started false (agents.Sessions.AttachOnStart, M2-12 design note 10); an invalid one
	// fails with 400 validation_failed and pointer /external_session_ref. resumeRun and a guest
	// run ignore it.
	ExternalSessionRef *string
}

// StartResult is the result of a start: the run and whether the answer is the replay of a start
// with the same token (header Idempotent-Replay).
type StartResult struct {
	Run    Run
	Replay bool
}

// RenewToken is the input of Runs.RenewToken (renewInvocationToken), schema InvocationTokenRenew.
type RenewToken struct {
	RunID ids.ID
	// Token is the presented token, also an expired one.
	Token string
}

// TokenGrant is the answer of a renewal, schema InvocationTokenGrant. Token is the compact JWS;
// it is a response field only and never stored or logged.
type TokenGrant struct {
	Token     string
	ExpiresAt time.Time
}

// AwaitInput is the input of Runs.AwaitInput (awaitRunInput), schema RunAwaitInput.
type AwaitInput struct {
	RunID      ids.ID
	StatusLine string
}

// FinishRun is the input of Runs.Finish (finishRun), schema RunFinish.
type FinishRun struct {
	RunID ids.ID
	// State is completed, failed or cancelled; Reason the reason of failed and cancelled.
	State, Reason, Summary string
	// Metrics is the object run_metrics.v1; nil when absent.
	Metrics map[string]any
	// Costs are the reports of RunFinish.costs in order; nil or empty books nothing (M2-22).
	Costs []CostReport
}

// CostReport is the cost of one model in a finished run as the adapter reports it (schema
// CostReport). It mirrors billing.CostReport, because a port imports only kernel (02 section 5);
// the use case maps it. Absent token counts are 0.
type CostReport struct {
	// Provider and Model name the model exactly as the runtime reports them.
	Provider, Model string
	// TokensIn, TokensOut, TokensCacheRead, TokensCacheWrite (5 minutes or no stated duration)
	// and TokensCacheWrite1h are the consumed tokens of this run.
	TokensIn, TokensOut, TokensCacheRead, TokensCacheWrite, TokensCacheWrite1h int64
	// AmountMicros is the adapter's amount; nil lets the server compute.
	AmountMicros *int64
	// Currency is the ISO 4217 code of the report.
	Currency string
}

// CancelRun is the input of Runs.Cancel (cancelRun), schema RunCancel.
type CancelRun struct {
	RunID ids.ID
	// Reason is the optional text of the person; it is checked and stored nowhere (D-M2-09-11).
	Reason string
}

// RetryRun is the input of Runs.Retry (retryRun, addendum of 2026-10-30).
type RetryRun struct {
	RunID ids.ID
}

// Runs is the read side of the runs module for handlers and other modules (M2-05). The caller is
// the principal of ctx (kernel/actor). Errors are kernel/errs errors; an unknown and an invisible
// run or thread give the same 404 not_found (02 section 9.2), and a foreign cursor 400
// cursor_invalid. Apart from the adapter operations and cancelRun of M2-09, which the REST handlers
// call, state changes go through runs/app (Open, Transition): no other module changes run states.
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
	// Start moves a delivered run queued -> running with its current invocation token (startRun,
	// run.start, M2-09). Errors: 404 for an unknown or invisible run, 403 forbidden, 403
	// invocation_token_invalid with reason expired, superseded or invalid, 409 already_started,
	// invalid_state_transition, billing_mode_mismatch or tool_class_mismatch (the last two fail
	// the run), 400 validation_failed.
	//
	// Example:
	//
	//	res, err := rs.Start(ctx, runs.StartRun{RunID: id, Token: tok, BillingModeUsed: "organization", ToolClassUsed: "scoped"})
	Start(ctx context.Context, cmd StartRun) (StartResult, error)
	// Resume moves a waiting run waiting_input -> running with the token of the answer
	// (resumeRun, run.start); errors as Start.
	Resume(ctx context.Context, cmd StartRun) (StartResult, error)
	// RenewToken issues a fresh token for an own run in queued or waiting_input (09 4.5.1,
	// run.token.renew). Errors: as Start for the token, 409 already_started, 403
	// agent_invocation_denied when the checks before a delivery refuse (the run fails).
	RenewToken(ctx context.Context, cmd RenewToken) (TokenGrant, error)
	// AwaitInput moves running -> waiting_input (awaitRunInput, run.finish); waiting_input again
	// answers without change. Errors: 409 invalid_state_transition from another state.
	AwaitInput(ctx context.Context, cmd AwaitInput) (Run, error)
	// Finish ends a run with completed, failed or cancelled (finishRun, run.finish); the same end
	// state again answers without change. Errors: 400 validation_failed, 409
	// invalid_state_transition.
	Finish(ctx context.Context, cmd FinishRun) (Run, error)
	// Patch applies a live-box patch of the run's own agent (patchRun and the gateway op
	// run.update, run.update; M2-10): RFC 6902 add, replace and remove on the paths of 09
	// section 5.3 with base_version as compare-and-swap on runs.version. A stored patch raises the
	// version by exactly one, writes no durable event and publishes the ephemeral run.updated
	// after the commit. Errors: 404 for an unknown or invisible run, 403 forbidden (another
	// agent's run, a person or an app), 400 validation_failed with errors[] (codes
	// patch_path_forbidden, patch_target_missing, patch_target_exists, limit_reached and the
	// value codes), 409 conflict with reason run_not_active for an ended run, 409
	// base_version_conflict with the run document in Current.
	//
	// Example:
	//
	//	res, err := rs.Patch(ctx, runs.PatchRun{RunID: id, BaseVersion: 7, Ops: []runs.PatchOp{
	//		{Op: "replace", Path: "/status_line", Value: json.RawMessage(`"Reading export.log"`)}}})
	Patch(ctx context.Context, in PatchRun) (PatchResult, error)
	// Cancel stops a run for a person (cancelRun, run.cancel): a queued run ends at once, an active
	// one gets run.cancel_requested and the job runs.cancel_timeout. Errors: 404, 403, 400
	// validation_failed for the reason, 409 invalid_state_transition for completed or failed.
	Cancel(ctx context.Context, cmd CancelRun) (RunSummary, error)
	// Retry starts a new run of the same agent for the triggering message of a failed or
	// cancelled run (retryRun, rights of agent.invoke; 09 5.7, addendum of 2026-10-30). Only the
	// newest run of its thread with trigger_kind message qualifies; a second retry of the same run
	// answers the first retry and writes nothing. Errors: 404 for an unknown or invisible run,
	// 409 invalid_state_transition, 403 agent_invocation_denied with the reason of agent.invoke or
	// operator_not_in_channel.
	Retry(ctx context.Context, cmd RetryRun) (RunSummary, error)
	// Context returns run_context.v1 of an own active run for its agent (getRunContext,
	// run.context.read; 09 section 4.6, M2-13): the triggering message whole, the earlier messages
	// of the thread above the floor of the session and, on the first run of a session in a thread
	// with a parent message in its current channel, the main-thread messages before it, each with
	// role and trust computed now. The principal is the one of ctx. Errors: 404 not_found alike for
	// an unknown run, an invisible run, a run that is not active and a triggering message the agent
	// cannot read; 403 forbidden with reason not_self (another agent) or subject_kind_not_allowed (a
	// person or an app); internal_error for a run whose trigger is not a message.
	//
	// Example:
	//
	//	rc, err := rs.Context(ctx, runID)
	Context(ctx context.Context, runID ids.ID) (RunContext, error)
}

// RunContext is run_context.v1 (schema RunContext, 09 4.6). ChannelContext is nil when the part
// is absent and an empty slice when it is built without elements; TruncatedBeforeSeq is set only
// when HasTruncatedBeforeSeq.
type RunContext struct {
	RunID ids.ID
	// ContextFloorSeq is the floor of the session (0 outside mode resume).
	ContextFloorSeq int64
	// TriggerMessage is the triggering message, never cut.
	TriggerMessage ContextMessage
	// ThreadContext are the earlier messages of the run's thread, ascending; never nil.
	ThreadContext []ContextMessage
	// ChannelContext are the main-thread messages before the parent message, ascending; nil when
	// absent.
	ChannelContext []ContextMessage
	// TruncatedBeforeSeq marks older missing messages of the thread when HasTruncatedBeforeSeq.
	TruncatedBeforeSeq    int64
	HasTruncatedBeforeSeq bool
}

// ContextAuthor is the author of an element (schema PrincipalRef): id, kind, handle and display
// name, never an e-mail address (20 section 6).
type ContextAuthor struct {
	ID                  ids.ID
	Kind                string
	Handle, DisplayName string
	// AvatarFileID is zero for none.
	AvatarFileID ids.ID
}

// ContextAttachment is one attachment of an element (schema RunContextAttachment); the file
// contents are fetched by the adapter with file.read and are always marked data.
type ContextAttachment struct {
	FileID                 ids.ID
	OriginalName, MimeType string
	SizeBytes              int64
	ScanStatus             string
	// Transcript is the transcript of a voice message; "" for none.
	Transcript string
}

// ContextMessage is one element of a run context (schema RunContextMessage). Zero values of the
// optional members are absent members.
type ContextMessage struct {
	MessageID, ChannelID ids.ID
	ThreadSeq            int64
	CreatedAt            time.Time
	Author               ContextAuthor
	// Role is user, assistant or other; Trust trigger, authorized, self, unverified or system.
	Role, Trust string
	// Origin and Kind are the stored values of the message.
	Origin, Kind string
	ContentMD    string
	// Truncated reports a cut at agent.context_message_max_bytes.
	Truncated bool
	// Attachments are never nil.
	Attachments []ContextAttachment
	// ReplyToMessageID is zero for none.
	ReplyToMessageID ids.ID
	// AlreadyDelivered marks a follow-up this session already received (mode resume).
	AlreadyDelivered bool
}

// PatchOp is one RFC 6902 operation as sent by the adapter (09 5.3).
type PatchOp struct {
	// Op is "add", "replace" or "remove".
	Op string
	// Path is a JSON Pointer (RFC 6901) into run_document.v1.
	Path string
	// Value is the raw JSON value; empty for "remove" and for an absent member, the bytes null
	// for a JSON null (which the checks refuse).
	Value json.RawMessage
}

// PatchRun is the input of Runs.Patch; REST (patchRun) and the gateway op run.update build the
// same value.
type PatchRun struct {
	RunID       ids.ID
	BaseVersion int64
	Ops         []PatchOp
}

// PatchResult carries the new runs.version (schema RunPatchResult).
type PatchResult struct{ Version int64 }

// RunUpdated is the payload of the ephemeral event run.updated as runs publishes it after the
// commit of a patch (schema RunUpdatedPayload, M2-10 design note 8). Ops are the normalized ops
// with the stored values; Truncated means the ops were left out because the encoded payload
// exceeds sync.event_payload_max_bytes, and the gateway sends the whole document instead. ThreadID
// is the run's thread, which the gateway needs for the focus index; it is not a member of the
// payload. MarshalJSON writes the payload form.
type RunUpdated struct {
	RunID, ThreadID      ids.ID
	BaseVersion, Version int64
	Ops                  []PatchOp
	Truncated            bool
}

// MarshalJSON implements json.Marshaler with the members of RunUpdatedPayload: run_id,
// base_version, version and ops (each {op, path, value}, value absent for remove); ops absent
// when Truncated.
func (u RunUpdated) MarshalJSON() ([]byte, error) {
	type op struct {
		Op    string          `json:"op"`
		Path  string          `json:"path"`
		Value json.RawMessage `json:"value,omitempty"`
	}
	type payload struct {
		RunID       ids.ID `json:"run_id"`
		BaseVersion int64  `json:"base_version"`
		Version     int64  `json:"version"`
		Ops         *[]op  `json:"ops,omitempty"`
	}
	p := payload{RunID: u.RunID, BaseVersion: u.BaseVersion, Version: u.Version}
	if !u.Truncated {
		ops := make([]op, 0, len(u.Ops))
		for _, o := range u.Ops {
			ops = append(ops, op(o))
		}
		p.Ops = &ops
	}
	return json.Marshal(p)
}

// LiveBox serves the delivery reads of the gateway (M2-10 design note 10): the whole run
// document of active runs for the snapshot after focus and resume and after a dropped frame.
// They do not authorize: the focus handler checked gateway.focus per thread and the gateway admits
// every recipient with run.read before it sends (07 section 7.4 rule 1, exception "loading a
// resource that has already been checked"); callers admit every recipient first. Errors are
// kernel/errs errors.
type LiveBox interface {
	// Documents returns the documents of the active runs of threadIDs (at most one per thread,
	// R-DM-14), in no particular order; threads without an active run are absent.
	Documents(ctx context.Context, threadIDs []ids.ID) ([]Run, error)
	// Document returns the document of the run runID; false when it is unknown or not active.
	Document(ctx context.Context, runID ids.ID) (Run, bool, error)
}

// LiveRun is one active run as the live frame stats.live shows it (docs/spec/25-visualization.md
// 5.2, M6-15): IDs, place, state, times, billing mode, current step kind and name, the approval
// flag and the cost so far. It carries no person, no trigger and no content (SEC-VZ-3).
type LiveRun struct {
	ID, ThreadID, AgentID ids.ID
	// ChannelID and SpaceID are the place of the run's thread now (a run follows its thread).
	ChannelID, SpaceID ids.ID
	// State is queued, running or waiting_input.
	State string
	// QueuedAt is never zero; StartedAt is zero before the start.
	QueuedAt, StartedAt time.Time
	// BillingMode is the mode the adapter reported for the run (billing_mode_used), else the
	// configured mode of its delivery (billing_mode_expected).
	BillingMode string
	// StepKind and StepName are the kind and name of the newest running step; "" for none. The
	// name of a tool step is the tool name; title and detail never leave runs here.
	StepKind, StepName string
	// ApprovalPending reports an active approval component (required capability approve_actions)
	// of the run.
	ApprovalPending bool
	// Cost is the cost so far stored on the run (runs.cost_micros, currency, cost_is_estimate).
	Cost Money
}

// LiveSource is the source port of the active runs for the live frame of stats (M6-15). It does
// not authorize: stats filters every run by the channel set of each receiver before anything is
// sent. Errors are kernel/errs errors.
type LiveSource interface {
	// LiveRuns returns the runs in queued, running or waiting_input, newest first (queued_at, then
	// id, descending), at most perChannel per channel of their thread; runs whose thread is gone
	// are left out.
	//
	// Example:
	//
	//	active, err := src.LiveRuns(ctx, 200)
	LiveRuns(ctx context.Context, perChannel int) ([]LiveRun, error)
}
