// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/authz"
	"github.com/OWNER/bothy/services/server/internal/authz/adapters/fake"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// wantErr checks code and reason of err.
func wantErr(t *testing.T, err error, code errs.Code, reason string) {
	t.Helper()
	e, ok := errs.As(err)
	if !ok || e.Code != code || e.Reason != reason {
		t.Fatalf("error %v (reason %q), want %s %s", err, reasonOf(e), code, reason)
	}
}

func reasonOf(e *errs.Error) string {
	if e == nil {
		return ""
	}
	return e.Reason
}

// allowRow is one allow row of audit_events.
type allowRow struct {
	Action, ResourceKind string
	ResourceID, Actor    ids.ID
	Details              string
}

// allows returns the allow rows with seq above after, ascending.
func (w *pgWorld) allows(t *testing.T, after int64) []allowRow {
	t.Helper()
	rows, err := w.db.SuperConn(t).Query(context.Background(), `SELECT action, coalesce(resource_kind, ''), coalesce(resource_id::text, ''),
		coalesce(actor_principal_id::text, ''), details::text FROM audit_events WHERE decision = 'allow' AND seq > $1 ORDER BY seq`, after)
	mustOK(t, err)
	defer rows.Close()
	var out []allowRow
	for rows.Next() {
		var r allowRow
		var rid, aid string
		mustOK(t, rows.Scan(&r.Action, &r.ResourceKind, &rid, &aid, &r.Details))
		if rid != "" {
			r.ResourceID = ids.MustParse(rid)
		}
		if aid != "" {
			r.Actor = ids.MustParse(aid)
		}
		out = append(out, r)
	}
	mustOK(t, rows.Err())
	return out
}

// auditSeq returns the highest seq of audit_events.
func (w *pgWorld) auditSeq(t *testing.T) int64 {
	t.Helper()
	var n int64
	mustOK(t, w.db.SuperConn(t).QueryRow(context.Background(), `SELECT coalesce(max(seq), 0) FROM audit_events`).Scan(&n))
	return n
}

// TestAZ_M31_GrantTriggerToAgentForbidden: even the owner cannot give an agent trigger.
func TestAZ_M31_GrantTriggerToAgentForbidden(t *testing.T) {
	t.Parallel()
	w := newPGWorld(t)
	_, err := w.admin.CreateGrant(context.Background(), w.user(w.owner), authz.CreateGrantCmd{
		SubjectKind: "agent", SubjectID: w.lotse, TargetKind: "space", TargetID: w.support, Level: "trigger", Reason: "x",
	})
	wantErr(t, err, errs.CodeForbidden, "subject_kind_not_allowed")
}

// TestAZ_M36_StaleStepUp: a step-up older than auth.step_up_max_age gives step_up_required.
func TestAZ_M36_StaleStepUp(t *testing.T) {
	t.Parallel()
	w := newPGWorld(t)
	owner := w.user(w.owner)
	owner.StepUpAt = w.clk.Now().Add(-10 * time.Minute)
	_, err := w.admin.CreateGrant(context.Background(), owner, authz.CreateGrantCmd{
		SubjectKind: "user", SubjectID: w.sam, TargetKind: "space", TargetID: w.marketing, Level: "read", Reason: "x",
	})
	if errs.KindOf(err) != errs.KindStepUpRequired {
		t.Fatalf("error %v, want step_up_required", err)
	}
	// A personal token of the owner acts without owner rights (SR5).
	tok := actor.Actor{ID: w.owner, Kind: actor.KindUser, TokenID: ids.New()}
	w.creds.PutToken(tok.TokenID, fakeToken(w.owner))
	_, err = w.admin.CreateGroup(context.Background(), tok, authz.CreateGroupCmd{Name: "X"})
	wantErr(t, err, errs.CodeForbidden, "owner_only")
}

