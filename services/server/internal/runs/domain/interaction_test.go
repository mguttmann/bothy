// SPDX-License-Identifier: Apache-2.0

package domain_test

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/runs/domain"
)

func TestEffectiveState(t *testing.T) {
	exp := cardNow.Add(time.Hour)
	cases := []struct {
		state string
		at    time.Time
		exp   time.Time
		want  string
	}{
		{domain.ComponentActive, exp.Add(-time.Millisecond), exp, domain.ComponentActive},
		{domain.ComponentActive, exp, exp, domain.ComponentExpired},
		{domain.ComponentActive, exp.Add(time.Hour), exp, domain.ComponentExpired},
		{domain.ComponentActive, exp.Add(time.Hour), time.Time{}, domain.ComponentActive},
		{domain.ComponentDisabled, exp.Add(time.Hour), exp, domain.ComponentDisabled},
		{domain.ComponentUsed, exp.Add(time.Hour), exp, domain.ComponentUsed},
		{domain.ComponentExpired, cardNow, exp, domain.ComponentExpired},
	}
	for i, tc := range cases {
		if got := domain.EffectiveState(tc.state, tc.exp, tc.at); got != tc.want {
			t.Errorf("case %d: EffectiveState(%s) = %s, want %s", i, tc.state, got, tc.want)
		}
	}
}

// TestComponentTransitions covers every cell of Design note 13 c: each stored state (with an
// active and a disabled one also after expires_at) against each requested state, for single-use
// components with and without a used sibling and for components outside the group.
func TestComponentTransitions(t *testing.T) {
	exp := cardNow.Add(time.Hour)
	before, after := cardNow, exp.Add(time.Minute)
	type from struct {
		name  string
		state string
		now   time.Time
	}
	froms := []from{
		{"active", domain.ComponentActive, before},
		{"active after expires_at", domain.ComponentActive, after},
		{"used", domain.ComponentUsed, before},
		{"expired", domain.ComponentExpired, after},
		{"disabled", domain.ComponentDisabled, before},
		{"disabled after expires_at", domain.ComponentDisabled, after},
	}
	tos := []string{"", domain.ComponentActive, domain.ComponentUsed, domain.ComponentExpired, domain.ComponentDisabled}
	type group struct {
		name                 string
		singleUse, groupUsed bool
	}
	groups := []group{{"single use", true, false}, {"single use with used sibling", true, true}, {"not single use", false, false}, {"not single use, group used", false, true}}

	// want returns "noop", "ok" or "409" for one cell.
	want := func(f from, to string, g group) string {
		eff := f.state
		if f.state == domain.ComponentActive && !f.now.Before(exp) {
			eff = domain.ComponentExpired
		}
		switch {
		case to == "" || to == eff:
			return "noop"
		case eff == domain.ComponentActive && to == domain.ComponentDisabled:
			return "ok"
		case eff == domain.ComponentDisabled && to == domain.ComponentActive && f.now.Before(exp) && (!g.singleUse || !g.groupUsed):
			return "ok"
		}
		return "409"
	}
	cells := 0
	for _, f := range froms {
		for _, to := range tos {
			for _, g := range groups {
				cells++
				changed, err := domain.StateTransition(f.state, exp, f.now, to, g.singleUse, g.groupUsed)
				got := "noop"
				switch {
				case err != nil:
					if e, ok := errs.As(err); !ok || e.Code != errs.CodeInvalidStateTransition {
						t.Errorf("%s -> %q (%s): error %v, want invalid_state_transition", f.name, to, g.name, err)
					}
					got = "409"
				case changed:
					got = "ok"
				}
				if w := want(f, to, g); got != w {
					t.Errorf("%s -> %q (%s) = %s, want %s", f.name, to, g.name, got, w)
				}
			}
		}
	}
	if cells != len(froms)*len(tos)*len(groups) {
		t.Fatalf("covered %d cells", cells)
	}

	// The cells the design note names explicitly.
	if _, err := domain.StateTransition(domain.ComponentDisabled, exp, before, domain.ComponentActive, true, true); err == nil {
		t.Errorf("disabled -> active with a used single-use sibling was allowed")
	}
	if _, err := domain.StateTransition(domain.ComponentUsed, exp, before, domain.ComponentDisabled, true, false); err == nil {
		t.Errorf("used -> disabled was allowed")
	}
	if _, err := domain.StateTransition(domain.ComponentActive, exp, after, domain.ComponentActive, true, false); err == nil {
		t.Errorf("expired-by-time active -> active was allowed")
	}
	if changed, err := domain.StateTransition(domain.ComponentActive, exp, after, domain.ComponentExpired, true, false); err != nil || changed {
		t.Errorf("expired-by-time active -> expired = %v, %v; want no change", changed, err)
	}
	if _, err := domain.StateTransition(domain.ComponentActive, exp, after, domain.ComponentDisabled, true, false); err == nil {
		t.Errorf("expired-by-time active -> disabled was allowed")
	}
	if changed, err := domain.StateTransition(domain.ComponentDisabled, time.Time{}, after, domain.ComponentActive, false, false); err != nil || !changed {
		t.Errorf("disabled -> active without expires_at = %v, %v; want allowed", changed, err)
	}
	if _, err := domain.StateTransition("bogus", exp, before, domain.ComponentDisabled, false, false); err == nil {
		t.Errorf("an unknown stored state was accepted")
	}
	if _, err := domain.StateTransition(domain.ComponentActive, exp, before, "paused", false, false); err == nil {
		t.Errorf("an unknown target state was accepted")
	}
}

