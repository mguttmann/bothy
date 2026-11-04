// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"bytes"
	"context"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/chat"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/runs"
	"github.com/OWNER/bothy/services/server/internal/runs/app"
	"github.com/OWNER/bothy/services/server/internal/runs/domain"
)

// places is the fake app.HistoryPlaces over the thread map of the world: the channels of a space
// and the threads of channels, as chat.Structure answers them.
type places struct {
	t     *threads
	mu    sync.Mutex
	calls int
}

func (p *places) ChannelIDsOfSpace(_ context.Context, spaceID ids.ID) ([]ids.ID, error) {
	p.t.mu.Lock()
	defer p.t.mu.Unlock()
	var out []ids.ID
	for _, in := range p.t.infos {
		if in.SpaceID == spaceID && !in.SpaceID.IsZero() && !slices.Contains(out, in.ChannelID) {
			out = append(out, in.ChannelID)
		}
	}
	slices.SortFunc(out, func(a, b ids.ID) int { return bytes.Compare(a[:], b[:]) })
	return out, nil
}

func (p *places) ThreadIDsOfChannels(_ context.Context, channelIDs []ids.ID) ([]ids.ID, error) {
	p.mu.Lock()
	p.calls++
	p.mu.Unlock()
	p.t.mu.Lock()
	defer p.t.mu.Unlock()
	var out []ids.ID
	for id, in := range p.t.infos {
		if slices.Contains(channelIDs, in.ChannelID) {
			out = append(out, id)
		}
	}
	slices.SortFunc(out, func(a, b ids.ID) int { return bytes.Compare(a[:], b[:]) })
	return out, nil
}

// chatNames is the fake app.HistoryChats: a name per channel and a title per thread; it answers
// only threads of the channels passed, as chat.StatsSource does.
type chatNames struct {
	t        *threads
	channel  map[ids.ID]string
	title    map[ids.ID]string
	channels [][]ids.ID
}

func (c *chatNames) StatsRunChats(_ context.Context, channelIDs, threadIDs []ids.ID) ([]chat.StatsRunChat, error) {
	c.channels = append(c.channels, slices.Clone(channelIDs))
	var out []chat.StatsRunChat
	for _, th := range threadIDs {
		in, ok := c.t.infos[th]
		if !ok || !slices.Contains(channelIDs, in.ChannelID) {
			continue
		}
		out = append(out, chat.StatsRunChat{ThreadID: th, ChannelID: in.ChannelID, ChannelName: c.channel[in.ChannelID], ThreadTitle: c.title[th]})
	}
	return out, nil
}

// historyWorld is the world with the history ports bound.
func historyWorld(t *testing.T) (*world, *places, *chatNames) {
	t.Helper()
	w := newWorld(t)
	p := &places{t: w.threads}
	c := &chatNames{t: w.threads, channel: map[ids.ID]string{w.channel: "backend"}, title: map[ids.ID]string{}}
	d := w.deps()
	d.History, d.Chats = p, c
	svc, err := app.New(w.config(), d)
	if err != nil {
		t.Fatal(err)
	}
	w.svc = svc
	return w, p, c
}

// historyRun puts a run of agent in a new thread of channel in space that ended in state at t0
// plus i minutes, with metrics and a model, and returns it.
func (w *world) historyRun(channel, space, agent ids.ID, state domain.State, i int) app.RunRow {
	th := ids.New()
	w.threads.mu.Lock()
	w.threads.infos[th] = chat.ThreadInfo{ID: th, ChannelID: channel, SpaceID: space, ChannelKind: "text", Kind: "thread", State: "open"}
	w.threads.mu.Unlock()
	start := t0.Add(time.Duration(i) * time.Minute)
	row := app.RunRow{
		ID: ids.New(), ThreadID: th, AgentID: agent, TriggeredBy: w.alex, TriggerKind: "message", State: state, Version: 4,
		BillingModeExpected: "personal", BillingModeUsed: "personal", Model: "model-large", Summary: "done",
		Metrics: []byte(`{"tokens_in":1200,"tokens_out":300,"tokens_cache_read":9000,"tokens_cache_write":40,"subagents":7}`),
		Plan:    []byte(`{}`), BillingEvidence: []byte(`{}`), CostMicros: 420000, Currency: "USD", CostIsEstimate: true,
		QueuedAt: start, StartedAt: start.Add(time.Second), FinishedAt: start.Add(39 * time.Minute),
		DurationMS: (39*time.Minute - time.Second).Milliseconds(), HasDuration: true,
	}
	w.repo.mu.Lock()
	w.repo.runs = append(w.repo.runs, row)
	w.repo.mu.Unlock()
	return row
}

