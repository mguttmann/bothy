// SPDX-License-Identifier: Apache-2.0

package domain_test

import (
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/notify/domain"
)

// quietNow is Friday 2026-10-23, 23:30 in Berlin: inside nightRule.
var quietNow = time.Date(2026, 10, 23, 21, 30, 0, 0, time.UTC)

// pushToPhone is the single route of facts(): the phone has no connection, the desktop neither.
var pushToPhone = []domain.Route{{DeviceID: phoneID, Via: domain.ViaPush, Registrations: []ids.ID{phoneReg}}}

// TestDecide_InputNeededMutedQuietHours: input_needed under mute arrives, quietly in quiet hours
// (PLAN row M3-17, 13 14 "input_needed bei mute", R-PN-03, R-PN-05).
func TestDecide_InputNeededMutedQuietHours(t *testing.T) {
	t.Parallel()
	mutes := map[string]*domain.Preference{
		"stored mute":       threadRow(domain.LevelMute, time.Time{}),
		"muted_until ahead": threadRow(domain.LevelAll, quietNow.Add(time.Hour)),
	}
	for name, row := range mutes {
		in := facts(domain.KindInputNeeded)
		in.Now = quietNow
		in.Level.Thread = row
		in.QuietHours = []domain.QuietRule{nightRule(t)}

		d := mustDecide(t, in)
		if d.Outcome != domain.OutcomeSend || !d.Quiet || len(d.Routes) == 0 || d.Effective.Level != domain.LevelMute {
			t.Errorf("%s in quiet hours: got %+v, want a quiet send under mute", name, d)
		}

		breaks := in
		breaks.Switches.InputNeededBreaksQuietHours = true
		if d := mustDecide(t, breaks); d.Outcome != domain.OutcomeSend || d.Quiet {
			t.Errorf("%s with the switch: got %+v, want a normal send", name, d)
		}

		day := in
		day.Now = quietNow.Add(-12 * time.Hour)
		if row.MutedUntil.After(day.Now) || row.MutedUntil.IsZero() {
			if d := mustDecide(t, day); d.Outcome != domain.OutcomeSend || d.Quiet {
				t.Errorf("%s outside quiet hours: got %+v, want a normal send", name, d)
			}
		}

		sam := in
		sam.RunStarter = false
		if d := mustDecide(t, sam); d.Outcome != domain.OutcomeNone || d.Skip != domain.SkipNotRecipient {
			t.Errorf("%s for Sam: got %+v, want none not_recipient", name, d)
		}
	}
}

// TestDecide_AZM48_NotAllowed mirrors AZ-M48 at decision level: without push.deliver nothing goes.
func TestDecide_AZM48_NotAllowed(t *testing.T) {
	t.Parallel()
	for _, k := range domain.Kinds() {
		in := facts(k)
		in.Allowed = false
		d := mustDecide(t, in)
		want := domain.Decision{Outcome: domain.OutcomeNone, Skip: domain.SkipNotAllowed, Kind: k}
		if !reflect.DeepEqual(d, want) {
			t.Errorf("%s: got %+v, want %+v", k, d, want)
		}
	}
}

