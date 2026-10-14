// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"context"
	"fmt"
	"os"
	"slices"
	"strconv"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/authz"
	"github.com/OWNER/bothy/services/server/internal/authz/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// explain runs access.explain as the owner.
func (w *pgWorld) explain(t *testing.T, kind string, id ids.ID) map[ids.ID]authz.AccessEntry {
	t.Helper()
	x, err := w.admin.ExplainAccess(ctxOf(w.user(w.owner)), w.user(w.owner), authz.AccessTargetRef{Kind: kind, ID: id})
	mustOK(t, err)
	out := map[ids.ID]authz.AccessEntry{}
	for _, e := range x.Entries {
		out[e.Principal.ID] = e
	}
	if len(x.Entries) > 0 && x.Entries[0].Principal.ID != w.owner {
		t.Fatalf("the owner comes first: %+v", x.Entries[0])
	}
	return out
}

// sourceKinds returns the kinds of the sources of e.
func sourceKinds(e authz.AccessEntry) []string {
	var out []string
	for _, s := range e.Sources {
		out = append(out, s.Kind)
	}
	return out
}

// TestExplainSources: the explanation of Support/Allgemein (07 section 11.1) lists the owner, Alex
// inherited from G-1, Sam through the group Leser with the expiry of the membership and Lotse with
// G-5 inherited from Support and as operator; on the space Support itself G-5 is direct.
func TestExplainSources(t *testing.T) {
	t.Parallel()
	w := newPGWorld(t)
	got := w.explain(t, "channel", w.allgemein)
	if e := got[w.owner]; e.Level != "trigger" || !slices.Equal(sourceKinds(e), []string{"owner"}) || len(e.Capabilities) != 5 {
		t.Errorf("owner %+v", e)
	}
	alex := got[w.alex]
	if alex.Level != "trigger" || len(alex.Sources) != 1 || alex.Sources[0].Kind != "inherited" || alex.Sources[0].GrantID != w.g1 ||
		alex.Sources[0].FromTargetKind != "space" || alex.Sources[0].FromTargetID != w.support ||
		!slices.Equal(alex.Capabilities, []string{"approve_actions", "upload"}) {
		t.Errorf("alex %+v", alex)
	}
	sam := got[w.sam]
	if sam.Level != "read" || len(sam.Sources) != 1 || sam.Sources[0].Kind != "group" || sam.Sources[0].GroupID != w.leser ||
		sam.Sources[0].GrantID != w.g4 || !sam.Sources[0].MembershipExpiresAt.Equal(pgStart.Add(48*time.Hour)) {
		t.Errorf("sam %+v", sam)
	}
	lotse := got[w.lotse]
	if lotse.Level != "write" || !slices.Equal(sourceKinds(lotse), []string{"inherited", "operator"}) || lotse.Sources[0].GrantID != w.g5 ||
		lotse.Sources[1].AgentID != w.lotse || !slices.Equal(lotse.Capabilities, []string{"upload"}) {
		t.Errorf("lotse %+v", lotse)
	}
	if _, ok := got[w.robin]; ok {
		t.Errorf("robin has no access to Allgemein: %+v", got[w.robin])
	}
	if _, ok := got[w.tracker]; ok {
		t.Errorf("tracker has no access to Allgemein")
	}
	space := w.explain(t, "space", w.support)
	if e := space[w.lotse]; !slices.Equal(sourceKinds(e), []string{"direct"}) || e.Sources[0].GrantID != w.g5 || e.Sources[0].Reason != "setup" {
		t.Errorf("lotse on Support %+v", e)
	}
	if _, ok := space[w.sam]; ok {
		t.Errorf("a channel grant does not explain the space: %+v", space[w.sam])
	}
}

// TestExplainReadHorizonEntry: Texter reads Marketing/Intern by a grant and Sam has trigger on
// Marketing/Kampagnen only: the explanation of Intern holds Sam with level none and the source
// via_agent of Texter (hint H1, 07 section 6).
func TestExplainReadHorizonEntry(t *testing.T) {
	t.Parallel()
	w := newPGWorld(t)
	ctx := ctxOf(w.user(w.owner))
	_, err := w.admin.RevokeGrant(ctx, w.user(w.owner), w.g2, "narrow")
	mustOK(t, err)
	intern := ids.New()
	w.structure.PutChannel(authz.ChannelFacts{ID: intern, SpaceID: w.marketing, Kind: authz.ChannelText})
	w.grantRow(t, domain.Subject{Kind: domain.SubjectAgent, PrincipalID: w.texter}, domain.Target{Kind: domain.TargetChannel, ChannelID: intern}, domain.LevelRead, time.Time{})
	w.grantRow(t, domain.Subject{Kind: domain.SubjectUser, PrincipalID: w.sam}, domain.Target{Kind: domain.TargetChannel, ChannelID: w.kampagnen}, domain.LevelTrigger, time.Time{})
	got := w.explain(t, "channel", intern)
	sam := got[w.sam]
	if sam.Level != "none" || !slices.Equal(sourceKinds(sam), []string{"via_agent"}) || sam.Sources[0].AgentID != w.texter || len(sam.Capabilities) != 0 {
		t.Fatalf("sam %+v", sam)
	}
	if e := got[w.texter]; e.Level != "read" || !slices.Equal(sourceKinds(e), []string{"direct"}) {
		t.Fatalf("texter %+v", e)
	}
	space := w.explain(t, "space", w.marketing)
	if e := space[w.sam]; !domain.HasSource(srcs(e), domain.SourceViaAgent) || e.Level != "none" {
		t.Fatalf("sam on Marketing %+v", e)
	}
}

