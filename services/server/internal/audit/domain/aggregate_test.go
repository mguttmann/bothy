// SPDX-License-Identifier: Apache-2.0

package domain_test

import (
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/audit/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

var t0 = time.Date(2026, 10, 12, 8, 0, 0, 0, time.UTC)

func key(action string) domain.AggregateKey {
	return domain.AggregateKey{Principal: ids.MustParse(uuidA), Action: action, Reason: "no_grant"}
}

func TestAggregation_SameKeyWithinWindowCounts(t *testing.T) {
	t.Parallel()
	a := domain.NewAggregation[string](time.Minute, 10)
	for i := range 5 {
		if _, evicted := a.Add(key("grant.create"), t0.Add(time.Duration(i)*10*time.Second), "first"); evicted {
			t.Fatal("eviction below the cap")
		}
	}
	if a.Len() != 1 {
		t.Fatalf("Len = %d, want 1", a.Len())
	}
	if due := a.TakeDue(t0.Add(59 * time.Second)); len(due) != 0 {
		t.Fatalf("a bucket was due before its window ended: %v", due)
	}
	if d, ok := a.NextDeadline(); !ok || !d.Equal(t0.Add(time.Minute)) {
		t.Fatalf("NextDeadline = %v, %v", d, ok)
	}
	due := a.TakeDue(t0.Add(time.Minute))
	if len(due) != 1 {
		t.Fatalf("due = %v", due)
	}
	b := due[0]
	if b.Count != 5 || !b.FirstAt.Equal(t0) || !b.LastAt.Equal(t0.Add(40*time.Second)) || b.First != "first" {
		t.Fatalf("bucket = %+v", b)
	}
	if a.Len() != 0 {
		t.Fatal("TakeDue left the bucket behind")
	}
	if _, ok := a.NextDeadline(); ok {
		t.Fatal("NextDeadline without buckets")
	}
}

func TestAggregation_AfterWindowNewBucket(t *testing.T) {
	t.Parallel()
	a := domain.NewAggregation[int](time.Minute, 10)
	a.Add(key("x"), t0, 1)
	a.Add(key("x"), t0.Add(time.Minute), 2) // window [t0, t0+1m) is over
	a.Add(key("x"), t0.Add(70*time.Second), 3)
	due := a.TakeDue(t0.Add(time.Minute))
	if len(due) != 1 || due[0].First != 1 || due[0].Count != 1 {
		t.Fatalf("first window = %+v", due)
	}
	rest := a.TakeAll()
	if len(rest) != 1 || rest[0].First != 2 || rest[0].Count != 2 || !rest[0].FirstAt.Equal(t0.Add(time.Minute)) {
		t.Fatalf("second window = %+v", rest)
	}
}

func TestAggregation_DifferentKeysSeparate(t *testing.T) {
	t.Parallel()
	a := domain.NewAggregation[int](time.Minute, 10)
	other := key("x")
	other.Principal = ids.MustParse(uuidB)
	anon1 := domain.AggregateKey{IPHMAC: "aa", Action: "x", Reason: "no_grant"}
	anon2 := domain.AggregateKey{IPHMAC: "bb", Action: "x", Reason: "no_grant"}
	reason := key("x")
	reason.Reason = "owner_only"
	for _, k := range []domain.AggregateKey{key("x"), key("y"), other, reason, anon1, anon2, anon1} {
		a.Add(k, t0, 0)
	}
	if a.Len() != 6 {
		t.Fatalf("Len = %d, want 6", a.Len())
	}
}

func TestAggregation_CapEvictsOldest(t *testing.T) {
	t.Parallel()
	a := domain.NewAggregation[int](time.Minute, 2)
	a.Add(key("a"), t0, 1)
	a.Add(key("b"), t0.Add(time.Second), 2)
	if _, evicted := a.Add(key("a"), t0.Add(2*time.Second), 0); evicted {
		t.Fatal("an existing key evicted a bucket")
	}
	b, evicted := a.Add(key("c"), t0.Add(3*time.Second), 3)
	if !evicted || b.First != 1 || b.Count != 2 {
		t.Fatalf("evicted = %+v, %v; want the oldest bucket a with count 2", b, evicted)
	}
	if a.Len() != 2 {
		t.Fatalf("Len = %d", a.Len())
	}
}

func TestAggregation_RestoreKeepsBucketsUpToTheCap(t *testing.T) {
	t.Parallel()
	a := domain.NewAggregation[int](time.Minute, 2)
	a.Add(key("a"), t0, 1)
	taken := a.TakeAll()
	a.Add(key("b"), t0.Add(time.Second), 2)
	if dropped := a.Restore(taken); len(dropped) != 0 {
		t.Fatalf("dropped %v below the cap", dropped)
	}
	if d, _ := a.NextDeadline(); !d.Equal(t0) { // closed, so due at once
		t.Fatalf("restored bucket not due first: %v", d)
	}
	a.Add(key("c"), t0.Add(2*time.Second), 3) // evicts the restored a
	more := []domain.Bucket[int]{{Key: key("d"), First: 4, FirstAt: t0, LastAt: t0, Count: 1}}
	dropped := a.Restore(more)
	if len(dropped) != 1 || dropped[0].First != 4 {
		t.Fatalf("dropped = %+v, want the restored oldest beyond the cap", dropped)
	}
	if a.Len() != 2 {
		t.Fatalf("Len = %d", a.Len())
	}
}
