// SPDX-License-Identifier: Apache-2.0

// Package errs is the error model of the server: every error that may reach a client carries one
// stable public code from the closed list of docs/spec/06-api.md, and the kind (with its HTTP
// status) follows from that code, never from the caller (docs/spec/02-architektur.md, section
// 14.1). Internal details such as causes and wrapped driver errors go to the log only, so the
// text of an error never leaks them into a response.
//
// Example:
//
//	err := errs.Wrap(errs.CodeUnavailable, dbErr, "database not reachable").
//		WithCause(errs.CauseDBPoolExhausted).
//		WithRetryAfter(5 * time.Second)
//	status := errs.KindOf(err).Status() // 503
package errs

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"slices"
	"strconv"
	"time"
)

// Kind groups public codes that share one HTTP status and one client reaction. The zero value
// KindNone means "no error" and is what KindOf returns for a nil error.
type Kind uint8

// The kinds of docs/spec/02-architektur.md section 14.1, in the order of that table. New kinds are
// appended before the end so that the lookup table below stays aligned.
const (
	// KindNone is the zero value: no error. Status 0.
	KindNone Kind = iota
	// KindBadRequest is a malformed or invalid request (400).
	KindBadRequest
	// KindUnauthenticated means no valid principal (401).
	KindUnauthenticated
	// KindStepUpRequired asks for a fresh confirmation of the person (401).
	KindStepUpRequired
	// KindForbidden means the principal lacks a level, capability or scope (403).
	KindForbidden
	// KindNotFound covers missing and invisible resources alike (404).
	KindNotFound
	// KindMethodNotAllowed is an undefined method on a path (405).
	KindMethodNotAllowed
	// KindConflict means the current state prevents the change (409).
	KindConflict
	// KindInvariantViolation means the change would break a domain invariant (409).
	KindInvariantViolation
	// KindGone means the object or the catch-up window no longer exists (410).
	KindGone
	// KindPreconditionFailed is an If-Match that does not match (412).
	KindPreconditionFailed
	// KindTooLarge is a body or file above its limit (413).
	KindTooLarge
	// KindUnsupportedMediaType is a media type that is not allowed (415).
	KindUnsupportedMediaType
	// KindRangeNotSatisfiable is a Range outside the file (416).
	KindRangeNotSatisfiable
	// KindUnprocessable is a reused idempotency key with different content (422).
	KindUnprocessable
	// KindPreconditionRequired is a missing If-Match (428).
	KindPreconditionRequired
	// KindRateLimited means a rate-limit class is exhausted (429).
	KindRateLimited
	// KindInternal is an unexpected error; also the kind of every error that is not an *Error (500).
	KindInternal
	// KindUnavailable means the server or a dependency is not ready (503).
	KindUnavailable
)

// kindInfo is one row of the kind table; a table instead of a switch keeps name and status of a
// kind on one line and needs no default branch.
type kindInfo struct {
	name   string
	status int
}

// kinds is indexed by Kind; every constant above has a row.
var kinds = [...]kindInfo{
	KindNone:                 {"none", 0},
	KindBadRequest:           {"bad_request", 400},
	KindUnauthenticated:      {"unauthenticated", 401},
	KindStepUpRequired:       {"step_up_required", 401},
	KindForbidden:            {"forbidden", 403},
	KindNotFound:             {"not_found", 404},
	KindMethodNotAllowed:     {"method_not_allowed", 405},
	KindConflict:             {"conflict", 409},
	KindInvariantViolation:   {"invariant_violation", 409},
	KindGone:                 {"gone", 410},
	KindPreconditionFailed:   {"precondition_failed", 412},
	KindTooLarge:             {"too_large", 413},
	KindUnsupportedMediaType: {"unsupported_media_type", 415},
	KindRangeNotSatisfiable:  {"range_not_satisfiable", 416},
	KindUnprocessable:        {"unprocessable", 422},
	KindPreconditionRequired: {"precondition_required", 428},
	KindRateLimited:          {"rate_limited", 429},
	KindInternal:             {"internal", 500},
	KindUnavailable:          {"unavailable", 503},
}

// Status returns the HTTP status of the kind, or 0 for KindNone and for values outside the
// defined kinds.
func (k Kind) Status() int {
	if int(k) < len(kinds) {
		return kinds[k].status
	}
	return 0
}

