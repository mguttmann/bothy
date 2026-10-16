// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"bytes"
	"context"
	"errors"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/audit/app"
	"github.com/OWNER/bothy/services/server/internal/audit/domain"
	"github.com/OWNER/bothy/services/server/internal/instance"
	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
)

// fakeStore is app.Store in memory with transaction semantics: rows inserted inside a unit of
// the fakeUoW become visible at its commit.
type fakeStore struct {
	// Store holds the methods of M1-50 these unit tests never call (nil: a call panics); the
	// integration tests of the verification run them on PostgreSQL.
	app.Store
	mu       sync.Mutex
	rows     []domain.Row
	keys     []domain.IPKey
	failRows error // returned by InsertRows while set
	inserts  int   // InsertIPKey calls
}

// staged holds the rows of one unit until its commit.
var staged = txn.NewKey[*[]domain.Row]("staged rows")

func (s *fakeStore) InsertRows(ctx context.Context, rows []domain.Row) error {
	s.mu.Lock()
	err := s.failRows
	s.mu.Unlock()
	if err != nil {
		return err
	}
	p, ok := txn.Load(ctx, staged)
	if !ok {
		return errors.New("fake store: InsertRows outside a unit")
	}
	*p = append(*p, rows...)
	return nil
}

func (s *fakeStore) HashAt(_ context.Context, seq int64) ([domain.HashSize]byte, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, r := range s.rows {
		if r.Seq == seq {
			return r.Hash, true, nil
		}
	}
	return [domain.HashSize]byte{}, false, nil
}

func (s *fakeStore) CurrentIPKey(_ context.Context, at time.Time) (domain.IPKey, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, k := range s.keys {
		if !at.Before(k.ValidFrom) && at.Before(k.ValidUntil) {
			return k, true, nil
		}
	}
	return domain.IPKey{}, false, nil
}

func (s *fakeStore) InsertIPKey(_ context.Context, k domain.IPKey) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.inserts++
	for _, have := range s.keys {
		if have.ValidFrom.Equal(k.ValidFrom) {
			return nil // ON CONFLICT (valid_from) DO NOTHING
		}
	}
	s.keys = append(s.keys, k)
	return nil
}

func (s *fakeStore) IPKeyByID(_ context.Context, id ids.ID) (domain.IPKey, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, k := range s.keys {
		if k.ID == id {
			return k, true, nil
		}
	}
	return domain.IPKey{}, false, nil
}

func (s *fakeStore) committed() []domain.Row {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.rows)
}

func (s *fakeStore) setFail(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.failRows = err
}

// fakeCounters is instance.Counters over one number; the fakeUoW serialises units, so a rollback
// gives the numbers back.
type fakeCounters struct {
	mu    sync.Mutex
	value int64
	calls int
}

