// SPDX-License-Identifier: Apache-2.0

package domain

import (
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// AggregateKey identifies identical denials (05 "2.4", R-AZ-04): the same principal, or without
// login the same IP pseudonym, the same action and the same reason.
type AggregateKey struct {
	// Principal is the actor principal; zero for an anonymous request.
	Principal ids.ID
	// IPHMAC is the IP pseudonym of an anonymous request; empty when Principal is set or the
	// request carried no client IP.
	IPHMAC string
	// Action is the action id.
	Action string
	// Reason is the reason code.
	Reason string
}

// Bucket is one aggregation of identical denials: it is written as one row with
// details.aggregate = {count, first_at, last_at} and the data of its first denial.
type Bucket[P any] struct {
	// Key is the aggregation key.
	Key AggregateKey
	// First is the data of the first denial (actor, resource, correlation, IP fields).
	First P
	// FirstAt is the time of the first denial; the window runs from here.
	FirstAt time.Time
	// LastAt is the latest time of a counted denial.
	LastAt time.Time
	// Count is the number of denials, at least 1.
	Count int64
}

// held is a bucket in memory; open buckets still count denials.
type held[P any] struct {
	Bucket[P]
	open bool
}

// Aggregation holds the buckets of one process (SEC-AUD-6) with the time passed in: a bucket
// counts identical denials for window from its first denial and is due then; at most maxKeys
// buckets are held, and a new key beyond that evicts the oldest for an immediate write. It is not
// safe for concurrent use; the app layer guards it.
type Aggregation[P any] struct {
	window  time.Duration
	maxKeys int
	open    map[AggregateKey]*held[P]
	buckets []*held[P] // oldest first
}

// NewAggregation returns an empty aggregation for audit.deny_aggregate_window and
// audit.deny_aggregate_max_keys; maxKeys below 1 counts as 1.
func NewAggregation[P any](window time.Duration, maxKeys int) *Aggregation[P] {
	return &Aggregation[P]{window: window, maxKeys: max(maxKeys, 1), open: map[AggregateKey]*held[P]{}}
}

// Add counts one denial of key at time at with the data first. Within the window of the open
// bucket of key it only raises Count and LastAt; otherwise that bucket closes (it is due) and a
// new bucket opens with first. When the new bucket would exceed the cap, the oldest bucket leaves
// the aggregation and is returned with true: the caller writes it at once.
func (a *Aggregation[P]) Add(key AggregateKey, at time.Time, first P) (Bucket[P], bool) {
	if h, ok := a.open[key]; ok {
		if at.Sub(h.FirstAt) < a.window {
			h.Count++
			if at.After(h.LastAt) {
				h.LastAt = at
			}
			return Bucket[P]{}, false
		}
		h.open = false
		delete(a.open, key)
	}
	var evicted Bucket[P]
	full := len(a.buckets) >= a.maxKeys
	if full {
		evicted = a.remove(0)
	}
	h := &held[P]{Bucket: Bucket[P]{Key: key, First: first, FirstAt: at, LastAt: at, Count: 1}, open: true}
	a.open[key] = h
	a.buckets = append(a.buckets, h)
	return evicted, full
}

// remove takes the bucket at index i out of the aggregation.
func (a *Aggregation[P]) remove(i int) Bucket[P] {
	h := a.buckets[i]
	a.buckets = append(a.buckets[:i], a.buckets[i+1:]...)
	if h.open && a.open[h.Key] == h {
		delete(a.open, h.Key)
	}
	return h.Bucket
}

// TakeDue removes and returns the buckets whose window ended at now (FirstAt + window <= now) and
// those already closed by a later denial of their key, oldest first.
func (a *Aggregation[P]) TakeDue(now time.Time) []Bucket[P] {
	var due []Bucket[P]
	keep := a.buckets[:0]
	for _, h := range a.buckets {
		if !h.open || !now.Before(h.FirstAt.Add(a.window)) {
			if h.open {
				delete(a.open, h.Key)
			}
			due = append(due, h.Bucket)
			continue
		}
		keep = append(keep, h)
	}
	clear(a.buckets[len(keep):])
	a.buckets = keep
	return due
}

// TakeAll removes and returns every bucket, oldest first (Flush, shutdown).
func (a *Aggregation[P]) TakeAll() []Bucket[P] {
	out := make([]Bucket[P], 0, len(a.buckets))
	for _, h := range a.buckets {
		out = append(out, h.Bucket)
	}
	a.buckets = nil
	clear(a.open)
	return out
}

// Restore puts buckets that could not be written back in front, closed, so the next attempt
// writes them first. Beyond the cap the oldest buckets are returned instead: the caller drops
// them with an error log.
func (a *Aggregation[P]) Restore(bs []Bucket[P]) []Bucket[P] {
	back := make([]*held[P], 0, len(bs)+len(a.buckets))
	for _, b := range bs {
		back = append(back, &held[P]{Bucket: b})
	}
	a.buckets = append(back, a.buckets...)
	var dropped []Bucket[P]
	for len(a.buckets) > a.maxKeys {
		dropped = append(dropped, a.remove(0))
	}
	return dropped
}

// NextDeadline returns the earliest time a bucket is due and true, or false without buckets.
// A closed bucket is due at once (its FirstAt).
func (a *Aggregation[P]) NextDeadline() (time.Time, bool) {
	var next time.Time
	for i, h := range a.buckets {
		d := h.FirstAt.Add(a.window)
		if !h.open {
			d = h.FirstAt
		}
		if i == 0 || d.Before(next) {
			next = d
		}
	}
	return next, len(a.buckets) > 0
}

// Len returns the number of held buckets, open and due.
func (a *Aggregation[P]) Len() int { return len(a.buckets) }
