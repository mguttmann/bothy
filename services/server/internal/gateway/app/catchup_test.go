// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"context"
	"slices"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/gateway/app"
	"github.com/OWNER/bothy/services/server/internal/gateway/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/sync"
)

// catchUpWorld stores ten events of a space, the odd ones in a channel Alex sees, the even ones
// in a channel he does not.
func catchUpWorld() (w *hubWorld, alex actor.Actor, space, visible, hidden ids.ID) {
	w = newHubWorld()
	space = ids.New()
	visible, hidden = w.channel(space), w.channel(space)
	alex = person()
	w.grantChannel(alex.ID, visible)
	for seq := int64(1); seq <= 10; seq++ {
		ch := hidden
		if seq%2 == 1 {
			ch = visible
		}
		w.store(msg(seq, space, ch))
	}
	return w, alex, space, visible, hidden
}

// waitComplete drains qc until catchup.complete was written and, for a connection of connect,
// until the hub has flushed the live buffer and moved it to the live phase, so that a later
// Dispatch is delivered live; it returns every frame. The hub queues catchup.complete before the
// flush, so the frame alone does not mean the connection is live.
func waitComplete(t *testing.T, qc *qconn) []frame {
	t.Helper()
	var fs []frame
	waitFor(t, "catchup.complete", func() bool {
		fs = framesOf(t, qc.drain())
		return slices.ContainsFunc(fs, func(f frame) bool { return f.op == "catchup.complete" }) &&
			(qc.live == nil || qc.live())
	})
	return framesOf(t, qc.drain())
}

// TestCatchUp_PagesCursorFramesAndComplete: about 2.5 pages (sync.catchup_page_size 4), half of
// the events invisible: the visible ones as dispatch, a cursor frame at the end of every page with
// its last checked seq, then catchup.complete {head_seq} (04 sections 7.1 and 7.2).
func TestCatchUp_PagesCursorFramesAndComplete(t *testing.T) {
	t.Parallel()
	w, alex, _, _, _ := catchUpWorld()
	h := newHub(t, w, hubCfg)
	qc := connect(t, h, alex, 100, 0, 10)
	got := waitComplete(t, qc)
	want := []frame{{"dispatch", 1}, {"dispatch", 3}, {"cursor", 4}, {"dispatch", 5}, {"dispatch", 7}, {"cursor", 8},
		{"dispatch", 9}, {"cursor", 10}, {"catchup.complete", 10}}
	if !slices.Equal(got, want) {
		t.Fatalf("frames %v\nwant   %v", got, want)
	}
}

// TestCatchUp_LiveEventsBufferedThenFlushedInSeqOrder: events committed while the pages are read
// are buffered and follow catchup.complete in seq order; a late event of the tail with seq <= H is
// covered by the catch-up and not sent twice (04 section 7.2 steps 3 to 5).
func TestCatchUp_LiveEventsBufferedThenFlushedInSeqOrder(t *testing.T) {
	t.Parallel()
	w, alex, space, visible, hidden := catchUpWorld()
	h := newHub(t, w, hubCfg)
	live := []sync.Event{msg(9, space, visible), msg(11, space, visible), msg(12, space, hidden), msg(13, space, visible)}
	next := 0
	// One live event per page, the rest with the last page (after 8).
	w.onPage = func(after int64) {
		for next < len(live) {
			h.Dispatch(context.Background(), live[next])
			next++
			if after < 8 {
				break
			}
		}
	}
	qc := connect(t, h, alex, 100, 0, 10)
	got := waitComplete(t, qc)
	waitFor(t, "the buffered events", func() bool {
		got = framesOf(t, qc.drain())
		return slices.Contains(dispatched(got), 11)
	})
	h.Dispatch(context.Background(), msg(14, space, visible))
	got = framesOf(t, qc.drain())
	i := slices.IndexFunc(got, func(f frame) bool { return f.op == "catchup.complete" })
	if before := dispatched(got[:i]); !slices.Equal(before, []int64{1, 3, 5, 7, 9}) {
		t.Fatalf("catch-up dispatches %v", before)
	}
	if after := got[i+1:]; !slices.Equal(after, []frame{{"dispatch", 11}, {"dispatch", 13}, {"dispatch", 14}}) {
		t.Fatalf("after catchup.complete %v", after)
	}
	d := dispatched(got)
	for k := 1; k < len(d); k++ {
		if d[k] <= d[k-1] {
			t.Fatalf("dispatch seqs %v not strictly increasing", d)
		}
	}
}

