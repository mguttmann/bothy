// SPDX-License-Identifier: Apache-2.0

package postgres_test

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/kernel/eventlog"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/sync"
	"github.com/OWNER/bothy/services/server/internal/sync/adapters/postgres"
)

// redactEnv appends the events of a deleted message msg (two message events), of another message
// and of a thread with the same id as msg, and returns the redactor over the pool of e.
func redactEnv(t *testing.T, e *env, msg ids.ID) *postgres.Redactor {
	t.Helper()
	other := ids.New()
	evs := []eventlog.Event{
		{Type: "message.created", SchemaVersion: 1, ActorKind: eventlog.ActorSystem, ResourceKind: "message", ResourceID: msg,
			ResourceVersion: 1, ReceiveAction: "message.read", Payload: []byte(`{"content_md":"secret one"}`), Truncated: true},
		{Type: "message.created", SchemaVersion: 1, ActorKind: eventlog.ActorSystem, ResourceKind: "message", ResourceID: other,
			ResourceVersion: 1, ReceiveAction: "message.read", Payload: []byte(`{"content_md":"other"}`)},
		{Type: "thread.updated", SchemaVersion: 1, ActorKind: eventlog.ActorSystem, ResourceKind: "thread", ResourceID: msg,
			ResourceVersion: 2, ReceiveAction: "thread.read", Payload: []byte(`{"title":"kept"}`)},
		{Type: "message.updated", SchemaVersion: 1, ActorKind: eventlog.ActorSystem, ResourceKind: "message", ResourceID: msg,
			ResourceVersion: 2, ReceiveAction: "message.read", Payload: []byte(`{"content_md":"secret two"}`), ClientMsgID: ids.New()},
	}
	if _, err := e.appendUnit(context.Background(), evs...); err != nil {
		t.Fatal(err)
	}
	return postgres.NewRedactor(e.pool)
}

// redact runs Redact for message msg with fn in one unit of work.
func redact(e *env, r *postgres.Redactor, msg ids.ID, fn eventlog.RedactFunc) (int, error) {
	var n int
	err := e.uow.Do(context.Background(), func(ctx context.Context) error {
		var err error
		n, err = r.Redact(ctx, "message", msg, fn)
		return err
	})
	return n, err
}

// constant returns a RedactFunc that records its calls and answers payload.
func constant(calls *[]string, payload string) eventlog.RedactFunc {
	return func(typ string, v int, p json.RawMessage) (json.RawMessage, error) {
		var c any
		if err := json.Unmarshal(p, &c); err != nil {
			return nil, err
		}
		b, _ := json.Marshal(c)
		*calls = append(*calls, typ+"/"+string(b))
		if v != 1 {
			return nil, errors.New("schema version")
		}
		return json.RawMessage(payload), nil
	}
}

