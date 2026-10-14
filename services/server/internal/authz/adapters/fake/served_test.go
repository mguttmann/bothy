// SPDX-License-Identifier: Apache-2.0

package fake_test

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/authz"
	"github.com/OWNER/bothy/services/server/internal/authz/adapters/fake"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// TestStructure_ServedTargets: operated channels, channels of a fixed thread operator (BR-02),
// default-operated spaces; archived channels and main threads do not count.
func TestStructure_ServedTargets(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s := fake.NewStructure()
	agent, other := ids.New(), ids.New()
	sp, sp2 := ids.New(), ids.New()
	operated, viaThread, archived, foreign := ids.New(), ids.New(), ids.New(), ids.New()
	s.PutChannel(authz.ChannelFacts{ID: operated, SpaceID: sp, Kind: authz.ChannelText, EffectiveOperatorID: agent})
	s.PutChannel(authz.ChannelFacts{ID: viaThread, SpaceID: sp, Kind: authz.ChannelText, EffectiveOperatorID: other})
	s.PutChannel(authz.ChannelFacts{ID: archived, SpaceID: sp, Kind: authz.ChannelText, EffectiveOperatorID: agent, Archived: true})
	s.PutChannel(authz.ChannelFacts{ID: foreign, SpaceID: sp, Kind: authz.ChannelText, EffectiveOperatorID: other})
	s.PutThread(authz.ThreadFacts{ID: ids.New(), ChannelID: viaThread, OperatorAgentID: agent})
	s.PutThread(authz.ThreadFacts{ID: ids.New(), ChannelID: foreign, Main: true, OperatorAgentID: agent})
	s.SetDefaultOperator(sp2, agent)
	got, err := s.ServedTargets(ctx, agent)
	if err != nil {
		t.Fatal(err)
	}
	want := []authz.ChannelRef{{ChannelID: operated, SpaceID: sp}, {ChannelID: viaThread, SpaceID: sp}, {SpaceID: sp2}}
	slices.SortFunc(want, func(a, b authz.ChannelRef) int {
		if c := slices.Compare(a.SpaceID[:], b.SpaceID[:]); c != 0 {
			return c
		}
		return slices.Compare(a.ChannelID[:], b.ChannelID[:])
	})
	if !slices.Equal(got, want) {
		t.Fatalf("ServedTargets = %v, want %v", got, want)
	}
	s.SetDefaultOperator(sp2, ids.ID{})
	if got, _ := s.ServedTargets(ctx, agent); len(got) != 2 {
		t.Fatalf("after removing the default operator: %v", got)
	}
	s.Fail(true)
	if _, err := s.ServedTargets(ctx, agent); !errors.Is(err, fake.ErrInjected) {
		t.Fatal(err)
	}
}

// TestPolicies_PersonalAgents: active personal agents, ascending.
func TestPolicies_PersonalAgents(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	p := fake.NewPolicies()
	a, b, org := ids.New(), ids.New(), ids.New()
	holder := ids.New()
	p.Put(b, authz.AgentPolicy{BillingMode: "personal", CredentialHolderID: holder})
	p.Put(a, authz.AgentPolicy{BillingMode: "personal", CredentialHolderID: holder})
	p.Put(org, authz.AgentPolicy{BillingMode: "organization"})
	got, err := p.PersonalAgents(ctx)
	if err != nil || len(got) != 2 || got[0].AgentID != a || got[1].AgentID != b || got[0].CredentialHolderID != holder {
		t.Fatalf("%v %v", got, err)
	}
	p.SetActive(a, false)
	if got, _ := p.PersonalAgents(ctx); len(got) != 1 || got[0].AgentID != b {
		t.Fatalf("inactive agent listed: %v", got)
	}
	p.SetActive(a, true)
	if got, _ := p.PersonalAgents(ctx); len(got) != 2 {
		t.Fatalf("reactivated agent missing: %v", got)
	}
}

// TestStructure_SpaceArchived: a space is archived only after SetSpaceArchived.
func TestStructure_SpaceArchived(t *testing.T) {
	t.Parallel()
	s := fake.NewStructure()
	sp := ids.New()
	s.PutSpace(sp)
	if a, err := s.SpaceArchived(context.Background(), sp); err != nil || a {
		t.Fatalf("%v %v", a, err)
	}
	s.SetSpaceArchived(sp, true)
	if a, _ := s.SpaceArchived(context.Background(), sp); !a {
		t.Fatal("not archived")
	}
}
