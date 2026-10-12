// SPDX-License-Identifier: Apache-2.0

package postgres

import (
	"errors"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
)

// The SQLSTATEs the adapter maps to client errors.
const (
	sqlstateUniqueViolation     = "23505"
	sqlstateCheckViolation      = "23514"
	sqlstateForeignKeyViolation = "23503"
	sqlstateNotNullViolation    = "23502"
)

// Field error codes of a mapped violation.
const (
	fieldTaken   = "taken"
	fieldInvalid = "invalid"
)

// uniqueFields maps the unique indexes and primary keys of the migration to the field a
// conflict names. A primary key conflict means a reused id, which only a programming error
// produces, so it is not in the list and stays internal.
var uniqueFields = map[string]string{
	"principals_handle_lower_key": "/handle",
	"users_email_lower_key":       "/email",
	"owners_active_user_key":      "/user_id",
	"users_pkey":                  "/principal_id",
}

// constraintFields maps checks and foreign keys to the column they guard. Triggers raise with
// the column set (COLUMN in RAISE), so they need no entry.
var constraintFields = map[string]string{
	"principals_kind_check":   "/kind",
	"principals_status_check": "/status",
	"principals_handle_check": "/handle",
	"users_principal_id_fkey": "/principal_id",
	"owners_user_id_fkey":     "/user_id",
	"owners_granted_by_fkey":  "/granted_by",
	"owners_revoked_by_fkey":  "/revoked_by",
}

// mapError turns a database error of the operation what into a kernel/errs error (brief M1-26
// "Error mapping"): a unique violation becomes kind conflict with the field, a check, trigger,
// foreign key or not null violation becomes validation_failed with the column as field. The
// text never contains SQL, constraint names or values; the driver error stays wrapped for the
// log only. An error that already is a kernel/errs error (for example the unavailable error of
// an exhausted pool) passes unchanged; anything else is internal.
func mapError(err error, what string) error {
	if _, ok := errs.As(err); ok {
		return err
	}
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return errs.Wrap(errs.CodeInternalError, err, "identity: "+what+" failed")
	}
	switch pgErr.Code {
	case sqlstateUniqueViolation:
		if field, ok := uniqueFields[pgErr.ConstraintName]; ok {
			return errs.Wrap(errs.CodeConflict, err, "the value of "+field[1:]+" is already taken").
				WithFields(errs.FieldError{Pointer: field, Code: fieldTaken, Detail: "already taken"})
		}
	case sqlstateCheckViolation, sqlstateForeignKeyViolation, sqlstateNotNullViolation:
		field := ""
		if pgErr.ColumnName != "" {
			field = "/" + pgErr.ColumnName
		} else {
			field = constraintFields[pgErr.ConstraintName]
		}
		if field != "" {
			return errs.Wrap(errs.CodeValidationFailed, err, "the value of "+field[1:]+" is not allowed").
				WithFields(errs.FieldError{Pointer: field, Code: fieldInvalid, Detail: "not allowed"})
		}
	}
	return errs.Wrap(errs.CodeInternalError, err, "identity: "+what+" failed")
}
