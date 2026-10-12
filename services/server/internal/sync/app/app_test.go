// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/instance"
	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/eventlog"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
	"github.com/OWNER/bothy/services/server/internal/sync/app"
)

// fixedCorrelation is a Correlator with a fixed answer.
type fixedCorrelation struct {
	id    ids.ID
	trace string
}

func (f fixedCorrelation) Correlation(context.Context) (ids.ID, string) { return f.id, f.trace }

// unitCtx returns a context inside a new unit in phase p, and the setter of the phase.
func unitCtx(p txn.Phase) (context.Context, func(txn.Phase)) {
	reg := txn.NewRegistry()
	reg.Seal()
	u, set := txn.NewUnit(reg, txn.Options{})
	set(p)
	return txn.WithUnit(context.Background(), u), set
}

// event returns a valid event.
func event() eventlog.Event {
	return eventlog.Event{
		Type: "thread.closed", SchemaVersion: 1, ActorKind: eventlog.ActorUser, ActorPrincipalID: ids.New(),
		ResourceKind: "thread", ResourceID: ids.New(), ReceiveAction: "thread.read", Payload: []byte(`{"closed":true}`),
	}
}

func newAppender(t *testing.T, corr app.Correlator) *app.Appender {
	t.Helper()
	a, err := app.NewAppender(app.AppenderConfig{PayloadMaxBytes: 1024}, corr)
	if err != nil {
		t.Fatal(err)
	}
	return a
}

func TestAppend_RequiresUnitAndPhase(t *testing.T) {
	t.Parallel()
	a := newAppender(t, fixedCorrelation{})
	if err := a.Append(context.Background(), event()); errs.KindOf(err) != errs.KindInternal {
		t.Fatalf("Append outside a unit = %v, want kind internal", err)
	}
	for _, p := range []txn.Phase{txn.PhaseNone, txn.PhaseRightsBegin, txn.PhaseSync, txn.PhaseAudit} {
		ctx, _ := unitCtx(p)
		err := a.Append(ctx, event())
		if errs.KindOf(err) != errs.KindInternal || !strings.Contains(err.Error(), p.String()) {
			t.Errorf("Append in phase %s = %v, want kind internal naming the phase", p, err)
		}
		if got := app.Pending(ctx); len(got) != 0 {
			t.Errorf("phase %s buffered %d events", p, len(got))
		}
	}
	for _, p := range []txn.Phase{txn.PhaseWork, txn.PhaseRightsCheck} {
		ctx, _ := unitCtx(p)
		if err := a.Append(ctx, event()); err != nil {
			t.Errorf("Append in phase %s = %v", p, err)
		}
	}
}

func TestAppend_Validation(t *testing.T) {
	t.Parallel()
	a := newAppender(t, fixedCorrelation{})
	cases := []struct {
		name string
		edit func(*eventlog.Event)
	}{
		{"type without dot", func(e *eventlog.Event) { e.Type = "closed" }},
		{"schema version 0", func(e *eventlog.Event) { e.SchemaVersion = 0 }},
		{"empty resource kind", func(e *eventlog.Event) { e.ResourceKind = "" }},
		{"empty receive action", func(e *eventlog.Event) { e.ReceiveAction = "" }},
		{"self without audience", func(e *eventlog.Event) { e.ReceiveAction = "self" }},
		{"unknown actor kind", func(e *eventlog.Event) { e.ActorKind = "robot" }},
		{"user without principal", func(e *eventlog.Event) { e.ActorPrincipalID = ids.ID{} }},
		{"system with principal", func(e *eventlog.Event) { e.ActorKind = eventlog.ActorSystem }},
		{"principal without kind", func(e *eventlog.Event) { e.ActorKind = "" }},
		{"payload not an object", func(e *eventlog.Event) { e.Payload = []byte(`[]`) }},
		{"payload with nested null", func(e *eventlog.Event) { e.Payload = []byte(`{"a":{"b":null}}`) }},
		{"oversized payload", func(e *eventlog.Event) { e.Payload = []byte(`{"a":"` + strings.Repeat("x", 1024) + `"}`) }},
		{"missing payload", func(e *eventlog.Event) { e.Payload = nil }},
	}
	for _, c := range cases {
		ctx, _ := unitCtx(txn.PhaseWork)
		bad := event()
		c.edit(&bad)
		err := a.Append(ctx, event(), bad)
		if errs.KindOf(err) != errs.KindInternal {
			t.Errorf("%s: Append = %v, want kind internal", c.name, err)
		}
		if got := app.Pending(ctx); len(got) != 0 {
			t.Errorf("%s: %d events buffered; a rejected call buffers nothing", c.name, len(got))
		}
	}
}

