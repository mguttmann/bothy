// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"context"
	"log/slog"
	"slices"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/authz"
	"github.com/OWNER/bothy/services/server/internal/authz/adapters/fake"
	"github.com/OWNER/bothy/services/server/internal/authz/app"
	"github.com/OWNER/bothy/services/server/internal/authz/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// hostWorld is the world of 07 section 11.1 with the hosts of M2-39: R1 (holder Alex), R2 and R3
// (no holder); Sam holds terminal on R2 until samR2Expiry and through the group Leser on R3.
type hostWorld struct {
	*world
	hosts       *fake.Hosts
	r1, r2, r3  ids.ID
	samR2Expiry time.Time
}

// newHostWorld builds the world and rebuilds its Authorizer with the host source.
func newHostWorld(t *testing.T) *hostWorld {
	t.Helper()
	w := &hostWorld{world: newWorld(t), hosts: fake.NewHosts(), r1: ids.New(), r2: ids.New(), r3: ids.New()}
	w.hosts.Put(authz.HostFacts{ID: w.r1, State: "active", AssignedUserID: w.alex, LLMCredentials: "none"})
	w.hosts.Put(authz.HostFacts{ID: w.r2, State: "active", LLMCredentials: "none"})
	w.hosts.Put(authz.HostFacts{ID: w.r3, State: "active", LLMCredentials: "none"})
	w.samR2Expiry = worldStart.Add(24 * time.Hour)
	terminal := []domain.Capability{domain.CapTerminal}
	w.grant(t, domain.Subject{Kind: domain.SubjectUser, PrincipalID: w.sam}, domain.Target{Kind: domain.TargetHost, HostID: w.r2},
		domain.LevelNone, w.samR2Expiry, terminal...)
	w.grant(t, domain.Subject{Kind: domain.SubjectGroup, GroupID: w.leser}, domain.Target{Kind: domain.TargetHost, HostID: w.r3},
		domain.LevelNone, time.Time{}, terminal...)
	az, err := app.NewAuthorizer(w.cfg, app.AuthorizerDeps{
		Catalog: app.NewCatalog(slog.New(slog.NewTextHandler(w.logs, nil))), Store: w.store, Principals: w.principals,
		Credentials: w.creds, Structure: w.structure, Policies: w.policies, Hosts: w.hosts, Versions: w.versions, Recorder: w.rec,
		Clock: w.clk, Log: slog.New(slog.NewTextHandler(w.logs, nil)),
	})
	if err != nil {
		t.Fatal(err)
	}
	w.az = az
	return w
}

// host is the resource of host id.
func host(id ids.ID) authorize.Resource {
	return authorize.Resource{Kind: authorize.ResourceHost, ID: id}
}

// personal returns p with a personal API token instead of its session (SR5).
func (w *hostWorld) personal(p ids.ID) actor.Actor {
	tok := ids.New()
	w.creds.PutToken(tok, fake.Token{PrincipalID: p})
	return actor.Actor{ID: p, Kind: actor.KindUser, TokenID: tok}
}