// String returns the snake_case name of the kind (for example "not_found"), "none" for KindNone
// and "kind(<n>)" for a value outside the defined kinds, so a corrupt value stays visible in logs.
func (k Kind) String() string {
	if int(k) < len(kinds) {
		return kinds[k].name
	}
	return "kind(" + strconv.Itoa(int(k)) + ")"
}

// Internal causes of docs/spec/02-architektur.md section 14.1. A cause is a label for logs and
// metrics and is never serialized into a response; untyped so any string field accepts it.
const (
	// CauseAuthzNotChecked marks a handler that returned without an authorization decision (500).
	CauseAuthzNotChecked = "authz_not_checked"
	// CauseUnknownAction marks an action name that is missing from the action register (500).
	CauseUnknownAction = "unknown_action"
	// CauseDBPoolExhausted marks that no database connection was free within database.acquire_timeout (503).
	CauseDBPoolExhausted = "db_pool_exhausted"
	// CauseShuttingDown marks that the server is draining and takes no new work (503).
	CauseShuttingDown = "shutting_down"
	// CauseDeadlineExceeded marks that the request context ran out (503).
	CauseDeadlineExceeded = "deadline_exceeded"
	// CauseDependencyUnavailable marks that a required dependency does not answer (503).
	CauseDependencyUnavailable = "dependency_unavailable"
)

// FieldError names one invalid field of a request for the errors[] member of a problem body.
type FieldError struct {
	// Pointer is the JSON Pointer of the field, for example "/name".
	Pointer string
	// Code is the machine-readable rule the field violates, for example "required".
	Code string
	// Detail is the human-readable explanation.
	Detail string
}

// Error is a server error with a public code. The public code, Detail, Reason, Hint, Fields,
// Current and RetryAfter may reach the client as the problem body of 06; Cause and the wrapped
// error are for logs only. A response takes its code from PublicCode and its status from Kind,
// never from the Code field, so a code outside the closed list never reaches a client. Build it
// with New or Wrap so that the code is checked.
type Error struct {
	// Code is the public code from 06; it determines the kind. Responses read it through
	// PublicCode, which replaces a code without a kind by internal_error.
	Code Code
	// Detail is a human-readable explanation for this occurrence.
	Detail string
	// Reason refines the code where 06 defines reasons (for example "channel_archived").
	Reason string
	// Hint names the way out where 06 asks for one (for example for i1_violation).
	Hint string
	// Cause is an internal label for logs and metrics (one of the Cause constants); never sent. The
	// json tag keeps it out if an encoder ever writes an *Error directly.
	Cause string `json:"-"`
	// Fields lists invalid fields for validation_failed.
	Fields []FieldError
	// Current carries the current state for conflicts such as version_conflict.
	Current any
	// RetryAfter becomes the Retry-After header when positive.
	RetryAfter time.Duration
	// Headers are response headers the contract asks for with this error (WWW-Authenticate,
	// DPoP-Nonce, Allow); the problem writer of platform/httpserver applies them. Never part of the
	// body. Set them with WithHeader.
	Headers http.Header `json:"-"`

	err error
}

// Compile-time check: loggers render an *Error through LogValue, including the internal fields.
var _ slog.LogValuer = (*Error)(nil)

// New returns an error with the given code and detail. It panics if the code has no kind, because
// an unknown code is a programming error that a test must catch, not a runtime condition.
func New(code Code, detail string) *Error {
	mustKnow(code)
	return &Error{Code: code, Detail: detail}
}

// Wrap is New plus an underlying error that Unwrap returns and LogValue records; err may be nil.
// It panics if the code has no kind.
func Wrap(code Code, err error, detail string) *Error {
	mustKnow(code)
	return &Error{Code: code, Detail: detail, err: err}
}

func mustKnow(code Code) {
	if code.Kind() == KindNone {
		panic("errs: code " + strconv.Quote(string(code)) + " has no kind; add it to the table in codes.go")
	}
}

// WithReason sets Reason and returns the receiver for chaining.
func (e *Error) WithReason(reason string) *Error {
	e.Reason = reason
	return e
}

// WithHint sets Hint and returns the receiver for chaining.
func (e *Error) WithHint(hint string) *Error {
	e.Hint = hint
	return e
}

