// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"time"

	"github.com/OWNER/bothy/services/server/internal/instance"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/terminal/domain"
)

// The outgoing ports of the terminal sessions (M2-44, Design notes 1, 2, 7 and 15).

// TokenSigner gives the signer of the active key of a purpose (instance.Keys.Signer); the session
// tokens use purpose terminal. Errors: kind unavailable without an active key.
type TokenSigner interface {
	Signer(ctx context.Context, p instance.Purpose) (instance.Signer, error)
}

// StepUps consumes the fresh step-up of a session for one opening or attach
// (identity.StepUp.ConsumeFresh, M2-40): inside the caller's unit of work, as the first domain row
// lock. Errors: 401 step_up_required for a stale or consumed step-up, 401 session_revoked.
type StepUps interface {
	ConsumeFresh(ctx context.Context, a actor.Actor) error
}

// Expectation is what one grant tells the relay of this node (Design note 7): the session, its
// host, the device of the grant, the deadline (grant time plus terminal.open_timeout), whether the
// PTY already started, whether the host reports require_local_confirm, and the working directory
// index of an opening of a claude profile (kept for an attach to that opening, ruling D-M2-44-4).
type Expectation struct {
	SessionID, HostID, DeviceID ids.ID
	Deadline                    time.Time
	Started                     bool
	LocalConfirm                bool
	WorkdirIndex                *int
	// Grant names the grant (the token jti of the row, M2-47): an expectation of the grant the
	// slot already holds changes nothing, so a signal that arrives after a side built the
	// expectation from the row, or twice, never resets the slot.
	Grant ids.ID
	// Times are the times of the row the deadlines of the relay start from (M2-46 Design note 7):
	// state, opened_at, the time of the grant, started_at, detached_at and last_input_at.
	Times domain.SessionTimes
}

// Counts are the live counts of the pairing of a session: bytes of the transport messages from and
// to the device and the time of the last device frame (zero for none).
type Counts struct {
	In, Out   int64
	LastInput time.Time
}

// RelayControl is the outgoing port to the relay of this node (adapters/wsrelay, Design note 7).
// Every method is safe for concurrent use and never waits on the network.
type RelayControl interface {
	// Expect creates or replaces the slot of e.SessionID: an older device side closes with 4103,
	// an older host side with 1000, and the deadline restarts.
	Expect(e Expectation)
	// Counters returns the live counts of the session's pairing; zero without a slot.
	Counters(sessionID ids.ID) Counts
	// OpeningWorkdir returns the working directory index the slot of an opening session remembers.
	OpeningWorkdir(sessionID ids.ID) (*int, bool)
}

// NoRelay is the RelayControl of a process without the stream routes: no slot.
type NoRelay struct{}

// Expect implements RelayControl: nothing to expect.
func (NoRelay) Expect(Expectation) {}

// Counters implements RelayControl: zero.
func (NoRelay) Counters(ids.ID) Counts { return Counts{} }

// OpeningWorkdir implements RelayControl: none.
func (NoRelay) OpeningWorkdir(ids.ID) (*int, bool) { return nil, false }

// Timers runs a function after a duration; the real one is time.AfterFunc, the tests drive a fake
// with the fake clock (Boundaries of M2-44: no test sleeps for a window).
type Timers interface {
	// AfterFunc calls f in its own goroutine after d; stop cancels it and reports whether it had
	// not run.
	AfterFunc(d time.Duration, f func()) (stop func() bool)
}

// RealTimers is Timers over time.AfterFunc.
type RealTimers struct{}

// AfterFunc implements Timers.
func (RealTimers) AfterFunc(d time.Duration, f func()) func() bool { return time.AfterFunc(d, f).Stop }

// Metrics records the instruments of the relay (D-W-105: only bothy.terminal.relay.unpaired here).
type Metrics interface {
	// RelayUnpaired counts one connection whose counterpart did not arrive before the deadline,
	// with the side of that connection.
	RelayUnpaired(ctx context.Context, side domain.Side)
}

// NoMetrics records nothing.
type NoMetrics struct{}

// RelayUnpaired implements Metrics.
func (NoMetrics) RelayUnpaired(context.Context, domain.Side) {}