// TestDecide_Recipients holds the recipient column of 13 2.
func TestDecide_Recipients(t *testing.T) {
	t.Parallel()
	// The allowed step runs before the recipient step: a denied author or a denied non-starter is
	// not_allowed.
	for _, k := range domain.Kinds() {
		in := facts(k)
		in.Allowed, in.Author, in.RunStarter = false, true, false
		if d := mustDecide(t, in); d.Outcome != domain.OutcomeNone || d.Skip != domain.SkipNotAllowed {
			t.Errorf("denied %s that is no recipient either: got %+v, want not_allowed", k, d)
		}
	}
	for _, k := range []domain.Kind{domain.KindMessage, domain.KindMention, domain.KindDM} {
		in := facts(k)
		in.Author = true
		if d := mustDecide(t, in); d.Outcome != domain.OutcomeNone || d.Skip != domain.SkipNotRecipient {
			t.Errorf("author with %s: got %+v", k, d)
		}
		in.Author = false
		in.RunStarter = false
		if d := mustDecide(t, in); d.Outcome != domain.OutcomeSend {
			t.Errorf("%s does not depend on the run starter: got %+v", k, d)
		}
	}
	for _, k := range []domain.Kind{domain.KindRunDone, domain.KindRunFailed, domain.KindInputNeeded} {
		in := facts(k)
		in.RunStarter = false
		if d := mustDecide(t, in); d.Outcome != domain.OutcomeNone || d.Skip != domain.SkipNotRecipient {
			t.Errorf("%s for Sam: got %+v", k, d)
		}
		in.RunStarter = true
		in.Author = true
		if d := mustDecide(t, in); d.Outcome != domain.OutcomeSend {
			t.Errorf("%s does not depend on the author: got %+v", k, d)
		}
	}
	// Robin gets the admin hints; security and admin ignore both flags.
	for _, k := range otherKinds {
		for _, flags := range [][2]bool{{false, false}, {true, false}, {false, true}, {true, true}} {
			in := facts(k)
			in.Author, in.RunStarter = flags[0], flags[1]
			if d := mustDecide(t, in); d.Outcome != domain.OutcomeSend {
				t.Errorf("%s with author %v starter %v: got %+v", k, flags[0], flags[1], d)
			}
		}
	}
}

// TestDecide_LevelAndKind holds the table of 13 4.3.
func TestDecide_LevelAndKind(t *testing.T) {
	t.Parallel()
	allows := map[domain.Level][]domain.Kind{
		domain.LevelAll:      threadKinds,
		domain.LevelMentions: {domain.KindMention, domain.KindDM, domain.KindRunDone, domain.KindRunFailed, domain.KindInputNeeded},
		domain.LevelMute:     {domain.KindInputNeeded},
	}
	for _, level := range domain.Levels() {
		for _, k := range threadKinds {
			in := facts(k)
			in.Level.Thread = threadRow(level, time.Time{})
			d := mustDecide(t, in)
			if d.Effective.Level != level || d.Effective.Source != domain.SourceThread {
				t.Errorf("%s at %s: effective %+v", k, level, d.Effective)
			}
			if slices.Contains(allows[level], k) {
				if d.Outcome != domain.OutcomeSend {
					t.Errorf("%s at %s: got %+v, want send", k, level, d)
				}
			} else if d.Outcome != domain.OutcomeNone || d.Skip != domain.SkipLevel {
				t.Errorf("%s at %s: got %+v, want none level", k, level, d)
			}
		}
		// Security and admin have no thread: no level is resolved, the stored row is ignored.
		for _, k := range otherKinds {
			in := facts(k)
			in.Level = domain.LevelFacts{Thread: threadRow(level, time.Time{}), InstanceDefault: level}
			d := mustDecide(t, in)
			if d.Outcome != domain.OutcomeSend || d.Effective != (domain.EffectiveLevel{}) {
				t.Errorf("%s at %s: got %+v, want send without an effective level", k, level, d)
			}
		}
	}
	in := facts(domain.KindAdmin)
	in.Switches.AdminEnabled = false
	if d := mustDecide(t, in); d.Outcome != domain.OutcomeNone || d.Skip != domain.SkipAdminDisabled {
		t.Errorf("admin without the switch: got %+v", d)
	}
	sec := facts(domain.KindSecurity)
	sec.Switches.AdminEnabled = false
	if d := mustDecide(t, sec); d.Outcome != domain.OutcomeSend {
		t.Errorf("security does not depend on the admin switch: got %+v", d)
	}
}

