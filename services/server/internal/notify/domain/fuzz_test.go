// SPDX-License-Identifier: Apache-2.0

package domain_test

import (
	"reflect"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/notify/domain"
)

// fuzzInput is the decoded fuzz input; build turns it into facts the same way every time.
type fuzzInput struct {
	kind, threadLevel, instanceDefault uint8
	mutedMinutes                       int16
	flags                              uint16
	nowMinutes                         uint16
	weekdays                           uint8
	start, end                         uint16
	devices                            []byte
	windowSeconds                      int16
	coalesceSeconds, delaySeconds      uint8
}

const (
	flagAllowed = 1 << iota
	flagAuthor
	flagRunStarter
	flagAdminEnabled
	flagBreaksQuiet
	flagRuleEnabled
	flagHasRule
	flagWrongDM
)

func (f fuzzInput) has(flag uint16) bool { return f.flags&flag != 0 }

// level picks a level from a byte; 3 means absent.
func pickLevel(b uint8) domain.Level {
	levels := domain.Levels()
	if int(b%4) < len(levels) {
		return levels[b%4]
	}
	return ""
}

func (f fuzzInput) build(loc *time.Location) domain.RecipientFacts {
	kinds := domain.Kinds()
	kind := kinds[int(f.kind)%len(kinds)]
	now := goldenNow.Add(time.Duration(f.nowMinutes%(7*24*60)) * time.Minute)
	in := domain.RecipientFacts{
		Kind:       kind,
		Now:        now,
		Allowed:    f.has(flagAllowed),
		Author:     f.has(flagAuthor),
		RunStarter: f.has(flagRunStarter),
		Switches:   domain.Switches{AdminEnabled: f.has(flagAdminEnabled), InputNeededBreaksQuietHours: f.has(flagBreaksQuiet)},
		Policy: domain.Policy{
			CoalesceWindow: time.Duration(f.coalesceSeconds) * time.Second,
			MobileDelay:    time.Duration(f.delaySeconds) * time.Second,
		},
	}
	if kind.HasThread() {
		in.ThreadID = threadID
		in.Level = domain.LevelFacts{InstanceDefault: pickLevel(f.instanceDefault % 3)}
		in.Level.DirectMessage = (kind == domain.KindDM) != f.has(flagWrongDM)
		if l := pickLevel(f.threadLevel); l != "" {
			row := &domain.Preference{Level: l}
			if f.mutedMinutes != 0 {
				row.MutedUntil = now.Add(time.Duration(f.mutedMinutes) * time.Minute)
			}
			in.Level.Thread = row
		}
		if f.windowSeconds != 0 {
			in.Window = domain.Window{End: now.Add(time.Duration(f.windowSeconds) * time.Second)}
		}
	}
	if f.has(flagHasRule) {
		in.QuietHours = []domain.QuietRule{{
			Weekdays: f.weekdays%127 + 1,
			Start:    domain.LocalTime(f.start % 1440),
			End:      domain.LocalTime(f.end % 1440),
			Location: loc,
			Enabled:  f.has(flagRuleEnabled),
		}}
	}
	platforms := domain.Platforms()
	for i, b := range f.devices {
		if i == 4 {
			break
		}
		d := domain.Device{ID: testID(0x40 + byte(i)), Platform: platforms[int(b&7)%len(platforms)]}
		if b&8 != 0 {
			focus := otherThreadID
			if b&32 != 0 {
				focus = threadID
			}
			d.Connection = &domain.Connection{Focus: []ids.ID{focus}, Idle: b&16 != 0}
		}
		if b&64 != 0 {
			d.Registrations = []ids.ID{testID(0x50 + byte(i))}
		}
		in.Devices = append(in.Devices, d)
	}
	return in
}

