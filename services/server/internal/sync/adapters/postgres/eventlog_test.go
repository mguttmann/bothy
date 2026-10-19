// SPDX-License-Identifier: Apache-2.0

package postgres_test

import (
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"slices"
	"strings"
	stdsync "sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/eventlog"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/pg"
	"github.com/OWNER/bothy/services/server/internal/platform/testkit"
	"github.com/OWNER/bothy/services/server/internal/sync"
	"github.com/OWNER/bothy/services/server/internal/sync/adapters/postgres"
)

// listen opens a connection of bothy_app that listens on event_log.
func listen(t *testing.T, e *env) *pgx.Conn {
	t.Helper()
	conn, err := e.pool.DialDedicated(context.Background(), "test-listener")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close(context.Background()) })
	if _, err := conn.Exec(context.Background(), "LISTEN event_log"); err != nil {
		t.Fatal(err)
	}
	return conn
}

// notification waits up to d for one notification on conn; "" when none came.
func notification(t *testing.T, conn *pgx.Conn, d time.Duration) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), d)
	defer cancel()
	n, err := conn.WaitForNotification(ctx)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return ""
		}
		t.Fatal(err)
	}
	return n.Payload
}

// counterRow returns the value and the xmin of the event_seq row: an UPDATE, also one that only
// locks for a moment and rolls back, would leave its own xmin on a committed row only on commit,
// so the value and the xmin together show whether a committed unit touched the counter.
func counterRow(t *testing.T, e *env) (int64, string) {
	t.Helper()
	var v int64
	var xmin string
	if err := e.db.SuperConn(t).QueryRow(context.Background(),
		"SELECT value, xmin::text FROM sequence_counters WHERE name = 'event_seq'").Scan(&v, &xmin); err != nil {
		t.Fatal(err)
	}
	return v, xmin
}

