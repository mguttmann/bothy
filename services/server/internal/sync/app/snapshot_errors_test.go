// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/agents"
	"github.com/OWNER/bothy/services/server/internal/chat"
	"github.com/OWNER/bothy/services/server/internal/identity"
	"github.com/OWNER/bothy/services/server/internal/instance"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/messaging"
	syncmod "github.com/OWNER/bothy/services/server/internal/sync"
	"github.com/OWNER/bothy/services/server/internal/sync/app"
)

// errStub is the failure a stub collaborator returns.
var errStub = errors.New("stub failure")

// snapStubs are collaborators of the snapshot; fail names the one step that fails.
type snapStubs struct {
	fail  string
	space ids.ID
	// syncmod.Settings and authorize.Authorizer are embedded (nil) for the methods the snapshot
	// never calls.
	syncmod.Settings
	authorize.Authorizer
}

func (s *snapStubs) err(step string) error {
	if s.fail == step {
		return errStub
	}
	return nil
}

func (s *snapStubs) Authorize(_ context.Context, _ authorize.Principal, a authorize.Action, _ authorize.Resource) (authorize.Decision, error) {
	if a == authorize.ActionThreadRead {
		return authorize.Decision{Outcome: authorize.OutcomeAllow}, s.err("thread")
	}
	return authorize.Decision{Outcome: authorize.OutcomeAllow}, s.err("authorize")
}

func (s *snapStubs) Scope(context.Context, authorize.Principal, authorize.Action) (authorize.Scope, error) {
	return authorize.Scope{}, s.err("scope")
}

func (s *snapStubs) EventSeq(context.Context) (int64, error) { return 9, s.err("head") }

func (s *snapStubs) Load(context.Context) (instance.Record, error) {
	return instance.Record{}, s.err("epoch")
}

func (s *snapStubs) Principal(_ context.Context, id ids.ID) (identity.PrincipalView, error) {
	return identity.PrincipalView{ID: id}, s.err("principal")
}

func (s *snapStubs) User(context.Context, ids.ID) (identity.UserView, error) {
	return identity.UserView{}, s.err("user")
}

func (s *snapStubs) ListSpaces(context.Context, actor.Actor) ([]chat.SpaceView, error) {
	gone := chat.SpaceView{Space: chat.Space{ID: ids.New()}}
	return []chat.SpaceView{gone, {Space: chat.Space{ID: s.space}}}, s.err("spaces")
}

func (s *snapStubs) ListSpaceChannels(_ context.Context, _ actor.Actor, space ids.ID) ([]chat.ChannelView, error) {
	if space != s.space {
		return nil, errs.New(errs.CodeNotFound, "")
	}
	return []chat.ChannelView{{Channel: chat.Channel{ID: ids.New(), SpaceID: space}}}, s.err("channels")
}

func (s *snapStubs) ListSidebar(context.Context) ([]syncmod.SidebarState, error) {
	return nil, s.err("sidebar")
}

func (s *snapStubs) List(context.Context) ([]syncmod.UserSetting, error) {
	return nil, s.err("settings")
}

// drafts is the Drafts collaborator: its List differs from the one of Settings.
type drafts struct {
	syncmod.Drafts
	s *snapStubs
}

func (d drafts) List(context.Context) ([]syncmod.Draft, error) { return nil, d.s.err("drafts") }

func (s *snapStubs) Counters(_ context.Context, channels []ids.ID) ([]chat.ThreadCounter, error) {
	return []chat.ThreadCounter{{ThreadID: ids.New(), ChannelID: channels[0], LastMessageSeq: 3}}, s.err("counters")
}

func (s *snapStubs) ReadStatesInChannels(context.Context, ids.ID, []ids.ID) ([]syncmod.ReadState, error) {
	return nil, s.err("read_states")
}

func (s *snapStubs) CountUnread(context.Context, messaging.UnreadMentionQuery) ([]messaging.ThreadMentionCount, error) {
	return nil, s.err("mentions")
}

// agentList is the AgentLister collaborator (its List differs too).
type agentList struct {
	s *snapStubs
}

func (l agentList) List(context.Context, actor.Actor, agents.Page) (agents.AgentPage, error) {
	return agents.AgentPage{}, l.s.err("agents")
}

func (s *snapStubs) ActiveRuns(context.Context, actor.Actor, []ids.ID) ([]syncmod.RunSummary, error) {
	return nil, s.err("runs")
}

func (s *snapStubs) ActiveThreads(_ context.Context, _ actor.Actor, q chat.ActiveThreadsQuery) ([]chat.Thread, error) {
	return []chat.Thread{{ID: ids.New(), ChannelID: q.ChannelIDs[0], Kind: "thread", State: "open"}}, s.err("active_threads")
}

func (s *snapStubs) EvaluateAll(context.Context, instance.FlagSubject) ([]instance.EvaluatedFlag, error) {
	return nil, s.err("flags")
}

// TestSnapshot_ReturnsEveryCollaboratorError: Build returns the error of each collaborator as it
// is, and a space that disappeared between both reads (404) is skipped.
func TestSnapshot_ReturnsEveryCollaboratorError(t *testing.T) {
	t.Parallel()
	steps := []string{"", "authorize", "head", "epoch", "scope", "principal", "user", "spaces", "channels", "sidebar", "settings",
		"drafts", "counters", "read_states", "mentions", "thread", "runs", "active_threads", "agents", "flags"}
	for _, step := range steps {
		t.Run("fail="+step, func(t *testing.T) {
			t.Parallel()
			s := &snapStubs{fail: step, space: ids.New()}
			snap, err := app.NewSnapshot(app.SnapshotDeps{Head: s, Epoch: s, Authorizer: s, Directory: s, Structure: s, Threads: s, Store: s,
				Mentions: s, Settings: s, Drafts: drafts{s: s}, Agents: agentList{s: s}, Runs: s, Flags: s, ActiveThreads: s,
				Config: app.SnapshotConfig{ThreadWindow: time.Hour, ThreadsPerChannel: 1}})
			if err != nil {
				t.Fatal(err)
			}
			ctx := actor.WithActor(context.Background(), actor.Actor{ID: ids.New(), Kind: actor.KindUser})
			v, err := snap.Build(ctx)
			if step != "" {
				if !errors.Is(err, errStub) {
					t.Fatalf("error %v, want the stub failure", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if v.HeadSeq != 9 || len(v.Spaces) != 1 || v.Spaces[0].ID != s.space || len(v.Channels) != 1 || len(v.UnreadThreads) != 1 ||
				len(v.ActiveThreads) != 1 || v.ActiveThreads[0].ChannelID != v.Channels[0].ID {
				t.Fatalf("snapshot %+v", v)
			}
		})
	}
}

// TestNewSnapshot_RefusesZeroBounds (M2-52): every collaborator present, but a thread window or a
// bound per channel below 1 is refused, so no snapshot is built without the keys of 04 18.
func TestNewSnapshot_RefusesZeroBounds(t *testing.T) {
	t.Parallel()
	s := &snapStubs{space: ids.New()}
	for _, cfg := range []app.SnapshotConfig{{}, {ThreadWindow: time.Hour}, {ThreadsPerChannel: 1}} {
		_, err := app.NewSnapshot(app.SnapshotDeps{Head: s, Epoch: s, Authorizer: s, Directory: s, Structure: s, Threads: s, Store: s,
			Mentions: s, Settings: s, Drafts: drafts{s: s}, Agents: agentList{s: s}, Runs: s, Flags: s, ActiveThreads: s, Config: cfg})
		if err == nil {
			t.Fatalf("config %+v accepted", cfg)
		}
	}
}
