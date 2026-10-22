// SPDX-License-Identifier: Apache-2.0

package runsummary_test

import (
	"context"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/runs"
	"github.com/OWNER/bothy/services/server/internal/sync/adapters/runsummary"
)

// fakeRuns returns fixed active runs and records the actor of the call.
type fakeRuns struct {
	runs.Runs
	list []runs.RunSummary
	seen actor.Actor
}

func (f *fakeRuns) ActiveForActor(ctx context.Context) ([]runs.RunSummary, error) {
	f.seen, _ = actor.FromContext(ctx)
	return f.list, nil
}

// TestSource_ActiveRuns: the snapshot gets the active runs of runs for the caller, only in the
// channels it lists, never nil.
func TestSource_ActiveRuns(t *testing.T) {
	t.Parallel()
	listed, other := ids.New(), ids.New()
	a := actor.Actor{ID: ids.New(), Kind: actor.KindUser}
	in := runs.RunSummary{ID: ids.New(), ChannelID: listed, State: "running", Version: 3, StepCount: 2, DurationMS: 5, HasDuration: true}
	f := &fakeRuns{list: []runs.RunSummary{in, {ID: ids.New(), ChannelID: other, State: "queued"}}}
	got, err := runsummary.New(f).ActiveRuns(context.Background(), a, []ids.ID{listed})
	if err != nil {
		t.Fatal(err)
	}
	if f.seen.ID != a.ID || len(got) != 1 || got[0].ID != in.ID || got[0].State != "running" || got[0].Version != 3 || got[0].StepCount != 2 {
		t.Fatalf("got %+v, actor %+v", got, f.seen)
	}
	f.list = nil
	if got, err = runsummary.New(f).ActiveRuns(context.Background(), a, nil); err != nil || got == nil || len(got) != 0 {
		t.Fatalf("empty %v %v", got, err)
	}
}