func TestSyncHook_NoEventsTakesNoCounterAndNoNotify(t *testing.T) {
	t.Parallel()
	e := newEnv(t, envOptions{})
	conn := listen(t, e)
	value, xmin := counterRow(t, e)
	err := e.uow.Do(context.Background(), func(ctx context.Context) error {
		_, err := pg.Querier(ctx, e.pool).Exec(ctx, "SELECT 1")
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	if v, x := counterRow(t, e); v != value || x != xmin {
		t.Errorf("event_seq changed from %d/%s to %d/%s by a unit without events", value, xmin, v, x)
	}
	if got := notification(t, conn, 300*time.Millisecond); got != "" {
		t.Errorf("notification %q without events", got)
	}
}

func TestSyncHook_ContiguousSeqInAppendOrder(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	e := newEnv(t, envOptions{})
	conn := listen(t, e)
	super := e.db.SuperConn(t)
	if _, err := super.Exec(ctx, "UPDATE sequence_counters SET value = 5 WHERE name = 'permission_version'"); err != nil {
		t.Fatal(err)
	}
	if _, err := e.appendUnit(ctx, testEvent("thread.first")); err != nil {
		t.Fatal(err)
	}
	if got := notification(t, conn, testkit.Patience); got != "1" {
		t.Errorf("notification after the first unit = %q, want 1", got)
	}
	if _, err := super.Exec(ctx, "UPDATE sequence_counters SET value = 6 WHERE name = 'permission_version'"); err != nil {
		t.Fatal(err)
	}
	if _, err := e.appendUnit(ctx, testEvent("thread.a"), testEvent("thread.b"), testEvent("thread.c")); err != nil {
		t.Fatal(err)
	}
	if got := notification(t, conn, testkit.Patience); got != "4" {
		t.Errorf("notification after the second unit = %q, want 4 (only the last seq)", got)
	}
	rows, err := super.Query(ctx, "SELECT seq, type, permission_version FROM event_log ORDER BY seq")
	if err != nil {
		t.Fatal(err)
	}
	type row struct {
		seq  int64
		typ  string
		perm int64
	}
	got, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (row, error) {
		var x row
		err := r.Scan(&x.seq, &x.typ, &x.perm)
		return x, err
	})
	if err != nil {
		t.Fatal(err)
	}
	want := []row{{1, "thread.first", 5}, {2, "thread.a", 6}, {3, "thread.b", 6}, {4, "thread.c", 6}}
	if !slices.Equal(got, want) {
		t.Fatalf("rows = %v, want %v", got, want)
	}
}

func TestSyncHook_StoresEveryField(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	e := newEnv(t, envOptions{})
	ev := eventlog.Event{
		ID: ids.New(), Type: "message.created", SchemaVersion: 2, ActorKind: eventlog.ActorUser,
		ActorPrincipalID: ids.New(), SpaceID: ids.New(), ChannelID: ids.New(), ThreadID: ids.New(),
		ResourceKind: "message", ResourceID: ids.New(), ResourceVersion: 9, AudiencePrincipalID: ids.New(),
		ReceiveAction: "self", Payload: []byte(`{"content_md":"hi"}`), Truncated: true, ClientMsgID: ids.New(),
	}
	if _, err := e.appendUnit(ctx, ev, testEvent("thread.closed")); err != nil {
		t.Fatal(err)
	}
	got, err := e.repo.EventsAfter(ctx, 0, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("read %d events", len(got))
	}
	g := got[0]
	if g.Seq != 1 || g.ID != ev.ID || g.Type != ev.Type || g.SchemaVersion != 2 || g.ActorKind != "user" ||
		g.ActorPrincipalID != ev.ActorPrincipalID || g.SpaceID != ev.SpaceID || g.ChannelID != ev.ChannelID ||
		g.ThreadID != ev.ThreadID || g.ResourceKind != "message" || g.ResourceID != ev.ResourceID ||
		g.ResourceVersion != 9 || g.AudiencePrincipalID != ev.AudiencePrincipalID || g.ReceiveAction != "self" ||
		string(g.Payload) != `{"content_md":"hi"}` || !g.Truncated || g.Redacted || g.ClientMsgID != ev.ClientMsgID ||
		g.OccurredAt.IsZero() || g.OccurredAt.Location().String() != "UTC" {
		t.Errorf("stored event = %+v", g)
	}
	n := got[1]
	if n.ActorKind != "system" || !n.ActorPrincipalID.IsZero() || !n.SpaceID.IsZero() || n.ResourceVersion != 0 ||
		!n.AudiencePrincipalID.IsZero() || !n.CorrelationID.IsZero() || n.TraceContext != "" || n.Truncated {
		t.Errorf("absent columns of the second event = %+v", n)
	}
	if !got[0].OccurredAt.Equal(got[1].OccurredAt) {
		t.Error("events of one unit carry different occurred_at; it is the transaction time")
	}
}

// auditChaos is a hook of the phase after sync that sleeps a little and rolls every fifth unit
// back after the counter step, so the counter is rolled back with the unit.
type auditChaos struct {
	mu   stdsync.Mutex
	rand *rand.Rand
}

func (a *auditChaos) hook(context.Context) error {
	a.mu.Lock()
	sleep := time.Duration(a.rand.IntN(3000)) * time.Microsecond
	fail := a.rand.IntN(5) == 0
	a.mu.Unlock()
	time.Sleep(sleep)
	if fail {
		return errors.New("audit chaos: roll back after the counter step")
	}
	return nil
}

// TestEventLog_ConcurrentWritersGapless is the confirmation of ADR 0009: many writers with 1 to 3
// events per unit, random delays and rollbacks after the counter step, and a tail reading at the
// same time; the tail sees exactly 1..committed, every committed event once, none of a rolled
// back unit, and the events of one unit contiguous.
func TestEventLog_ConcurrentWritersGapless(t *testing.T) {
	t.Parallel()
	chaos := &auditChaos{rand: rand.New(rand.NewPCG(1, 2))} // #nosec G404 -- test randomness
	e := newEnv(t, envOptions{audit: chaos.hook, maxConns: 12})
	col := &collector{}
	tail := postgres.NewTail(e.pool, e.repo, tailConfig(), []sync.Sink{col.sink}, nil)
	_, _ = runTail(t, tail)
	started(t, tail)

	const writers, units = 10, 30
	var mu stdsync.Mutex
	var committed [][]ids.ID
	var rolledBack []ids.ID
	var wg stdsync.WaitGroup
	for w := range writers {
		wg.Go(func() {
			r := rand.New(rand.NewPCG(uint64(w), 7)) // #nosec G404 G115 -- test randomness; w is small and not negative
			for range units {
				n := 1 + r.IntN(3)
				events := make([]eventlog.Event, n)
				for i := range events {
					events[i] = testEvent(fmt.Sprintf("thread.w_%c", 'a'+rune(w))) // #nosec G115 -- w < writers
				}
				got, err := e.appendUnit(context.Background(), events...)
				mu.Lock()
				switch {
				case err == nil:
					committed = append(committed, got)
				case strings.Contains(err.Error(), "audit chaos"):
					rolledBack = append(rolledBack, got...)
				default:
					t.Errorf("unit failed: %v", err)
				}
				mu.Unlock()
			}
		})
	}
	wg.Wait()
	var total int64
	for _, u := range committed {
		total += int64(len(u))
	}
	if len(rolledBack) == 0 {
		t.Fatal("no unit rolled back; the test does not exercise rollbacks")
	}
	testkit.Until(t, "the tail to reach the head", func() bool { return tail.Head() == total })
	events := col.snapshot()
	checkContiguous(t, events, total)
	seqOf := map[ids.ID]int64{}
	for _, ev := range events {
		if _, dup := seqOf[ev.ID]; dup {
			t.Fatalf("event %s delivered twice", ev.ID)
		}
		seqOf[ev.ID] = ev.Seq
	}
	for _, id := range rolledBack {
		if _, ok := seqOf[id]; ok {
			t.Fatalf("event %s of a rolled back unit was delivered", id)
		}
	}
	for _, u := range committed {
		first, ok := seqOf[u[0]]
		if !ok {
			t.Fatalf("committed event %s missing", u[0])
		}
		for i, id := range u {
			if seqOf[id] != first+int64(i) {
				t.Fatalf("the events of one unit are not contiguous: %v", u)
			}
		}
	}
	var counter int64
	if err := e.db.SuperConn(t).QueryRow(context.Background(), "SELECT value FROM sequence_counters WHERE name = 'event_seq'").Scan(&counter); err != nil {
		t.Fatal(err)
	}
	if counter != total {
		t.Fatalf("event_seq = %d, committed %d; a rollback did not return its numbers", counter, total)
	}
}

func TestTail_ListenConnectionLossLosesNothing(t *testing.T) {
	t.Parallel()
	e := newEnv(t, envOptions{maxConns: 8})
	col := &collector{}
	tail := postgres.NewTail(e.pool, e.repo, tailConfig(), []sync.Sink{col.sink}, nil)
	_, _ = runTail(t, tail)
	super := e.db.SuperConn(t)
	listeners := func() int {
		var n int
		if err := super.QueryRow(context.Background(), `SELECT count(*) FROM pg_stat_activity
			WHERE application_name = 'bothy-event-tail' AND datname = current_database()`).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}
	testkit.Until(t, "the LISTEN connection", func() bool { return listeners() == 1 })

	ctx, stop := context.WithCancel(context.Background())
	var wg stdsync.WaitGroup
	var mu stdsync.Mutex
	var total int64
	for range 3 {
		wg.Go(func() {
			for ctx.Err() == nil {
				if _, err := e.appendUnit(context.Background(), testEvent("thread.x"), testEvent("thread.y")); err != nil {
					t.Errorf("append: %v", err)
					return
				}
				mu.Lock()
				total += 2
				mu.Unlock()
				time.Sleep(5 * time.Millisecond)
			}
		})
	}
	for range 3 {
		time.Sleep(150 * time.Millisecond)
		if _, err := super.Exec(context.Background(), `SELECT pg_terminate_backend(pid) FROM pg_stat_activity
			WHERE application_name = 'bothy-event-tail' AND datname = current_database()`); err != nil {
			t.Fatal(err)
		}
		testkit.Until(t, "the tail to reconnect", func() bool { return listeners() == 1 })
	}
	stop()
	wg.Wait()
	testkit.Until(t, "the tail to reach the head", func() bool { return tail.Head() == total })
	checkContiguous(t, col.snapshot(), total)
}

func TestTail_PollsWithoutNotify(t *testing.T) {
	t.Parallel()
	e := newEnv(t, envOptions{})
	col := &collector{}
	cfg := tailConfig()
	cfg.PollInterval = 200 * time.Millisecond
	tail := postgres.NewTail(e.pool, e.repo, cfg, []sync.Sink{col.sink}, nil)
	tail.SetDialForTest(func(context.Context) (*pgx.Conn, error) { return nil, errors.New("no LISTEN in this test") })
	_, _ = runTail(t, tail)
	started(t, tail)
	e.mustAppend(t, 2)
	// Without LISTEN only the poll delivers; the time is logged, not bounded, because a bound in
	// wall-clock time fails on loaded hosts (PL/8).
	start := time.Now()
	testkit.Until(t, "delivery by polling", func() bool { return len(col.snapshot()) == 2 })
	t.Logf("delivery after %s with a poll interval of %s", time.Since(start), cfg.PollInterval)
	checkContiguous(t, col.snapshot(), 2)
}

func TestTail_StartsAtHead(t *testing.T) {
	t.Parallel()
	e := newEnv(t, envOptions{})
	e.mustAppend(t, 3)
	col := &collector{}
	tail := postgres.NewTail(e.pool, e.repo, tailConfig(), []sync.Sink{col.sink}, nil)
	_, _ = runTail(t, tail)
	testkit.Until(t, "the tail to start at the head", func() bool { return tail.Head() == 3 })
	e.mustAppend(t, 1)
	testkit.Until(t, "the new event", func() bool { return tail.Head() == 4 })
	if got := col.snapshot(); len(got) != 1 || got[0].Seq != 4 {
		t.Fatalf("delivered %+v, want only seq 4", got)
	}
}

func TestTail_SinksInOrderAndSinkErrorIsFatal(t *testing.T) {
	t.Parallel()
	e := newEnv(t, envOptions{})
	var order []string
	var mu stdsync.Mutex
	first := func(_ context.Context, b []sync.Event) error {
		mu.Lock()
		defer mu.Unlock()
		order = append(order, fmt.Sprintf("first:%d", b[len(b)-1].Seq))
		return nil
	}
	boom := errors.New("sink failed")
	second := func(_ context.Context, b []sync.Event) error {
		mu.Lock()
		defer mu.Unlock()
		order = append(order, fmt.Sprintf("second:%d", b[len(b)-1].Seq))
		return boom
	}
	tail := postgres.NewTail(e.pool, e.repo, tailConfig(), []sync.Sink{first, second}, nil)
	done, _ := runTail(t, tail)
	started(t, tail)
	e.mustAppend(t, 1)
	select {
	case err := <-done:
		if !errors.Is(err, boom) {
			t.Fatalf("Run = %v, want the sink error", err)
		}
	case <-time.After(testkit.Patience):
		t.Fatal("Run did not end after a sink error")
	}
	mu.Lock()
	defer mu.Unlock()
	if !slices.Equal(order, []string{"first:1", "second:1"}) {
		t.Fatalf("sink order = %v", order)
	}
}

func TestTail_GapIsFatal(t *testing.T) {
	t.Parallel()
	e := newEnv(t, envOptions{})
	col := &collector{}
	tail := postgres.NewTail(e.pool, e.repo, tailConfig(), []sync.Sink{col.sink}, nil)
	done, _ := runTail(t, tail)
	started(t, tail)
	owner := e.db.Pool(t, pg.RoleOwner)
	if _, err := owner.Exec(context.Background(), `INSERT INTO event_log (seq, id, type, resource_kind, receive_action, payload, permission_version)
		VALUES (2, gen_random_uuid(), 'thread.jump', 'thread', 'thread.read', '{}', 0)`); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		if !errors.Is(err, sync.ErrGap) || !strings.Contains(err.Error(), "expected seq 1, read 2") {
			t.Fatalf("Run = %v, want ErrGap naming both numbers", err)
		}
	case <-time.After(testkit.Patience):
		t.Fatal("Run did not end on a gap")
	}
	if len(col.snapshot()) != 0 {
		t.Fatal("events after a gap were delivered")
	}
}

