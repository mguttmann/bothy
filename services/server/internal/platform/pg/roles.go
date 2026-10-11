// SPDX-License-Identifier: Apache-2.0

package pg

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/OWNER/bothy/services/server/internal/platform/secrets"
)

// Role is a database role of docs/spec/16-betrieb.md 7.1.
type Role string

// The four roles of the server. bothy_owner owns the database and the schema public and runs
// the DDL of bothy migrate; bothy_app does the DML of api and worker; bothy_maint trims the
// audit log; bothy_monitor has pg_monitor for the collector.
const (
	RoleOwner   Role = "bothy_owner"
	RoleApp     Role = "bothy_app"
	RoleMaint   Role = "bothy_maint"
	RoleMonitor Role = "bothy_monitor"
)

// Roles returns the four roles in a fixed order: owner, app, maint, monitor.
func Roles() []Role { return []Role{RoleOwner, RoleApp, RoleMaint, RoleMonitor} }

func (r Role) known() bool {
	switch r {
	case RoleOwner, RoleApp, RoleMaint, RoleMonitor:
		return true
	default:
		return false
	}
}

// identifier returns the role name quoted for SQL.
func (r Role) identifier() string { return pgx.Identifier{string(r)}.Sanitize() }

// roleAttributes are set on every bootstrap, also on a role that exists, so that a role an
// operator widened by hand is narrowed again (16 7.1 rule 1).
const roleAttributes = "LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS"

// bootstrapLockID serialises concurrent bootstraps on one database with a transaction-level
// advisory lock; its value is arbitrary but fixed ("bothyboo" as eight ASCII bytes).
const bootstrapLockID int64 = 0x626f746879626f6f

// Bootstrap creates or updates the four roles and their passwords (BootstrapRoles), then makes
// database owned by bothy_owner and grants the database and schema rights
// (BootstrapDatabase). super is a connection of a superuser (migrate.bootstrap_user) to
// database. It is idempotent: a second run with the same passwords changes nothing visible,
// with new passwords it rotates them.
//
// passwords holds a value for each role of Roles and nothing else; each must be non-empty
// printable ASCII. The passwords reach the server only as SCRAM-SHA-256 verifiers, so neither
// the server log (also with log_statement = all) nor pg_stat_activity ever shows one. Errors
// name roles and the database, never a password.
func Bootstrap(ctx context.Context, super *pgx.Conn, database string, passwords map[Role]secrets.Value) error {
	if err := BootstrapRoles(ctx, super, passwords); err != nil {
		return err
	}
	return BootstrapDatabase(ctx, super, database)
}

// BootstrapRoles is the cluster-wide part of Bootstrap: per role CREATE ROLE when it is missing,
// else ALTER ROLE, always with LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION
// NOBYPASSRLS and the verifier of its password; then GRANT pg_monitor TO bothy_monitor. All of
// it runs in one transaction. A concurrent bootstrap that creates the same role first makes the
// transaction fail with duplicate_object or unique_violation; it is then run once more and finds
// the roles present.
func BootstrapRoles(ctx context.Context, super *pgx.Conn, passwords map[Role]secrets.Value) error {
	if err := checkPasswords(passwords); err != nil {
		return err
	}
	verifiers := make(map[Role]string, len(passwords))
	for _, r := range Roles() {
		pw := passwords[r].Bytes()
		v, err := newSCRAMVerifier(pw)
		clear(pw)
		if err != nil {
			return err
		}
		verifiers[r] = v
	}
	const attempts = 2
	var err error
	for range attempts {
		err = pgx.BeginFunc(ctx, super, func(tx pgx.Tx) error { return bootstrapRolesTx(ctx, tx, verifiers) })
		if !isRace(err) {
			break
		}
	}
	if err != nil {
		return fmt.Errorf("pg: bootstrap roles: %w", err)
	}
	return nil
}

// bootstrapRolesTx runs the statements of BootstrapRoles in tx.
func bootstrapRolesTx(ctx context.Context, tx pgx.Tx, verifiers map[Role]string) error {
	if _, err := tx.Exec(ctx, "SELECT pg_advisory_xact_lock($1)", bootstrapLockID); err != nil {
		return err
	}
	for _, r := range Roles() {
		var exists bool
		if err := tx.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = $1)", string(r)).Scan(&exists); err != nil {
			return err
		}
		stmt, err := roleStatement(!exists, r, verifiers[r])
		if err != nil {
			return err
		}
		// Simple protocol: the statement is sent once as it is, never kept in pgx's statement
		// cache.
		if _, err := tx.Exec(ctx, stmt, pgx.QueryExecModeSimpleProtocol); err != nil {
			return fmt.Errorf("role %s: %w", r, err)
		}
	}
	_, err := tx.Exec(ctx, grantMonitorStatement(), pgx.QueryExecModeSimpleProtocol)
	return err
}

