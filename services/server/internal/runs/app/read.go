// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"log/slog"
	"slices"
	"time"

	"github.com/OWNER/bothy/services/server/internal/chat"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/runs"
)

// The list names of the cursors (06 "Pagination": a cursor is bound to its list, filters and
// principal).
const (
	listThreadRuns = "runs.thread"
	listRunSteps   = "runs.steps"
	listActiveRuns = "runs.active"
)

// msgFilterMismatch is the log line of a run the channel filter kept and run.read refused (07
// section 7.4 rule 3); it names only the action and the resource kind.
const (
	msgFilterMismatch    = "runs: a run of a visible channel failed run.read (resource kind run)"
	reasonFilterMismatch = "filter_mismatch"
)

// runKey is the cursor key of the run lists: the keyset (queued_at DESC, id DESC) of the last
// examined row.
type runKey struct {
	QueuedAt time.Time `json:"q"`
	ID       ids.ID    `json:"i"`
}

// stepKey is the cursor key of listRunSteps: the ordinal of the last step of the page.
type stepKey struct {
	Ordinal int `json:"o"`
}

// runResource describes a run for authorize (brief note 8): its thread as the path hint and the
// person who triggered it as owner; authz resolves the thread's path at decision time.
func runResource(r RunRow) authorize.Resource {
	return authorize.Resource{Kind: authorize.ResourceRun, ID: r.ID, ThreadID: r.ThreadID, OwnerID: r.TriggeredBy}
}

// pageSize returns the page size of a request: api.page_size_default for none, capped at
// api.page_size_max.
func (s *Service) pageSize(limit int) int {
	if limit <= 0 {
		return s.cfg.PageSizeDefault
	}
	return min(limit, s.cfg.PageSizeMax)
}

// readRun loads the run id and authorizes run.read on it. An unknown run is checked against a
// path that cannot resolve (its id as thread hint), so it costs about the same as an invisible
// one and answers the same not_found value (02 section 9.2, AZ-P4).
func (s *Service) readRun(ctx context.Context, a actor.Actor, id ids.ID) (RunRow, error) {
	row, found, err := s.d.Repo.Run(ctx, id)
	if err != nil {
		return RunRow{}, err
	}
	res := authorize.Resource{Kind: authorize.ResourceRun, ID: id, ThreadID: id}
	if found {
		res = runResource(row)
	}
	d, err := s.d.Authorizer.Authorize(ctx, a, authorize.ActionRunRead, res)
	if err != nil {
		return RunRow{}, err
	}
	// The answer of the principal comes first, so a pending device or person gets the same answer
	// for an unknown and an existing run (07 4.2, AZ-P4).
	if d.Outcome != authorize.OutcomeAllow && d.Outcome != authorize.OutcomeNotFound {
		return RunRow{}, d.Err()
	}
	if !found || d.Outcome == authorize.OutcomeNotFound {
		return RunRow{}, invisible()
	}
	return row, nil
}

// Get implements runs.Runs (getRun, brief note 8): run.read on the run, then the document with
// all steps and the channel of its thread now.
func (s *Service) Get(ctx context.Context, runID ids.ID) (runs.Run, error) {
	row, err := s.readRun(ctx, caller(ctx), runID)
	if err != nil {
		return runs.Run{}, err
	}
	return s.documentOf(ctx, row)
}

// ListSteps implements runs.Runs (listRunSteps): the checks of Get, then a keyset page by ordinal
// with a cursor bound to the run and the principal.
func (s *Service) ListSteps(ctx context.Context, q runs.RunStepsQuery) (runs.RunStepPage, error) {
	a := caller(ctx)
	row, err := s.readRun(ctx, a, q.RunID)
	if err != nil {
		return runs.RunStepPage{}, err
	}
	binding := []byte(a.ID.String() + "|" + row.ID.String())
	var after stepKey
	if q.Cursor != "" {
		if err := s.d.Cursors.Decode(listRunSteps, binding, q.Cursor, &after); err != nil {
			return runs.RunStepPage{}, err
		}
		if after.Ordinal < 1 {
			return runs.RunStepPage{}, errs.New(errs.CodeCursorInvalid, "")
		}
	}
	limit := s.pageSize(q.Limit)
	rows, err := s.d.Repo.Steps(ctx, StepsRead{RunID: row.ID, AfterOrdinal: after.Ordinal, Limit: limit + 1})
	if err != nil {
		return runs.RunStepPage{}, err
	}
	out := runs.RunStepPage{Items: make([]runs.RunStep, 0, min(len(rows), limit))}
	for _, r := range rows[:min(len(rows), limit)] {
		out.Items = append(out.Items, step(r))
	}
	if len(rows) > limit {
		last := out.Items[len(out.Items)-1]
		if out.NextCursor, err = s.d.Cursors.Encode(listRunSteps, binding, stepKey{Ordinal: last.Ordinal}); err != nil {
			return runs.RunStepPage{}, errs.Wrap(errs.CodeInternalError, err, "runs: step list cursor")
		}
	}
	return out, nil
}

