// SPDX-License-Identifier: Apache-2.0

package postgres_test

import (
	"context"
	"errors"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/OWNER/bothy/services/server/internal/authz/adapters/postgres"
	"github.com/OWNER/bothy/services/server/internal/authz/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
	"github.com/OWNER/bothy/services/server/internal/platform/pg"
	"github.com/OWNER/bothy/services/server/internal/platform/pg/pgtest"
	"github.com/OWNER/bothy/services/server/test/terminaltest"
)

func TestMain(m *testing.M) { os.Exit(pgtest.Main(m)) }

// t0 is a fixed time in the past with microsecond precision, the precision of timestamptz.
var t0 = time.Date(2026, 10, 12, 9, 0, 0, 0, time.UTC)

// limits are the defaults of group.name_max_chars and group.description_max_chars.
var limits = domain.GroupLimits{NameMaxChars: 100, DescriptionMaxChars: 500}

// env is one test database with the repository, a unit of work and a pool of bothy_app.
type env struct {
	repo *postgres.Repository
	uow  *pg.UnitOfWork
	pool *pg.Pool
}

func newEnv(t *testing.T) env {
	t.Helper()
	pool := pgtest.New(t).Pool(t, pg.RoleApp)
	reg := txn.NewRegistry()
	reg.Seal()
	uow := pg.NewUnitOfWork(pool, pg.UnitConfig{
		StatementTimeout: 30 * time.Second, LockTimeout: 10 * time.Second, IdleInTxTimeout: 60 * time.Second,
	}, reg)
	return env{repo: postgres.NewRepository(pool), uow: uow, pool: pool}
}

// write runs fn in a unit of work declared as a rights change: every write of memberships and
// grants changes rights and is refused otherwise (M1-29). The test registry has no rights hooks.
func (e env) write(fn func(ctx context.Context) error) error {
	return e.uow.Do(context.Background(), fn, txn.RightsChange())
}

// exec runs raw SQL as bothy_app outside a unit of work. Only tests insert principals and
// users this way; the identity adapters are not imported (brief M1-27 "Test data").
func (e env) exec(sql string, args ...any) error {
	_, err := e.pool.Exec(context.Background(), sql, args...)
	return err
}

func (e env) mustExec(t *testing.T, sql string, args ...any) {
	t.Helper()
	if err := e.exec(sql, args...); err != nil {
		t.Fatalf("%s: %v", sql, err)
	}
}

// principal inserts a principal of kind with handle, and a users row for a person.
func (e env) principal(t *testing.T, kind, handle string) ids.ID {
	t.Helper()
	id := ids.New()
	e.mustExec(t, `INSERT INTO principals (id, kind, handle, display_name) VALUES ($1, $2, $3, $3)`, id.String(), kind, handle)
	if kind == "user" {
		e.mustExec(t, `INSERT INTO users (principal_id) VALUES ($1)`, id.String())
	}
	return id
}

// space inserts a spaces row created by by, the target of the foreign key of
// grants.target_space_id (M1-52), and returns its id.
func (e env) space(t *testing.T, by ids.ID) ids.ID {
	t.Helper()
	id := ids.New()
	e.mustExec(t, `INSERT INTO spaces (id, name, sort_key, created_by) VALUES ($1, $2, 'a0', $3)`, id.String(), id.String(), by.String())
	return id
}

func (e env) group(t *testing.T, name string, by ids.ID) domain.Group {
	t.Helper()
	g, err := domain.NewGroup(ids.New(), name, "", limits, by, t0)
	if err != nil {
		t.Fatal(err)
	}
	if err := e.write(func(ctx context.Context) error { return e.repo.InsertGroup(ctx, g) }); err != nil {
		t.Fatalf("insert group %s: %v", name, err)
	}
	return g
}

