// SPDX-License-Identifier: Apache-2.0

package httpapi

import (
	"context"
	"time"

	"github.com/OWNER/bothy/services/server/api/rest"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/sync"
)

// Sync implements the REST operations listEvents and getSnapshot of the sync module (M1-63). The
// use cases authorize (events.catch_up, sync.snapshot.read); the handlers only convert.
//
// Example:
//
//	h := httpapi.NewSync(catchUp, snapshot)
//	built, err := routes.Build(wire.Handlers{Sync: h}, wire.RouteOptions(), cat)
type Sync struct {
	catchUp  sync.CatchUp
	snapshot sync.Snapshot
}

// NewSync returns the handlers over the ports of catch-up and snapshot.
func NewSync(c sync.CatchUp, s sync.Snapshot) *Sync {
	return &Sync{catchUp: c, snapshot: s}
}

// ListEvents implements rest.StrictServerInterface (GET /events): Check, then one page up to its
// head, or 410 resync_required with the reason (04 section 7.4). next_cursor is present on every
// page, also an empty one. A limit above sync.catchup_page_size is capped by the use case.
func (h *Sync) ListEvents(ctx context.Context, req rest.ListEventsRequestObject) (rest.ListEventsResponseObject, error) {
	in := sync.CheckInput{After: req.Params.After}
	if req.Params.Epoch != nil {
		e := ids.ID(*req.Params.Epoch)
		in.Epoch = &e
	}
	res, err := h.catchUp.Check(ctx, in)
	if err != nil {
		return nil, err
	}
	if res.Reason != "" {
		return nil, errs.New(errs.CodeResyncRequired, "catch-up from this cursor is not possible; load a snapshot").WithReason(string(res.Reason))
	}
	limit := 0
	if req.Params.Limit != nil {
		limit = *req.Params.Limit
	}
	page, err := h.catchUp.Page(ctx, sync.PageInput{After: req.Params.After, Head: res.HeadSeq, Limit: limit})
	if err != nil {
		return nil, err
	}
	out := rest.EventPage{Events: make([]rest.Event, len(page.Events)), NextCursor: page.NextCursor, HeadSeq: res.HeadSeq,
		SyncEpoch: rest.Uuid(res.SyncEpoch)}
	for i, raw := range page.Events {
		if err := out.Events[i].UnmarshalJSON(raw); err != nil {
			return nil, errs.Wrap(errs.CodeInternalError, err, "sync: an envelope is not JSON")
		}
	}
	return rest.ListEvents200JSONResponse{Body: out}, nil
}

// GetSnapshot implements rest.StrictServerInterface (GET /sync/snapshot).
func (h *Sync) GetSnapshot(ctx context.Context, _ rest.GetSnapshotRequestObject) (rest.GetSnapshotResponseObject, error) {
	v, err := h.snapshot.Build(ctx)
	if err != nil {
		return nil, err
	}
	out, err := SnapshotREST(v)
	if err != nil {
		return nil, err
	}
	return rest.GetSnapshot200JSONResponse{Body: out}, nil
}

// SnapshotREST converts a snapshot (schema Snapshot); the gateway frame ready (M1-64) reuses it.
// Every list is an array, never null. Errors: internal for a stored setting that is not JSON.
func SnapshotREST(v sync.SnapshotView) (rest.Snapshot, error) {
	out := rest.Snapshot{
		HeadSeq: v.HeadSeq, SyncEpoch: rest.Uuid(v.SyncEpoch), PermissionVersion: v.PermissionVersion, IsOwner: v.IsOwner,
		Principal: rest.PrincipalRef{Id: rest.Uuid(v.Principal.ID), Kind: rest.PrincipalKind(v.Principal.Kind), Handle: v.Principal.Handle,
			DisplayName: v.Principal.DisplayName, AvatarFileId: optionalID(v.Principal.AvatarFileID)},
		Spaces: make([]rest.SpaceView, 0, len(v.Spaces)), Channels: make([]rest.ChannelView, 0, len(v.Channels)),
		Sidebar: make([]rest.SidebarState, 0, len(v.Sidebar)), UnreadThreads: make([]rest.UnreadThread, 0, len(v.UnreadThreads)),
		ActiveRuns: make([]rest.RunSummary, 0, len(v.ActiveRuns)), Agents: make([]rest.AgentPresence, 0, len(v.Agents)),
		Settings: make([]rest.UserSetting, 0, len(v.Settings)), Drafts: make([]rest.DraftRef, 0, len(v.Drafts)),
		Features: make([]rest.EvaluatedFlag, 0, len(v.Features)),
	}
	for _, s := range v.Spaces {
		out.Spaces = append(out.Spaces, spaceViewREST(s))
	}
	for _, c := range v.Channels {
		out.Channels = append(out.Channels, channelViewREST(c))
	}
	for _, st := range v.Sidebar {
		out.Sidebar = append(out.Sidebar, sidebarREST(st))
	}
	for _, u := range v.UnreadThreads {
		out.UnreadThreads = append(out.UnreadThreads, rest.UnreadThread{ThreadId: rest.Uuid(u.ThreadID), ChannelId: rest.Uuid(u.ChannelID),
			UnreadCount: u.UnreadCount, MentionCount: u.MentionCount, LastMessageSeq: u.LastMessageSeq, LastReadSeq: u.LastReadSeq})
	}
	for _, r := range v.ActiveRuns {
		out.ActiveRuns = append(out.ActiveRuns, runREST(r))
	}
	for _, a := range v.Agents {
		p := rest.AgentPresence{AgentId: rest.Uuid(a.AgentID), State: rest.AgentRuntimeState(a.State)}
		if a.ReportedBillingMode != "" {
			m := rest.BillingMode(a.ReportedBillingMode)
			p.ReportedBillingMode = &m
		}
		out.Agents = append(out.Agents, p)
	}
	for _, st := range v.Settings {
		s, err := SettingREST(st)
		if err != nil {
			return rest.Snapshot{}, err
		}
		out.Settings = append(out.Settings, s)
	}
	for _, d := range v.Drafts {
		out.Drafts = append(out.Drafts, rest.DraftRef{Id: rest.Uuid(d.ID), ThreadId: optionalID(d.ThreadID), ChannelId: rest.Uuid(d.ChannelID),
			Version: d.Version})
	}
	for _, f := range v.Features {
		out.Features = append(out.Features, rest.EvaluatedFlag{Key: f.Key, Enabled: f.Enabled})
	}
	return out, nil
}

