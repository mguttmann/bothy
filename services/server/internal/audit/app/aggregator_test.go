// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/audit/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/auditing"
	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/testkit"
)

// denyN records n identical denials of ctx, one second apart.
func denyN(t *testing.T, w *world, ctx context.Context, n int, e auditing.Entry) {
	t.Helper()
	for range n {
		if err := w.rec.RecordDenial(ctx, e); err != nil {
			t.Fatal(err)
		}
		w.clock.Advance(time.Second)
	}
}

func TestAZP13_IdenticalDenialsInWindowYieldOneEntry(t *testing.T) {
	t.Parallel()
	w := newWorld(t, time.Minute, 10)
	res := ids.New()
	e := deny("grant.create", auditing.ReasonNoGrant)
	e.ResourceKind, e.ResourceID = "grant", res
	denyN(t, w, as(alex), 7, e)
	if err := w.agg.FlushDue(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(w.store.committed()) != 0 {
		t.Fatal("a bucket was written before its window ended")
	}
	w.clock.Set(t0.Add(time.Minute))
	if err := w.agg.FlushDue(context.Background()); err != nil {
		t.Fatal(err)
	}
	rows := w.store.committed()
	if len(rows) != 1 {
		t.Fatalf("rows = %d, want 1", len(rows))
	}
	r := rows[0]
	a := aggregate(t, r)
	if a["count"] != int64(7) || a["first_at"] != "2026-10-12T08:00:00.000Z" || a["last_at"] != "2026-10-12T08:00:06.000Z" {
		t.Fatalf("aggregate = %v", a)
	}
	if !r.OccurredAt.Equal(t0) || r.Decision != "deny" || r.DenyReason != "no_grant" || r.ActorPrincipalID != alex || r.ResourceID != res {
		t.Fatalf("row = %+v", r)
	}
	if err := domain.DetailsV1(r.Details).Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestAZP13_DifferentKeysSeparate(t *testing.T) {
	t.Parallel()
	w := newWorld(t, time.Minute, 10)
	for _, c := range []struct {
		ctx context.Context
		e   auditing.Entry
	}{
		{as(alex), deny("grant.create", auditing.ReasonNoGrant)},
		{as(alex), deny("grant.create", auditing.ReasonNoGrant)},
		{as(sam), deny("grant.create", auditing.ReasonNoGrant)},
		{as(alex), deny("grant.revoke", auditing.ReasonNoGrant)},
		{as(alex), deny("grant.create", auditing.ReasonOwnerOnly)},
	} {
		if err := w.rec.RecordDenial(c.ctx, c.e); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.agg.Flush(context.Background()); err != nil {
		t.Fatal(err)
	}
	rows := w.store.committed()
	if len(rows) != 4 {
		t.Fatalf("rows = %d, want 4 (principal, action and reason each separate)", len(rows))
	}
	if aggregate(t, rows[0])["count"] != int64(2) {
		t.Fatalf("first key count = %v", aggregate(t, rows[0]))
	}
}

func TestAZP13_NewWindowNewEntry(t *testing.T) {
	t.Parallel()
	w := newWorld(t, time.Minute, 10)
	e := deny("grant.create", auditing.ReasonNoGrant)
	denyN(t, w, as(alex), 2, e)
	w.clock.Set(t0.Add(time.Minute))
	denyN(t, w, as(alex), 3, e)
	if err := w.agg.Flush(context.Background()); err != nil {
		t.Fatal(err)
	}
	rows := w.store.committed()
	if len(rows) != 2 || aggregate(t, rows[0])["count"] != int64(2) || aggregate(t, rows[1])["count"] != int64(3) {
		t.Fatalf("rows = %v", rows)
	}
	if !rows[1].OccurredAt.Equal(t0.Add(time.Minute)) {
		t.Fatalf("second window starts at %v", rows[1].OccurredAt)
	}
}

func TestAZP13_AnonymousKeyedByIPHMAC(t *testing.T) {
	t.Parallel()
	w := newWorld(t, time.Minute, 10)
	e := deny("auth.login.begin", auditing.ReasonUnauthenticated)
	denyN(t, w, anonymousFrom("192.0.2.44"), 3, e)
	denyN(t, w, anonymousFrom("::ffff:192.0.2.44"), 1, e) // the same pseudonym
	denyN(t, w, anonymousFrom("192.0.2.45"), 2, e)
	if err := w.agg.Flush(context.Background()); err != nil {
		t.Fatal(err)
	}
	rows := w.store.committed()
	if len(rows) != 2 || aggregate(t, rows[0])["count"] != int64(4) || aggregate(t, rows[1])["count"] != int64(2) {
		t.Fatalf("rows = %v", rows)
	}
	if rows[0].Details[domain.DetailIPHMAC] == rows[1].Details[domain.DetailIPHMAC] || !rows[0].ActorPrincipalID.IsZero() {
		t.Fatalf("details = %v / %v", rows[0].Details, rows[1].Details)
	}
}

func TestDenyAggregation_CapWritesOldest(t *testing.T) {
	t.Parallel()
	w := newWorld(t, time.Minute, 2)
	for _, action := range []string{"a.one", "a.two", "a.one", "a.three"} {
		if err := w.rec.RecordDenial(as(alex), deny(action, auditing.ReasonNoGrant)); err != nil {
			t.Fatal(err)
		}
	}
	rows := w.store.committed()
	if len(rows) != 1 || rows[0].Action != "a.one" || aggregate(t, rows[0])["count"] != int64(2) {
		t.Fatalf("rows = %v; want the oldest bucket written at once", rows)
	}
}

func TestDenyAggregation_CapDropsOldestWhenWriteFails(t *testing.T) {
	t.Parallel()
	w := newWorld(t, time.Minute, 1)
	w.store.setFail(errors.New("database down"))
	for _, action := range []string{"a.one", "a.one", "a.two"} {
		if err := w.rec.RecordDenial(as(alex), deny(action, auditing.ReasonNoGrant)); err != nil {
			t.Fatalf("RecordDenial returned a storage error: %v", err)
		}
	}
	if !w.rep.has("dropped a.one no_grant") || w.rep.dropped != 2 {
		t.Fatalf("reports = %v, dropped %d", w.rep.events, w.rep.dropped)
	}
	w.store.setFail(nil)
	if err := w.agg.Flush(context.Background()); err != nil {
		t.Fatal(err)
	}
	if rows := w.store.committed(); len(rows) != 1 || rows[0].Action != "a.two" {
		t.Fatalf("rows = %v", rows)
	}
}

func TestDenyAggregation_FlushOnShutdown(t *testing.T) {
	t.Parallel()
	w := newWorld(t, time.Hour, 10)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- w.agg.Run(ctx) }()
	denyN(t, w, as(alex), 3, deny("grant.create", auditing.ReasonNoGrant))
	cancel()
	if err := <-done; err != nil {
		t.Fatalf("Run = %v", err)
	}
	if len(w.store.committed()) != 0 {
		t.Fatal("Run wrote before the window ended")
	}
	if err := w.agg.Flush(context.Background()); err != nil {
		t.Fatal(err)
	}
	if rows := w.store.committed(); len(rows) != 1 || aggregate(t, rows[0])["count"] != int64(3) {
		t.Fatalf("rows = %v", rows)
	}
}

func TestDenyAggregation_WriteFailureKeepsBuckets(t *testing.T) {
	t.Parallel()
	w := newWorld(t, time.Minute, 10)
	denyN(t, w, as(alex), 2, deny("grant.create", auditing.ReasonNoGrant))
	w.store.setFail(errors.New("database down"))
	w.clock.Set(t0.Add(time.Minute))
	if err := w.agg.FlushDue(context.Background()); err == nil {
		t.Fatal("FlushDue hid the write failure")
	}
	if !w.rep.has("write_failed") {
		t.Fatalf("reports = %v", w.rep.events)
	}
	if err := w.agg.Flush(context.Background()); err == nil {
		t.Fatal("Flush hid the write failure")
	}
	w.store.setFail(nil)
	if err := w.agg.FlushDue(context.Background()); err != nil {
		t.Fatal(err)
	}
	if rows := w.store.committed(); len(rows) != 1 || aggregate(t, rows[0])["count"] != int64(2) {
		t.Fatalf("rows = %v; the bucket must survive the failed writes", rows)
	}
}

func TestDenyAggregation_RunWritesAtWindowEnd(t *testing.T) {
	t.Parallel()
	w := newWorldWith(t, 50*time.Millisecond, 10, clock.System())
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { _ = w.agg.Run(ctx) }()
	if err := w.rec.RecordDenial(as(alex), deny("grant.create", auditing.ReasonNoGrant)); err != nil {
		t.Fatal(err)
	}
	deadline := testkit.Deadline()
	for len(w.store.committed()) == 0 {
		if time.Now().After(deadline) {
			t.Fatal("Run did not write the bucket at the end of its window")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestDenyAggregation_CloseReportsWhatItCannotWrite(t *testing.T) {
	t.Parallel()
	w := newWorld(t, time.Hour, 10)
	denyN(t, w, as(alex), 3, deny("grant.create", auditing.ReasonNoGrant))
	w.store.setFail(errors.New("database down"))
	if err := w.agg.Close(context.Background()); err == nil {
		t.Fatal("Close hid the write failure")
	}
	if !w.rep.has("dropped grant.create no_grant") || w.rep.dropped != 3 {
		t.Fatalf("reports = %v, dropped %d; the lost denials must be reported", w.rep.events, w.rep.dropped)
	}
	w.store.setFail(nil)
	if err := w.agg.Flush(context.Background()); err != nil || len(w.store.committed()) != 0 {
		t.Fatalf("buckets survived Close: %v", w.store.committed())
	}
}
