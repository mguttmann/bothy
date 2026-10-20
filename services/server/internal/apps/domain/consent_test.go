// SPDX-License-Identifier: Apache-2.0

package domain_test

import (
	"slices"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/apps/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
)

// consentManifest is a manifest with one required and three optional scopes and two events.
func consentManifest() domain.Manifest {
	return domain.Manifest{
		Slug:   "tracker",
		Scopes: domain.ManifestScopes{Required: []string{"messages:write"}, Optional: []string{"messages:read", "events:subscribe", "users:read"}},
		Events: []string{"message.created", "instance.updated"},
	}
}

// wantFieldErr checks a validation_failed error with code at pointer.
func wantFieldErr(t *testing.T, err error, pointer, code string) {
	t.Helper()
	e, ok := errs.As(err)
	if !ok || e.Code != errs.CodeValidationFailed {
		t.Fatalf("error %v, want validation_failed", err)
	}
	for _, f := range e.Fields {
		if f.Pointer == pointer && f.Code == code {
			return
		}
	}
	t.Fatalf("fields %+v, want %s at %s", e.Fields, code, pointer)
}

// TestConsent_Install covers M2-24 design note 6 c.
func TestConsent_Install(t *testing.T) {
	t.Parallel()
	m := consentManifest()
	g, err := domain.ConsentInstall(m, domain.ConsentRequest{Scopes: []string{"messages:write"}}, eventScope)
	if err != nil || !slices.Equal(g.Scopes, []string{"messages:write"}) || g.Events == nil || len(g.Events) != 0 {
		t.Fatalf("plain install %+v %v", g, err)
	}
	g, err = domain.ConsentInstall(m, domain.ConsentRequest{
		Scopes: []string{"messages:write", "messages:read", "events:subscribe"}, Subscriptions: []string{"message.created", "instance.updated"}, SubscriptionsSet: true,
	}, eventScope)
	if err != nil || !slices.Equal(g.Scopes, []string{"events:subscribe", "messages:read", "messages:write"}) ||
		!slices.Equal(g.Events, []string{"instance.updated", "message.created"}) {
		t.Fatalf("sorted lists %+v %v", g, err)
	}
	_, err = domain.ConsentInstall(m, domain.ConsentRequest{Scopes: []string{"messages:read"}}, eventScope)
	wantFieldErr(t, err, "/granted_scopes", "required")
	_, err = domain.ConsentInstall(m, domain.ConsentRequest{Scopes: []string{"messages:write", "files:read"}}, eventScope)
	wantFieldErr(t, err, "/granted_scopes/1", "invalid")
	_, err = domain.ConsentInstall(m, domain.ConsentRequest{Scopes: []string{"messages:write", "messages:write"}}, eventScope)
	wantFieldErr(t, err, "/granted_scopes/1", "invalid")
	_, err = domain.ConsentInstall(m, domain.ConsentRequest{Scopes: []string{"messages:write", "events:subscribe"},
		Subscriptions: []string{"message.created"}, SubscriptionsSet: true}, eventScope)
	wantFieldErr(t, err, "/event_subscriptions/0", "invalid")
	_, err = domain.ConsentInstall(m, domain.ConsentRequest{Scopes: []string{"messages:write", "messages:read"},
		Subscriptions: []string{"message.created"}, SubscriptionsSet: true}, eventScope)
	wantFieldErr(t, err, "/event_subscriptions/0", "invalid")
	_, err = domain.ConsentInstall(m, domain.ConsentRequest{Scopes: []string{"messages:write", "events:subscribe"},
		Subscriptions: []string{"thread.created"}, SubscriptionsSet: true}, eventScope)
	wantFieldErr(t, err, "/event_subscriptions/0", "invalid")
}

