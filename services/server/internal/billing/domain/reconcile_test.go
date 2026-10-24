// SPDX-License-Identifier: Apache-2.0

package domain_test

import (
	"math"
	"math/big"
	"math/rand/v2"
	"reflect"
	"slices"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/billing/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

var (
	day   = time.Date(2026, 10, 20, 0, 0, 0, 0, time.UTC)
	runA  = ids.MustParse("0192f4a0-7b1c-7000-8000-00000000a001")
	runB  = ids.MustParse("0192f4a0-7b1c-7000-8000-00000000a002")
	idOne = ids.MustParse("0192f4a0-7b1c-7000-8000-000000000001")
	idTwo = ids.MustParse("0192f4a0-7b1c-7000-8000-000000000002")
	idSix = ids.MustParse("0192f4a0-7b1c-7000-8000-000000000003")
)

// request is a reconciliation of day for provider example with the entries.
func request(entries ...domain.RequestEntry) domain.Request {
	return domain.Request{Provider: "example", PeriodStart: day, PeriodEnd: day.Add(24 * time.Hour), Entries: entries}
}

// TestValidateReconcile (M2-28 design note 3 b): the period and the repeated run IDs, every
// violation at once with the pointer of the later occurrence.
func TestValidateReconcile(t *testing.T) {
	ok := request(domain.RequestEntry{RunID: runA, AmountMicros: 1, Currency: "USD"})
	if v := domain.Validate(ok); len(v) != 0 {
		t.Fatalf("valid request: %+v", v)
	}
	equal := ok
	equal.PeriodEnd = equal.PeriodStart
	after := ok
	after.PeriodEnd = equal.PeriodStart.Add(-time.Second)
	for name, req := range map[string]domain.Request{"equal": equal, "after": after} {
		if got, want := domain.Validate(req), []domain.Violation{{Pointer: "/period_end", Code: "invalid"}}; !reflect.DeepEqual(got, want) {
			t.Errorf("%s: %+v, want %+v", name, got, want)
		}
	}
	repeated := request(
		domain.RequestEntry{RunID: runA, AmountMicros: 1, Currency: "USD"},
		domain.RequestEntry{RunID: runB, AmountMicros: 1, Currency: "USD"},
		domain.RequestEntry{RunID: runA, AmountMicros: 2, Currency: "USD"},
	)
	if got, want := domain.Validate(repeated), []domain.Violation{{Pointer: "/entries/2/run_id", Code: "invalid"}}; !reflect.DeepEqual(got, want) {
		t.Errorf("repeated: %+v, want %+v", got, want)
	}
	both := repeated
	both.PeriodEnd = both.PeriodStart
	both.Entries = append(both.Entries, domain.RequestEntry{RunID: runB, AmountMicros: domain.MaxAmount + 1, Currency: "USD"})
	want := []domain.Violation{
		{Pointer: "/period_end", Code: "invalid"},
		{Pointer: "/entries/2/run_id", Code: "invalid"},
		{Pointer: "/entries/3/run_id", Code: "invalid"},
		{Pointer: "/entries/3/amount_micros", Code: "too_large"},
	}
	if got := domain.Validate(both); !reflect.DeepEqual(got, want) {
		t.Errorf("together: %+v, want %+v", got, want)
	}
}

// costEntry is a valid estimate of runA in mode organization of provider example in USD.
func costEntry(id ids.ID, amount int64, at time.Time) domain.Entry {
	return domain.Entry{ID: id, RunID: runA, BillingMode: "organization", Provider: "example", Model: "model-large",
		AmountMicros: amount, Currency: "USD", IsEstimate: true, Source: "computed", OccurredAt: at}
}

// reconciledEntry is costEntry as a reconciled correction.
func reconciledEntry(id ids.ID, amount int64, at time.Time) domain.Entry {
	e := costEntry(id, amount, at)
	e.IsEstimate, e.Source = false, "reconciled"
	return e
}

// TestMatch (M2-28 design note 4): one row per rule.
func TestMatch(t *testing.T) {
	in := day.Add(time.Hour)
	out := day.Add(-time.Hour)
	with := func(mod func(e *domain.Entry)) domain.Entry {
		e := costEntry(idOne, 100, in)
		mod(&e)
		return e
	}
	cases := []struct {
		name    string
		valid   []domain.Entry
		amount  int64
		matched bool
		noop    bool
		reason  domain.UnmatchedReason
	}{
		{"no entry", nil, 10, false, false, domain.UnmatchedNoEntries},
		{"no entry in the period", []domain.Entry{costEntry(idOne, 100, out)}, 10, false, false, domain.UnmatchedNoEntries},
		{"end of the period is exclusive", []domain.Entry{costEntry(idOne, 100, day.Add(24*time.Hour))}, 10, false, false, domain.UnmatchedNoEntries},
		{"personal", []domain.Entry{with(func(e *domain.Entry) { e.BillingMode = "personal" })}, 10, false, false, domain.UnmatchedPersonal},
		{"another provider", []domain.Entry{with(func(e *domain.Entry) { e.Provider = "other" })}, 10, false, false, domain.UnmatchedProvider},
		{"another currency", []domain.Entry{with(func(e *domain.Entry) { e.Currency = "EUR" })}, 10, false, false, domain.UnmatchedCurrency},
		{"mixed currencies", []domain.Entry{costEntry(idOne, 100, in), with(func(e *domain.Entry) { e.ID, e.Currency = idTwo, "EUR" })}, 10, false, false,
			domain.UnmatchedCurrency},
		{"another run", []domain.Entry{with(func(e *domain.Entry) { e.RunID = runB })}, 10, false, false, domain.UnmatchedNoEntries},
		{"estimate in the period", []domain.Entry{costEntry(idOne, 100, in)}, 10, true, false, ""},
		{"partly outside: only the inside counts", []domain.Entry{costEntry(idOne, 100, in), with(func(e *domain.Entry) {
			e.ID, e.OccurredAt, e.BillingMode = idTwo, out, "personal"
		})}, 10, true, false, ""},
		{"reconciled with the same amount", []domain.Entry{reconciledEntry(idOne, 4, in), reconciledEntry(idTwo, 6, in)}, 10, true, true, ""},
		{"reconciled with another amount", []domain.Entry{reconciledEntry(idOne, 4, in), reconciledEntry(idTwo, 6, in)}, 11, true, false, ""},
		{"reconciled and an estimate", []domain.Entry{reconciledEntry(idOne, 10, in), costEntry(idTwo, 0, in)}, 10, true, false, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			e := domain.RequestEntry{RunID: runA, AmountMicros: c.amount, Currency: "USD"}
			matched, noop, reason := domain.Match(c.valid, request(e), e)
			if matched != c.matched || noop != c.noop || reason != c.reason {
				t.Fatalf("got %v %v %q, want %v %v %q", matched, noop, reason, c.matched, c.noop, c.reason)
			}
		})
	}
}

