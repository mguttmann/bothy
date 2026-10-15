// SPDX-License-Identifier: Apache-2.0

package domain

import (
	"slices"
	"time"
)

// KeyState is server_signing_keys.state (docs/spec/03-data-model.md): the life of a signing key
// from announced to withdrawn (19 section 10).
type KeyState string

// The states of a signing key.
const (
	// KeyNext is announced: published in the JWKS, not yet signing.
	KeyNext KeyState = "next"
	// KeyActive signs; exactly one per purpose (partial unique index of 03).
	KeyActive KeyState = "active"
	// KeyRetiring stopped signing at not_after and stays published until everything it signed
	// has expired.
	KeyRetiring KeyState = "retiring"
	// KeyRetired is withdrawn: never published, never accepted, its private file is deleted.
	KeyRetired KeyState = "retired"
)

// Valid reports whether s is one of the four states.
func (s KeyState) Valid() bool {
	switch s {
	case KeyNext, KeyActive, KeyRetiring, KeyRetired:
		return true
	}
	return false
}

// Published reports whether a key in state s is in the JWKS and verifies signatures: next, active
// and retiring (09 section 4.5).
func (s KeyState) Published() bool { return s == KeyNext || s == KeyActive || s == KeyRetiring }

// SigningKey is what the transitions read of one key of a purpose.
type SigningKey struct {
	// KID is the key id (RFC 7638 thumbprint).
	KID string
	// State is the current state.
	State KeyState
	// CreatedAt is when the row was created; the rotation interval and the lead of a next key
	// count from it.
	CreatedAt time.Time
	// NotAfter is when the key stopped signing; zero while it never did.
	NotAfter time.Time
}

// RotationPolicy are the times of one purpose (19 section 10, 09 section 4.5).
type RotationPolicy struct {
	// Interval is the scheduled rotation (signing_key.invocation_rotation_interval or
	// signing_key.access_token_rotation_interval); zero for a purpose that rotates on owner
	// request only.
	Interval time.Duration
	// Lead is agent.jwks_max_age: how long a next key is published before it signs.
	Lead time.Duration
	// Window is the longest validity of a signature after the key stopped signing (Window); a
	// retiring key is retired once it has passed.
	Window time.Duration
}

// Transition is one change of state of one key.
type Transition struct {
	// KID names the key.
	KID string
	// From is the state the key must still have; a transition whose key moved on in the meantime
	// does not apply.
	From KeyState
	// To is the new state.
	To KeyState
	// NotAfter is set as the new not_after when not zero; zero keeps the column.
	NotAfter time.Time
}

// Plan is what DueTransitions found due for one purpose at one time.
type Plan struct {
	// Transitions apply in this order: retirements, then the old active key to retiring before
	// its next key becomes active, so the unique index of active keys never sees two.
	Transitions []Transition
	// CreateNext asks for a new key in state next.
	CreateNext bool
}

// Empty reports whether nothing is due.
func (p Plan) Empty() bool { return len(p.Transitions) == 0 && !p.CreateNext }

// Window returns the window of a purpose: the validity of what it signs plus auth.clock_leeway
// (19 section 10; 09 section 4.5 point 2). The audit checkpoint purpose has the zero window,
// because old checkpoints are verified through the key in any state.
func Window(validity, leeway time.Duration) time.Duration { return validity + leeway }