// logRows returns every row of event_log as text, in seq order.
func logRows(t *testing.T, e *env) []map[string]string {
	t.Helper()
	rows, err := e.db.SuperConn(t).Query(context.Background(), `SELECT to_jsonb(l) FROM event_log l ORDER BY seq`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var out []map[string]string
	for rows.Next() {
		var raw map[string]any
		if err := rows.Scan(&raw); err != nil {
			t.Fatal(err)
		}
		m := map[string]string{}
		for k, v := range raw {
			b, _ := json.Marshal(v)
			m[k] = string(b)
		}
		out = append(out, m)
	}
	return out
}

// TestRedactor_OnlyMatchingRows: only the events of resource (message, id) are redacted, in
// ascending seq; another message and a thread with the same id keep their payloads.
func TestRedactor_OnlyMatchingRows(t *testing.T) {
	t.Parallel()
	e := newEnv(t, envOptions{})
	msg := ids.New()
	r := redactEnv(t, e, msg)
	var calls []string
	n, err := redact(e, r, msg, constant(&calls, `{"content_md":""}`))
	if err != nil || n != 2 {
		t.Fatalf("n %d, err %v", n, err)
	}
	if want := []string{`message.created/{"content_md":"secret one"}`, `message.updated/{"content_md":"secret two"}`}; !reflect.DeepEqual(calls, want) {
		t.Fatalf("calls %q", calls)
	}
	rows := logRows(t, e)
	for i, want := range []string{`{"content_md":""}`, `{"content_md":"other"}`, `{"title":"kept"}`, `{"content_md":""}`} {
		if p := rows[i]["payload"]; p != want {
			t.Errorf("row %d payload %s, want %s", i, p, want)
		}
		if redacted := rows[i]["redacted_at"] != "null"; redacted != (i == 0 || i == 3) {
			t.Errorf("row %d redacted_at %s", i, rows[i]["redacted_at"])
		}
	}
}

// TestRedactor_EnvelopeUnchanged: every column except payload and redacted_at keeps its value.
func TestRedactor_EnvelopeUnchanged(t *testing.T) {
	t.Parallel()
	e := newEnv(t, envOptions{})
	msg := ids.New()
	r := redactEnv(t, e, msg)
	before := logRows(t, e)
	var calls []string
	if _, err := redact(e, r, msg, constant(&calls, `{"content_md":""}`)); err != nil {
		t.Fatal(err)
	}
	after := logRows(t, e)
	for i := range before {
		for k, v := range before[i] {
			if k == "payload" || k == "redacted_at" {
				continue
			}
			if after[i][k] != v {
				t.Errorf("row %d column %s: %s, before %s", i, k, after[i][k], v)
			}
		}
	}
}

// TestRedactor_Idempotent: a second redaction writes the same payloads and keeps the first
// redacted_at.
func TestRedactor_Idempotent(t *testing.T) {
	t.Parallel()
	e := newEnv(t, envOptions{})
	msg := ids.New()
	r := redactEnv(t, e, msg)
	var calls []string
	if n, err := redact(e, r, msg, constant(&calls, `{"content_md":""}`)); err != nil || n != 2 {
		t.Fatalf("first: %d %v", n, err)
	}
	first := logRows(t, e)
	if n, err := redact(e, r, msg, constant(&calls, `{"content_md":""}`)); err != nil || n != 2 {
		t.Fatalf("second: %d %v", n, err)
	}
	if second := logRows(t, e); !reflect.DeepEqual(first, second) {
		t.Fatalf("second redaction changed rows:\n%v\n%v", first, second)
	}
}

// TestRedactor_ErrorRollsBack: an error of fn on the second row, or a result that is not a JSON
// object, fails the unit, and no row changes; outside a unit Redact refuses.
func TestRedactor_ErrorRollsBack(t *testing.T) {
	t.Parallel()
	e := newEnv(t, envOptions{})
	msg := ids.New()
	r := redactEnv(t, e, msg)
	before := logRows(t, e)
	boom := errors.New("boom")
	calls := 0
	_, err := redact(e, r, msg, func(string, int, json.RawMessage) (json.RawMessage, error) {
		calls++
		if calls == 2 {
			return nil, boom
		}
		return json.RawMessage(`{}`), nil
	})
	if !errors.Is(err, boom) {
		t.Fatalf("err %v", err)
	}
	for _, bad := range []string{`[]`, `null`, `{"a":`, ``} {
		if _, err := redact(e, r, msg, func(string, int, json.RawMessage) (json.RawMessage, error) {
			return json.RawMessage(bad), nil
		}); err == nil {
			t.Errorf("result %q accepted", bad)
		}
	}
	if after := logRows(t, e); !reflect.DeepEqual(before, after) {
		t.Fatal("a failed redaction changed rows")
	}
	if _, err := r.Redact(context.Background(), "message", msg, constant(new([]string), `{}`)); err == nil {
		t.Fatal("Redact outside a unit of work")
	}
}

// TestEnvelope_RedactedFlag: a redacted row reads back with Redacted, and its envelope carries
// "redacted": true; an unredacted one has no member.
func TestEnvelope_RedactedFlag(t *testing.T) {
	t.Parallel()
	e := newEnv(t, envOptions{})
	msg := ids.New()
	r := redactEnv(t, e, msg)
	var calls []string
	if _, err := redact(e, r, msg, constant(&calls, `{"content_md":""}`)); err != nil {
		t.Fatal(err)
	}
	evs, err := e.repo.EventsAfter(context.Background(), 0, 10)
	if err != nil || len(evs) != 4 {
		t.Fatalf("%d events, %v", len(evs), err)
	}
	for i, ev := range evs {
		b, err := json.Marshal(ev.Envelope(sync.DeliveryFields{}))
		if err != nil {
			t.Fatal(err)
		}
		var m map[string]any
		_ = json.Unmarshal(b, &m)
		want := i == 0 || i == 3
		if ev.Redacted != want || (m["redacted"] == true) != want {
			t.Errorf("event %d: Redacted %v, envelope %s", i, ev.Redacted, b)
		}
		if i == 0 && m["truncated"] != true {
			t.Errorf("event 0 lost truncated: %s", b)
		}
	}
}