// TestListHistory_NewestEndFirstAcrossPages: only finished runs of visible channels, newest end
// first, with metrics, model, space and chat; the cursor continues after the last examined row
// and repeats nothing.
func TestListHistory_NewestEndFirstAcrossPages(t *testing.T) {
	t.Parallel()
	w, _, names := historyWorld(t)
	other := ids.New() // a channel outside the scope
	var visible []app.RunRow
	for i := 1; i <= 6; i++ {
		ch := w.channel
		if i%2 == 0 {
			ch = other
		}
		r := w.historyRun(ch, w.space, w.lotse, domain.StateCompleted, i)
		if ch == w.channel {
			visible = append(visible, r)
		}
	}
	w.activeRun(w.channel, 99) // active: never in the history
	page, err := w.svc.ListHistory(w.asAlex(), runs.HistoryQuery{Limit: 2})
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Items) != 2 || page.Items[0].ID != visible[2].ID || page.Items[1].ID != visible[1].ID || page.NextCursor == "" {
		t.Fatalf("page 1 %+v", page)
	}
	e := page.Items[0]
	if e.SpaceID != w.space || e.ChannelID != w.channel || e.Model != "model-large" || e.Metrics["subagents"] != float64(7) ||
		e.Metrics["tokens_cache_write"] != float64(40) || e.Cost.AmountMicros != 420000 || !e.HasDuration || e.Summary != "done" {
		t.Fatalf("entry %+v", e)
	}
	if e.Chat == nil || e.Chat.ThreadID != e.ThreadID || e.Chat.ChannelName != "backend" {
		t.Fatalf("chat %+v", e.Chat)
	}
	// The chat names are asked only for the visible channels.
	if len(names.channels) != 1 || !slices.Equal(names.channels[0], []ids.ID{w.channel}) {
		t.Fatalf("chat channels %v", names.channels)
	}
	next, err := w.svc.ListHistory(w.asAlex(), runs.HistoryQuery{Limit: 2, Cursor: page.NextCursor})
	if err != nil {
		t.Fatal(err)
	}
	if len(next.Items) != 1 || next.Items[0].ID != visible[0].ID || next.NextCursor != "" {
		t.Fatalf("page 2 %+v", next)
	}
	// A cursor of another filter is cursor_invalid.
	_, err = w.svc.ListHistory(w.asAlex(), runs.HistoryQuery{Limit: 2, Cursor: page.NextCursor, State: "failed"})
	if e, ok := errs.As(err); !ok || e.Code != errs.CodeCursorInvalid {
		t.Fatalf("foreign cursor %v", err)
	}
}

