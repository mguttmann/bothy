// SPDX-License-Identifier: Apache-2.0

package domain

import (
	"errors"
	gosync "sync"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// ErrTooManyFocused is the error of a focus with more distinct threads than
// sync.max_focused_threads; the gateway answers validation_failed and keeps the previous set
// (docs/spec/04-sync-protocol.md section 11.3 item 2).
var ErrTooManyFocused = errors.New("gateway: more focused threads than sync.max_focused_threads")

// FocusSet is the body of one focus op after its rules: the distinct thread ids in the order the
// client sent them and the idle flag (04 section 6.2 "focus"). The handler then drops the ids
// the principal may not read.
type FocusSet struct {
	// Threads are the distinct thread ids.
	Threads []ids.ID
	// Idle is true when the window is open but unused (13 section 4.4); only stored in M1.
	Idle bool
}

// NewFocusSet returns the focus set of threads and idle: repeated ids count once and keep their
// first position. Errors: ErrTooManyFocused when more than maxThreads distinct ids remain.
//
// Example:
//
//	set, err := domain.NewFocusSet(in.ThreadIDs, in.Idle, cfg.MaxFocusedThreads)
func NewFocusSet(threads []ids.ID, idle bool, maxThreads int) (FocusSet, error) {
	seen := make(map[ids.ID]bool, len(threads))
	out := make([]ids.ID, 0, len(threads))
	for _, t := range threads {
		if seen[t] {
			continue
		}
		seen[t] = true
		out = append(out, t)
	}
	if len(out) > maxThreads {
		return FocusSet{}, ErrTooManyFocused
	}
	return FocusSet{Threads: out, Idle: idle}, nil
}

// FocusEntry is one focused thread that the principal may read, with the channel and space it
// lay in when it was checked, so the hub finds the entries an archive event affects.
type FocusEntry struct {
	ThreadID, ChannelID, SpaceID ids.ID
}

// Focus is the stored focus of a connection: the readable threads and the idle flag.
type Focus struct {
	Entries []FocusEntry
	Idle    bool
}

// TypingThrottle is the fan-out throttle of typing.start of one connection (04 section 12 item
// 1): at most one typing.started per thread every sync.typing_min_interval. Time comes from the
// caller's clock. It is safe for concurrent use.
type TypingThrottle struct {
	interval time.Duration

	mu   gosync.Mutex
	last map[ids.ID]time.Time
}

// NewTypingThrottle returns an empty throttle with the interval sync.typing_min_interval.
func NewTypingThrottle(interval time.Duration) *TypingThrottle {
	return &TypingThrottle{interval: interval, last: map[ids.ID]time.Time{}}
}

// Allow reports whether a typing.start on thread at now fans out, and records now when it does:
// true when the thread has no fan-out younger than the interval. Entries older than the interval
// are dropped on the way, so the map stays as small as the rate limit rl.typing allows.
func (t *TypingThrottle) Allow(thread ids.ID, now time.Time) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	for id, at := range t.last {
		if now.Sub(at) >= t.interval {
			delete(t.last, id)
		}
	}
	if _, recent := t.last[thread]; recent {
		return false
	}
	t.last[thread] = now
	return true
}

// Reset forgets the last fan-out of thread (typing.stop), so the next typing.start fans out at
// once.
func (t *TypingThrottle) Reset(thread ids.ID) {
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.last, thread)
}

// EphemeralType is the delivery rule of one ephemeral event type (docs/spec/05-actions.md
// "Ephemeral events").
type EphemeralType struct {
	// ReceiveAction is the action a receiver needs on the event's resource.
	ReceiveAction string
	// FocusBound marks the additional condition "thread in focus": only connections that focus
	// the thread of the event's scope are candidates.
	FocusBound bool
	// ExcludeActor keeps the event away from every connection of its actor (typing.started,
	// 04 section 12 item 1).
	ExcludeActor bool
}

// ephemeralTypes is the register of the ephemeral types the gateway relays. typing.started and
// run.updated carry the additional condition "thread in focus" of 05 "Ephemeral events";
// host.presence_changed (M2-43) has no scope, so every live connection is a candidate and its
// receive action host.read admits it, and agent.presence_changed (M2-20) likewise with
// agent.read. notification.created (M3-23) is personal: its candidates are the connections of
// its audience, and sync.Admit admits it with push.deliver on the thread, or for a hint without
// a thread (resource user) only to the recipient's own connections. The other ephemeral types of
// 05 (command.autocomplete_requested, stats.live) are added by the tickets that build them, with
// their own candidate rule. TestEphemeralTypes holds the receive actions against
// docs/spec/actions.yaml.
var ephemeralTypes = map[string]EphemeralType{
	"typing.started":        {ReceiveAction: "thread.read", FocusBound: true, ExcludeActor: true},
	"run.updated":           {ReceiveAction: "run.read", FocusBound: true},
	"host.presence_changed": {ReceiveAction: "host.read"},
	// agent.presence_changed (M2-20) has no scope either; agent.read admits owners, readers of a
	// served channel and the agent itself.
	"agent.presence_changed": {ReceiveAction: "agent.read"},
	// notification.created (M3-23): a hint to the live connections of its recipient (audience
	// self), instead of a push to that device (13 6.3).
	"notification.created": {ReceiveAction: "push.deliver"},
}

// LookupEphemeral returns the delivery rule of the ephemeral type typ; false when the gateway
// does not relay it.
func LookupEphemeral(typ string) (EphemeralType, bool) {
	et, ok := ephemeralTypes[typ]
	return et, ok
}