// withManager gives Alex level with manage on Support instead of G-1.
func (w *pgWorld) withManager(t *testing.T, level string) {
	t.Helper()
	ctx := context.Background()
	_, err := w.admin.RevokeGrant(ctx, w.user(w.owner), w.g1, "")
	mustOK(t, err)
	_, err = w.admin.CreateGrant(ctx, w.user(w.owner), authz.CreateGrantCmd{SubjectKind: "user", SubjectID: w.alex, TargetKind: "space",
		TargetID: w.support, Level: level, Capabilities: []string{"manage", "upload"}, Reason: "team lead"})
	mustOK(t, err)
}

// TestAZ_M37_DelegationAboveOwnLevel: Alex with write and manage cannot give trigger.
func TestAZ_M37_DelegationAboveOwnLevel(t *testing.T) {
	t.Parallel()
	w := newPGWorld(t)
	w.withManager(t, "write")
	_, err := w.admin.CreateGrant(context.Background(), w.user(w.alex), authz.CreateGrantCmd{SubjectKind: "user", SubjectID: w.sam,
		TargetKind: "channel", TargetID: w.allgemein, Level: "trigger", Reason: "x"})
	wantErr(t, err, errs.CodeForbidden, "delegation_exceeds_own_level")
	// Within the own level it works, and the delegate sees the channel.
	_, err = w.admin.CreateGrant(context.Background(), w.user(w.alex), authz.CreateGrantCmd{SubjectKind: "user", SubjectID: w.sam,
		TargetKind: "channel", TargetID: w.allgemein, Level: "write", Capabilities: []string{"upload"}, Reason: "helps"})
	mustOK(t, err)
}

// TestAZ_M38_DelegationBeyondOwnCapabilities: Alex with trigger and manage but without export.
func TestAZ_M38_DelegationBeyondOwnCapabilities(t *testing.T) {
	t.Parallel()
	w := newPGWorld(t)
	w.withManager(t, "trigger")
	_, err := w.admin.CreateGrant(context.Background(), w.user(w.alex), authz.CreateGrantCmd{SubjectKind: "user", SubjectID: w.sam,
		TargetKind: "channel", TargetID: w.allgemein, Level: "read", Capabilities: []string{"export"}, Reason: "x"})
	wantErr(t, err, errs.CodeForbidden, "delegation_exceeds_own_capabilities")
}

// TestAZ_M39_RevokeAboveOwnLevel: Alex with write and manage cannot revoke the owner's trigger
// grant for Sam.
func TestAZ_M39_RevokeAboveOwnLevel(t *testing.T) {
	t.Parallel()
	w := newPGWorld(t)
	ctx := context.Background()
	g, err := w.admin.CreateGrant(ctx, w.user(w.owner), authz.CreateGrantCmd{SubjectKind: "user", SubjectID: w.sam, TargetKind: "channel",
		TargetID: w.allgemein, Level: "trigger", Reason: "support"})
	mustOK(t, err)
	w.withManager(t, "write")
	v := w.version(t)
	_, err = w.admin.RevokeGrant(ctx, w.user(w.alex), g.ID, "")
	wantErr(t, err, errs.CodeForbidden, "delegation_exceeds_own_level")
	if w.version(t) != v {
		t.Fatal("a denied revocation raised the rights version")
	}
}

// TestGroupDelete_RevokesGrantsAndMembersLoseAccess: deleting Leser revokes G-4 and Sam loses
// Allgemein at once, with access.changed.
func TestGroupDelete_RevokesGrantsAndMembersLoseAccess(t *testing.T) {
	t.Parallel()
	w := newPGWorld(t)
	ctx := context.Background()
	res := authorize.Resource{Kind: authorize.ResourceChannel, ID: w.allgemein}
	if d, _ := w.az.Authorize(ctx, w.user(w.sam), authorize.ActionChannelRead, res); !d.Allowed() {
		t.Fatal("Sam does not read Allgemein before")
	}
	seq := w.lastSeq(t)
	mustOK(t, w.admin.DeleteGroup(ctx, w.user(w.owner), w.leser, "reorganised"))
	g4, _, err := w.store.GetGrant(ctx, w.g4)
	mustOK(t, err)
	if g4.RevokedAt.IsZero() || g4.RevokedBy != w.owner || g4.RevokeReason != "reorganised" {
		t.Fatalf("G-4 %+v", g4)
	}
	if d, _ := w.az.Authorize(ctx, w.user(w.sam), authorize.ActionChannelRead, res); d.Outcome != authorize.OutcomeNotFound {
		t.Fatalf("Sam after the deletion: %+v", d)
	}
	e := eventFor(t, w.events(t, seq), w.sam)
	if p := decodePayload(t, e.Payload); len(p.Removed) != 2 {
		t.Fatalf("payload %s", e.Payload)
	}
	// Deleting again is a call without effect.
	v := w.version(t)
	mustOK(t, w.admin.DeleteGroup(ctx, w.user(w.owner), w.leser, ""))
	if w.version(t) != v {
		t.Fatal("deleting a deleted group raised the rights version")
	}
}

