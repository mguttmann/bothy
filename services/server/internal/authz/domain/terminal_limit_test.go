// SPDX-License-Identifier: Apache-2.0

package domain_test

import (
	"testing"

	"github.com/OWNER/bothy/services/server/internal/authz/domain"
)

// The limits of the limit tests, standing for terminal.max_sessions_per_person and
// terminal.max_sessions_per_host.
const (
	limitPerPerson = 2
	limitPerHost   = 3
)

// live returns connected live facts with a known profile, the counts and the test limits.
func live(person, host int) *domain.TerminalLiveFacts {
	return &domain.TerminalLiveFacts{HostConnected: true, ProfileKnown: true, OpenSessionsOfPerson: person, OpenSessionsOfHost: host,
		MaxSessionsPerPerson: limitPerPerson, MaxSessionsPerHost: limitPerHost}
}

// TestTerminalSessionLimit (24 sections 6.1 step 2 and 6.3, Design note 9 of M2-46): "at least
// the key value" is the limit test for either count; zero limits are not configured.
func TestTerminalSessionLimit(t *testing.T) {
	t.Parallel()
	unset := live(100, 100)
	unset.MaxSessionsPerPerson, unset.MaxSessionsPerHost = 0, 0
	cases := map[string]struct {
		f    domain.TerminalLiveFacts
		want bool
	}{
		"below both limits":          {*live(limitPerPerson-1, limitPerHost-1), false},
		"at the per-person value":    {*live(limitPerPerson, 0), true},
		"above the per-person value": {*live(limitPerPerson+1, 0), true},
		"at the per-host value":      {*live(0, limitPerHost), true},
		"limits not configured":      {*unset, false},
	}
	for name, c := range cases {
		if got := domain.TerminalSessionLimit(c.f); got != c.want {
			t.Errorf("%s: TerminalSessionLimit = %v, want %v", name, got, c.want)
		}
	}
}

// TestTerminalLiveGuard_SessionLimit (Design note 9 of M2-46): terminal_session_limit comes before
// host_not_active and terminal_profile_unknown for terminal.open; terminal.attach is never
// counted.
func TestTerminalLiveGuard_SessionLimit(t *testing.T) {
	t.Parallel()
	active := guardHosts["R1"]
	active.State = "active"
	disabled := active
	disabled.State = "disabled"
	offline := live(limitPerPerson, 0)
	offline.HostConnected = false
	unknown := live(0, limitPerHost)
	unknown.ProfileKnown = false
	cases := map[string]struct {
		action string
		h      domain.HostFacts
		f      *domain.TerminalLiveFacts
		want   domain.Reason
	}{
		"open below the limits":            {domain.ActionTerminalOpen, active, live(1, 1), ""},
		"open at the per-person value":     {domain.ActionTerminalOpen, active, live(limitPerPerson, 0), domain.ReasonTerminalSessionLimit},
		"open at the per-host value":       {domain.ActionTerminalOpen, active, live(0, limitPerHost), domain.ReasonTerminalSessionLimit},
		"limit before a disconnected host": {domain.ActionTerminalOpen, active, offline, domain.ReasonTerminalSessionLimit},
		"limit before a disabled host":     {domain.ActionTerminalOpen, disabled, live(limitPerPerson, 0), domain.ReasonTerminalSessionLimit},
		"limit before an unknown profile":  {domain.ActionTerminalOpen, active, unknown, domain.ReasonTerminalSessionLimit},
		"attach at the limits":             {domain.ActionTerminalAttach, active, live(limitPerPerson, limitPerHost), ""},
	}
	for name, c := range cases {
		reason, ok := domain.TerminalLiveGuard(c.action, c.h, c.f)
		if reason != c.want || ok != (c.want == "") {
			t.Errorf("%s: TerminalLiveGuard = %q, %v; want %q", name, reason, ok, c.want)
		}
	}
}
