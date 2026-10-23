// SPDX-License-Identifier: Apache-2.0

package pushhandoff_test

import (
	"math"
	"strings"
	"testing"

	"github.com/OWNER/bothy/services/server/perf/internal/pushhandoff"
)

func defaults(t *testing.T) pushhandoff.Inputs {
	t.Helper()
	in, err := pushhandoff.Defaults()
	if err != nil {
		t.Fatal(err)
	}
	return in
}

// TestPlan_DefaultsWithinRateLimits: with the default configuration each sender plans at most
// BudgetShare of rl.message, evenly spaced (never a burst above ratelimit.rl.message.burst); the
// sender count is minimal; the other counts follow the formulas; Window holds MinSamples.
func TestPlan_DefaultsWithinRateLimits(t *testing.T) {
	t.Parallel()
	in := defaults(t)
	p, err := pushhandoff.NewPlan(in)
	if err != nil {
		t.Fatal(err)
	}
	budget := pushhandoff.BudgetShare * float64(in.PerMinute)
	if p.SenderPerMinute > budget+1e-9 {
		t.Errorf("a sender plans %.2f messages per minute, more than %.2f (BudgetShare of %d)", p.SenderPerMinute, budget, in.PerMinute)
	}
	if fewer := in.Rate / float64(p.Senders-1) * 60; p.Senders > 1 && fewer <= budget {
		t.Errorf("%d senders are not minimal: %d would plan %.2f per minute", p.Senders, p.Senders-1, fewer)
	}
	// Evenly spaced: between two sends of one sender lies at least the refill time of one token
	// of the limit, so the planned burst is one message, at most ratelimit.rl.message.burst.
	if refill := 60 / float64(in.PerMinute); p.SenderInterval().Seconds() < refill || in.Burst < 1 {
		t.Errorf("sender interval %s below the refill time %.2fs of one token, burst %d", p.SenderInterval(), refill, in.Burst)
	}
	if p.Channels != int(math.Ceil(float64(p.Senders)/pushhandoff.SendersPerChannel)) || p.Recipients != p.Channels*pushhandoff.RecipientsPerChannel {
		t.Errorf("channels %d, recipients %d for %d senders", p.Channels, p.Recipients, p.Senders)
	}
	if p.MentionRate != in.Rate/pushhandoff.MentionEvery || p.PushesPerSecond != p.MentionRate*pushhandoff.MentionsPerMessage {
		t.Errorf("mention rate %v, pushes per second %v for rate %v", p.MentionRate, p.PushesPerSecond, in.Rate)
	}
	if p.ExpectedSamples < pushhandoff.MinSamples {
		t.Errorf("Window holds %.0f expected pushes, fewer than MinSamples", p.ExpectedSamples)
	}
	msgs, err := p.Schedule(p.Total())
	if err != nil {
		t.Fatal(err)
	}
	perSender := make([]int, p.Senders)
	mentioned := make([]int, p.Recipients)
	for _, m := range msgs {
		perSender[m.Sender]++
		if m.Channel != m.Sender/pushhandoff.SendersPerChannel {
			t.Fatalf("message %d of sender %d in channel %d", m.Index, m.Sender, m.Channel)
		}
		for _, r := range m.Mentions {
			if r/pushhandoff.RecipientsPerChannel != m.Channel {
				t.Fatalf("message %d in channel %d mentions recipient %d of another channel", m.Index, m.Channel, r)
			}
			mentioned[r]++
		}
	}
	minutes := (p.Warmup + p.Window).Minutes()
	for s, n := range perSender {
		if float64(n) > budget*minutes+1 {
			t.Fatalf("sender %d sends %d messages in %.1f minutes, above BudgetShare", s, n, minutes)
		}
	}
	for r, n := range mentioned {
		if n == 0 {
			t.Fatalf("recipient %d is never mentioned", r)
		}
	}
}

// TestPlan_FollowsKeys: the plan follows the keys it reads, and a Window too short for
// MinSamples is an error naming the constants.
func TestPlan_FollowsKeys(t *testing.T) {
	t.Parallel()
	in := defaults(t)
	base, err := pushhandoff.NewPlan(in)
	if err != nil {
		t.Fatal(err)
	}
	half := in
	half.PerMinute /= 2
	p, err := pushhandoff.NewPlan(half)
	if err != nil {
		t.Fatal(err)
	}
	if p.Senders != 2*base.Senders {
		t.Errorf("halving %d per minute: %d senders, want %d", in.PerMinute, p.Senders, 2*base.Senders)
	}
	double := in
	double.Rate *= 2
	if p, err = pushhandoff.NewPlan(double); err != nil || p.MentionRate != 2*base.MentionRate {
		t.Errorf("doubling the rate: mention rate %v, %v; want %v", p.MentionRate, err, 2*base.MentionRate)
	}
	short := in
	short.Window = 1
	_, err = pushhandoff.NewPlan(short)
	if err == nil || !strings.Contains(err.Error(), "MinSamples") || !strings.Contains(err.Error(), "Window") || !strings.Contains(err.Error(), "MentionEvery") {
		t.Errorf("a short Window: %v, want an error naming Window, MinSamples and MentionEvery", err)
	}
}
