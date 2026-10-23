// SPDX-License-Identifier: Apache-2.0

package domain_test

// The fuzz test of PlanRunEnd.

import (
	"slices"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/runs/domain"
)

// FuzzPlanRunEnd decodes an end and up to eight open follow-ups from the fuzz bytes: every open
// follow-up lands in exactly one of Deleted, Stopped and Candidates; Candidates is sorted by
// thread_seq; Stopped is non-empty only for user_cancelled and Candidates only for every other
// end; the input is unchanged.
func FuzzPlanRunEnd(f *testing.F) {
	for i := range planEnds {
		f.Add(uint8(i), []byte{3, 1, 0, 9, 2, 1, 5, 0, 0, 7, 1, 1})
	}
	f.Fuzz(func(t *testing.T, endIndex uint8, raw []byte) {
		end := planEnds[int(endIndex)%len(planEnds)]
		var open []domain.OpenFollowup
		for i := 0; i+2 < len(raw) && len(open) < 8; i += 3 {
			st := domain.FollowupQueued
			if raw[i+1]%2 == 1 {
				st = domain.FollowupDelivered
			}
			open = append(open, domain.OpenFollowup{MessageID: ids.New(), ThreadSeq: int64(raw[i]), State: st, Deleted: raw[i+2]%3 == 0})
		}
		before := slices.Clone(open)
		plan, err := domain.PlanRunEnd(end, open)
		if err != nil {
			t.Fatal(err)
		}
		if !slices.Equal(before, open) {
			t.Fatal("the input changed")
		}
		seen := map[ids.ID]int{}
		for _, list := range [][]ids.ID{plan.Deleted, plan.Stopped, plan.Candidates} {
			for _, id := range list {
				seen[id]++
			}
		}
		seq := map[ids.ID]int64{}
		for _, o := range open {
			seq[o.MessageID] = o.ThreadSeq
			if seen[o.MessageID] != 1 {
				t.Fatalf("follow-up in %d lists", seen[o.MessageID])
			}
		}
		if len(seen) != len(open) {
			t.Fatal("the plan names a message that is not open")
		}
		if !slices.IsSortedFunc(plan.Candidates, func(a, b ids.ID) int { return int(seq[a] - seq[b]) }) {
			t.Fatal("candidates are not in thread order")
		}
		stop := end.State == domain.StateCancelled && end.Reason == "user_cancelled"
		if (stop && len(plan.Candidates) > 0) || (!stop && len(plan.Stopped) > 0) {
			t.Fatalf("plan %+v for %v", plan, end)
		}
	})
}
