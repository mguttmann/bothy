// SPDX-License-Identifier: Apache-2.0

package authzsource_test

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"slices"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/agents/adapters/authzsource"
	agentspg "github.com/OWNER/bothy/services/server/internal/agents/adapters/postgres"
	"github.com/OWNER/bothy/services/server/internal/authz"
	"github.com/OWNER/bothy/services/server/internal/authz/adapters/fake"
	identitypg "github.com/OWNER/bothy/services/server/internal/identity/adapters/postgres"
	identityapp "github.com/OWNER/bothy/services/server/internal/identity/app"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/pg"
	"github.com/OWNER/bothy/services/server/internal/platform/pg/pgtest"
)

func TestMain(m *testing.M) { os.Exit(pgtest.Main(m)) }

// agentFixture is one agent of the contract world.
type agentFixture struct {
	id       ids.ID
	policy   authz.AgentPolicy
	reported string
	active   bool
}

// policyWorld builds an authz.AgentPolicySource that knows the agents of a fixture list.
type policyWorld func(t *testing.T, holder ids.ID, list []agentFixture) authz.AgentPolicySource

// fakeWorld is the authz fake (adapters/fake.Policies).
func fakeWorld(_ *testing.T, _ ids.ID, list []agentFixture) authz.AgentPolicySource {
	p := fake.NewPolicies()
	for _, a := range list {
		pol := a.policy
		pol.ReportedToolClass = a.reported
		p.Put(a.id, pol)
		p.SetActive(a.id, a.active)
	}
	return p
}

// realWorld is the agents implementation over PostgreSQL: principals, the holder as a person and
// the agents rows written as fixtures.
func realWorld(t *testing.T, holder ids.ID, list []agentFixture) authz.AgentPolicySource {
	t.Helper()
	db := pgtest.New(t)
	owner := db.Pool(t, pg.RoleOwner)
	app := db.Pool(t, pg.RoleApp)
	exec := func(stmt string, args ...any) {
		t.Helper()
		if _, err := owner.Exec(context.Background(), stmt, args...); err != nil {
			t.Fatalf("%s: %v", stmt, err)
		}
	}
	exec(`INSERT INTO principals (id, kind, handle, display_name) VALUES ($1, 'user', 'alex', 'Alex')`, holder.String())
	exec(`INSERT INTO users (principal_id) VALUES ($1)`, holder.String())
	for i, a := range list {
		status := "active"
		if !a.active {
			status = "disabled"
		}
		exec(`INSERT INTO principals (id, kind, status, handle, display_name) VALUES ($1, 'agent', $2, $3, $3)`,
			a.id.String(), status, "agent"+string(rune('a'+i)))
		var h any
		if !a.policy.CredentialHolderID.IsZero() {
			h = a.policy.CredentialHolderID.String()
		}
		var reported any
		if a.reported != "" {
			reported = a.reported
		}
		details := "{}"
		if len(a.policy.BillingModesSupported) > 0 {
			b, err := json.Marshal(map[string]any{"billing_modes_supported": a.policy.BillingModesSupported})
			if err != nil {
				t.Fatal(err)
			}
			details = string(b)
		}
		exec(`INSERT INTO agents (principal_id, adapter_kind, billing_mode, credential_holder_id, tool_class, reported_tool_class, reported_details)
			VALUES ($1, 'hermes', $2, $3, $4, $5, $6)`, a.id.String(), a.policy.BillingMode, h, a.policy.ToolClass, reported, details)
	}
	repo := identitypg.NewRepository(app)
	return authzsource.New(agentspg.NewRepository(app), identityapp.NewDirectory(repo))
}

// TestAgentPolicySourceContract is the shared contract of authz.AgentPolicySource, run against the
// authz fake and the agents implementation: PersonalAgents lists exactly the active personal
// agents ascending (a disabled personal agent is not in the I1 list), AgentPolicy reports mode,
// holder, target and reported tool class (the effective class is their maximum) and ok false for
// an unknown agent, the billing modes the adapter reported (M1-49), no dev environment is ready before V2,
// and AgentIDs lists every agent, disabled ones too, ascending (M6-07).
func TestAgentPolicySourceContract(t *testing.T) {
	t.Parallel()
	for name, build := range map[string]policyWorld{"fake": fakeWorld, "agents": realWorld} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			holder := ids.New()
			list := []agentFixture{
				{id: ids.New(), policy: authz.AgentPolicy{BillingMode: "personal", CredentialHolderID: holder, ToolClass: "chat",
					BillingModesSupported: []string{"personal", "organization"}}, reported: "host", active: true},
				{id: ids.New(), policy: authz.AgentPolicy{BillingMode: "organization", ToolClass: "scoped"}, active: true},
				{id: ids.New(), policy: authz.AgentPolicy{BillingMode: "personal", CredentialHolderID: holder, ToolClass: "host"}, active: false},
				{id: ids.New(), policy: authz.AgentPolicy{BillingMode: "personal", CredentialHolderID: holder, ToolClass: "scoped"}, active: true},
			}
			src := build(t, holder, list)
			ctx := context.Background()
			got, err := src.PersonalAgents(ctx)
			if err != nil {
				t.Fatal(err)
			}
			want := []authz.PersonalAgent{{AgentID: list[0].id, CredentialHolderID: holder}, {AgentID: list[3].id, CredentialHolderID: holder}}
			slices.SortFunc(want, func(a, b authz.PersonalAgent) int { return bytes.Compare(a.AgentID[:], b.AgentID[:]) })
			if !slices.Equal(got, want) {
				t.Fatalf("PersonalAgents = %+v, want %+v", got, want)
			}
			pol, ok, err := src.AgentPolicy(ctx, list[0].id)
			if err != nil || !ok || pol.BillingMode != "personal" || pol.CredentialHolderID != holder || pol.ToolClass != "chat" || pol.ReportedToolClass != "host" ||
				!slices.Equal(pol.BillingModesSupported, []string{"personal", "organization"}) {
				t.Fatalf("AgentPolicy personal = %+v, %v, %v", pol, ok, err)
			}
			pol, ok, err = src.AgentPolicy(ctx, list[1].id)
			if err != nil || !ok || pol.BillingMode != "organization" || !pol.CredentialHolderID.IsZero() || pol.ReportedToolClass != "" ||
				len(pol.BillingModesSupported) != 0 {
				t.Fatalf("AgentPolicy organization = %+v, %v, %v", pol, ok, err)
			}
			if pol, ok, err := src.AgentPolicy(ctx, list[2].id); err != nil || !ok || pol.ToolClass != "host" {
				t.Fatalf("AgentPolicy of a disabled agent = %+v, %v, %v", pol, ok, err)
			}
			if _, ok, err := src.AgentPolicy(ctx, ids.New()); err != nil || ok {
				t.Fatalf("AgentPolicy unknown = %v, %v", ok, err)
			}
			if ready, err := src.DevEnvironmentReady(ctx, holder, ids.New()); err != nil || ready {
				t.Fatalf("DevEnvironmentReady = %v, %v", ready, err)
			}
			dir, ok := src.(authz.AgentDirectory)
			if !ok {
				t.Fatal("the source does not list the agents (M6-07)")
			}
			all, err := dir.AgentIDs(ctx)
			wantIDs := []ids.ID{list[0].id, list[1].id, list[2].id, list[3].id}
			slices.SortFunc(wantIDs, func(a, b ids.ID) int { return bytes.Compare(a[:], b[:]) })
			if err != nil || !slices.Equal(all, wantIDs) {
				t.Fatalf("AgentIDs = %v, %v, want every agent ascending %v", all, err, wantIDs)
			}
		})
	}
}
