// SPDX-License-Identifier: Apache-2.0

package load

import (
	"testing"
)

func TestPlan_SendersKeepEveryoneWithinTheRateLimit(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		rate      float64
		perMinute int
		want      int
	}{
		{rate: 50, perMinute: 60, want: 100},   // 0.5 messages per second each
		{rate: 200, perMinute: 60, want: 400},  // the documented sustained load
		{rate: 1000, perMinute: 60, want: 2000},
		{rate: 10, perMinute: 7, want: 172},    // rounds up: 10 / (0.5 x 7/60) = 171.4
		{rate: 0.1, perMinute: 60, want: 1},
	} {
		s, err := PlanStep(0, tc.rate, tc.perMinute)
		if err != nil {
			t.Fatalf("rate %g: %v", tc.rate, err)
		}
		if s.Senders != tc.want {
			t.Errorf("rate %g at %d per minute: %d senders, want %d", tc.rate, tc.perMinute, s.Senders, tc.want)
		}
		if per := tc.rate / float64(s.Senders); per > Headroom*float64(tc.perMinute)/60+1e-9 {
			t.Errorf("rate %g: %g messages per second and sender, above the headroom", tc.rate, per)
		}
		if s.Groups != (s.Senders+GroupSize-1)/GroupSize {
			t.Errorf("rate %g: %d groups for %d senders", tc.rate, s.Groups, s.Senders)
		}
		if s.Connections != s.Senders {
			t.Errorf("rate %g: %d connections, want one per sender (%d)", tc.rate, s.Connections, s.Senders)
		}
	}
}

func TestPlan_RejectsBadInput(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		rate      float64
		perMinute int
	}{{0, 60}, {-1, 60}, {10, 0}} {
		if _, err := PlanStep(0, tc.rate, tc.perMinute); err == nil {
			t.Errorf("PlanStep(%g, %d) passed", tc.rate, tc.perMinute)
		}
	}
	if _, err := PlanRamp([]float64{100, 50}, 60); err == nil {
		t.Error("a falling ramp passed")
	}
	if _, err := PlanRamp(nil, 60); err == nil {
		t.Error("an empty ramp passed")
	}
}

func TestPlan_RampPeopleIsTheLargestStep(t *testing.T) {
	t.Parallel()
	steps, err := PlanRamp(RampRates, 60)
	if err != nil {
		t.Fatal(err)
	}
	if len(steps) != len(RampRates) {
		t.Fatalf("%d steps for %d rates", len(steps), len(RampRates))
	}
	for i, s := range steps {
		if s.Index != i || s.Rate != RampRates[i] {
			t.Errorf("step %d = %+v", i, s)
		}
	}
	if got, want := People(steps), steps[len(steps)-1].Senders; got != want {
		t.Errorf("People = %d, want the senders of the last step %d", got, want)
	}
}

func TestPlan_GroupOfPerson(t *testing.T) {
	t.Parallel()
	for i, want := range map[int]int{0: 0, GroupSize - 1: 0, GroupSize: 1, 3*GroupSize + 2: 3} {
		if got := GroupOf(i); got != want {
			t.Errorf("GroupOf(%d) = %d, want %d", i, got, want)
		}
	}
	// Observers are the first ObserversPerGroup members of a group.
	if !IsObserver(0) || IsObserver(ObserversPerGroup) || !IsObserver(GroupSize) {
		t.Error("IsObserver does not pick the first members of each group")
	}
}