// TestDecide_ActiveInThread holds 13 4.4 "Im Thread aktiv" and mirrors the M3-23 case
// "Thread im Fokus auf dem Desktop".
func TestDecide_ActiveInThread(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		devices []domain.Device
		active  bool
	}{
		{"desktop focusing the thread", []domain.Device{phone(), connected(desktop(), false, threadID)}, true},
		{"phone focusing the thread", []domain.Device{connected(phone(), false, otherThreadID, threadID), desktop()}, true},
		{"idle desktop focusing the thread", []domain.Device{phone(), connected(desktop(), true, threadID)}, false},
		{"desktop focusing another thread", []domain.Device{phone(), connected(desktop(), false, otherThreadID)}, false},
		{"console focusing the thread", []domain.Device{phone(), connected(console(), false, threadID)}, true},
	}
	for _, c := range cases {
		for _, k := range threadKinds {
			in := facts(k)
			in.Devices = c.devices
			d := mustDecide(t, in)
			if c.active && (d.Outcome != domain.OutcomeNone || d.Skip != domain.SkipActiveInThread) {
				t.Errorf("%s, %s: got %+v, want none active_in_thread", c.name, k, d)
			}
			if !c.active && d.Outcome != domain.OutcomeSend {
				t.Errorf("%s, %s: got %+v, want send", c.name, k, d)
			}
			if c.active && d.Effective.Level != domain.LevelAll {
				t.Errorf("%s, %s: the level step ran first, effective %+v", c.name, k, d.Effective)
			}
		}
	}
	// Hints without thread know no activity in a thread, even with a focus on the zero ID.
	for _, k := range otherKinds {
		in := facts(k)
		in.Devices = []domain.Device{phone(), connected(desktop(), false, threadID)}
		if d := mustDecide(t, in); d.Outcome != domain.OutcomeSend {
			t.Errorf("%s with a focused desktop: got %+v", k, d)
		}
	}
	// The level step comes first.
	in := facts(domain.KindMessage)
	in.Level.Thread = threadRow(domain.LevelMute, time.Time{})
	in.Devices = []domain.Device{connected(desktop(), false, threadID)}
	if d := mustDecide(t, in); d.Skip != domain.SkipLevel {
		t.Errorf("muted and active: got %+v, want none level", d)
	}
}