var (
	cardAgent  = ids.ID{0x21, 1}
	otherAgent = ids.ID{0x22, 1}
	compRun    = ids.ID{0x23, 1}
)

func TestInteractionRouting(t *testing.T) {
	cases := []struct {
		name string
		f    domain.RouteFacts
		want domain.Route
	}{
		{"component run", domain.RouteFacts{CardAgentID: cardAgent, ComponentRunID: compRun, ComponentRunActive: true, ActiveRunAgentID: cardAgent}, domain.Route{Kind: domain.RouteComponentRun}},
		{"component run ended", domain.RouteFacts{CardAgentID: cardAgent, ComponentRunID: compRun}, domain.Route{Kind: domain.RouteRefused, Reason: "component_inactive"}},
		{"no run and no active run", domain.RouteFacts{CardAgentID: cardAgent}, domain.Route{Kind: domain.RouteOpenRun}},
		{"run-less component with the agent's active run", domain.RouteFacts{CardAgentID: cardAgent, ActiveRunAgentID: cardAgent}, domain.Route{Kind: domain.RouteRefused, Reason: "run_active"}},
		{"active run of another agent", domain.RouteFacts{CardAgentID: cardAgent, ActiveRunAgentID: otherAgent}, domain.Route{Kind: domain.RouteRefused, Reason: "run_active_other_agent"}},
	}
	for _, tc := range cases {
		if got := domain.RouteInteraction(tc.f); got != tc.want {
			t.Errorf("%s: RouteInteraction = %+v, want %+v", tc.name, got, tc.want)
		}
	}
}

func TestAuditDetails(t *testing.T) {
	base := domain.AuditFacts{
		InteractionID: ids.ID{0x31, 1}, MessageID: ids.ID{0x32, 1}, RunID: compRun,
		ComponentType: domain.ComponentButton, RequiredCapability: domain.CapabilityApproveActions, RestrictedTo: domain.RestrictRunStarter,
	}
	for _, v := range []string{"once", "session", "always", "deny"} {
		f := base
		f.Definition = json.RawMessage(`{"label":"Choice ` + v + `","style":"primary","value":"` + v + `"}`)
		got := domain.InteractionAuditDetails(f)
		want := domain.AuditDetails{InteractionID: base.InteractionID, MessageID: base.MessageID, RunID: compRun, ComponentType: "button", RequiredCapability: "approve_actions", RestrictedTo: "run_starter", Choice: v}
		if got != want {
			t.Errorf("value %s: details = %+v, want %+v", v, got, want)
		}
	}
	for _, tc := range []struct{ typ, def string }{
		{"button", `{"label":"Allow","style":"secondary","value":"Once"}`},
		{"button", `{"label":"once","style":"secondary","value":"yes"}`},
		{"select", `{"options":[{"label":"once","value":"once"}],"min_values":1,"max_values":1}`},
		{"form", `{"title":"once","fields":[{"name":"once","label":"once","type":"text","required":false}]}`},
		{"button", `not json`},
	} {
		f := base
		f.ComponentType, f.Definition, f.RunID = tc.typ, json.RawMessage(tc.def), ids.ID{}
		got := domain.InteractionAuditDetails(f)
		if got.Choice != "" || !got.RunID.IsZero() || got.ComponentType != tc.typ {
			t.Errorf("%s %s: details = %+v, want no choice and no run", tc.typ, tc.def, got)
		}
	}
	// The detail set has no member for custom_id, labels or values.
	names := map[string]bool{}
	rt := reflect.TypeOf(domain.AuditDetails{})
	for i := range rt.NumField() {
		names[rt.Field(i).Name] = true
	}
	for _, n := range []string{"CustomID", "Label", "Value", "Values"} {
		if names[n] {
			t.Errorf("AuditDetails has a member %s", n)
		}
	}
}