// TestConsent_Upgrade covers M2-24 design note 7 e: no new scope through an upgrade, dropped
// scopes and events act at once, absent subscriptions keep what is still allowed.
func TestConsent_Upgrade(t *testing.T) {
	t.Parallel()
	m := consentManifest()
	current := domain.Grants{Scopes: []string{"events:subscribe", "messages:read", "messages:write"}, Events: []string{"instance.updated", "message.created"}}
	g, err := domain.ConsentUpgrade(m, current, domain.ConsentRequest{Scopes: current.Scopes}, eventScope)
	if err != nil || !g.Equal(current) {
		t.Fatalf("same consent %+v %v", g, err)
	}
	g, err = domain.ConsentUpgrade(m, current, domain.ConsentRequest{Scopes: []string{"messages:write", "events:subscribe"}}, eventScope)
	if err != nil || !slices.Equal(g.Events, []string{"instance.updated"}) {
		t.Fatalf("dropping messages:read keeps message.created: %+v %v", g, err)
	}
	_, err = domain.ConsentUpgrade(m, current, domain.ConsentRequest{Scopes: []string{"messages:write", "users:read"}}, eventScope)
	wantFieldErr(t, err, "/granted_scopes/1", "invalid")
	narrower := m
	narrower.Scopes.Optional = []string{"events:subscribe"}
	narrower.Events = []string{"instance.updated"}
	_, err = domain.ConsentUpgrade(narrower, current, domain.ConsentRequest{Scopes: []string{"messages:write", "messages:read"}}, eventScope)
	wantFieldErr(t, err, "/granted_scopes/1", "invalid")
	g, err = domain.ConsentUpgrade(narrower, current, domain.ConsentRequest{Scopes: []string{"messages:write", "events:subscribe"}}, eventScope)
	if err != nil || !slices.Equal(g.Events, []string{"instance.updated"}) {
		t.Fatalf("dropped event stays: %+v %v", g, err)
	}
	small := domain.Grants{Scopes: current.Scopes, Events: []string{"instance.updated"}}
	_, err = domain.ConsentUpgrade(m, small, domain.ConsentRequest{Scopes: current.Scopes, Subscriptions: []string{"message.created"}, SubscriptionsSet: true}, eventScope)
	wantFieldErr(t, err, "/event_subscriptions/0", "invalid")
}

// TestConsent_SetScopes covers M2-24 design note 9 a.
func TestConsent_SetScopes(t *testing.T) {
	t.Parallel()
	m := consentManifest()
	current := domain.Grants{Scopes: []string{"messages:write"}, Events: []string{}}
	g, err := domain.ConsentSetScopes(m, current, domain.ConsentRequest{Scopes: []string{"messages:write", "messages:read", "events:subscribe"},
		Subscriptions: []string{"message.created"}, SubscriptionsSet: true}, eventScope)
	if err != nil || !slices.Equal(g.Events, []string{"message.created"}) || len(g.Scopes) != 3 {
		t.Fatalf("new consent %+v %v", g, err)
	}
	g, err = domain.ConsentSetScopes(m, g, domain.ConsentRequest{Scopes: []string{"events:subscribe"}}, eventScope)
	if err != nil || len(g.Events) != 0 || !slices.Equal(g.Scopes, []string{"events:subscribe"}) {
		t.Fatalf("narrowing drops required scope and the subscription without its scope: %+v %v", g, err)
	}
	_, err = domain.ConsentSetScopes(m, current, domain.ConsentRequest{Scopes: []string{"files:write"}}, eventScope)
	wantFieldErr(t, err, "/granted_scopes/0", "invalid")
	_, err = domain.ConsentSetScopes(m, current, domain.ConsentRequest{Scopes: []string{"messages:write"},
		Subscriptions: []string{"instance.updated"}, SubscriptionsSet: true}, eventScope)
	wantFieldErr(t, err, "/event_subscriptions/0", "invalid")
}

