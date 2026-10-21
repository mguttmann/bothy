// SPDX-License-Identifier: Apache-2.0

package domain

import (
	"encoding/json"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// The types of live-box patches (M2-10, docs/spec/09-agenten.md section 5.3): the input as the
// adapter sends it (RawOp), the statically checked operation (Op), the locked state a patch applies
// to (DocumentState), the stored result (Change) and the normalized RFC 6902 operations that go to
// clients (NormOp). The domain has no dependency on the port package runs, so RawOp mirrors
// runs.PatchOp; runs/app converts.

// RawOp is one RFC 6902 operation as the adapter sent it: Op is "add", "replace" or "remove" (the
// schema JsonPatchOperation already refused every other value), Path a JSON Pointer (RFC 6901)
// into run_document.v1, Value the raw JSON value; nil when the member is absent, the four bytes
// null when the adapter sent null.
type RawOp struct {
	Op    string
	Path  string
	Value json.RawMessage
}

// OpKind is the RFC 6902 operation of an Op.
type OpKind string

// The three operations a patch may use (04 section 11.2).
const (
	OpAdd     OpKind = "add"
	OpReplace OpKind = "replace"
	OpRemove  OpKind = "remove"
)

// Target is the member of run_document.v1 an Op addresses, one row of the path table of 09 5.3.
type Target uint8

// The patchable members of run_document.v1.
const (
	// TargetStatusLine is /status_line.
	TargetStatusLine Target = iota + 1
	// TargetLastTextExcerpt is /last_text_excerpt.
	TargetLastTextExcerpt
	// TargetModel is /model.
	TargetModel
	// TargetMetric is /metrics/<Field> for one of the numeric fields of run_metrics.v1.
	TargetMetric
	// TargetLastAPIRetry is /metrics/last_api_retry.
	TargetLastAPIRetry
	// TargetCost is /cost.
	TargetCost
	// TargetPlan is /plan.
	TargetPlan
	// TargetStep is /steps/<ID>, a whole new step.
	TargetStep
	// TargetStepMember is /steps/<ID>/<Field> for status, title, detail or finished_at.
	TargetStepMember
	// TargetFollowupState is /followups/<ID>/state.
	TargetFollowupState
)

// Op is one operation after the static checks of ParseOps: its position in the patch, the
// operation, the addressed member and the decoded, checked value.
type Op struct {
	// Index is the position of the operation in the patch (pointer /ops/<Index>).
	Index int
	Kind  OpKind
	// Path is the pointer as sent.
	Path   string
	Target Target
	// Field is the metric field of TargetMetric and the step member of TargetStepMember.
	Field string
	// ID is the step of TargetStep and TargetStepMember, the message of TargetFollowupState.
	ID ids.ID
	// Value is the checked value; nil for remove. Its concrete type per target:
	//   TargetStatusLine, TargetLastTextExcerpt, TargetModel: string (the excerpt not cut yet)
	//   TargetMetric: int64, or float64 for context_pct
	//   TargetLastAPIRetry: APIRetry
	//   TargetCost: Money
	//   TargetPlan: Plan
	//   TargetStep: StepInput
	//   TargetStepMember: string (status, title), json.RawMessage (detail, a JSON object not cut
	//     yet), time.Time (finished_at)
	//   TargetFollowupState: string (delivered or absorbed)
	Value any
}

// APIRetry is the object run_metrics.v1 last_api_retry (schema RunApiRetry); nil pointers are
// absent members.
type APIRetry struct {
	Attempt      int64
	MaxRetries   *int64
	RetryDelayMS *int64
	Error        *string
}

// Money is the schema Money of /cost.
type Money struct {
	AmountMicros int64
	Currency     string
	IsEstimate   bool
}

// PlanItem is one item of run_plan.v1.
type PlanItem struct {
	ID, Text, Status string
}

// Plan is run_plan.v1; a plan without items is absent from the document.
type Plan struct {
	Items []PlanItem
}

// StepInput is the value of an add on /steps/<id>: the step without ordinal (09 section 4.7).
// Zero values are absent members; Status defaults to running, Detail to {}, StartedAt to the time
// of the transaction.
type StepInput struct {
	Kind, Name, Title, Status string
	// ParentStepID is zero for none.
	ParentStepID ids.ID
	// Detail is the JSON object as sent, not cut yet; nil for absent.
	Detail json.RawMessage
	// StartedAt and FinishedAt are zero when absent.
	StartedAt, FinishedAt time.Time
}

// DocStep is one stored step of a run (03 run_steps) as a patch sees it. (The name Step is
// taken by the decision table of the trigger.)
type DocStep struct {
	ID ids.ID
	// Ordinal is the position in the run, given by the server.
	Ordinal int
	// ParentStepID is zero for none.
	ParentStepID              ids.ID
	Kind, Name, Title, Status string
	// Detail is the stored JSON object. The use case loads it only for the summary step (Apply
	// reads no other detail); nil means not loaded.
	Detail json.RawMessage
	// FinishedAt is zero while unset.
	StartedAt, FinishedAt time.Time
}

// DocumentState is the patchable part of a run as the locked row and its steps hold it at
// base_version. Empty strings are NULL columns.
type DocumentState struct {
	StatusLine, LastTextExcerpt, Model string
	// Metrics is the stored object run_metrics.v1 ("{}" at least).
	Metrics json.RawMessage
	// Plan is the stored object runs.plan ("{}" at least); the member plan exists when it has
	// items.
	Plan json.RawMessage
	Cost Money
	// Steps are all steps of the run in ordinal order. The use case loads them only when the
	// patch has an op on /steps; Apply never reads them otherwise.
	Steps []DocStep
	// Followups are the states of the follow-ups of the run the patch names (message id to
	// state); an id that is absent is a follow-up the run does not have.
	Followups map[ids.ID]string
}

// Limits are the configuration keys of the patch rules.
type Limits struct {
	// ExcerptMaxChars is run.excerpt_max_chars: last_text_excerpt keeps that many runes.
	ExcerptMaxChars int
	// StepDetailMaxBytes is run.step_detail_max_bytes: the canonical JSON of a stored detail is
	// at most that long.
	StepDetailMaxBytes int
	// MaxStepsPersisted is run.max_steps_persisted: after a patch the run has at most that many
	// steps.
	MaxStepsPersisted int
}

// Env is what Apply needs from outside the patch: the time of the transaction (default started_at
// of a step) and the source of the id of a new summary step.
type Env struct {
	Now   time.Time
	NewID func() ids.ID
}

// TextColumn is the new value of a text column; Set false keeps the column, Value "" is NULL.
type TextColumn struct {
	Set   bool
	Value string
}

// FollowupWrite is one follow-up state to store through the port FollowupStates.
type FollowupWrite struct {
	MessageID ids.ID
	State     string
}

// Change is what one stored patch writes (Design note 1 f): the run columns that change, the
// step rows to delete, update and insert, the follow-up states, and the steps the adapter added
// (for bothy.run.steps; the summary step is not among them). After is the state at the new
// version.
type Change struct {
	StatusLine, LastTextExcerpt, Model TextColumn
	// Metrics and Plan are the new stored objects; nil keeps the column.
	Metrics, Plan json.RawMessage
	// Cost is the new cost; nil keeps the columns.
	Cost *Money
	// DeleteSteps are the ids of the steps the summarization removed, ascending.
	DeleteSteps []ids.ID
	// UpdateSteps are the changes of existing steps (also of the summary step), ascending by id
	// (lock order of 03); one entry per step.
	UpdateSteps []StepUpdate
	// InsertSteps are the new steps in ordinal order, the summary step included when it is new.
	InsertSteps []DocStep
	// Followups are the follow-up states to store, in op order.
	Followups []FollowupWrite
	// AddedSteps are the steps the adapter added in this patch, in op order.
	AddedSteps []DocStep
	// After is the document state after the patch.
	After DocumentState
}

// StepUpdate is the change of one existing step row: only the set members change.
type StepUpdate struct {
	ID ids.ID
	// Status and Title are nil when unchanged.
	Status, Title *string
	// Detail is the new stored object, already cut; nil when unchanged.
	Detail json.RawMessage
	// FinishedAt is nil when unchanged.
	FinishedAt *time.Time
	// Ordinal is nil when unchanged (only the summary step moves to a lower ordinal).
	Ordinal *int
}

// NormOp is one normalized RFC 6902 operation (Design note 4): Op add, replace or remove, Path
// the JSON Pointer, Value the stored value encoded as in the run document of getRun; nil for
// remove.
type NormOp struct {
	Op    OpKind
	Path  string
	Value json.RawMessage
}