// WithCause sets the internal Cause (a Cause constant) and returns the receiver for chaining.
func (e *Error) WithCause(cause string) *Error {
	e.Cause = cause
	return e
}

// WithFields appends field errors, so validators can add them step by step, and returns the
// receiver for chaining.
func (e *Error) WithFields(fields ...FieldError) *Error {
	e.Fields = append(e.Fields, fields...)
	return e
}

// WithCurrent sets Current and returns the receiver for chaining.
func (e *Error) WithCurrent(current any) *Error {
	e.Current = current
	return e
}

// WithRetryAfter sets RetryAfter and returns the receiver for chaining.
func (e *Error) WithRetryAfter(d time.Duration) *Error {
	e.RetryAfter = d
	return e
}

// WithHeader adds the response header name with value (canonicalised like http.Header.Add) and
// returns the receiver for chaining. Use it for headers the contract ties to an error:
// WWW-Authenticate and DPoP-Nonce with 401, Allow with 405. Retry-After has its own field.
//
// Example:
//
//	err := errs.New(errs.CodeUseDPoPNonce, "").
//		WithHeader("WWW-Authenticate", `DPoP error="use_dpop_nonce"`).
//		WithHeader("DPoP-Nonce", nonce)
func (e *Error) WithHeader(name, value string) *Error {
	if e.Headers == nil {
		e.Headers = http.Header{}
	}
	e.Headers.Add(name, value)
	return e
}

// VersionConflict returns 412 version_conflict with current as the current representation (or at
// least its id and version) of the resource (06 "Nebenläufigkeit" item 2). current must encode to
// a JSON object; nil leaves the member out.
func VersionConflict(current any) *Error {
	return New(CodeVersionConflict, "").WithCurrent(current)
}

// PreconditionRequired returns 428 precondition_required for an operation that requires If-Match
// and got none, or got the wildcard "*" (06 "Nebenläufigkeit" item 2, docs/DECISIONS.md M1-18).
func PreconditionRequired() *Error {
	return New(CodePreconditionRequired, "")
}

// Kind returns the kind of the code. It never returns KindNone, because a non-nil error must
// never map to status 0 (net/http panics on WriteHeader(0)): a nil *Error and an *Error whose
// Code has no kind (a zero Error, a struct literal or a Code changed after New) report
// KindInternal, like KindOf does for errors that are not an *Error.
func (e *Error) Kind() Kind {
	if e == nil {
		return KindInternal
	}
	if k := e.Code.Kind(); k != KindNone {
		return k
	}
	return KindInternal
}

// PublicCode returns the code a response may carry: Code when it is in the closed list of 06,
// otherwise CodeInternalError, also for a nil *Error. It matches Kind, so a problem writer that
// reads both never sends a code outside the list or a code whose status disagrees with the kind.
func (e *Error) PublicCode() Code {
	if e == nil || e.Code.Kind() == KindNone {
		return CodeInternalError
	}
	return e.Code
}

// Unwrap returns the wrapped error, or nil.
func (e *Error) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.err
}

// Error returns "<code>" or "<code>: <detail>" with the code of PublicCode. It never contains the
// cause, reason, hint, the wrapped error or a code outside the closed list, because error texts
// end up in places a response can reach.
func (e *Error) Error() string {
	if e == nil {
		return "<nil>"
	}
	code := string(e.PublicCode())
	if e.Detail == "" {
		return code
	}
	return code + ": " + e.Detail
}

// MarshalJSON writes the exported fields under their own names, with the code of PublicCode as
// Error() does, so an encoder that meets an Error directly never writes a code outside the closed
// list; Cause and the wrapped error stay out. The value receiver also covers an Error value behind
// an interface; encoding/json writes null for a nil *Error without calling it.
func (e Error) MarshalJSON() ([]byte, error) {
	type fields Error // same fields and tags without this method, so Marshal does not recurse
	f := fields(e)
	f.Code = e.PublicCode()
	return json.Marshal(f)
}

