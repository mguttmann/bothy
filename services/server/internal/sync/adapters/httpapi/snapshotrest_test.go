// SPDX-License-Identifier: Apache-2.0

package httpapi_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	bsync "github.com/OWNER/bothy/services/server/internal/sync"
	"github.com/OWNER/bothy/services/server/internal/sync/adapters/httpapi"
)

// TestSnapshotRESTConvertsEveryOptionalMember checks the members a seeded snapshot of the
// integration tests rarely carries: a running run with billing mode and duration, a forum channel
// with sort and notification level, an agent with a reported billing mode and a feature flag.
func TestSnapshotRESTConvertsEveryOptionalMember(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 10, 20, 12, 0, 0, 123456789, time.UTC)
	run := bsync.RunSummary{ID: ids.New(), ThreadID: ids.New(), ChannelID: ids.New(), AgentID: ids.New(), TriggeredBy: ids.New(),
		State: "running", StateReason: "tool_call", Version: 3, BillingModeExpected: "api", BillingModeUsed: "subscription",
		StatusLine: "reading files", Summary: "summary", Cost: bsync.Money{AmountMicros: 1500, Currency: "USD", IsEstimate: true},
		StepCount: 4, DurationMS: 2500, QueuedAt: now, StartedAt: now.Add(time.Second)}
	v := bsync.SnapshotView{
		HeadSeq: 7, SyncEpoch: ids.New(), PermissionVersion: 2,
		Principal: bsync.SnapshotPrincipal{ID: ids.New(), Kind: "user", Handle: "ada", DisplayName: "Ada"},
		Spaces:    []bsync.SnapshotSpace{{ID: ids.New(), Name: "Space", SortKey: "a", CreatedAt: now, Level: "read"}},
		Channels: []bsync.SnapshotChannel{{ID: ids.New(), Kind: "forum", Name: "ideas", SortKey: "a", OperatorMode: "inherit",
			ForumDefaultSort: "latest_activity", DefaultNotificationLevel: "mentions", CreatedAt: now, Level: "write",
			Capabilities: []string{"message.send"}}},
		Sidebar:         []bsync.SidebarState{{SpaceID: ids.New(), Collapsed: true, SortKey: "b", Version: 1}, {SpaceID: ids.New(), ChannelOrder: []ids.ID{ids.New()}, Version: 2}},
		SidebarChannels: []bsync.SidebarChannelState{{ChannelID: ids.New(), Version: 1}},
		UnreadThreads:   []bsync.UnreadThread{{ThreadID: ids.New(), ChannelID: ids.New(), UnreadCount: 2, LastMessageSeq: 5}},
		ActiveRuns:      []bsync.RunSummary{run},
		Agents:          []bsync.AgentPresence{{AgentID: ids.New(), State: "online", ReportedBillingMode: "api"}, {AgentID: ids.New(), State: "offline"}},
		Settings:        []bsync.UserSetting{{Key: "layout.column_widths", Value: json.RawMessage(`[240,320]`), Version: 1}},
		Drafts:          []bsync.DraftRef{{ID: ids.New(), ChannelID: ids.New(), Version: 1}},
		Features:        []bsync.EvaluatedFlag{{Key: "personal_tokens", Enabled: true}},
	}
	out, err := httpapi.SnapshotREST(v)
	if err != nil {
		t.Fatal(err)
	}
	r := out.ActiveRuns[0]
	if r.BillingModeUsed == nil || string(*r.BillingModeUsed) != "subscription" || r.DurationMs == nil || *r.DurationMs != 2500 {
		t.Fatalf("run: billing mode used %v, duration %v", r.BillingModeUsed, r.DurationMs)
	}
	if r.StateReason == nil || *r.StateReason != "tool_call" || r.FinishedAt != nil || r.StartedAt == nil || !r.QueuedAt.Equal(now.Truncate(time.Millisecond)) {
		t.Fatalf("run: state reason %v, started %v, finished %v, queued %v", r.StateReason, r.StartedAt, r.FinishedAt, r.QueuedAt)
	}
	if !r.Cost.IsEstimate || r.Cost.AmountMicros != 1500 || r.Cost.Currency != "USD" {
		t.Fatalf("run cost %+v", r.Cost)
	}
	c := out.Channels[0].Channel
	if c.ForumDefaultSort == nil || string(*c.ForumDefaultSort) != "latest_activity" || c.DefaultNotificationLevel == nil ||
		string(*c.DefaultNotificationLevel) != "mentions" || c.SpaceId != nil {
		t.Fatalf("channel: sort %v, level %v, space %v", c.ForumDefaultSort, c.DefaultNotificationLevel, c.SpaceId)
	}
	if len(out.Channels[0].Capabilities) != 1 || out.Spaces[0].Capabilities == nil {
		t.Fatalf("capabilities: channel %v, space %v", out.Channels[0].Capabilities, out.Spaces[0].Capabilities)
	}
	if a := out.Agents[0]; a.ReportedBillingMode == nil || string(*a.ReportedBillingMode) != "api" {
		t.Fatalf("agent billing mode %v", a.ReportedBillingMode)
	}
	if out.Agents[1].ReportedBillingMode != nil {
		t.Fatalf("agent without a billing mode: %v", out.Agents[1].ReportedBillingMode)
	}
	if len(out.Features) != 1 || !out.Features[0].Enabled || len(out.Drafts) != 1 || out.Drafts[0].ThreadId != nil {
		t.Fatalf("features %v, drafts %v", out.Features, out.Drafts)
	}
	if len(out.Sidebar) != 2 || out.Sidebar[0].SortKey == nil || len(out.UnreadThreads) != 1 || len(out.Settings) != 1 {
		t.Fatalf("sidebar %v, unread %v, settings %v", out.Sidebar, out.UnreadThreads, out.Settings)
	}
	// M4-51: a nil channel order is absent, a list is carried; thread_order is always an array.
	if out.Sidebar[0].ChannelOrder != nil || out.Sidebar[1].ChannelOrder == nil || len(*out.Sidebar[1].ChannelOrder) != 1 {
		t.Fatalf("channel_order %v, %v", out.Sidebar[0].ChannelOrder, out.Sidebar[1].ChannelOrder)
	}
	if out.SidebarChannels == nil || len(*out.SidebarChannels) != 1 || (*out.SidebarChannels)[0].ThreadOrder == nil {
		t.Fatalf("sidebar_channels %v", out.SidebarChannels)
	}
}

// TestSnapshotRESTRejectsAStoredSettingThatIsNotJSON checks the internal error for a damaged
// stored setting.
func TestSnapshotRESTRejectsAStoredSettingThatIsNotJSON(t *testing.T) {
	t.Parallel()
	_, err := httpapi.SnapshotREST(bsync.SnapshotView{Settings: []bsync.UserSetting{{Key: "theme", Value: json.RawMessage(`{`)}}})
	if err == nil {
		t.Fatal("a setting that is not JSON was converted")
	}
}
