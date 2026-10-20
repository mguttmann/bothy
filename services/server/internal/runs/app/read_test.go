// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/chat"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/runs"
	"github.com/OWNER/bothy/services/server/internal/runs/app"
	"github.com/OWNER/bothy/services/server/internal/runs/domain"
)

// activeRun inserts an active run of Lotse in a new thread of channel, queued at t0 plus i
// seconds, and returns it.
func (w *world) activeRun(channel ids.ID, i int) app.RunRow {
	th := ids.New()
	w.threads.infos[th] = chat.ThreadInfo{ID: th, ChannelID: channel, SpaceID: w.space, ChannelKind: "text", Kind: "thread", State: "open"}
	row, _, err := w.repo.InsertRun(context.Background(), app.NewRun{
		ID: ids.New(), ThreadID: th, AgentID: w.lotse, TriggeredBy: w.alex, TriggerKind: "message", BillingModeExpected: "organization",
		InvocationJTI: ids.New(), QueuedAt: t0.Add(time.Duration(i) * time.Second),
	})
	if err != nil {
		w.t.Fatal(err)
	}
	return row
}

// asAlex is a context with Alex as the principal of the request.
func (w *world) asAlex() context.Context {
	return actor.WithActor(context.Background(), actor.Actor{ID: w.alex, Kind: actor.KindUser})
}

// TestListActive_FilterMismatchDropped: the channel filter keeps a row that run.read denies; the
// item is dropped, the log names no id, and bothy.authz.errors with filter_mismatch rises by one.
func TestListActive_FilterMismatchDropped(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	keep, drop := w.activeRun(w.channel, 1), w.activeRun(w.channel, 2)
	w.authz.decide = func(act authorize.Action, _ bool, r authorize.Resource) decision {
		if act == authorize.ActionRunRead && r.ID == drop.ID {
			return decision{reason: "no_grant"}
		}
		return decision{allowed: true}
	}
	page, err := w.svc.ListActive(w.asAlex(), runs.ActiveRunsQuery{})
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Items) != 1 || page.Items[0].ID != keep.ID || page.NextCursor != "" {
		t.Fatalf("page %+v", page)
	}
	if w.errs.n["filter_mismatch"] != 1 {
		t.Fatalf("errors %v", w.errs.n)
	}
	log := w.log.String()
	if !strings.Contains(log, "filter_mismatch") || strings.Contains(log, drop.ID.String()) || strings.Contains(log, drop.ThreadID.String()) {
		t.Fatalf("log %s", log)
	}
	for _, c := range w.j.list() {
		if c.what == "authz:run.read:request" {
			t.Fatal("the per item check ran in request mode")
		}
	}
}

// TestListActive_PageFilledAcrossBatches: invisible rows between visible ones; the page is full,
// next_cursor points after the last examined row, and the next page starts there and repeats
// nothing.
func TestListActive_PageFilledAcrossBatches(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	other := ids.New() // a channel outside the scope
	var visible []app.RunRow
	for i := 1; i <= 9; i++ {
		ch := w.channel
		if i%3 != 0 {
			ch = other
		}
		r := w.activeRun(ch, i)
		if ch == w.channel {
			visible = append(visible, r)
		}
	}
	// Newest first: visible are i = 9, 6, 3.
	page, err := w.svc.ListActive(w.asAlex(), runs.ActiveRunsQuery{Limit: 2})
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Items) != 2 || page.Items[0].ID != visible[2].ID || page.Items[1].ID != visible[1].ID || page.NextCursor == "" {
		t.Fatalf("page 1 %+v", page)
	}
	next, err := w.svc.ListActive(w.asAlex(), runs.ActiveRunsQuery{Limit: 2, Cursor: page.NextCursor})
	if err != nil {
		t.Fatal(err)
	}
	if len(next.Items) != 1 || next.Items[0].ID != visible[0].ID || next.NextCursor != "" {
		t.Fatalf("page 2 %+v", next)
	}
	// A cursor of another list or another filter is cursor_invalid.
	_, err = w.svc.ListActive(w.asAlex(), runs.ActiveRunsQuery{Limit: 2, Cursor: page.NextCursor, AgentID: w.lotse})
	if e, ok := errs.As(err); !ok || e.Code != errs.CodeCursorInvalid {
		t.Fatalf("foreign cursor %v", err)
	}
	// An unknown agent gives an empty page.
	empty, err := w.svc.ListActive(w.asAlex(), runs.ActiveRunsQuery{AgentID: ids.New()})
	if err != nil || len(empty.Items) != 0 {
		t.Fatalf("unknown agent %+v %v", empty, err)
	}
	// The snapshot list is the same pipeline without paging.
	all, err := w.svc.ActiveForActor(w.asAlex())
	if err != nil || len(all) != 3 {
		t.Fatalf("snapshot %+v %v", all, err)
	}
}

// TestActiveGroups: the gauge source groups the active runs by handle and state.
func TestActiveGroups(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	w.activeRun(w.channel, 1)
	w.activeRun(w.channel, 2)
	groups, err := w.svc.ActiveGroups(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	n := int64(0)
	for _, g := range groups {
		if g.AgentHandle != "lotse" || g.State != domain.StateQueued {
			t.Fatalf("group %+v", g)
		}
		n += g.Count
	}
	if n != 2 {
		t.Fatalf("groups %+v", groups)
	}
}