// TestCatchUp_LiveBufferOverflowCloses4014: more live events during catch-up than
// sync.outbound_queue_max close the connection with 4014 (04 section 7.2 step 5).
func TestCatchUp_LiveBufferOverflowCloses4014(t *testing.T) {
	t.Parallel()
	w, alex, space, visible, _ := catchUpWorld()
	cfg := hubCfg
	cfg.QueueMax = 2
	h := newHub(t, w, cfg)
	w.onPage = func(after int64) {
		if after == 0 {
			for seq := int64(11); seq <= 13; seq++ {
				h.Dispatch(context.Background(), msg(seq, space, visible))
			}
		}
	}
	qc := connect(t, h, alex, 100, 0, 10)
	waitFor(t, "the close", func() bool { return qc.closedWith() != 0 })
	if code := qc.closedWith(); code != domain.CloseSlowConsumer {
		t.Fatalf("closed with %d, want 4014", code)
	}
}

// TestCatchUp_FlushWaitsForQueueSpace: a catch-up that fills the outbound queue
// (sync.outbound_queue_max 3) still delivers the buffered live event after catchup.complete; the
// flush waits for queue space like the pages do, and only a live buffer above the bound closes
// with 4014 (04 section 7.2 step 5).
func TestCatchUp_FlushWaitsForQueueSpace(t *testing.T) {
	t.Parallel()
	w, alex, space, visible, _ := catchUpWorld()
	cfg := hubCfg
	cfg.QueueMax = 3
	h := newHub(t, w, cfg)
	w.onPage = func(after int64) {
		if after == 0 {
			h.Dispatch(context.Background(), msg(11, space, visible))
		}
	}
	qc := connect(t, h, alex, 3, 0, 10)
	var got []frame
	waitFor(t, "the live event after catchup.complete", func() bool {
		got = framesOf(t, qc.drain())
		return slices.Contains(dispatched(got), 11) || qc.closedWith() != 0
	})
	want := []frame{{"dispatch", 1}, {"dispatch", 3}, {"cursor", 4}, {"dispatch", 5}, {"dispatch", 7}, {"cursor", 8},
		{"dispatch", 9}, {"cursor", 10}, {"catchup.complete", 10}, {"dispatch", 11}}
	if code := qc.closedWith(); code != 0 || !slices.Equal(got, want) {
		t.Fatalf("close %d, frames %v\nwant %v", code, got, want)
	}
}

// TestCatchUp_PagesReadCurrentRights: the pages are read in filter mode with a fresh request memo
// each, not in delivery mode and not with the memo of the op that attached the connection: the
// events up to H can lie ahead of the log tail of this process, whose Observe has not seen a
// revocation committed before them yet, so the rights version is read per page (red team M1-65,
// blocker 1 and the delta blocker; AZ-P9).
func TestCatchUp_PagesReadCurrentRights(t *testing.T) {
	t.Parallel()
	w, alex, _, _, _ := catchUpWorld()
	h := newHub(t, w, hubCfg)
	qc := newQConn(100)
	c, s := app.NewTestConnection(qc, alex)
	h.Register(context.Background(), c, s)
	// Attach runs inside the identify or resume op, whose context carries the op's memo.
	op := authorize.WithRequestScope(context.Background())
	opMemo, _ := authorize.RequestMemoFrom(op)
	if err := h.Attach(op, c, 0, 10); err != nil {
		t.Fatal(err)
	}
	waitComplete(t, qc)
	w.mu.Lock()
	defer w.mu.Unlock()
	if len(w.pageModes) < 2 {
		t.Fatalf("%d pages read, want at least 2", len(w.pageModes))
	}
	seen := map[*authorize.RequestMemo]bool{}
	for i, m := range w.pageModes {
		if m.delivery || m.memo == nil || m.memo == opMemo || seen[m.memo] {
			t.Fatalf("page %d read with delivery=%v memo=%p (op memo %p), want filter mode with a fresh memo per page", i, m.delivery, m.memo, opMemo)
		}
		seen[m.memo] = true
	}
}

// TestCatchUp_RegrantedTargetDelivered: a channel removed and granted again during one catch-up
// is no longer filtered: its later events reach the connection (04 section 9 item 1; red team
// M1-65, major 1).
func TestCatchUp_RegrantedTargetDelivered(t *testing.T) {
	t.Parallel()
	w, alex, space, visible, _ := catchUpWorld()
	h := newHub(t, w, hubCfg)
	w.onPage = func(after int64) {
		if after != 0 {
			return
		}
		w.revokeChannel(alex.ID, visible)
		h.Dispatch(context.Background(), accessChanged(11, alex.ID, visible))
		w.grantChannel(alex.ID, visible)
		h.Dispatch(context.Background(), accessGranted(12, alex.ID, visible))
		h.Dispatch(context.Background(), msg(13, space, visible))
	}
	qc := connect(t, h, alex, 100, 0, 10)
	var got []frame
	waitFor(t, "the event after the re-grant", func() bool {
		got = framesOf(t, qc.drain())
		return slices.Contains(dispatched(got), 13) || qc.closedWith() != 0
	})
	if !slices.Contains(dispatched(got), 13) {
		t.Fatalf("frames %v: the event of the re-granted channel is missing", got)
	}
}