func TestInteractionCreatedPayload(t *testing.T) {
	text := "Refund approved"
	v := domain.InteractionView{
		ID: ids.ID{0x41, 1}, ComponentID: ids.ID{0x42, 1}, MessageID: ids.ID{0x43, 1}, PrincipalID: ids.ID{0x44, 1}, ClientInteractionID: ids.ID{0x45, 1},
		CustomID: "approve", ThreadID: ids.ID{0x46, 1}, RunID: compRun,
		Values: json.RawMessage(`{"note":"Sam at example.com"}`), State: "received",
		ResponseText: &text, HasResponse: true,
		CreatedAt: time.Date(2026, 10, 21, 11, 30, 0, 456789000, time.FixedZone("CEST", 2*60*60)),
	}
	b, err := domain.InteractionCreatedPayload(v)
	if err != nil {
		t.Fatalf("InteractionCreatedPayload: %v", err)
	}
	want := `{"interaction":{"id":"` + v.ID.String() + `","component_id":"` + v.ComponentID.String() + `","custom_id":"approve","message_id":"` + v.MessageID.String() +
		`","thread_id":"` + v.ThreadID.String() + `","run_id":"` + compRun.String() + `","principal_id":"` + v.PrincipalID.String() +
		`","client_interaction_id":"` + v.ClientInteractionID.String() + `","values":{"note":"Sam at example.com"},"state":"received","created_at":"2026-10-21T09:30:00.456Z"}}`
	if string(b) != want {
		t.Fatalf("payload =\n%s\nwant\n%s", b, want)
	}
	if strings.Contains(string(b), "response") || strings.Contains(string(b), "null") {
		t.Fatalf("payload carries a response or null: %s", b)
	}

	t.Run("optional members absent and values default", func(t *testing.T) {
		w := v
		w.ThreadID, w.RunID, w.CustomID, w.Values = ids.ID{}, ids.ID{}, "", nil
		b, err := domain.InteractionCreatedPayload(w)
		if err != nil {
			t.Fatalf("InteractionCreatedPayload: %v", err)
		}
		var doc map[string]map[string]json.RawMessage
		if err := json.Unmarshal(b, &doc); err != nil {
			t.Fatal(err)
		}
		got := doc["interaction"]
		for _, m := range []string{"thread_id", "run_id", "custom_id", "response"} {
			if _, ok := got[m]; ok {
				t.Errorf("member %s present", m)
			}
		}
		if string(got["values"]) != `{}` {
			t.Errorf("values = %s, want {}", got["values"])
		}
		if len(got) != 8 {
			t.Errorf("members = %d, want only the required ones", len(got))
		}
	})

	t.Run("broken views are refused", func(t *testing.T) {
		broken := map[string]func(*domain.InteractionView){
			"zero id":                    func(w *domain.InteractionView) { w.ID = ids.ID{} },
			"zero component_id":          func(w *domain.InteractionView) { w.ComponentID = ids.ID{} },
			"zero message_id":            func(w *domain.InteractionView) { w.MessageID = ids.ID{} },
			"zero principal_id":          func(w *domain.InteractionView) { w.PrincipalID = ids.ID{} },
			"zero client_interaction_id": func(w *domain.InteractionView) { w.ClientInteractionID = ids.ID{} },
			"unknown state":              func(w *domain.InteractionView) { w.State = "lost" },
			"zero created_at":            func(w *domain.InteractionView) { w.CreatedAt = time.Time{} },
			"values not an object":       func(w *domain.InteractionView) { w.Values = json.RawMessage(`[1]`) },
			"values with null":           func(w *domain.InteractionView) { w.Values = json.RawMessage(`{"a":[null]}`) },
		}
		for name, breakIt := range broken {
			w := v
			breakIt(&w)
			if _, err := domain.InteractionCreatedPayload(w); errs.KindOf(err) != errs.KindInternal {
				t.Errorf("%s: error = %v, want internal", name, err)
			}
		}
	})
}
