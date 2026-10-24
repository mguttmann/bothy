// SPDX-License-Identifier: Apache-2.0

package postgres_test

import (
	"context"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/agents"
	"github.com/OWNER/bothy/services/server/internal/agents/app"
	"github.com/OWNER/bothy/services/server/internal/agents/domain"
	"github.com/OWNER/bothy/services/server/internal/chat"
	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// operatorThreads answers the routing data of every thread as an imported thread of kind thread
// with the operator op; CreateEarlier reads nothing else of chat.
type operatorThreads struct{ op ids.ID }

// Info implements app.ThreadInfos.
func (o operatorThreads) Info(_ context.Context, threadID ids.ID) (chat.ThreadInfo, error) {
	return chat.ThreadInfo{ID: threadID, Kind: "thread", OperatorAgentID: o.op}, nil
}

// TestCreateEarlier runs ImportedSessions.CreateEarlier over the postgres store (M8-06 design note
// 2): the first call writes the earlier row of requirement 11 (taken_over), a repeated call and a
// second file with the same reference write nothing (already_present), another reference of an
// untouched thread is taken over too, and a thread with a reset row or a current row gets nothing
// (dropped_late). Validation and the unit of work are refused before any read.
func TestCreateEarlier(t *testing.T) {
	t.Parallel()
	e := newSessionEnv(t)
	ctx := context.Background()
	clk := clock.NewFake(e.baseTime)
	svc, err := app.NewSessions(app.SessionConfig{SessionRefMaxBytes: 64}, app.SessionDeps{
		Store: e.store, Threads: operatorThreads{op: e.lotse}, Channels: unusedChannels{}, UoW: e.uow, Authorizer: unusedAuthorizer{},
		Audit: unusedAudit{}, Events: unusedEvents{}, Clock: clk,
	})
	if err != nil {
		t.Fatal(err)
	}
	create := func(in agents.EarlierSession) (string, error) {
		t.Helper()
		var out string
		err := e.uow.Do(ctx, func(ctx context.Context) error {
			var err error
			out, err = svc.CreateEarlier(ctx, in)
			return err
		})
		return out, err
	}
	rows := func(thread ids.ID) []agents.Session {
		t.Helper()
		got, err := e.store.ThreadSessions(ctx, thread)
		if err != nil {
			t.Fatal(err)
		}
		return got
	}

	in := agents.EarlierSession{ThreadID: e.threadA, AgentID: e.lotse, ExternalSessionRef: "sess-import-a", CostBaselineMicros: 1500, Currency: "EUR"}
	if got, err := create(in); err != nil || got != agents.EarlierTakenOver {
		t.Fatalf("first CreateEarlier = %q, %v", got, err)
	}
	stored := rows(e.threadA)
	if len(stored) != 1 {
		t.Fatalf("rows after the first call = %d, want 1", len(stored))
	}
	row := stored[0]
	want := agents.Session{ID: row.ID, ThreadID: e.threadA, AgentID: e.lotse, ExternalSessionRef: "sess-import-a", Started: true,
		CostBaselineMicros: 1500, Currency: "EUR", CostIsEstimate: true, CreatedAt: e.baseTime, EndedAt: e.baseTime, EndReason: agents.EndImport,
		Version: 1}
	if row != want {
		t.Fatalf("earlier row %+v, want %+v", row, want)
	}

	clk.Advance(time.Second)
	for _, again := range []agents.EarlierSession{in, {ThreadID: e.threadA, AgentID: e.lotse, ExternalSessionRef: "sess-import-a"}} {
		if got, err := create(again); err != nil || got != agents.EarlierAlreadyPresent {
			t.Fatalf("repeated CreateEarlier = %q, %v", got, err)
		}
	}
	if n := len(rows(e.threadA)); n != 1 {
		t.Fatalf("rows after the repeats = %d, want 1", n)
	}
	other := agents.EarlierSession{ThreadID: e.threadA, AgentID: e.lotse, ExternalSessionRef: "sess-import-other"}
	if got, err := create(other); err != nil || got != agents.EarlierTakenOver {
		t.Fatalf("another reference = %q, %v", got, err)
	}
	if got := rows(e.threadA)[0]; got.Currency != "USD" || got.CostBaselineMicros != 0 {
		t.Fatalf("a line without a baseline stored %q %d, want the column default USD and 0", got.Currency, got.CostBaselineMicros)
	}

	// A reset row: the thread had a current session since the import.
	reset, _ := e.insert(t, app.NewSession{ID: ids.New(), ThreadID: e.threadB, AgentID: e.lotse, ExternalSessionRef: "sess-live-b",
		Started: true, CreatedAt: e.baseTime, EndedAt: e.baseTime, EndReason: domain.EndReset})
	// A current row.
	e.insert(t, app.NewSession{ID: ids.New(), ThreadID: e.threadC, AgentID: e.lotse, ExternalSessionRef: "sess-live-c", IsCurrent: true,
		CreatedAt: e.baseTime})
	for _, thread := range []ids.ID{e.threadB, e.threadC} {
		before := rows(thread)
		got, err := create(agents.EarlierSession{ThreadID: thread, AgentID: e.lotse, ExternalSessionRef: "sess-import-late"})
		if err != nil || got != agents.EarlierDroppedLate {
			t.Fatalf("CreateEarlier after a session = %q, %v", got, err)
		}
		if after := rows(thread); len(after) != len(before) {
			t.Fatalf("a late line wrote a row: %d rows, were %d", len(after), len(before))
		}
	}
	if rows(e.threadB)[0] != reset {
		t.Fatal("the reset row changed")
	}

	for _, c := range []struct {
		name    string
		in      agents.EarlierSession
		pointer string
	}{
		{"not the operator", agents.EarlierSession{ThreadID: e.threadC, AgentID: ids.New(), ExternalSessionRef: "x"}, domain.PointerAgentID},
		{"empty reference", agents.EarlierSession{ThreadID: e.threadC, AgentID: e.lotse}, domain.PointerExternalSessionRef},
		{"negative baseline", agents.EarlierSession{ThreadID: e.threadC, AgentID: e.lotse, ExternalSessionRef: "x", CostBaselineMicros: -1}, domain.PointerCostBaselineMicros},
		{"baseline without currency", agents.EarlierSession{ThreadID: e.threadC, AgentID: e.lotse, ExternalSessionRef: "x", CostBaselineMicros: 1}, domain.PointerCurrency},
		{"bad currency", agents.EarlierSession{ThreadID: e.threadC, AgentID: e.lotse, ExternalSessionRef: "x", Currency: "usd"}, domain.PointerCurrency},
	} {
		_, err := create(c.in)
		ee, ok := errs.As(err)
		if !ok || ee.Code != errs.CodeValidationFailed || len(ee.Fields) != 1 || ee.Fields[0].Pointer != c.pointer {
			t.Errorf("%s: err = %v, want validation_failed at %s", c.name, err, c.pointer)
		}
	}
	if _, err := svc.CreateEarlier(ctx, in); err == nil {
		t.Fatal("CreateEarlier outside a unit of work")
	}
}
