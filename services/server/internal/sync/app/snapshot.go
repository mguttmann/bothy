// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"errors"
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
	"github.com/OWNER/bothy/services/server/internal/sync"
)

// RunSummarySource is the outgoing port of the running runs of the snapshot (schema RunSummary).
// internal/wire binds adapters/runsummary over the runs module (M2-05), which applies the
// visibility pipeline of listActiveRuns.
type RunSummarySource interface {
	// ActiveRuns returns the runs that are not finished in the threads of channelIDs (the visible
	// channels of the caller a). Errors: the source's.
	ActiveRuns(ctx context.Context, a actor.Actor, channelIDs []ids.ID) ([]sync.RunSummary, error)
}

// SnapshotStructure is the part of chat.Structure the snapshot lists spaces and channels with:
// the same listings that serve listSpaces and listSpaceChannels.
type SnapshotStructure interface {
	ListSpaces(ctx context.Context, a actor.Actor) ([]chat.SpaceView, error)
	ListSpaceChannels(ctx context.Context, a actor.Actor, spaceID ids.ID) ([]chat.ChannelView, error)
}

// ThreadCounters is the part of chat.Threads with the counters of the threads (M1-63).
type ThreadCounters interface {
	Counters(ctx context.Context, channelIDs []ids.ID) ([]chat.ThreadCounter, error)
}

// SnapshotStore reads the read positions of a person in many channels (adapters/postgres).
type SnapshotStore interface {
	// ReadStatesInChannels returns the rows of user in channels, ascending by thread.
	ReadStatesInChannels(ctx context.Context, user ids.ID, channels []ids.ID) ([]sync.ReadState, error)
}

// ActiveThreadSource is the part of chat the snapshot reads its active threads with
// (chat.ActiveThreads, M2-52).
type ActiveThreadSource interface {
	ActiveThreads(ctx context.Context, a actor.Actor, q chat.ActiveThreadsQuery) ([]chat.Thread, error)
}

// SnapshotConfig are the bounds of the active threads of the snapshot (04 section 18).
type SnapshotConfig struct {
	// ThreadWindow is sync.snapshot_thread_window: an open thread with activity within it is an
	// active thread.
	ThreadWindow time.Duration `config:"sync.snapshot_thread_window"`
	// ThreadsPerChannel is sync.snapshot_threads_per_channel: the bound per channel, which threads
	// with an active run and unread threads count toward but are never left out for.
	ThreadsPerChannel int `config:"sync.snapshot_threads_per_channel"`
}

// AgentLister is the part of agents.Agents behind listAgents.
type AgentLister interface {
	List(ctx context.Context, a actor.Actor, p agents.Page) (agents.AgentPage, error)
}

// PrincipalDirectory is the part of identity.Directory the snapshot names the caller with.
type PrincipalDirectory interface {
	Principal(ctx context.Context, id ids.ID) (identity.PrincipalView, error)
	User(ctx context.Context, id ids.ID) (identity.UserView, error)
}

// FlagEvaluator evaluates every feature flag for a caller (instance.Flags).
type FlagEvaluator interface {
	EvaluateAll(ctx context.Context, s instance.FlagSubject) ([]instance.EvaluatedFlag, error)
}

// SnapshotDeps are the collaborators of the snapshot.
type SnapshotDeps struct {
	Head       HeadSource
	Epoch      EpochSource
	Authorizer authorize.Authorizer
	Directory  PrincipalDirectory
	Structure  SnapshotStructure
	Threads    ThreadCounters
	Store      SnapshotStore
	Mentions   MentionCounter
	// Settings gives the sidebar and the stored settings, Drafts the drafts of the caller (the
	// personal data ports of M1-61, Personal.Settings and Personal.Drafts).
	Settings sync.Settings
	Drafts   sync.Drafts
	Agents   AgentLister
	Runs     RunSummarySource
	Flags    FlagEvaluator
	// ActiveThreads reads the active threads (chat, M2-52) within the bounds of Config.
	ActiveThreads ActiveThreadSource
	Config        SnapshotConfig
}

// SnapshotService implements sync.Snapshot (M1-63). It is safe for concurrent use.
type SnapshotService struct {
	d SnapshotDeps
}

// Compile-time check: the incoming port.
var _ sync.Snapshot = (*SnapshotService)(nil)

