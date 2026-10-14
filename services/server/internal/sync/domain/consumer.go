// SPDX-License-Identifier: Apache-2.0

package domain

import (
	"errors"
	"fmt"
	"math"
	"slices"
)

// ErrConsumerGap reports a hole in the numbers a worker consumer read that retention does not
// explain. The counter pattern rules holes out (04 "Pattern with a counter row" items 3 and 4), so
// it is a bug: the round rolls back and the cursor never passes the hole (docs/DECISIONS.md
// D-M2-01-2).
var ErrConsumerGap = errors.New("sync: gap in the events of a worker consumer")

// RoundPlan is what one round of a worker consumer processes (docs/spec/02-architecture.md
// "Worker consumers").
type RoundPlan struct {
	// Process are the sequence numbers to hand to the handler, in this order.
	Process []int64
	// NewLastSeq is the cursor after the round: the last number of Process.
	NewLastSeq int64
	// LostFrom and LostTo are the numbers retention removed before the consumer read them
	// (inclusive); LostFrom is zero when nothing was lost.
	LostFrom, LostTo int64
}

// PlanRound decides what a round processes after the cursor lastSeq, given the increasing
// numbers seqs it read (seq > lastSeq ORDER BY seq) and the smallest retained number minRetained
// (min(seq) of event_log; the caller reads it only when seqs[0] does not follow lastSeq and may
// pass 0 otherwise).
//
//   - seqs continue lastSeq without a hole: process all.
//   - seqs[0] equals minRetained and lastSeq+1 lies below it: the events lastSeq+1 to
//     minRetained-1 were removed by retention (03 event_log "Retention"); process all and
//     report them in LostFrom and LostTo.
//   - any other jump, a hole or a repeat inside seqs, or a number at or below lastSeq:
//     ErrConsumerGap.
//
// NewLastSeq is the last number of seqs and therefore greater than lastSeq. The returned plan
// owns its slice.
//
// Errors: ErrConsumerGap as above; a plain input error for an empty seqs (the round ends idle
// before) and for a negative lastSeq or one with no larger number (the cursor of 03 starts at 0
// and only grows).
//
// Example:
//
//	plan, err := domain.PlanRound(3, []int64{8, 9}, 8) // Process [8 9], NewLastSeq 9, lost 4..7
func PlanRound(lastSeq int64, seqs []int64, minRetained int64) (RoundPlan, error) {
	switch {
	case len(seqs) == 0:
		return RoundPlan{}, errors.New("sync: PlanRound needs at least one event")
	case lastSeq < 0:
		return RoundPlan{}, errors.New("sync: a consumer cursor is never negative")
	case lastSeq == math.MaxInt64:
		return RoundPlan{}, fmt.Errorf("%w: the cursor is at the largest sequence number", ErrConsumerGap)
	}
	var plan RoundPlan
	next := lastSeq + 1
	if first := seqs[0]; first != next {
		if next >= minRetained || first != minRetained {
			return RoundPlan{}, fmt.Errorf("%w: after seq %d read seq %d with the smallest retained seq %d", ErrConsumerGap, lastSeq, first, minRetained)
		}
		plan.LostFrom, plan.LostTo = next, minRetained-1
	}
	for i := 1; i < len(seqs); i++ {
		if seqs[i-1] == math.MaxInt64 || seqs[i] != seqs[i-1]+1 {
			return RoundPlan{}, fmt.Errorf("%w: seq %d follows seq %d", ErrConsumerGap, seqs[i], seqs[i-1])
		}
	}
	plan.Process = slices.Clone(seqs)
	plan.NewLastSeq = seqs[len(seqs)-1]
	return plan, nil
}
