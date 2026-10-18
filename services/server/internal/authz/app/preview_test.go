// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/authz"
	"github.com/OWNER/bothy/services/server/internal/authz/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// preview runs access.preview of cmd as caller.
func (w *pgWorld) preview(t *testing.T, caller actor.Actor, cmd authz.CreateGrantCmd) authz.GrantPreview {
	t.Helper()
	if cmd.Reason == "" {
		cmd.Reason = "preview"
	}
	p, err := w.admin.PreviewGrant(ctxOf(caller), caller, cmd)
	mustOK(t, err)
	if p.Effects == nil || p.InvariantViolations == nil || p.Hints == nil || p.Text == "" {
		t.Fatalf("incomplete preview %+v", p)
	}
	return p
}

// hintsOf returns the hints of p with code.
func hintsOf(p authz.GrantPreview, code string) []authz.PreviewHint {
	var out []authz.PreviewHint
	for _, h := range p.Hints {
		if h.Code == code {
			out = append(out, h)
		}
	}
	return out
}

// name sets the display name of a principal.
func (w *pgWorld) name(t *testing.T, id ids.ID, display string) {
	t.Helper()
	w.sql(t, `UPDATE principals SET display_name = $2 WHERE id = $1`, id.String(), display)
}

// example07 builds the world of the example of 07 section 9.3 on top of the matrix world: Sam
// without grants, Marketing with Kampagnen and Intern, both operated by Texter (organization,
// chat) as default operator of Marketing, and the owner with locale.
func (w *pgWorld) example07(t *testing.T, locale string) (intern ids.ID) {
	t.Helper()
	_, err := w.admin.RevokeGrant(ctxOf(w.user(w.owner)), w.user(w.owner), w.g2, "example")
	mustOK(t, err)
	_, err = w.admin.RevokeGrant(ctxOf(w.user(w.owner)), w.user(w.owner), w.g4, "example")
	mustOK(t, err)
	w.sql(t, `UPDATE users SET locale = $2, time_zone = 'UTC' WHERE principal_id = $1`, w.owner.String(), locale)
	w.name(t, w.sam, "Sam")
	w.name(t, w.texter, "Texter")
	w.policies.Put(w.texter, authz.AgentPolicy{BillingMode: domain.BillingOrganization, ToolClass: domain.ToolChat})
	w.structure.PutSpaceDetail(authz.SpaceDetail{ID: w.marketing, Name: "Marketing", SortKey: "a1", DefaultOperatorID: w.texter, Version: 1, CreatedAt: pgStart})
	intern = ids.New()
	w.structure.PutChannel(authz.ChannelFacts{ID: intern, SpaceID: w.marketing, Kind: authz.ChannelText, EffectiveOperatorID: w.texter})
	w.structure.PutChannelDetail(authz.ChannelDetail{ID: w.kampagnen, Name: "Kampagnen", OperatorMode: authz.OperatorInherit})
	w.structure.PutChannelDetail(authz.ChannelDetail{ID: intern, Name: "Intern", OperatorMode: authz.OperatorInherit})
	return intern
}

// samKampagnen is the grant of the example of 07 section 9.3.
func (w *pgWorld) samKampagnen() authz.CreateGrantCmd {
	return authz.CreateGrantCmd{SubjectKind: "user", SubjectID: w.sam, TargetKind: "channel", TargetID: w.kampagnen, Level: "trigger",
		ExpiresAt: time.Date(2026, 12, 31, 12, 0, 0, 0, time.UTC), Reason: "Kampagne"}
}