// NewSnapshot returns the use case. Errors: a missing collaborator, a thread window or a bound per
// channel below 1.
func NewSnapshot(d SnapshotDeps) (*SnapshotService, error) {
	if d.Head == nil || d.Epoch == nil || d.Authorizer == nil || d.Directory == nil || d.Structure == nil || d.Threads == nil ||
		d.Store == nil || d.Mentions == nil || d.Settings == nil || d.Drafts == nil || d.Agents == nil || d.Runs == nil || d.Flags == nil ||
		d.ActiveThreads == nil {
		return nil, errors.New("sync: NewSnapshot needs every collaborator")
	}
	if d.Config.ThreadWindow <= 0 || d.Config.ThreadsPerChannel < 1 {
		return nil, errors.New("sync: sync.snapshot_thread_window and sync.snapshot_threads_per_channel must be positive")
	}
	return &SnapshotService{d: d}, nil
}

// Build implements sync.Snapshot (brief M1-63 design notes 7 to 10): sync.snapshot.read, then
// head_seq and sync_epoch first and every other part after them (READ COMMITTED), so every change
// with seq <= head_seq is contained and later ones arrive by catch-up (04 section 9). It carries no
// thread content.
func (s *SnapshotService) Build(ctx context.Context) (sync.SnapshotView, error) {
	a := caller(ctx)
	d, err := s.d.Authorizer.Authorize(ctx, a, authorize.ActionSyncSnapshotRead, authorize.Resource{Kind: authorize.ResourceInstance})
	if err != nil {
		return sync.SnapshotView{}, err
	}
	if err := d.Err(); err != nil {
		return sync.SnapshotView{}, err
	}
	v := emptyView()
	if v.HeadSeq, v.SyncEpoch, err = logHead(ctx, s.d.Head, s.d.Epoch); err != nil {
		return sync.SnapshotView{}, err
	}
	sc, err := s.d.Authorizer.Scope(ctx, a, authorize.ActionChannelRead)
	if err != nil {
		return sync.SnapshotView{}, err
	}
	v.PermissionVersion = sc.PermissionVersion
	if err := s.principal(ctx, a, &v); err != nil {
		return sync.SnapshotView{}, err
	}
	if err := s.structure(ctx, a, &v); err != nil {
		return sync.SnapshotView{}, err
	}
	channels := make([]ids.ID, 0, len(v.Channels))
	for _, c := range v.Channels {
		channels = append(channels, c.ID)
	}
	if a.Kind == actor.KindUser {
		if err := s.personal(ctx, a, channels, &v); err != nil {
			return sync.SnapshotView{}, err
		}
	}
	runs, err := s.d.Runs.ActiveRuns(ctx, a, channels)
	if err != nil {
		return sync.SnapshotView{}, err
	}
	v.ActiveRuns = append(v.ActiveRuns, runs...)
	if err := s.activeThreads(ctx, a, channels, &v); err != nil {
		return sync.SnapshotView{}, err
	}
	if err := s.agents(ctx, a, &v); err != nil {
		return sync.SnapshotView{}, err
	}
	flags, err := s.d.Flags.EvaluateAll(ctx, instance.SubjectOf(a))
	if err != nil {
		return sync.SnapshotView{}, err
	}
	for _, f := range flags {
		v.Features = append(v.Features, sync.EvaluatedFlag{Key: f.Key, Enabled: f.Enabled})
	}
	return v, nil
}

// emptyView returns a view whose lists are empty, never nil (no JSON null, ADR 0053).
func emptyView() sync.SnapshotView {
	return sync.SnapshotView{
		Spaces: []sync.SnapshotSpace{}, Channels: []sync.SnapshotChannel{}, Sidebar: []sync.SidebarState{},
		SidebarChannels: []sync.SidebarChannelState{},
		UnreadThreads:   []sync.UnreadThread{}, ActiveRuns: []sync.RunSummary{}, ActiveThreads: []sync.SnapshotThread{},
		Agents:   []sync.AgentPresence{},
		Settings: []sync.UserSetting{}, Drafts: []sync.DraftRef{}, Features: []sync.EvaluatedFlag{},
	}
}

// principal fills principal from the directory and is_owner (an active owners row; only people
// are owners).
func (s *SnapshotService) principal(ctx context.Context, a actor.Actor, v *sync.SnapshotView) error {
	p, err := s.d.Directory.Principal(ctx, a.ID)
	if err != nil {
		return err
	}
	v.Principal = sync.SnapshotPrincipal{ID: p.ID, Kind: string(p.Kind), Handle: p.Handle, DisplayName: p.DisplayName, AvatarFileID: p.AvatarFileID}
	if a.Kind != actor.KindUser {
		return nil
	}
	u, err := s.d.Directory.User(ctx, a.ID)
	if err != nil {
		return err
	}
	v.IsOwner = u.IsOwner
	return nil
}

