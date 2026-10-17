// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"bufio"
	"context"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/authz"
	"github.com/OWNER/bothy/services/server/internal/authz/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
)

// The refs of the deny entries.
func ref(kind string, id ids.ID) txn.Ref { return txn.Ref{Kind: kind, ID: id} }

// declared runs a test use case: fn inside a unit of work declared as a rights change of action
// on res with principals, called by the owner in a session.
func (w *pgWorld) declared(action string, res txn.Ref, principals []ids.ID, fn func(ctx context.Context) error) error {
	return w.change(ctxOf(w.user(w.owner)), action, res, principals, fn)
}

// noop is a test use case that writes nothing.
func noop(context.Context) error { return nil }

// rawGrant inserts a grant directly in SQL, bypassing the rights hooks: the fixtures of an
// already invalid world (brief M1-29 test 5, shrinking actions).
func (w *pgWorld) rawGrant(t *testing.T, subjectKind string, subject ids.ID, channel ids.ID, exp time.Time) ids.ID {
	t.Helper()
	id := ids.New()
	col := "subject_principal_id"
	if subjectKind == "group" {
		col = "subject_group_id"
	}
	var expires any
	if !exp.IsZero() {
		expires = exp
	}
	w.sql(t, `INSERT INTO grants (id, subject_kind, `+col+`, target_kind, target_channel_id, level, capabilities, expires_at, reason, granted_by, granted_at)
		VALUES ($1, $2, $3, 'channel', $4, 'trigger', '{}', $5, 'fixture', $6, $7)`,
		id.String(), subjectKind, subject.String(), channel.String(), expires, w.owner.String(), pgStart)
	return id
}

// rawGroup inserts a local group with one member in SQL, bypassing the hooks.
func (w *pgWorld) rawGroup(t *testing.T, name string, member ids.ID, exp time.Time) ids.ID {
	t.Helper()
	id := ids.New()
	w.sql(t, `INSERT INTO groups (id, name, source, created_by, created_at) VALUES ($1, $2, 'local', $3, $4)`, id.String(), name, w.owner.String(), pgStart)
	var expires any
	if !exp.IsZero() {
		expires = exp
	}
	w.sql(t, `INSERT INTO group_memberships (id, group_id, user_id, added_by, added_at, expires_at) VALUES ($1, $2, $3, $4, $5, $6)`,
		ids.New().String(), id.String(), member.String(), w.owner.String(), pgStart, expires)
	return id
}

// assistentViolated checks the I1 deny details: Assistent with the owner as holder and extra.
func assistentViolated(t *testing.T, w *pgWorld, d denyRow, extra ...ids.ID) {
	t.Helper()
	list, _ := d.Details["violations"].([]any)
	if len(list) == 0 {
		t.Fatalf("deny details without violations: %v", d.Details)
	}
	for _, item := range list {
		v, _ := item.(map[string]any)
		if v["agent_id"] != w.assistent.String() {
			continue
		}
		if v["code"] != "i1_violation" || v["credential_holder_id"] != w.owner.String() {
			t.Fatalf("violation %v", v)
		}
		got, _ := v["principal_ids"].([]any)
		var want []any
		for _, id := range domain.SortIDs(slices.Clone(extra)) {
			want = append(want, id.String())
		}
		if !slices.Equal(got, want) {
			t.Fatalf("principal_ids %v, want %v", got, want)
		}
		return
	}
	t.Fatalf("no violation of Assistent in %v", list)
}

// TestAZ_M21_GrantTriggerOnPersonalTargetRejected: the owner gives Sam trigger on
// Research/Notizen, which the personal Assistent serves.
func TestAZ_M21_GrantTriggerOnPersonalTargetRejected(t *testing.T) {
	t.Parallel()
	w := newPGWorld(t)
	before := w.snapshot(t)
	_, err := w.admin.CreateGrant(context.Background(), w.user(w.owner), authz.CreateGrantCmd{
		SubjectKind: "user", SubjectID: w.sam, TargetKind: "channel", TargetID: w.notizen, Level: "trigger", Reason: "research",
	})
	d := w.expectRejected(t, before, err, errs.CodeI1Violation, "grant.create", ref("channel", w.notizen), w.owner)
	assistentViolated(t, w, d, w.sam)
	var ie *authz.InvariantError
	if !errors.As(err, &ie) || len(ie.Conflicts) != 1 || ie.Conflicts[0].AgentID != w.assistent {
		t.Fatalf("conflicts %+v", ie)
	}
}