// TestPreviewExample07EndToEnd: the use case renders the example of 07 section 9.3 byte for byte
// in German and English from the real names, and reports H1 for Texter and Marketing/Intern.
func TestPreviewExample07EndToEnd(t *testing.T) {
	t.Parallel()
	for _, lang := range []string{"de", "en"} {
		t.Run(lang, func(t *testing.T) {
			t.Parallel()
			w := newPGWorld(t)
			intern := w.example07(t, lang)
			p := w.preview(t, w.user(w.owner), w.samKampagnen())
			want, err := os.ReadFile(filepath.Clean(filepath.Join("..", "domain", "previewtext", "testdata", "example_07_"+lang+".golden")))
			mustOK(t, err)
			if p.Text != strings.TrimSuffix(string(want), "\n") || p.TextLocale != lang || !p.Allowed {
				t.Fatalf("preview (%s, allowed %v):\n%s", p.TextLocale, p.Allowed, p.Text)
			}
			h := hintsOf(p, "read_horizon")
			if len(h) != 1 || h[0].AgentID != w.texter || h[0].ChannelID != intern || h[0].SpaceID != w.marketing || h[0].PrincipalID != w.sam {
				t.Fatalf("read horizon %+v", h)
			}
			var sam []authz.AccessEffect
			for _, e := range p.Effects {
				if e.Principal.ID == w.sam {
					sam = append(sam, e)
				}
			}
			if len(sam) != 2 {
				t.Fatalf("effects of Sam %+v", p.Effects)
			}
			for _, e := range sam {
				switch e.TargetKind {
				case "channel":
					if e.TargetID != w.kampagnen || e.BeforeLevel != "none" || e.AfterLevel != "trigger" || len(e.CapabilitiesAdded) != 0 {
						t.Errorf("channel effect %+v", e)
					}
				case "space":
					if e.TargetID != w.marketing || e.BeforeLevel != "none" || e.AfterLevel != "read" {
						t.Errorf("space effect %+v", e)
					}
				}
			}
		})
	}
}

// TestPreviewLeavesNoGrant: after a preview the grant does not exist and the rights version is
// unchanged (the integration test of adapters/httpapi checks every table and counter).
func TestPreviewLeavesNoGrant(t *testing.T) {
	t.Parallel()
	w := newPGWorld(t)
	before := w.snapshot(t)
	w.preview(t, w.user(w.owner), authz.CreateGrantCmd{SubjectKind: "user", SubjectID: w.robin, TargetKind: "space", TargetID: w.support, Level: "read"})
	mustOK(t, w.rec.Aggregator().Flush(t.Context()))
	if after := w.snapshot(t); after != before {
		t.Fatalf("the preview left something behind:\nbefore %+v\nafter  %+v", before, after)
	}
	if len(w.published()) != 0 {
		t.Fatal("the preview published access.changed")
	}
}

// TestPreviewHintToolClassHost: Runner (host) operates Engineering/Werkzeuge; the preview of Alex
// with trigger there has the hint, and Runner is not in the abilities line.
func TestPreviewHintToolClassHost(t *testing.T) {
	t.Parallel()
	w := newPGWorld(t)
	w.name(t, w.runner, "Runner")
	p := w.preview(t, w.user(w.owner), authz.CreateGrantCmd{SubjectKind: "user", SubjectID: w.alex, TargetKind: "channel", TargetID: w.werkzeuge, Level: "trigger"})
	h := hintsOf(p, "trigger_blocked_tool_class")
	if len(h) != 1 || h[0].AgentID != w.runner || h[0].PrincipalID != w.alex {
		t.Fatalf("hints %+v", p.Hints)
	}
	lines := strings.Split(p.Text, "\n")
	if strings.Contains(lines[1], "Runner") || !strings.Contains(p.Text, "Hinweis: Runner hat die Werkzeugklasse Host") {
		t.Fatalf("text:\n%s", p.Text)
	}
	// The owner is not blocked by SR2: no hint, and Runner is in the abilities line.
	ownerP := w.preview(t, w.user(w.owner), authz.CreateGrantCmd{SubjectKind: "user", SubjectID: w.owner, TargetKind: "channel", TargetID: w.werkzeuge, Level: "trigger"})
	if len(hintsOf(ownerP, "trigger_blocked_tool_class")) != 0 || !strings.Contains(strings.Split(ownerP.Text, "\n")[1], "Runner") {
		t.Fatalf("owner preview %+v", ownerP)
	}
}

// TestPreviewHintGroupReach: a group with three valid and one expired membership reaches three.
func TestPreviewHintGroupReach(t *testing.T) {
	t.Parallel()
	w := newPGWorld(t)
	ctx, owner := ctxOf(w.user(w.owner)), w.user(w.owner)
	g, err := w.admin.CreateGroup(ctx, owner, authz.CreateGroupCmd{Name: "Team"})
	mustOK(t, err)
	for _, u := range []ids.ID{w.alex, w.sam, w.robin} {
		_, err := w.admin.AddGroupMember(ctx, owner, g.ID, authz.AddMemberCmd{UserID: u})
		mustOK(t, err)
	}
	_, err = w.admin.AddGroupMember(ctx, owner, g.ID, authz.AddMemberCmd{UserID: w.owner, ExpiresAt: w.clk.Now().Add(time.Hour)})
	mustOK(t, err)
	w.clk.Set(w.clk.Now().Add(2 * time.Hour))
	p := w.preview(t, w.user(w.owner), authz.CreateGrantCmd{SubjectKind: "group", SubjectID: g.ID, TargetKind: "space", TargetID: w.support, Level: "read"})
	h := hintsOf(p, "group_reach")
	if len(h) != 1 || h[0].Count != 3 || !h[0].HasCount || h[0].GroupID != g.ID {
		t.Fatalf("group reach %+v", p.Hints)
	}
	if !strings.Contains(p.Text, "Hinweis: Die Gruppe Team erreicht 3 Menschen.") {
		t.Fatalf("text:\n%s", p.Text)
	}
}