// structure fills spaces and channels: the visible spaces that are not archived, and per space its
// visible channels that are not archived, through the listings of chat. A space that became
// invisible or unknown between both reads is skipped.
func (s *SnapshotService) structure(ctx context.Context, a actor.Actor, v *sync.SnapshotView) error {
	spaces, err := s.d.Structure.ListSpaces(ctx, a)
	if err != nil {
		return err
	}
	for _, sp := range spaces {
		chs, err := s.d.Structure.ListSpaceChannels(ctx, a, sp.Space.ID)
		if errs.KindOf(err) == errs.KindNotFound {
			continue
		}
		if err != nil {
			return err
		}
		v.Spaces = append(v.Spaces, spaceOf(sp))
		for _, c := range chs {
			v.Channels = append(v.Channels, channelOf(c))
		}
	}
	return nil
}

// personal fills the parts of a person: sidebar (spaces and channels), settings, drafts and unread
// threads.
func (s *SnapshotService) personal(ctx context.Context, a actor.Actor, channels []ids.ID, v *sync.SnapshotView) error {
	sidebar, err := s.d.Settings.ListSidebar(ctx)
	if err != nil {
		return err
	}
	v.Sidebar = append(v.Sidebar, sidebar...)
	sidebarChannels, err := s.d.Settings.ListSidebarChannels(ctx)
	if err != nil {
		return err
	}
	v.SidebarChannels = append(v.SidebarChannels, sidebarChannels...)
	settings, err := s.d.Settings.List(ctx)
	if err != nil {
		return err
	}
	v.Settings = append(v.Settings, settings...)
	drafts, err := s.d.Drafts.List(ctx)
	if err != nil {
		return err
	}
	for _, dr := range drafts {
		v.Drafts = append(v.Drafts, sync.DraftRef{ID: dr.ID, ThreadID: dr.ThreadID, ChannelID: dr.ChannelID, Version: dr.Version})
	}
	unread, err := s.unread(ctx, a, channels)
	if err != nil {
		return err
	}
	v.UnreadThreads = append(v.UnreadThreads, unread...)
	return nil
}

// unread computes the unread threads of a in channels through the ports of chat, sync and
// messaging, never by a join across module tables (07 section 7.4 rule 6, docs/DECISIONS.md
// M1-63/4): unread = last_message_seq - GREATEST(last_read_seq, unread_floor_seq), never below 0,
// and the unread mentions above the same position. A thread is listed when either is above 0 and
// thread.read allows it in filter mode (07 7.4 rule 3). A thread without a message above its
// floor has nothing unread for anybody, so its mentions do not count either.
func (s *SnapshotService) unread(ctx context.Context, a actor.Actor, channels []ids.ID) ([]sync.UnreadThread, error) {
	out := []sync.UnreadThread{}
	if len(channels) == 0 {
		return out, nil
	}
	counters, err := s.d.Threads.Counters(ctx, channels)
	if err != nil || len(counters) == 0 {
		return out, err
	}
	read, position, mentions, err := s.positions(ctx, a, channels, counters)
	if err != nil {
		return nil, err
	}
	fctx := authorize.Filter(ctx)
	for _, c := range counters {
		n := max(c.LastMessageSeq-position[c.ThreadID], 0)
		m := mentions[c.ThreadID]
		if n == 0 && m == 0 {
			continue
		}
		d, err := s.d.Authorizer.Authorize(fctx, a, authorize.ActionThreadRead, authorize.Resource{Kind: authorize.ResourceThread, ID: c.ThreadID})
		if err != nil {
			return nil, err
		}
		if !d.Allowed() {
			continue
		}
		out = append(out, sync.UnreadThread{ThreadID: c.ThreadID, ChannelID: c.ChannelID, UnreadCount: int(n), MentionCount: m,
			LastMessageSeq: c.LastMessageSeq, LastReadSeq: read[c.ThreadID]})
	}
	return out, nil
}

// positions returns the read position of a per thread (read), the position unread counts from
// (GREATEST(last_read_seq, unread_floor_seq)) per thread of counters, and the unread mentions
// above it.
func (s *SnapshotService) positions(ctx context.Context, a actor.Actor, channels []ids.ID, counters []chat.ThreadCounter) (
	read, position map[ids.ID]int64, mentions map[ids.ID]int, err error) {
	rows, err := s.d.Store.ReadStatesInChannels(ctx, a.ID, channels)
	if err != nil {
		return nil, nil, nil, err
	}
	read = make(map[ids.ID]int64, len(rows))
	for _, r := range rows {
		read[r.ThreadID] = r.LastReadSeq
	}
	position = make(map[ids.ID]int64, len(counters))
	for _, c := range counters {
		position[c.ThreadID] = max(read[c.ThreadID], c.UnreadFloorSeq)
	}
	counts, err := s.d.Mentions.CountUnread(ctx, messaging.UnreadMentionQuery{PrincipalID: a.ID, VisibleChannelIDs: channels, LastReadSeq: position})
	if err != nil {
		return nil, nil, nil, err
	}
	mentions = make(map[ids.ID]int, len(counts))
	for _, m := range counts {
		mentions[m.ThreadID] = m.Count
	}
	return read, position, mentions, nil
}