// ListByThread implements runs.Runs (listThreadRuns, brief note 8): Scope(run.read) proves the
// route action (02 section 9.3); a thread whose channel is not visible gives the same not_found as
// an unknown one; then the runs of the thread newest first, each checked with run.read in filter
// mode.
func (s *Service) ListByThread(ctx context.Context, q runs.ThreadRunsQuery) (runs.RunSummaryPage, error) {
	a := caller(ctx)
	sc, err := s.d.Authorizer.Scope(ctx, a, authorize.ActionRunRead)
	if err != nil {
		return runs.RunSummaryPage{}, err
	}
	info, err := s.d.Threads.Info(ctx, q.ThreadID)
	if err != nil {
		if errs.KindOf(err) == errs.KindNotFound {
			return runs.RunSummaryPage{}, invisible()
		}
		return runs.RunSummaryPage{}, err
	}
	if _, ok := slices.BinarySearchFunc(sc.ChannelIDs, info.ChannelID, compareIDs); !ok {
		return runs.RunSummaryPage{}, invisible()
	}
	binding := []byte(a.ID.String() + "|" + info.ID.String())
	after, err := s.runCursor(listThreadRuns, binding, q.Cursor)
	if err != nil {
		return runs.RunSummaryPage{}, err
	}
	limit := s.pageSize(q.Limit)
	out := runs.RunSummaryPage{Items: []runs.RunSummary{}}
	read := func(after *RunKey, n int) ([]RunRow, error) {
		return s.d.Repo.ThreadRuns(ctx, ThreadRunsRead{ThreadID: info.ID, After: after, Limit: n})
	}
	kept, next, err := s.fill(ctx, a, limit, after, read, func([]RunRow) (map[ids.ID]chat.ThreadInfo, error) {
		return map[ids.ID]chat.ThreadInfo{info.ID: info}, nil
	}, sc.ChannelIDs)
	if err != nil {
		return runs.RunSummaryPage{}, err
	}
	if out.Items, err = s.summaries(ctx, kept); err != nil {
		return runs.RunSummaryPage{}, err
	}
	if next != nil {
		if out.NextCursor, err = s.d.Cursors.Encode(listThreadRuns, binding, runKey{QueuedAt: next.QueuedAt, ID: next.ID}); err != nil {
			return runs.RunSummaryPage{}, errs.Wrap(errs.CodeInternalError, err, "runs: thread run list cursor")
		}
	}
	return out, nil
}

// ListActive implements runs.Runs (listActiveRuns, brief note 8, D-M2-05-11): run.overview.read on
// the instance, Scope(run.read), then batches of active runs in keyset order whose threads are
// resolved through chat, kept when the channel is visible and run.read allows each in filter mode,
// until the page is full. next_cursor is the last examined row when the page is full and another
// row exists. An unknown or invisible agent_id gives an empty page.
func (s *Service) ListActive(ctx context.Context, q runs.ActiveRunsQuery) (runs.RunSummaryPage, error) {
	a := caller(ctx)
	d, err := s.d.Authorizer.Authorize(ctx, a, authorize.ActionRunOverviewRead, authorize.Resource{Kind: authorize.ResourceInstance})
	if err != nil {
		return runs.RunSummaryPage{}, err
	}
	if err := d.Err(); err != nil {
		return runs.RunSummaryPage{}, err
	}
	sc, err := s.d.Authorizer.Scope(ctx, a, authorize.ActionRunRead)
	if err != nil {
		return runs.RunSummaryPage{}, err
	}
	binding := []byte(a.ID.String() + "|" + optionalID(q.AgentID))
	after, err := s.runCursor(listActiveRuns, binding, q.Cursor)
	if err != nil {
		return runs.RunSummaryPage{}, err
	}
	limit := s.pageSize(q.Limit)
	out := runs.RunSummaryPage{Items: []runs.RunSummary{}}
	if len(sc.ChannelIDs) == 0 {
		return out, nil
	}
	read := func(after *RunKey, n int) ([]RunRow, error) {
		return s.d.Repo.ActiveRuns(ctx, ActiveRunsRead{AgentID: q.AgentID, After: after, Limit: n})
	}
	kept, next, err := s.fill(ctx, a, limit, after, read, s.locate(ctx), sc.ChannelIDs)
	if err != nil {
		return runs.RunSummaryPage{}, err
	}
	if out.Items, err = s.summaries(ctx, kept); err != nil {
		return runs.RunSummaryPage{}, err
	}
	if next != nil {
		if out.NextCursor, err = s.d.Cursors.Encode(listActiveRuns, binding, runKey{QueuedAt: next.QueuedAt, ID: next.ID}); err != nil {
			return runs.RunSummaryPage{}, errs.Wrap(errs.CodeInternalError, err, "runs: active run list cursor")
		}
	}
	return out, nil
}

