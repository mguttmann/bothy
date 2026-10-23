// SPDX-License-Identifier: Apache-2.0

package postgres_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
	"github.com/OWNER/bothy/services/server/internal/notify"
	"github.com/OWNER/bothy/services/server/internal/notify/adapters/postgres"
	"github.com/OWNER/bothy/services/server/internal/notify/domain"
	"github.com/OWNER/bothy/services/server/internal/platform/pg"
	"github.com/OWNER/bothy/services/server/internal/platform/pg/pgtest"
)

// env is one test database with both adapters of notify, a unit of work and the owner pool for
// fixture rows.
type env struct {
	repo  *postgres.Repository
	regs  *postgres.Registrations
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
	return env{repo: postgres.NewRepository(pool), regs: postgres.NewRegistrations(pool), uow: uow, owner: db.Pool(t, pg.RoleOwner)}
}

// exec runs a fixture statement as bothy_owner.
func (e env) exec(t *testing.T, sql string, args ...any) {
	t.Helper()
	if _, err := e.owner.Exec(context.Background(), sql, args...); err != nil {
		t.Fatalf("%s: %v", sql, err)
	}
}

// do runs fn in one unit of work and fails the test on an error.
func (e env) do(t *testing.T, fn func(ctx context.Context) error) {
	t.Helper()
	if err := e.uow.Do(context.Background(), fn); err != nil {
		t.Fatal(err)
	}
}

// person inserts an active human with one device of platform and one session on it; it
// returns the person, the device and the session.
func (e env) person(t *testing.T, handle, platform string) (user, device, session ids.ID) {
	t.Helper()
	user, device, session = ids.New(), ids.New(), ids.New()
	e.exec(t, `INSERT INTO principals (id, kind, status, handle, display_name) VALUES ($1, 'user', 'active', $2, $2)`, user.String(), handle)
	e.exec(t, `INSERT INTO users (principal_id) VALUES ($1)`, user.String())
	e.exec(t, `INSERT INTO devices (id, user_id, name, platform, state) VALUES ($1, $2, 'phone', $3, 'active')`, device.String(), user.String(), platform)
	e.exec(t, `INSERT INTO auth_sessions (id, user_id, device_id, auth_method, expires_at) VALUES ($1, $2, $3, 'oidc', now() + interval '1 day')`,
		session.String(), user.String(), device.String())
	return user, device, session
}

// registration inserts one push registration as the fixture rows of M3-19 do (registerPush is
// M3-22) and returns its id; a non-empty reason inserts it deactivated.
func (e env) registration(t *testing.T, user, device, session ids.ID, provider, token, reason string) ids.ID {
	t.Helper()
	id := ids.New()
	if reason == "" {
		e.exec(t, `INSERT INTO push_registrations (id, user_id, device_id, auth_session_id, provider, token, app_id)
			VALUES ($1, $2, $3, $4, $5, $6, 'com.example.app')`, id.String(), user.String(), device.String(), session.String(), provider, token)
		return id
	}
	e.exec(t, `INSERT INTO push_registrations (id, user_id, device_id, auth_session_id, provider, token, app_id, disabled_at, disabled_reason)
		VALUES ($1, $2, $3, $4, $5, $6, 'com.example.app', now(), $7)`, id.String(), user.String(), device.String(), session.String(), provider, token, reason)
	return id
}

