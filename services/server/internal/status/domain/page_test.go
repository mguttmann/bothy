// SPDX-License-Identifier: Apache-2.0

package domain_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/status/domain"
)

// TestChecksOfHealthSources (D-M7-09-2): the group keys of the standard components name the checks
// of their part of 16 12, every single key names itself, an unknown key none.
func TestChecksOfHealthSources(t *testing.T) {
	t.Parallel()
	for hs, want := range map[string][]string{
		domain.GroupAPI:            {domain.SourceAPIHTTP, domain.SourceSyncFanout},
		domain.GroupPush:           {domain.SourcePushAPNs, domain.SourcePushFCM},
		domain.GroupFiles:          {domain.SourceFilesStorage, domain.SourceFilesScan},
		domain.GroupSearch:         {domain.SourceSearchQuery},
		domain.SourceSyncFanout:    {domain.SourceSyncFanout},
		domain.SourceAgentPresence: {domain.SourceAgentPresence},
	} {
		if got, ok := domain.Checks(hs); !ok || !slices.Equal(got, want) {
			t.Errorf("Checks(%s) = %v %v, want %v", hs, got, ok, want)
		}
	}
	if _, ok := domain.Checks("db"); ok {
		t.Error("an unknown key has checks")
	}
	seen := map[string]bool{}
	for _, s := range domain.StandardComponents() {
		if _, ok := domain.Checks(s.HealthSource); !ok || seen[s.HealthSource] || s.Name == "" {
			t.Errorf("standard component %+v", s)
		}
		seen[s.HealthSource] = true
	}
	agent := domain.Component{HealthSource: domain.SourceAgentPresence}
	if agent.Automatic() || (domain.Component{}).Automatic() || !(domain.Component{HealthSource: domain.GroupAPI}).Automatic() {
		t.Error("Automatic: a presence component without agent or a manual one is automatic")
	}
}

// TestWorstAndValues: the overall state is the worst one; the closed value lists.
func TestWorstAndValues(t *testing.T) {
	t.Parallel()
	if got := domain.Worst(); got != domain.StateOperational {
		t.Errorf("Worst() = %s", got)
	}
	if got := domain.Worst(domain.StateMaintenance, domain.StateDegraded, domain.StateOperational); got != domain.StateDegraded {
		t.Errorf("Worst = %s", got)
	}
	if got := domain.Worst(domain.StatePartialOutage, domain.StateMajorOutage); got != domain.StateMajorOutage {
		t.Errorf("Worst = %s", got)
	}
	if domain.ComponentState("down").Valid() || !domain.StateMaintenance.Valid() {
		t.Error("ComponentState.Valid")
	}
	if domain.IncidentKind("outage").Valid() || !domain.KindMaintenance.Valid() || domain.Impact("huge").Valid() || !domain.ImpactCritical.Valid() {
		t.Error("kind or impact Valid")
	}
	if domain.IncidentState("open").Valid() || !domain.IncidentResolved.Valid() {
		t.Error("IncidentState.Valid")
	}
	for in, want := range map[string]string{"": "empty", "  ": "empty", "ok": "", "a\x00b": "invalid", "\xff": "invalid"} {
		if got := domain.TextProblem(in, true); got != want {
			t.Errorf("TextProblem(%q) = %q, want %q", in, got, want)
		}
	}
	if got := domain.NameProblem(strings.Repeat("ä", domain.MaxNameRunes)); got != "" {
		t.Errorf("a name of the longest length: %q", got)
	}
	if got := domain.NameProblem(strings.Repeat("a", domain.MaxNameRunes+1)); got != "too_long" {
		t.Errorf("a name above the longest length: %q", got)
	}
	if got := domain.TextProblem("", false); got != "" {
		t.Errorf("an optional empty text: %q", got)
	}
}

// TestPlace: the order of the components after a create or a move; the sort keys order as the
// positions.
func TestPlace(t *testing.T) {
	t.Parallel()
	a, b, c := ids.New(), ids.New(), ids.New()
	for _, tc := range []struct {
		order []ids.ID
		id    ids.ID
		after *ids.ID
		want  []ids.ID
		ok    bool
	}{
		{[]ids.ID{a, b}, c, nil, []ids.ID{a, b, c}, true},
		{[]ids.ID{a, b}, c, &a, []ids.ID{a, c, b}, true},
		{[]ids.ID{a, b, c}, a, &c, []ids.ID{b, c, a}, true},
		{[]ids.ID{a, b, c}, b, nil, []ids.ID{a, b, c}, true},
		{[]ids.ID{a, b}, a, &a, nil, false},
		{[]ids.ID{a}, b, &c, nil, false},
	} {
		got, ok := domain.Place(tc.order, tc.id, tc.after)
		if ok != tc.ok || !slices.Equal(got, tc.want) {
			t.Errorf("Place(%v, %v, %v) = %v %v, want %v %v", tc.order, tc.id, tc.after, got, ok, tc.want, tc.ok)
		}
	}
	if domain.SortKey(8) >= domain.SortKey(9) || domain.SortKey(9) >= domain.SortKey(10) {
		t.Error("sort keys do not order as their positions")
	}
	if domain.NoAnswer.String() != "no_answer" || domain.Outcome(9).String() == "" {
		t.Error("Outcome.String")
	}
}

// TestInMaintenance (M7-08): in maintenance mode an operational component shows maintenance and a
// worse state stays, so the mode never hides an outage.
func TestInMaintenance(t *testing.T) {
	t.Parallel()
	for in, want := range map[domain.ComponentState]domain.ComponentState{
		domain.StateOperational:   domain.StateMaintenance,
		domain.StateMaintenance:   domain.StateMaintenance,
		domain.StateDegraded:      domain.StateDegraded,
		domain.StatePartialOutage: domain.StatePartialOutage,
		domain.StateMajorOutage:   domain.StateMajorOutage,
	} {
		if got := domain.InMaintenance(in); got != want {
			t.Errorf("InMaintenance(%s) = %s, want %s", in, got, want)
		}
	}
}
