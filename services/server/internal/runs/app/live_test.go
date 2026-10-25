// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"context"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/runs/app"
)

// TestLiveRuns (M6-15): the source of the live frame reads every active run across batches,
// newest first, keeps at most perChannel per channel, takes the place of the thread now, the
// reported billing mode before the configured one and the newest running step.
func TestLiveRuns(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	other := ids.New()
	var mine []app.RunRow
	for i := 1; i <= 5; i++ {
		mine = append(mine, w.activeRun(w.channel, i))
	}
	elsewhere := w.activeRun(other, 6)
	w.repo.mu.Lock()
	for i := range w.repo.runs {
		if w.repo.runs[i].ID == mine[4].ID {
			w.repo.runs[i].BillingModeUsed = "personal"
		}
	}
	w.repo.steps = append(w.repo.steps,
		app.StepRow{ID: ids.New(), RunID: mine[4].ID, Ordinal: 1, Kind: "tool_call", Name: "Read", Status: "completed"},
		app.StepRow{ID: ids.New(), RunID: mine[4].ID, Ordinal: 2, Kind: "tool_call", Name: "Bash", Status: "running"})
	w.repo.mu.Unlock()

	got, err := w.svc.LiveRuns(context.Background(), 3)
	if err != nil {
		t.Fatal(err)
	}
	want := []ids.ID{elsewhere.ID, mine[4].ID, mine[3].ID, mine[2].ID}
	if len(got) != len(want) {
		t.Fatalf("%d runs, want %d: %+v", len(got), len(want), got)
	}
	for i, id := range want {
		if got[i].ID != id {
			t.Fatalf("run %d = %s, want %s", i, got[i].ID, id)
		}
	}
	if got[0].ChannelID != other || got[1].ChannelID != w.channel || got[1].SpaceID != w.space {
		t.Fatalf("places %+v", got[:2])
	}
	if got[1].BillingMode != "personal" || got[2].BillingMode != "organization" {
		t.Fatalf("billing modes %q %q", got[1].BillingMode, got[2].BillingMode)
	}
	if got[1].StepKind != "tool_call" || got[1].StepName != "Bash" || got[2].StepKind != "" {
		t.Fatalf("steps %+v %+v", got[1], got[2])
	}
}
