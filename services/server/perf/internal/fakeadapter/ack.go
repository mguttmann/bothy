// SPDX-License-Identifier: Apache-2.0

package fakeadapter

import "time"

// AckPolicy decides when the adapter sends a cumulative ack with the highest received seq (design
// note 3 c; 04 section 15.2; an ack costs rl.agent_runtime): right after an invocation-bearing
// event reached its ack step; otherwise when the unacknowledged events reach half of
// agent.max_inflight_events; otherwise when agent.ack_persist_interval has passed since the last
// ack and an event is unacknowledged. So the window never fills and the server never waits on the
// adapter during a measurement. Not safe for concurrent use.
type AckPolicy struct {
	half    int
	persist time.Duration
	highest int64
	unacked int
	lastAck time.Time
}

// NewAckPolicy returns the policy for agent.max_inflight_events maxInflight and
// agent.ack_persist_interval persist, starting at now.
func NewAckPolicy(maxInflight int, persist time.Duration, now time.Time) *AckPolicy {
	return &AckPolicy{half: max(maxInflight/2, 1), persist: persist, lastAck: now}
}

// Received records the event seq. For an event without invocation it returns the seq to ack and
// true when the unacknowledged events reached half of the window; an invocation-bearing event
// waits for Invoked.
func (p *AckPolicy) Received(seq int64, invocation bool, now time.Time) (int64, bool) {
	p.highest = max(p.highest, seq)
	p.unacked++
	if invocation || p.unacked < p.half {
		return 0, false
	}
	return p.ack(now)
}

// Invoked returns the ack after an invocation-bearing event reached its ack step.
func (p *AckPolicy) Invoked(now time.Time) (int64, bool) {
	if p.unacked == 0 {
		return 0, false
	}
	return p.ack(now)
}

// Tick returns an ack when persist has passed since the last ack and an event is unacknowledged.
func (p *AckPolicy) Tick(now time.Time) (int64, bool) {
	if p.unacked == 0 || now.Sub(p.lastAck) < p.persist {
		return 0, false
	}
	return p.ack(now)
}

// Unacked returns how many received events are not acknowledged yet.
func (p *AckPolicy) Unacked() int { return p.unacked }

// ack acknowledges everything up to the highest received seq.
func (p *AckPolicy) ack(now time.Time) (int64, bool) {
	p.unacked, p.lastAck = 0, now
	return p.highest, true
}
