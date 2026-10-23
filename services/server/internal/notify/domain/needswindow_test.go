// SPDX-License-Identifier: Apache-2.0

package domain_test

import (
	"reflect"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/notify/domain"
)

// TestNeedsWindowMatchesDecide (M3-24 design note 14): whenever NeedsWindow is false, Decide gives
// the same decision with and without the coalescing window, so the push path may leave the window
// unread for such a recipient. The inputs enumerate every kind, thread level, instance default,
// the switches that decide the steps before the window, devices with and without focus, and an
// open, a closed and no window.
func TestNeedsWindowMatchesDecide(t *testing.T) {
	t.Parallel()
	loc, err := time.LoadLocation("Europe/Berlin")
	if err != nil {
		t.Fatal(err)
	}
	flagSets := []uint16{0}
	for _, f := range []uint16{flagAllowed, flagAuthor, flagRunStarter, flagAdminEnabled, flagWrongDM} {
		for _, s := range flagSets {
			flagSets = append(flagSets, s|f)
		}
	}
	needed, skipped := 0, 0
	for kind := uint8(0); int(kind) < len(domain.Kinds()); kind++ {
		for level := range uint8(4) {
			for def := range uint8(3) {
				for _, flags := range flagSets {
					for _, devices := range [][]byte{nil, {64}, {64 | 8 | 32}, {64 | 8 | 32 | 16}} {
						for _, window := range []int16{0, 30, -5} {
							fi := fuzzInput{kind: kind, threadLevel: level, instanceDefault: def, flags: flags, devices: devices,
								windowSeconds: window, coalesceSeconds: 60, delaySeconds: 30}
							in := fi.build(loc)
							if domain.NeedsWindow(in) {
								needed++
								continue
							}
							skipped++
							with, errWith := domain.Decide(in)
							without := in
							without.Window = domain.Window{}
							got, errWithout := domain.Decide(without)
							if (errWith == nil) != (errWithout == nil) || !reflect.DeepEqual(with, got) {
								t.Fatalf("NeedsWindow false but the window changes the decision for %+v: %+v (%v) vs %+v (%v)", fi, with, errWith, got, errWithout)
							}
						}
					}
				}
			}
		}
	}
	if needed == 0 || skipped == 0 {
		t.Fatalf("the enumeration covers only one side: %d need the window, %d do not", needed, skipped)
	}
}

// TestNeedsWindow: a message under the level mentions and a recipient without push.deliver
// decide without the window; a mention under mentions and a message under all need it; kinds
// without a thread never do.
func TestNeedsWindow(t *testing.T) {
	t.Parallel()
	base := domain.RecipientFacts{Now: goldenNow, Allowed: true, ThreadID: threadID,
		Level: domain.LevelFacts{InstanceDefault: domain.LevelMentions}, Policy: domain.Policy{CoalesceWindow: time.Minute}}
	cases := []struct {
		name string
		edit func(*domain.RecipientFacts)
		want bool
	}{
		{"message under mentions", func(in *domain.RecipientFacts) { in.Kind = domain.KindMessage }, false},
		{"mention under mentions", func(in *domain.RecipientFacts) { in.Kind = domain.KindMention }, true},
		{"message under all", func(in *domain.RecipientFacts) {
			in.Kind, in.Level.InstanceDefault = domain.KindMessage, domain.LevelAll
		}, true},
		{"mention without push.deliver", func(in *domain.RecipientFacts) { in.Kind, in.Allowed = domain.KindMention, false }, false},
		{"mention to its author", func(in *domain.RecipientFacts) { in.Kind, in.Author = domain.KindMention, true }, false},
		{"security", func(in *domain.RecipientFacts) {
			in.Kind, in.ThreadID, in.Level = domain.KindSecurity, threadID, domain.LevelFacts{}
		}, false},
	}
	for _, c := range cases {
		in := base
		c.edit(&in)
		if got := domain.NeedsWindow(in); got != c.want {
			t.Errorf("%s: NeedsWindow = %v, want %v", c.name, got, c.want)
		}
	}
}
