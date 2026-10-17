// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"log/slog"
	"slices"
	"time"

	"github.com/OWNER/bothy/services/server/internal/gateway/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/sync"
)

// runCatchUp is the catch-up runner of one connection (04 section 7.2): it reads the pages after
// from up to head with sync.catchup_page_size in filter mode at the current rights version, queues every admitted event as
// dispatch with the cursor frames of domain.PageSteps, sends catchup.complete {head_seq} and goes
// live through flush. It waits for queue space (only its own goroutine) and stops when the
// connection closes. A page that cannot be read is retried with a backoff from sync.log_poll_interval
// up to database.listen_reconnect_backoff_cap; an unauthenticated answer closes with 4012. A panic
// closes the connection with 4000 (02 "Panics").
func (h *Hub) runCatchUp(ctx context.Context, m *member, from, head int64) {
	defer func() {
		if p := recover(); p != nil {
			h.d.Log.LogAttrs(ctx, slog.LevelError, msgPanic, slog.String(logKeyErrorType, errorTypePanic))
			h.closeMember(m, domain.CloseUnknownError)
		}
	}()
	_, a := m.c.current()
	after, pause := from, h.cfg.RetryBase
	for after < head {
		// The pages are read in filter mode with a request scope, not in delivery mode: events up
		// to head can lie ahead of the log tail of this process, so Observe may not have seen a
		// revocation committed before them; the rights version is read once per page (AZ-P9).
		pctx := actor.WithActor(authorize.WithRequestScope(authorize.Filter(ctx)), a)
		page, err := h.d.CatchUp.PageEvents(pctx, sync.PageInput{After: after, Head: head, Limit: h.cfg.PageSize})
		if errs.KindOf(err) == errs.KindUnauthenticated {
			h.closeMember(m, domain.CloseRevoked)
			return
		}
		if err != nil {
			h.d.Log.LogAttrs(ctx, slog.LevelWarn, msgCatchUpRetry, slog.String(logKeyErrorType, errorTypeCatchUp))
			if !h.sleep(ctx, pause) {
				return
			}
			pause = min(2*pause, h.cfg.RetryCap)
			continue
		}
		pause = h.cfg.RetryBase
		if !h.queuePage(ctx, m, a.Kind, page, after) {
			return
		}
		after = max(page.NextCursor, after+1)
	}
	if !h.offer(ctx, m, controlEntry("catchup.complete", completeFrame{HeadSeq: head}, 0)) {
		return
	}
	h.flush(ctx, m)
}

// queuePage queues the frames of one page; it reports false when the connection closed.
func (h *Hub) queuePage(ctx context.Context, m *member, kind actor.Kind, page sync.EventPage, after int64) bool {
	m.mu.Lock()
	last := max(m.lastSeq, after)
	m.mu.Unlock()
	byseq := make(map[int64]sync.Event, len(page.Events))
	admitted := make([]int64, 0, len(page.Events))
	for _, ev := range page.Events {
		byseq[ev.Seq] = ev
		admitted = append(admitted, ev.Seq)
	}
	for _, st := range domain.PageSteps(last, admitted, page.NextCursor, int64(h.cfg.CursorEvery)) {
		var e domain.Entry
		if st.Dispatch {
			h.learn(m, byseq[st.Seq])
			var err error
			if e, err = dispatchEntry(&encoded{ev: byseq[st.Seq]}, kind); err != nil {
				h.d.Log.LogAttrs(ctx, slog.LevelError, msgEncodeEvent, slog.String(logKeyErrorType, errorTypeEncodeEv))
				continue
			}
		} else {
			e = controlEntry("cursor", cursorFrame{Seq: st.Seq}, st.Seq)
		}
		if !h.offer(ctx, m, e) {
			return false
		}
	}
	return true
}

// offer queues e on m once the queue has room below domain.ControlReserve (WaitSpace), so control
// frames of the client's ops still fit; a dispatch of a target removed meanwhile is dropped. It
// reports false when the connection closed or ctx ended.
func (h *Hub) offer(ctx context.Context, m *member, e domain.Entry) bool {
	for {
		if err := m.c.conn.WaitSpace(ctx); err != nil {
			return false
		}
		m.mu.Lock()
		switch {
		case m.phase == domain.PhaseClosing:
			m.mu.Unlock()
			return false
		case e.Class == domain.ClassDispatch && slices.ContainsFunc(m.removed, func(t domain.Targets) bool {
			return t.Covers(e.SpaceID, e.ChannelID, e.Seq)
		}):
			m.mu.Unlock()
			return true
		case m.c.conn.Offer(e):
			if e.Seq > 0 {
				m.lastSeq = e.Seq
			}
			m.mu.Unlock()
			return true
		}
		m.mu.Unlock()
	}
}

// sleep waits d; false when ctx ends first.
func (h *Hub) sleep(ctx context.Context, d time.Duration) bool {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-t.C:
		return true
	}
}