// spaceViewREST converts a space of the snapshot (schema SpaceView).
func spaceViewREST(s sync.SnapshotSpace) rest.SpaceView {
	return rest.SpaceView{
		Space: rest.Space{
			Id: rest.Uuid(s.ID), Name: s.Name, Icon: optionalString(s.Icon), Color: optionalString(s.Color), SortKey: s.SortKey,
			DefaultOperatorAgentId: optionalID(s.DefaultOperatorAgentID), ArchivedAt: optionalTime(s.ArchivedAt),
			CreatedAt: s.CreatedAt.UTC().Truncate(time.Millisecond), Version: s.Version,
		},
		Level: rest.Level(s.Level), Capabilities: capabilities(s.Capabilities),
	}
}

// channelViewREST converts a channel of the snapshot (schema ChannelView).
func channelViewREST(c sync.SnapshotChannel) rest.ChannelView {
	ch := rest.Channel{
		Id: rest.Uuid(c.ID), SpaceId: optionalID(c.SpaceID), Kind: rest.ChannelKind(c.Kind), Name: c.Name, Topic: c.Topic, SortKey: c.SortKey,
		Operator: rest.ChannelOperator{Mode: rest.OperatorMode(c.OperatorMode), AgentId: optionalID(c.OperatorAgentID),
			EffectiveAgentId: optionalID(c.EffectiveOperatorID)},
		ReadOnly: c.ReadOnly, MainThreadId: optionalID(c.MainThreadID), ArchivedAt: optionalTime(c.ArchivedAt),
		CreatedAt: c.CreatedAt.UTC().Truncate(time.Millisecond), Version: c.Version, SettingsVersion: c.SettingsVersion,
	}
	if c.ForumDefaultSort != "" {
		s := rest.ForumSort(c.ForumDefaultSort)
		ch.ForumDefaultSort = &s
	}
	if c.DefaultNotificationLevel != "" {
		l := rest.NotificationLevel(c.DefaultNotificationLevel)
		ch.DefaultNotificationLevel = &l
	}
	return rest.ChannelView{Channel: ch, Level: rest.Level(c.Level), Capabilities: capabilities(c.Capabilities)}
}

// sidebarREST converts a sidebar state (schema SidebarState).
func sidebarREST(st sync.SidebarState) rest.SidebarState {
	return rest.SidebarState{SpaceId: rest.Uuid(st.SpaceID), Collapsed: st.Collapsed, SortKey: optionalString(st.SortKey), Version: st.Version,
		UpdatedByDeviceId: optionalID(st.UpdatedByDeviceID)}
}

// runREST converts a running run (schema RunSummary).
func runREST(r sync.RunSummary) rest.RunSummary {
	out := rest.RunSummary{
		Id: rest.Uuid(r.ID), ThreadId: rest.Uuid(r.ThreadID), ChannelId: rest.Uuid(r.ChannelID), AgentId: rest.Uuid(r.AgentID),
		TriggeredBy: rest.Uuid(r.TriggeredBy), State: rest.RunState(r.State), StateReason: optionalString(r.StateReason), Version: r.Version,
		BillingModeExpected: rest.BillingMode(r.BillingModeExpected), StatusLine: optionalString(r.StatusLine), Summary: optionalString(r.Summary),
		Cost:      rest.Money{AmountMicros: r.Cost.AmountMicros, Currency: r.Cost.Currency, IsEstimate: r.Cost.IsEstimate},
		StepCount: r.StepCount, QueuedAt: r.QueuedAt.UTC().Truncate(time.Millisecond), StartedAt: optionalTime(r.StartedAt),
		FinishedAt: optionalTime(r.FinishedAt), StallWarnedAt: optionalTime(r.StallWarnedAt),
	}
	if r.BillingModeUsed != "" {
		m := rest.BillingMode(r.BillingModeUsed)
		out.BillingModeUsed = &m
	}
	if r.DurationMS != 0 {
		d := r.DurationMS
		out.DurationMs = &d
	}
	return out
}

// capabilities converts capability names; never nil.
func capabilities(list []string) []rest.Capability {
	out := make([]rest.Capability, 0, len(list))
	for _, c := range list {
		out = append(out, rest.Capability(c))
	}
	return out
}

// optionalString returns nil for "".
func optionalString(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
