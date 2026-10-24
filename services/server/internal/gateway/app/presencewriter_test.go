// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"context"
	"errors"
	gosync "sync"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/gateway/app"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/testkit"
)

// flushCall is one Flush of the fake store.
type flushCall struct {
	node            ids.ID
	rows            []app.PresenceRow
	at, staleBefore time.Time
}

// fakePresenceStore records the writes of the presence writer; fail makes every write fail.
type fakePresenceStore struct {
	mu      gosync.Mutex
	flushes []flushCall
	deleted []ids.ID
	fail    bool
	flushed chan struct{}
}

func newFakePresenceStore() *fakePresenceStore {
	return &fakePresenceStore{flushed: make(chan struct{}, 64)}
}

func (s *fakePresenceStore) Flush(_ context.Context, node ids.ID, rows []app.PresenceRow, at, staleBefore time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.fail {
		return errors.New("database down")
	}
	s.flushes = append(s.flushes, flushCall{node: node, rows: rows, at: at, staleBefore: staleBefore})
	select {
	case s.flushed <- struct{}{}:
	default:
	}
	return nil
}

func (s *fakePresenceStore) DeleteNode(_ context.Context, node ids.ID) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.fail {
		return errors.New("database down")
	}
	s.deleted = append(s.deleted, node)
	return nil
}

// countingPresenceMetrics counts the failed writes.
type countingPresenceMetrics struct {
	mu gosync.Mutex
	n  int
}

func (m *countingPresenceMetrics) PresenceWriteFailed(context.Context) {
	m.mu.Lock()
	m.n++
	m.mu.Unlock()
}

func (m *countingPresenceMetrics) count() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.n
}

const (
	presenceFlush = time.Hour // the loop never ticks again within a test; Flush is called directly
	presenceTTL   = 10 * time.Second
)

// TestPresenceWriter_FlushWritesTheCountedConnections: a flush writes one row per counted live
// connection of the hub (the human with its device, focus and idle; no agent), with updated_at =
// now of the clock and the stale bound now - cluster.presence_ttl, under the node of the process.
func TestPresenceWriter_FlushWritesTheCountedConnections(t *testing.T) {
	t.Parallel()
	e := newFocusEnv(t)
	alex := e.newPerson()
	th := e.w.thread(e.support)
	e.w.grant(alex.ID, th.ChannelID, levelRead)
	cl := e.join(alex)
	e.join(e.newAgent())
	e.focus(cl, true, th.ID)
	store, node := newFakePresenceStore(), ids.New()
	w, err := app.NewPresenceWriter(app.PresenceWriterConfig{Node: node, FlushInterval: presenceFlush, TTL: presenceTTL}, e.hub, store, e.clk, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := w.Flush(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(store.flushes) != 1 {
		t.Fatalf("%d flushes", len(store.flushes))
	}
	f := store.flushes[0]
	if f.node != node || !f.at.Equal(e.clk.Now()) || !f.staleBefore.Equal(e.clk.Now().Add(-presenceTTL)) || len(f.rows) != 1 {
		t.Fatalf("flush %+v", f)
	}
	r := f.rows[0]
	if r.PrincipalID != alex.ID || r.DeviceID != alex.DeviceID || r.SessionID.IsZero() || !r.Idle || len(r.Focus) != 1 || r.Focus[0] != th.ID {
		t.Fatalf("row %+v", r)
	}
}

// TestPresenceWriter_FailureIsCountedAndNeverBlocks: a failed write is returned, logged and
// counted, and the connections stay open; the next flush after the database came back writes the
// rows again.
func TestPresenceWriter_FailureIsCountedAndNeverBlocks(t *testing.T) {
	t.Parallel()
	e := newFocusEnv(t)
	cl := e.join(e.newPerson())
	store, metrics := newFakePresenceStore(), &countingPresenceMetrics{}
	store.fail = true
	w, err := app.NewPresenceWriter(app.PresenceWriterConfig{Node: ids.New(), FlushInterval: presenceFlush, TTL: presenceTTL}, e.hub, store, e.clk, nil, metrics)
	if err != nil {
		t.Fatal(err)
	}
	if err := w.Flush(context.Background()); err == nil || metrics.count() != 1 {
		t.Fatalf("flush %v, failures %d", err, metrics.count())
	}
	wantOpen(t, cl.fc)
	store.mu.Lock()
	store.fail = false
	store.mu.Unlock()
	if err := w.Flush(context.Background()); err != nil || len(store.flushes) != 1 || len(store.flushes[0].rows) != 1 {
		t.Fatalf("flush after recovery %v %+v", err, store.flushes)
	}
}

// TestPresenceWriter_StartAndStop: Start flushes at once; Stop ends the loop and deletes the rows
// of the node once.
func TestPresenceWriter_StartAndStop(t *testing.T) {
	t.Parallel()
	e := newFocusEnv(t)
	store, node := newFakePresenceStore(), ids.New()
	w, err := app.NewPresenceWriter(app.PresenceWriterConfig{Node: node, FlushInterval: presenceFlush, TTL: presenceTTL}, e.hub, store, e.clk, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	w.Start()
	testkit.Receive(t, "the first flush", store.flushed)
	if err := w.Stop(testkit.Context(t)); err != nil {
		t.Fatal(err)
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	if len(store.deleted) != 1 || store.deleted[0] != node {
		t.Fatalf("deleted %v", store.deleted)
	}
}

// TestPresenceWriter_RefusesAnIncompleteConfiguration: the node, the hub, the store and positive
// durations are required.
func TestPresenceWriter_RefusesAnIncompleteConfiguration(t *testing.T) {
	t.Parallel()
	e := newFocusEnv(t)
	store := newFakePresenceStore()
	for name, cfg := range map[string]app.PresenceWriterConfig{
		"zero node":     {FlushInterval: presenceFlush, TTL: presenceTTL},
		"zero interval": {Node: ids.New(), TTL: presenceTTL},
		"zero ttl":      {Node: ids.New(), FlushInterval: presenceFlush},
	} {
		if _, err := app.NewPresenceWriter(cfg, e.hub, store, nil, nil, nil); err == nil {
			t.Errorf("%s accepted", name)
		}
	}
	if _, err := app.NewPresenceWriter(app.PresenceWriterConfig{Node: ids.New(), FlushInterval: presenceFlush, TTL: presenceTTL}, nil, store, nil, nil, nil); err == nil {
		t.Error("no hub accepted")
	}
}
