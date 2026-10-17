// SPDX-License-Identifier: Apache-2.0

package httpapi_test

import (
	"net/http"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// The rows of the matrix of 07 section 11.1 that the API of M1-48 serves, with real access
// tokens, DPoP, step-up and personal tokens.

// TestAZM31_OwnerGrantsTriggerToAgent: even the owner cannot give an agent trigger.
func TestAZM31_OwnerGrantsTriggerToAgent(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	after := w.auditSeq()
	r := w.boss.call(http.MethodPost, "/api/v1/grants", grantBody("agent", w.lotse, "space", w.support, "trigger"))
	w.wantReason(r, http.StatusForbidden, "forbidden", "subject_kind_not_allowed")
	// An agent with a capability beyond upload is refused the same way.
	r = w.boss.call(http.MethodPost, "/api/v1/grants", grantBody("agent", w.lotse, "space", w.support, "read", "manage"))
	w.wantReason(r, http.StatusForbidden, "forbidden", "subject_kind_not_allowed")
	if rows := w.audits("grant.create", after); len(rows) != 1 || rows[0].Decision != "deny" || rows[0].Reason != "subject_kind_not_allowed" {
		t.Fatalf("deny audit %+v", rows)
	}
	// write and upload for an agent is the owner's to give.
	w.want(w.boss.call(http.MethodPost, "/api/v1/grants", grantBody("agent", w.lotse, "space", w.support, "write", "upload")), http.StatusCreated, "")
}

// TestAZM35_PersonalTokenCannotGrant: Sam with trigger and manage on Marketing, acting with a
// personal token, gets step_up_not_possible for a read grant to Robin.
func TestAZM35_PersonalTokenCannotGrant(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	w.grantAs(w.boss.actor(), "user", w.sam, "space", w.marketing, "trigger", "manage")
	sam := w.login(w.sam)
	sam.stepUp()
	secret := sam.personalToken()
	r := w.withToken(secret, http.MethodPost, "/api/v1/grants", grantBody("user", w.robin, "space", w.marketing, "read"))
	w.wantReason(r, http.StatusForbidden, "forbidden", "step_up_not_possible")
	// The same person in a session with a fresh step-up may.
	w.want(sam.call(http.MethodPost, "/api/v1/grants", grantBody("user", w.robin, "space", w.marketing, "read")), http.StatusCreated, "")
}

// TestAZM36_StaleStepUp: an owner whose step-up is older than auth.step_up_max_age gets 401
// step_up_required.
func TestAZM36_StaleStepUp(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	w.clk.Advance(5*time.Minute + time.Second)
	w.want(w.boss.call(http.MethodPost, "/api/v1/grants", grantBody("user", w.robin, "space", w.marketing, "read")), http.StatusUnauthorized, "step_up_required")
	w.want(w.boss.call(http.MethodPost, "/api/v1/groups", `{"name":"Team"}`), http.StatusUnauthorized, "step_up_required")
	w.boss.stepUp()
	w.want(w.boss.call(http.MethodPost, "/api/v1/grants", grantBody("user", w.robin, "space", w.marketing, "read")), http.StatusCreated, "")
}

// alexWriteManager replaces G-1 by write with manage and upload on Support (AZ-M37, AZ-M39).
func (w *world) alexWriteManager() *client {
	w.t.Helper()
	w.want(w.boss.call(http.MethodPost, revokePath(w.g1), `{"reason":"new role"}`), http.StatusOK, "")
	return w.manager(w.alex, "write", "upload")
}

// TestAZM37_DelegationAboveOwnLevel: Alex with write and manage cannot give trigger.
func TestAZM37_DelegationAboveOwnLevel(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	alex := w.alexWriteManager()
	r := alex.call(http.MethodPost, "/api/v1/grants", grantBody("user", w.sam, "channel", w.allgemein, "trigger"))
	w.wantReason(r, http.StatusForbidden, "forbidden", "delegation_exceeds_own_level")
	w.want(alex.call(http.MethodPost, "/api/v1/grants", grantBody("user", w.sam, "channel", w.allgemein, "write")), http.StatusCreated, "")
}

// TestAZM38_DelegationAboveOwnCapabilities: Alex with trigger and manage but without export
// cannot give export.
func TestAZM38_DelegationAboveOwnCapabilities(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	alex := w.manager(w.alex, "trigger")
	r := alex.call(http.MethodPost, "/api/v1/grants", grantBody("user", w.sam, "channel", w.allgemein, "read", "export"))
	w.wantReason(r, http.StatusForbidden, "forbidden", "delegation_exceeds_own_capabilities")
	// manage itself may be passed on (07 section 2.6 item 2).
	w.want(alex.call(http.MethodPost, "/api/v1/grants", grantBody("user", w.sam, "channel", w.allgemein, "read", "manage")), http.StatusCreated, "")
}

// TestAZM39_RevokeStrongerGrant: Alex with write and manage cannot revoke the owner's trigger
// grant for Sam, but a grant within his limits.
func TestAZM39_RevokeStrongerGrant(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	strong := w.grantAs(w.boss.actor(), "user", w.sam, "space", w.support, "trigger")
	weak := w.grantAs(w.boss.actor(), "user", w.sam, "channel", w.eskalation, "read")
	alex := w.alexWriteManager()
	w.wantReason(alex.call(http.MethodPost, revokePath(strong), `{"reason":"cleanup"}`), http.StatusForbidden, "forbidden", "delegation_exceeds_own_level")
	w.want(alex.call(http.MethodPost, revokePath(weak), `{"reason":"cleanup"}`), http.StatusOK, "")
}

// snapshot returns the rows of grants, group_memberships and sequence_counters as text.
func (w *world) snapshot() string {
	w.t.Helper()
	var s string
	w.scalar(&s, `SELECT concat_ws('|',
		(SELECT coalesce(string_agg(g::text, ',' ORDER BY g.id), '') FROM grants g),
		(SELECT coalesce(string_agg(m::text, ',' ORDER BY m.id), '') FROM group_memberships m),
		(SELECT coalesce(string_agg(c::text, ',' ORDER BY c.name), '') FROM sequence_counters c))`)
	return s
}

// TestAZM21_GrantBreaksI1: the owner gives Sam trigger on Research/Notizen, served by the personal
// Assistent: 409 i1_violation with hint, nothing changed, a deny audit entry.
func TestAZM21_GrantBreaksI1(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	before, after := w.snapshot(), w.auditSeq()
	r := w.boss.call(http.MethodPost, "/api/v1/grants", grantBody("user", w.sam, "channel", w.notizen, "trigger"))
	m := w.want(r, http.StatusConflict, "i1_violation")
	if m["hint"] == nil || m["hint"] == "" {
		t.Fatalf("no hint: %s", r.Body)
	}
	if got := w.snapshot(); got != before {
		t.Fatalf("rows changed by a rolled back grant:\n%s\n%s", before, got)
	}
	rows := w.audits("grant.create", after)
	if len(rows) != 1 || rows[0].Decision != "deny" || rows[0].Reason != "i1_violation" {
		t.Fatalf("audit after the rollback %+v", rows)
	}
}

// TestAZM25_MemberAddBreaksI1: Sam joins a group with trigger on Research: 409 i1_violation with
// hint, nothing changed, a deny audit entry.
func TestAZM25_MemberAddBreaksI1(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	g := w.want(w.boss.call(http.MethodPost, "/api/v1/groups", `{"name":"Forschung"}`), http.StatusCreated, "")
	group := idOf(t, g)
	w.want(w.boss.call(http.MethodPost, "/api/v1/grants", grantBody("group", group, "space", w.research, "trigger")), http.StatusCreated, "")
	before, after := w.snapshot(), w.auditSeq()
	r := w.boss.call(http.MethodPost, membersPath(group), `{"user_id":"`+w.sam.String()+`"}`)
	m := w.want(r, http.StatusConflict, "i1_violation")
	if m["hint"] == nil || m["hint"] == "" {
		t.Fatalf("no hint: %s", r.Body)
	}
	if got := w.snapshot(); got != before {
		t.Fatalf("rows changed by a rolled back membership:\n%s\n%s", before, got)
	}
	rows := w.audits("group.member.add", after)
	if len(rows) != 1 || rows[0].Decision != "deny" || rows[0].Reason != "i1_violation" {
		t.Fatalf("audit after the rollback %+v", rows)
	}
}

// TestManageCannotGrantToAgentOrOnInstance (07 section 2.6 items 4 and 5): with manage, grants
// to agents are subject_kind_not_allowed and grants on the instance owner_only.
func TestManageCannotGrantToAgentOrOnInstance(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	alex := w.manager(w.alex, "trigger")
	w.wantReason(alex.call(http.MethodPost, "/api/v1/grants", grantBody("agent", w.lotse, "space", w.support, "read")),
		http.StatusForbidden, "forbidden", "subject_kind_not_allowed")
	w.wantReason(alex.call(http.MethodPost, "/api/v1/grants", grantBody("user", w.sam, "instance", ids.ID{}, "read")),
		http.StatusForbidden, "forbidden", "owner_only")
}

// TestRequirementBeforeStepUp (07 section 4.2): a caller without the right, or beyond the
// delegation limits, gets 403 and is never asked for a step-up.
func TestRequirementBeforeStepUp(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	sam := w.login(w.sam) // reads Support/Allgemein through Leser, no step-up
	w.wantReason(sam.call(http.MethodPost, "/api/v1/grants", grantBody("user", w.robin, "channel", w.allgemein, "read")),
		http.StatusForbidden, "forbidden", "owner_only")
	w.wantReason(sam.call(http.MethodPost, "/api/v1/groups", `{"name":"Team"}`), http.StatusForbidden, "forbidden", "owner_only")
	alex := w.alexWriteManager()
	w.clk.Advance(6 * time.Minute) // the step-up of Alex is stale now
	w.wantReason(alex.call(http.MethodPost, "/api/v1/grants", grantBody("user", w.sam, "channel", w.allgemein, "trigger")),
		http.StatusForbidden, "forbidden", "delegation_exceeds_own_level")
	w.want(alex.call(http.MethodPost, "/api/v1/grants", grantBody("user", w.sam, "channel", w.allgemein, "read")), http.StatusUnauthorized, "step_up_required")
}