// TestHostRequirement (M2-39, 05 section 4.14, 07 section 2.12, TM-T17 server part): host.read
// for owner, holder and grant holders, 404 for everyone else and at the expiry of a grant;
// agents, apps and personal tokens are refused with subject_kind_not_allowed for an existing and
// a random host alike; host.manage is owner only with a fresh step-up.
func TestHostRequirement(t *testing.T) {
	t.Parallel()
	w := newHostWorld(t)
	read, manage := authorize.ActionHostRead, authorize.ActionHostManage
	nf, fb := authorize.OutcomeNotFound, authorize.OutcomeForbidden
	w.allow(t, w.user(w.owner), read, host(w.r1))
	w.allow(t, w.user(w.owner), read, host(w.r3))
	w.allow(t, w.user(w.alex), read, host(w.r1))
	w.allow(t, w.user(w.sam), read, host(w.r2))
	w.allow(t, w.user(w.sam), read, host(w.r3))
	w.expect(t, w.user(w.alex), read, host(w.r2), nf, "no_grant")
	w.expect(t, w.user(w.sam), read, host(w.r1), nf, "no_grant")
	w.expect(t, w.user(w.owner), read, host(ids.New()), nf, "no_grant")
	for name, p := range map[string]actor.Actor{
		"agent": w.agent(w.lotse), "app": w.app(), "personal token": w.personal(w.alex), "owner with a personal token": w.personal(w.owner),
	} {
		for _, act := range []authorize.Action{read, manage} {
			for _, r := range []authorize.Resource{host(w.r1), host(ids.New())} {
				d := w.decide(t, context.Background(), p, act, r)
				if d.Outcome != fb || d.Reason != "subject_kind_not_allowed" {
					t.Errorf("%s %s: %s/%s, want forbidden/subject_kind_not_allowed", name, act, d.Outcome, d.Reason)
				}
			}
		}
	}
	w.allow(t, w.user(w.owner), manage, host(w.r1))
	w.allow(t, w.user(w.owner), manage, authorize.Resource{Kind: authorize.ResourceInstance})
	w.expect(t, w.user(w.alex), manage, host(w.r1), fb, "owner_only")
	w.expect(t, w.user(w.sam), manage, host(w.r2), fb, "owner_only")
	w.expect(t, w.user(w.sam), manage, host(w.r1), nf, "no_grant")
	w.expect(t, w.user(w.alex), manage, authorize.Resource{Kind: authorize.ResourceInstance}, fb, "owner_only")
	stale := w.user(w.owner)
	stale.StepUpAt = w.clk.Now().Add(-w.cfg.StepUpMaxAge - time.Second)
	w.expect(t, stale, manage, host(w.r1), authorize.OutcomeStepUpRequired, "step_up_required")
	// At exactly the expiry of the grant on R2 Sam no longer sees it (07 section 2.5).
	w.clk.Set(w.samR2Expiry)
	w.expect(t, w.user(w.sam), read, host(w.r2), nf, "no_grant")
	w.allow(t, w.user(w.sam), read, host(w.r3))
}

// TestAZM60_InstanceGrantGivesNoHost (AZ-M60, R-AZ-24): a grant on the instance with trigger and
// every capability gives no host: R3 (no holder, no grant for Alex) is not found like a random id,
// while R1 stays visible because Alex holds it.
func TestAZM60_InstanceGrantGivesNoHost(t *testing.T) {
	t.Parallel()
	w := newHostWorld(t)
	w.grant(t, domain.Subject{Kind: domain.SubjectUser, PrincipalID: w.alex}, domain.Target{Kind: domain.TargetInstance}, domain.LevelTrigger,
		time.Time{}, domain.CapApproveActions, domain.CapUpload, domain.CapExport, domain.CapManage)
	w.versions.Bump()
	w.expect(t, w.user(w.alex), authorize.ActionHostRead, host(w.r3), authorize.OutcomeNotFound, "no_grant")
	w.expect(t, w.user(w.alex), authorize.ActionHostRead, host(ids.New()), authorize.OutcomeNotFound, "no_grant")
	w.allow(t, w.user(w.alex), authorize.ActionHostRead, host(w.r1))
	sc, err := w.az.Scope(context.Background(), w.user(w.alex), authorize.ActionHostRead)
	if err != nil || sc.AllHosts || !slices.Equal(sc.HostIDs, []ids.ID{w.r1}) {
		t.Fatalf("Scope = %+v, %v; want only R1", sc, err)
	}
}

