// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"context"
	"slices"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/authz"
	"github.com/OWNER/bothy/services/server/internal/authz/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// wantTriggerers checks the set of agent and the version it carries.
func wantTriggerers(t *testing.T, w *world, agent ids.ID, version int64, want ...ids.ID) {
	t.Helper()
	got, err := w.az.Triggerers(context.Background(), agent)
	if err != nil {
		t.Fatalf("Triggerers: %v", err)
	}
	if got.PrincipalIDs == nil || !slices.Equal(got.PrincipalIDs, ascendingIDs(want...)) {
		t.Errorf("Triggerers = %v, want %v", got.PrincipalIDs, ascendingIDs(want...))
	}
	if got.PermissionVersion != version {
		t.Errorf("permission_version %d, want %d", got.PermissionVersion, version)
	}
}

// TestAgentTriggerers_OperatedTargets (agent.triggerers.read, 09 17.7, 07 section 6): T(A) of the
// matrix world. Lotse operates Support, where Alex holds trigger; Texter operates Marketing, where
// Sam holds trigger; Runner has the class host, so SR2 keeps Robin out and only the owner stays;
// Assistent is personal for the owner. Sam's read through the group Leser does not count. The set
// carries the rights version of the request; no audit entry and no check-log entry is written.
func TestAgentTriggerers_OperatedTargets(t *testing.T) {
	w := newWorld(t)
	w.versions.Set(7)
	wantTriggerers(t, w, w.lotse, 7, w.owner, w.alex)
	wantTriggerers(t, w, w.texter, 7, w.owner, w.sam)
	wantTriggerers(t, w, w.runner, 7, w.owner)
	wantTriggerers(t, w, w.assistent, 7, w.owner)
	if n := len(w.rec.denials) + len(w.rec.records) + len(w.rec.detached); n != 0 {
		t.Errorf("%d audit entries", n)
	}
	ctx := authorize.WithCheckLog(context.Background())
	if _, err := w.az.Triggerers(ctx, w.lotse); err != nil {
		t.Fatal(err)
	}
	if n := len(authorize.Checks(ctx)); n != 0 {
		t.Errorf("%d check-log entries", n)
	}
}

// TestAgentTriggerers_GroupsAndStates: a member of a group with trigger on a served channel counts;
// a person with only write, an expired grant, a disabled person and an agent with trigger do not.
func TestAgentTriggerers_GroupsAndStates(t *testing.T) {
	w := newWorld(t)
	ctx := context.Background()
	user := func(id ids.ID) domain.Subject { return domain.Subject{Kind: domain.SubjectUser, PrincipalID: id} }
	onAllgemein := domain.Target{Kind: domain.TargetChannel, ChannelID: w.allgemein}
	person := func(status authz.PrincipalStatus) ids.ID {
		id := ids.New()
		w.principals.Put(authz.PrincipalFacts{ID: id, Kind: actor.KindUser, Status: status})
		return id
	}
	member, writer, expired, disabled := person(authz.PrincipalActive), person(authz.PrincipalActive), person(authz.PrincipalActive), person(authz.PrincipalDisabled)
	group := ids.New()
	mustNoErr(t, w.store.InsertGroup(ctx, domain.Group{ID: group, Name: "Trigger", Source: domain.GroupSourceLocal, CreatedAt: worldStart}))
	mustNoErr(t, w.store.InsertMembership(ctx, domain.Membership{ID: ids.New(), GroupID: group, UserID: member, AddedAt: worldStart}))
	w.grant(t, domain.Subject{Kind: domain.SubjectGroup, GroupID: group}, onAllgemein, domain.LevelTrigger, time.Time{})
	w.grant(t, user(writer), onAllgemein, domain.LevelWrite, time.Time{})
	w.grant(t, user(expired), onAllgemein, domain.LevelTrigger, worldStart.Add(time.Hour))
	w.grant(t, user(disabled), onAllgemein, domain.LevelTrigger, time.Time{})
	w.grant(t, domain.Subject{Kind: domain.SubjectAgent, PrincipalID: w.texter}, onAllgemein, domain.LevelTrigger, time.Time{})
	w.clk.Advance(2 * time.Hour)
	wantTriggerers(t, w, w.lotse, 1, w.owner, w.alex, member)
}