func TestAppend_BuffersInOrderWithIDsAndCorrelation(t *testing.T) {
	t.Parallel()
	corr := fixedCorrelation{id: ids.New(), trace: "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01"}
	a := newAppender(t, corr)
	ctx, set := unitCtx(txn.PhaseWork)
	first, second, third := event(), event(), event()
	own := ids.New()
	second.ID = own
	if err := a.Append(ctx, first, second); err != nil {
		t.Fatal(err)
	}
	set(txn.PhaseRightsCheck)
	third.Type = "access.changed"
	if err := a.Append(ctx, third); err != nil {
		t.Fatal(err)
	}
	got := app.Pending(ctx)
	if len(got) != 3 || got[1].Event.ID != own || got[0].Event.ID.IsZero() || got[2].Event.Type != "access.changed" {
		t.Fatalf("pending = %+v", got)
	}
	for _, p := range got {
		if p.CorrelationID != corr.id || p.TraceContext != corr.trace {
			t.Errorf("correlation of %s = %v %q", p.Event.Type, p.CorrelationID, p.TraceContext)
		}
	}
	other, _ := unitCtx(txn.PhaseWork)
	if len(app.Pending(other)) != 0 {
		t.Fatal("a buffer leaked into another unit")
	}
}

func TestNewAppender_RejectsBadConfig(t *testing.T) {
	t.Parallel()
	if _, err := app.NewAppender(app.AppenderConfig{PayloadMaxBytes: 0}, fixedCorrelation{}); err == nil {
		t.Error("a payload limit of 0 was accepted")
	}
	if _, err := app.NewAppender(app.AppenderConfig{PayloadMaxBytes: 10}, nil); err == nil {
		t.Error("a nil correlator was accepted")
	}
}

// fakeCounters records the calls of the hook.
type fakeCounters struct {
	value, pv int64
	calls     []string
	err       error
}

func (c *fakeCounters) Advance(_ context.Context, name instance.CounterName, n int64) (int64, error) {
	c.calls = append(c.calls, "advance "+string(name))
	if c.err != nil {
		return 0, c.err
	}
	c.value += n
	return c.value, nil
}

func (c *fakeCounters) Current(_ context.Context, name instance.CounterName) (int64, error) {
	c.calls = append(c.calls, "current "+string(name))
	return c.pv, nil
}

// fakeWriter records the writes.
type fakeWriter struct {
	calls []string
	first int64
	pv    int64
	rows  []app.PendingEvent
}

func (w *fakeWriter) Write(_ context.Context, first, pv int64, rows []app.PendingEvent) error {
	w.calls = append(w.calls, "write")
	w.first, w.pv, w.rows = first, pv, rows
	return nil
}

func TestPhaseHook_NoEventsTouchesNothing(t *testing.T) {
	t.Parallel()
	c, w := &fakeCounters{}, &fakeWriter{}
	ctx, _ := unitCtx(txn.PhaseSync)
	if err := app.NewPhaseHook(c, w).Run(ctx); err != nil {
		t.Fatal(err)
	}
	if len(c.calls)+len(w.calls) != 0 {
		t.Fatalf("calls without events: %v %v", c.calls, w.calls)
	}
}

