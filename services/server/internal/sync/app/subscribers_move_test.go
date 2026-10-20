// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"context"
	"errors"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/chat"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/sync/app"
)

// fakeMoveRows records the copies OnThreadMoved changes and fails with err.
type fakeMoveRows struct {
	err           error
	states, drfts []ids.ID
}

func (f *fakeMoveRows) MoveThreadReadStates(_ context.Context, thread, channel ids.ID) error {
	f.states = append(f.states, thread, channel)
	return f.err
}

func (f *fakeMoveRows) MoveThreadDrafts(_ context.Context, thread, channel ids.ID) error {
	f.drfts = append(f.drfts, thread, channel)
	return nil
}

// TestOnThreadMoved: the read states and the drafts of the thread follow it to the target
// channel; a failed read state copy stops before the drafts, and a subscriber without the move
// rows is an internal error (brief M1-62 note 1f).
func TestOnThreadMoved(t *testing.T) {
	t.Parallel()
	ev := chat.ThreadMoved{ThreadID: ids.New(), FromChannelID: ids.New(), ToChannelID: ids.New()}
	rows := &fakeMoveRows{}
	if err := app.NewSubscribers(&fakeDraftRows{}, rows).OnThreadMoved(workCtx(), ev); err != nil {
		t.Fatal(err)
	}
	want := []ids.ID{ev.ThreadID, ev.ToChannelID}
	if len(rows.states) != 2 || rows.states[0] != want[0] || rows.states[1] != want[1] || len(rows.drfts) != 2 || rows.drfts[1] != want[1] {
		t.Fatalf("copies %v %v", rows.states, rows.drfts)
	}
	boom := errors.New("boom")
	failing := &fakeMoveRows{err: boom}
	if err := app.NewSubscribers(&fakeDraftRows{}, failing).OnThreadMoved(workCtx(), ev); !errors.Is(err, boom) || len(failing.drfts) != 0 {
		t.Fatalf("failed copy: %v, drafts %v", err, failing.drfts)
	}
	err := app.NewSubscribers(&fakeDraftRows{}, nil).OnThreadMoved(workCtx(), ev)
	if e, ok := errs.As(err); !ok || e.Code != errs.CodeInternalError {
		t.Fatalf("no rows: %v", err)
	}
}