// TestDiffManifests: scopes, events, commands and delivery hosts added and removed.
func TestDiffManifests(t *testing.T) {
	t.Parallel()
	old := domain.Manifest{
		Scopes:   domain.ManifestScopes{Required: []string{"messages:write"}, Optional: []string{"users:read"}},
		Events:   []string{"message.created"},
		Commands: []domain.Command{{Name: "Issue"}, {Name: "close-ticket"}},
		Delivery: &domain.Delivery{Mode: "webhook", EventsURL: "https://hooks.example.com/e"},
	}
	next := domain.Manifest{
		Scopes:   domain.ManifestScopes{Required: []string{"messages:write", "messages:read"}},
		Events:   []string{"instance.updated"},
		Commands: []domain.Command{{Name: "issue"}, {Name: "assign"}},
		Delivery: &domain.Delivery{Mode: "webhook", EventsURL: "https://Hooks.Example.com/e", InteractionsURL: "https://bridge.example.com/i"},
	}
	d := domain.DiffManifests(old, next)
	for name, got := range map[string][2][]string{
		"scopes":   {d.ScopesAdded, d.ScopesRemoved},
		"events":   {d.EventsAdded, d.EventsRemoved},
		"commands": {d.CommandsAdded, d.CommandsRemoved},
		"hosts":    {d.DeliveryHostsAdded, d.DeliveryHostsRemoved},
	} {
		want := map[string][2][]string{
			"scopes":   {{"messages:read"}, {"users:read"}},
			"events":   {{"instance.updated"}, {"message.created"}},
			"commands": {{"assign"}, {"close-ticket"}},
			"hosts":    {{"bridge.example.com"}, nil},
		}[name]
		if !slices.Equal(got[0], want[0]) || !slices.Equal(got[1], want[1]) {
			t.Errorf("%s: added %v removed %v, want %v %v", name, got[0], got[1], want[0], want[1])
		}
	}
}

// TestInstallationTransitions covers every pair of state and transition (M2-24 design note 9).
func TestInstallationTransitions(t *testing.T) {
	t.Parallel()
	cases := []struct {
		from   domain.State
		tr     domain.Transition
		to     domain.State
		change bool
		code   errs.Code
	}{
		{domain.StateActive, domain.TransitionSuspend, domain.StateSuspended, true, ""},
		{domain.StateSuspended, domain.TransitionSuspend, domain.StateSuspended, false, ""},
		{domain.StateUninstalled, domain.TransitionSuspend, domain.StateUninstalled, false, errs.CodeInvalidStateTransition},
		{domain.StateSuspended, domain.TransitionResume, domain.StateActive, true, ""},
		{domain.StateActive, domain.TransitionResume, domain.StateActive, false, ""},
		{domain.StateUninstalled, domain.TransitionResume, domain.StateUninstalled, false, errs.CodeInvalidStateTransition},
		{domain.StateActive, domain.TransitionUninstall, domain.StateUninstalled, true, ""},
		{domain.StateSuspended, domain.TransitionUninstall, domain.StateUninstalled, true, ""},
		{domain.StateUninstalled, domain.TransitionUninstall, domain.StateUninstalled, false, ""},
		{"paused", domain.TransitionSuspend, "paused", false, errs.CodeInternalError},
		{domain.StateActive, "pause", domain.StateActive, false, errs.CodeInternalError},
	}
	for _, c := range cases {
		to, change, err := domain.Next(c.from, c.tr)
		if to != c.to || change != c.change || codeOf(err) != c.code {
			t.Errorf("%s %s: %s %v %v, want %s %v %s", c.from, c.tr, to, change, err, c.to, c.change, c.code)
		}
	}
}

// TestSemVerPrecedence covers the parse and the precedence of SemVer 2.0.0 (rule 11 and its
// example chain).
func TestSemVerPrecedence(t *testing.T) {
	t.Parallel()
	chain := []string{"1.0.0-alpha", "1.0.0-alpha.1", "1.0.0-alpha.beta", "1.0.0-beta", "1.0.0-beta.2", "1.0.0-beta.11",
		"1.0.0-rc.1", "1.0.0", "1.0.1", "1.1.0", "2.0.0"}
	for i := 0; i+1 < len(chain); i++ {
		a, okA := domain.ParseSemVer(chain[i])
		b, okB := domain.ParseSemVer(chain[i+1])
		again, _ := domain.ParseSemVer(chain[i])
		if !okA || !okB || a.Compare(b) != -1 || b.Compare(a) != 1 || a.Compare(again) != 0 {
			t.Errorf("%s < %s does not hold", chain[i], chain[i+1])
		}
	}
	x, _ := domain.ParseSemVer("1.0.0+build.1")
	y, _ := domain.ParseSemVer("1.0.0+build.2")
	if x.Compare(y) != 0 || x.Build != "build.1" {
		t.Error("build metadata counts")
	}
	for _, bad := range []string{"", "1", "1.0", "01.0.0", "1.0.0-", "1.0.0-01", "1.0.0+", "1.0.0-a..b", "v1.0.0", "1.0.0-a_b", "1.0.x"} {
		if _, ok := domain.ParseSemVer(bad); ok {
			t.Errorf("%q parses", bad)
		}
	}
}