// TestGroupMembers: duplicate membership is conflict, an agent or unknown person not found, an
// expiry in the past validation_failed; removing a person that never was a member is not found,
// removing twice is without effect (no rights version, audit no_change).
func TestGroupMembers(t *testing.T) {
	t.Parallel()
	w := newPGWorld(t)
	ctx := context.Background()
	owner := w.user(w.owner)
	_, err := w.admin.AddGroupMember(ctx, owner, w.leser, authz.AddMemberCmd{UserID: w.sam})
	wantErr(t, err, errs.CodeConflict, "")
	_, err = w.admin.AddGroupMember(ctx, owner, w.leser, authz.AddMemberCmd{UserID: w.lotse})
	wantErr(t, err, errs.CodeNotFound, "")
	_, err = w.admin.AddGroupMember(ctx, owner, w.leser, authz.AddMemberCmd{UserID: ids.New()})
	wantErr(t, err, errs.CodeNotFound, "")
	_, err = w.admin.AddGroupMember(ctx, owner, w.leser, authz.AddMemberCmd{UserID: w.robin, ExpiresAt: w.clk.Now()})
	wantErr(t, err, errs.CodeValidationFailed, "")
	m, err := w.admin.AddGroupMember(ctx, owner, w.leser, authz.AddMemberCmd{UserID: w.robin, ExpiresAt: w.clk.Now().Add(time.Hour)})
	mustOK(t, err)
	if m.UserID != w.robin || m.GroupID != w.leser {
		t.Fatalf("membership %+v", m)
	}
	err = w.admin.RemoveGroupMember(ctx, owner, w.leser, w.alex)
	wantErr(t, err, errs.CodeNotFound, "")
	mustOK(t, w.admin.RemoveGroupMember(ctx, owner, w.leser, w.robin))
	v, seq, aseq := w.version(t), w.lastSeq(t), w.auditSeq(t)
	mustOK(t, w.admin.RemoveGroupMember(ctx, owner, w.leser, w.robin))
	if w.version(t) != v || len(w.events(t, seq)) != 0 {
		t.Fatal("a removal without effect raised the version or wrote an event")
	}
	rows := w.allows(t, aseq)
	if len(rows) != 1 || !strings.Contains(strings.ReplaceAll(rows[0].Details, " ", ""), `"no_change":true`) {
		t.Fatalf("audit of the call without effect: %+v", rows)
	}
	// An expired membership is replaced by a new one, not a conflict.
	w.clk.Advance(72 * time.Hour)
	_, err = w.admin.AddGroupMember(ctx, w.user(w.owner), w.leser, authz.AddMemberCmd{UserID: w.sam})
	mustOK(t, err)
}