func (c *fakeCounters) Advance(ctx context.Context, name instance.CounterName, n int64) (int64, error) {
	if name != instance.CounterAuditSeq || txn.CurrentPhase(ctx) != txn.PhaseAudit {
		return 0, errors.New("fake counters: wrong counter or phase")
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.calls++
	c.value += n
	return c.value, nil
}

func (c *fakeCounters) Current(context.Context, instance.CounterName) (int64, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.value, nil
}

// fakeUoW runs the work phase, then the audit hook and the commit into the store one unit at a
// time; an error rolls back the staged rows and the counter.
type fakeUoW struct {
	mu       sync.Mutex
	reg      *txn.Registry
	store    *fakeStore
	counters *fakeCounters
}

func (u *fakeUoW) Do(ctx context.Context, fn func(context.Context) error, _ ...txn.Option) error {
	if txn.InUnit(ctx) {
		return fn(ctx)
	}
	unit, setPhase := txn.NewUnit(u.reg, txn.Options{})
	ctx = txn.WithUnit(ctx, unit)
	rows := &[]domain.Row{}
	if err := txn.Store(ctx, staged, rows); err != nil {
		return err
	}
	setPhase(txn.PhaseWork)
	if err := fn(ctx); err != nil {
		return err
	}
	// The audit hook and the commit run under one lock, as the row lock of audit_seq serialises
	// them in PostgreSQL.
	u.mu.Lock()
	defer u.mu.Unlock()
	u.counters.mu.Lock()
	before := u.counters.value
	u.counters.mu.Unlock()
	setPhase(txn.PhaseAudit)
	err := u.reg.Hook(txn.PhaseAudit)(ctx)
	if err != nil {
		u.counters.mu.Lock()
		u.counters.value = before
		u.counters.mu.Unlock()
		return err
	}
	u.store.mu.Lock()
	u.store.rows = append(u.store.rows, *rows...)
	u.store.mu.Unlock()
	return nil
}

// fakeKEK is app.KEKSource with one key per reference.
type fakeKEK map[string][]byte

func (f fakeKEK) KEK(_ context.Context, ref string) ([]byte, error) {
	k, ok := f[ref]
	if !ok {
		return nil, errors.New("fake KEK: unknown reference")
	}
	return k, nil
}

// kekRef is the configured reference of the tests.
const kekRef = "file:<secrets.dir>/audit_ip_kek"

// kekHex is the KEK of the tests as the secret file holds it.
var kekHex = []byte(strings.Repeat("3c", 32) + "\n")

// reporter records the reports of the module.
type reporter struct {
	mu      sync.Mutex
	events  []string
	dropped int64
}

func (r *reporter) add(s string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = append(r.events, s)
}

func (r *reporter) ChainBroken(context.Context, int64) { r.add("chain_broken") }
func (r *reporter) UnknownReason(_ context.Context, _, reason string) {
	r.add("unknown_reason " + reason)
}
func (r *reporter) WriteFailed(context.Context, error) { r.add("write_failed") }
func (r *reporter) IPKeyFailed(context.Context, error) { r.add("ip_key_failed") }
func (r *reporter) SinkNotConfigured(context.Context)  { r.add("sink_not_configured") }
func (r *reporter) VerifyFailed(_ context.Context, reason domain.FailureReason) {
	r.add("verify_failed " + string(reason))
}
func (r *reporter) ExportCut(context.Context, error)    { r.add("export_cut") }
func (r *reporter) RecordFailed(context.Context, error) { r.add("record_failed") }
func (r *reporter) DenialsDropped(_ context.Context, action, reason string, count int64) {
	r.mu.Lock()
	r.dropped += count
	r.mu.Unlock()
	r.add("dropped " + action + " " + reason)
}

func (r *reporter) has(prefix string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return slices.ContainsFunc(r.events, func(e string) bool { return strings.HasPrefix(e, prefix) })
}

// world is a recorder over the fakes.
type world struct {
	store    *fakeStore
	counters *fakeCounters
	uow      *fakeUoW
	clock    *clock.Fake
	rep      *reporter
	rec      *app.Recorder
	agg      *app.Aggregator
	keyring  *app.Keyring
}

// t0 is the start time of the fake clock.
var t0 = time.Date(2026, 10, 12, 8, 0, 0, 0, time.UTC)

func newWorld(t *testing.T, window time.Duration, maxKeys int) *world {
	t.Helper()
	fake := clock.NewFake(t0)
	w := newWorldWith(t, window, maxKeys, fake)
	w.clock = fake
	return w
}

// newWorldWith is newWorld with the clock clk.
func newWorldWith(t *testing.T, window time.Duration, maxKeys int, clk clock.Clock) *world {
	t.Helper()
	w := &world{store: &fakeStore{}, counters: &fakeCounters{}, rep: &reporter{}}
	chain := app.NewChain(w.counters, w.store, w.rep)
	reg := txn.NewRegistry()
	if err := reg.SetHook(txn.PhaseAudit, chain.Hook); err != nil {
		t.Fatal(err)
	}
	reg.Seal()
	w.uow = &fakeUoW{reg: reg, store: w.store, counters: w.counters}
	cfg := app.Config{
		DenyAggregateWindow: window, DenyAggregateMaxKeys: maxKeys,
		IPKeyPeriod: 30 * 24 * time.Hour, IPKeyRetention: 90 * 24 * time.Hour, IPKEKRef: kekRef,
	}
	var err error
	w.rec, err = app.NewRecorder(cfg, app.Deps{
		UoW: w.uow, Store: w.store, KEK: fakeKEK{kekRef: kekHex}, Clock: clk, Reporter: w.rep,
	})
	if err != nil {
		t.Fatal(err)
	}
	w.agg = w.rec.Aggregator()
	w.keyring = w.rec.Keyring()
	return w
}

// equalBytes is bytes.Equal, named for readability in the key tests.
func equalBytes(a, b []byte) bool { return bytes.Equal(a, b) }
