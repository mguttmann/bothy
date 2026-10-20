// SPDX-License-Identifier: Apache-2.0

package postgres_test

import (
	"bytes"
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/agents/adapters/postgres"
	"github.com/OWNER/bothy/services/server/internal/agents/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
	"github.com/OWNER/bothy/services/server/internal/platform/pg"
	"github.com/OWNER/bothy/services/server/internal/platform/pg/pgtest"
)

func TestMain(m *testing.M) { os.Exit(pgtest.Main(m)) }

// env is one test database with the repository, a unit of work and the owner pool for fixtures.
type env struct {
	repo  *postgres.Repository
	uow   *pg.UnitOfWork
	owner *pg.Pool
}

func newEnv(t *testing.T) env {
	t.Helper()
	db := pgtest.New(t)
	pool := db.Pool(t, pg.RoleApp)
	reg := txn.NewRegistry()
	reg.Seal()
	uow := pg.NewUnitOfWork(pool, pg.UnitConfig{StatementTimeout: 30 * time.Second, LockTimeout: 10 * time.Second, IdleInTxTimeout: 60 * time.Second}, reg)
	return env{repo: postgres.NewRepository(pool), uow: uow, owner: db.Pool(t, pg.RoleOwner)}
}

// principal inserts a principal of kind as a fixture.
func (e env) principal(t *testing.T, kind, handle string) ids.ID {
	t.Helper()
	id := ids.New()
	if _, err := e.owner.Exec(context.Background(), `INSERT INTO principals (id, kind, handle, display_name) VALUES ($1, $2, $3, $3)`,
		id.String(), kind, handle); err != nil {
		t.Fatal(err)
	}
	if kind == "user" {
		if _, err := e.owner.Exec(context.Background(), `INSERT INTO users (principal_id) VALUES ($1)`, id.String()); err != nil {
			t.Fatal(err)
		}
	}
	return id
}

