// SPDX-License-Identifier: Apache-2.0

package domain

import "time"

// State is the state of a gateway session (docs/spec/04-sync-protokoll.md section 6.3). A
// connection starts in awaiting_identify; identify or resume make its session ready; a lost
// connection leaves the session detached, resumable within sync.resume_window; closed is final.
type State string

// The states of a gateway session.
const (
	// StateAwaitingIdentify is a connection after hello and before identify or resume.
	StateAwaitingIdentify State = "awaiting_identify"
	// StateReady is a session with an open connection.
	StateReady State = "ready"
	// StateDetached is a session whose connection ended; resume can pick it up.
	StateDetached State = "detached"
	// StateClosed is a session that no resume can reach any more.
	StateClosed State = "closed"
)

// transitions are the allowed successors of each state.
var transitions = map[State][]State{
	StateAwaitingIdentify: {StateReady, StateClosed},
	StateReady:            {StateDetached, StateClosed},
	StateDetached:         {StateReady, StateClosed},
}

// CanBecome reports whether s may change to to.
func (s State) CanBecome(to State) bool {
	for _, t := range transitions[s] {
		if t == to {
			return true
		}
	}
	return false
}

// HeartbeatDeadline returns the instant the server closes with 4009 when no heartbeat arrives
// after last (ready, resumed or the last heartbeat): interval times (missLimit + 1), the values of
// sync.heartbeat_interval and sync.heartbeat_miss_limit. The extra interval absorbs the random
// delay of the client's first heartbeat (04 section 6.3 step 5; docs/DECISIONS.md M1-64).
func HeartbeatDeadline(last time.Time, interval time.Duration, missLimit int) time.Time {
	return last.Add(interval * time.Duration(missLimit+1))
}

// ReauthAt returns the instant reauth_required goes out: lead (sync.reauth_lead_time) before the
// expiry of the access token (04 section 6.3 step 6).
func ReauthAt(expiresAt time.Time, lead time.Duration) time.Time {
	return expiresAt.Add(-lead)
}