// DueTransitions returns the scheduled transitions of the keys of one purpose at time t
// (brief M1-32 section 4), each condition inclusive of its due time:
//
//   - retire: a retiring key whose not_after + Window <= t becomes retired;
//   - promote: a next key whose created_at + Lead <= t becomes active, and the active key before
//     it becomes retiring with not_after = t;
//   - announce: with an Interval, no next key and an active key (after the promotion) whose
//     created_at + Interval <= t, a new next key is due (CreateNext).
//
// Retired keys are ignored. With several next keys (which the transactions never create) the
// oldest is promoted.
func DueTransitions(keys []SigningKey, p RotationPolicy, t time.Time) Plan {
	var plan Plan
	for _, k := range sorted(keys) {
		if k.State == KeyRetiring && !k.NotAfter.IsZero() && !k.NotAfter.Add(p.Window).After(t) {
			plan.Transitions = append(plan.Transitions, Transition{KID: k.KID, From: KeyRetiring, To: KeyRetired})
		}
	}
	active, hasActive := ActiveKey(keys)
	next, hasNext := PendingNext(keys)
	if hasNext && !next.CreatedAt.Add(p.Lead).After(t) {
		if hasActive {
			plan.Transitions = append(plan.Transitions, Transition{KID: active.KID, From: KeyActive, To: KeyRetiring, NotAfter: t})
		}
		plan.Transitions = append(plan.Transitions, Transition{KID: next.KID, From: KeyNext, To: KeyActive})
		active, hasActive, hasNext = next, true, false
	}
	if p.Interval > 0 && !hasNext && hasActive && !active.CreatedAt.Add(p.Interval).After(t) {
		plan.CreateNext = true
	}
	return plan
}

// EmergencyTransitions returns the transitions of an emergency rotation at t (09 section 4.5
// point 3, 19 section 10): every next, active and retiring key of the purpose becomes retired at
// once, because a leak of the key store exposes all of them; not_after becomes t where it is not
// set. The caller creates a fresh active key in the same transaction.
func EmergencyTransitions(keys []SigningKey, t time.Time) []Transition {
	var out []Transition
	for _, k := range sorted(keys) {
		if !k.State.Published() {
			continue
		}
		tr := Transition{KID: k.KID, From: k.State, To: KeyRetired}
		if k.NotAfter.IsZero() {
			tr.NotAfter = t
		}
		out = append(out, tr)
	}
	return out
}

// LostTransitions handles keys whose private file is gone from the key store (a restore without
// the key store, docs/operations/disaster-recovery.md case B): every unretired key in lost becomes
// retired, with not_after = t where it is not set, because nothing can sign with it and an
// announced or retiring key without its file must not stay published as if it could. needActive
// reports that the purpose has no active key left afterwards; the caller then creates a fresh
// one. Kids in lost that are not among keys are ignored.
func LostTransitions(keys []SigningKey, lost map[string]bool, t time.Time) (trs []Transition, needActive bool) {
	needActive = true
	for _, k := range sorted(keys) {
		if !k.State.Published() {
			continue
		}
		if !lost[k.KID] {
			if k.State == KeyActive {
				needActive = false
			}
			continue
		}
		tr := Transition{KID: k.KID, From: k.State, To: KeyRetired}
		if k.NotAfter.IsZero() {
			tr.NotAfter = t
		}
		trs = append(trs, tr)
	}
	return trs, needActive
}

// PendingNext returns the oldest key in state next; ok is false when there is none. A rotation
// with mode scheduled returns it instead of announcing a second one.
func PendingNext(keys []SigningKey) (SigningKey, bool) { return first(keys, KeyNext) }

// ActiveKey returns the key in state active; ok is false when the purpose has none.
func ActiveKey(keys []SigningKey) (SigningKey, bool) { return first(keys, KeyActive) }

// first returns the oldest key in state s.
func first(keys []SigningKey, s KeyState) (SigningKey, bool) {
	for _, k := range sorted(keys) {
		if k.State == s {
			return k, true
		}
	}
	return SigningKey{}, false
}

// sorted returns the keys by state order (retiring, active, next, retired) and age, so plans list
// retirements first and the old active key before its successor.
func sorted(keys []SigningKey) []SigningKey {
	order := map[KeyState]int{KeyRetiring: 0, KeyActive: 1, KeyNext: 2, KeyRetired: 3}
	out := slices.Clone(keys)
	slices.SortStableFunc(out, func(a, b SigningKey) int {
		if d := order[a.State] - order[b.State]; d != 0 {
			return d
		}
		return a.CreatedAt.Compare(b.CreatedAt)
	})
	return out
}