// TestScope_Hosts (M2-39, 07 section 7.4 rule 1): Scope(host.read) gives an owner every host,
// Alex the own host R1, Sam R2 and R3 in ascending order, intersected with "view as"; agents,
// apps and personal tokens get the gate's refusal.
func TestScope_Hosts(t *testing.T) {
	t.Parallel()
	w := newHostWorld(t)
	scope := func(p actor.Actor) authorize.Scope {
		t.Helper()
		sc, err := w.az.Scope(context.Background(), p, authorize.ActionHostRead)
		if err != nil {
			t.Fatalf("Scope: %v", err)
		}
		return sc
	}
	if sc := scope(w.user(w.owner)); !sc.AllHosts || len(sc.HostIDs) != 0 {
		t.Fatalf("owner scope %+v, want AllHosts", sc)
	}
	if sc := scope(w.user(w.alex)); sc.AllHosts || !slices.Equal(sc.HostIDs, []ids.ID{w.r1}) {
		t.Fatalf("Alex scope %+v, want [R1]", sc)
	}
	want := domain.SortIDs([]ids.ID{w.r2, w.r3})
	if sc := scope(w.user(w.sam)); sc.AllHosts || !slices.Equal(sc.HostIDs, want) {
		t.Fatalf("Sam scope %+v, want %v", sc, want)
	}
	viewAs := w.user(w.owner)
	viewAs.ViewAs = &actor.ViewAs{UserID: w.sam, StartedAt: w.clk.Now()}
	if sc := scope(viewAs); sc.AllHosts || !slices.Equal(sc.HostIDs, want) {
		t.Fatalf("owner viewing as Sam %+v, want %v", sc, want)
	}
	viewAs.ViewAs = &actor.ViewAs{UserID: w.alex, StartedAt: w.clk.Now()}
	if sc := scope(viewAs); sc.AllHosts || !slices.Equal(sc.HostIDs, []ids.ID{w.r1}) {
		t.Fatalf("owner viewing as Alex %+v, want [R1]", sc)
	}
	// One request memo for both lists: the channel lists never answer the host list.
	req := authorize.WithRequestScope(context.Background())
	if _, err := w.az.Scope(req, w.user(w.sam), authorize.ActionMessageRead); err != nil {
		t.Fatal(err)
	}
	if sc, err := w.az.Scope(req, w.user(w.sam), authorize.ActionHostRead); err != nil || !slices.Equal(sc.HostIDs, want) {
		t.Fatalf("host scope after a channel scope in one request %+v, %v", sc, err)
	}
	for name, p := range map[string]actor.Actor{"agent": w.agent(w.lotse), "app": w.app(), "personal token": w.personal(w.sam)} {
		_, err := w.az.Scope(context.Background(), p, authorize.ActionHostRead)
		if e, ok := errs.As(err); !ok || e.Code != errs.CodeForbidden || e.Reason != "subject_kind_not_allowed" {
			t.Errorf("%s: Scope error %v, want forbidden subject_kind_not_allowed", name, err)
		}
	}
}

// TestCreateGrant_HostTargetDecision (M2-39, 07 section 2.12 points 1 and 2): only an owner may
// grant terminal on a host; the holder gets owner_only, a stranger not_found.
func TestCreateGrant_HostTargetDecision(t *testing.T) {
	t.Parallel()
	w := newHostWorld(t)
	res := func(h ids.ID) authorize.Resource {
		r := host(h)
		r.Grant = &authorize.GrantFacts{SubjectKind: "user", Caps: []string{"terminal"}, TargetKind: "host", TargetHostID: h}
		return r
	}
	w.allow(t, w.user(w.owner), authorize.ActionGrantCreate, res(w.r1))
	w.expect(t, w.user(w.alex), authorize.ActionGrantCreate, res(w.r1), authorize.OutcomeForbidden, "owner_only")
	w.expect(t, w.user(w.alex), authorize.ActionGrantCreate, res(w.r2), authorize.OutcomeNotFound, "no_grant")
	w.expect(t, w.user(w.owner), authorize.ActionGrantCreate, res(ids.New()), authorize.OutcomeNotFound, "no_grant")
}

// TestHosts_WithoutSource (M2-39): an Authorizer without a host source knows no host: every host
// is not found and the host scope of a person is empty (fail closed).
func TestHosts_WithoutSource(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	w.expect(t, w.user(w.owner), authorize.ActionHostRead, host(ids.New()), authorize.OutcomeNotFound, "no_grant")
	sc, err := w.az.Scope(context.Background(), w.user(w.sam), authorize.ActionHostRead)
	if err != nil || sc.AllHosts || len(sc.HostIDs) != 0 {
		t.Fatalf("Scope = %+v, %v", sc, err)
	}
}
