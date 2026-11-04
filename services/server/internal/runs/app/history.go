// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"slices"
	"time"

	"github.com/OWNER/bothy/services/server/internal/chat"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/runs"
)

// listRunHistory is the list name of the history cursor (06 "Pagination").
const listRunHistory = "runs.history"

// HistoryPlaces is the part of chat.Structure the run history reads (F-LAUFHISTORIE): the
// channels of a space and every thread of channels. It never authorizes; the use case passes
// only channels of the caller's Scope.
type HistoryPlaces interface {
	ChannelIDsOfSpace(ctx context.Context, spaceID ids.ID) ([]ids.ID, error)
	ThreadIDsOfChannels(ctx context.Context, channelIDs []ids.ID) ([]ids.ID, error)
}

// HistoryChats is the part of chat.StatsSource that names the chats of the run history: the
// channel name and thread title of threadIDs whose channel is in channelIDs.
type HistoryChats interface {
	StatsRunChats(ctx context.Context, channelIDs, threadIDs []ids.ID) ([]chat.StatsRunChat, error)
}

// historyKey is the cursor key of listRunHistory: the keyset (finished_at DESC, id DESC) of the
// last examined row.
type historyKey struct {
	FinishedAt time.Time `json:"f"`
	ID         ids.ID    `json:"i"`
}

// endStates are the states of the history filter (schema parameter state of listRunHistory).
var endStates = []string{"completed", "failed", "cancelled"}

// ListHistory implements runs.Runs (listRunHistory, docs/spec/25-visualization.md 6.7): the
// pipeline of ListActive over the finished runs. run.overview.read on the instance and
// Scope(run.read) come first; the visible channels, narrowed to the channels of q.SpaceID, are
// resolved to their threads through chat, the repository reads the finished runs of those threads
// in batches by finished_at descending, and every row is resolved again through chat.Locate, kept
// when its channel is visible and authorized with run.read in filter mode until the page is full.
// run.read on a run is the read right on its thread, so the entry names its chat (25 4 rule 6).
func (s *Service) ListHistory(ctx context.Context, q runs.HistoryQuery) (runs.HistoryPage, error) {
	a := caller(ctx)
	d, err := s.d.Authorizer.Authorize(ctx, a, authorize.ActionRunOverviewRead, authorize.Resource{Kind: authorize.ResourceInstance})
	if err != nil {
		return runs.HistoryPage{}, err
	}
	if err := d.Err(); err != nil {
		return runs.HistoryPage{}, err
	}
	if q.State != "" && !slices.Contains(endStates, q.State) {
		return runs.HistoryPage{}, errs.New(errs.CodeValidationFailed, "runs: the history lists only ended runs")
	}
	if s.d.History == nil {
		return runs.HistoryPage{}, errs.New(errs.CodeInternalError, "runs: the run history has no chat places")
	}
	sc, err := s.d.Authorizer.Scope(ctx, a, authorize.ActionRunRead)
	if err != nil {
		return runs.HistoryPage{}, err
	}
	binding := []byte(a.ID.String() + "|" + optionalID(q.SpaceID) + "|" + optionalID(q.AgentID) + "|" + q.State)
	after, err := s.historyCursor(binding, q.Cursor)
	if err != nil {
		return runs.HistoryPage{}, err
	}
	out := runs.HistoryPage{Items: []runs.HistoryEntry{}}
	visible, err := s.historyChannels(ctx, sc.ChannelIDs, q.SpaceID)
	if err != nil || len(visible) == 0 {
		return out, err
	}
	threads, err := s.d.History.ThreadIDsOfChannels(ctx, visible)
	if err != nil || len(threads) == 0 {
		return out, err
	}
	read := func(after *RunKey, n int) ([]RunRow, error) {
		return s.d.Repo.HistoryRuns(ctx, HistoryRead{ThreadIDs: threads, AgentID: q.AgentID, State: q.State, After: after, Limit: n})
	}
	kept, next, err := s.fill(ctx, a, s.pageSize(q.Limit), after, read, s.locate(ctx), visible)
	if err != nil {
		return runs.HistoryPage{}, err
	}
	if out.Items, err = s.historyEntries(ctx, kept, visible); err != nil {
		return runs.HistoryPage{}, err
	}
	if next != nil {
		if out.NextCursor, err = s.d.Cursors.Encode(listRunHistory, binding, historyKey{FinishedAt: next.FinishedAt, ID: next.ID}); err != nil {
			return runs.HistoryPage{}, errs.Wrap(errs.CodeInternalError, err, "runs: run history cursor")
		}
	}
	return out, nil
}

// historyCursor decodes the history cursor; nil for none. Errors: 400 cursor_invalid.
func (s *Service) historyCursor(binding []byte, cursor string) (*RunKey, error) {
	if cursor == "" {
		return nil, nil
	}
	var k historyKey
	if err := s.d.Cursors.Decode(listRunHistory, binding, cursor, &k); err != nil {
		return nil, err
	}
	if k.FinishedAt.IsZero() || k.ID.IsZero() {
		return nil, errs.New(errs.CodeCursorInvalid, "")
	}
	return &RunKey{FinishedAt: k.FinishedAt, ID: k.ID}, nil
}

// historyChannels returns the visible channels in the order of Scope.ChannelIDs, narrowed to the
// channels of spaceID unless it is zero; an unknown or invisible space gives none.
func (s *Service) historyChannels(ctx context.Context, visible []ids.ID, spaceID ids.ID) ([]ids.ID, error) {
	if spaceID.IsZero() || len(visible) == 0 {
		return visible, nil
	}
	of, err := s.d.History.ChannelIDsOfSpace(ctx, spaceID)
	if err != nil {
		return nil, err
	}
	out := make([]ids.ID, 0, len(of))
	for _, ch := range visible {
		if slices.Contains(of, ch) {
			out = append(out, ch)
		}
	}
	return out, nil
}

// historyEntries converts the kept rows with the step counts and the chats of the page.
func (s *Service) historyEntries(ctx context.Context, kept []located, visible []ids.ID) ([]runs.HistoryEntry, error) {
	sums, err := s.summaries(ctx, kept)
	if err != nil {
		return nil, err
	}
	chats, err := s.historyChats(ctx, kept, visible)
	if err != nil {
		return nil, err
	}
	out := make([]runs.HistoryEntry, 0, len(kept))
	for i, k := range kept {
		e := runs.HistoryEntry{RunSummary: sums[i], SpaceID: k.info.SpaceID, Model: k.row.Model, Metrics: object(k.row.Metrics)}
		if c, ok := chats[k.row.ThreadID]; ok {
			e.Chat = &runs.HistoryChat{ThreadID: c.ThreadID, ChannelName: c.ChannelName, ThreadTitle: c.ThreadTitle}
		}
		out = append(out, e)
	}
	return out, nil
}

// historyChats names the chats of the kept rows through Chats, restricted to the visible
// channels; none without Chats.
func (s *Service) historyChats(ctx context.Context, kept []located, visible []ids.ID) (map[ids.ID]chat.StatsRunChat, error) {
	out := map[ids.ID]chat.StatsRunChat{}
	if s.d.Chats == nil || len(kept) == 0 {
		return out, nil
	}
	threads := make([]ids.ID, 0, len(kept))
	for _, k := range kept {
		if !slices.Contains(threads, k.row.ThreadID) {
			threads = append(threads, k.row.ThreadID)
		}
	}
	list, err := s.d.Chats.StatsRunChats(ctx, visible, threads)
	if err != nil {
		return nil, err
	}
	for _, c := range list {
		out[c.ThreadID] = c
	}
	return out, nil
}