// TestRepositoryRoundTrip covers insert, read, lock, list, the personal lists and update; writes
// and locks outside a unit of work fail.
func TestRepositoryRoundTrip(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	ctx := context.Background()
	holder := e.principal(t, "user", "alex")
	cfg, err := domain.ParseOperatorConfig(map[string]any{"default_model": "vendor/model-1", "supports_voice": false})
	if err != nil {
		t.Fatal(err)
	}
	a := domain.NewAgent(e.principal(t, "agent", "lotse"), "hermes", "Pilot", cfg)
	b := domain.NewAgent(e.principal(t, "agent", "scout"), "custom", "", domain.OperatorConfig{})
	if err := e.repo.InsertAgent(ctx, a); err == nil {
		t.Fatal("InsertAgent outside a unit of work")
	}
	if _, _, err := e.repo.LockAgent(ctx, a.ID); err == nil {
		t.Fatal("LockAgent outside a unit of work")
	}
	if err := e.uow.Do(ctx, func(ctx context.Context) error {
		if err := e.repo.InsertAgent(ctx, a); err != nil {
			return err
		}
		return e.repo.InsertAgent(ctx, b)
	}); err != nil {
		t.Fatal(err)
	}
	got, found, err := e.repo.Agent(ctx, a.ID)
	if err != nil || !found || got.AdapterKind != "hermes" || got.Description != "Pilot" || !got.Config.Equal(cfg) || got.Version != 1 ||
		got.Policy.Mode != domain.Organization || got.ToolClass != domain.ToolChat || got.Reported.State != domain.RuntimeUnknown {
		t.Fatalf("Agent = %+v, %v, %v", got, found, err)
	}
	if _, found, err := e.repo.Agent(ctx, ids.New()); err != nil || found {
		t.Fatalf("unknown = %v, %v", found, err)
	}
	list, err := e.repo.ListAgents(ctx, ids.ID{}, 10)
	if err != nil || len(list) != 2 || list[0].ID != a.ID {
		t.Fatalf("ListAgents = %+v, %v", list, err)
	}
	if after, err := e.repo.ListAgents(ctx, a.ID, 10); err != nil || len(after) != 1 || after[0].ID != b.ID {
		t.Fatalf("ListAgents after = %+v, %v", after, err)
	}
	at := time.Date(2026, 10, 13, 9, 0, 0, 0, time.UTC)
	err = e.uow.Do(ctx, func(ctx context.Context) error {
		rec, found, err := e.repo.LockAgent(ctx, a.ID)
		if err != nil || !found {
			t.Fatalf("LockAgent = %v, %v", found, err)
		}
		if _, err := rec.SetPolicy(domain.Policy{Mode: domain.Personal, HolderID: holder}, holder, at); err != nil {
			return err
		}
		rec.SetToolClass(domain.ToolScoped)
		if err := e.repo.UpdateAgent(ctx, rec); err != nil {
			return err
		}
		if _, found, err := e.repo.LockAgent(ctx, ids.New()); err != nil || found {
			t.Fatalf("LockAgent unknown = %v, %v", found, err)
		}
		rows, err := e.repo.LockAgents(ctx, []ids.ID{b.ID, a.ID, ids.New(), a.ID})
		if err != nil || len(rows) != 2 || rows[0].ID != a.ID {
			t.Fatalf("LockAgents = %+v, %v", rows, err)
		}
		if rows, err := e.repo.LockAgents(ctx, nil); err != nil || len(rows) != 0 {
			t.Fatalf("LockAgents empty = %+v, %v", rows, err)
		}
		personal, err := e.repo.LockPersonalAgents(ctx)
		if err != nil || len(personal) != 1 || personal[0].ID != a.ID {
			t.Fatalf("LockPersonalAgents = %+v, %v", personal, err)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	got, _, _ = e.repo.Agent(ctx, a.ID)
	if got.Policy.HolderID != holder || got.Version != 3 || got.ToolClass != domain.ToolScoped || !got.BillingModeChangedAt.Equal(at) || got.BillingModeChangedBy != holder {
		t.Fatalf("after update %+v", got)
	}
	if personal, err := e.repo.PersonalAgents(ctx); err != nil || len(personal) != 1 {
		t.Fatalf("PersonalAgents = %+v, %v", personal, err)
	}
}

// TestRepositoryReportedState reads the reported columns a heartbeat writes from M2-20 on.
func TestRepositoryReportedState(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	ctx := context.Background()
	a := domain.NewAgent(e.principal(t, "agent", "lotse"), "hermes", "", domain.OperatorConfig{})
	if err := e.uow.Do(ctx, func(ctx context.Context) error { return e.repo.InsertAgent(ctx, a) }); err != nil {
		t.Fatal(err)
	}
	if _, err := e.owner.Exec(ctx, `UPDATE agents SET reported_runtime_state = 'online', reported_billing_mode = 'personal',
		reported_tool_class = 'host', reported_runtime_version = '2.3.1', last_heartbeat_at = now(),
		reported_details = '{"billing_modes_supported":["organization"]}' WHERE principal_id = $1`, a.ID.String()); err != nil {
		t.Fatal(err)
	}
	got, _, err := e.repo.Agent(ctx, a.ID)
	if err != nil || got.Reported.State != domain.RuntimeOnline || got.Reported.BillingMode != domain.Personal || got.EffectiveToolClass() != domain.ToolHost ||
		got.Reported.RuntimeVersion != "2.3.1" || got.Reported.LastHeartbeatAt.IsZero() || len(got.Reported.BillingModesSupported) != 1 {
		t.Fatalf("reported %+v, %v", got.Reported, err)
	}
	// A stored configuration outside its schema is an internal error, never a client error.
	if _, err := e.owner.Exec(ctx, `UPDATE agents SET operator_config = '{"api_key":"x"}' WHERE principal_id = $1`, a.ID.String()); err != nil {
		t.Fatal(err)
	}
	_, _, err = e.repo.Agent(ctx, a.ID)
	if ee, ok := errs.As(err); !ok || ee.Kind() != errs.KindInternal || strings.Contains(err.Error(), "api_key\":\"x") {
		t.Fatalf("broken config: %v", err)
	}
}

// TestRepositoryMapsConstraints: the kind trigger and the holder check become validation_failed
// with the field, without SQL or values.
func TestRepositoryMapsConstraints(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	ctx := context.Background()
	person := e.principal(t, "user", "alex")
	err := e.uow.Do(ctx, func(ctx context.Context) error {
		return e.repo.InsertAgent(ctx, domain.NewAgent(person, "hermes", "", domain.OperatorConfig{}))
	})
	wantField(t, err, "/principal_id")
	agent := e.principal(t, "agent", "lotse")
	broken := domain.NewAgent(agent, "hermes", "", domain.OperatorConfig{})
	broken.Policy = domain.Policy{Mode: domain.Personal}
	err = e.uow.Do(ctx, func(ctx context.Context) error { return e.repo.InsertAgent(ctx, broken) })
	wantField(t, err, "/credential_holder_id")
	broken.Policy = domain.Policy{Mode: domain.Personal, HolderID: agent}
	err = e.uow.Do(ctx, func(ctx context.Context) error { return e.repo.InsertAgent(ctx, broken) })
	wantField(t, err, "/credential_holder_id")
}

// wantField checks validation_failed on pointer without SQL text or constraint names.
func wantField(t *testing.T, err error, pointer string) {
	t.Helper()
	e, ok := errs.As(err)
	if !ok || e.Code != errs.CodeValidationFailed || len(e.Fields) != 1 || e.Fields[0].Pointer != pointer {
		t.Fatalf("error %v, want validation_failed on %s", err, pointer)
	}
	for _, s := range []string{e.Detail, e.Fields[0].Detail} {
		if strings.Contains(s, "agents_") || strings.Contains(s, "INSERT") {
			t.Errorf("error text %q leaks SQL", s)
		}
	}
}

// TestRepositoryAgents reads the records of a list without a lock (M2-05): unknown ids, people
// and duplicates are left out, the order is ascending by id, an empty list reads nothing, and the
// read works on the pool as well as inside a unit of work.
func TestRepositoryAgents(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	ctx := context.Background()
	sam := e.principal(t, "user", "sam")
	a := domain.NewAgent(e.principal(t, "agent", "lotse"), "hermes", "", domain.OperatorConfig{})
	b := domain.NewAgent(e.principal(t, "agent", "scout"), "custom", "", domain.OperatorConfig{})
	if err := e.uow.Do(ctx, func(ctx context.Context) error {
		if err := e.repo.InsertAgent(ctx, a); err != nil {
			return err
		}
		return e.repo.InsertAgent(ctx, b)
	}); err != nil {
		t.Fatal(err)
	}
	first, second := a.ID, b.ID
	if bytes.Compare(first[:], second[:]) > 0 {
		first, second = second, first
	}
	rows, err := e.repo.Agents(ctx, []ids.ID{second, sam, ids.New(), first, second})
	if err != nil || len(rows) != 2 || rows[0].ID != first || rows[1].ID != second {
		t.Fatalf("Agents = %+v, %v", rows, err)
	}
	if rows, err := e.repo.Agents(ctx, nil); err != nil || len(rows) != 0 {
		t.Fatalf("Agents empty = %+v, %v", rows, err)
	}
	if err := e.uow.Do(ctx, func(ctx context.Context) error {
		rows, err := e.repo.Agents(ctx, []ids.ID{a.ID})
		if err != nil || len(rows) != 1 || rows[0].ID != a.ID || rows[0].AdapterKind != "hermes" {
			t.Fatalf("Agents in a unit of work = %+v, %v", rows, err)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}