// TestCatchUp_LeavesControlReserve: the catch-up runner fills the outbound queue only up to
// sync.outbound_queue_max minus domain.ControlReserve, so heartbeat_ack, reauth_required and error
// frames of a client still fit (red team M1-65, major 2).
func TestCatchUp_LeavesControlReserve(t *testing.T) {
	t.Parallel()
	w, alex, _, _, _ := catchUpWorld()
	h := newHub(t, w, hubCfg)
	const max = domain.ControlReserve + 2
	qc := connect(t, h, alex, max, 0, 10)
	waitFor(t, "the runner to wait for space", func() bool { return qc.len() == max-domain.ControlReserve })
	time.Sleep(20 * time.Millisecond)
	if n := qc.len(); n != max-domain.ControlReserve {
		t.Fatalf("queue holds %d frames, want %d", n, max-domain.ControlReserve)
	}
	for range domain.ControlReserve {
		if !qc.Offer(domain.Entry{Class: domain.ClassControl}) {
			t.Fatal("a control frame did not fit beside the catch-up")
		}
	}
}

// TestCatchUp_PageFailureRetried: a page that cannot be read is read again after a pause; the
// connection stays and the catch-up completes.
func TestCatchUp_PageFailureRetried(t *testing.T) {
	t.Parallel()
	w, alex, _, _, _ := catchUpWorld()
	w.fail = 3
	h := newHub(t, w, hubCfg)
	qc := connect(t, h, alex, 100, 0, 10)
	got := waitComplete(t, qc)
	if d := dispatched(got); !slices.Equal(d, []int64{1, 3, 5, 7, 9}) || qc.closedWith() != 0 {
		t.Fatalf("after retries: %v, close %d", d, qc.closedWith())
	}
}

// TestCatchUp_NoCursorNoCatchupFrames: an identify without a cursor gets neither cursor nor
// catchup.complete; it is live from the head of ready (04 section 6.3 step 4).
func TestCatchUp_NoCursorNoCatchupFrames(t *testing.T) {
	t.Parallel()
	w, alex, space, visible, _ := catchUpWorld()
	h := newHub(t, w, hubCfg)
	qc := connect(t, h, alex, 100, 10, 10)
	h.Dispatch(context.Background(), msg(11, space, visible))
	if got := framesOf(t, qc.drain()); !slices.Equal(got, []frame{{"dispatch", 11}}) {
		t.Fatalf("frames %v", got)
	}
}

// TestCatchUp_ResyncReasons: identify with a cursor that catch-up refuses (retention,
// too_far_behind, cursor_ahead) gets resync_required {reason, head_seq, sync_epoch} after ready;
// the connection stays open and receives later live events (04 section 7.3).
func TestCatchUp_ResyncReasons(t *testing.T) {
	t.Parallel()
	for _, reason := range []sync.ResyncReason{sync.ResyncRetention, sync.ResyncTooFarBehind, sync.ResyncCursorAhead} {
		t.Run(string(reason), func(t *testing.T) {
			t.Parallel()
			w := newHubWorld()
			var h *app.Hub
			e := newEnv(t, func(_ *app.Config, d *app.Deps) {
				h = newHub(t, w, hubCfg)
				d.Delivery, d.CatchUp = h, fakeCatchUp{head: 41, reason: reason}
			})
			w.instanceWide[e.alex.ID] = true
			c, fc := e.open()
			e.send(c, "identify", map[string]any{"protocol_version": 1, "access_token": alexToken, "dpop": "proof",
				"device_id": e.alex.DeviceID, "cursor": 3, "sync_epoch": epoch})
			wantOpen(t, fc)
			rs := sent[app.ResyncRequired](fc)
			if len(rs) != 1 || rs[0].Reason != reason || rs[0].HeadSeq != 41 || rs[0].SyncEpoch != epoch {
				t.Fatalf("resync_required %+v", rs)
			}
			h.Dispatch(context.Background(), msg(42, ids.ID{}, ids.ID{}))
			entries := sent[domain.Entry](fc)
			if got := framesOf(t, entries); !slices.Equal(got, []frame{{"dispatch", 42}}) {
				t.Fatalf("frames after resync_required %v", got)
			}
			wantOpen(t, fc)
		})
	}
}