// grantMonitorStatement returns the GRANT of pg_monitor to bothy_monitor; the role is a constant,
// quoted as an identifier.
func grantMonitorStatement() string { return "GRANT pg_monitor TO " + RoleMonitor.identifier() }

// isRace reports the errors of a concurrent bootstrap that created a role first.
func isRace(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && (pgErr.Code == "42710" || pgErr.Code == "23505")
}

// roleStatement returns CREATE ROLE (create) or ALTER ROLE for role with the attributes of
// roleAttributes and the password verifier. DDL takes no bind parameters, so the statement is
// built here: the role is one of the four constants, quoted as an identifier, and the verifier
// must have the exact form of verifierPattern (base64, digits, "$" and ":"), so the literal
// cannot be broken out of.
func roleStatement(create bool, role Role, verifier string) (string, error) {
	if !role.known() {
		return "", fmt.Errorf("pg: unknown role %q", string(role))
	}
	if !verifierPattern.MatchString(verifier) {
		return "", errors.New("pg: password verifier has not the SCRAM-SHA-256 form")
	}
	verb := "ALTER"
	if create {
		verb = "CREATE"
	}
	return verb + " ROLE " + role.identifier() + " WITH " + roleAttributes + " PASSWORD '" + verifier + "'", nil
}

// checkPasswords requires exactly one value per role of Roles, each non-empty printable ASCII
// (0x20 to 0x7e): for those, SASLprep (PostgreSQL, libpq) and its successor OpaqueString (pgx)
// change nothing, so every client derives the key the verifier was computed from. Generated
// passwords are base64url (configuration reference, "Geheimnisse"). Errors name the role only.
func checkPasswords(passwords map[Role]secrets.Value) error {
	var problems []string
	for r := range passwords {
		if !r.known() {
			problems = append(problems, fmt.Sprintf("unknown role %q", string(r)))
		}
	}
	for _, r := range Roles() {
		v, ok := passwords[r]
		switch {
		case !ok:
			problems = append(problems, fmt.Sprintf("no password for role %s", r))
		case v.Len() == 0:
			problems = append(problems, fmt.Sprintf("the password of role %s is empty", r))
		default:
			b := v.Bytes()
			if !printableASCII(string(b)) {
				problems = append(problems, fmt.Sprintf("the password of role %s must be printable ASCII", r))
			}
			clear(b)
		}
	}
	if len(problems) > 0 {
		return errors.New("pg: bootstrap: " + strings.Join(problems, "; "))
	}
	return nil
}

// BootstrapDatabase is the per-database part of Bootstrap: bothy_owner owns database and its
// schema public; PUBLIC loses every right on both; bothy_app, bothy_maint and bothy_monitor get
// CONNECT, bothy_app and bothy_maint USAGE on public (16 7.1 rule 2). No default privileges:
// each migration grants what bothy_app needs on its tables (rule 3).
//
// super must be connected to database (the schema statements act on the current database);
// otherwise BootstrapDatabase fails naming both databases. pgtest runs it again for every cloned
// database, because a template does not pass on database-level grants.
func BootstrapDatabase(ctx context.Context, super *pgx.Conn, database string) error {
	var current string
	if err := super.QueryRow(ctx, "SELECT current_database()").Scan(&current); err != nil {
		return fmt.Errorf("pg: bootstrap database %s: %w", database, err)
	}
	if current != database {
		return fmt.Errorf("pg: bootstrap: connected to database %q, not to %q", current, database)
	}
	db := pgx.Identifier{database}.Sanitize()
	stmts := []string{
		"ALTER DATABASE " + db + " OWNER TO " + RoleOwner.identifier(),
		"REVOKE ALL ON DATABASE " + db + " FROM PUBLIC",
		"GRANT CONNECT ON DATABASE " + db + " TO " + roleList(RoleApp, RoleMaint, RoleMonitor),
		"ALTER SCHEMA public OWNER TO " + RoleOwner.identifier(),
		"REVOKE ALL ON SCHEMA public FROM PUBLIC",
		"GRANT USAGE ON SCHEMA public TO " + roleList(RoleApp, RoleMaint),
	}
	err := pgx.BeginFunc(ctx, super, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, "SELECT pg_advisory_xact_lock($1)", bootstrapLockID); err != nil {
			return err
		}
		for _, s := range stmts {
			if _, err := tx.Exec(ctx, s, pgx.QueryExecModeSimpleProtocol); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("pg: bootstrap database %s: %w", database, err)
	}
	return nil
}

func roleList(roles ...Role) string {
	names := make([]string, 0, len(roles))
	for _, r := range roles {
		names = append(names, r.identifier())
	}
	return strings.Join(names, ", ")
}
