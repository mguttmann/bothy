// SPDX-License-Identifier: Apache-2.0

package domain_test

import (
	"errors"
	"math"
	"slices"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/sync/domain"
)

func TestPlanRound(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name        string
		lastSeq     int64
		seqs        []int64
		minRetained int64
		want        domain.RoundPlan
		gap         bool
	}{
		{name: "contiguous batch", lastSeq: 10, seqs: []int64{11, 12, 13}, minRetained: 1,
			want: domain.RoundPlan{Process: []int64{11, 12, 13}, NewLastSeq: 13}},
		{name: "single event", lastSeq: 0, seqs: []int64{1}, minRetained: 0,
			want: domain.RoundPlan{Process: []int64{1}, NewLastSeq: 1}},
		{name: "minimum not read", lastSeq: 4, seqs: []int64{5, 6}, minRetained: 0,
			want: domain.RoundPlan{Process: []int64{5, 6}, NewLastSeq: 6}},
		{name: "retention loss", lastSeq: 3, seqs: []int64{8, 9, 10}, minRetained: 8,
			want: domain.RoundPlan{Process: []int64{8, 9, 10}, NewLastSeq: 10, LostFrom: 4, LostTo: 7}},
		{name: "retention loss of the first event", lastSeq: 0, seqs: []int64{2}, minRetained: 2,
			want: domain.RoundPlan{Process: []int64{2}, NewLastSeq: 2, LostFrom: 1, LostTo: 1}},
		{name: "retention loss claimed but the first number differs from the minimum", lastSeq: 3,
			seqs: []int64{9, 10}, minRetained: 8, gap: true},
		{name: "first number jumps although the minimum is not above lastSeq+1", lastSeq: 3,
			seqs: []int64{6, 7}, minRetained: 4, gap: true},
		{name: "first number jumps with the minimum at lastSeq+1", lastSeq: 3,
			seqs: []int64{5}, minRetained: 4, gap: true},
		{name: "hole inside the batch", lastSeq: 3, seqs: []int64{4, 5, 7}, minRetained: 1, gap: true},
		{name: "hole after a retention loss", lastSeq: 3, seqs: []int64{8, 10}, minRetained: 8, gap: true},
		{name: "repeat inside the batch", lastSeq: 3, seqs: []int64{4, 4}, minRetained: 1, gap: true},
		{name: "number at the cursor", lastSeq: 3, seqs: []int64{3, 4}, minRetained: 1, gap: true},
		{name: "empty batch", lastSeq: 3, seqs: nil, minRetained: 1, gap: false},
		{name: "negative cursor", lastSeq: -1, seqs: []int64{0}, minRetained: 0, gap: false},
		{name: "cursor at the largest number", lastSeq: math.MaxInt64, seqs: []int64{1}, minRetained: 1, gap: true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			got, err := domain.PlanRound(c.lastSeq, c.seqs, c.minRetained)
			switch {
			case c.gap:
				if !errors.Is(err, domain.ErrConsumerGap) {
					t.Fatalf("PlanRound = %+v, %v; want ErrConsumerGap", got, err)
				}
			case c.want.Process == nil:
				if err == nil || errors.Is(err, domain.ErrConsumerGap) {
					t.Fatalf("PlanRound = %+v, %v; want an input error other than ErrConsumerGap", got, err)
				}
			default:
				if err != nil {
					t.Fatalf("PlanRound: %v", err)
				}
				if !slices.Equal(got.Process, c.want.Process) || got.NewLastSeq != c.want.NewLastSeq ||
					got.LostFrom != c.want.LostFrom || got.LostTo != c.want.LostTo {
					t.Fatalf("PlanRound = %+v, want %+v", got, c.want)
				}
				if got.NewLastSeq != got.Process[len(got.Process)-1] {
					t.Fatalf("NewLastSeq %d is not the last number processed %d", got.NewLastSeq, got.Process[len(got.Process)-1])
				}
			}
		})
	}
}

// TestPlanRoundCopiesTheInput: the plan owns its numbers, so a caller that reuses its slice
// cannot change a plan already taken.
func TestPlanRoundCopiesTheInput(t *testing.T) {
	t.Parallel()
	seqs := []int64{1, 2}
	got, err := domain.PlanRound(0, seqs, 1)
	if err != nil {
		t.Fatal(err)
	}
	seqs[0] = 99
	if got.Process[0] != 1 {
		t.Fatalf("the plan shares the input slice: %v", got.Process)
	}
}

// FuzzPlanRound: for any cursor, increasing numbers and minimum, a nil error means the numbers
// to process are contiguous, start at lastSeq+1 or at the minimum (then LostFrom..LostTo is
// exactly the hole before it), and the cursor moves forward; no input panics.
func FuzzPlanRound(f *testing.F) {
	f.Add(int64(0), int64(1), uint8(3), int64(0), uint8(0))
	f.Add(int64(10), int64(11), uint8(1), int64(1), uint8(0))
	f.Add(int64(3), int64(8), uint8(3), int64(8), uint8(0))
	f.Add(int64(3), int64(9), uint8(2), int64(8), uint8(0))
	f.Add(int64(3), int64(4), uint8(4), int64(1), uint8(2))
	f.Add(int64(math.MaxInt64), int64(math.MaxInt64), uint8(1), int64(1), uint8(0))
	f.Add(int64(-5), int64(1), uint8(2), int64(-3), uint8(0))
	f.Fuzz(func(t *testing.T, lastSeq, first int64, n uint8, minRetained int64, holeAt uint8) {
		seqs := make([]int64, 0, int(n)%32)
		next := first
		for i := range int(n) % 32 {
			if i > 0 && holeAt != 0 && i == int(holeAt)%32 {
				if next == math.MaxInt64 {
					break
				}
				next++
			}
			seqs = append(seqs, next)
			if next == math.MaxInt64 {
				break
			}
			next++
		}
		plan, err := domain.PlanRound(lastSeq, seqs, minRetained)
		if err != nil {
			return
		}
		if len(plan.Process) == 0 || !slices.Equal(plan.Process, seqs) {
			t.Fatalf("Process %v is not the batch %v", plan.Process, seqs)
		}
		for i := 1; i < len(plan.Process); i++ {
			if plan.Process[i] != plan.Process[i-1]+1 {
				t.Fatalf("Process %v is not contiguous", plan.Process)
			}
		}
		start := plan.Process[0]
		switch start {
		case lastSeq + 1:
			if plan.LostFrom != 0 || plan.LostTo != 0 {
				t.Fatalf("loss %d..%d reported without a hole", plan.LostFrom, plan.LostTo)
			}
		case minRetained:
			if plan.LostFrom != lastSeq+1 || plan.LostTo != minRetained-1 || plan.LostFrom > plan.LostTo {
				t.Fatalf("loss %d..%d, want %d..%d", plan.LostFrom, plan.LostTo, lastSeq+1, minRetained-1)
			}
		default:
			t.Fatalf("Process starts at %d, neither lastSeq+1 = %d nor the minimum %d", start, lastSeq+1, minRetained)
		}
		if plan.NewLastSeq <= lastSeq || plan.NewLastSeq != plan.Process[len(plan.Process)-1] {
			t.Fatalf("NewLastSeq %d after lastSeq %d, last processed %d", plan.NewLastSeq, lastSeq, plan.Process[len(plan.Process)-1])
		}
	})
}