func TestPhaseHook_NumbersAfterTheLockAndNotifiesLast(t *testing.T) {
	t.Parallel()
	c, w := &fakeCounters{value: 10, pv: 7}, &fakeWriter{}
	a := newAppender(t, fixedCorrelation{})
	ctx, set := unitCtx(txn.PhaseWork)
	if err := a.Append(ctx, event(), event(), event()); err != nil {
		t.Fatal(err)
	}
	set(txn.PhaseSync)
	if err := app.NewPhaseHook(c, w).Run(ctx); err != nil {
		t.Fatal(err)
	}
	if want := []string{"advance event_seq", "current permission_version"}; !slices.Equal(c.calls, want) {
		t.Errorf("counter calls = %v, want %v (permission_version after the lock)", c.calls, want)
	}
	if !slices.Equal(w.calls, []string{"write"}) || w.first != 11 || w.pv != 7 || len(w.rows) != 3 {
		t.Errorf("writer = %+v", w)
	}
}

func TestPhaseHook_CounterErrorStopsTheWrite(t *testing.T) {
	t.Parallel()
	boom := errors.New("lock timeout")
	c, w := &fakeCounters{err: boom}, &fakeWriter{}
	a := newAppender(t, fixedCorrelation{})
	ctx, set := unitCtx(txn.PhaseWork)
	if err := a.Append(ctx, event()); err != nil {
		t.Fatal(err)
	}
	set(txn.PhaseSync)
	if err := app.NewPhaseHook(c, w).Run(ctx); !errors.Is(err, boom) {
		t.Fatalf("Run = %v, want the counter error (so the unit of work can retry)", err)
	}
	if len(w.calls) != 0 {
		t.Fatalf("writer called after a failed counter step: %v", w.calls)
	}
}

// fakeUoW runs fn directly in a unit in the work phase.
type fakeUoW struct{ runs int }

func (u *fakeUoW) Do(ctx context.Context, fn func(context.Context) error, _ ...txn.Option) error {
	u.runs++
	c, _ := unitCtx(txn.PhaseWork)
	_ = ctx
	return fn(c)
}

// fakePartitions records the calls of the maintenance use case.
type fakePartitions struct {
	head          int64
	dropErr       error
	calls         []string
	size          int64
	ahead         int
	cutoff        time.Time
	lockTimeout   time.Duration
	ensureHead    int64
	dropHead      int64
	created, drop int
}

func (p *fakePartitions) Head(context.Context) (int64, error) {
	p.calls = append(p.calls, "head")
	return p.head, nil
}

func (p *fakePartitions) Ensure(_ context.Context, size int64, ahead int, head int64) (int, error) {
	p.calls = append(p.calls, "ensure")
	p.size, p.ahead, p.ensureHead = size, ahead, head
	return p.created, nil
}

func (p *fakePartitions) DropExpired(_ context.Context, cutoff time.Time, head int64, lockTimeout time.Duration) (int, error) {
	p.calls = append(p.calls, "drop")
	p.cutoff, p.dropHead, p.lockTimeout = cutoff, head, lockTimeout
	return p.drop, p.dropErr
}

func TestPartitionMaintenance_UsesConfigAndClock(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 10, 12, 0, 0, 0, 0, time.UTC)
	cfg := app.PartitionConfig{Size: 100, Ahead: 3, Retention: 30 * 24 * time.Hour, LockTimeout: 2 * time.Second}
	parts := &fakePartitions{head: 250, created: 2, drop: 1}
	uow := &fakeUoW{}
	m, err := app.NewPartitionMaintenance(cfg, uow, parts, clock.NewFake(now))
	if err != nil {
		t.Fatal(err)
	}
	res, err := m.Run(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if res != (app.MaintenanceResult{Head: 250, Created: 2, Dropped: 1}) || uow.runs != 2 {
		t.Errorf("result = %+v, runs %d", res, uow.runs)
	}
	if !slices.Equal(parts.calls, []string{"head", "ensure", "drop"}) || parts.size != 100 || parts.ahead != 3 ||
		parts.ensureHead != 250 || parts.dropHead != 250 || !parts.cutoff.Equal(now.Add(-cfg.Retention)) ||
		parts.lockTimeout != 2*time.Second {
		t.Errorf("partitions = %+v", parts)
	}
}