// TestDecide_Routes holds 13 4.4 "Desktop aktiv" and 4.6, and mirrors the M3-23 case
// "Desktop aktiv".
func TestDecide_Routes(t *testing.T) {
	t.Parallel()
	notBefore := weekdayNoon.Add(fixturePolicy.MobileDelay)
	gw := func(id ids.ID) domain.Route { return domain.Route{DeviceID: id, Via: domain.ViaGateway} }
	push := func(id, reg ids.ID) domain.Route {
		return domain.Route{DeviceID: id, Via: domain.ViaPush, Registrations: []ids.ID{reg}}
	}
	delayed := func(id, reg ids.ID) domain.Route {
		return domain.Route{DeviceID: id, Via: domain.ViaPushDelayed, Registrations: []ids.ID{reg}, NotBefore: notBefore}
	}
	cases := []struct {
		name    string
		devices []domain.Device
		policy  domain.Policy
		want    []domain.Route
	}{
		{"idle connections get the gateway", []domain.Device{connected(phone(), true), connected(desktop(), true)}, fixturePolicy, []domain.Route{gw(phoneID), gw(desktopID)}},
		{"desktop active delays mobiles", []domain.Device{phone(), connected(desktop(), false, otherThreadID), tablet()}, fixturePolicy, []domain.Route{delayed(phoneID, phoneReg), gw(desktopID), delayed(tabletID, tabletReg)}},
		{"laptop active delays mobiles", []domain.Device{tablet(), connected(laptop(), false), phone()}, fixturePolicy, []domain.Route{delayed(tabletID, tabletReg), gw(laptopID), delayed(phoneID, phoneReg)}},
		{"idle desktop delays nothing", []domain.Device{phone(), connected(desktop(), true)}, fixturePolicy, []domain.Route{push(phoneID, phoneReg), gw(desktopID)}},
		{"zero delay pushes at once", []domain.Device{phone(), connected(desktop(), false)}, domain.Policy{CoalesceWindow: time.Minute}, []domain.Route{push(phoneID, phoneReg), gw(desktopID)}},
		{"console with a connection", []domain.Device{connected(console(), false), phone()}, fixturePolicy, []domain.Route{gw(consoleID), push(phoneID, phoneReg)}},
		{"connected console does not delay", []domain.Device{phone(), connected(console(), false)}, fixturePolicy, []domain.Route{push(phoneID, phoneReg), gw(consoleID)}},
		{
			"registered desktop and laptop are never delayed",
			[]domain.Device{
				{ID: desktopID, Platform: domain.PlatformMacOS, Registrations: []ids.ID{desktopReg}},
				connected(console(), false),
				{ID: laptopID, Platform: domain.PlatformWindows, Registrations: []ids.ID{laptopReg}},
				connected(domain.Device{ID: workstationID, Platform: domain.PlatformWindows}, false),
			},
			fixturePolicy,
			[]domain.Route{push(desktopID, desktopReg), gw(consoleID), push(laptopID, laptopReg), gw(workstationID)},
		},
		{"device with neither is left out", []domain.Device{desktop(), phone(), console()}, fixturePolicy, []domain.Route{push(phoneID, phoneReg)}},
		{
			"every registration of a device", []domain.Device{{ID: phoneID, Platform: domain.PlatformIOS, Registrations: []ids.ID{phoneReg, tabletReg}}}, fixturePolicy,
			[]domain.Route{{DeviceID: phoneID, Via: domain.ViaPush, Registrations: []ids.ID{phoneReg, tabletReg}}},
		},
	}
	for _, c := range cases {
		in := facts(domain.KindMention)
		in.Devices = c.devices
		in.Policy = c.policy
		d := mustDecide(t, in)
		if d.Outcome != domain.OutcomeSend || !reflect.DeepEqual(d.Routes, c.want) {
			t.Errorf("%s: got %+v, want routes %+v", c.name, d, c.want)
		}
	}
	for _, k := range domain.Kinds() {
		in := facts(k)
		in.Devices = []domain.Device{desktop(), console()}
		d := mustDecide(t, in)
		if d.Outcome != domain.OutcomeNone || d.Skip != domain.SkipNoRoute || !d.OpensWindowUntil.IsZero() || d.Routes != nil {
			t.Errorf("%s without any route: got %+v, want none no_route and no window", k, d)
		}
		in.Devices = nil
		if d := mustDecide(t, in); d.Skip != domain.SkipNoRoute {
			t.Errorf("%s without devices: got %+v", k, d)
		}
	}
}