// TestPushRegistrationsActive: Active returns only rows with disabled_at IS NULL of the asked
// people, grouped by person and in ascending id, reads the ambient transaction, and the DTO has
// no field that could carry the token or the preview hash (M3-19 design note 3).
func TestPushRegistrationsActive(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	e := newEnv(t)
	alex, alexPhone, alexSession := e.person(t, "alex", "ios")
	sam, samPhone, samSession := e.person(t, "sam", "android")
	robin, robinPhone, robinSession := e.person(t, "robin", "ios")
	a1 := e.registration(t, alex, alexPhone, alexSession, "apns", "token-alex-1", "")
	a2 := e.registration(t, alex, alexPhone, alexSession, "wns", "token-alex-2", "")
	e.registration(t, alex, alexPhone, alexSession, "fcm", "token-alex-old", domain.DisableTokenInvalid)
	s1 := e.registration(t, sam, samPhone, samSession, "fcm", "token-sam-1", "")
	e.registration(t, robin, robinPhone, robinSession, "apns", "token-robin", domain.DisableLogout)

	got, err := e.regs.Active(ctx, []ids.ID{alex, sam, robin})
	if err != nil {
		t.Fatal(err)
	}
	ordered := func(list ...ids.ID) []ids.ID {
		slices.SortFunc(list, func(a, b ids.ID) int { return bytes.Compare(a[:], b[:]) })
		return list
	}
	idsOf := func(regs []notify.PushRegistration) []ids.ID {
		out := []ids.ID{}
		for _, r := range regs {
			out = append(out, r.ID)
		}
		return out
	}
	if want := ordered(a1, a2); !slices.Equal(idsOf(got[alex]), want) {
		t.Errorf("Alex: %v, want %v", idsOf(got[alex]), want)
	}
	if !slices.Equal(idsOf(got[sam]), []ids.ID{s1}) {
		t.Errorf("Sam: %v, want %v", idsOf(got[sam]), []ids.ID{s1})
	}
	if regs, has := got[robin]; has {
		t.Errorf("Robin has only a deactivated row, got %v", regs)
	}
	if len(got) != 2 {
		t.Errorf("%d people, want 2", len(got))
	}
	s := got[sam][0]
	if s.UserID != sam || s.DeviceID != samPhone || s.Provider != "fcm" || s.Environment != "production" ||
		s.AppID != "com.example.app" || s.CreatedAt.IsZero() || !s.LastSuccessAt.IsZero() || s.FailureCount != 0 {
		t.Errorf("Sam's registration %+v", s)
	}
	if text := fmt.Sprintf("%+v", got); strings.Contains(text, "token-") {
		t.Errorf("the read port returned a token: %s", text)
	}
	typ := reflect.TypeFor[notify.PushRegistration]()
	for i := range typ.NumField() {
		if name := strings.ToLower(typ.Field(i).Name); strings.Contains(name, "token") || strings.Contains(name, "preview") {
			t.Errorf("PushRegistration has the field %s", typ.Field(i).Name)
		}
	}

	if none, err := e.regs.Active(ctx, nil); err != nil || len(none) != 0 {
		t.Errorf("Active(nil) = %v, %v; want empty", none, err)
	}

	// Inside a unit of work Active sees the uncommitted row of the same transaction.
	e.do(t, func(ctx context.Context) error {
		tx, err := pg.Tx(ctx)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO push_registrations (id, user_id, device_id, auth_session_id, provider, token, app_id)
			VALUES ($1, $2, $3, $4, 'apns', 'token-robin-new', 'com.example.app')`, ids.New().String(), robin.String(), robinPhone.String(), robinSession.String()); err != nil {
			return err
		}
		in, err := e.regs.Active(ctx, []ids.ID{robin})
		if err == nil && len(in[robin]) != 1 {
			t.Errorf("inside the transaction Robin has %d rows, want 1", len(in[robin]))
		}
		return err
	})
}

// TestRegistrationStore_RoundTrip: Load returns the token and the delivery state, Lock locks the
// row FOR UPDATE inside a unit of work only, Update writes only the delivery columns, and
// ActiveCounts counts the active rows per provider.
func TestRegistrationStore_RoundTrip(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	e := newEnv(t)
	alex, phone, session := e.person(t, "alex", "ios")
	sam, samPhone, samSession := e.person(t, "sam", "android")
	id := e.registration(t, alex, phone, session, "apns", "token-alex", "")
	e.registration(t, alex, phone, session, "fcm", "token-alex-fcm", "")
	e.registration(t, sam, samPhone, samSession, "fcm", "token-sam", "")
	e.registration(t, sam, samPhone, samSession, "wns", "token-sam-old", domain.DisableRepeatedFailures)

	reg, state, found, err := e.regs.Load(ctx, id)
	if err != nil || !found {
		t.Fatalf("Load: found %v, %v", found, err)
	}
	if reg.ID != id || reg.UserID != alex || reg.DeviceID != phone || reg.Provider != "apns" || reg.Environment != "production" ||
		reg.AppID != "com.example.app" || reg.Token != "token-alex" || reg.CreatedAt.IsZero() || !reg.CreatedAt.Equal(state.CreatedAt) {
		t.Errorf("Load registration %+v", reg)
	}
	if !state.LastSuccessAt.IsZero() || state.FailureCount != 0 || !state.DisabledAt.IsZero() || state.DisabledReason != "" {
		t.Errorf("Load state %+v, want a fresh row", state)
	}
	if _, _, found, err := e.regs.Load(ctx, ids.New()); found || err != nil {
		t.Errorf("Load of an unknown id: found %v, %v", found, err)
	}

	if _, _, err := e.regs.Lock(ctx, id); !isInternal(err) {
		t.Errorf("Lock outside a unit of work: %v, want an internal error", err)
	}
	if err := e.regs.Update(ctx, id, state); !isInternal(err) {
		t.Errorf("Update outside a unit of work: %v, want an internal error", err)
	}

	success := time.Date(2026, 10, 21, 9, 30, 0, 0, time.UTC)
	disabled := success.Add(time.Hour)
	e.do(t, func(ctx context.Context) error {
		locked, found, err := e.regs.Lock(ctx, id)
		if err != nil || !found {
			return fmt.Errorf("Lock: found %v, %w", found, err)
		}
		if locked != state {
			t.Errorf("Lock state %+v, want %+v", locked, state)
		}
		// The row is locked FOR UPDATE: another transaction cannot lock it.
		_, lockErr := e.owner.Exec(context.Background(), `SELECT 1 FROM push_registrations WHERE id = $1 FOR UPDATE NOWAIT`, id.String())
		if code := sqlstate(lockErr); code != "55P03" {
			t.Errorf("concurrent FOR UPDATE NOWAIT: %v, want SQLSTATE 55P03", lockErr)
		}
		next := locked
		next.LastSuccessAt, next.FailureCount, next.DisabledAt, next.DisabledReason = success, 4, disabled, domain.DisableRepeatedFailures
		// created_at is never written, whatever the state holds.
		next.CreatedAt = success.Add(-24 * time.Hour)
		return e.regs.Update(ctx, id, next)
	})
	e.do(t, func(ctx context.Context) error {
		if _, found, err := e.regs.Lock(ctx, ids.New()); found || err != nil {
			t.Errorf("Lock of an unknown id: found %v, %v", found, err)
		}
		return nil
	})
	_, after, _, err := e.regs.Load(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	want := domain.RegistrationState{CreatedAt: state.CreatedAt, LastSuccessAt: success, FailureCount: 4, DisabledAt: disabled, DisabledReason: domain.DisableRepeatedFailures}
	if !after.CreatedAt.Equal(want.CreatedAt) || !after.LastSuccessAt.Equal(want.LastSuccessAt) || after.FailureCount != want.FailureCount ||
		!after.DisabledAt.Equal(want.DisabledAt) || after.DisabledReason != want.DisabledReason {
		t.Errorf("after Update %+v, want %+v", after, want)
	}

	counts, err := e.regs.ActiveCounts(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if want := map[string]int64{"fcm": 2}; !reflect.DeepEqual(counts, want) {
		t.Errorf("ActiveCounts %v, want %v", counts, want)
	}

	// Clearing the state reactivates the row with NULL columns.
	e.do(t, func(ctx context.Context) error {
		return e.regs.Update(ctx, id, domain.RegistrationState{CreatedAt: state.CreatedAt})
	})
	if _, cleared, _, err := e.regs.Load(ctx, id); err != nil || !cleared.LastSuccessAt.IsZero() || !cleared.DisabledAt.IsZero() ||
		cleared.DisabledReason != "" || cleared.FailureCount != 0 {
		t.Errorf("after clearing: %+v, %v", cleared, err)
	}
	if counts, err := e.regs.ActiveCounts(ctx); err != nil || !reflect.DeepEqual(counts, map[string]int64{"apns": 1, "fcm": 2}) {
		t.Errorf("ActiveCounts after reactivation %v, %v", counts, err)
	}
}

// TestInsertPreferenceIfAbsent: the first insert writes version 1; a second one for the same
// person and target keeps the existing row and returns nothing (13 3.2, design note 17).
func TestInsertPreferenceIfAbsent(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	alex, _, _ := e.person(t, "alex", "ios")
	thread := notify.Target{Kind: "thread", ID: ids.New()}
	at := time.Date(2026, 10, 21, 10, 0, 0, 0, time.UTC)

	if _, _, err := e.repo.InsertPreferenceIfAbsent(context.Background(), alex, notify.Preference{Target: thread, Level: "all"}, at); !isInternal(err) {
		t.Errorf("outside a unit of work: %v, want an internal error", err)
	}
	e.do(t, func(ctx context.Context) error {
		got, inserted, err := e.repo.InsertPreferenceIfAbsent(ctx, alex, notify.Preference{Target: thread, Level: "mute"}, at)
		if err != nil {
			return err
		}
		if want := (notify.Preference{Target: thread, Level: "mute", Version: 1}); !inserted || got != want {
			t.Errorf("first insert %+v inserted %v, want %+v", got, inserted, want)
		}
		return nil
	})
	e.do(t, func(ctx context.Context) error {
		got, inserted, err := e.repo.InsertPreferenceIfAbsent(ctx, alex, notify.Preference{Target: thread, Level: "all"}, at.Add(time.Minute))
		if err != nil {
			return err
		}
		if inserted || got != (notify.Preference{}) {
			t.Errorf("second insert %+v inserted %v, want nothing", got, inserted)
		}
		return nil
	})
	stored, found, err := e.repo.Preference(context.Background(), alex, thread)
	if err != nil || !found || stored.Level != "mute" || stored.Version != 1 {
		t.Errorf("stored %+v found %v, %v; want the first row unchanged", stored, found, err)
	}
}

// isInternal reports an errs error with code internal_error.
func isInternal(err error) bool {
	e, ok := errs.As(err)
	return ok && e.Code == errs.CodeInternalError
}

// sqlstate returns the SQLSTATE of err, or "".
func sqlstate(err error) string {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code
	}
	return ""
}
