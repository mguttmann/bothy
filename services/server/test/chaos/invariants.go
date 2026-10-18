// SPDX-License-Identifier: Apache-2.0

//go:build perf

package chaos

import (
	"context"
	"fmt"
	"net/http"
	"slices"
	"testing"

	"github.com/OWNER/bothy/services/server/test/harness"
)

// CheckInvariants holds after a chaos test what 03 "Transaction rules" and 04 "Pattern with a
// counter row" and "Delivery guarantee" promise, with read-only queries through AdminDB:
//
//  1. seq in event_log is gapless from 1 and the event_seq counter equals its maximum;
//  2. every message has exactly one message.created event, and every such event its message;
//  3. per thread, thread_seq is gapless from 1 and threads.last_message_seq equals its maximum;
//  4. every acknowledged send exists exactly once, and no attempted send exists twice;
//  5. no idempotency_keys row is left in_progress;
//  6. verifyAuditChain reports a valid chain;
//  7. every receiver saw every message.created of the channel after its first ready, without a
//     new seq below one it had seen (duplicates are dropped by seq).
//
// Each violation is an error of t.
func CheckInvariants(t *testing.T, w *World) {
	t.Helper()
	ctx := t.Context()
	db := w.P.AdminDB()
	var count, lo, hi, counter int64
	if err := db.QueryRow(ctx, `SELECT count(*), coalesce(min(seq), 0), coalesce(max(seq), 0),
		(SELECT value FROM sequence_counters WHERE name = 'event_seq') FROM event_log`).Scan(&count, &lo, &hi, &counter); err != nil {
		t.Fatal(err)
	}
	if count == 0 || lo != 1 || hi != count || counter != hi {
		t.Errorf("event_log: %d rows, seq %d..%d, counter event_seq %d; want 1..count and the counter at the maximum", count, lo, hi, counter)
	}
	var orphanMsgs, orphanEvents int64
	if err := db.QueryRow(ctx, `SELECT
		(SELECT count(*) FROM messages m WHERE (SELECT count(*) FROM event_log e WHERE e.type = 'message.created' AND e.resource_id = m.id) <> 1),
		(SELECT count(*) FROM event_log e WHERE e.type = 'message.created' AND NOT EXISTS (SELECT 1 FROM messages m WHERE m.id = e.resource_id))`).
		Scan(&orphanMsgs, &orphanEvents); err != nil {
		t.Fatal(err)
	}
	if orphanMsgs != 0 || orphanEvents != 0 {
		t.Errorf("%d messages without exactly one message.created, %d message.created without a message", orphanMsgs, orphanEvents)
	}
	rows, err := db.Query(ctx, `SELECT t.id::text, t.last_message_seq, count(m.id), coalesce(min(m.thread_seq), 0), coalesce(max(m.thread_seq), 0)
		FROM threads t LEFT JOIN messages m ON m.thread_id = t.id GROUP BY t.id, t.last_message_seq`)
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var id string
		var last, n, mn, mx int64
		if err := rows.Scan(&id, &last, &n, &mn, &mx); err != nil {
			t.Fatal(err)
		}
		if n > 0 && (mn != 1 || mx != n) || last != mx {
			t.Errorf("thread %s: %d messages, thread_seq %d..%d, last_message_seq %d", id, n, mn, mx, last)
		}
	}
	rows.Close()
	acked := w.Acked()
	ackedIDs := make([]string, 0, len(acked))
	for cm := range acked {
		ackedIDs = append(ackedIDs, cm)
	}
	counts := map[string]int{}
	rows, err = db.Query(ctx, `SELECT client_msg_id::text, count(*) FROM messages WHERE client_msg_id::text = ANY($1) GROUP BY 1`, w.Attempted())
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var cm string
		var n int
		if err := rows.Scan(&cm, &n); err != nil {
			t.Fatal(err)
		}
		counts[cm] = n
		if n > 1 {
			t.Errorf("client_msg_id %s exists %d times", cm, n)
		}
	}
	rows.Close()
	missing := 0
	for _, cm := range ackedIDs {
		if counts[cm] != 1 {
			missing++
		}
	}
	if missing > 0 || len(acked) == 0 {
		t.Errorf("%d of %d acknowledged sends do not exist exactly once", missing, len(acked))
	}
	var inProgress int64
	if err := db.QueryRow(ctx, `SELECT count(*) FROM idempotency_keys WHERE state = 'in_progress'`).Scan(&inProgress); err != nil {
		t.Fatal(err)
	}
	if inProgress != 0 {
		t.Errorf("%d idempotency_keys rows left in_progress", inProgress)
	}
	resp, err := harness.DoWithStepUp(context.WithoutCancel(ctx), w.P.Owner, harness.Request{Method: http.MethodPost, Path: "/audit/verify", Body: map[string]any{}})
	if err != nil {
		t.Fatal(err)
	}
	var verify struct {
		OK bool `json:"ok"`
	}
	resp.JSON(t, &verify)
	if resp.Status != http.StatusOK || !verify.OK {
		t.Errorf("verifyAuditChain: %d %s", resp.Status, resp.Body)
	}
	for i, r := range w.receivers {
		if err := r.check(ctx, w); err != nil {
			t.Errorf("receiver %d: %v", i, err)
		}
	}
}

// check holds invariant 7 for one receiver.
func (r *Receiver) check(ctx context.Context, w *World) error {
	rows, err := w.P.AdminDB().Query(ctx, `SELECT seq FROM event_log
		WHERE type = 'message.created' AND channel_id = $1 AND seq > $2 ORDER BY seq`, w.Channel, r.start)
	if err != nil {
		return err
	}
	defer rows.Close()
	r.mu.Lock()
	defer r.mu.Unlock()
	var missing []int64
	for rows.Next() {
		var seq int64
		if err := rows.Scan(&seq); err != nil {
			return err
		}
		if !r.seen[seq] {
			missing = append(missing, seq)
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if len(r.outOfOrder) > 0 {
		return fmt.Errorf("%d dispatches out of seq order, first: %s", len(r.outOfOrder), r.outOfOrder[0])
	}
	if len(missing) > 0 {
		return fmt.Errorf("%d message.created of the channel never arrived, first seqs %v", len(missing), missing[:min(5, len(missing))])
	}
	if !slices.IsSorted(r.order) {
		return fmt.Errorf("the order of distinct seqs is not ascending")
	}
	return nil
}

// Report returns per receiver its reconnects and dropped duplicates, for the log of a test.
func (w *World) Report() string {
	s := ""
	for i, r := range w.receivers {
		r.mu.Lock()
		s += fmt.Sprintf("receiver %d: %d events, %d duplicates dropped, %d reconnects; ", i, len(r.order), r.dupes, r.reconnects)
		r.mu.Unlock()
	}
	return s + fmt.Sprintf("%d sends acknowledged", len(w.Acked()))
}
