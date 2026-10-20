// SPDX-License-Identifier: Apache-2.0

package domain

import (
	"bytes"
	"encoding/json"
	"errors"
	"sort"
	"unicode/utf8"

	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/textcheck"
)

// Setter names who may set a reason of runs.state_reason (column "set by" of 09 5.2).
type Setter uint8

// The setters of 09 5.2. The zero value is invalid, so a forgotten field never reads as a setter.
const (
	// SetterServer: only the server sets the reason.
	SetterServer Setter = iota + 1
	// SetterAdapter: only the adapter reports the reason.
	SetterAdapter
	// SetterBoth: the adapter reports it or the server sets it ("Adapter or Server").
	SetterBoth
)

// String returns "server", "adapter" or "both", and "invalid" for any other value.
func (s Setter) String() string {
	switch s {
	case SetterServer:
		return "server"
	case SetterAdapter:
		return "adapter"
	case SetterBoth:
		return "both"
	}
	return "invalid"
}

// byServer reports whether the server may set a reason of this setter.
func (s Setter) byServer() bool { return s == SetterServer || s == SetterBoth }

// byAdapter reports whether an adapter may report a reason of this setter.
func (s Setter) byAdapter() bool { return s == SetterAdapter || s == SetterBoth }

// The codes of runs.state_reason (09 5.2 plus user_disabled, ruling D-M2-05-7). The prefix
// RunReason keeps them apart from the trigger rejection codes of type Reason (09 3.5), some of
// which share a value (agent_disabled, project_not_ready, billing_mode_personal_not_holder).
const (
	// RunReasonUserCancelled: a person stopped the run.
	RunReasonUserCancelled = "user_cancelled"
	// RunReasonAccessRevoked: the triggering person lost trigger (04 8.3).
	RunReasonAccessRevoked = "access_revoked"
	// RunReasonCancelTimeout: the adapter did not report the cancellation within agent.cancel_grace.
	RunReasonCancelTimeout = "cancel_timeout"
	// RunReasonAgentDisabled: the agent was disabled.
	RunReasonAgentDisabled = "agent_disabled"
	// RunReasonBillingModePersonalNotHolder: I2 denied the delivery.
	RunReasonBillingModePersonalNotHolder = "billing_mode_personal_not_holder"
	// RunReasonChannelReadOnly: the channel became read-only or archived (04 8.3).
	RunReasonChannelReadOnly = "channel_read_only"
	// RunReasonBillingModeMismatch: I3, reported by the adapter or found by the server.
	RunReasonBillingModeMismatch = "billing_mode_mismatch"
	// RunReasonToolClassOwnerOnly: SR2 at delivery or at run.token.renew.
	RunReasonToolClassOwnerOnly = "tool_class_owner_only"
	// RunReasonToolClassMismatch: the actual tool class is higher than the one of the token (SR2).
	RunReasonToolClassMismatch = "tool_class_mismatch"
	// RunReasonInvalidInvocationToken: the adapter found the invocation token invalid.
	RunReasonInvalidInvocationToken = "invalid_invocation_token"
	// RunReasonProjectNotReady: the project binding is not ready.
	RunReasonProjectNotReady = "project_not_ready"
	// RunReasonAgentUnavailable: the run stayed queued longer than run.queued_max_age.
	RunReasonAgentUnavailable = "agent_unavailable"
	// RunReasonInterrupted: the adapter no longer reports the run as active.
	RunReasonInterrupted = "interrupted"
	// RunReasonStale: no heartbeat since run.orphan_timeout while the run is active.
	RunReasonStale = "stale"
	// RunReasonRuntimeError: the runtime reported an error; details in the last step.
	RunReasonRuntimeError = "runtime_error"
	// RunReasonAdapterError: an error in the adapter itself, and the stored value of every adapter
	// reason 09 5.2 does not list for the adapter (ruling D-M2-05-8).
	RunReasonAdapterError = "adapter_error"
	// RunReasonRuntimeNotAuthenticated: the runtime has no valid login.
	RunReasonRuntimeNotAuthenticated = "runtime_not_authenticated"
	// RunReasonUserDisabled: the triggering person was disabled (user.disable, 02 4.3).
	RunReasonUserDisabled = "user_disabled"
)

// ReasonRow is one row of the reason table: a code, the end state it belongs to and who may set
// it.
type ReasonRow struct {
	// Code is the value of runs.state_reason.
	Code string
	// State is failed or cancelled; no other state carries a reason (03 runs).
	State State
	// Setter names who may set the code.
	Setter Setter
}

// reasonTable is 09 5.2 in the order of the spec, plus user_disabled last (ruling D-M2-05-7).
var reasonTable = []ReasonRow{
	{RunReasonUserCancelled, StateCancelled, SetterServer},
	{RunReasonAccessRevoked, StateCancelled, SetterServer},
	{RunReasonCancelTimeout, StateCancelled, SetterServer},
	{RunReasonAgentDisabled, StateCancelled, SetterServer},
	{RunReasonBillingModePersonalNotHolder, StateFailed, SetterServer},
	{RunReasonChannelReadOnly, StateCancelled, SetterServer},
	{RunReasonBillingModeMismatch, StateFailed, SetterBoth},
	{RunReasonToolClassOwnerOnly, StateFailed, SetterServer},
	{RunReasonToolClassMismatch, StateFailed, SetterBoth},
	{RunReasonInvalidInvocationToken, StateFailed, SetterAdapter},
	{RunReasonProjectNotReady, StateFailed, SetterServer},
	{RunReasonAgentUnavailable, StateFailed, SetterServer},
	{RunReasonInterrupted, StateFailed, SetterServer},
	{RunReasonStale, StateFailed, SetterServer},
	{RunReasonRuntimeError, StateFailed, SetterAdapter},
	{RunReasonAdapterError, StateFailed, SetterAdapter},
	{RunReasonRuntimeNotAuthenticated, StateFailed, SetterAdapter},
	{RunReasonUserDisabled, StateCancelled, SetterServer},
}

