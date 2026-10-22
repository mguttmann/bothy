// SPDX-License-Identifier: Apache-2.0

package agentstest_test

import (
	"context"
	"os"
	"os/exec"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/agents/adapters/agentstest"
	"github.com/OWNER/bothy/services/server/internal/agents/adapters/postgres"
	"github.com/OWNER/bothy/services/server/internal/agents/app"
	"github.com/OWNER/bothy/services/server/internal/agents/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
	"github.com/OWNER/bothy/services/server/internal/platform/pg"
	"github.com/OWNER/bothy/services/server/internal/platform/pg/pgtest"
)

func TestMain(m *testing.M) { os.Exit(pgtest.Main(m)) }

// TestAgentstest_NotInBinary: the server binary never depends on the test helpers.
func TestAgentstest_NotInBinary(t *testing.T) {
	t.Parallel()
	out, err := exec.CommandContext(t.Context(), "go", "list", "-deps", "../../../../cmd/bothy").CombinedOutput()
	if err != nil {
		t.Fatalf("go list: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "/internal/agents/adapters/postgres") {
		t.Fatalf("go list output does not look like the dependencies of cmd/bothy:\n%.400s", out)
	}
	if strings.Contains(string(out), "/internal/agents/adapters/agentstest") {
		t.Fatal("cmd/bothy depends on internal/agents/adapters/agentstest")
	}
}

// TestPersistHeartbeat stores heartbeats with the helper and reads them back through the agents
// repository and app.HeartbeatReader, inside a unit of work and on the pool: time, run list, an
// empty reported list, no list, no heartbeat at all, an unknown agent.
func TestPersistHeartbeat(t *testing.T) {
	t.Parallel()
	db := pgtest.New(t)
	pool := db.Pool(t, pg.RoleApp)
	owner := db.Pool(t, pg.RoleOwner)
	reg := txn.NewRegistry()
	reg.Seal()
	uow := pg.NewUnitOfWork(pool, pg.UnitConfig{StatementTimeout: 30 * time.Second, LockTimeout: 10 * time.Second, IdleInTxTimeout: 60 * time.Second}, reg)
	repo := postgres.NewRepository(pool)
	ctx := context.Background()

	agent := func(handle string) ids.ID {
		id := ids.New()
		if _, err := owner.Exec(ctx, `INSERT INTO principals (id, kind, handle, display_name) VALUES ($1, 'agent', $2, $2)`, id.String(), handle); err != nil {
			t.Fatal(err)
		}
		if err := uow.Do(ctx, func(ctx context.Context) error {
			return repo.InsertAgent(ctx, domain.NewAgent(id, "hermes", "", domain.OperatorConfig{}))
		}); err != nil {
			t.Fatal(err)
		}
		return id
	}
	lotse, texter, scout, quiet := agent("lotse"), agent("texter"), agent("scout"), agent("quiet")
	at := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)
	run1, run2 := ids.New(), ids.New()
	agentstest.PersistHeartbeat(t, pool, lotse, at, []ids.ID{run1, run2})
	agentstest.PersistHeartbeat(t, owner, texter, at.Add(time.Second), []ids.ID{})
	agentstest.PersistHeartbeat(t, pool, scout, at.Add(2*time.Second), nil)
	// A later heartbeat replaces the earlier one, also with an older time: the helper sets the
	// row as given, unlike the monotone flush of the tracker.
	agentstest.PersistHeartbeat(t, pool, lotse, at.Add(-time.Minute), []ids.ID{run2})

	reader, err := app.NewHeartbeatReader(repo)
	if err != nil {
		t.Fatal(err)
	}
	list := []ids.ID{quiet, scout, ids.New(), texter, lotse}
	check := func(ctx context.Context) {
		t.Helper()
		got, err := reader.HeartbeatStates(ctx, list)
		if err != nil {
			t.Fatal(err)
		}
		byID := map[ids.ID]int{}
		for i, s := range got {
			byID[s.AgentID] = i
			if i > 0 && got[i-1].AgentID.String() >= s.AgentID.String() {
				t.Fatalf("not ascending by id: %v", got)
			}
		}
		if len(got) != 4 {
			t.Fatalf("got %d states, want 4 (the unknown id is absent): %+v", len(got), got)
		}
		if s := got[byID[lotse]]; !s.LastHeartbeatAt.Equal(at.Add(-time.Minute)) || !s.ActiveRunsReported || !slices.Equal(s.ActiveRunIDs, []ids.ID{run2}) {
			t.Errorf("lotse: %+v", s)
		}
		if s := got[byID[texter]]; !s.LastHeartbeatAt.Equal(at.Add(time.Second)) || !s.ActiveRunsReported || len(s.ActiveRunIDs) != 0 {
			t.Errorf("texter: %+v", s)
		}
		if s := got[byID[scout]]; !s.LastHeartbeatAt.Equal(at.Add(2*time.Second)) || s.ActiveRunsReported || s.ActiveRunIDs != nil {
			t.Errorf("scout: %+v", s)
		}
		if s := got[byID[quiet]]; !s.LastHeartbeatAt.IsZero() || s.ActiveRunsReported {
			t.Errorf("quiet: %+v", s)
		}
	}
	check(ctx)
	if err := uow.Do(ctx, func(ctx context.Context) error { check(ctx); return nil }); err != nil {
		t.Fatal(err)
	}
}

// TestPersistHeartbeat_UnknownAgent: a heartbeat for an agent without a row fails the test.
func TestPersistHeartbeat_UnknownAgent(t *testing.T) {
	t.Parallel()
	db := pgtest.New(t)
	rec := &recorder{TB: t}
	agentstest.PersistHeartbeat(rec, db.Pool(t, pg.RoleApp), ids.New(), time.Now(), nil)
	if !rec.failed {
		t.Fatal("PersistHeartbeat for an unknown agent did not fail")
	}
}

// recorder is a testing.TB that records Fatalf instead of stopping the test.
type recorder struct {
	testing.TB
	failed bool
}

func (r *recorder) Helper() {}

func (r *recorder) Fatalf(string, ...any) { r.failed = true }
