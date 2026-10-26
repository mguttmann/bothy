// SPDX-License-Identifier: Apache-2.0

package load

import (
	"errors"
	"fmt"
	"math"
)

// Step is the plan of one ramp step: the offered rate, the people who send at it and hold a
// gateway connection, and the group spaces they fill.
type Step struct {
	// Index is the position in the ramp, from 0.
	Index int `json:"index"`
	// Rate is the offered message rate in messages per second (open loop).
	Rate float64 `json:"rate"`
	// Senders is how many people send; each keeps within Headroom of the message rate limit.
	Senders int `json:"senders"`
	// Connections is how many people hold a gateway connection: every sender.
	Connections int `json:"connections"`
	// Groups is how many group spaces the senders fill (GroupSize per group).
	Groups int `json:"groups"`
}

// PlanStep returns the plan of the step index at rate messages per second when one person may
// send perMinute messages per minute (ratelimit.rl.message.per_minute): as many senders as keep
// each at Headroom of that limit, one gateway connection each, GroupSize people per group. The
// error reports a rate or limit that is not positive.
//
// Example:
//
//	s, _ := PlanStep(0, 200, 60) // 400 senders, 400 connections, 20 groups
func PlanStep(index int, rate float64, perMinute int) (Step, error) {
	if rate <= 0 || math.IsNaN(rate) || math.IsInf(rate, 0) {
		return Step{}, fmt.Errorf("load: rate %g is not positive", rate)
	}
	if perMinute <= 0 {
		return Step{}, fmt.Errorf("load: the message rate limit %d per minute is not positive", perMinute)
	}
	perSender := Headroom * float64(perMinute) / 60
	senders := int(math.Ceil(rate/perSender - 1e-9))
	senders = max(senders, 1)
	return Step{Index: index, Rate: rate, Senders: senders, Connections: senders, Groups: (senders + GroupSize - 1) / GroupSize}, nil
}

// PlanRamp returns the steps of rates in order. The error reports an empty list, a rate that does
// not rise or an invalid limit.
func PlanRamp(rates []float64, perMinute int) ([]Step, error) {
	if len(rates) == 0 {
		return nil, errors.New("load: the ramp has no step")
	}
	out := make([]Step, len(rates))
	prev := math.Inf(-1)
	for i, r := range rates {
		if r <= prev {
			return nil, fmt.Errorf("load: rate %g of step %d does not rise above %g", r, i, prev)
		}
		prev = r
		s, err := PlanStep(i, r, perMinute)
		if err != nil {
			return nil, err
		}
		out[i] = s
	}
	return out, nil
}

// People returns how many people the steps need: the senders of the largest step.
func People(steps []Step) int {
	n := 0
	for _, s := range steps {
		n = max(n, s.Senders)
	}
	return n
}

// GroupOf returns the group of person i: people fill groups of GroupSize in order.
func GroupOf(i int) int { return i / GroupSize }

// IsObserver reports whether person i measures the fan-out of its group: the first
// ObserversPerGroup members of every group.
func IsObserver(i int) bool { return i%GroupSize < ObserversPerGroup }
