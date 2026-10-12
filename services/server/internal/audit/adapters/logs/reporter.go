// SPDX-License-Identifier: Apache-2.0

// Package logs implements app.Reporter of the audit module with the server logger: constant
// messages, keys only from the attribute catalogue of platform/otel/attr, never an audit row, an
// error text, an IP address or a person (docs/observability/signals.md "Logs", 17 rule 6,
// SEC-LOG-4). The record carries the correlation ID of ctx through the handler of platform/otel.
//
// Example:
//
//	rec, err := app.NewRecorder(cfg, app.Deps{..., Reporter: logs.New(logger)})
package logs

import (
	"context"
	"errors"
	"log/slog"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/OWNER/bothy/services/server/internal/audit/app"
	"github.com/OWNER/bothy/services/server/internal/platform/otel/attr"
)

// The messages of the reports.
const (
	msgChainBroken    = "audit chain broken"
	msgUnknownReason  = "audit reason not in the closed list"
	msgDenialsDropped = "aggregated audit denials dropped"
	msgWriteFailed    = "write of aggregated audit denials failed, retrying"
	msgIPKeyFailed    = "audit IP pseudonym key unavailable, denial kept without IP fields"
)

// The values of error.type of the reports.
const (
	errChainBroken    = "audit_chain_broken"
	errUnknownReason  = "audit_unknown_reason"
	errDenialsDropped = "audit_denials_dropped"
	errWriteFailed    = "audit_write_failed"
	errIPKeyFailed    = "audit_ip_key_failed"
)

// Reporter writes the reports of the audit module as log lines. The count of dropped denials and
// the sequence number of a missing row are not logged: the attribute catalogue has no key for
// them (docs/DECISIONS.md M1-24).
type Reporter struct {
	log *slog.Logger
}

// Compile-time check: Reporter is the outgoing port of app.
var _ app.Reporter = (*Reporter)(nil)

// New returns the reporter over log.
func New(log *slog.Logger) *Reporter { return &Reporter{log: log} }

// ChainBroken logs an error: the predecessor of a new row is missing; the unit failed.
func (r *Reporter) ChainBroken(ctx context.Context, _ int64) {
	r.log.ErrorContext(ctx, msgChainBroken, slog.String(attr.ErrorType, errChainBroken))
}

// UnknownReason logs an error with action and reason; the entry was written (R-DM-28).
func (r *Reporter) UnknownReason(ctx context.Context, action, reason string) {
	r.log.ErrorContext(ctx, msgUnknownReason, slog.String(attr.ErrorType, errUnknownReason),
		slog.String(attr.Action, action), slog.String(attr.Reason, reason))
}

// DenialsDropped logs an error with action and reason of the dropped aggregation.
func (r *Reporter) DenialsDropped(ctx context.Context, action, reason string, _ int64) {
	r.log.ErrorContext(ctx, msgDenialsDropped, slog.String(attr.ErrorType, errDenialsDropped),
		slog.String(attr.Action, action), slog.String(attr.Reason, reason))
}

// WriteFailed logs a warning with the SQLSTATE of err, if any; the buckets stay for the next
// attempt.
func (r *Reporter) WriteFailed(ctx context.Context, err error) {
	r.log.WarnContext(ctx, msgWriteFailed, withSQLState(err, slog.String(attr.ErrorType, errWriteFailed))...)
}

// IPKeyFailed logs an error with the SQLSTATE of err, if any; the denial is aggregated without IP
// fields.
func (r *Reporter) IPKeyFailed(ctx context.Context, err error) {
	r.log.ErrorContext(ctx, msgIPKeyFailed, withSQLState(err, slog.String(attr.ErrorType, errIPKeyFailed))...)
}

// withSQLState appends db.response.status_code when err carries a PostgreSQL error; the error
// text itself never goes into the log (it may hold values).
func withSQLState(err error, attrs ...any) []any {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		attrs = append(attrs, slog.String(attr.DBResponseStatusCode, pgErr.Code))
	}
	return attrs
}
