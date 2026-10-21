// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/agents"
	"github.com/OWNER/bothy/services/server/internal/agents/app"
	"github.com/OWNER/bothy/services/server/internal/agents/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// TestHeartbeatReader reads the stored heartbeat of each known agent: time, reported runs, and
// whether active_runs was reported at all; unknown ids are absent.
func TestHeartbeatReader(t *testing.T) {
	t.Parallel()
	store := newFakeStore()
	at := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)
	listed, empty, none, never := ids.New(), ids.New(), ids.New(), ids.New()
	run1, run2 := ids.New(), ids.New()
	store.rows[listed] = domain.PresenceRow{ID: listed, LastHeartbeatAt: at,
		Stored: domain.Report{State: domain.RuntimeState(agents.StateOnline), Details: domain.Details{ActiveRuns: []ids.ID{run1, run2}}}}
	store.rows[empty] = domain.PresenceRow{ID: empty, LastHeartbeatAt: at.Add(time.Second),
		Stored: domain.Report{State: domain.RuntimeState(agents.StateOnline), Details: domain.Details{ActiveRuns: []ids.ID{}}}}
	store.rows[none] = domain.PresenceRow{ID: none, LastHeartbeatAt: at.Add(2 * time.Second),
		Stored: domain.Report{State: domain.RuntimeState(agents.StateDegraded)}}
	store.rows[never] = domain.PresenceRow{ID: never, Stored: domain.Report{State: domain.RuntimeState(agents.StateUnknown)}}
	r, err := app.NewHeartbeatReader(store)
	if err != nil {
		t.Fatal(err)
	}
	var _ agents.Heartbeats = r
	got, err := r.HeartbeatStates(context.Background(), []ids.ID{listed, empty, ids.New(), none, never})
	if err != nil {
		t.Fatal(err)
	}
	want := []agents.HeartbeatState{
		{AgentID: listed, LastHeartbeatAt: at, ActiveRunIDs: []ids.ID{run1, run2}, ActiveRunsReported: true},
		{AgentID: empty, LastHeartbeatAt: at.Add(time.Second), ActiveRunIDs: []ids.ID{}, ActiveRunsReported: true},
		{AgentID: none, LastHeartbeatAt: at.Add(2 * time.Second)},
		{AgentID: never},
	}
	if !slices.EqualFunc(got, want, func(a, b agents.HeartbeatState) bool {
		return a.AgentID == b.AgentID && a.LastHeartbeatAt.Equal(b.LastHeartbeatAt) && slices.Equal(a.ActiveRunIDs, b.ActiveRunIDs) &&
			a.ActiveRunsReported == b.ActiveRunsReported && (a.ActiveRunIDs == nil) == (b.ActiveRunIDs == nil)
	}) {
		t.Fatalf("got %+v\nwant %+v", got, want)
	}
	// The result is the reader's own: changing it leaves the store untouched.
	got[0].ActiveRunIDs[0] = ids.ID{}
	if store.rows[listed].Stored.Details.ActiveRuns[0] != run1 {
		t.Fatal("HeartbeatStates shares the slice of the store")
	}
	if got, err := r.HeartbeatStates(context.Background(), nil); err != nil || len(got) != 0 {
		t.Fatalf("empty list: %v, %v", got, err)
	}
}

// TestHeartbeatReader_Errors: a missing store fails at construction and a store error passes
// through unchanged.
func TestHeartbeatReader_Errors(t *testing.T) {
	t.Parallel()
	if _, err := app.NewHeartbeatReader(nil); err == nil {
		t.Fatal("NewHeartbeatReader(nil) succeeded")
	}
	boom := errors.New("boom")
	r, err := app.NewHeartbeatReader(failingRows{err: boom})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := r.HeartbeatStates(context.Background(), []ids.ID{ids.New()}); !errors.Is(err, boom) {
		t.Fatalf("err %v, want the store error", err)
	}
}

// failingRows is a presence store whose GetPresenceRows fails.
type failingRows struct {
	app.PresenceStore
	err error
}

func (f failingRows) GetPresenceRows(context.Context, []ids.ID) ([]domain.PresenceRow, error) {
	return nil, f.err
}
