// SPDX-License-Identifier: Apache-2.0

package domain_test

import (
	"testing"

	"github.com/OWNER/bothy/services/server/internal/authz/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
)

// TestAppEventScope_AllCatalogueEvents classifies every event type of the catalogue for the
// manifest rules (M2-24 design note 4): every subscribable type is durable and either has a
// receive action of the scope table or is a personal event apps receive; every other personal
// type and the receive actions apps may not run are refused.
func TestAppEventScope_AllCatalogueEvents(t *testing.T) {
	t.Parallel()
	want := map[string]struct {
		scope string
		ok    bool
	}{
		"message.created":     {"messages:read", true},
		"instance.updated":    {"", true},
		"user.updated":        {"users:read", true},
		"command.invoked":     {"events:subscribe", true},
		"interaction.created": {"events:subscribe", true},
		"access.changed":      {"", false},
		"draft.updated":       {"", false},
		"agent.invocation":    {"", false},
		"grant.changed":       {"", false},
		"host.updated":        {"", false},
	}
	subscribable := 0
	for _, et := range authorize.AllEventTypes() {
		spec, known := domain.LookupEvent(string(et))
		if !known {
			t.Fatalf("%s is not in the generated catalogue", et)
		}
		scope, ok := domain.AppEventScope(string(et))
		if w, listed := want[string(et)]; listed && (scope != w.scope || ok != w.ok) {
			t.Errorf("%s: %q %v, want %q %v", et, scope, ok, w.scope, w.ok)
		}
		if !ok {
			continue
		}
		subscribable++
		switch {
		case !spec.Durable:
			t.Errorf("%s is ephemeral but subscribable", et)
		case spec.AudienceSelf && spec.Receivers != "agent_or_app":
			t.Errorf("%s is a personal event of another principal but subscribable", et)
		case !spec.AudienceSelf:
			if s, listed := domain.AppScopeOf(spec.MentionedActions[0]); !listed || s != scope {
				t.Errorf("%s: scope %q does not come from the scope table", et, scope)
			}
		}
	}
	if subscribable == 0 {
		t.Fatal("no event type is subscribable")
	}
	if _, ok := domain.AppEventScope("no.such.event"); ok {
		t.Error("an unknown type is subscribable")
	}
}

// TestAppScope_TokenAndInstallation covers M2-24 design note 10 c: the scope must be held by the
// installation and, with a token, by the token; without a token the installation decides alone.
func TestAppScope_TokenAndInstallation(t *testing.T) {
	t.Parallel()
	w := []string{domain.ScopeMessagesWrite}
	cases := []struct {
		name           string
		granted, token []string
		hasToken       bool
		want           domain.Reason
	}{
		{"only in the installation", w, nil, true, domain.ReasonScopeMissing},
		{"only in the token", nil, w, true, domain.ReasonScopeMissing},
		{"in both", w, w, true, ""},
		{"no token, in the installation", w, nil, false, ""},
		{"no token, not in the installation", nil, nil, false, domain.ReasonScopeMissing},
	}
	for _, c := range cases {
		if got := domain.CheckAppScope("message.create", c.granted, c.token, c.hasToken); got != c.want {
			t.Errorf("%s: %q, want %q", c.name, got, c.want)
		}
	}
	if got := domain.CheckAppScope("self.read", nil, nil, true); got != "" {
		t.Errorf("an action without scope: %q", got)
	}
	if got := domain.CheckAppScope("grant.create", w, w, true); got != domain.ReasonSubjectKindNotAllowed {
		t.Errorf("an action closed to apps: %q", got)
	}
	if n := domain.AppScopeNames(); len(n) != 9 || n[0] != domain.ScopeCommandsWrite {
		t.Errorf("scope names %v", n)
	}
}