// ActiveForActor implements runs.Runs: the pipeline of ListActive without paging and without
// run.overview.read (the snapshot checked sync.snapshot.read, brief note 9).
func (s *Service) ActiveForActor(ctx context.Context) ([]runs.RunSummary, error) {
	a := caller(ctx)
	sc, err := s.d.Authorizer.Scope(ctx, a, authorize.ActionRunRead)
	if err != nil {
		return nil, err
	}
	if len(sc.ChannelIDs) == 0 {
		return []runs.RunSummary{}, nil
	}
	read := func(after *RunKey, n int) ([]RunRow, error) {
		return s.d.Repo.ActiveRuns(ctx, ActiveRunsRead{After: after, Limit: n})
	}
	kept, _, err := s.fill(ctx, a, 0, nil, read, s.locate(ctx), sc.ChannelIDs)
	if err != nil {
		return nil, err
	}
	return s.summaries(ctx, kept)
}

// runCursor decodes the run list cursor s; nil for none. Errors: 400 cursor_invalid.
func (s *Service) runCursor(list string, binding []byte, cursor string) (*RunKey, error) {
	if cursor == "" {
		return nil, nil
	}
	var k runKey
	if err := s.d.Cursors.Decode(list, binding, cursor, &k); err != nil {
		return nil, err
	}
	if k.QueuedAt.IsZero() || k.ID.IsZero() {
		return nil, errs.New(errs.CodeCursorInvalid, "")
	}
	return &RunKey{QueuedAt: k.QueuedAt, ID: k.ID}, nil
}

// located is a run row with the routing data of its thread now.
type located struct {
	row  RunRow
	info chat.ThreadInfo
}

// locate returns the resolver of the threads of a batch through chat.Threads.Locate.
func (s *Service) locate(ctx context.Context) func([]RunRow) (map[ids.ID]chat.ThreadInfo, error) {
	return func(rows []RunRow) (map[ids.ID]chat.ThreadInfo, error) {
		want := make([]ids.ID, 0, len(rows))
		for _, r := range rows {
			if !slices.Contains(want, r.ThreadID) {
				want = append(want, r.ThreadID)
			}
		}
		infos, err := s.d.Threads.Locate(ctx, want)
		if err != nil {
			return nil, err
		}
		out := make(map[ids.ID]chat.ThreadInfo, len(infos))
		for _, in := range infos {
			out[in.ID] = in
		}
		return out, nil
	}
}

// fill reads batches of limit rows (a whole batch of api.page_size_max when limit is 0, which
// means no paging) through read, keeps the rows whose thread resolves into a visible channel and
// that run.read allows in filter mode, and stops when limit rows are kept or no rows remain. next
// is the keyset of the last examined row when the page is full and another row exists.
func (s *Service) fill(ctx context.Context, a actor.Actor, limit int, after *RunKey, read func(*RunKey, int) ([]RunRow, error),
	resolve func([]RunRow) (map[ids.ID]chat.ThreadInfo, error), visible []ids.ID) ([]located, *RunKey, error) {
	batch := limit
	if batch <= 0 {
		batch = s.cfg.PageSizeMax
	}
	var kept []located
	for {
		rows, err := read(after, batch)
		if err != nil || len(rows) == 0 {
			return kept, nil, err
		}
		infos, err := resolve(rows)
		if err != nil {
			return nil, nil, err
		}
		var at int
		if kept, at, err = s.scan(ctx, a, rows, infos, visible, kept, limit); err != nil {
			return nil, nil, err
		}
		if at >= 0 {
			return s.full(kept, rows[at+1:], keyOf(rows[at]), read)
		}
		after = keyOf(rows[len(rows)-1])
		if len(rows) < batch {
			return kept, nil, nil
		}
	}
}

