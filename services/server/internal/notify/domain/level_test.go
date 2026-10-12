// SPDX-License-Identifier: Apache-2.0

package domain_test

import (
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/notify/domain"
)

// TestResolveLevel_Order walks the seven steps of 13 3.1; each case removes the sources above it.
func TestResolveLevel_Order(t *testing.T) {
	t.Parallel()
	full := domain.LevelFacts{
		Thread:          threadRow(domain.LevelMute, time.Time{}),
		Channel:         threadRow(domain.LevelMentions, time.Time{}),
		Space:           threadRow(domain.LevelAll, time.Time{}),
		ChannelDefault:  domain.LevelMute,
		DirectMessage:   true,
		PersonalDefault: domain.LevelMentions,
		InstanceDefault: domain.LevelMute,
	}
	steps := []struct {
		name   string
		strip  func(f *domain.LevelFacts)
		level  domain.Level
		source domain.LevelSource
	}{
		{"thread row", func(*domain.LevelFacts) {}, domain.LevelMute, domain.SourceThread},
		{"channel row", func(f *domain.LevelFacts) { f.Thread = nil }, domain.LevelMentions, domain.SourceChannel},
		{"space row", func(f *domain.LevelFacts) { f.Channel = nil }, domain.LevelAll, domain.SourceSpace},
		{"channel default", func(f *domain.LevelFacts) { f.Space = nil }, domain.LevelMute, domain.SourceChannelDefault},
		{"dm channel", func(f *domain.LevelFacts) { f.ChannelDefault = "" }, domain.LevelAll, domain.SourceDirectMessage},
		{"personal default", func(f *domain.LevelFacts) { f.DirectMessage = false }, domain.LevelMentions, domain.SourcePersonalDefault},
		{"instance default", func(f *domain.LevelFacts) { f.PersonalDefault = "" }, domain.LevelMute, domain.SourceInstanceDefault},
	}
	f := full
	for _, s := range steps {
		s.strip(&f)
		got, err := domain.ResolveLevel(f, weekdayNoon)
		if err != nil {
			t.Fatalf("%s: %v", s.name, err)
		}
		if got.Level != s.level || got.Source != s.source || got.Muted {
			t.Errorf("%s: got %+v, want level %s source %s not muted", s.name, got, s.level, s.source)
		}
	}

	// Step 4 comes before step 5: a DM channel with a channel default uses the default.
	dm := domain.LevelFacts{ChannelDefault: domain.LevelMentions, DirectMessage: true, InstanceDefault: domain.LevelMute}
	got, err := domain.ResolveLevel(dm, weekdayNoon)
	if err != nil || got.Level != domain.LevelMentions || got.Source != domain.SourceChannelDefault {
		t.Errorf("dm with channel default: got %+v, %v", got, err)
	}
}

// TestResolveLevel_MutedUntil covers muted_until on the chosen row only, strictly after now.
func TestResolveLevel_MutedUntil(t *testing.T) {
	t.Parallel()
	now := weekdayNoon
	cases := []struct {
		name  string
		facts domain.LevelFacts
		level domain.Level
		muted bool
	}{
		{"muted one second ahead", domain.LevelFacts{Thread: threadRow(domain.LevelAll, now.Add(time.Second)), InstanceDefault: domain.LevelAll}, domain.LevelMute, true},
		{"muted until now", domain.LevelFacts{Thread: threadRow(domain.LevelAll, now), InstanceDefault: domain.LevelAll}, domain.LevelAll, false},
		{"muted until the past", domain.LevelFacts{Thread: threadRow(domain.LevelAll, now.Add(-time.Hour)), InstanceDefault: domain.LevelAll}, domain.LevelAll, false},
		{
			"space row muted under a thread row",
			domain.LevelFacts{
				Thread:          threadRow(domain.LevelMentions, time.Time{}),
				Space:           threadRow(domain.LevelAll, now.Add(time.Hour)),
				InstanceDefault: domain.LevelAll,
			},
			domain.LevelMentions, false,
		},
		{"stored mute with past muted_until", domain.LevelFacts{Thread: threadRow(domain.LevelMute, now.Add(-time.Hour)), InstanceDefault: domain.LevelAll}, domain.LevelMute, false},
		{"stored mute with future muted_until", domain.LevelFacts{Thread: threadRow(domain.LevelMute, now.Add(time.Hour)), InstanceDefault: domain.LevelAll}, domain.LevelMute, true},
		{"muted channel row", domain.LevelFacts{Channel: threadRow(domain.LevelAll, now.Add(time.Minute)), InstanceDefault: domain.LevelAll}, domain.LevelMute, true},
	}
	for _, c := range cases {
		got, err := domain.ResolveLevel(c.facts, now)
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		if got.Level != c.level || got.Muted != c.muted {
			t.Errorf("%s: got %+v, want level %s muted %v", c.name, got, c.level, c.muted)
		}
	}
}

// TestResolveLevel_InvalidInput covers the cases of design note 10 that reach ResolveLevel.
func TestResolveLevel_InvalidInput(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		facts domain.LevelFacts
		now   time.Time
	}{
		"zero now":                 {domain.LevelFacts{InstanceDefault: domain.LevelAll}, time.Time{}},
		"empty instance default":   {domain.LevelFacts{}, weekdayNoon},
		"unknown instance default": {domain.LevelFacts{InstanceDefault: "loud"}, weekdayNoon},
		"unknown thread level":     {domain.LevelFacts{Thread: threadRow("loud", time.Time{}), InstanceDefault: domain.LevelAll}, weekdayNoon},
		"empty channel row level":  {domain.LevelFacts{Channel: threadRow("", time.Time{}), InstanceDefault: domain.LevelAll}, weekdayNoon},
		"unknown space level":      {domain.LevelFacts{Space: threadRow("ALL", time.Time{}), InstanceDefault: domain.LevelAll}, weekdayNoon},
		"unknown channel default":  {domain.LevelFacts{ChannelDefault: "none", InstanceDefault: domain.LevelAll}, weekdayNoon},
		"unknown personal default": {domain.LevelFacts{PersonalDefault: "x", InstanceDefault: domain.LevelAll}, weekdayNoon},
		// A broken row below the chosen one still fails: the input is broken, whichever row wins.
		"unknown level below the chosen row": {domain.LevelFacts{Thread: threadRow(domain.LevelAll, time.Time{}), Space: threadRow("x", time.Time{}), InstanceDefault: domain.LevelAll}, weekdayNoon},
	}
	for name, c := range cases {
		got, err := domain.ResolveLevel(c.facts, c.now)
		assertInternal(t, name, err)
		if got != (domain.EffectiveLevel{}) {
			t.Errorf("%s: got %+v, want the zero EffectiveLevel", name, got)
		}
	}
}
