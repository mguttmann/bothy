// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"context"
	"log/slog"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/authz/app"
	"github.com/OWNER/bothy/services/server/internal/kernel/auditing"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
)

// The limits of the limit world, standing for terminal.max_sessions_per_person and
// terminal.max_sessions_per_host.
const (
	termPerPerson = 2
	termPerHost   = 3
)

// newLimitWorld is the terminal world of M2-40 with both session limits configured.
func newLimitWorld(t *testing.T) *termWorld {
	t.Helper()
	w := newTermWorld(t)
	w.cfg.MaxSessionsPerPerson, w.cfg.MaxSessionsPerHost = termPerPerson, termPerHost
	w.rebuild(t)
	return w
}

// counted returns r with the open sessions of the person and of the host.
func counted(r authorize.Resource, person, host int) authorize.Resource {
	t := *r.Terminal
	t.OpenSessionsOfPerson, t.OpenSessionsOfHost = person, host
	r.Terminal = &t
	return r
}

// TestTerminalOpenLimitGuard (Design note 9 of M2-46; 24 sections 6.1 step 2 and 6.3): the limit
// guard of terminal.open with "at least the key value", after SR12 and before host_not_active and
// terminal_profile_unknown; attaching is never counted; missing counts are an internal error.
func TestTerminalOpenLimitGuard(t *testing.T) {
	t.Parallel()
	w := newLimitWorld(t)
	alex := w.person(w.alex)
	limit := "terminal_session_limit"

	w.allow(t, alex, actOpen, counted(hostOpen(w.r1), termPerPerson-1, termPerHost-1))
	w.expect(t, alex, actOpen, counted(hostOpen(w.r1), termPerPerson, 0), authorize.OutcomeForbidden, limit)
	w.expect(t, alex, actOpen, counted(hostOpen(w.r1), 0, termPerHost), authorize.OutcomeForbidden, limit)

	// After SR12: the Owner has no session user on R1 and is refused by SR12 even at the limit.
	w.expect(t, w.person(w.owner), actOpen, counted(hostOpen(w.r1), termPerPerson, termPerHost), authorize.OutcomeForbidden,
		"terminal_no_session_user")
	// Before host_not_active and terminal_profile_unknown.
	offline := counted(hostOpen(w.r1), termPerPerson, 0)
	offline.Terminal.HostConnected = false
	w.expect(t, alex, actOpen, offline, authorize.OutcomeForbidden, limit)
	unknown := counted(hostOpen(w.r1), 0, termPerHost)
	unknown.Terminal.ProfileKnown = false
	w.expect(t, alex, actOpen, unknown, authorize.OutcomeForbidden, limit)

	// Attaching creates no session and is not counted.
	ses := w.session(w.alex, w.r1, "detached", w.clk.Now())
	ses.Terminal.OpenSessionsOfPerson, ses.Terminal.OpenSessionsOfHost = termPerPerson, termPerHost
	w.allow(t, alex, authorize.ActionTerminalAttach, ses)

	// Missing counts for terminal.open are an internal error, not a denial.
	_, err := w.az.Authorize(context.Background(), alex, actOpen, host(w.r1))
	if e, ok := errs.As(err); !ok || e.Code != errs.CodeInternalError {
		t.Fatalf("terminal.open without TerminalFacts: err %v, want internal_error", err)
	}
}

// TestTerminalOpenLimitGuard_NotConfigured: limits left zero in a hand-built configuration are not
// configured and never deny (the production binding requires at least 1 for both keys).
func TestTerminalOpenLimitGuard_NotConfigured(t *testing.T) {
	t.Parallel()
	w := newTermWorld(t)
	w.allow(t, w.person(w.alex), actOpen, counted(hostOpen(w.r1), 1000, 1000))
}

// TestNewAuthorizer_NegativeSessionLimit: a negative limit is refused like the other limits.
func TestNewAuthorizer_NegativeSessionLimit(t *testing.T) {
	t.Parallel()
	w := newTermWorld(t)
	for _, c := range []struct{ person, host int }{{-1, 0}, {0, -1}} {
		cfg := w.cfg
		cfg.MaxSessionsPerPerson, cfg.MaxSessionsPerHost = c.person, c.host
		_, err := app.NewAuthorizer(cfg, app.AuthorizerDeps{
			Catalog: app.NewCatalog(slog.New(slog.DiscardHandler)), Store: w.store, Principals: w.principals,
			Credentials: w.creds, Structure: w.structure, Policies: w.policies, Hosts: w.hosts, Versions: w.versions, Recorder: w.rec,
			Clock: w.clk,
		})
		if err == nil {
			t.Errorf("limits %d/%d: NewAuthorizer accepted a negative limit", c.person, c.host)
		}
	}
}

// TestDecisionErr_TerminalSessionLimit (07 section 4.1, 24 section 13): the limit denial answers
// 403 terminal_denied with reason session_limit and is audited with the reason
// terminal_session_limit.
func TestDecisionErr_TerminalSessionLimit(t *testing.T) {
	t.Parallel()
	w := newLimitWorld(t)
	before, _ := w.rec.counts()
	d := w.decide(t, context.Background(), w.person(w.alex), actOpen, counted(hostOpen(w.r1), termPerPerson, 0))
	if d.Outcome != authorize.OutcomeForbidden || d.Reason != authorize.Reason(auditing.ReasonTerminalSessionLimit) {
		t.Fatalf("decision %+v", d)
	}
	e, ok := errs.As(d.Err())
	if !ok || e.Code != errs.CodeTerminalDenied || e.Reason != "session_limit" || e.Kind().Status() != 403 {
		t.Fatalf("problem %+v, want 403 terminal_denied session_limit", e)
	}
	after, _ := w.rec.counts()
	if after != before+1 {
		t.Fatalf("denials %d -> %d, want one more", before, after)
	}
	got := w.rec.denials[after-1]
	if got.Reason != auditing.ReasonTerminalSessionLimit || got.Decision != auditing.DecisionDeny || got.Action != string(actOpen) {
		t.Fatalf("audit entry %+v", got)
	}
}