// TestListHistory_Filters: the space, agent and state filters; an unknown space or agent gives an
// empty page, an unknown state validation_failed.
func TestListHistory_Filters(t *testing.T) {
	t.Parallel()
	w, _, _ := historyWorld(t)
	otherSpace, otherChannel := ids.New(), ids.New()
	w.authz.scope = append(w.authz.scope, otherChannel)
	done := w.historyRun(w.channel, w.space, w.lotse, domain.StateCompleted, 1)
	failed := w.historyRun(w.channel, w.space, w.texter, domain.StateFailed, 2)
	elsewhere := w.historyRun(otherChannel, otherSpace, w.lotse, domain.StateCancelled, 3)
	ask := func(q runs.HistoryQuery) []ids.ID {
		t.Helper()
		page, err := w.svc.ListHistory(w.asAlex(), q)
		if err != nil {
			t.Fatal(err)
		}
		var out []ids.ID
		for _, e := range page.Items {
			out = append(out, e.ID)
		}
		return out
	}
	if got := ask(runs.HistoryQuery{}); !slices.Equal(got, []ids.ID{elsewhere.ID, failed.ID, done.ID}) {
		t.Fatalf("all %v", got)
	}
	if got := ask(runs.HistoryQuery{SpaceID: w.space}); !slices.Equal(got, []ids.ID{failed.ID, done.ID}) {
		t.Fatalf("space %v", got)
	}
	if got := ask(runs.HistoryQuery{SpaceID: otherSpace}); !slices.Equal(got, []ids.ID{elsewhere.ID}) {
		t.Fatalf("other space %v", got)
	}
	if got := ask(runs.HistoryQuery{AgentID: w.texter}); !slices.Equal(got, []ids.ID{failed.ID}) {
		t.Fatalf("agent %v", got)
	}
	if got := ask(runs.HistoryQuery{State: "cancelled"}); !slices.Equal(got, []ids.ID{elsewhere.ID}) {
		t.Fatalf("state %v", got)
	}
	if got := ask(runs.HistoryQuery{SpaceID: ids.New()}); len(got) != 0 {
		t.Fatalf("unknown space %v", got)
	}
	if got := ask(runs.HistoryQuery{AgentID: ids.New()}); len(got) != 0 {
		t.Fatalf("unknown agent %v", got)
	}
	_, err := w.svc.ListHistory(w.asAlex(), runs.HistoryQuery{State: "running"})
	if e, ok := errs.As(err); !ok || e.Code != errs.CodeValidationFailed {
		t.Fatalf("active state %v", err)
	}
}

// TestListHistory_Permissions: without run.overview.read the list is refused before any read; a
// run the channel filter keeps and run.read refuses is dropped as a filter mismatch, checked in
// filter mode, and the log names no id.
func TestListHistory_Permissions(t *testing.T) {
	t.Parallel()
	w, p, _ := historyWorld(t)
	keep := w.historyRun(w.channel, w.space, w.lotse, domain.StateCompleted, 1)
	drop := w.historyRun(w.channel, w.space, w.lotse, domain.StateCompleted, 2)
	w.authz.decide = func(act authorize.Action, _ bool, r authorize.Resource) decision {
		if act == authorize.ActionRunRead && r.ID == drop.ID {
			return decision{reason: "no_grant"}
		}
		return decision{allowed: true}
	}
	page, err := w.svc.ListHistory(w.asAlex(), runs.HistoryQuery{})
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Items) != 1 || page.Items[0].ID != keep.ID {
		t.Fatalf("page %+v", page)
	}
	if w.errs.n["filter_mismatch"] != 1 || strings.Contains(w.log.String(), drop.ID.String()) {
		t.Fatalf("mismatch %v %s", w.errs.n, w.log.String())
	}
	for _, c := range w.j.list() {
		if c.what == "authz:run.read:request" {
			t.Fatal("the per item check ran in request mode")
		}
	}
	w.authz.decide = func(act authorize.Action, _ bool, _ authorize.Resource) decision {
		return decision{allowed: act != authorize.ActionRunOverviewRead, reason: "no_grant"}
	}
	before := p.calls
	if _, err := w.svc.ListHistory(w.asAlex(), runs.HistoryQuery{}); errs.KindOf(err) != errs.KindForbidden {
		t.Fatalf("without run.overview.read %v", err)
	}
	if p.calls != before {
		t.Fatal("the threads were read before the check")
	}
}

// TestListHistory_NoChatSource: without a chat source the entries carry no chat; without the
// places the list is an internal error, never an unfiltered read.
func TestListHistory_NoChatSource(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	p := &places{t: w.threads}
	d := w.deps()
	d.History = p
	svc, err := app.New(w.config(), d)
	if err != nil {
		t.Fatal(err)
	}
	w.svc = svc
	w.historyRun(w.channel, w.space, w.lotse, domain.StateCompleted, 1)
	page, err := w.svc.ListHistory(w.asAlex(), runs.HistoryQuery{})
	if err != nil || len(page.Items) != 1 || page.Items[0].Chat != nil {
		t.Fatalf("page %+v %v", page, err)
	}
	d.History = nil
	bare, err := app.New(w.config(), d)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := bare.ListHistory(w.asAlex(), runs.HistoryQuery{}); errs.KindOf(err) != errs.KindInternal {
		t.Fatalf("without places %v", err)
	}
}
