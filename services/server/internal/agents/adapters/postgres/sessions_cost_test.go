// SPDX-License-Identifier: Apache-2.0

package postgres_test

import (
	"context"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/agents"
	"github.com/OWNER/bothy/services/server/internal/agents/app"
	"github.com/OWNER/bothy/services/server/internal/chat"
	"github.com/OWNER/bothy/services/server/internal/kernel/auditing"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/eventlog"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// The collaborators AddCost must never call: each embeds its nil interface, so a call panics and
// fails the test (no authorization, no event, no audit entry; M2-22 design note 5 g).
type (
	unusedThreads    struct{ app.ThreadInfos }
	unusedChannels   struct{ chat.TriggerFacts }
	unusedAuthorizer struct{ authorize.Authorizer }
	unusedAudit      struct{ auditing.Recorder }
	unusedEvents     struct{ eventlog.Appender }
)

// TestSessionStore_AddCost runs Sessions.AddCost over the postgres store (M2-22 design note 5 g):
// two adds raise cumulative_cost_micros and version by one each, another currency is not added and
// leaves the version, a fresh row with cumulative and baseline 0 takes the currency of the first
// add; SetSessionCost is a compare-and-swap over the version and needs a unit of work.
func TestSessionStore_AddCost(t *testing.T) {
	t.Parallel()
	e := newSessionEnv(t)
	ctx := context.Background()
	svc, err := app.NewSessions(app.SessionConfig{SessionRefMaxBytes: 64}, app.SessionDeps{
		Store: e.store, Threads: unusedThreads{}, Channels: unusedChannels{}, UoW: e.uow, Authorizer: unusedAuthorizer{},
		Audit: unusedAudit{}, Events: unusedEvents{},
	})
	if err != nil {
		t.Fatal(err)
	}
	add := func(cmd agents.AddSessionCost) {
		t.Helper()
		if err := e.uow.Do(ctx, func(ctx context.Context) error { return svc.AddCost(ctx, cmd) }); err != nil {
			t.Fatalf("AddCost(%+v): %v", cmd, err)
		}
	}
	read := func(thread ids.ID) agents.Session {
		t.Helper()
		row, found, err := e.store.CurrentSession(ctx, thread)
		if err != nil || !found {
			t.Fatalf("CurrentSession = %v, %v", found, err)
		}
		return row
	}

	fresh, _ := e.insert(t, app.NewSession{ID: ids.New(), ThreadID: e.threadA, AgentID: e.lotse, ExternalSessionRef: "sess-a",
		IsCurrent: true, CreatedAt: e.baseTime})
	add(agents.AddSessionCost{SessionID: fresh.ID, AmountMicros: 1500, Currency: "EUR"})
	add(agents.AddSessionCost{SessionID: fresh.ID, AmountMicros: 2500, Currency: "EUR"})
	got := read(e.threadA)
	want := fresh
	want.CumulativeCostMicros, want.Currency, want.Version = 4000, "EUR", fresh.Version+2
	if got != want {
		t.Fatalf("after two adds %+v, want %+v", got, want)
	}
	add(agents.AddSessionCost{SessionID: fresh.ID, AmountMicros: 900, Currency: "USD"})
	if after := read(e.threadA); after != got {
		t.Fatalf("another currency changed the row: %+v, was %+v", after, got)
	}

	based, _ := e.insert(t, app.NewSession{ID: ids.New(), ThreadID: e.threadB, AgentID: e.lotse, ExternalSessionRef: "sess-b",
		IsCurrent: true, CostBaselineMicros: 700, Currency: "EUR", CreatedAt: e.baseTime})
	add(agents.AddSessionCost{SessionID: based.ID, AmountMicros: 900, Currency: "USD"})
	if after := read(e.threadB); after != based {
		t.Fatalf("a row with a baseline took another currency: %+v, was %+v", after, based)
	}
	err = e.uow.Do(ctx, func(ctx context.Context) error {
		return svc.AddCost(ctx, agents.AddSessionCost{SessionID: ids.New(), AmountMicros: 1, Currency: "EUR"})
	})
	if err == nil {
		t.Fatal("AddCost for an unknown session")
	}

	if _, _, err := e.store.SetSessionCost(ctx, app.SessionCost{ID: based.ID, Version: based.Version, Currency: "EUR"}); err == nil {
		t.Fatal("SetSessionCost outside a unit of work")
	}
	if err := e.uow.Do(ctx, func(ctx context.Context) error {
		row, changed, err := e.store.SetSessionCost(ctx, app.SessionCost{ID: based.ID, Version: based.Version + 1, CumulativeCostMicros: 5, Currency: "EUR"})
		if err != nil || changed {
			t.Errorf("SetSessionCost at a stale version = %+v, %v, %v", row, changed, err)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}
