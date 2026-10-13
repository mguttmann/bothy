// SPDX-License-Identifier: Apache-2.0

package domain_test

import (
	"reflect"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/notify/domain"
)

// TestFlush holds the end of a coalescing window (13 4.5, D-M3-17-5).
func TestFlush(t *testing.T) {
	t.Parallel()
	state := domain.FlushState{Collected: 3, Unread: 4}
	base := facts(domain.KindMessage)

	cases := []struct {
		name  string
		edit  func(in *domain.RecipientFacts)
		state domain.FlushState
		skip  domain.SkipReason
	}{
		{"nothing collected", func(*domain.RecipientFacts) {}, domain.FlushState{Collected: 0, Unread: 4}, domain.SkipNothingCollected},
		{"nothing collected before read", func(*domain.RecipientFacts) {}, domain.FlushState{Collected: 0, Unread: 0}, domain.SkipNothingCollected},
		{"all read", func(*domain.RecipientFacts) {}, domain.FlushState{Collected: 3, Unread: 0}, domain.SkipRead},
		{"unread below zero", func(*domain.RecipientFacts) {}, domain.FlushState{Collected: 3, Unread: -1}, domain.SkipRead},
		{"level changed", func(in *domain.RecipientFacts) { in.Level.Thread = threadRow(domain.LevelMentions, time.Time{}) }, state, domain.SkipLevel},
		{"active in thread", func(in *domain.RecipientFacts) {
			in.Devices = []domain.Device{phone(), connected(desktop(), false, threadID)}
		}, state, domain.SkipActiveInThread},
		{"not allowed", func(in *domain.RecipientFacts) { in.Allowed = false }, state, domain.SkipNotAllowed},
		{"no route", func(in *domain.RecipientFacts) { in.Devices = []domain.Device{desktop()} }, state, domain.SkipNoRoute},
	}
	for _, c := range cases {
		in := base
		c.edit(&in)
		d, err := domain.Flush(in, c.state)
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		if d.Outcome != domain.OutcomeNone || d.Skip != c.skip || d.Routes != nil || !d.OpensWindowUntil.IsZero() {
			t.Errorf("%s: got %+v, want none %s", c.name, d, c.skip)
		}
	}

	// A send carries the unread count and opens the next window.
	in := base
	in.Author = true // a bundle has no single author
	in.Window = domain.Window{End: weekdayNoon.Add(time.Hour)}
	d, err := domain.Flush(in, state)
	if err != nil {
		t.Fatal(err)
	}
	want := domain.Decision{
		Outcome: domain.OutcomeSend, Kind: domain.KindMessage,
		Effective:        domain.EffectiveLevel{Level: domain.LevelAll, Source: domain.SourceInstanceDefault},
		Count:            4,
		OpensWindowUntil: weekdayNoon.Add(fixturePolicy.CoalesceWindow),
		Routes:           pushToPhone,
	}
	if !reflect.DeepEqual(d, want) {
		t.Errorf("send: got %+v, want %+v", d, want)
	}

	// Quiet by the quiet hours at the flush time.
	in.Now = quietNow
	in.QuietHours = []domain.QuietRule{nightRule(t)}
	if d, err := domain.Flush(in, state); err != nil || !d.Quiet || d.Outcome != domain.OutcomeSend {
		t.Errorf("flush in quiet hours: got %+v, %v", d, err)
	}

	// A zero window length opens nothing.
	in = base
	in.Policy.CoalesceWindow = 0
	if d, err := domain.Flush(in, state); err != nil || d.Outcome != domain.OutcomeSend || !d.OpensWindowUntil.IsZero() {
		t.Errorf("flush with a zero window: got %+v, %v", d, err)
	}
}

// TestFlush_InvalidInput: Flush validates like Decide and only for message.
func TestFlush_InvalidInput(t *testing.T) {
	t.Parallel()
	state := domain.FlushState{Collected: 3, Unread: 4}
	cases := map[string]struct {
		in    domain.RecipientFacts
		state domain.FlushState
	}{
		"kind mention":       {facts(domain.KindMention), state},
		"kind security":      {facts(domain.KindSecurity), state},
		"negative collected": {facts(domain.KindMessage), domain.FlushState{Collected: -1, Unread: 4}},
	}
	// Every broken input of Decide whose defect does not depend on its kind is also broken as a
	// message, so it is checked for Flush as one.
	kindBound := map[string]bool{
		"unknown kind": true, "empty kind": true, "security with a thread": true,
		"admin with a thread": true, "dm outside a dm channel": true, "mention in a dm channel": true,
		"run_done in a dm channel": true,
	}
	for name, in := range invalidCases(t) {
		if kindBound[name] {
			continue
		}
		if in.Kind != domain.KindMessage {
			if !in.Kind.HasThread() {
				in.ThreadID = threadID
				in.Level = domain.LevelFacts{InstanceDefault: domain.LevelAll}
			}
			in.Kind = domain.KindMessage
		}
		cases["decide case "+name] = struct {
			in    domain.RecipientFacts
			state domain.FlushState
		}{in, state}
	}
	if len(cases) < 3+len(invalidCases(t))-len(kindBound) {
		t.Fatalf("only %d cases", len(cases))
	}
	for name, c := range cases {
		d, err := domain.Flush(c.in, c.state)
		assertInternal(t, name, err)
		if !reflect.DeepEqual(d, domain.Decision{}) {
			t.Errorf("%s: got %+v, want the zero Decision", name, d)
		}
		// Validation comes before nothing_collected.
		c.state.Collected = 0
		if _, err := domain.Flush(c.in, c.state); err == nil && name != "negative collected" {
			t.Errorf("%s: no error with nothing collected", name)
		}
	}
}

// TestFlush_TenMessagesInOneWindow mirrors the M3-19 case of 13 14 "Ten messages in one
// thread within one window": at most two hints, the second with the count.
func TestFlush_TenMessagesInOneWindow(t *testing.T) {
	t.Parallel()
	var window domain.Window
	sends, collects := 0, 0
	for i := range 10 {
		in := facts(domain.KindMessage)
		in.Now = weekdayNoon.Add(time.Duration(i*5) * time.Second)
		in.Window = window
		d := mustDecide(t, in)
		switch d.Outcome {
		case domain.OutcomeSend:
			sends++
			window = domain.Window{End: d.OpensWindowUntil}
		case domain.OutcomeCollect:
			collects++
		case domain.OutcomeNone:
			t.Fatalf("message %d: none %s", i, d.Skip)
		}
	}
	if sends != 1 || collects != 9 {
		t.Fatalf("got %d sends and %d collects, want 1 and 9", sends, collects)
	}
	in := facts(domain.KindMessage)
	in.Now = window.End
	d, err := domain.Flush(in, domain.FlushState{Collected: collects, Unread: 10})
	if err != nil {
		t.Fatal(err)
	}
	if d.Outcome != domain.OutcomeSend || d.Count != 10 {
		t.Errorf("flush: got %+v, want send with count 10", d)
	}
}