func TestTail_StopsOnContextEnd(t *testing.T) {
	t.Parallel()
	e := newEnv(t, envOptions{})
	tail := postgres.NewTail(e.pool, e.repo, tailConfig(), nil, nil)
	done, cancel := runTail(t, tail)
	started(t, tail)
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Run after the end of its context = %v, want nil", err)
		}
	case <-time.After(testkit.Patience):
		t.Fatal("Run did not end")
	}
}

func TestAppend_MissingPartitionIsUnavailable(t *testing.T) {
	t.Parallel()
	e := newEnv(t, envOptions{noPartitions: true})
	ev := testEvent("thread.closed")
	ev.Payload = []byte(`{"secret_marker":"x"}`)
	_, err := e.appendUnit(context.Background(), ev)
	if errs.KindOf(err) != errs.KindUnavailable {
		t.Fatalf("append without a partition = %v, want kind unavailable", err)
	}
	logs := e.logs.String()
	if strings.Count(logs, `"level":"ERROR"`) != 1 || !strings.Contains(logs, `"bothy.event.seq":1`) ||
		!strings.Contains(logs, `"error.type":"23514"`) || strings.Contains(logs, "secret_marker") {
		t.Fatalf("log = %s; want one error line with the first seq and no payload", logs)
	}
	if v, _ := counterRow(t, e); v != 0 {
		t.Fatalf("event_seq = %d after the rollback", v)
	}
}