// TestAZ_M22_BillingModePersonalWithTriggerRejected: switching Lotse to personal while Alex has
// trigger on its channels.
func TestAZ_M22_BillingModePersonalWithTriggerRejected(t *testing.T) {
	t.Parallel()
	w := newPGWorld(t)
	before := w.snapshot(t)
	err := w.declared("agent.billing_mode.set", ref("agent", w.lotse), nil, func(context.Context) error {
		w.policies.Put(w.lotse, authz.AgentPolicy{BillingMode: domain.BillingPersonal, CredentialHolderID: w.owner})
		return nil
	})
	w.expectRejected(t, before, err, errs.CodeI1Violation, "agent.billing_mode.set", ref("agent", w.lotse), w.owner)
}

// TestAZ_M23_SecondOwnerWithPersonalAgentRejected: owner.add of Alex while Assistent is personal.
func TestAZ_M23_SecondOwnerWithPersonalAgentRejected(t *testing.T) {
	t.Parallel()
	w := newPGWorld(t)
	before := w.snapshot(t)
	err := w.declared("owner.add", ref("user", w.alex), []ids.ID{w.alex}, func(ctx context.Context) error {
		return txSQL(ctx, `INSERT INTO owners (id, user_id, granted_by, reason) VALUES ($1, $2, $3, 'handover')`, ids.New().String(), w.alex.String(), w.owner.String())
	})
	d := w.expectRejected(t, before, err, errs.CodeI1Violation, "owner.add", ref("user", w.alex), w.owner)
	assistentViolated(t, w, d, w.alex)
	if owners, _ := w.principals.ActiveOwnerIDs(context.Background()); len(owners) != 1 {
		t.Fatalf("the owners row survived the rollback: %v", owners)
	}
}

// TestAZ_M24_PersonalOperatorWithTriggerRejected: Assistent becomes operator of Support/Allgemein,
// where Alex has trigger.
func TestAZ_M24_PersonalOperatorWithTriggerRejected(t *testing.T) {
	t.Parallel()
	w := newPGWorld(t)
	before := w.snapshot(t)
	err := w.declared("channel.operator.set", ref("channel", w.allgemein), []ids.ID{w.lotse}, func(ctx context.Context) error {
		if err := w.hook.AddAffected(ctx, w.assistent); err != nil {
			return err
		}
		w.setOperator(t, w.allgemein, w.assistent)
		return nil
	})
	d := w.expectRejected(t, before, err, errs.CodeI1Violation, "channel.operator.set", ref("channel", w.allgemein), w.owner)
	assistentViolated(t, w, d, w.alex)
}

// setOperator sets the effective operator of channel in the fake structure.
func (w *pgWorld) setOperator(t *testing.T, channel, agent ids.ID) {
	t.Helper()
	c, ok, _ := w.structure.Channel(context.Background(), channel)
	if !ok {
		t.Fatal("no channel")
	}
	c.EffectiveOperatorID = agent
	w.structure.PutChannel(c)
}

// TestAZ_M25_MemberOfTriggerGroupRejected: Sam joins a group with trigger on Research.
func TestAZ_M25_MemberOfTriggerGroupRejected(t *testing.T) {
	t.Parallel()
	w := newPGWorld(t)
	ctx := context.Background()
	g, err := w.admin.CreateGroup(ctx, w.user(w.owner), authz.CreateGroupCmd{Name: "Research"})
	mustOK(t, err)
	_, err = w.admin.CreateGrant(ctx, w.user(w.owner), authz.CreateGrantCmd{
		SubjectKind: "group", SubjectID: g.ID, TargetKind: "space", TargetID: w.resea, Level: "trigger", Reason: "research",
	})
	mustOK(t, err)
	before := w.snapshot(t)
	_, err = w.admin.AddGroupMember(ctx, w.user(w.owner), g.ID, authz.AddMemberCmd{UserID: w.sam})
	d := w.expectRejected(t, before, err, errs.CodeI1Violation, "group.member.add", ref("group", g.ID), w.owner)
	assistentViolated(t, w, d, w.sam)
}