func TestNewPartitionMaintenance_RejectsBadConfig(t *testing.T) {
	t.Parallel()
	good := app.PartitionConfig{Size: 1, Ahead: 0, Retention: time.Hour, LockTimeout: time.Second}
	for _, bad := range []app.PartitionConfig{
		{Size: 0, Ahead: 0, Retention: time.Hour, LockTimeout: time.Second},
		{Size: 1, Ahead: -1, Retention: time.Hour, LockTimeout: time.Second},
		{Size: 1, Ahead: 0, Retention: 0, LockTimeout: time.Second},
		{Size: 1, Ahead: 0, Retention: time.Hour, LockTimeout: 0},
	} {
		if _, err := app.NewPartitionMaintenance(bad, &fakeUoW{}, &fakePartitions{}, clock.System()); err == nil {
			t.Errorf("config %+v was accepted", bad)
		}
	}
	if _, err := app.NewPartitionMaintenance(good, &fakeUoW{}, &fakePartitions{}, clock.System()); err != nil {
		t.Errorf("config %+v: %v", good, err)
	}
}

// TestPartitionMaintenance_DropFailureKeepsTheCreatedPartitions: creating partitions and dropping
// expired ones run in two units of work, so a DETACH that cannot get its lock (a long reader)
// never rolls back the partitions ahead; the error says the drop is deferred.
func TestPartitionMaintenance_DropFailureKeepsTheCreatedPartitions(t *testing.T) {
	t.Parallel()
	cfg := app.PartitionConfig{Size: 10, Ahead: 3, Retention: time.Hour, LockTimeout: time.Second}
	lockTimeout := errs.Wrap(errs.CodeUnavailable, errors.New("lock timeout"), "retries spent")
	parts := &fakePartitions{head: 5, created: 3, dropErr: lockTimeout}
	uow := &fakeUoW{}
	m, err := app.NewPartitionMaintenance(cfg, uow, parts, clock.System())
	if err != nil {
		t.Fatal(err)
	}
	res, err := m.Run(context.Background())
	if !errors.Is(err, app.ErrDropDeferred) || !errors.Is(err, lockTimeout) {
		t.Fatalf("Run = %v, want ErrDropDeferred wrapping the cause", err)
	}
	if res.Created != 3 || res.Head != 5 || uow.runs != 2 {
		t.Fatalf("result = %+v after %d units, want the created partitions reported from their own unit", res, uow.runs)
	}
}

// TestPartitionMaintenance_OtherDropErrorsAreNotDeferred: only a lock timeout defers the drop; any
// other error of the drop unit is returned as it is, so bothy migrate up and the job report it.
func TestPartitionMaintenance_OtherDropErrorsAreNotDeferred(t *testing.T) {
	t.Parallel()
	cfg := app.PartitionConfig{Size: 10, Ahead: 3, Retention: time.Hour, LockTimeout: time.Second}
	boom := errors.New("permission denied")
	m, err := app.NewPartitionMaintenance(cfg, &fakeUoW{}, &fakePartitions{head: 5, created: 1, dropErr: boom}, clock.System())
	if err != nil {
		t.Fatal(err)
	}
	res, err := m.Run(context.Background())
	if !errors.Is(err, boom) || errors.Is(err, app.ErrDropDeferred) || res.Created != 1 {
		t.Fatalf("Run = %+v, %v; want the error itself, not deferred", res, err)
	}
}