// TestPreviewHintExpiryMissing: a person without expiry has the hint, a group never.
func TestPreviewHintExpiryMissing(t *testing.T) {
	t.Parallel()
	w := newPGWorld(t)
	p := w.preview(t, w.user(w.owner), authz.CreateGrantCmd{SubjectKind: "user", SubjectID: w.robin, TargetKind: "space", TargetID: w.support, Level: "read"})
	if h := hintsOf(p, "expiry_missing"); len(h) != 1 || h[0].PrincipalID != w.robin {
		t.Fatalf("hints %+v", p.Hints)
	}
	p = w.preview(t, w.user(w.owner), authz.CreateGrantCmd{SubjectKind: "user", SubjectID: w.robin, TargetKind: "space", TargetID: w.support,
		Level: "read", ExpiresAt: w.clk.Now().Add(time.Hour)})
	if len(hintsOf(p, "expiry_missing")) != 0 {
		t.Fatalf("hints %+v", p.Hints)
	}
	p = w.preview(t, w.user(w.owner), authz.CreateGrantCmd{SubjectKind: "group", SubjectID: w.leser, TargetKind: "space", TargetID: w.support, Level: "read"})
	if len(hintsOf(p, "expiry_missing")) != 0 {
		t.Fatalf("group hints %+v", p.Hints)
	}
}

// TestPreviewAZM21_I1ViolationIsReportedNotVetoed: the owner previews Sam with trigger on
// Research/Notizen while Assistent is personal: allowed false with one violation, no error.
func TestPreviewAZM21_I1ViolationIsReportedNotVetoed(t *testing.T) {
	t.Parallel()
	w := newPGWorld(t)
	w.name(t, w.assistent, "Assistent")
	w.name(t, w.owner, "Owner")
	p := w.preview(t, w.user(w.owner), authz.CreateGrantCmd{SubjectKind: "user", SubjectID: w.sam, TargetKind: "channel", TargetID: w.notizen, Level: "trigger"})
	if p.Allowed || len(p.InvariantViolations) != 1 {
		t.Fatalf("preview %+v", p)
	}
	v := p.InvariantViolations[0]
	if v.Code != "i1_violation" || v.AgentID != w.assistent || !slices.Equal(v.PrincipalIDs, []ids.ID{w.sam}) ||
		v.CredentialHolderID != w.owner || v.OrganizationReady || v.Hint == "" {
		t.Fatalf("violation %+v", v)
	}
	// language-check: off
	if !strings.Contains(p.Text, "Abgelehnt: Assistent läuft auf dem persönlichen Abo von Owner. Zuerst auf Organisation umstellen.") ||
		strings.Contains(strings.Split(p.Text, "\n")[1], "Assistent") {
		t.Fatalf("text:\n%s", p.Text)
	}
	// language-check: on
	w.policies.Put(w.assistent, authz.AgentPolicy{BillingMode: domain.BillingPersonal, CredentialHolderID: w.owner, ToolClass: domain.ToolScoped,
		BillingModesSupported: []string{"personal", "organization"}})
	p = w.preview(t, w.user(w.owner), authz.CreateGrantCmd{SubjectKind: "user", SubjectID: w.sam, TargetKind: "channel", TargetID: w.notizen, Level: "trigger"})
	if !p.InvariantViolations[0].OrganizationReady {
		t.Fatalf("organization_ready %+v", p.InvariantViolations)
	}
}

// TestPreviewAZM31_AgentTriggerIsDenialNotError: an agent with trigger is allowed false with
// subject_kind_not_allowed, also for the owner.
func TestPreviewAZM31_AgentTriggerIsDenialNotError(t *testing.T) {
	t.Parallel()
	w := newPGWorld(t)
	p := w.preview(t, w.user(w.owner), authz.CreateGrantCmd{SubjectKind: "agent", SubjectID: w.lotse, TargetKind: "space", TargetID: w.support, Level: "trigger"})
	if p.Allowed || p.DenialReason != "subject_kind_not_allowed" || len(p.Effects) != 0 {
		t.Fatalf("preview %+v", p)
	}
	if !strings.Contains(p.Text, "Abgelehnt: Agenten und Apps bekommen Freigaben nur vom Owner") {
		t.Fatalf("text:\n%s", p.Text)
	}
}

