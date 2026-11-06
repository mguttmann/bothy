// SPDX-License-Identifier: Apache-2.0

package interview

import "time"

// State is the state of one open interview request.
type State uint8

// The states of a request: open until the first answer of the builder or the end of the wait.
const (
	StateOpen State = iota
	// StateAnswered: the builder's answer passed the answer schema.
	StateAnswered
	// StateInvalid: the builder's answer was refused as a whole (builder_invalid).
	StateInvalid
	// StateExpired: the wait ended without an answer (builder_timeout).
	StateExpired
)

// Request is the state machine open -> answered | invalid | expired of one interview request. The
// caller guards it with a mutex; every transition is a compare-and-swap out of open, so a request
// has exactly one outcome and the builder answers once (26 6.1 item 2).
type Request struct {
	expiresAt time.Time
	state     State
	answer    Answer
}

// NewRequest returns an open request that waits until expiresAt.
func NewRequest(expiresAt time.Time) Request { return Request{expiresAt: expiresAt} }

// State returns the state.
func (r *Request) State() State { return r.state }

// Answered returns the accepted answer; the zero answer unless answered.
func (r *Request) Answered() Answer { return r.answer }

// Answer moves open -> answered with a; false in any other state.
func (r *Request) Answer(a Answer) bool {
	if r.state != StateOpen {
		return false
	}
	r.state, r.answer = StateAnswered, a
	return true
}

// Refuse moves open -> invalid; false in any other state.
func (r *Request) Refuse() bool {
	if r.state != StateOpen {
		return false
	}
	r.state = StateInvalid
	return true
}

// Expire moves open -> expired; false in any other state.
func (r *Request) Expire() bool {
	if r.state != StateOpen {
		return false
	}
	r.state = StateExpired
	return true
}

// Forgotten reports whether the request may be forgotten at now: from expires_at plus keep
// (agents.interview.timeout) on. Until then a late or second answer learns 410 or 409 instead of
// 404.
func (r *Request) Forgotten(now time.Time, keep time.Duration) bool {
	return !now.Before(r.expiresAt.Add(keep))
}