// TestGrantTargets: an unknown target and a host are not found, an archived channel or space is
// conflict, a missing reason validation_failed; a revoked grant revoked again is without effect.
func TestGrantTargets(t *testing.T) {
	t.Parallel()
	w := newPGWorld(t)
	ctx := context.Background()
	owner := w.user(w.owner)
	cmd := func(kind string, id ids.ID) authz.CreateGrantCmd {
		return authz.CreateGrantCmd{SubjectKind: "user", SubjectID: w.robin, TargetKind: kind, TargetID: id, Level: "read", Reason: "reads"}
	}
	_, err := w.admin.CreateGrant(ctx, owner, cmd("channel", ids.New()))
	wantErr(t, err, errs.CodeNotFound, "")
	_, err = w.admin.CreateGrant(ctx, owner, authz.CreateGrantCmd{SubjectKind: "user", SubjectID: w.robin, TargetKind: "host", TargetID: ids.New(),
		Capabilities: []string{"terminal"}, Reason: "x"})
	wantErr(t, err, errs.CodeNotFound, "")
	c, _, _ := w.structure.Channel(ctx, w.kampagnen)
	c.Archived = true
	w.structure.PutChannel(c)
	_, err = w.admin.CreateGrant(ctx, owner, cmd("channel", w.kampagnen))
	wantErr(t, err, errs.CodeConflict, "channel_archived")
	w.structure.SetSpaceArchived(w.engineering, true)
	_, err = w.admin.CreateGrant(ctx, owner, cmd("space", w.engineering))
	wantErr(t, err, errs.CodeConflict, "channel_archived")
	noReason := cmd("space", w.support)
	noReason.Reason = " "
	_, err = w.admin.CreateGrant(ctx, owner, noReason)
	wantErr(t, err, errs.CodeValidationFailed, "")
	_, err = w.admin.CreateGrant(ctx, owner, authz.CreateGrantCmd{SubjectKind: "user", SubjectID: w.lotse, TargetKind: "space", TargetID: w.support, Level: "read", Reason: "x"})
	wantErr(t, err, errs.CodeNotFound, "")
	_, err = w.admin.RevokeGrant(ctx, owner, ids.New(), "")
	wantErr(t, err, errs.CodeNotFound, "")
	_, err = w.admin.RevokeGrant(ctx, owner, w.g2, "")
	mustOK(t, err)
	v, seq := w.version(t), w.lastSeq(t)
	g, err := w.admin.RevokeGrant(ctx, owner, w.g2, "")
	mustOK(t, err)
	if g.RevokedAt.IsZero() || w.version(t) != v || len(w.events(t, seq)) != 0 {
		t.Fatalf("second revocation: %+v, version %d (was %d)", g, w.version(t), v)
	}
}

// TestGroups: names are checked (SEC-IN-4) and unique without regard to case; a group the caller
// may not see is not found like an unknown one; a member gets owner_only; a claim group keeps its
// name.
func TestGroups(t *testing.T) {
	t.Parallel()
	w := newPGWorld(t)
	ctx := context.Background()
	owner := w.user(w.owner)
	_, err := w.admin.CreateGroup(ctx, owner, authz.CreateGroupCmd{Name: "Bad\u202ename"})
	wantErr(t, err, errs.CodeValidationFailed, "")
	_, err = w.admin.CreateGroup(ctx, owner, authz.CreateGroupCmd{Name: "LESER"})
	wantErr(t, err, errs.CodeConflict, "")
	name := "Leserinnen"
	_, err = w.admin.UpdateGroup(ctx, w.user(w.alex), w.leser, authz.UpdateGroupCmd{Name: &name})
	wantErr(t, err, errs.CodeNotFound, "")
	_, err = w.admin.UpdateGroup(ctx, owner, ids.New(), authz.UpdateGroupCmd{Name: &name})
	wantErr(t, err, errs.CodeNotFound, "")
	_, err = w.admin.UpdateGroup(ctx, w.user(w.sam), w.leser, authz.UpdateGroupCmd{Name: &name})
	wantErr(t, err, errs.CodeForbidden, "owner_only")
	g, err := w.admin.UpdateGroup(ctx, owner, w.leser, authz.UpdateGroupCmd{Name: &name})
	mustOK(t, err)
	if g.Name != name {
		t.Fatalf("group %+v", g)
	}
	claim, provider := ids.New(), ids.New()
	// groups.provider_id references identity_providers since M1-36.
	w.sql(t, `INSERT INTO identity_providers (id, kind, name, config) VALUES ($1, 'oidc', 'Company SSO', '{}')`, provider.String())
	w.sql(t, `INSERT INTO groups (id, name, source, provider_id, external_ref, created_by, created_at) VALUES ($1, 'Claim', 'oidc_claim', $2, 'team', $3, $4)`,
		claim.String(), provider.String(), w.owner.String(), pgStart)
	other := "Renamed"
	_, err = w.admin.UpdateGroup(ctx, owner, claim, authz.UpdateGroupCmd{Name: &other})
	wantErr(t, err, errs.CodeConflict, "claim_managed_group")
	desc := "from the provider"
	_, err = w.admin.UpdateGroup(ctx, owner, claim, authz.UpdateGroupCmd{Description: &desc})
	mustOK(t, err)
	_, err = w.admin.AddGroupMember(ctx, owner, claim, authz.AddMemberCmd{UserID: w.robin})
	wantErr(t, err, errs.CodeConflict, "claim_managed_group")
	// The caller must be the request principal.
	_, err = w.admin.CreateGroup(ctxOf(w.user(w.alex)), owner, authz.CreateGroupCmd{Name: "Mixed"})
	if errs.KindOf(err) != errs.KindInternal {
		t.Fatalf("a caller other than the request principal: %v", err)
	}
}