// TestDecide_Window holds 13 4.5 for Decide.
func TestDecide_Window(t *testing.T) {
	t.Parallel()
	opens := weekdayNoon.Add(fixturePolicy.CoalesceWindow)
	open := domain.Window{End: weekdayNoon.Add(30 * time.Second)}
	for _, k := range threadKinds {
		in := facts(k)
		d := mustDecide(t, in)
		if d.Outcome != domain.OutcomeSend || !d.OpensWindowUntil.Equal(opens) || d.Count != 1 || !reflect.DeepEqual(d.Routes, pushToPhone) {
			t.Errorf("%s without window: got %+v", k, d)
		}

		in.Window = open
		d = mustDecide(t, in)
		if k == domain.KindMessage {
			want := domain.Decision{Outcome: domain.OutcomeCollect, Kind: k, Effective: d.Effective}
			if !reflect.DeepEqual(d, want) || d.Effective.Level != domain.LevelAll {
				t.Errorf("message in an open window: got %+v, want collect without routes", d)
			}
		} else if d.Outcome != domain.OutcomeSend || !d.OpensWindowUntil.IsZero() || d.Count != 1 {
			t.Errorf("%s in an open window: got %+v, want send without a new window", k, d)
		}

		in.Window = domain.Window{End: weekdayNoon}
		if d := mustDecide(t, in); d.Outcome != domain.OutcomeSend || !d.OpensWindowUntil.Equal(opens) {
			t.Errorf("%s at the window end: got %+v, want send that opens", k, d)
		}

		in.Window = domain.Window{End: weekdayNoon.Add(-time.Second)}
		if d := mustDecide(t, in); d.Outcome != domain.OutcomeSend || !d.OpensWindowUntil.Equal(opens) {
			t.Errorf("%s after the window end: got %+v", k, d)
		}

		in.Window = domain.Window{}
		in.Policy.CoalesceWindow = 0
		if d := mustDecide(t, in); d.Outcome != domain.OutcomeSend || !d.OpensWindowUntil.IsZero() {
			t.Errorf("%s with a zero window: got %+v", k, d)
		}
		// A window opened before the length became zero is still open: message collects into it.
		in.Window = open
		want := domain.OutcomeSend
		if k == domain.KindMessage {
			want = domain.OutcomeCollect
		}
		if d := mustDecide(t, in); d.Outcome != want || !d.OpensWindowUntil.IsZero() {
			t.Errorf("%s with a zero window and an earlier open window: got %+v, want %s", k, d, want)
		}
	}
	for _, k := range otherKinds {
		in := facts(k)
		in.Window = open
		if d := mustDecide(t, in); d.Outcome != domain.OutcomeSend || !d.OpensWindowUntil.IsZero() || d.Count != 1 {
			t.Errorf("%s with a window: got %+v, want send without a window", k, d)
		}
	}
}

// TestDecide_QuietHours: quiet hours make a hint quiet, never drop it (R-PN-05).
func TestDecide_QuietHours(t *testing.T) {
	t.Parallel()
	for _, k := range domain.Kinds() {
		for _, breaks := range []bool{false, true} {
			in := facts(k)
			in.QuietHours = []domain.QuietRule{nightRule(t)}
			in.Switches.InputNeededBreaksQuietHours = breaks
			in.Now = quietNow
			d := mustDecide(t, in)
			wantQuiet := k != domain.KindInputNeeded || !breaks
			if d.Outcome != domain.OutcomeSend || d.Quiet != wantQuiet {
				t.Errorf("%s in quiet hours, switch %v: got %+v, want send quiet=%v", k, breaks, d, wantQuiet)
			}
			in.Now = weekdayNoon
			if d := mustDecide(t, in); d.Outcome != domain.OutcomeSend || d.Quiet {
				t.Errorf("%s outside quiet hours, switch %v: got %+v", k, breaks, d)
			}
		}
	}
	// A delayed push keeps the quiet flag of the decision (D-M3-17-4).
	in := facts(domain.KindMessage)
	in.Now = quietNow
	in.QuietHours = []domain.QuietRule{nightRule(t)}
	in.Devices = []domain.Device{phone(), connected(desktop(), false)}
	d := mustDecide(t, in)
	if !d.Quiet || len(d.Routes) != 2 || d.Routes[0].Via != domain.ViaPushDelayed {
		t.Errorf("delayed push in quiet hours: got %+v", d)
	}
}

// TestDecide_DoesNotMutateInput: the decision copies what it returns.
func TestDecide_DoesNotMutateInput(t *testing.T) {
	t.Parallel()
	in := facts(domain.KindMention)
	in.Devices = []domain.Device{
		{ID: phoneID, Platform: domain.PlatformIOS, Registrations: []ids.ID{phoneReg, tabletReg}},
		connected(desktop(), false, otherThreadID),
	}
	d := mustDecide(t, in)
	if d.Routes[0].Via != domain.ViaPushDelayed {
		t.Fatalf("route 0: got %s, want push_delayed", d.Routes[0].Via)
	}
	d.Routes[0].Registrations[0] = ids.ID{}
	if in.Devices[0].Registrations[0] != phoneReg {
		t.Error("the delayed route shares the registration slice of the input")
	}
	// The same for a route that pushes at once.
	in.Devices[1] = desktop()
	d = mustDecide(t, in)
	if d.Routes[0].Via != domain.ViaPush {
		t.Fatalf("route 0: got %s, want push", d.Routes[0].Via)
	}
	d.Routes[0].Registrations[0] = ids.ID{}
	if in.Devices[0].Registrations[0] != phoneReg {
		t.Error("the push route shares the registration slice of the input")
	}
}

