// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"log/slog"
	"time"

	"github.com/OWNER/bothy/services/server/internal/agents/domain"
	"github.com/OWNER/bothy/services/server/internal/identity"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// StopStepName is the name of the stop step of the tracker (D-M2-20-5).
const StopStepName = "agent_presence"

// Name is the name of the tracker as a component of the start sequence (platform/lifecycle,
// adapted in internal/wire).
func (t *Tracker) Name() string { return StopStepName }

// Start is the start of the tracker as a component of the start sequence (D-M2-20-6): it loads the agents stored as online or
// degraded with their principal (handle, status), applies the start rule to each and starts the
// flush loop. An agent whose stored heartbeat is missing or older than agent.presence_timeout
// plus agent.heartbeat_persist_interval is offline at once, stored by the first flush; the others
// stay as stored until the start plus agent.presence_timeout. Nothing is published: no
// connection exists yet. Errors: the reads'.
func (t *Tracker) Start(ctx context.Context) error {
	rows, err := t.d.Store.ListLivePresence(ctx)
	if err != nil {
		return err
	}
	list := make([]ids.ID, 0, len(rows))
	for _, r := range rows {
		list = append(list, r.ID)
	}
	principals, err := t.d.Principals.Principals(ctx, list)
	if err != nil {
		return err
	}
	now := t.d.Clock.Now()
	for _, row := range rows {
		p, ok := principals[row.ID]
		if !ok {
			continue
		}
		t.load(row, p, now)
	}
	t.scheduleFlush()
	return nil
}

// load creates the entry of a live row at start.
func (t *Tracker) load(row domain.PresenceRow, p identity.PrincipalView, now time.Time) {
	e := &entry{id: row.ID, handle: p.Handle, active: p.Status == identity.StatusActive, loaded: true,
		target: row.Target, report: row.Stored, state: row.Stored.State, last: row.LastHeartbeatAt}
	e.pubState, e.pubMode = e.state, row.Stored.BillingMode
	offline, deadline := domain.StartRule(domain.StoredPresence{State: row.Stored.State, LastHeartbeatAt: row.LastHeartbeatAt}, now,
		t.cfg.PresenceTimeout, t.cfg.PersistInterval)
	if offline {
		e.state, e.dirty, e.pubState = domain.RuntimeOffline, true, domain.RuntimeOffline
	} else {
		e.deadline = deadline
		t.arm(e, now)
	}
	t.reassess(e, now)
	t.mu.Lock()
	t.entries[row.ID] = e
	t.mu.Unlock()
}

// scheduleFlush arms the next run of the flush loop after agent.heartbeat_persist_interval.
func (t *Tracker) scheduleFlush() {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.stopped.Load() {
		return
	}
	t.stopFlush = t.d.Timers.AfterFunc(t.cfg.PersistInterval, func() {
		if t.stopped.Load() {
			return
		}
		if err := t.Flush(context.Background()); err != nil {
			t.d.Log.LogAttrs(context.Background(), slog.LevelError, "agent presence flush failed")
		}
		t.scheduleFlush()
	})
}

// Flush writes every dirty entry with PersistHeartbeat in ascending agent id, all in one unit of
// work (D-M2-20-5). It takes each entry's lock only to copy its state; an entry is clean
// afterwards whether its row took the write or held a fresher heartbeat. When the unit of work
// fails the entries are dirty again for the next run. No counter, no NOTIFY, no audit, no event.
// Errors: the unit of work's.
func (t *Tracker) Flush(ctx context.Context) error {
	var writes []PresenceWrite
	var flushed []*entry
	for _, e := range t.sorted() {
		e.mu.Lock()
		if e.loaded && e.dirty {
			writes = append(writes, PresenceWrite{ID: e.id, State: e.state, Report: e.report, LastHeartbeatAt: e.last})
			flushed = append(flushed, e)
			e.dirty = false
		}
		e.mu.Unlock()
	}
	if len(writes) == 0 {
		return nil
	}
	err := t.d.UoW.Do(ctx, func(ctx context.Context) error {
		for _, w := range writes {
			if _, err := t.d.Store.PersistHeartbeat(ctx, w); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		for _, e := range flushed {
			e.mu.Lock()
			e.dirty = true
			e.mu.Unlock()
		}
	}
	return err
}

// Stop is the stop step agent_presence (D-M2-20-5; internal/wire registers it in the phase of the
// connections after the gateway's step, so no heartbeat arrives later): it stops the timers and
// the flush loop without publishing, then writes every dirty entry within the rest of
// shutdown.total_timeout; a restart of the server is not an agent going offline. Errors: the
// flush's.
func (t *Tracker) Stop(ctx context.Context) error {
	t.stopped.Store(true)
	t.mu.Lock()
	if t.stopFlush != nil {
		t.stopFlush()
	}
	t.mu.Unlock()
	for _, e := range t.sorted() {
		e.mu.Lock()
		if e.stop != nil {
			e.stop()
			e.stop = nil
		}
		e.mu.Unlock()
	}
	return t.Flush(ctx)
}