// TestAZ_M26_DisableLastOwnerRejected: user.disable of the only owner ends with last_owner.
func TestAZ_M26_DisableLastOwnerRejected(t *testing.T) {
	t.Parallel()
	w := newPGWorld(t)
	before := w.snapshot(t)
	err := w.declared("user.disable", ref("user", w.owner), []ids.ID{w.owner}, func(ctx context.Context) error {
		return txSQL(ctx, `UPDATE principals SET status = 'disabled', disabled_at = now() WHERE id = $1`, w.owner.String())
	})
	d := w.expectRejected(t, before, err, errs.CodeLastOwner, "user.disable", ref("user", w.owner), w.owner)
	if d.Details["code"] != "last_owner" {
		t.Fatalf("details %v", d.Details)
	}
	if e, _ := errs.As(err); e.Hint != authz.HintLastOwner {
		t.Fatalf("hint %q", e.Hint)
	}
}

// p7Case is one case of AZ-P7.
type p7Case func(t *testing.T, w *pgWorld)

// p7Cases holds one case per action of 05 rule 1.6.
var p7Cases = map[string]p7Case{
	"grant.create": func(t *testing.T, w *pgWorld) {
		before := w.snapshot(t)
		_, err := w.admin.CreateGrant(context.Background(), w.user(w.owner), authz.CreateGrantCmd{
			SubjectKind: "user", SubjectID: w.robin, TargetKind: "instance", Level: "trigger", Reason: "all",
		})
		w.expectRejected(t, before, err, errs.CodeI1Violation, "grant.create", ref("instance", ids.ID{}), w.owner)
	},
	"group.member.add": func(t *testing.T, w *pgWorld) {
		ctx := context.Background()
		g, err := w.admin.CreateGroup(ctx, w.user(w.owner), authz.CreateGroupCmd{Name: "Notizen"})
		mustOK(t, err)
		_, err = w.admin.CreateGrant(ctx, w.user(w.owner), authz.CreateGrantCmd{SubjectKind: "group", SubjectID: g.ID, TargetKind: "channel",
			TargetID: w.notizen, Level: "trigger", Reason: "notes"})
		mustOK(t, err)
		before := w.snapshot(t)
		_, err = w.admin.AddGroupMember(ctx, w.user(w.owner), g.ID, authz.AddMemberCmd{UserID: w.robin})
		w.expectRejected(t, before, err, errs.CodeI1Violation, "group.member.add", ref("group", g.ID), w.owner)
	},
	// AZ-P7 for group.member.sync (M1-42): Robin's claim maps him into a group with trigger on
	// Notizen, which the personal Assistent serves; the login's sync is refused and leaves only
	// the deny entry with Robin as actor and resource.
	"group.member.sync": func(t *testing.T, w *pgWorld) {
		ctx := context.Background()
		prov := w.provider(t, "Company")
		g := w.claimGroup(t, "Engineering", prov, "engineering")
		_, err := w.admin.CreateGrant(ctx, w.user(w.owner), authz.CreateGrantCmd{SubjectKind: "group", SubjectID: g, TargetKind: "channel",
			TargetID: w.notizen, Level: "trigger", Reason: "notes"})
		mustOK(t, err)
		before := w.snapshot(t)
		_, err = w.syncClaims(w.robin, prov, "engineering")
		d := w.expectRejected(t, before, err, errs.CodeI1Violation, "group.member.sync", ref("user", w.robin), w.robin)
		if e, _ := errs.As(err); e.Hint != authz.HintI1ClaimSync {
			t.Fatalf("hint %q", e.Hint)
		}
		if d.Details["provider_id"] != prov.String() || !slices.Equal(anyStrings(d.Details["group_ids"]), []string{g.String()}) ||
			!slices.Equal(anyStrings(d.Details["agent_ids"]), []string{w.assistent.String()}) {
			t.Fatalf("details %v", d.Details)
		}
	},
	"owner.add": func(t *testing.T, w *pgWorld) {
		before := w.snapshot(t)
		err := w.declared("owner.add", ref("user", w.sam), []ids.ID{w.sam}, func(ctx context.Context) error {
			return txSQL(ctx, `INSERT INTO owners (id, user_id, granted_by, reason) VALUES ($1, $2, $3, 'x')`, ids.New().String(), w.sam.String(), w.owner.String())
		})
		w.expectRejected(t, before, err, errs.CodeI1Violation, "owner.add", ref("user", w.sam), w.owner)
	},
	"space.operator.set": func(t *testing.T, w *pgWorld) {
		before := w.snapshot(t)
		err := w.declared("space.operator.set", ref("space", w.support), nil, func(ctx context.Context) error {
			if err := w.hook.AddAffected(ctx, w.assistent); err != nil {
				return err
			}
			w.structure.SetDefaultOperator(w.support, w.assistent)
			return nil
		})
		w.structure.SetDefaultOperator(w.support, ids.ID{})
		w.expectRejected(t, before, err, errs.CodeI1Violation, "space.operator.set", ref("space", w.support), w.owner)
	},
	"channel.operator.set": func(t *testing.T, w *pgWorld) {
		before := w.snapshot(t)
		err := w.declared("channel.operator.set", ref("channel", w.eskalation), []ids.ID{w.lotse, w.assistent}, func(context.Context) error {
			w.setOperator(t, w.eskalation, w.assistent)
			return nil
		})
		w.setOperator(t, w.eskalation, w.lotse)
		w.expectRejected(t, before, err, errs.CodeI1Violation, "channel.operator.set", ref("channel", w.eskalation), w.owner)
	},
	"agent.billing_mode.set": func(t *testing.T, w *pgWorld) {
		before := w.snapshot(t)
		err := w.declared("agent.billing_mode.set", ref("agent", w.texter), nil, func(context.Context) error {
			w.policies.Put(w.texter, authz.AgentPolicy{BillingMode: domain.BillingPersonal, CredentialHolderID: w.owner})
			return nil
		})
		w.policies.Put(w.texter, authz.AgentPolicy{BillingMode: domain.BillingOrganization})
		w.expectRejected(t, before, err, errs.CodeI1Violation, "agent.billing_mode.set", ref("agent", w.texter), w.owner)
	},
	"agent.billing_mode.set_all": func(t *testing.T, w *pgWorld) {
		before := w.snapshot(t)
		err := w.declared("agent.billing_mode.set_all", ref("instance", ids.ID{}), nil, func(context.Context) error {
			for _, a := range []ids.ID{w.lotse, w.texter, w.runner} {
				w.policies.Put(a, authz.AgentPolicy{BillingMode: domain.BillingPersonal, CredentialHolderID: w.owner})
			}
			return nil
		})
		for _, a := range []ids.ID{w.lotse, w.texter, w.runner} {
			w.policies.Put(a, authz.AgentPolicy{BillingMode: domain.BillingOrganization})
		}
		w.expectRejected(t, before, err, errs.CodeI1Violation, "agent.billing_mode.set_all", ref("instance", ids.ID{}), w.owner)
	},
	"agent.credential_holder.set": func(t *testing.T, w *pgWorld) {
		before := w.snapshot(t)
		err := w.declared("agent.credential_holder.set", ref("agent", w.assistent), nil, func(context.Context) error {
			w.policies.Put(w.assistent, authz.AgentPolicy{BillingMode: domain.BillingPersonal, CredentialHolderID: w.alex})
			return nil
		})
		w.policies.Put(w.assistent, authz.AgentPolicy{BillingMode: domain.BillingPersonal, CredentialHolderID: w.owner})
		w.expectRejected(t, before, err, errs.CodeI1Violation, "agent.credential_holder.set", ref("agent", w.assistent), w.owner)
	},
	"agent.enable": func(t *testing.T, w *pgWorld) {
		// A disabled personal Lotse may stand in violation; enabling it may not.
		w.policies.Put(w.lotse, authz.AgentPolicy{BillingMode: domain.BillingPersonal, CredentialHolderID: w.owner})
		w.policies.SetActive(w.lotse, false)
		before := w.snapshot(t)
		err := w.declared("agent.enable", ref("agent", w.lotse), []ids.ID{w.lotse}, func(context.Context) error {
			w.policies.SetActive(w.lotse, true)
			return nil
		})
		w.policies.SetActive(w.lotse, false)
		w.expectRejected(t, before, err, errs.CodeI1Violation, "agent.enable", ref("agent", w.lotse), w.owner)
	},
	"user.enable": func(t *testing.T, w *pgWorld) {
		// Robin is a disabled second owner: valid while disabled.
		w.makeOwner(t, w.robin)
		w.sql(t, `UPDATE principals SET status = 'disabled' WHERE id = $1`, w.robin.String())
		before := w.snapshot(t)
		err := w.declared("user.enable", ref("user", w.robin), []ids.ID{w.robin}, func(ctx context.Context) error {
			return txSQL(ctx, `UPDATE principals SET status = 'active' WHERE id = $1`, w.robin.String())
		})
		w.expectRejected(t, before, err, errs.CodeI1Violation, "user.enable", ref("user", w.robin), w.owner)
	},
	// The shrinking actions cannot break a valid world: each starts from a world made invalid in
	// SQL, a change that does not repair it is rejected (the check runs), one that repairs commits.
	"grant.revoke": func(t *testing.T, w *pgWorld) {
		bad := w.rawGrant(t, "user", w.sam, w.notizen, time.Time{})
		before := w.snapshot(t)
		_, err := w.admin.RevokeGrant(context.Background(), w.user(w.owner), w.g2, "test")
		w.expectRejected(t, before, err, errs.CodeI1Violation, "grant.revoke", ref("grant", w.g2), w.owner)
		_, err = w.admin.RevokeGrant(context.Background(), w.user(w.owner), bad, "repair")
		mustOK(t, err)
	},
	"grant.expire": func(t *testing.T, w *pgWorld) {
		bad := w.rawGrant(t, "user", w.sam, w.notizen, w.clk.Now().Add(time.Hour))
		before := w.snapshot(t)
		err := w.change(context.Background(), "grant.expire", ref("grant", bad), []ids.ID{w.sam}, noop)
		w.expectRejected(t, before, err, errs.CodeI1Violation, "grant.expire", ref("grant", bad), ids.ID{})
		w.clk.Advance(2 * time.Hour)
		mustOK(t, w.change(context.Background(), "grant.expire", ref("grant", bad), []ids.ID{w.sam}, noop))
	},
	"group.member.remove": func(t *testing.T, w *pgWorld) {
		bad := w.rawGroup(t, "Bad", w.sam, time.Time{})
		w.rawGrant(t, "group", bad, w.notizen, time.Time{})
		before := w.snapshot(t)
		err := w.admin.RemoveGroupMember(context.Background(), w.user(w.owner), w.leser, w.sam)
		w.expectRejected(t, before, err, errs.CodeI1Violation, "group.member.remove", ref("group", w.leser), w.owner)
		mustOK(t, w.admin.RemoveGroupMember(context.Background(), w.user(w.owner), bad, w.sam))
	},
	"group.member.expire": func(t *testing.T, w *pgWorld) {
		bad := w.rawGroup(t, "Bad", w.sam, w.clk.Now().Add(time.Hour))
		w.rawGrant(t, "group", bad, w.notizen, time.Time{})
		before := w.snapshot(t)
		err := w.change(context.Background(), "group.member.expire", ref("group", bad), []ids.ID{w.sam}, noop)
		w.expectRejected(t, before, err, errs.CodeI1Violation, "group.member.expire", ref("group", bad), ids.ID{})
		w.clk.Advance(2 * time.Hour)
		mustOK(t, w.change(context.Background(), "group.member.expire", ref("group", bad), []ids.ID{w.sam}, noop))
	},
	"group.delete": func(t *testing.T, w *pgWorld) {
		bad := w.rawGroup(t, "Bad", w.sam, time.Time{})
		w.rawGrant(t, "group", bad, w.notizen, time.Time{})
		before := w.snapshot(t)
		err := w.admin.DeleteGroup(context.Background(), w.user(w.owner), w.leser)
		w.expectRejected(t, before, err, errs.CodeI1Violation, "group.delete", ref("group", w.leser), w.owner)
		mustOK(t, w.admin.DeleteGroup(context.Background(), w.user(w.owner), bad))
	},
	"owner.remove": func(t *testing.T, w *pgWorld) {
		w.makeOwner(t, w.robin)
		before := w.snapshot(t)
		err := w.declared("owner.remove", ref("user", w.robin), []ids.ID{w.robin}, noop)
		w.expectRejected(t, before, err, errs.CodeI1Violation, "owner.remove", ref("user", w.robin), w.owner)
		mustOK(t, w.declared("owner.remove", ref("user", w.robin), []ids.ID{w.robin}, func(ctx context.Context) error {
			return txSQL(ctx, `UPDATE owners SET revoked_at = now(), revoked_by = $2 WHERE user_id = $1 AND revoked_at IS NULL`, w.robin.String(), w.owner.String())
		}))
	},
	"user.disable": func(t *testing.T, w *pgWorld) {
		w.makeOwner(t, w.robin)
		before := w.snapshot(t)
		err := w.declared("user.disable", ref("user", w.sam), []ids.ID{w.sam}, func(ctx context.Context) error {
			return txSQL(ctx, `UPDATE principals SET status = 'disabled' WHERE id = $1`, w.sam.String())
		})
		w.expectRejected(t, before, err, errs.CodeI1Violation, "user.disable", ref("user", w.sam), w.owner)
		mustOK(t, w.declared("user.disable", ref("user", w.robin), []ids.ID{w.robin}, func(ctx context.Context) error {
			return txSQL(ctx, `UPDATE principals SET status = 'disabled' WHERE id = $1`, w.robin.String())
		}))
	},
}