// keyOf is the keyset position of a row.
func keyOf(r RunRow) *RunKey { return &RunKey{QueuedAt: r.QueuedAt, ID: r.ID} }

// scan adds the rows of one batch that pass keep to kept. at is the index of the row that filled
// the page (limit above 0), -1 when the page is not full.
func (s *Service) scan(ctx context.Context, a actor.Actor, rows []RunRow, infos map[ids.ID]chat.ThreadInfo, visible []ids.ID,
	kept []located, limit int) ([]located, int, error) {
	for i, r := range rows {
		k, ok, err := s.keep(ctx, a, r, infos, visible)
		if err != nil {
			return nil, -1, err
		}
		if !ok {
			continue
		}
		kept = append(kept, k)
		if limit > 0 && len(kept) == limit {
			return kept, i, nil
		}
	}
	return kept, -1, nil
}

// keep decides one row of a list: its thread must resolve into a visible channel, and run.read
// must allow it in filter mode; a refusal after the channel filter is a filter mismatch.
func (s *Service) keep(ctx context.Context, a actor.Actor, r RunRow, infos map[ids.ID]chat.ThreadInfo, visible []ids.ID) (located, bool, error) {
	info, ok := infos[r.ThreadID]
	if !ok {
		return located{}, false, nil
	}
	if _, ok := slices.BinarySearchFunc(visible, info.ChannelID, compareIDs); !ok {
		return located{}, false, nil
	}
	d, err := s.d.Authorizer.Authorize(authorize.Filter(ctx), a, authorize.ActionRunRead, runResource(r))
	if err != nil {
		return located{}, false, err
	}
	if !d.Allowed() {
		s.filterMismatch(ctx)
		return located{}, false, nil
	}
	return located{row: r, info: info}, true, nil
}

// full ends a full page: the cursor is the last examined row when another row follows it.
func (s *Service) full(kept []located, rest []RunRow, after *RunKey, read func(*RunKey, int) ([]RunRow, error)) ([]located, *RunKey, error) {
	more, err := s.more(rest, after, read)
	if err != nil || !more {
		return kept, nil, err
	}
	return kept, after, nil
}

// more reports whether a row follows the position after: one of rest, else one more read.
func (s *Service) more(rest []RunRow, after *RunKey, read func(*RunKey, int) ([]RunRow, error)) (bool, error) {
	if len(rest) > 0 {
		return true, nil
	}
	rows, err := read(after, 1)
	return len(rows) > 0, err
}

// summaries converts the kept rows, with the step counts of the page in one statement.
func (s *Service) summaries(ctx context.Context, kept []located) ([]runs.RunSummary, error) {
	out := make([]runs.RunSummary, 0, len(kept))
	if len(kept) == 0 {
		return out, nil
	}
	list := make([]ids.ID, 0, len(kept))
	for _, k := range kept {
		list = append(list, k.row.ID)
	}
	counts, err := s.d.Repo.StepCounts(ctx, list)
	if err != nil {
		return nil, err
	}
	for _, k := range kept {
		out = append(out, summary(k.row, k.info.ChannelID, counts[k.row.ID]))
	}
	return out, nil
}

// filterMismatch logs and counts a run the channel filter kept and run.read refused: only the
// action and the reason, never an id.
func (s *Service) filterMismatch(ctx context.Context) {
	s.d.Log.LogAttrs(ctx, slog.LevelError, msgFilterMismatch,
		slog.String(keyAction, string(authorize.ActionRunRead)), slog.String(keyReason, reasonFilterMismatch))
	if s.d.Errors != nil {
		s.d.Errors.CountError(ctx, reasonFilterMismatch)
	}
}

// compareIDs orders ids bytewise, the order of Scope.ChannelIDs.
func compareIDs(a, b ids.ID) int {
	for i := range a {
		if a[i] != b[i] {
			return int(a[i]) - int(b[i])
		}
	}
	return 0
}
