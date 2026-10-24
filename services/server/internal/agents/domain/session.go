// SPDX-License-Identifier: Apache-2.0

package domain

import (
	"bytes"
	"errors"
	"strings"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// The values of agent_sessions.end_reason (03 agent_sessions, "Meaning of end_reason").
const (
	EndReset        = "reset"
	EndRebind       = "rebind"
	EndImport       = "import"
	EndAgentChanged = "agent_changed"
)

// BindFailedSessionMissing is the only value of bind_failed (09 9.5 item 4).
const BindFailedSessionMissing = "session_missing"

// ReasonSessionNotBound is the reason of 409 conflict of a bind_failed report without a started
// session row (M2-12 design note 12).
const ReasonSessionNotBound = "session_not_bound"

// The JSON pointers of the session bodies (M2-12 design note 13).
const (
	PointerExternalSessionRef = "/external_session_ref"
	PointerModel              = "/model"
	PointerBindFailed         = "/bind_failed"
)

// Session is one row of agent_sessions (03) as the rules see it; the app layer converts it from
// and to agents.Session. Zero values are NULL.
type Session struct {
	ID, ThreadID, AgentID ids.ID
	ExternalSessionRef    string
	IsCurrent, Started    bool
	Model                 string
	// CumulativeCostMicros, CostBaselineMicros, Currency and CostIsEstimate are the cost columns.
	CumulativeCostMicros, CostBaselineMicros int64
	Currency                                 string
	CostIsEstimate                           bool
	CreatedAt, EndedAt                       time.Time
	EndReason                                string
	Version                                  int64
}

// ErrSessionEnded is returned by EndSession for a row that is no longer current: a row ends
// once (03 check is_current = (ended_at IS NULL)).
var ErrSessionEnded = errors.New("agents: the session has already ended")

// errEndReason is returned by EndSession for a value outside end_reason.
var errEndReason = errors.New("agents: unknown end reason")

// EndSession returns s ended at at with reason (03 agent_sessions): ended_at set, is_current
// false, end_reason, version + 1. Errors: ErrSessionEnded for a row already ended; an error for a
// reason outside the CHECK of 03 (a programming error of the caller).
//
// Example:
//
//	ended, err := domain.EndSession(cur, domain.EndReset, now)
func EndSession(s Session, reason string, at time.Time) (Session, error) {
	switch reason {
	case EndReset, EndRebind, EndImport, EndAgentChanged:
	default:
		return Session{}, errEndReason
	}
	if !s.IsCurrent || !s.EndedAt.IsZero() {
		return Session{}, ErrSessionEnded
	}
	s.IsCurrent, s.EndedAt, s.EndReason, s.Version = false, at, reason, s.Version+1
	return s, nil
}

// SessionReport is the body of agent_session.report as the rules see it (schema AgentSessionReport):
// Model "" and BindFailed "" are absent members.
type SessionReport struct {
	ExternalSessionRef string
	Model              string
	Started            bool
	BindFailed         string
}

// ReportKind is the outcome of ApplyReport.
type ReportKind int

// The outcomes of a report on an existing session row (M2-12 design notes 11 and 12).
const (
	// ReportUnchanged: the report changes nothing; 200 with the row, no new version, no event.
	ReportUnchanged ReportKind = iota
	// ReportChanged: reference, model or started change in place; version + 1 and an event.
	ReportChanged
	// ReportBindFailed: the bound session is missing in the runtime (fail open, 09 9.5 item 4).
	ReportBindFailed
)

// ReportOutcome is the result of ApplyReport: the kind and, for ReportChanged, the values to
// store. RefChanged reports a new reference, which the caller checks against the partial unique
// index (session_bound_elsewhere).
type ReportOutcome struct {
	Kind               ReportKind
	ExternalSessionRef string
	Model              string
	Started            bool
	RefChanged         bool
}

// ApplyReport applies r to the session row s of the reporting run (M2-12 design note 11): a
// different reference replaces the stored one; a given, different model replaces it; started
// true sets started; started false never resets it. With bind_failed the row must be started
// (design note 12). Errors: 409 conflict with reason session_not_bound for bind_failed on a row
// that is not started.
//
// Example:
//
//	out, err := domain.ApplyReport(row, domain.Report{ExternalSessionRef: "sess-7f3a", Started: true})
func ApplyReport(s Session, r SessionReport) (ReportOutcome, error) {
	if r.BindFailed != "" {
		if !s.Started {
			return ReportOutcome{}, NotBound()
		}
		return ReportOutcome{Kind: ReportBindFailed, ExternalSessionRef: r.ExternalSessionRef, Model: r.Model, Started: r.Started}, nil
	}
	out := ReportOutcome{Kind: ReportUnchanged, ExternalSessionRef: s.ExternalSessionRef, Model: s.Model, Started: s.Started}
	if r.ExternalSessionRef != s.ExternalSessionRef {
		out.ExternalSessionRef, out.RefChanged, out.Kind = r.ExternalSessionRef, true, ReportChanged
	}
	if r.Model != "" && r.Model != s.Model {
		out.Model, out.Kind = r.Model, ReportChanged
	}
	if r.Started && !s.Started {
		out.Started, out.Kind = true, ReportChanged
	}
	return out, nil
}

// NotBound is the 409 conflict with reason session_not_bound: bind_failed without a started
// session row of the run.
func NotBound() error {
	return errs.New(errs.CodeConflict, "the run has no bound session").WithReason(ReasonSessionNotBound)
}

// ImportBindingCandidate returns the import row the first trigger of agentID binds (09 9.5, 22
// section 9; M2-12 design note 9 d): only when the thread has no row other than rows with
// end_reason import ("has had no current session since the import", so a later reset never binds
// again), the newest import row of agentID by created_at, then id, descending. ok is false
// otherwise.
//
// Example:
//
//	row, ok := domain.ImportBindingCandidate(history, operator)
func ImportBindingCandidate(rows []Session, agentID ids.ID) (Session, bool) {
	var best Session
	found := false
	for _, r := range rows {
		if r.EndReason != EndImport || r.IsCurrent {
			return Session{}, false
		}
		if r.AgentID != agentID {
			continue
		}
		if !found || newer(r, best) {
			best, found = r, true
		}
	}
	return best, found
}

// newer reports whether a is newer than b by created_at, then id.
func newer(a, b Session) bool {
	if !a.CreatedAt.Equal(b.CreatedAt) {
		return a.CreatedAt.After(b.CreatedAt)
	}
	return bytes.Compare(a.ID[:], b.ID[:]) > 0
}

// CostBaseline returns the cost baseline and currency of a row a bind creates (M2-12 design note
// 14; 03 cost_baseline_micros): cost_baseline_micros + cumulative_cost_micros of latest, the most
// recent row of the same agent and reference in any thread, with its currency; latest nil gives 0
// and "" (the column default). So a runtime that reports sums per session is not booked twice
// after a rebind.
//
// Example:
//
//	micros, currency := domain.CostBaseline(&previous)
func CostBaseline(latest *Session) (int64, string) {
	if latest == nil {
		return 0, ""
	}
	return latest.CostBaselineMicros + latest.CumulativeCostMicros, latest.Currency
}

// CheckExternalSessionRef validates an external session reference (bind, report, run.start;
// M2-12 design note 13): not empty and not only white space (code required), a display text under
// SEC-IN-4 (codes invalid_utf8, control_character, bidi_control), at most maxBytes bytes
// (agent.session_ref_max_bytes, code too_long). The reference is stored exactly as sent. Errors:
// validation_failed on /external_session_ref; the error never repeats the input.
//
// Example:
//
//	err := domain.CheckExternalSessionRef(ref, cfg.SessionRefMaxBytes)
func CheckExternalSessionRef(ref string, maxBytes int) error {
	if strings.TrimSpace(ref) == "" {
		return fieldError(PointerExternalSessionRef, FieldRequired, "the external session reference is required")
	}
	if err := displayText(PointerExternalSessionRef, "external session reference", ref, true); err != nil {
		return err
	}
	if len(ref) > maxBytes {
		return fieldError(PointerExternalSessionRef, FieldTooLong, "the external session reference is longer than agent.session_ref_max_bytes")
	}
	return nil
}

// CheckModel validates a reported model (report, run.start): the pattern of default_model of
// agent_operator_config.v1 (09 section 2.2) and at most maxBytes bytes
// (agent.session_ref_max_bytes): the model is shown in the session column and carried in every
// agent.invocation of the thread, so an unbounded one would push those events past
// sync.event_payload_max_bytes. "" is an absent model and passes. Errors: validation_failed on
// /model with code pattern or too_long.
//
// Example:
//
//	err := domain.CheckModel("model-large", cfg.SessionRefMaxBytes)
func CheckModel(model string, maxBytes int) error {
	if model == "" {
		return nil
	}
	if !modelPattern.MatchString(model) {
		return fieldError(PointerModel, FieldPattern, "the model is not a model name")
	}
	if len(model) > maxBytes {
		return fieldError(PointerModel, FieldTooLong, "the model is longer than agent.session_ref_max_bytes")
	}
	return nil
}

// CheckBindFailed validates bind_failed: "" or session_missing. Errors: validation_failed on
// /bind_failed with code invalid.
func CheckBindFailed(v string) error {
	if v != "" && v != BindFailedSessionMissing {
		return fieldError(PointerBindFailed, FieldInvalid, "bind_failed is not a known value")
	}
	return nil
}

// The pointers of an earlier session of the import (M8-06 design note 2), relative to the member
// earlier_session of the thread item.
const (
	PointerAgentID            = "/agent_id"
	PointerCostBaselineMicros = "/cost_baseline_micros"
	PointerCurrency           = "/currency"
)

// MaxCostBaselineMicros is the largest cost baseline: 2^53 - 1, the largest integer every JSON
// reader keeps exactly, as for the session sums (M2-22 design note 5 g).
const MaxCostBaselineMicros = 1<<53 - 1

// Earlier is an earlier session reference of an import as the rules see it (22 section 9 step 1).
type Earlier struct {
	ThreadID, AgentID  ids.ID
	ExternalSessionRef string
	CostBaselineMicros int64
	// Currency is "" for the column default.
	Currency string
}

// CheckEarlier validates e for the thread whose operator is operator (M8-06 design note 2): the
// agent is the operator (code invalid at /agent_id), the reference passes CheckExternalSessionRef,
// the baseline lies in 0..MaxCostBaselineMicros (code invalid), and the currency is an ISO 4217
// code (code pattern), required for a baseline above 0. Errors: one validation_failed with every
// violation.
//
// Example:
//
//	err := domain.CheckEarlier(e, operator, maxBytes)
func CheckEarlier(e Earlier, operator ids.ID, maxBytes int) error {
	var fields []errs.FieldError
	add := func(err error) {
		if ee, ok := errs.As(err); ok {
			fields = append(fields, ee.Fields...)
		}
	}
	if operator.IsZero() || e.AgentID != operator {
		add(fieldError(PointerAgentID, FieldInvalid, "the agent is not the operator of the thread"))
	}
	add(CheckExternalSessionRef(e.ExternalSessionRef, maxBytes))
	if e.CostBaselineMicros < 0 || e.CostBaselineMicros > MaxCostBaselineMicros {
		add(fieldError(PointerCostBaselineMicros, FieldInvalid, "the cost baseline must lie between 0 and 2^53 - 1"))
	}
	switch {
	case e.Currency != "" && !isoCurrency(e.Currency):
		add(fieldError(PointerCurrency, FieldPattern, "the currency is an ISO 4217 code of three capital letters"))
	case e.Currency == "" && e.CostBaselineMicros > 0:
		add(fieldError(PointerCurrency, FieldRequired, "a cost baseline above 0 needs its currency"))
	}
	if len(fields) == 0 {
		return nil
	}
	return errs.New(errs.CodeValidationFailed, "the earlier session is invalid").WithFields(fields...)
}

// isoCurrency reports three ASCII capital letters.
func isoCurrency(s string) bool {
	if len(s) != 3 {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] < 'A' || s[i] > 'Z' {
			return false
		}
	}
	return true
}

// EarlierOutcome decides an earlier session against the rows of its thread (22 section 9 steps 1
// and 6; M8-06 design note 2): "dropped_late" when a row is current or has an end_reason other
// than import (the thread has had a current session since the import), "already_present" when an
// import row of the same agent and reference exists, "" when the row is to be written.
//
// Example:
//
//	outcome := domain.EarlierOutcome(rows, agentID, ref)
func EarlierOutcome(rows []Session, agentID ids.ID, ref string) string {
	present := false
	for _, r := range rows {
		if r.IsCurrent || r.EndReason != EndImport {
			return earlierDroppedLate
		}
		if r.AgentID == agentID && r.ExternalSessionRef == ref {
			present = true
		}
	}
	if present {
		return earlierAlreadyPresent
	}
	return ""
}

// The outcomes of EarlierOutcome; agents.EarlierDroppedLate and agents.EarlierAlreadyPresent carry
// the same values.
const (
	earlierDroppedLate    = "dropped_late"
	earlierAlreadyPresent = "already_present"
)