// TestAZ_P7_I1PerAction runs one case per action of 05 rule 1.6 (brief M1-29 test 5); every
// rejection leaves nothing but one deny row (test 6).
func TestAZ_P7_I1PerAction(t *testing.T) {
	t.Parallel()
	for action, c := range p7Cases {
		t.Run(action, func(t *testing.T) {
			t.Parallel()
			c(t, newPGWorld(t))
		})
	}
}

// rule16Action finds the action ids of the I1 bullet of 05 rule 1.6.
var rule16Action = regexp.MustCompile("`([a-z_]+(?:\\.[a-z_]+)+)`")

// TestAZ_P7_CoversRule16 reads rule 1.6 from 05 and fails for an action there without a case in
// p7Cases, and for a case without an action there.
func TestAZ_P7_CoversRule16(t *testing.T) {
	t.Parallel()
	f, err := os.Open(filepath.Join("..", "..", "..", "..", "..", "docs", "spec", "05-actions.md"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	var text strings.Builder
	in := false
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()
		if strings.Contains(line, "I1 is checked transactionally on") {
			in = true
		}
		if in {
			text.WriteString(line + "\n")
			if strings.Contains(line, "check in [03-data-model.md]") {
				break
			}
		}
	}
	mustOK(t, sc.Err())
	var actions []string
	for _, m := range rule16Action.FindAllStringSubmatch(text.String(), -1) {
		actions = append(actions, m[1])
	}
	if len(actions) < 10 || !slices.Contains(actions, "group.member.sync") {
		t.Fatalf("read %v from rule 1.6; the rule moved?", actions)
	}
	for _, a := range actions {
		if _, ok := p7Cases[a]; !ok {
			t.Errorf("05 rule 1.6 names %s, AZ-P7 has no case for it", a)
		}
	}
	for a := range p7Cases {
		if !slices.Contains(actions, a) {
			t.Errorf("AZ-P7 has a case for %s, which 05 rule 1.6 does not name", a)
		}
	}
}

// anyStrings converts a decoded JSON array of strings.
func anyStrings(v any) []string {
	list, _ := v.([]any)
	out := make([]string, 0, len(list))
	for _, e := range list {
		s, _ := e.(string)
		out = append(out, s)
	}
	return out
}