// ReasonTable returns the rows of 09 5.2 in the order of the spec, followed by user_disabled
// (cancelled, server), which 02 4.3 names for runs cancelled by user.disable (ruling
// D-M2-05-7). The slice is a copy: the caller may change it.
//
// Example:
//
//	for _, r := range domain.ReasonTable() {
//		fmt.Println(r.Code, r.State, r.Setter) // user_cancelled cancelled server ...
//	}
func ReasonTable() []ReasonRow {
	out := make([]ReasonRow, len(reasonTable))
	copy(out, reasonTable)
	return out
}

// ReasonOf returns the row of code and true, or a zero row and false for a code the table does
// not list. The match is exact: codes are lower case and never trimmed.
func ReasonOf(code string) (ReasonRow, bool) {
	for _, r := range reasonTable {
		if r.Code == code {
			return r, true
		}
	}
	return ReasonRow{}, false
}

// ErrReasonRequired is returned by NormalizeAdapterReason for an adapter failed without a
// reason. It is the adapter's mistake, not the server's: the caller maps it to 400
// validation_failed with the pointer /reason.
var ErrReasonRequired = errors.New("runs: a failed run needs a reason")

// ValidateServerReason checks the reason the server itself sets for a transition to the state to.
// For failed and cancelled, code must be a row of the reason table with State to and setter
// server or both. For queued, running, waiting_input and completed, code must be empty.
//
// Every violation, including an empty code for failed or cancelled and an unknown state, is an
// *errs.Error with errs.CodeInternalError: the server always knows why it ends a run, so a
// missing or wrong code is a programming error of the caller, never a client error (unlike
// ErrReasonRequired on the adapter path).
//
// Example:
//
//	err := domain.ValidateServerReason(domain.StateFailed, domain.RunReasonStale) // nil
//	err = domain.ValidateServerReason(domain.StateFailed, domain.RunReasonRuntimeError) // internal_error
func ValidateServerReason(to State, code string) error {
	switch to {
	case StateQueued, StateRunning, StateWaitingInput, StateCompleted:
		if code != "" {
			return errs.New(errs.CodeInternalError, "runs: a reason is set for a state without reasons")
		}
		return nil
	case StateFailed, StateCancelled:
		r, ok := ReasonOf(code)
		if !ok || r.State != to || !r.Setter.byServer() {
			return errs.New(errs.CodeInternalError, "runs: the server reason does not fit the target state")
		}
		return nil
	}
	return errs.New(errs.CodeInternalError, "runs: unknown target state")
}

// NormalizeAdapterReason turns the reason an adapter reports for failed into the stored reason
// (ruling D-M2-05-8). A code whose row is a failed row with the adapter as setter (adapter or
// both) is kept and original is empty. Every other non-empty value, including server-only codes
// such as stale, interrupted or agent_unavailable and the cancelled reasons, is stored as
// adapter_error, and original returns the code as reported, for the detail of a server step
// (see OriginalReasonDetail). An empty code gives ErrReasonRequired.
//
// Example:
//
//	r, orig, _ := domain.NormalizeAdapterReason("runtime_error")   // "runtime_error", ""
//	r, orig, _ = domain.NormalizeAdapterReason("quota_exhausted")  // "adapter_error", "quota_exhausted"
func NormalizeAdapterReason(code string) (reason, original string, err error) {
	if code == "" {
		return "", "", ErrReasonRequired
	}
	if r, ok := ReasonOf(code); ok && r.State == StateFailed && r.Setter.byAdapter() {
		return code, "", nil
	}
	return RunReasonAdapterError, code, nil
}

// originalReasonKey is the key of the step detail that keeps a replaced adapter reason.
const originalReasonKey = "original_reason"

// OriginalReasonDetail builds the detail of the server step kind = status, name = adapter_reason
// that keeps an adapter reason replaced by adapter_error: {"original_reason": code}. maxBytes is
// the configuration key run.step_detail_max_bytes; the code is cut at the longest valid UTF-8
// prefix whose detail, encoded with encoding/json without HTML escaping, fits maxBytes.
//
// It returns an empty map, never nil, when textcheck.DisplayText rejects the code (invalid UTF-8,
// control or bidi characters must not reach a person through the run view) and when maxBytes is
// too small even for an empty code.
//
// Example:
//
//	d := domain.OriginalReasonDetail("quota_exhausted", maxBytes) // {"original_reason": "quota_exhausted"}
func OriginalReasonDetail(code string, maxBytes int) map[string]any {
	if textcheck.DisplayText(code) != nil {
		return map[string]any{}
	}
	// The rune boundaries of code, ending with len(code); the encoded length grows with the
	// prefix, so the longest fitting prefix is found by binary search.
	bounds := make([]int, 0, utf8.RuneCountInString(code)+1)
	for i := range code {
		bounds = append(bounds, i)
	}
	bounds = append(bounds, len(code))
	n := sort.Search(len(bounds), func(i int) bool {
		return encodedDetailLen(code[:bounds[i]]) > maxBytes
	})
	if n == 0 {
		return map[string]any{}
	}
	return map[string]any{originalReasonKey: code[:bounds[n-1]]}
}

// encodedDetailLen is the length of {"original_reason": s} encoded like the step detail is
// stored: encoding/json without HTML escaping and without the trailing newline of Encode.
func encodedDetailLen(s string) int {
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	// A map of one string never fails to encode.
	_ = enc.Encode(map[string]any{originalReasonKey: s})
	return len(bytes.TrimSuffix(b.Bytes(), []byte("\n")))
}