// invalidCases lists every broken input of design note 10 for Decide.
func invalidCases(t *testing.T) map[string]domain.RecipientFacts {
	t.Helper()
	with := func(k domain.Kind, f func(in *domain.RecipientFacts)) domain.RecipientFacts {
		in := facts(k)
		f(&in)
		return in
	}
	return map[string]domain.RecipientFacts{
		"unknown kind":           with(domain.KindMessage, func(in *domain.RecipientFacts) { in.Kind = "reaction" }),
		"empty kind":             with(domain.KindMessage, func(in *domain.RecipientFacts) { in.Kind = "" }),
		"unknown level":          with(domain.KindMessage, func(in *domain.RecipientFacts) { in.Level.Thread = threadRow("loud", time.Time{}) }),
		"unknown platform":       with(domain.KindMessage, func(in *domain.RecipientFacts) { in.Devices[0].Platform = "linux" }),
		"empty platform":         with(domain.KindSecurity, func(in *domain.RecipientFacts) { in.Devices[1].Platform = "" }),
		"zero now":               with(domain.KindMessage, func(in *domain.RecipientFacts) { in.Now = time.Time{} }),
		"zero now for admin":     with(domain.KindAdmin, func(in *domain.RecipientFacts) { in.Now = time.Time{} }),
		"negative window":        with(domain.KindMessage, func(in *domain.RecipientFacts) { in.Policy.CoalesceWindow = -time.Second }),
		"negative delay":         with(domain.KindSecurity, func(in *domain.RecipientFacts) { in.Policy.MobileDelay = -time.Second }),
		"zero thread":            with(domain.KindMention, func(in *domain.RecipientFacts) { in.ThreadID = ids.ID{} }),
		"security with a thread": with(domain.KindSecurity, func(in *domain.RecipientFacts) { in.ThreadID = threadID }),
		"admin with a thread":    with(domain.KindAdmin, func(in *domain.RecipientFacts) { in.ThreadID = threadID }),
		"empty instance default": with(domain.KindRunDone, func(in *domain.RecipientFacts) { in.Level.InstanceDefault = "" }),
		"unknown instance default": with(domain.KindRunDone, func(in *domain.RecipientFacts) {
			in.Level.InstanceDefault = "loud"
		}),
		"dm outside a dm channel":  with(domain.KindDM, func(in *domain.RecipientFacts) { in.Level.DirectMessage = false }),
		"message in a dm channel":  with(domain.KindMessage, func(in *domain.RecipientFacts) { in.Level.DirectMessage = true }),
		"mention in a dm channel":  with(domain.KindMention, func(in *domain.RecipientFacts) { in.Level.DirectMessage = true }),
		"run_done in a dm channel": with(domain.KindRunDone, func(in *domain.RecipientFacts) { in.Level.DirectMessage = true }),
		"zero device id":           with(domain.KindMessage, func(in *domain.RecipientFacts) { in.Devices[1].ID = ids.ID{} }),
		"duplicate device id":      with(domain.KindMessage, func(in *domain.RecipientFacts) { in.Devices[1].ID = phoneID }),
		"zero registration id": with(domain.KindSecurity, func(in *domain.RecipientFacts) {
			in.Devices[0].Registrations = []ids.ID{phoneReg, {}}
		}),
		"zero thread in a focus": with(domain.KindMessage, func(in *domain.RecipientFacts) {
			in.Devices[1] = connected(desktop(), false, otherThreadID, ids.ID{})
		}),
		"rule weekdays": with(domain.KindMessage, func(in *domain.RecipientFacts) {
			in.QuietHours = []domain.QuietRule{{Weekdays: 0, Start: 0, End: 60, Location: time.UTC, Enabled: true}}
		}),
		"rule time": with(domain.KindAdmin, func(in *domain.RecipientFacts) {
			in.QuietHours = []domain.QuietRule{{Weekdays: 1, Start: 0, End: 1440, Location: time.UTC, Enabled: true}}
		}),
		"rule location": with(domain.KindInputNeeded, func(in *domain.RecipientFacts) {
			in.QuietHours = []domain.QuietRule{{Weekdays: 1, Start: 0, End: 60, Enabled: true}}
		}),
	}
}

