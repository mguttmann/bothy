// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/idempotency"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
	"github.com/OWNER/bothy/services/server/internal/sync/app"
)

// fakeKeyRows is an in-memory KeyRows with programmable answers.
type fakeKeyRows struct {
	insertClaimed []bool // answers of Insert, in order; the last one repeats
	insertErr     error
	getEntry      idempotency.Entry
	getFound      []bool // answers of Get, in order; the last one repeats
	completeOK    bool
	deleteErr     error
	batches       []int64 // answers of DeleteExpiredBatch, in order
	calls         []string
	inUnit        []bool // whether each call ran inside a unit of work
}

func (f *fakeKeyRows) note(ctx context.Context, name string) {
	f.calls = append(f.calls, name)
	f.inUnit = append(f.inUnit, txn.InUnit(ctx))
}

func next[T any](xs []T, i int) T {
	if i < len(xs) {
		return xs[i]
	}
	return xs[len(xs)-1]
}

func (f *fakeKeyRows) count(name string) int {
	n := 0
	for _, c := range f.calls {
		if c == name {
			n++
		}
	}
	return n
}

func (f *fakeKeyRows) DeleteExpired(ctx context.Context, _, _ ids.ID, _ time.Time) error {
	f.note(ctx, "DeleteExpired")
	return f.deleteErr
}

func (f *fakeKeyRows) Insert(ctx context.Context, _ idempotency.Request) (bool, error) {
	i := f.count("Insert")
	f.note(ctx, "Insert")
	if f.insertErr != nil {
		return false, f.insertErr
	}
	return next(f.insertClaimed, i), nil
}

func (f *fakeKeyRows) Get(ctx context.Context, _, _ ids.ID) (idempotency.Entry, bool, error) {
	i := f.count("Get")
	f.note(ctx, "Get")
	found := next(f.getFound, i)
	if !found {
		return idempotency.Entry{}, false, nil
	}
	return f.getEntry, true, nil
}

func (f *fakeKeyRows) Complete(ctx context.Context, _ idempotency.Request, _ idempotency.Response) (bool, error) {
	f.note(ctx, "Complete")
	return f.completeOK, nil
}

func (f *fakeKeyRows) SaveFinal(ctx context.Context, _ idempotency.Request, _ idempotency.Response) error {
	f.note(ctx, "SaveFinal")
	return nil
}

func (f *fakeKeyRows) DeleteExpiredBatch(ctx context.Context, _ time.Time, _ int) (int64, error) {
	i := f.count("DeleteExpiredBatch")
	f.note(ctx, "DeleteExpiredBatch")
	return next(f.batches, i), nil
}

func keyRequest() idempotency.Request {
	return idempotency.Request{
		PrincipalID: ids.New(), Key: ids.New(), Action: "message.create",
		Now: time.Date(2026, 10, 14, 12, 0, 0, 0, time.UTC), TTL: 24 * time.Hour,
	}
}

func wantInternal(t *testing.T, err error) {
	t.Helper()
	if errs.KindOf(err) != errs.KindInternal {
		t.Fatalf("err = %v, want an internal error", err)
	}
}

func TestIdempotencyStore_ClaimFreshKey(t *testing.T) {
	t.Parallel()
	rows := &fakeKeyRows{insertClaimed: []bool{true}}
	s := app.NewIdempotencyStore(rows, &fakeUoW{})
	ctx, _ := unitCtx(txn.PhaseWork)
	_, found, err := s.Claim(ctx, keyRequest())
	if err != nil || found {
		t.Fatalf("Claim = found %v, %v; want a claimed key", found, err)
	}
	if want := []string{"DeleteExpired", "Insert"}; !equalCalls(rows.calls, want) {
		t.Fatalf("calls = %v, want %v", rows.calls, want)
	}
}

func TestIdempotencyStore_ClaimTakenKeyReturnsTheRow(t *testing.T) {
	t.Parallel()
	entry := idempotency.Entry{Response: idempotency.Response{Status: 201}, Completed: true}
	rows := &fakeKeyRows{insertClaimed: []bool{false}, getFound: []bool{true}, getEntry: entry}
	s := app.NewIdempotencyStore(rows, &fakeUoW{})
	ctx, _ := unitCtx(txn.PhaseWork)
	got, found, err := s.Claim(ctx, keyRequest())
	if err != nil || !found || got.Status != 201 || !got.Completed {
		t.Fatalf("Claim = %+v, %v, %v; want the committed row", got, found, err)
	}
}

func TestIdempotencyStore_ClaimRetriesOnceWhenTheRowVanished(t *testing.T) {
	t.Parallel()
	// The first claim finds the key taken, but the row is gone before it is read (cleanup); the
	// second attempt claims the key.
	rows := &fakeKeyRows{insertClaimed: []bool{false, true}, getFound: []bool{false}}
	s := app.NewIdempotencyStore(rows, &fakeUoW{})
	ctx, _ := unitCtx(txn.PhaseWork)
	if _, found, err := s.Claim(ctx, keyRequest()); err != nil || found {
		t.Fatalf("Claim = found %v, %v; want a claimed key on the second attempt", found, err)
	}
	if rows.count("Insert") != 2 {
		t.Fatalf("Insert ran %d times, want 2", rows.count("Insert"))
	}
}

func TestIdempotencyStore_ClaimGivesUpAfterTwoVanishedRows(t *testing.T) {
	t.Parallel()
	rows := &fakeKeyRows{insertClaimed: []bool{false}, getFound: []bool{false}}
	s := app.NewIdempotencyStore(rows, &fakeUoW{})
	ctx, _ := unitCtx(txn.PhaseWork)
	_, _, err := s.Claim(ctx, keyRequest())
	wantInternal(t, err)
}

