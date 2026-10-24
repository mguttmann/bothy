// SPDX-License-Identifier: Apache-2.0

package domain_test

import (
	"testing"

	"github.com/OWNER/bothy/services/server/internal/authz/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// TestAppOwnerReachable (brief M2-25 design note 3 b): the guard holds only for an active app with
// an active installation and the scope of the feature, and, on a resolved channel, write there
// outside dm channels; without a channel the level does not count.
func TestAppOwnerReachable(t *testing.T) {
	t.Parallel()
	scopes := []string{domain.ScopeCommandsWrite, domain.ScopeComponentsWrite}
	base := domain.AppOwnerFacts{ID: ids.New(), Active: true, InstallationActive: true, GrantedScopes: scopes, OnChannel: true, Level: domain.LevelWrite}
	cases := []struct {
		name  string
		mod   func(*domain.AppOwnerFacts)
		scope string
		want  bool
	}{
		{"active with scope and write", func(*domain.AppOwnerFacts) {}, domain.ScopeCommandsWrite, true},
		{"components scope", func(*domain.AppOwnerFacts) {}, domain.ScopeComponentsWrite, true},
		{"suspended", func(f *domain.AppOwnerFacts) { f.InstallationActive = false }, domain.ScopeCommandsWrite, false},
		{"uninstalled", func(f *domain.AppOwnerFacts) { f.InstallationActive, f.Active = false, false }, domain.ScopeCommandsWrite, false},
		{"principal disabled", func(f *domain.AppOwnerFacts) { f.Active = false }, domain.ScopeCommandsWrite, false},
		{"scope missing", func(f *domain.AppOwnerFacts) { f.GrantedScopes = []string{domain.ScopeComponentsWrite} }, domain.ScopeCommandsWrite, false},
		{"only read", func(f *domain.AppOwnerFacts) { f.Level = domain.LevelRead }, domain.ScopeCommandsWrite, false},
		{"no level", func(f *domain.AppOwnerFacts) { f.Level = domain.LevelNone }, domain.ScopeCommandsWrite, false},
		{"dm channel", func(f *domain.AppOwnerFacts) { f.DM = true }, domain.ScopeCommandsWrite, false},
		{"no channel, no level", func(f *domain.AppOwnerFacts) { f.OnChannel, f.Level = false, domain.LevelNone }, domain.ScopeCommandsWrite, true},
		{"no channel, suspended", func(f *domain.AppOwnerFacts) { f.OnChannel, f.InstallationActive = false, false }, domain.ScopeCommandsWrite, false},
	}
	for _, c := range cases {
		f := base
		f.GrantedScopes = append([]string(nil), base.GrantedScopes...)
		c.mod(&f)
		if got := domain.AppOwnerReachable(f, c.scope); got != c.want {
			t.Errorf("%s: %v, want %v", c.name, got, c.want)
		}
	}
}

// TestAppReceive (brief M2-25 design note 4): the audience app with an active installation and
// events:subscribe receives; another principal's event is not_self, no installation
// app_not_installed, no scope scope_missing.
func TestAppReceive(t *testing.T) {
	t.Parallel()
	app, other := ids.New(), ids.New()
	sub := []string{domain.ScopeEventsSubscribe}
	cases := []struct {
		name     string
		active   bool
		scopes   []string
		audience ids.ID
		want     domain.Decision
	}{
		{"audience with scope", true, sub, app, domain.Allow},
		{"other audience", true, sub, other, domain.Decision{Outcome: domain.OutcomeNotFound, Reason: domain.ReasonNotSelf}},
		{"no installation", false, sub, app, domain.Decision{Outcome: domain.OutcomeUnauthenticated, Reason: domain.ReasonAppNotInstalled}},
		{"no scope", true, nil, app, domain.Decision{Outcome: domain.OutcomeForbidden, Reason: domain.ReasonScopeMissing}},
	}
	for _, c := range cases {
		if got := domain.AppReceive(app, c.active, c.scopes, c.audience); got != c.want {
			t.Errorf("%s: %+v, want %+v", c.name, got, c.want)
		}
	}
	if !domain.IsAppReceive("agent.events.consume", domain.KindApp, true) || domain.IsAppReceive("agent.events.consume", domain.KindApp, false) ||
		domain.IsAppReceive("agent.events.consume", domain.KindAgent, true) {
		t.Error("IsAppReceive must hold only for apps in filter mode")
	}
}
