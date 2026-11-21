// SPDX-License-Identifier: Apache-2.0

package domain_test

import (
	"slices"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/runs/domain"
)

// TestFeedSourceOf_EveryInputEvent: every invocation-bearing event type, the inputs an agent
// receives for a run, records a source of run_feeders at its delivery, and every source is one of
// FeedSources; a new input event without a source fails here (FEAT-TEAM-CHANNEL-SERVER).
func TestFeedSourceOf_EveryInputEvent(t *testing.T) {
	seen := map[string]bool{}
	for _, typ := range domain.InvocationBearingTypes() {
		src := domain.FeedSourceOf(typ)
		if !slices.Contains(domain.FeedSources(), src) {
			t.Errorf("event %s records source %q, which is no source of run_feeders", typ, src)
		}
		if seen[src] {
			t.Errorf("source %s is recorded by two event types", src)
		}
		seen[src] = true
		if !domain.InvocationBearing(typ) {
			t.Errorf("%s is listed but not invocation-bearing", typ)
		}
	}
	if got := domain.FeedSourceOf("message.created"); got != "" {
		t.Errorf("an event without an invocation records %q", got)
	}
	if !seen[domain.FeedTrigger] || seen[domain.FeedContext] {
		t.Errorf("the deliveries record %v; the context is recorded by the run context only", seen)
	}
}

// TestContextFeeders_EveryAuthorKind: every element of a run context is on the record (red team
// rounds 5 and 6 of FEAT-TEAM-CHANNEL-SERVER): a team input with its authority, a person's message
// (any origin, a system line a person caused included) with its author, an agent's answer (the
// run's own agent included) with the record of the run it answers, and every other author
// (another agent or the own agent outside a run of the thread, an app, a server principal, an
// author of unknown kind, an answer of a run without its trigger row) as an input that speaks for
// no person, which the owner check refuses.
func TestContextFeeders_EveryAuthorKind(t *testing.T) {
	agent, other, person, app, authority, sam := ids.New(), ids.New(), ids.New(), ids.New(), ids.New(), ids.New()
	none := []ids.ID{{}}
	cases := []struct {
		name string
		e    domain.ContextElement
		want []ids.ID
	}{
		{"person", domain.ContextElement{AuthorID: person, AuthorKind: "user"}, []ids.ID{person}},
		{"another agent outside a run", domain.ContextElement{AuthorID: other, AuthorKind: "agent"}, none},
		{"own agent outside a run", domain.ContextElement{AuthorID: agent, AuthorKind: "agent"}, none},
		{"another agent's team input", domain.ContextElement{AuthorID: other, AuthorKind: "agent", TeamInput: true, Authority: authority}, []ids.ID{authority}},
		{"a person's team input", domain.ContextElement{AuthorID: person, AuthorKind: "user", TeamInput: true, Authority: authority}, []ids.ID{authority}},
		{"own answer of an owner run", domain.ContextElement{AuthorID: agent, AuthorKind: "agent", Answer: true,
			AnswerRecord: []ids.ID{person}, AnswerTriggered: true}, []ids.ID{person}},
		{"own answer of a run Sam fed", domain.ContextElement{AuthorID: agent, AuthorKind: "agent", Answer: true,
			AnswerRecord: []ids.ID{person, sam, {}}, AnswerTriggered: true}, []ids.ID{person, sam, {}}},
		{"answer of a run without its trigger row", domain.ContextElement{AuthorID: other, AuthorKind: "agent", Answer: true,
			AnswerRecord: []ids.ID{person}}, none},
		{"app", domain.ContextElement{AuthorID: app, AuthorKind: "app"}, none},
		{"unknown kind", domain.ContextElement{AuthorID: app, AuthorKind: ""}, none},
		{"no author", domain.ContextElement{}, none},
	}
	for _, c := range cases {
		if got := domain.ContextFeeders(c.e); !slices.Equal(got, c.want) {
			t.Errorf("%s: got %v, want %v", c.name, got, c.want)
		}
	}
}