func TestRepository_HeadAndMinimum(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	e := newEnv(t, envOptions{})
	if h, err := e.repo.Head(ctx); err != nil || h != 0 {
		t.Fatalf("Head of an empty log = %d, %v", h, err)
	}
	if m, err := e.repo.MinRetainedSeq(ctx); err != nil || m != 0 {
		t.Fatalf("MinRetainedSeq of an empty log = %d, %v", m, err)
	}
	e.mustAppend(t, 3)
	if h, err := e.repo.Head(ctx); err != nil || h != 3 {
		t.Fatalf("Head = %d, %v", h, err)
	}
	if m, err := e.repo.MinRetainedSeq(ctx); err != nil || m != 1 {
		t.Fatalf("MinRetainedSeq = %d, %v", m, err)
	}
	got, err := e.repo.EventsAfter(ctx, 1, 1)
	if err != nil || len(got) != 1 || got[0].Seq != 2 {
		t.Fatalf("EventsAfter(1, 1) = %+v, %v", got, err)
	}
}

// TestSyncHook_ReadPayloadIsNotLarger: jsonb normalises the stored payload (key order, spaces in
// its text form); the tail hands the sinks the compact form, which is never larger than the
// appended payload, so the size rule of 04 holds for what the envelope carries.
func TestSyncHook_ReadPayloadIsNotLarger(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	e := newEnv(t, envOptions{})
	payloads := []string{
		`{"b":[1,2,{"c":"x"}],"a":{"d":true,"e":-12.50}}`,
		`{"k":"\u00e9\/","n":12345678901234567890123}`,
		`{"z":1,"z":2}`,
	}
	for _, p := range payloads {
		ev := testEvent("thread.updated")
		ev.Payload = []byte(p)
		if _, err := e.appendUnit(ctx, ev); err != nil {
			t.Fatalf("append %s: %v", p, err)
		}
	}
	got, err := e.repo.EventsAfter(ctx, 0, 10)
	if err != nil {
		t.Fatal(err)
	}
	for i, ev := range got {
		if len(ev.Payload) > len(payloads[i]) {
			t.Errorf("payload %s is read as %s, larger than appended", payloads[i], ev.Payload)
		}
	}
}
