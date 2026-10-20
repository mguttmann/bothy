// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/sync"
	"github.com/OWNER/bothy/services/server/internal/sync/app"
)

// lossHandler is a recordingHandler that also implements sync.ConsumerLossHandler: it records the
// lost ranges and the order of the calls, and fails HandleLost when failLost is set.
type lossHandler struct {
	recordingHandler
	lost     [][2]int64
	calls    []string
	failLost bool
}

func (h *lossHandler) HandleLost(_ context.Context, from, to int64) error {
	h.calls = append(h.calls, "lost")
	if h.failLost {
		return errors.New("probe loss failure")
	}
	h.lost = append(h.lost, [2]int64{from, to})
	return nil
}

func (h *lossHandler) HandleEvent(ctx context.Context, ev sync.Event) error {
	h.calls = append(h.calls, "event")
	return h.recordingHandler.HandleEvent(ctx, ev)
}

// TestRoundCallsHandleLost: a handler implementing sync.ConsumerLossHandler gets the lost range of
// the round once, before the first event; an error rolls the round back; a handler without the
// interface runs as before (M2-26 design note 3 f).
func TestRoundCallsHandleLost(t *testing.T) {
	t.Parallel()
	run := func(t *testing.T, h sync.ConsumerHandler, log *fakeLog) (*fakeCursors, app.RoundResult, error) {
		t.Helper()
		cursors := newFakeCursors()
		s := newService(t, cursors, log, nil)
		if err := s.Register(sync.ConsumerWebhooks, h); err != nil {
			t.Fatal(err)
		}
		if err := s.Ensure(context.Background()); err != nil {
			t.Fatal(err)
		}
		res, err := s.Round(context.Background(), sync.ConsumerWebhooks)
		return cursors, res, err
	}
	h := &lossHandler{}
	_, res, err := run(t, h, &fakeLog{seqs: []int64{5, 6}, minRetained: 5})
	if err != nil || res.Processed != 2 {
		t.Fatalf("Round = %+v, %v", res, err)
	}
	if !slices.Equal(h.lost, [][2]int64{{1, 4}}) || !slices.Equal(h.calls, []string{"lost", "event", "event"}) {
		t.Fatalf("lost %v, calls %v", h.lost, h.calls)
	}
	// Without a loss HandleLost is not called.
	h = &lossHandler{}
	if _, _, err := run(t, h, &fakeLog{seqs: []int64{1, 2}, minRetained: 1}); err != nil || len(h.lost) != 0 || slices.Contains(h.calls, "lost") {
		t.Fatalf("HandleLost without a loss: %v %v", h.calls, err)
	}
	// An error of HandleLost rolls the round back: no event handled, the cursor stays.
	h = &lossHandler{failLost: true}
	cursors, res, err := run(t, h, &fakeLog{seqs: []int64{5, 6}, minRetained: 5})
	if err == nil || res != (app.RoundResult{}) || len(cursors.advanced) != 0 || len(h.seen) != 0 {
		t.Fatalf("failed loss: %+v %v, cursor %v, seen %v", res, err, cursors.advanced, h.seen)
	}
	// A handler without the interface still processes the retained events.
	plain := &recordingHandler{}
	if _, res, err := run(t, plain, &fakeLog{seqs: []int64{5, 6}, minRetained: 5}); err != nil || res.Processed != 2 ||
		!slices.Equal(plain.seen, []int64{5, 6}) {
		t.Fatalf("plain handler: %+v %v %v", res, err, plain.seen)
	}
}