// FuzzDecide holds the invariants of design notes 3 to 8 for any valid input.
func FuzzDecide(f *testing.F) {
	// Seeds from the golden table: a message under all, a muted input_needed in quiet hours, an
	// admin hint without the switch, a mention in an open window with an active desktop.
	f.Add(uint8(0), uint8(0), uint8(1), int16(0), uint16(flagAllowed|flagRunStarter), uint16(0), uint8(127), uint16(1320), uint16(420), []byte{64, 8}, int16(0), uint8(60), uint8(30))
	f.Add(uint8(5), uint8(2), uint8(1), int16(60), uint16(flagAllowed|flagRunStarter|flagHasRule|flagRuleEnabled), uint16(0), uint8(126), uint16(1320), uint16(420), []byte{64, 10}, int16(30), uint8(60), uint8(30))
	f.Add(uint8(7), uint8(0), uint8(1), int16(0), uint16(flagAllowed), uint16(0), uint8(1), uint16(0), uint16(0), []byte{64}, int16(0), uint8(60), uint8(30))
	f.Add(uint8(1), uint8(1), uint8(1), int16(0), uint16(flagAllowed|flagRunStarter|flagAuthor), uint16(30), uint8(127), uint16(0), uint16(0), []byte{64, 10, 105, 43}, int16(30), uint8(60), uint8(30))
	// No device in whole-day quiet hours: none with no_route, which must not carry the quiet flag.
	f.Add(uint8(0), uint8(0), uint8(1), int16(0), uint16(flagAllowed|flagRunStarter|flagHasRule|flagRuleEnabled), uint16(0), uint8(126), uint16(0), uint16(0), []byte{}, int16(0), uint8(60), uint8(30))
	f.Add(uint8(0), uint8(0), uint8(0), int16(0), uint16(flagAllowed), uint16(0), uint8(0), uint16(0), uint16(0), []byte{64, 74}, int16(-5), uint8(0), uint8(0))
	loc, err := time.LoadLocation("Europe/Berlin")
	if err != nil {
		f.Fatal(err)
	}
	f.Fuzz(func(t *testing.T, kind, threadLevel, instanceDefault uint8, mutedMinutes int16, flags, nowMinutes uint16,
		weekdays uint8, start, end uint16, devices []byte, windowSeconds int16, coalesceSeconds, delaySeconds uint8,
	) {
		fi := fuzzInput{kind, threadLevel, instanceDefault, mutedMinutes, flags, nowMinutes, weekdays, start, end, devices, windowSeconds, coalesceSeconds, delaySeconds}
		in := fi.build(loc)
		d, err := domain.Decide(in)
		if err != nil {
			if !reflect.DeepEqual(d, domain.Decision{}) {
				t.Fatalf("error %v with a non-zero decision %+v", err, d)
			}
			return
		}
		if !reflect.DeepEqual(in, fi.build(loc)) {
			t.Fatal("Decide changed its input")
		}
		again, err := domain.Decide(in)
		if err != nil || !reflect.DeepEqual(d, again) {
			t.Fatalf("second call differs: %+v, %v", again, err)
		}
		checkInvariants(t, in, d)
	})
}

func checkInvariants(t *testing.T, in domain.RecipientFacts, d domain.Decision) {
	t.Helper()
	threadKind := in.Kind.HasThread()
	windowOpen := threadKind && !in.Window.End.IsZero() && in.Now.Before(in.Window.End)
	if (d.Outcome == domain.OutcomeNone) != (d.Skip != "") {
		t.Fatalf("outcome %s with skip %q", d.Outcome, d.Skip)
	}
	switch d.Outcome {
	case domain.OutcomeSend:
		if len(d.Routes) == 0 || d.Count != 1 {
			t.Fatalf("send without routes or with count %d: %+v", d.Count, d)
		}
	case domain.OutcomeCollect:
		if in.Kind != domain.KindMessage || !windowOpen || d.Routes != nil {
			t.Fatalf("collect for %s, window open %v, routes %v", in.Kind, windowOpen, d.Routes)
		}
	case domain.OutcomeNone:
		if d.Routes != nil || d.Quiet || !d.OpensWindowUntil.IsZero() {
			t.Fatalf("none carries a send field: %+v", d)
		}
	}
	delivers := d.Outcome != domain.OutcomeNone
	if threadKind && delivers {
		eff, err := domain.ResolveLevel(in.Level, in.Now)
		if err != nil || eff != d.Effective {
			t.Fatalf("effective %+v, ResolveLevel %+v, %v", d.Effective, eff, err)
		}
		if eff.Level == domain.LevelMute && in.Kind != domain.KindInputNeeded {
			t.Fatalf("mute delivers %s", in.Kind)
		}
		if eff.Level == domain.LevelMentions && in.Kind == domain.KindMessage {
			t.Fatal("mentions delivers message")
		}
		if !in.Allowed {
			t.Fatal("delivers without push.deliver")
		}
	}
	quiet, err := domain.InQuietHours(in.QuietHours, in.Now)
	if err != nil {
		t.Fatal(err)
	}
	wantQuiet := d.Outcome == domain.OutcomeSend && quiet && (in.Kind != domain.KindInputNeeded || !in.Switches.InputNeededBreaksQuietHours)
	if d.Quiet != wantQuiet {
		t.Fatalf("quiet %v, want %v", d.Quiet, wantQuiet)
	}
	desktopActive := false
	for _, dev := range in.Devices {
		if dev.Connection == nil || dev.Connection.Idle {
			continue
		}
		desktopActive = desktopActive || dev.Platform.Desktop()
		for _, f := range dev.Connection.Focus {
			if threadKind && f == in.ThreadID && delivers {
				t.Fatalf("delivers %s while %s is active in the thread", in.Kind, dev.Platform)
			}
		}
	}
	byID := map[ids.ID]domain.Device{}
	for _, dev := range in.Devices {
		byID[dev.ID] = dev
	}
	for _, r := range d.Routes {
		dev := byID[r.DeviceID]
		if r.Via == domain.ViaPushDelayed && (!dev.Platform.Mobile() || dev.Connection != nil || !desktopActive) {
			t.Fatalf("push_delayed to %s, connected %v, desktop active %v", dev.Platform, dev.Connection != nil, desktopActive)
		}
	}
	if !d.OpensWindowUntil.IsZero() && (!threadKind || windowOpen) {
		t.Fatalf("%s opens a window, open window %v", in.Kind, windowOpen)
	}
}