// TestAdminAudit (brief M1-29 test 13): one allow entry per use case with action, actor and
// resource; the details carry ids and codes, never names.
func TestAdminAudit(t *testing.T) {
	t.Parallel()
	w := newPGWorld(t)
	ctx := context.Background()
	owner := w.user(w.owner)
	type step struct {
		action, kind string
		run          func() ids.ID
	}
	var group ids.ID
	var grant ids.ID
	steps := []step{
		{"group.create", "group", func() ids.ID {
			g, err := w.admin.CreateGroup(ctx, owner, authz.CreateGroupCmd{Name: "Marketing Team", Description: "campaigns"})
			mustOK(t, err)
			group = g.ID
			return g.ID
		}},
		{"group.update", "group", func() ids.ID {
			n := "Marketing Crew"
			_, err := w.admin.UpdateGroup(ctx, owner, group, authz.UpdateGroupCmd{Name: &n})
			mustOK(t, err)
			return group
		}},
		{"group.member.add", "group", func() ids.ID {
			_, err := w.admin.AddGroupMember(ctx, owner, group, authz.AddMemberCmd{UserID: w.robin})
			mustOK(t, err)
			return group
		}},
		{"grant.create", "grant", func() ids.ID {
			g, err := w.admin.CreateGrant(ctx, owner, authz.CreateGrantCmd{SubjectKind: "group", SubjectID: group, TargetKind: "space",
				TargetID: w.marketing, Level: "write", Capabilities: []string{"upload"}, Reason: "campaign work", ExpiresAt: w.clk.Now().Add(time.Hour)})
			mustOK(t, err)
			grant = g.ID
			return g.ID
		}},
		{"grant.revoke", "grant", func() ids.ID {
			_, err := w.admin.RevokeGrant(ctx, owner, grant, "done")
			mustOK(t, err)
			return grant
		}},
		{"group.member.remove", "group", func() ids.ID {
			mustOK(t, w.admin.RemoveGroupMember(ctx, owner, group, w.robin))
			return group
		}},
		{"group.delete", "group", func() ids.ID {
			mustOK(t, w.admin.DeleteGroup(ctx, owner, group, "closed"))
			return group
		}},
	}
	for _, s := range steps {
		seq := w.auditSeq(t)
		id := s.run()
		rows := w.allows(t, seq)
		if len(rows) != 1 {
			t.Fatalf("%s: %d allow rows, want 1", s.action, len(rows))
		}
		r := rows[0]
		if r.Action != s.action || r.ResourceKind != s.kind || r.ResourceID != id || r.Actor != w.owner {
			t.Fatalf("%s: row %+v", s.action, r)
		}
		var details map[string]any
		mustOK(t, json.Unmarshal([]byte(r.Details), &details))
		for _, name := range []string{"Marketing", "robin", "owner"} {
			if strings.Contains(r.Details, name) {
				t.Fatalf("%s: details name %q: %s", s.action, name, r.Details)
			}
		}
	}
}

// fakeToken is a live personal token of p.
func fakeToken(p ids.ID) fake.Token { return fake.Token{PrincipalID: p} }
