// SPDX-License-Identifier: Apache-2.0

package agents

import (
	"context"
	"encoding/json"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// The values of agent_sessions.end_reason (03 agent_sessions, "Meaning of end_reason").
const (
	EndReset        = "reset"
	EndRebind       = "rebind"
	EndImport       = "import"
	EndAgentChanged = "agent_changed"
)

// BindFailedSessionMissing is the only value of AgentSessionReport.bind_failed: the bound session
// no longer exists in the runtime and the adapter started a new one (09 9.5 item 4).
const BindFailedSessionMissing = "session_missing"

// The reasons of 409 conflict of the session operations (09 9.2, 9.4, 9.5; M2-12 design notes 7,
// 8, 11 and 12).
const (
	ReasonNoCurrentSession      = "no_current_session"
	ReasonSessionBoundElsewhere = "session_bound_elsewhere"
	ReasonSessionChanged        = "session_changed"
	ReasonSessionNotBound       = "session_not_bound"
)

// Session is one row of agent_sessions (03) as the module hands it out: every column the schema
// AgentSession shows plus CostBaselineMicros, which the API never shows (M2-12 design note 4).
// Zero values are NULL: Model "", EndedAt zero and EndReason "" for a current row.
// dev_environment_id stays NULL until V2-01 and is not carried.
type Session struct {
	ID, ThreadID, AgentID ids.ID
	// ExternalSessionRef is stored exactly as the adapter sent it; the server does not interpret it.
	ExternalSessionRef string
	// IsCurrent equals EndedAt.IsZero() (03 check is_current = (ended_at IS NULL)).
	IsCurrent bool
	// Started is true once the external session exists in the runtime.
	Started bool
	// Model is the last reported model; "" for NULL.
	Model string
	// CumulativeCostMicros, Currency and CostIsEstimate are the cost columns (booked by M2-22);
	// CostBaselineMicros is the amount of the external session before the binding (22 section 9
	// step 7).
	CumulativeCostMicros, CostBaselineMicros int64
	Currency                                 string
	CostIsEstimate                           bool
	// CreatedAt is the creation; EndedAt the end, zero while current.
	CreatedAt, EndedAt time.Time
	// EndReason is a value of end_reason; "" while current.
	EndReason string
	// Version is the ETag, the compare-and-swap of agent_session.report (03).
	Version int64
}

// BindCmd is the body of bindAgentSession (schema AgentSessionBind).
type BindCmd struct {
	// ExternalSessionRef is the session to bind.
	ExternalSessionRef string
	// ExpectedCurrentSessionID is the current session of the thread as the client saw it; zero
	// when the client saw none (the compare-and-swap of 09 9.4 point 3).
	ExpectedCurrentSessionID ids.ID
	// TakeOver allows taking the session over from another thread where it is current.
	TakeOver bool
}

// ReportCmd is the body of reportAgentSession (schema AgentSessionReport) with its If-Match.
type ReportCmd struct {
	// ExternalSessionRef is the reference the runtime uses.
	ExternalSessionRef string
	// Model is the reported model; "" when absent.
	Model string
	// Started reports that the runtime confirmed the session; false never resets it.
	Started bool
	// BindFailed is "" or BindFailedSessionMissing.
	BindFailed string
	// IfMatch is the compare-and-swap over agent_sessions.version; absent means "this run has
	// no session row yet" (M2-12 design note 11).
	IfMatch Precondition
}

// InvocationSessionQuery is the input of Sessions.ForInvocation: the thread, the operator whose
// run is about to be invoked and the person who triggered it (actor of the audit entry and of the
// events of an import binding).
type InvocationSessionQuery struct {
	ThreadID, AgentID, TriggeredBy ids.ID
}

// InvocationSession is the session decision of ForInvocation: the current row of the operator
// after the rules of 09 9.1 and 9.5, nil for none. runs decides the mode from it (runs/domain).
type InvocationSession struct {
	Row *Session
}

// AttachOnStartCmd is the input of Sessions.AttachOnStart: the run that starts (queued ->
// running), its agent, the row it was invoked with (runs.agent_session_id, zero for none) and the
// reference and model of run.start ("" when absent).
type AttachOnStartCmd struct {
	ThreadID, RunID, AgentID ids.ID
	// SessionID is the session row the invocation named (runs records it at the acceptance);
	// zero when the run was invoked without a row (session_mode new).
	SessionID                 ids.ID
	ExternalSessionRef, Model string
}

// Sessions is the incoming port of the agent sessions of a thread (09 section 9, M2-12): the four
// operations of the API, which authorize, and the two calls of runs, which do not (runs decided
// agent.invoke and run.start). Every write runs in a unit of work; ForInvocation and AttachOnStart
// need the caller's unit of work. Errors are kernel/errs errors; an unknown and an invisible
// thread give the same 404 not_found.
type Sessions interface {
	// List returns the sessions of the thread newest first (created_at DESC, id DESC),
	// agent_session.read. Errors: not_found.
	List(ctx context.Context, a actor.Actor, threadID ids.ID) ([]Session, error)
	// Reset ends the current session of the thread with reset and returns the ended row
	// (agent_session.reset). Errors: not_found, forbidden, agent_invocation_denied, conflict with
	// reason no_current_session.
	Reset(ctx context.Context, a actor.Actor, threadID ids.ID) (Session, error)
	// Bind binds an external session to the thread and returns the new current row
	// (agent_session.bind). Errors: not_found, forbidden, agent_invocation_denied,
	// validation_failed, conflict with reason session_changed or session_bound_elsewhere.
	Bind(ctx context.Context, a actor.Actor, threadID ids.ID, cmd BindCmd) (Session, error)
	// Report applies a report of the adapter a for the session of its active run in the thread
	// (agent_session.report). Errors: not_found, forbidden, validation_failed, version_conflict,
	// conflict with reason session_bound_elsewhere, session_changed or session_not_bound.
	Report(ctx context.Context, a actor.Actor, threadID ids.ID, cmd ReportCmd) (Session, error)
	// ForInvocation decides the session of a new run of the thread's operator inside the unit of
	// work that appends agent.invocation (09 9.1, 9.5; M2-12 design note 9): it ends the current
	// row of another agent with agent_changed, binds an import row once, and returns the current
	// row of q.AgentID or none. It never authorizes.
	//
	// Example:
	//
	//	s, err := sessions.ForInvocation(ctx, agents.InvocationSessionQuery{ThreadID: t, AgentID: op, TriggeredBy: alex})
	ForInvocation(ctx context.Context, q InvocationSessionQuery) (InvocationSession, error)
	// AttachOnStart returns the session row a starting operator run attaches to (M2-12 design note
	// 10): the row it was invoked with when it has one, otherwise the row created from the
	// reference of run.start; zero for a guest run and for a start without row and reference. It
	// never changes an existing row.
	// Errors: validation_failed with pointer /external_session_ref for an invalid reference.
	AttachOnStart(ctx context.Context, cmd AttachOnStartCmd) (sessionID ids.ID, err error)
	// CheckStartModel validates the model of run.start and resumeRun before the run changes (M2-12
	// design note 13): the pattern of default_model and at most agent.session_ref_max_bytes bytes,
	// for every run, so runs.model never stores a value the session rules refuse. "" passes. It
	// touches no database.
	// Errors: validation_failed with pointer /model.
	//
	// Example:
	//
	//	err := sessions.CheckStartModel("model-large")
	CheckStartModel(model string) error
}

// ActiveRun is the active run of a thread as runs reports it to agents (R-DM-14): its id, agent,
// session row (runs.agent_session_id, zero for NULL) and state (queued, running or waiting_input).
type ActiveRun struct {
	RunID, AgentID, SessionID ids.ID
	State                     string
}

// RunSource is the declared source of the active run of a thread (02 section 4.4; M2-12 design
// note 2): runs implements it in its postgres adapter and internal/wire wires it; agents may not
// use runs. ActiveRun reads the run of the thread in queued, running or waiting_input with its row
// locked FOR UPDATE for the rest of the unit of work; found is false for none.
type RunSource interface {
	ActiveRun(ctx context.Context, threadID ids.ID) (ActiveRun, bool, error)
}

// SessionAttached is the in-transaction event of a report that created the session row of a run
// or replaced it after bind_failed (02 section 4.3; M2-12 design notes 11 and 12). runs subscribes:
// it sets runs.agent_session_id = SessionID for RunID and, when BindFailed, writes the system line
// agent_session.resume_failed with AgentID as author (R-DM-42).
type SessionAttached struct {
	ThreadID, RunID, AgentID, SessionID ids.ID
	// ReplacedSessionID is the row that bind_failed ended; zero otherwise.
	ReplacedSessionID ids.ID
	// BindFailed marks the fail open of 09 9.5 item 4.
	BindFailed bool
}

// sessionJSON is the schema AgentSession of docs/api/openapi.yaml; optional members are left out,
// never null (ADR 0053). cost_baseline_micros and dev_environment_id are never shown.
type sessionJSON struct {
	ID                 string    `json:"id"`
	ThreadID           string    `json:"thread_id"`
	AgentID            string    `json:"agent_id"`
	ExternalSessionRef string    `json:"external_session_ref"`
	IsCurrent          bool      `json:"is_current"`
	Started            bool      `json:"started"`
	Model              string    `json:"model,omitempty"`
	Cost               moneyJSON `json:"cost"`
	CreatedAt          string    `json:"created_at"`
	EndedAt            string    `json:"ended_at,omitempty"`
	EndReason          string    `json:"end_reason,omitempty"`
	Version            int64     `json:"version"`
}

// moneyJSON is the schema Money.
type moneyJSON struct {
	AmountMicros int64  `json:"amount_micros"`
	Currency     string `json:"currency"`
	IsEstimate   bool   `json:"is_estimate"`
}

// sessionTimeLayout is the form of every API time (06 section 2): UTC, milliseconds, suffix Z.
const sessionTimeLayout = "2006-01-02T15:04:05.000Z"

// sessionStamp formats t; the zero time is absent.
func sessionStamp(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Truncate(time.Millisecond).Format(sessionTimeLayout)
}

// SessionJSON encodes s in the form of the schema AgentSession (M2-12 design note 4): the one
// representation of a session row in the payload of agent_session.changed (agents) and in the
// member agent_session of AgentInvocationPayload (runs), so both carry the same bytes. cost is
// Money{amount_micros: cumulative_cost_micros, currency, is_estimate: cost_is_estimate}.
// Errors: none in practice (the encoding of plain values).
//
// Example:
//
//	raw, err := agents.SessionJSON(row)
func SessionJSON(s Session) ([]byte, error) {
	return json.Marshal(sessionJSON{
		ID: s.ID.String(), ThreadID: s.ThreadID.String(), AgentID: s.AgentID.String(), ExternalSessionRef: s.ExternalSessionRef,
		IsCurrent: s.IsCurrent, Started: s.Started, Model: s.Model,
		Cost:      moneyJSON{AmountMicros: s.CumulativeCostMicros, Currency: s.Currency, IsEstimate: s.CostIsEstimate},
		CreatedAt: sessionStamp(s.CreatedAt), EndedAt: sessionStamp(s.EndedAt), EndReason: s.EndReason, Version: s.Version,
	})
}