// TestPreviewAZM37_DelegationLevel: Alex with write and manage on Support previews Sam with
// trigger: delegation_exceeds_own_level.
func TestPreviewAZM37_DelegationLevel(t *testing.T) {
	t.Parallel()
	w := newPGWorld(t)
	w.withManager(t, "write")
	p := w.preview(t, w.user(w.alex), authz.CreateGrantCmd{SubjectKind: "user", SubjectID: w.sam, TargetKind: "channel", TargetID: w.allgemein, Level: "trigger"})
	if p.Allowed || p.DenialReason != "delegation_exceeds_own_level" {
		t.Fatalf("preview %+v", p)
	}
}

// TestPreviewAZM38_DelegationCapabilities: Alex with trigger and manage but without export previews
// Sam with read and export: delegation_exceeds_own_capabilities.
func TestPreviewAZM38_DelegationCapabilities(t *testing.T) {
	t.Parallel()
	w := newPGWorld(t)
	w.withManager(t, "trigger")
	p := w.preview(t, w.user(w.alex), authz.CreateGrantCmd{SubjectKind: "user", SubjectID: w.sam, TargetKind: "channel", TargetID: w.allgemein,
		Level: "read", Capabilities: []string{"export"}})
	if p.Allowed || p.DenialReason != "delegation_exceeds_own_capabilities" {
		t.Fatalf("preview %+v", p)
	}
	ok := w.preview(t, w.user(w.alex), authz.CreateGrantCmd{SubjectKind: "user", SubjectID: w.sam, TargetKind: "channel", TargetID: w.allgemein,
		Level: "read", ExpiresAt: w.clk.Now().Add(time.Hour)})
	if !ok.Allowed || ok.DenialReason != "" {
		t.Fatalf("within the limits %+v", ok)
	}
}

// TestPreviewReadHorizonHidesChannelsOfTheCaller: a manager who cannot read Marketing/Intern
// never sees it named in a hint (05 rule 2); the owner does.
func TestPreviewReadHorizonHidesChannelsOfTheCaller(t *testing.T) {
	t.Parallel()
	w := newPGWorld(t)
	intern := w.example07(t, "de")
	w.grantRow(t, domain.Subject{Kind: domain.SubjectUser, PrincipalID: w.alex}, domain.Target{Kind: domain.TargetChannel, ChannelID: w.kampagnen},
		domain.LevelTrigger, time.Time{}, domain.CapManage)
	alex := w.user(w.alex)
	p := w.preview(t, alex, w.samKampagnen())
	if len(hintsOf(p, "read_horizon")) != 0 || strings.Contains(p.Text, "Intern") || strings.Contains(p.Text, intern.String()) {
		t.Fatalf("manager preview names an invisible channel:\n%s\n%+v", p.Text, p.Hints)
	}
	if len(hintsOf(w.preview(t, w.user(w.owner), w.samKampagnen()), "read_horizon")) != 1 {
		t.Fatal("the owner sees the hint")
	}
}

// TestPreviewEffectsOnlyOfTheProposedGrant (red team M1-49): a due expiry elsewhere that the dry
// run folds in is no effect of the preview, and a manager sees no principal or channel outside
// the grant.
func TestPreviewEffectsOnlyOfTheProposedGrant(t *testing.T) {
	t.Parallel()
	w := newPGWorld(t)
	w.grantRow(t, domain.Subject{Kind: domain.SubjectUser, PrincipalID: w.robin}, domain.Target{Kind: domain.TargetChannel, ChannelID: w.werkzeuge},
		domain.LevelWrite, w.clk.Now().Add(time.Hour))
	w.withManager(t, "write")
	w.clk.Set(w.clk.Now().Add(2 * time.Hour))
	p := w.preview(t, w.user(w.alex), authz.CreateGrantCmd{SubjectKind: "user", SubjectID: w.sam, TargetKind: "channel", TargetID: w.eskalation,
		Level: "read", ExpiresAt: w.clk.Now().Add(time.Hour)})
	for _, e := range p.Effects {
		if e.Principal.ID != w.sam || e.TargetID == w.werkzeuge {
			t.Fatalf("foreign effect %+v", e)
		}
	}
	if len(p.Effects) == 0 {
		t.Fatal("the grant's own effect is missing")
	}
}