// shares returns the shares as entry -> amount and checks their sum.
func shares(t *testing.T, amount int64, valid []domain.Entry) map[ids.ID]int64 {
	t.Helper()
	got := domain.Split(amount, valid)
	if len(got) != len(valid) {
		t.Fatalf("%d shares for %d entries", len(got), len(valid))
	}
	out := map[ids.ID]int64{}
	var sum int64
	for _, s := range got {
		if s.AmountMicros < 0 {
			t.Fatalf("negative share %+v", s)
		}
		out[s.EntryID] = s.AmountMicros
		sum += s.AmountMicros
	}
	if sum != amount {
		t.Fatalf("shares sum to %d, want %d", sum, amount)
	}
	return out
}

// TestSplit (M2-28 design note 5): proportional shares, the remainder on the largest entry with
// ties to the smallest id, all zero on the smallest id, and no overflow near the int64 limit.
func TestSplit(t *testing.T) {
	at := day.Add(time.Hour)
	if got := shares(t, 77, []domain.Entry{costEntry(idOne, 5, at)}); got[idOne] != 77 {
		t.Errorf("one entry: %v", got)
	}
	// 100 over 1:3 is 25 and 75 exactly.
	if got := shares(t, 100, []domain.Entry{costEntry(idOne, 10, at), costEntry(idTwo, 30, at)}); got[idOne] != 25 || got[idTwo] != 75 {
		t.Errorf("proportional: %v", got)
	}
	// 10 over 1:1:2 is 2, 2 and 5 and the remainder 1 goes to the largest, idSix.
	if got := shares(t, 10, []domain.Entry{costEntry(idOne, 1, at), costEntry(idTwo, 1, at), costEntry(idSix, 2, at)}); got[idSix] != 6 || got[idOne] != 2 ||
		got[idTwo] != 2 {
		t.Errorf("remainder on the largest: %v", got)
	}
	// A tie of the largest goes to the smallest id, also when it comes last.
	if got := shares(t, 10, []domain.Entry{costEntry(idTwo, 1, at), costEntry(idSix, 1, at), costEntry(idOne, 1, at)}); got[idOne] != 4 || got[idTwo] != 3 ||
		got[idSix] != 3 {
		t.Errorf("tie: %v", got)
	}
	if got := shares(t, 9, []domain.Entry{costEntry(idTwo, 0, at), costEntry(idOne, 0, at)}); got[idOne] != 9 || got[idTwo] != 0 {
		t.Errorf("all zero: %v", got)
	}
	if got := shares(t, 0, []domain.Entry{costEntry(idOne, 3, at), costEntry(idTwo, 5, at)}); got[idOne] != 0 || got[idTwo] != 0 {
		t.Errorf("amount 0: %v", got)
	}
	big1 := int64(math.MaxInt64 / 3)
	if got := shares(t, domain.MaxAmount, []domain.Entry{costEntry(idOne, big1, at), costEntry(idTwo, big1, at), costEntry(idSix, big1, at)}); got[idOne] <
		got[idTwo] {
		t.Errorf("near the limit: %v", got)
	}
}

