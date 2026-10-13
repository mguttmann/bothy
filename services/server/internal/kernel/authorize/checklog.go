// SPDX-License-Identifier: Apache-2.0

package authorize

import (
	"context"
	"slices"
	"strconv"
	"sync"
)

// CheckKind says which function of the Authorizer made a check.
type CheckKind int

// The kinds of a recorded check. Lists count through Scope (docs/spec/02-architecture.md 9.3).
const (
	// CheckAuthorize is one call of Authorize for one resource.
	CheckAuthorize CheckKind = iota + 1
	// CheckScope is one call of Scope for a list, search or snapshot.
	CheckScope
)

// String returns "authorize", "scope" or "CheckKind(<n>)" for a value outside the constants.
func (k CheckKind) String() string {
	switch k {
	case CheckAuthorize:
		return "authorize"
	case CheckScope:
		return "scope"
	}
	return "CheckKind(" + strconv.Itoa(int(k)) + ")"
}

// Check is one entry of the check log: the action that was checked, how, and whether it was
// allowed.
type Check struct {
	// Action is the action ID of docs/spec/actions.yaml, for example "thread.read".
	Action string
	// Kind is CheckAuthorize or CheckScope.
	Kind CheckKind
	// Allowed reports whether the decision allowed the action.
	Allowed bool
}

// checkLog is the concurrency-safe list of checks of one request; a handler may authorize from
// several goroutines.
type checkLog struct {
	mu     sync.Mutex
	checks []Check
}

type checkLogKey struct{}

// WithCheckLog returns a copy of ctx with a new, empty check log. platform/httpserver installs it
// per request (stage StageProof) and reads it back with Checks before the first byte of a
// successful response leaves the server: a status below 400 without a check of the route's action
// becomes 500 internal_error with the cause authz_not_checked (02 "Proof of the check", B7).
//
// Example:
//
//	ctx = authorize.WithCheckLog(ctx)
//	authorize.RecordCheck(ctx, authorize.Check{Action: "thread.read", Kind: authorize.CheckAuthorize, Allowed: true})
//	_ = authorize.Checks(ctx) // one entry
func WithCheckLog(ctx context.Context) context.Context {
	return context.WithValue(ctx, checkLogKey{}, &checkLog{})
}

// HasCheckLog reports whether ctx carries a check log installed by WithCheckLog.
func HasCheckLog(ctx context.Context) bool {
	_, ok := ctx.Value(checkLogKey{}).(*checkLog)
	return ok
}

// RecordCheck appends c to the check log of ctx. Every implementation of the Authorizer (the authz
// module, M1-28) must call it exactly once per Authorize and once per Scope call, with the action
// it decided and the outcome, also when it denies; that is how the HTTP chain proves that a handler
// checked its route's action. Without a log in ctx (a job, a test without the chain) the call does
// nothing. It is safe for concurrent use.
func RecordCheck(ctx context.Context, c Check) {
	l, ok := ctx.Value(checkLogKey{}).(*checkLog)
	if !ok {
		return
	}
	l.mu.Lock()
	l.checks = append(l.checks, c)
	l.mu.Unlock()
}

// Checks returns a copy of the checks recorded in ctx in the order they were recorded, or nil
// when ctx has no log. It is safe for concurrent use with RecordCheck.
func Checks(ctx context.Context) []Check {
	l, ok := ctx.Value(checkLogKey{}).(*checkLog)
	if !ok {
		return nil
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	return slices.Clone(l.checks)
}
