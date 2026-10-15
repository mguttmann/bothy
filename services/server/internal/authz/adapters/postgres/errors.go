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

// Field pointers that several constraints share.
const (
	pointerSubject      = "/subject"
	pointerTarget       = "/target"
	pointerCapabilities = "/capabilities"
)

// uniqueFields maps the unique indexes of the migration to the field a conflict names. A
// primary key conflict means a reused id, which only a programming error produces, so it is not
// in the list and stays internal.
var uniqueFields = map[string]string{
	"groups_live_name_key":         "/name",
	"groups_claim_key":             "/external_ref",
	"group_memberships_active_key": "/user_id",
}

// constraintFields maps checks, foreign keys and the immutability trigger to the field they
// guard. The subject and target checks name the whole subject or target, as domain.NewGrant
// does; the bot limits name the subject, because its kind is what the limits depend on.
var constraintFields = map[string]string{
	"groups_source_check":                "/source",
	"groups_provider_shape_check":        "/provider_id",
	"groups_external_ref_shape_check":    "/external_ref",
	"groups_provider_id_fkey":            "/provider_id",
	"groups_created_by_fkey":             "/created_by",
	"group_memberships_group_id_fkey":    "/group_id",
	"group_memberships_user_id_fkey":     "/user_id",
	"group_memberships_added_by_fkey":    "/added_by",
	"group_memberships_removed_by_fkey":  "/removed_by",
	"grants_subject_kind_check":          pointerSubject,
	"grants_subject_shape_check":         pointerSubject,
	"grants_subject_principal_id_fkey":   pointerSubject,
	"grants_subject_group_id_fkey":       pointerSubject,
	"grants_bot_limits_check":            pointerSubject,
	"grants_target_kind_check":           pointerTarget,
	"grants_target_shape_check":          pointerTarget,
	"grants_level_check":                 "/level",
	"grants_capabilities_check":          pointerCapabilities,
	"grants_host_shape_check":            pointerCapabilities,
	"grants_terminal_only_on_host_check": pointerCapabilities,
	"grants_granted_by_fkey":             "/granted_by",
	"grants_revoked_by_fkey":             "/revoked_by",
	"authz_grants_immutable":             "/id",
}

// mapError turns a database error of the operation what into a kernel/errs error (brief M1-27
// "Error mapping"): a unique violation becomes kind conflict with the field, a check, trigger,
// foreign key or not null violation becomes validation_failed with the field of the constraint
// or column. The text never contains SQL, constraint names or values; the driver error stays
// wrapped for the log only. An error that already is a kernel/errs error (for example the
// unavailable error of an exhausted pool) passes unchanged; anything else is internal.
func mapError(err error, what string) error {
	if _, ok := errs.As(err); ok {
		return err
	}
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return errs.Wrap(errs.CodeInternalError, err, "authz: "+what+" failed")
	}
	switch pgErr.Code {
	case sqlstateUniqueViolation:
		if field, ok := uniqueFields[pgErr.ConstraintName]; ok {
			return errs.Wrap(errs.CodeConflict, err, "the value of "+field[1:]+" is already taken").
				WithFields(errs.FieldError{Pointer: field, Code: fieldTaken, Detail: "already taken"})
		}
	case sqlstateCheckViolation, sqlstateForeignKeyViolation, sqlstateNotNullViolation:
		field, ok := constraintFields[pgErr.ConstraintName]
		if !ok && pgErr.ColumnName != "" {
			field, ok = "/"+pgErr.ColumnName, true
		}
		if ok {
			return errs.Wrap(errs.CodeValidationFailed, err, "the value of "+field[1:]+" is not allowed").
				WithFields(errs.FieldError{Pointer: field, Code: fieldInvalid, Detail: "not allowed"})
		}
	}
	return errs.Wrap(errs.CodeInternalError, err, "authz: "+what+" failed")
}