// activeThreads fills the active threads (04 section 6.3 item 4, R-SY-28): the threads of the
// active runs and the unread threads of the same snapshot are pinned, so they are never left
// out, and chat adds the threads active within sync.snapshot_thread_window up to
// sync.snapshot_threads_per_channel per channel in one read. It runs after head_seq like every
// other part (READ COMMITTED), so a thread event above head_seq arrives by catch-up or live
// even when the thread is in the snapshot already, and one at or below it is in the snapshot.
func (s *SnapshotService) activeThreads(ctx context.Context, a actor.Actor, channels []ids.ID, v *sync.SnapshotView) error {
	if len(channels) == 0 {
		return nil
	}
	pinned := make([]ids.ID, 0, len(v.ActiveRuns)+len(v.UnreadThreads))
	for _, r := range v.ActiveRuns {
		pinned = append(pinned, r.ThreadID)
	}
	for _, u := range v.UnreadThreads {
		pinned = append(pinned, u.ThreadID)
	}
	list, err := s.d.ActiveThreads.ActiveThreads(ctx, a, chat.ActiveThreadsQuery{
		ChannelIDs: channels, PinnedIDs: pinned, Window: s.d.Config.ThreadWindow, PerChannel: s.d.Config.ThreadsPerChannel,
	})
	if err != nil {
		return err
	}
	for _, th := range list {
		v.ActiveThreads = append(v.ActiveThreads, threadOf(th))
	}
	return nil
}

// threadOf converts a thread of chat; the tags are never nil.
func threadOf(th chat.Thread) sync.SnapshotThread {
	if th.TagIDs == nil {
		th.TagIDs = []ids.ID{}
	}
	return sync.SnapshotThread(th)
}

// agents fills the agents a may read (agent.read per item, the listing behind listAgents) with
// their presence: state and reported billing mode as the listing shows them, which takes them from
// the presence tracker of the process (agents.Presence, M2-20). Only agents have presence; a
// caller the listing refuses as a whole (an app without agents:read) sees none.
func (s *SnapshotService) agents(ctx context.Context, a actor.Actor, v *sync.SnapshotView) error {
	p := agents.Page{}
	for {
		page, err := s.d.Agents.List(ctx, a, p)
		if errs.KindOf(err) == errs.KindForbidden {
			return nil
		}
		if err != nil {
			return err
		}
		for _, ag := range page.Items {
			state := ag.Presence.State
			if state == "" {
				state = agents.StateUnknown
			}
			v.Agents = append(v.Agents, sync.AgentPresence{AgentID: ag.ID, State: state, ReportedBillingMode: ag.Presence.ReportedBillingMode})
		}
		if page.NextCursor == "" {
			return nil
		}
		p.Cursor = page.NextCursor
	}
}

// spaceOf converts a space view of chat.
func spaceOf(sp chat.SpaceView) sync.SnapshotSpace {
	s := sp.Space
	return sync.SnapshotSpace{
		ID: s.ID, Name: s.Name, Icon: s.Icon, Color: s.Color, SortKey: s.SortKey, DefaultOperatorAgentID: s.DefaultOperatorAgentID,
		CreatedAt: s.CreatedAt, ArchivedAt: s.ArchivedAt, Version: s.Version, Level: sp.Level, Capabilities: nonNil(sp.Capabilities),
	}
}

// channelOf converts a channel view of chat.
func channelOf(cv chat.ChannelView) sync.SnapshotChannel {
	c := cv.Channel
	return sync.SnapshotChannel{
		ID: c.ID, SpaceID: c.SpaceID, Kind: c.Kind, Name: c.Name, Topic: c.Topic, SortKey: c.SortKey,
		OperatorMode: c.Operator.Mode, OperatorAgentID: c.Operator.AgentID, EffectiveOperatorID: c.Operator.EffectiveAgentID,
		ForumDefaultSort: c.ForumDefaultSort, DefaultNotificationLevel: c.DefaultNotificationLevel, ReadOnly: c.ReadOnly,
		ProjectBound: c.ProjectBound, MainThreadID: c.MainThreadID, CreatedAt: c.CreatedAt, ArchivedAt: c.ArchivedAt, Version: c.Version,
		SettingsVersion: c.SettingsVersion, Level: cv.Level, Capabilities: nonNil(cv.Capabilities),
	}
}

// nonNil returns list, or an empty list for nil.
func nonNil(list []string) []string {
	if list == nil {
		return []string{}
	}
	return list
}