func caps(t *testing.T, names ...string) domain.Caps {
	t.Helper()
	c, err := domain.ParseCaps(names)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

// wantError checks kind, code and field pointer of err and that its text names no SQL,
// constraint or value.
func wantError(t *testing.T, err error, kind errs.Kind, code errs.Code, pointer string, values ...string) {
	t.Helper()
	e, ok := errs.As(err)
	if !ok {
		t.Fatalf("error %v (%T) is no *errs.Error", err, err)
	}
	if e.Kind() != kind || e.Code != code {
		t.Fatalf("error %v: kind %v code %q, want %v %q", err, e.Kind(), e.Code, kind, code)
	}
	if pointer != "" && (len(e.Fields) != 1 || e.Fields[0].Pointer != pointer) {
		t.Fatalf("fields %+v, want one on %s", e.Fields, pointer)
	}
	texts := []string{e.Error(), e.Detail}
	for _, f := range e.Fields {
		texts = append(texts, f.Detail, f.Code)
	}
	for _, s := range texts {
		for _, v := range append(values, "INSERT", "SELECT", "UPDATE", "groups_", "grants_", "group_memberships_", "authz_") {
			if strings.Contains(s, v) {
				t.Errorf("error text %q contains %q", s, v)
			}
		}
	}
}

// wantViolation checks a raw statement error for SQLSTATE code and constraint name.
func wantViolation(t *testing.T, name string, err error, code, constraint string) {
	t.Helper()
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != code || pgErr.ConstraintName != constraint {
		t.Errorf("%s: %v, want SQLSTATE %s on %s", name, err, code, constraint)
	}
}

// TestGrantChecks holds the checks of grants in 03 section 4.3: each row is rejected by the named
// constraint or accepted. The host rows name a host of the terminal fixtures, since the foreign
// key of target_host_id exists (M2-39).
func TestGrantChecks(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	alex := e.principal(t, "user", "alex")
	sam := e.principal(t, "user", "sam")
	helper := e.principal(t, "agent", "helper")
	tracker := e.principal(t, "app", "tracker")
	support := e.group(t, "Support", alex)
	const ins = `INSERT INTO grants (id, subject_kind, subject_principal_id, subject_group_id, target_kind,
		target_space_id, target_channel_id, target_host_id, level, capabilities, reason, granted_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, 'test', $11)`
	type row struct {
		subjectKind      string
		principal, group any
		targetKind       string
		space, ch, host  any
		level            any
		caps             []string
	}
	hostID, err := terminaltest.InsertHost(context.Background(), e.pool, terminaltest.Host{State: "pairing", CreatedBy: alex})
	if err != nil {
		t.Fatal(err)
	}
	space, channel, host := e.space(t, alex).String(), ids.New().String(), hostID.String()
	for _, tt := range []struct {
		name       string
		r          row
		constraint string // "" means accepted
	}{
		{"agent with trigger", row{"agent", helper.String(), nil, "channel", nil, channel, nil, "trigger", []string{}}, "grants_bot_limits_check"},
		{"app with manage", row{"app", tracker.String(), nil, "channel", nil, channel, nil, "read", []string{"manage"}}, "grants_bot_limits_check"},
		{"agent with upload passes", row{"agent", helper.String(), nil, "channel", nil, channel, nil, "write", []string{"upload"}}, ""},
		{"unknown capability", row{"user", sam.String(), nil, "space", space, nil, nil, "read", []string{"admin"}}, "grants_capabilities_check"},
		{"user grant with group id set", row{"user", sam.String(), support.ID.String(), "space", space, nil, nil, "read", []string{}}, "grants_subject_shape_check"},
		{"group grant without group id", row{"group", nil, nil, "space", space, nil, nil, "read", []string{}}, "grants_subject_shape_check"},
		{"space target without space id", row{"user", sam.String(), nil, "space", nil, nil, nil, "read", []string{}}, "grants_target_shape_check"},
		{"instance target with channel id", row{"user", sam.String(), nil, "instance", nil, channel, nil, "read", []string{}}, "grants_target_shape_check"},
		{"space grant without level", row{"user", sam.String(), nil, "space", space, nil, nil, nil, []string{}}, "grants_level_check"},
		{"unknown level", row{"user", sam.String(), nil, "space", space, nil, nil, "admin", []string{}}, "grants_level_check"},
		{"host target with a level", row{"user", sam.String(), nil, "host", nil, nil, host, "read", []string{"terminal"}}, "grants_level_check"},
		{"host target with upload", row{"user", sam.String(), nil, "host", nil, nil, host, nil, []string{"terminal", "upload"}}, "grants_host_shape_check"},
		{"host target without terminal", row{"group", nil, support.ID.String(), "host", nil, nil, host, nil, []string{}}, "grants_host_shape_check"},
		{"terminal on a space", row{"user", sam.String(), nil, "space", space, nil, nil, "write", []string{"terminal"}}, "grants_terminal_only_on_host_check"},
		// Both grants_bot_limits_check (terminal is not upload) and grants_host_shape_check refuse
		// it; PostgreSQL reports the first in name order.
		{"host grant to an agent", row{"agent", helper.String(), nil, "host", nil, nil, host, nil, []string{"terminal"}}, "grants_bot_limits_check"},
		{"host grant to an app with upload only", row{"app", tracker.String(), nil, "host", nil, nil, host, nil, []string{"upload"}}, "grants_host_shape_check"},
		{"host grant to a person passes", row{"user", sam.String(), nil, "host", nil, nil, host, nil, []string{"terminal"}}, ""},
		{"host grant to a group passes", row{"group", nil, support.ID.String(), "host", nil, nil, host, nil, []string{"terminal"}}, ""},
		{"unknown subject kind", row{"robot", sam.String(), nil, "space", space, nil, nil, "read", []string{}}, "grants_subject_kind_check"},
		{"unknown target kind", row{"user", sam.String(), nil, "thread", nil, nil, nil, "read", []string{}}, "grants_target_kind_check"},
		{"unknown principal", row{"user", ids.New().String(), nil, "space", space, nil, nil, "read", []string{}}, "grants_subject_principal_id_fkey"},
		{"group grant on a space passes", row{"group", nil, support.ID.String(), "space", space, nil, nil, "trigger", []string{"manage", "export", "approve_actions"}}, ""},
	} {
		r := tt.r
		err := e.exec(ins, ids.New().String(), r.subjectKind, r.principal, r.group, r.targetKind, r.space, r.ch, r.host, r.level, r.caps, alex.String())
		if tt.constraint == "" {
			if err != nil {
				t.Errorf("%s: %v", tt.name, err)
			}
			continue
		}
		code := "23514"
		if strings.HasSuffix(tt.constraint, "_fkey") {
			code = "23503"
		}
		wantViolation(t, tt.name, err, code, tt.constraint)
	}
}

// TestGrantImmutable holds R-DM-09 in the database: the trigger authz_grants_immutable allows
// only one revocation and setting each expiry bookkeeping column once.
func TestGrantImmutable(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	alex := e.principal(t, "user", "alex")
	sam := e.principal(t, "user", "sam")
	g, err := domain.NewGrant(ids.New(), domain.Subject{Kind: domain.SubjectUser, PrincipalID: sam},
		domain.Target{Kind: domain.TargetSpace, SpaceID: e.space(t, alex)}, domain.LevelRead, 0, t0.Add(48*time.Hour), "support rotation", alex, t0)
	if err != nil {
		t.Fatal(err)
	}
	if err := e.write(func(ctx context.Context) error { return e.repo.InsertGrant(ctx, g) }); err != nil {
		t.Fatal(err)
	}
	id := g.ID.String()
	const trig = "authz_grants_immutable"
	for name, sql := range map[string]string{
		"level":                            `UPDATE grants SET level = 'trigger' WHERE id = $1`,
		"capabilities":                     `UPDATE grants SET capabilities = '{export}' WHERE id = $1`,
		"subject":                          `UPDATE grants SET subject_principal_id = granted_by WHERE id = $1`,
		"target":                           `UPDATE grants SET target_space_id = gen_random_uuid() WHERE id = $1`,
		"expiry":                           `UPDATE grants SET expires_at = expires_at + interval '1 day' WHERE id = $1`,
		"reason":                           `UPDATE grants SET reason = 'other' WHERE id = $1`,
		"grantor":                          `UPDATE grants SET granted_by = subject_principal_id WHERE id = $1`,
		"granted_at":                       `UPDATE grants SET granted_at = now() WHERE id = $1`,
		"revoke reason without revocation": `UPDATE grants SET revoke_reason = 'x' WHERE id = $1`,
	} {
		wantViolation(t, name, e.exec(sql, id), "23514", trig)
	}
	// An update that changes nothing passes; it is no change.
	e.mustExec(t, `UPDATE grants SET level = level WHERE id = $1`, id)

	e.mustExec(t, `UPDATE grants SET expiry_reminded_at = now() WHERE id = $1`, id)
	wantViolation(t, "second reminder", e.exec(`UPDATE grants SET expiry_reminded_at = now() + interval '1 second' WHERE id = $1`, id), "23514", trig)
	wantViolation(t, "reminder cleared", e.exec(`UPDATE grants SET expiry_reminded_at = NULL WHERE id = $1`, id), "23514", trig)
	e.mustExec(t, `UPDATE grants SET expiry_reported_at = now() WHERE id = $1`, id)
	wantViolation(t, "second report", e.exec(`UPDATE grants SET expiry_reported_at = now() + interval '1 second' WHERE id = $1`, id), "23514", trig)

	e.mustExec(t, `UPDATE grants SET revoked_at = now(), revoked_by = $2, revoke_reason = 'left the team' WHERE id = $1`, id, alex.String())
	wantViolation(t, "second revocation", e.exec(`UPDATE grants SET revoked_at = now() + interval '1 second' WHERE id = $1`, id), "23514", trig)
	wantViolation(t, "revocation undone", e.exec(`UPDATE grants SET revoked_at = NULL, revoked_by = NULL, revoke_reason = NULL WHERE id = $1`, id), "23514", trig)
	wantViolation(t, "revoke reason changed", e.exec(`UPDATE grants SET revoke_reason = 'other' WHERE id = $1`, id), "23514", trig)

	// The repository maps the trigger like a check and never reaches it for a second revocation.
	err = e.write(func(ctx context.Context) error {
		_, found, err := e.repo.RevokeGrant(ctx, g.ID, alex, "again", t0.Add(time.Hour))
		if found {
			t.Error("second RevokeGrant found the grant")
		}
		return err
	})
	if err != nil {
		t.Errorf("second RevokeGrant: %v", err)
	}
}

// TestGroupMembersArePeople holds R-DM-07: a membership for an agent principal fails, because it
// has no users row.
func TestGroupMembersArePeople(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	alex := e.principal(t, "user", "alex")
	helper := e.principal(t, "agent", "helper")
	support := e.group(t, "Support", alex)
	m, err := domain.NewMembership(ids.New(), support.ID, helper, alex, time.Time{}, t0)
	if err != nil {
		t.Fatal(err)
	}
	err = e.write(func(ctx context.Context) error { return e.repo.InsertMembership(ctx, m) })
	wantError(t, err, errs.KindBadRequest, errs.CodeValidationFailed, "/user_id", helper.String())
}

func TestActiveMembershipUnique(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	alex := e.principal(t, "user", "alex")
	sam := e.principal(t, "user", "sam")
	support := e.group(t, "Support", alex)
	add := func() (domain.Membership, error) {
		m, err := domain.NewMembership(ids.New(), support.ID, sam, alex, time.Time{}, t0)
		if err != nil {
			t.Fatal(err)
		}
		return m, e.write(func(ctx context.Context) error { return e.repo.InsertMembership(ctx, m) })
	}
	first, err := add()
	if err != nil {
		t.Fatal(err)
	}
	_, err = add()
	wantError(t, err, errs.KindConflict, errs.CodeConflict, "/user_id", sam.String())

	var removed bool
	if err := e.write(func(ctx context.Context) (err error) {
		removed, err = e.repo.RemoveMembership(ctx, first.ID, alex, t0.Add(time.Hour))
		return err
	}); err != nil || !removed {
		t.Fatalf("RemoveMembership: %v, removed %v", err, removed)
	}
	if err := e.write(func(ctx context.Context) (err error) {
		removed, err = e.repo.RemoveMembership(ctx, first.ID, alex, t0.Add(2*time.Hour))
		return err
	}); err != nil || removed {
		t.Fatalf("second RemoveMembership: %v, removed %v", err, removed)
	}
	if _, err := add(); err != nil {
		t.Fatalf("re-adding after removal: %v", err)
	}
}

func TestGroupNameUniqueAmongLive(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	alex := e.principal(t, "user", "alex")
	support := e.group(t, "Support", alex)
	dup, err := domain.NewGroup(ids.New(), "support", "", limits, alex, t0)
	if err != nil {
		t.Fatal(err)
	}
	err = e.write(func(ctx context.Context) error { return e.repo.InsertGroup(ctx, dup) })
	wantError(t, err, errs.KindConflict, errs.CodeConflict, "/name", "support", "Support")

	var deleted bool
	if err := e.write(func(ctx context.Context) (err error) {
		deleted, err = e.repo.SoftDeleteGroup(ctx, support.ID, t0.Add(time.Hour))
		return err
	}); err != nil || !deleted {
		t.Fatalf("SoftDeleteGroup: %v, deleted %v", err, deleted)
	}
	if err := e.write(func(ctx context.Context) (err error) {
		deleted, err = e.repo.SoftDeleteGroup(ctx, support.ID, t0.Add(2*time.Hour))
		return err
	}); err != nil || deleted {
		t.Fatalf("second SoftDeleteGroup: %v, deleted %v", err, deleted)
	}
	if err := e.write(func(ctx context.Context) error { return e.repo.InsertGroup(ctx, dup) }); err != nil {
		t.Fatalf("a deleted group did not free its name: %v", err)
	}
	got, found, err := e.repo.GetGroup(context.Background(), support.ID)
	if err != nil || !found || !got.DeletedAt.Equal(t0.Add(time.Hour)) {
		t.Fatalf("GetGroup of the deleted group: %+v, %v, %v", got, found, err)
	}
}

func TestWritesNeedAUnitOfWork(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	alex := e.principal(t, "user", "alex")
	g, err := domain.NewGroup(ids.New(), "Research", "", limits, alex, t0)
	if err != nil {
		t.Fatal(err)
	}
	wantError(t, e.repo.InsertGroup(context.Background(), g), errs.KindInternal, errs.CodeInternalError, "")
}

// TestErrorMapping: violations reached through the repository become validation_failed with the
// field of the constraint, never with SQL text.
func TestErrorMapping(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	alex := e.principal(t, "user", "alex")
	helper := e.principal(t, "agent", "helper")
	insert := func(g domain.Grant) error {
		return e.write(func(ctx context.Context) error { return e.repo.InsertGrant(ctx, g) })
	}
	base := domain.Grant{ID: ids.New(), Subject: domain.Subject{Kind: domain.SubjectAgent, PrincipalID: helper},
		Target: domain.Target{Kind: domain.TargetChannel, ChannelID: ids.New()}, Level: domain.LevelTrigger,
		Reason: "bypassing NewGrant", GrantedBy: alex, GrantedAt: t0}
	// Built by hand: CheckCreate refuses it, the database is the last line.
	wantError(t, insert(base), errs.KindBadRequest, errs.CodeValidationFailed, "/subject")

	unknown := base
	unknown.ID, unknown.Level, unknown.Subject.PrincipalID = ids.New(), domain.LevelRead, ids.New()
	wantError(t, insert(unknown), errs.KindBadRequest, errs.CodeValidationFailed, "/subject", unknown.Subject.PrincipalID.String())

	terminal := base
	terminal.ID, terminal.Level, terminal.Subject = ids.New(), domain.LevelRead, domain.Subject{Kind: domain.SubjectUser, PrincipalID: alex}
	terminal.Caps = caps(t, "terminal")
	wantError(t, insert(terminal), errs.KindBadRequest, errs.CodeValidationFailed, "/capabilities")

	grantor := terminal
	grantor.ID, grantor.Caps, grantor.GrantedBy = ids.New(), 0, ids.New()
	wantError(t, insert(grantor), errs.KindBadRequest, errs.CodeValidationFailed, "/granted_by")

	g, err := domain.NewGroup(ids.New(), "Marketing", "", limits, ids.New(), t0)
	if err != nil {
		t.Fatal(err)
	}
	wantError(t, e.write(func(ctx context.Context) error { return e.repo.InsertGroup(ctx, g) }),
		errs.KindBadRequest, errs.CodeValidationFailed, "/created_by")

	m, err := domain.NewMembership(ids.New(), ids.New(), alex, alex, time.Time{}, t0)
	if err != nil {
		t.Fatal(err)
	}
	wantError(t, e.write(func(ctx context.Context) error { return e.repo.InsertMembership(ctx, m) }),
		errs.KindBadRequest, errs.CodeValidationFailed, "/group_id")

	reused := domain.Grant{ID: ids.New(), Subject: domain.Subject{Kind: domain.SubjectUser, PrincipalID: alex},
		Target: domain.Target{Kind: domain.TargetInstance}, Level: domain.LevelRead, Reason: "first", GrantedBy: alex, GrantedAt: t0}
	if err := insert(reused); err != nil {
		t.Fatal(err)
	}
	wantError(t, insert(reused), errs.KindInternal, errs.CodeInternalError, "")
}

// TestStoreRoundTrip: insert, get, revoke, the active lists per principal and per groups, and
// memberships with their group rows, all ordered by id.
func TestStoreRoundTrip(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	ctx := context.Background()
	alex := e.principal(t, "user", "alex")
	sam := e.principal(t, "user", "sam")
	robin := e.principal(t, "user", "robin")
	helper := e.principal(t, "agent", "helper")
	support := e.group(t, "Support", alex)
	engineering := e.group(t, "Engineering", alex)
	research := e.group(t, "Research", alex)

	mustGrant := func(s domain.Subject, tg domain.Target, l domain.Level, c domain.Caps, exp time.Time) domain.Grant {
		t.Helper()
		g, err := domain.NewGrant(ids.New(), s, tg, l, c, exp, "project work", alex, t0)
		if err != nil {
			t.Fatal(err)
		}
		if err := e.write(func(ctx context.Context) error { return e.repo.InsertGrant(ctx, g) }); err != nil {
			t.Fatal(err)
		}
		return g
	}
	samS := domain.Subject{Kind: domain.SubjectUser, PrincipalID: sam}
	space := domain.Target{Kind: domain.TargetSpace, SpaceID: e.space(t, alex)}
	g1 := mustGrant(samS, space, domain.LevelWrite, caps(t, "upload", "export"), t0.Add(24*time.Hour))
	g2 := mustGrant(samS, domain.Target{Kind: domain.TargetInstance}, domain.LevelRead, 0, time.Time{})
	hostID, err := terminaltest.InsertHost(ctx, e.pool, terminaltest.Host{State: "pairing", CreatedBy: alex})
	if err != nil {
		t.Fatal(err)
	}
	g3 := mustGrant(samS, domain.Target{Kind: domain.TargetHost, HostID: hostID}, domain.LevelNone, caps(t, "terminal"), time.Time{})
	mustGrant(domain.Subject{Kind: domain.SubjectUser, PrincipalID: robin}, space, domain.LevelRead, 0, time.Time{})
	mustGrant(domain.Subject{Kind: domain.SubjectAgent, PrincipalID: helper}, space, domain.LevelWrite, caps(t, "upload"), time.Time{})
	gs := mustGrant(domain.Subject{Kind: domain.SubjectGroup, GroupID: support.ID}, space, domain.LevelTrigger, caps(t, "manage", "approve_actions"), time.Time{})
	ge := mustGrant(domain.Subject{Kind: domain.SubjectGroup, GroupID: engineering.ID}, domain.Target{Kind: domain.TargetChannel, ChannelID: ids.New()}, domain.LevelRead, 0, time.Time{})
	mustGrant(domain.Subject{Kind: domain.SubjectGroup, GroupID: research.ID}, space, domain.LevelRead, 0, time.Time{})

	got, found, err := e.repo.GetGrant(ctx, g1.ID)
	if err != nil || !found || !reflect.DeepEqual(got, g1) {
		t.Fatalf("GetGrant:\n got %+v, %v, %v\nwant %+v", got, found, err, g1)
	}
	if got, found, _ := e.repo.GetGrant(ctx, g3.ID); !found || !reflect.DeepEqual(got, g3) {
		t.Fatalf("GetGrant host:\n got %+v\nwant %+v", got, g3)
	}
	if _, found, err := e.repo.GetGrant(ctx, ids.New()); err != nil || found {
		t.Fatalf("GetGrant unknown: %v, %v", found, err)
	}

	var revoked domain.Grant
	if err := e.write(func(ctx context.Context) (err error) {
		revoked, found, err = e.repo.RevokeGrant(ctx, g2.ID, alex, "no longer needed", t0.Add(time.Hour))
		return err
	}); err != nil || !found {
		t.Fatalf("RevokeGrant: %v, %v", err, found)
	}
	want := g2
	want.RevokedAt, want.RevokedBy, want.RevokeReason = t0.Add(time.Hour), alex, "no longer needed"
	if !reflect.DeepEqual(revoked, want) {
		t.Fatalf("revoked:\n got %+v\nwant %+v", revoked, want)
	}
	if revoked.InEffect(t0) {
		t.Error("a revoked grant is in effect")
	}

	active, err := e.repo.ActiveGrantsOfPrincipal(ctx, sam)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(active, []domain.Grant{g1, g3}) {
		t.Fatalf("ActiveGrantsOfPrincipal:\n got %+v\nwant %+v", active, []domain.Grant{g1, g3})
	}
	byGroups, err := e.repo.ActiveGrantsOfGroups(ctx, []ids.ID{engineering.ID, support.ID})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(byGroups, []domain.Grant{gs, ge}) {
		t.Fatalf("ActiveGrantsOfGroups:\n got %+v\nwant %+v", byGroups, []domain.Grant{gs, ge})
	}
	if none, err := e.repo.ActiveGrantsOfGroups(ctx, nil); err != nil || len(none) != 0 {
		t.Fatalf("ActiveGrantsOfGroups(nil): %v, %v", none, err)
	}

	// Memberships: Sam in Support (expiring), Engineering (group deleted later) and Research
	// (removed); Robin in Support.
	addM := func(g domain.Group, u ids.ID, exp time.Time) domain.Membership {
		t.Helper()
		m, err := domain.NewMembership(ids.New(), g.ID, u, alex, exp, t0)
		if err != nil {
			t.Fatal(err)
		}
		if err := e.write(func(ctx context.Context) error { return e.repo.InsertMembership(ctx, m) }); err != nil {
			t.Fatal(err)
		}
		return m
	}
	m1 := addM(support, sam, t0.Add(time.Hour))
	m2 := addM(engineering, sam, time.Time{})
	m3 := addM(research, sam, time.Time{})
	addM(support, robin, time.Time{})
	if err := e.write(func(ctx context.Context) error {
		if _, err := e.repo.RemoveMembership(ctx, m3.ID, alex, t0.Add(time.Minute)); err != nil {
			return err
		}
		_, err := e.repo.SoftDeleteGroup(ctx, engineering.ID, t0.Add(time.Minute))
		return err
	}); err != nil {
		t.Fatal(err)
	}
	ms, err := e.repo.ActiveMembershipsOfUser(ctx, sam)
	if err != nil {
		t.Fatal(err)
	}
	deletedEng := engineering
	deletedEng.DeletedAt = t0.Add(time.Minute)
	wantMs := []domain.MembershipWithGroup{{Membership: m1, Group: support}, {Membership: m2, Group: deletedEng}}
	if !reflect.DeepEqual(ms, wantMs) {
		t.Fatalf("ActiveMembershipsOfUser:\n got %+v\nwant %+v", ms, wantMs)
	}
	if !ms[0].Membership.InEffect(ms[0].Group, t0) || ms[1].Membership.InEffect(ms[1].Group, t0.Add(time.Hour)) {
		t.Error("validity of the read memberships")
	}

	// Reads join the ambient transaction: a grant inserted in the same unit of work is visible.
	if err := e.write(func(ctx context.Context) error {
		g, err := domain.NewGrant(ids.New(), samS, space, domain.LevelRead, 0, time.Time{}, "inside", alex, t0)
		if err != nil {
			return err
		}
		if err := e.repo.InsertGrant(ctx, g); err != nil {
			return err
		}
		list, err := e.repo.ActiveGrantsOfPrincipal(ctx, sam)
		if err != nil {
			return err
		}
		if len(list) != 3 {
			t.Errorf("inside the unit of work: %d grants, want 3", len(list))
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}