// LogValue renders the error for log/slog as a group with code, kind (always present), detail,
// reason, hint, cause and the wrapped error's text, each only when set. This is the one place the
// internal fields become visible. The code is the raw Code field, not PublicCode, so a log shows
// which unknown code turned into internal_error.
func (e *Error) LogValue() slog.Value {
	if e == nil {
		return slog.GroupValue()
	}
	attrs := []slog.Attr{
		slog.String(logKeyCode, string(e.Code)),
		slog.String(logKeyKind, e.Kind().String()),
		slog.String(logKeyDetail, e.Detail),
		slog.String(logKeyReason, e.Reason),
		slog.String(logKeyHint, e.Hint),
		slog.String(logKeyCause, e.Cause),
	}
	if e.err != nil {
		attrs = append(attrs, slog.String(logKeyError, e.err.Error()))
	}
	// Unset fields stay out; kind and the wrapped error are always written as they are.
	attrs = slices.DeleteFunc(attrs, func(a slog.Attr) bool {
		return a.Key != logKeyKind && a.Key != logKeyError && a.Value.String() == ""
	})
	return slog.GroupValue(attrs...)
}

// The keys of the group LogValue writes. They are constants because the linter sloglint
// (no-raw-keys, docs/observability/signals.md "Logs") accepts no other keys.
const (
	logKeyCode   = "code"
	logKeyKind   = "kind"
	logKeyDetail = "detail"
	logKeyReason = "reason"
	logKeyHint   = "hint"
	logKeyCause  = "cause"
	logKeyError  = "error"
)

// As finds the first *Error in the chain of err (through fmt.Errorf %w and errors.Join). It
// reports false for nil, for chains without an *Error and for a typed nil *Error, so a true
// result is always safe to dereference.
func As(err error) (*Error, bool) {
	var e *Error
	if errors.As(err, &e) && e != nil {
		return e, true
	}
	return nil, false
}

// KindOf returns the kind of err: KindNone for nil, the kind of the first *Error in the chain,
// and KindInternal for anything else, including a typed nil *Error and an *Error whose Code has
// no kind. So KindNone (status 0) comes back only for a nil error. Handlers use it to choose the
// status without knowing the concrete error.
func KindOf(err error) Kind {
	if err == nil {
		return KindNone
	}
	if e, ok := As(err); ok {
		return e.Kind()
	}
	return KindInternal
}

// ProblemExtender is implemented by errors that carry extension members of the problem body
// (RFC 9457 section 3.2) beyond the members of Error, for example the conflicts of i1_violation
// (docs/spec/10-backend-richtlinie.md section 5.4). The problem writer of platform/httpserver
// finds it anywhere in the chain of the error (errors.As) and merges the members into the body;
// a member never replaces a standard member of the problem, and members with a nil value are left
// out, because the server never sends JSON null (ADR 0053). An internal error never carries
// extensions.
//
// Example:
//
//	func (e *InvariantError) ProblemExtensions() map[string]any {
//		return map[string]any{"conflicts": e.conflictsJSON()}
//	}
type ProblemExtender interface {
	// ProblemExtensions returns the extension members by name; nil or empty for none. The values
	// must marshal to JSON.
	ProblemExtensions() map[string]any
}

// memberAttemptsRemaining is the problem member of fingerprint_mismatch (M2-41).
const memberAttemptsRemaining = "attempts_remaining"

// attemptsError is fingerprint_mismatch with the remaining confirmation attempts of a host; the
// problem writer adds them as attempts_remaining through ProblemExtender.
type attemptsError struct {
	e         *Error
	remaining int
}

// FingerprintMismatch returns 409 fingerprint_mismatch with the problem member attempts_remaining
// = remaining (docs/api/openapi.yaml Problem, 24 section 3.2): the confirmations a host has left
// before it falls back to pairing; 0 means it fell back and needs a new pairing code. Screen A8
// shows the number; no other field carries it. errs.As finds the *Error in it.
//
// Example:
//
//	return errs.FingerprintMismatch(outcome.Remaining)
func FingerprintMismatch(remaining int) error {
	return &attemptsError{e: New(CodeFingerprintMismatch, "The fingerprint prefix does not match."), remaining: max(remaining, 0)}
}

// Error returns the text of the wrapped error.
func (a *attemptsError) Error() string { return a.e.Error() }

// Unwrap returns the *Error.
func (a *attemptsError) Unwrap() error { return a.e }

// ProblemExtensions implements ProblemExtender: attempts_remaining.
func (a *attemptsError) ProblemExtensions() map[string]any {
	return map[string]any{memberAttemptsRemaining: a.remaining}
}