func TestIdempotencyStore_ClaimPassesStoreErrors(t *testing.T) {
	t.Parallel()
	ctx, _ := unitCtx(txn.PhaseWork)
	rows := &fakeKeyRows{insertErr: idempotency.ErrInProgress}
	if _, _, err := app.NewIdempotencyStore(rows, &fakeUoW{}).Claim(ctx, keyRequest()); !errors.Is(err, idempotency.ErrInProgress) {
		t.Fatalf("Claim err = %v, want ErrInProgress", err)
	}
	boom := errors.New("boom")
	rows = &fakeKeyRows{deleteErr: boom}
	if _, _, err := app.NewIdempotencyStore(rows, &fakeUoW{}).Claim(ctx, keyRequest()); !errors.Is(err, boom) {
		t.Fatalf("Claim err = %v, want the DeleteExpired error", err)
	}
}

func TestIdempotencyStore_RefusesInvalidRequests(t *testing.T) {
	t.Parallel()
	s := app.NewIdempotencyStore(&fakeKeyRows{insertClaimed: []bool{true}}, &fakeUoW{})
	ctx, _ := unitCtx(txn.PhaseWork)
	_, _, err := s.Claim(context.Background(), keyRequest())
	wantInternal(t, err) // outside a unit of work
	noKey := keyRequest()
	noKey.Key = ids.ID{}
	_, _, err = s.Claim(ctx, noKey)
	wantInternal(t, err)
	noTTL := keyRequest()
	noTTL.TTL = 0
	_, _, err = s.Claim(ctx, noTTL)
	wantInternal(t, err)
	wantInternal(t, s.Complete(ctx, keyRequest(), idempotency.Response{Status: 99}))
	wantInternal(t, s.Complete(ctx, keyRequest(), idempotency.Response{Status: 40000}))
	wantInternal(t, s.SaveFinal(ctx, keyRequest(), idempotency.Response{Status: 0}))
}

func TestIdempotencyStore_Complete(t *testing.T) {
	t.Parallel()
	ctx, _ := unitCtx(txn.PhaseWork)
	ok := &fakeKeyRows{completeOK: true}
	if err := app.NewIdempotencyStore(ok, &fakeUoW{}).Complete(ctx, keyRequest(), idempotency.Response{Status: 200}); err != nil {
		t.Fatalf("Complete = %v", err)
	}
	notClaimed := &fakeKeyRows{completeOK: false}
	wantInternal(t, app.NewIdempotencyStore(notClaimed, &fakeUoW{}).Complete(ctx, keyRequest(), idempotency.Response{Status: 200}))
}

func TestIdempotencyStore_SaveFinalRunsInItsOwnUnit(t *testing.T) {
	t.Parallel()
	rows := &fakeKeyRows{}
	uow := &fakeUoW{}
	s := app.NewIdempotencyStore(rows, uow)
	if err := s.SaveFinal(context.Background(), keyRequest(), idempotency.Response{Status: 409}); err != nil {
		t.Fatalf("SaveFinal = %v", err)
	}
	if uow.runs != 1 || !equalCalls(rows.calls, []string{"DeleteExpired", "SaveFinal"}) {
		t.Fatalf("runs %d, calls %v; want one unit with DeleteExpired and SaveFinal", uow.runs, rows.calls)
	}
	for i, in := range rows.inUnit {
		if !in {
			t.Errorf("call %s ran outside the unit", rows.calls[i])
		}
	}
}

func TestNewIdempotencyCleanup_RejectsBadConfig(t *testing.T) {
	t.Parallel()
	c := clock.NewFake(time.Now())
	if _, err := app.NewIdempotencyCleanup(app.IdempotencyCleanupConfig{Interval: 0, BatchSize: 1}, &fakeUoW{}, &fakeKeyRows{}, c); err == nil {
		t.Error("interval 0 accepted")
	}
	if _, err := app.NewIdempotencyCleanup(app.IdempotencyCleanupConfig{Interval: time.Minute, BatchSize: 0}, &fakeUoW{}, &fakeKeyRows{}, c); err == nil {
		t.Error("batch size 0 accepted")
	}
}

func TestIdempotencyCleanup_RunsBatchesUntilAShortOne(t *testing.T) {
	t.Parallel()
	rows := &fakeKeyRows{batches: []int64{10, 10, 3}}
	uow := &fakeUoW{}
	c, err := app.NewIdempotencyCleanup(app.IdempotencyCleanupConfig{Interval: time.Minute, BatchSize: 10}, uow, rows, clock.NewFake(time.Now()))
	if err != nil {
		t.Fatal(err)
	}
	if c.Interval() != time.Minute {
		t.Errorf("Interval = %v", c.Interval())
	}
	res, err := c.Run(context.Background())
	if err != nil || res.Deleted != 23 || res.Batches != 3 || uow.runs != 3 {
		t.Fatalf("Run = %+v, %v, %d units; want 23 rows in 3 batches", res, err, uow.runs)
	}
}

func TestIdempotencyCleanup_StopsWhenTheContextEnds(t *testing.T) {
	t.Parallel()
	c, err := app.NewIdempotencyCleanup(app.IdempotencyCleanupConfig{Interval: time.Minute, BatchSize: 10}, &fakeUoW{}, &fakeKeyRows{batches: []int64{10}}, clock.NewFake(time.Now()))
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := c.Run(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("Run err = %v, want context.Canceled", err)
	}
}

func equalCalls(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}