// TestDecide_InvalidInput: a broken input is an internal error and never sends or collects
// (D-M3-17-7).
func TestDecide_InvalidInput(t *testing.T) {
	t.Parallel()
	for name, in := range invalidCases(t) {
		d, err := domain.Decide(in)
		assertInternal(t, name, err)
		if !reflect.DeepEqual(d, domain.Decision{}) || d.Outcome != domain.OutcomeNone {
			t.Errorf("%s: got %+v, want the zero Decision", name, d)
		}
		// Not even a denied or muted input hides a broken one: validation runs first.
		in.Allowed = false
		if _, err := domain.Decide(in); err == nil {
			t.Errorf("%s: no error once Allowed is false", name)
		}
	}
	// The error text names the field, never the value.
	in := facts(domain.KindMessage)
	in.Kind = "secret-kind-value"
	_, err := domain.Decide(in)
	if err == nil || strings.Contains(err.Error(), "secret-kind-value") {
		t.Errorf("error text %v carries the value", err)
	}
}

// TestKindHelpers holds HasThread, Urgent, Desktop and Mobile against 13 2, 4.4 and 4.5.
func TestKindHelpers(t *testing.T) {
	t.Parallel()
	for _, k := range domain.Kinds() {
		if !k.Valid() {
			t.Errorf("%s is not valid", k)
		}
		if got, want := k.HasThread(), slices.Contains(threadKinds, k); got != want {
			t.Errorf("%s.HasThread() = %v", k, got)
		}
		wantUrgent := k != domain.KindMessage && k != domain.KindAdmin
		if k.Urgent() != wantUrgent {
			t.Errorf("%s.Urgent() = %v", k, k.Urgent())
		}
	}
	if domain.Kind("x").Valid() || domain.Level("").Valid() || domain.Platform("linux").Valid() {
		t.Error("an unknown value is valid")
	}
	for _, p := range domain.Platforms() {
		if !p.Valid() {
			t.Errorf("%s is not valid", p)
		}
		desk := p == domain.PlatformMacOS || p == domain.PlatformWindows
		mob := p == domain.PlatformIOS || p == domain.PlatformAndroid
		if p.Desktop() != desk || p.Mobile() != mob {
			t.Errorf("%s: Desktop %v Mobile %v", p, p.Desktop(), p.Mobile())
		}
	}
	for _, l := range domain.Levels() {
		if !l.Valid() {
			t.Errorf("%s is not valid", l)
		}
	}
	for _, o := range []domain.Outcome{domain.OutcomeNone, domain.OutcomeCollect, domain.OutcomeSend} {
		if !o.Valid() {
			t.Errorf("outcome %s is not valid", o)
		}
	}
	if domain.OutcomeNone.String() != "none" || domain.OutcomeCollect.String() != "collect" || domain.OutcomeSend.String() != "send" {
		t.Error("outcome names differ from design note 3")
	}
	if (domain.Decision{}).Outcome != domain.OutcomeNone {
		t.Error("the zero Decision is not none")
	}
	// The list functions return copies.
	ks := domain.Kinds()
	ks[0] = "changed"
	if domain.Kinds()[0] != domain.KindMessage {
		t.Error("Kinds() shares its slice")
	}
}