// TestAgentTriggerers_GuestChannels (05 section 5 point 3): a person with trigger on a channel with
// allow_mentioned_agents, where the agent has at least write, may trigger the agent by a mention,
// so the set holds Sam for Lotse once Lotse may write in Kampagnen of Marketing. Without the flag,
// without the agent's write, in a read-only or an archived channel it does not; a personal agent
// never is a guest agent.
func TestAgentTriggerers_GuestChannels(t *testing.T) {
	w := newWorld(t)
	agent := func(id ids.ID) domain.Subject { return domain.Subject{Kind: domain.SubjectAgent, PrincipalID: id} }
	onKampagnen := domain.Target{Kind: domain.TargetChannel, ChannelID: w.kampagnen}
	guest := func(c *authz.ChannelFacts) {
		*c = authz.ChannelFacts{ID: c.ID, SpaceID: c.SpaceID, Kind: c.Kind, EffectiveOperatorID: c.EffectiveOperatorID, AllowMentionedAgents: true}
	}

	// The flag alone is not enough: Lotse has no write there.
	w.setChannel(t, w.kampagnen, guest)
	wantTriggerers(t, w, w.lotse, 1, w.owner, w.alex)
	// Read is not enough either.
	w.grant(t, agent(w.lotse), onKampagnen, domain.LevelRead, time.Time{})
	wantTriggerers(t, w, w.lotse, 1, w.owner, w.alex)

	w.grant(t, agent(w.lotse), onKampagnen, domain.LevelWrite, time.Time{})
	w.versions.Set(2)
	wantTriggerers(t, w, w.lotse, 2, w.owner, w.alex, w.sam)

	for name, change := range map[string]func(*authz.ChannelFacts){
		"no flag":   func(c *authz.ChannelFacts) { c.AllowMentionedAgents = false },
		"read-only": func(c *authz.ChannelFacts) { c.ReadOnly = true },
		"archived":  func(c *authz.ChannelFacts) { c.Archived = true },
	} {
		w.setChannel(t, w.kampagnen, change)
		got, err := w.az.Triggerers(context.Background(), w.lotse)
		if err != nil || slices.Contains(got.PrincipalIDs, w.sam) {
			t.Errorf("%s: %v, %v", name, got.PrincipalIDs, err)
		}
		w.setChannel(t, w.kampagnen, guest)
	}

	// A personal agent with write on a guest channel: still only its credential holder.
	w.grant(t, agent(w.assistent), onKampagnen, domain.LevelWrite, time.Time{})
	wantTriggerers(t, w, w.assistent, 2, w.owner)
}

// TestAgentTriggerers_Personal (05 section 5 point 4, 09 17.7 layer 3): a personal agent admits
// only its credential holder. A holder outside T(A) gives the empty set; the other people of T(A)
// never appear.
func TestAgentTriggerers_Personal(t *testing.T) {
	w := newWorld(t)
	user := func(id ids.ID) domain.Subject { return domain.Subject{Kind: domain.SubjectUser, PrincipalID: id} }
	w.grant(t, user(w.alex), domain.Target{Kind: domain.TargetChannel, ChannelID: w.notizen}, domain.LevelTrigger, time.Time{})
	wantTriggerers(t, w, w.assistent, 1, w.owner)

	w.policies.Put(w.assistent, authz.AgentPolicy{BillingMode: domain.BillingPersonal, CredentialHolderID: w.alex, ToolClass: domain.ToolScoped})
	wantTriggerers(t, w, w.assistent, 1, w.alex)

	w.policies.Put(w.assistent, authz.AgentPolicy{BillingMode: domain.BillingPersonal, CredentialHolderID: w.robin, ToolClass: domain.ToolScoped})
	wantTriggerers(t, w, w.assistent, 1)
}

// TestAgentTriggerers_NoSet: an unknown id, a person, a disabled agent and an agent without a
// policy have the empty set, never nil; a source error is an error without a set.
func TestAgentTriggerers_NoSet(t *testing.T) {
	w := newWorld(t)
	wantTriggerers(t, w, ids.New(), 1)
	wantTriggerers(t, w, w.alex, 1)
	noPolicy := ids.New()
	w.principals.Put(authz.PrincipalFacts{ID: noPolicy, Kind: actor.KindAgent, Status: authz.PrincipalActive})
	wantTriggerers(t, w, noPolicy, 1)
	w.principals.Put(authz.PrincipalFacts{ID: w.lotse, Kind: actor.KindAgent, Status: authz.PrincipalDisabled})
	wantTriggerers(t, w, w.lotse, 1)
	w.principals.Put(authz.PrincipalFacts{ID: w.lotse, Kind: actor.KindAgent, Status: authz.PrincipalActive})

	w.structure.Fail(true)
	if got, err := w.az.Triggerers(context.Background(), w.lotse); err == nil || got.PrincipalIDs != nil {
		t.Errorf("structure error: %v, %v", got, err)
	}
	w.structure.Fail(false)
	w.store.Fail(true)
	if got, err := w.az.Triggerers(context.Background(), w.lotse); err == nil || got.PrincipalIDs != nil {
		t.Errorf("store error: %v, %v", got, err)
	}
	w.store.Fail(false)
	w.versions.Fail(true)
	if got, err := w.az.Triggerers(context.Background(), w.lotse); err == nil || got.PrincipalIDs != nil {
		t.Errorf("version error: %v, %v", got, err)
	}
}