// TestSplitProperty: for random amounts the shares sum to the amount, each differs from the exact
// proportion by less than one except the remainder entry, and the remainder sits on the largest
// entry with the smallest id. The seed is printed, so a failure is reproducible.
func TestSplitProperty(t *testing.T) {
	seed := uint64(time.Now().UnixNano())
	t.Logf("seed %d", seed)
	r := rand.New(rand.NewPCG(seed, 2028)) // #nosec G404 -- reproducible test data from a printed seed, not a secret
	at := day.Add(time.Hour)
	for range 2000 {
		n := 1 + r.IntN(6)
		valid := make([]domain.Entry, n)
		var sum int64
		for i := range valid {
			valid[i] = costEntry(ids.New(), r.Int64N(domain.MaxAmount/int64(n)), at)
			sum += valid[i].AmountMicros
		}
		amount := r.Int64N(domain.MaxAmount)
		got := shares(t, amount, valid)
		if sum == 0 {
			continue
		}
		top := slices.MinFunc(valid, func(a, b domain.Entry) int {
			if a.AmountMicros != b.AmountMicros {
				if a.AmountMicros > b.AmountMicros {
					return -1
				}
				return 1
			}
			return compareID(a.ID, b.ID)
		})
		for _, e := range valid {
			exact := new(big.Int).Quo(new(big.Int).Mul(big.NewInt(amount), big.NewInt(e.AmountMicros)), big.NewInt(sum))
			if e.ID != top.ID && got[e.ID] != exact.Int64() {
				t.Fatalf("seed %d: share %d, want %d", seed, got[e.ID], exact.Int64())
			}
		}
	}
}

// compareID orders ids by their text, which is the order of their bytes.
func compareID(a, b ids.ID) int {
	switch {
	case a.String() < b.String():
		return -1
	case a.String() > b.String():
		return 1
	}
	return 0
}