// srcs returns the domain kinds of the sources of e.
func srcs(e authz.AccessEntry) []domain.Source {
	out := make([]domain.Source, 0, len(e.Sources))
	for _, s := range e.Sources {
		out = append(out, domain.Source{Kind: domain.SourceKind(s.Kind)})
	}
	return out
}

// TestExplainGuardBlockedSR2: Alex has trigger on Engineering/Werkzeuge, whose operator Runner has
// the tool class host: the source guard_blocked with rule sr2 and agent Runner (07 SR2).
func TestExplainGuardBlockedSR2(t *testing.T) {
	t.Parallel()
	w := newPGWorld(t)
	w.grantRow(t, domain.Subject{Kind: domain.SubjectUser, PrincipalID: w.alex}, domain.Target{Kind: domain.TargetChannel, ChannelID: w.werkzeuge}, domain.LevelTrigger, time.Time{})
	alex := w.explain(t, "channel", w.werkzeuge)[w.alex]
	if alex.Level != "trigger" || !slices.Equal(sourceKinds(alex), []string{"direct", "guard_blocked"}) ||
		alex.Sources[1].Rule != "sr2" || alex.Sources[1].AgentID != w.runner {
		t.Fatalf("alex %+v", alex)
	}
}

// TestExplainInactivePrincipalMarked: a disabled Robin with G-3 appears with inactive and the level
// G-3 would give (07 section 9.1, SR7).
func TestExplainInactivePrincipalMarked(t *testing.T) {
	t.Parallel()
	w := newPGWorld(t)
	w.sql(t, `UPDATE principals SET status = 'disabled', disabled_at = now() WHERE id = $1`, w.robin.String())
	robin := w.explain(t, "channel", w.projektA)[w.robin]
	if !robin.Inactive || robin.Level != "trigger" || robin.Sources[0].GrantID != w.g3 {
		t.Fatalf("robin %+v", robin)
	}
	if owner := w.explain(t, "channel", w.projektA)[w.owner]; owner.Inactive {
		t.Fatal("the active owner is not inactive")
	}
}

// TestExplainCallerNeedsOwnerOrManage: read without owner or manage is 403, no read 404.
func TestExplainCallerNeedsOwnerOrManage(t *testing.T) {
	t.Parallel()
	w := newPGWorld(t)
	ctx := ctxOf(w.user(w.sam))
	_, err := w.admin.ExplainAccess(ctx, w.user(w.sam), authz.AccessTargetRef{Kind: "channel", ID: w.allgemein})
	if errs.KindOf(err) != errs.KindForbidden {
		t.Fatalf("read without manage: %v", err)
	}
	_, err = w.admin.ExplainAccess(ctx, w.user(w.sam), authz.AccessTargetRef{Kind: "channel", ID: w.notizen})
	if errs.KindOf(err) != errs.KindNotFound {
		t.Fatalf("no read: %v", err)
	}
}

// TestExplainMatchesAuthorizeInRandomWorlds (AZ-P8 generator): for every target of random worlds,
// every entry of an active principal has the level and capabilities authorize derives, and every
// active principal that authorize lets read the target appears.
func TestExplainMatchesAuthorizeInRandomWorlds(t *testing.T) {
	seed := uint64(time.Now().UnixNano())
	if s := os.Getenv("BOTHY_TEST_SEED"); s != "" { //nolint:forbidigo // test input (seed), not server configuration
		n, err := strconv.ParseUint(s, 10, 64)
		mustOK(t, err)
		seed = n
	}
	ctx := context.Background()
	compared := 0
	for wi := range 12 {
		ws := seed + uint64(wi)
		m := newRefWorld(t, ws)
		for step := range 25 {
			m.mutate()
			var targets []authorize.Resource
			for _, s := range m.spaces {
				targets = append(targets, authorize.Resource{Kind: authorize.ResourceSpace, ID: s})
			}
			for _, c := range m.channels {
				if !c.dm {
					targets = append(targets, authorize.Resource{Kind: authorize.ResourceChannel, ID: c.id})
				}
			}
			for _, tg := range targets {
				entries, err := m.az.ExplainAt(ctx, tg.Kind, tg.ID, m.now)
				if err != nil {
					t.Fatalf("seed %d world %d step %d: %v", ws, wi, step, err)
				}
				byID := map[ids.ID]authz.AccessEntry{}
				for _, e := range entries {
					byID[e.Principal.ID] = e
				}
				for _, p := range m.principals {
					want, ok, err := m.az.RightsOn(ctx, m.actorOf(p), tg.Kind, tg.ID)
					mustOK(t, err)
					e, listed := byID[p.id]
					where := fmt.Sprintf("seed %d (BOTHY_TEST_SEED=%d) world %d step %d %s %s %s", seed, ws, wi, step, p.kind, tg.Kind, tg.ID)
					if !ok {
						if listed && !e.Inactive {
							t.Fatalf("%s: refused principal listed as active: %+v", where, e)
						}
						continue
					}
					if want.Level >= domain.LevelRead && !listed {
						t.Fatalf("%s: reader missing (level %s)", where, want.Level)
					}
					if !listed {
						continue
					}
					compared++
					if e.Level != levelName(want.Level) || !slices.Equal(e.Capabilities, want.Caps.Names()) {
						t.Fatalf("%s: entry %s %v, authorize %s %v", where, e.Level, e.Capabilities, want.Level, want.Caps.Names())
					}
				}
			}
		}
	}
	t.Logf("seed %d, %d entries compared", seed, compared)
}

// levelName is the LevelOrNone of l.
func levelName(l domain.Level) string {
	if l == domain.LevelNone {
		return "none"
	}
	return l.String()
}
