// SPDX-License-Identifier: Apache-2.0

package postgres_test

import (
	"context"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/chat/app"
	"github.com/OWNER/bothy/services/server/internal/chat/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// agent inserts an agent principal with its agents row.
func (e env) agent(t *testing.T, handle string) ids.ID {
	t.Helper()
	id := ids.New()
	e.exec(t, `INSERT INTO principals (id, kind, handle, display_name) VALUES ($1, 'agent', $2, $2)`, id.String(), handle)
	e.exec(t, `INSERT INTO agents (principal_id, adapter_kind) VALUES ($1, 'hermes')`, id.String())
	return id
}

// TestRepositoryOperators (M1-54): the operator writes, the lock of the inheriting channels, the
// served targets of 10 section 5.3 and the thread facts against the tables.
func TestRepositoryOperators(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	ctx := context.Background()
	alex, _ := e.person(t, "alex")
	lotse, assistent := e.agent(t, "lotse"), e.agent(t, "assistent")
	sp := domain.NewSpace(ids.New(), "Support", "", "", "a0", alex)
	empty := domain.NewSpace(ids.New(), "Labor", "", "", "a1", alex)
	inherit := domain.NewChannel(ids.New(), sp.ID, domain.ChannelText, "inherit", "", "a0", alex, 20)
	own := domain.NewChannel(ids.New(), sp.ID, domain.ChannelText, "own", "", "a1", alex, 20)
	gone := domain.NewChannel(ids.New(), sp.ID, domain.ChannelText, "gone", "", "a2", alex, 20)
	var main domain.Thread
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	must(e.write(t, func(ctx context.Context) error {
		for _, s := range []domain.Space{sp, empty} {
			if err := e.repo.InsertSpace(ctx, s); err != nil {
				return err
			}
		}
		for _, c := range []domain.Channel{inherit, own, gone} {
			if err := e.repo.InsertChannel(ctx, c); err != nil {
				return err
			}
		}
		var err error
		main, err = e.repo.InsertMainThread(ctx, domain.NewMainThread(ids.New(), inherit.ID, alex, t0))
		return err
	}))
	sp.SetOperator(domain.OperatorAssignment{Mode: domain.OperatorAgent, AgentID: lotse})
	empty.SetOperator(domain.OperatorAssignment{Mode: domain.OperatorAgent, AgentID: assistent})
	own.SetOperator(domain.OperatorAssignment{Mode: domain.OperatorAgent, AgentID: assistent})
	gone.Archive(t0)
	var locked []app.ChannelRow
	must(e.write(t, func(ctx context.Context) error {
		if err := e.repo.SaveSpaceOperator(ctx, sp); err != nil {
			return err
		}
		if err := e.repo.SaveSpaceOperator(ctx, empty); err != nil {
			return err
		}
		if err := e.repo.SaveChannelOperator(ctx, own); err != nil {
			return err
		}
		if err := e.repo.SaveChannel(ctx, gone); err != nil {
			return err
		}
		inherit.Touch()
		if err := e.repo.TouchChannel(ctx, inherit); err != nil {
			return err
		}
		var err error
		locked, err = e.repo.LockInheritingChannels(ctx, sp.ID)
		return err
	}))
	if len(locked) != 1 || locked[0].Channel.ID != inherit.ID || locked[0].SpaceDefaultOperatorID != lotse || locked[0].Channel.Version != 2 {
		t.Fatalf("inheriting %+v", locked)
	}
	row, _, err := e.repo.Channel(ctx, own.ID)
	if err != nil || row.Channel.OperatorInherit || row.Channel.OperatorAgentID != assistent || row.Channel.Version != own.Version {
		t.Fatalf("own %+v %v", row, err)
	}
	// A closed thread with Assistent as fixed operator makes the channel inherit a target of
	// Assistent although Lotse operates it (BR-02).
	th := ids.New()
	e.exec(t, `INSERT INTO threads (id, channel_id, kind, state, closed_at, operator_agent_id, created_by)
		VALUES ($1, $2, 'thread', 'closed', now(), $3, $4)`, th.String(), inherit.ID.String(), assistent.String(), alex.String())
	served, err := e.repo.ServedTargets(ctx, assistent)
	if err != nil {
		t.Fatal(err)
	}
	want := map[app.ServedTarget]bool{{SpaceID: empty.ID}: true, {ChannelID: own.ID, SpaceID: sp.ID}: true, {ChannelID: inherit.ID, SpaceID: sp.ID}: true}
	if len(served) != len(want) {
		t.Fatalf("served %+v", served)
	}
	for _, s := range served {
		if !want[s] {
			t.Fatalf("served %+v", served)
		}
	}
	served, err = e.repo.ServedTargets(ctx, lotse)
	// Lotse is the default of Support: the space entry (zero channel first), then the channel.
	if err != nil || len(served) != 2 || served[0] != (app.ServedTarget{SpaceID: sp.ID}) || served[1] != (app.ServedTarget{ChannelID: inherit.ID, SpaceID: sp.ID}) {
		t.Fatalf("lotse served %+v %v", served, err)
	}
	f, found, err := e.repo.Thread(ctx, th)
	if err != nil || !found || f.Main || f.OperatorAgentID != assistent || f.ChannelID != inherit.ID || f.CreatedBy != alex {
		t.Fatalf("thread %+v %v %v", f, found, err)
	}
	f, found, err = e.repo.Thread(ctx, main.ID)
	if err != nil || !found || !f.Main || !f.OperatorAgentID.IsZero() {
		t.Fatalf("main thread %+v %v %v", f, found, err)
	}
	if _, found, err := e.repo.Thread(ctx, ids.New()); found || err != nil {
		t.Fatalf("unknown thread %v %v", found, err)
	}
	// Writes and locks need a unit of work.
	for _, err := range []error{e.repo.SaveSpaceOperator(ctx, sp), e.repo.SaveChannelOperator(ctx, own), e.repo.TouchChannel(ctx, own)} {
		if err == nil {
			t.Fatal("write outside a unit of work")
		}
	}
	if _, err := e.repo.LockInheritingChannels(ctx, sp.ID); err == nil {
		t.Fatal("lock outside a unit of work")
	}
}
