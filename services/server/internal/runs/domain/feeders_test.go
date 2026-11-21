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

// TestContextFeeder_EveryAuthorKind: every element of a run context that the run's own agent did
// not write is on the record (red team round 5 of FEAT-TEAM-CHANNEL-SERVER): a team input with its
// authority, a person's message (any origin, a system line a person caused included) with its
// author, and every other author (another agent, an app, a server principal, an author of unknown
// kind) as an input that speaks for no person, which the owner check refuses. Only the own
// agent's messages are left out (ruling D-TEAM-11).
func TestContextFeeder_EveryAuthorKind(t *testing.T) {
	agent, other, person, app, authority := ids.New(), ids.New(), ids.New(), ids.New(), ids.New()
	cases := []struct {
		name   string
		e      domain.ContextElement
		want   ids.ID
		record bool
	}{
		{"person", domain.ContextElement{AuthorID: person, AuthorKind: "user"}, person, true},
		{"another agent", domain.ContextElement{AuthorID: other, AuthorKind: "agent"}, ids.ID{}, true},
		{"another agent's team input", domain.ContextElement{AuthorID: other, AuthorKind: "agent", TeamInput: true, Authority: authority}, authority, true},
		{"a person's team input", domain.ContextElement{AuthorID: person, AuthorKind: "user", TeamInput: true, Authority: authority}, authority, true},
		{"app", domain.ContextElement{AuthorID: app, AuthorKind: "app"}, ids.ID{}, true},
		{"unknown kind", domain.ContextElement{AuthorID: app, AuthorKind: ""}, ids.ID{}, true},
		{"no author", domain.ContextElement{}, ids.ID{}, true},
		{"own agent", domain.ContextElement{AuthorID: agent, AuthorKind: "agent"}, ids.ID{}, false},
	}
	for _, c := range cases {
		got, record := domain.ContextFeeder(c.e, agent)
		if got != c.want || record != c.record {
			t.Errorf("%s: got %v %v, want %v %v", c.name, got, record, c.want, c.record)
		}
	}
}
