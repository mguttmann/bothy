// SPDX-License-Identifier: Apache-2.0

package domain_test

import (
	"slices"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/agents/domain"
)

// t0 is the reference time of the presence tests.
var t0 = time.Date(2026, 10, 20, 9, 0, 0, 0, time.UTC)

// TestStoredStateBoundary: a stored online or degraded state holds before last + stale and is
// offline at and after it (inclusive boundary, D-M2-20-4); other states and stale 0 stay as
// stored.
func TestStoredStateBoundary(t *testing.T) {
	t.Parallel()
	const stale = 90 * time.Second
	cases := []struct {
		stored domain.RuntimeState
		now    time.Time
		stale  time.Duration
		want   domain.RuntimeState
	}{
		{domain.RuntimeDegraded, t0.Add(stale - time.Nanosecond), stale, domain.RuntimeDegraded},
		{domain.RuntimeDegraded, t0.Add(stale), stale, domain.RuntimeOffline},
		{domain.RuntimeOnline, t0.Add(stale + time.Second), stale, domain.RuntimeOffline},
		{domain.RuntimeUnknown, t0.Add(stale + time.Second), stale, domain.RuntimeUnknown},
		{domain.RuntimeOnline, t0.Add(time.Hour), 0, domain.RuntimeOnline},
	}
	for _, c := range cases {
		if got := domain.StoredState(c.stored, t0, c.now, c.stale); got != c.want {
			t.Errorf("StoredState(%s, %v, %v) = %s, want %s", c.stored, c.now, c.stale, got, c.want)
		}
	}
}

// TestStartRule: a stored live agent older than agent.presence_timeout plus
// agent.heartbeat_persist_interval is offline at once; a younger one gets start plus
// agent.presence_timeout; without last_heartbeat_at it is offline at once (note 7).
func TestStartRule(t *testing.T) {
	t.Parallel()
	const timeout, persist = 90 * time.Second, 60 * time.Second
	start := t0.Add(time.Hour)
	if off, _ := domain.StartRule(domain.StoredPresence{State: domain.RuntimeOnline, LastHeartbeatAt: start.Add(-timeout - persist - time.Second)},
		start, timeout, persist); !off {
		t.Error("an old heartbeat is not offline at once")
	}
	if off, _ := domain.StartRule(domain.StoredPresence{State: domain.RuntimeOnline}, start, timeout, persist); !off {
		t.Error("a missing heartbeat is not offline at once")
	}
	off, deadline := domain.StartRule(domain.StoredPresence{State: domain.RuntimeDegraded, LastHeartbeatAt: start.Add(-timeout)}, start, timeout, persist)
	if off || !deadline.Equal(start.Add(timeout)) {
		t.Errorf("young heartbeat: offline %v deadline %v, want false %v", off, deadline, start.Add(timeout))
	}
}

// TestAssess: one case per difference kind and warning code (note 8); billing_mode_mismatch
// only after billing.mismatch_alert_after; nothing while offline or unknown.
func TestAssess(t *testing.T) {
	t.Parallel()
	const after = 5 * time.Minute
	org := domain.Target{BillingMode: domain.Organization, ToolClass: domain.ToolChat}
	cases := map[string]struct {
		target domain.Target
		report domain.Report
		state  domain.RuntimeState
		mode   bool // kind (a)
		any    bool
		warn   []string // with the difference older than after
	}{
		"equal": {org, domain.Report{BillingMode: domain.Organization, Evidence: &domain.Evidence{KeyType: "api_key"}}, domain.RuntimeOnline, false, false, nil},
		"a":     {org, domain.Report{BillingMode: domain.Personal}, domain.RuntimeOnline, true, true, []string{domain.WarningBillingModeMismatch}},
		"b": {org, domain.Report{BillingModesSupported: []domain.BillingMode{domain.Personal}}, domain.RuntimeDegraded, false, true,
			[]string{domain.WarningBillingModeUnsupported}},
		"b supported": {org, domain.Report{BillingModesSupported: []domain.BillingMode{domain.Personal, domain.Organization}}, domain.RuntimeOnline, false, false, nil},
		"c":           {org, domain.Report{Evidence: &domain.Evidence{KeyType: "unknown"}}, domain.RuntimeOnline, false, true, []string{domain.WarningBillingEvidenceUnknown}},
		"tool higher": {org, domain.Report{ToolClass: domain.ToolHost}, domain.RuntimeOnline, false, false, []string{domain.WarningToolClassMismatch}},
		"tool lower": {domain.Target{BillingMode: domain.Organization, ToolClass: domain.ToolHost}, domain.Report{ToolClass: domain.ToolChat}, domain.RuntimeOnline,
			false, false, []string{domain.WarningToolClassMismatch}},
		"offline": {org, domain.Report{BillingMode: domain.Personal, ToolClass: domain.ToolHost, Evidence: &domain.Evidence{KeyType: "unknown"}},
			domain.RuntimeOffline, false, false, nil},
		"unknown": {org, domain.Report{BillingMode: domain.Personal}, domain.RuntimeUnknown, false, false, nil},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			as := domain.Assess(c.target, c.report, c.state)
			if as.ModeDiffers != c.mode || as.Differs() != c.any {
				t.Fatalf("assessment %+v, want mode %v any %v", as, c.mode, c.any)
			}
			if got := as.Warnings(t0, t0.Add(after+time.Millisecond), after); !slices.Equal(got, c.warn) {
				t.Fatalf("warnings %v, want %v", got, c.warn)
			}
		})
	}
	as := domain.Assess(org, domain.Report{BillingMode: domain.Personal}, domain.RuntimeOnline)
	if got := as.Warnings(t0, t0.Add(after), after); len(got) != 0 {
		t.Errorf("billing_mode_mismatch exactly at billing.mismatch_alert_after: %v", got)
	}
}
