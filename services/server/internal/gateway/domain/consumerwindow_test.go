// SPDX-License-Identifier: Apache-2.0

package domain_test

import (
	"slices"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/gateway/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// t0 is the start time of the window tests.
var t0 = time.Date(2026, 10, 17, 9, 0, 0, 0, time.UTC)

// windowCfg is a window with room for three events, a timeout of ten seconds and a cursor frame
// every five skipped numbers in the live phase.
func windowCfg() domain.WindowConfig {
	return domain.WindowConfig{MaxInflight: 3, AckTimeout: 10 * time.Second, CursorEvery: 5}
}

// page feeds the admitted seqs of one page into w the way the runner does: each while there is
// budget, then the end of the page when it was not cut. It returns the dispatched seqs with their
// flag and whether the page was cut.
func page(w *domain.ConsumerWindow, admitted []int64, end int64, now time.Time) (sent []int64, flags []bool, cut bool) {
	for _, s := range admitted {
		if w.Budget() == 0 {
			return sent, flags, true
		}
		flags = append(flags, w.Dispatch(s, ids.ID{}, ids.ID{}, now))
		sent = append(sent, s)
	}
	w.Checked(end)
	return sent, flags, false
}

// TestConsumerWindow_BudgetBoundsInflight: at zero budget nothing more is dispatched; a page with
// more admitted events than the budget is cut, and read is the last dispatched seq (04 section
// 15.2 item 2).
func TestConsumerWindow_BudgetBoundsInflight(t *testing.T) {
	t.Parallel()
	w := domain.NewConsumerWindow(windowCfg(), 10, 10)
	if w.Budget() != 3 || w.Read() != 10 || w.Acked() != 10 {
		t.Fatalf("fresh window budget %d read %d acked %d", w.Budget(), w.Read(), w.Acked())
	}
	sent, _, cut := page(w, []int64{11, 13, 14, 16, 17}, 20, t0)
	if !cut || !slices.Equal(sent, []int64{11, 13, 14}) || w.Read() != 14 || w.Budget() != 0 {
		t.Fatalf("sent %v cut %v read %d budget %d", sent, cut, w.Read(), w.Budget())
	}
	if w.Inflight() != 3 {
		t.Fatalf("inflight %d", w.Inflight())
	}
}

// TestConsumerWindow_AckCumulativeAndMonotone: acknowledging n drops every inflight entry up to n,
// a lower value changes nothing, and an acknowledgement above read raises read (04 section 15.2
// item 1).
func TestConsumerWindow_AckCumulativeAndMonotone(t *testing.T) {
	t.Parallel()
	w := domain.NewConsumerWindow(windowCfg(), 10, 10)
	page(w, []int64{11, 12, 13}, 13, t0)
	if !w.Ack(12) || w.Acked() != 12 || w.Inflight() != 1 || w.Budget() != 2 {
		t.Fatalf("after ack 12: acked %d inflight %d budget %d", w.Acked(), w.Inflight(), w.Budget())
	}
	if w.Ack(11) || w.Ack(12) || w.Acked() != 12 || w.Inflight() != 1 {
		t.Fatalf("a lower acknowledgement changed the window: acked %d inflight %d", w.Acked(), w.Inflight())
	}
	if !w.Ack(30) || w.Read() != 30 || w.Inflight() != 0 || w.Budget() != 3 {
		t.Fatalf("after ack 30: read %d inflight %d budget %d", w.Read(), w.Inflight(), w.Budget())
	}
}

// TestConsumerWindow_TimeoutRestartsFromAcked: after the deadline reading restarts at acked + 1;
// exactly the numbers dispatched before carry the flag, numbers never dispatched do not; the
// deadline follows the oldest inflight entry after a partial ack (04 section 15.2 item 3).
func TestConsumerWindow_TimeoutRestartsFromAcked(t *testing.T) {
	t.Parallel()
	w := domain.NewConsumerWindow(windowCfg(), 10, 10)
	page(w, []int64{11}, 11, t0)
	page(w, []int64{12, 13}, 13, t0.Add(4*time.Second))
	if d, ok := w.Deadline(); !ok || !d.Equal(t0.Add(10*time.Second)) {
		t.Fatalf("deadline %v %v", d, ok)
	}
	w.Ack(11)
	if d, ok := w.Deadline(); !ok || !d.Equal(t0.Add(14*time.Second)) {
		t.Fatalf("deadline after the partial ack %v %v", d, ok)
	}
	if w.Expire(t0.Add(13 * time.Second)) {
		t.Fatal("expired before the deadline")
	}
	if !w.Expire(t0.Add(14*time.Second)) || w.Read() != 11 || w.Inflight() != 0 || w.Budget() != 3 {
		t.Fatalf("after the timeout read %d inflight %d", w.Read(), w.Inflight())
	}
	sent, flags, _ := page(w, []int64{12, 13, 15}, 15, t0.Add(15*time.Second))
	if !slices.Equal(sent, []int64{12, 13, 15}) || !slices.Equal(flags, []bool{true, true, false}) {
		t.Fatalf("redelivery %v flags %v", sent, flags)
	}
	if _, ok := domain.NewConsumerWindow(windowCfg(), 10, 10).Deadline(); ok {
		t.Fatal("an empty window has a deadline")
	}
}

// TestConsumerWindow_PurgeReleasesBudget: the inflight entries of a removed target hold no budget
// and come again after no timeout.
func TestConsumerWindow_PurgeReleasesBudget(t *testing.T) {
	t.Parallel()
	support, marketing := ids.New(), ids.New()
	w := domain.NewConsumerWindow(windowCfg(), 0, 0)
	w.Dispatch(1, support, ids.ID{}, t0)
	w.Dispatch(2, marketing, ids.ID{}, t0)
	w.Dispatch(3, support, ids.ID{}, t0)
	if w.Budget() != 0 {
		t.Fatal("budget left")
	}
	if n := w.Purge(domain.NewTargets([]ids.ID{support}, nil)); n != 2 || w.Budget() != 2 || w.Inflight() != 1 {
		t.Fatalf("purged %d budget %d inflight %d", n, w.Budget(), w.Inflight())
	}
}

// TestConsumerWindow_CursorAndCatchupFrames: in the backlog phase a cursor follows each page with
// skipped numbers, catchup.complete comes exactly once when read reaches the ready head, and live
// a cursor follows once sync.cursor_advance_every numbers were skipped (04 sections 7.1 and 7.2).
func TestConsumerWindow_CursorAndCatchupFrames(t *testing.T) {
	t.Parallel()
	cfg := windowCfg()
	cfg.MaxInflight = 100
	w := domain.NewConsumerWindow(cfg, 10, 30)
	page(w, []int64{12}, 20, t0)
	if got := w.Frames(); !slices.Equal(got, []domain.WindowFrame{{Kind: domain.FrameCursor, Seq: 20}}) {
		t.Fatalf("backlog page frames %v", got)
	}
	page(w, []int64{25}, 25, t0)
	if got := w.Frames(); len(got) != 0 {
		t.Fatalf("a page that ends on a dispatch sent %v", got)
	}
	page(w, nil, 30, t0)
	want := []domain.WindowFrame{{Kind: domain.FrameCursor, Seq: 30}, {Kind: domain.FrameComplete, Seq: 30}}
	if got := w.Frames(); !slices.Equal(got, want) || !w.Live() {
		t.Fatalf("end of the backlog %v, live %v", got, w.Live())
	}
	page(w, nil, 34, t0)
	if got := w.Frames(); len(got) != 0 {
		t.Fatalf("live frames below the advance step %v", got)
	}
	page(w, nil, 35, t0)
	if got := w.Frames(); !slices.Equal(got, []domain.WindowFrame{{Kind: domain.FrameCursor, Seq: 35}}) {
		t.Fatalf("live cursor %v", got)
	}
	fresh := domain.NewConsumerWindow(cfg, 40, 40)
	if got := fresh.Frames(); !slices.Equal(got, []domain.WindowFrame{{Kind: domain.FrameComplete, Seq: 40}}) {
		t.Fatalf("a window at the ready head %v", got)
	}
}

// TestConsumerWindow_ResyncReasons: retention and cursor_ahead restart the window at the head
// without catchup.complete; too_far_behind and epoch_changed do not apply to consumers.
func TestConsumerWindow_ResyncReasons(t *testing.T) {
	t.Parallel()
	for _, r := range []string{"retention", "cursor_ahead"} {
		if !domain.ConsumerResync(r) {
			t.Errorf("%s ignored", r)
		}
	}
	for _, r := range []string{"", "too_far_behind", "epoch_changed"} {
		if domain.ConsumerResync(r) {
			t.Errorf("%q restarts", r)
		}
	}
	w := domain.NewConsumerWindow(windowCfg(), 5, 50)
	w.Dispatch(6, ids.ID{}, ids.ID{}, t0)
	w.Restart(80)
	if w.Acked() != 80 || w.Read() != 80 || w.Inflight() != 0 || !w.Live() {
		t.Fatalf("restart acked %d read %d inflight %d live %v", w.Acked(), w.Read(), w.Inflight(), w.Live())
	}
	if got := w.Frames(); len(got) != 0 {
		t.Fatalf("frames after a restart %v", got)
	}
}
