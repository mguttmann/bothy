// SPDX-License-Identifier: Apache-2.0

package pg

import (
	"context"
	"database/sql"
	"errors"
	"regexp"

	"github.com/exaring/otelpgx"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.opentelemetry.io/otel"
)

// The providers the tracer of new pools uses: the global ones, so that the telemetry setup
// (M1-10) reaches every pool. Tests swap them (export_test.go).
var (
	tracerProvider = otel.GetTracerProvider
	meterProvider  = otel.GetMeterProvider
)

// newTracer returns the tracer of a connection: otelpgx without query parameters (the option
// WithIncludeQueryParameters stays off), so spans carry db.query.text with placeholders only
// (docs/observability/signals.md "Spans"), wrapped in redactingTracer. The operation name is
// added to the metric db.client.operation.duration, as the metric catalogue lists it.
func newTracer() *redactingTracer {
	return &redactingTracer{Tracer: otelpgx.NewTracer(
		otelpgx.WithTracerProvider(tracerProvider()),
		otelpgx.WithMeterProvider(meterProvider()),
		otelpgx.WithMetricOperationName(otelpgx.SQLOperationName),
	)}
}

// redactingTracer passes every event to otelpgx but removes password literals from the statement
// text and replaces the error of a statement before otelpgx records them. otelpgx writes err.Error() into the span status and an exception event,
// and the message of a PostgreSQL error can quote a parameter (invalid input syntax for type
// integer: "<value>"), so a failed statement would carry the value the parameter option keeps
// out. The span keeps the SQLSTATE (pgx.sql_state); the full error stays with the caller and its
// log. Connect and acquire errors carry no statement data and pass unchanged.
type redactingTracer struct {
	*otelpgx.Tracer
}

// errStatementFailed replaces an error of a statement that is not a PostgreSQL error (an encode
// error can quote the value as well).
var errStatementFailed = errors.New("statement failed before the database answered")

// redactStatementError returns err without any text that could quote a value: a PostgreSQL
// error keeps severity and SQLSTATE, context errors and sql.ErrNoRows stay as they are (otelpgx
// does not record the latter), anything else becomes errStatementFailed.
func redactStatementError(err error) error {
	if err == nil || errors.Is(err, sql.ErrNoRows) || errors.Is(err, context.Canceled) ||
		errors.Is(err, context.DeadlineExceeded) {
		return err
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return &pgconn.PgError{
			Severity: pgErr.Severity, SeverityUnlocalized: pgErr.SeverityUnlocalized,
			Code: pgErr.Code, Message: "statement failed",
		}
	}
	return errStatementFailed
}

// passwordLiteral finds the password of CREATE ROLE and ALTER ROLE (pg.Bootstrap sends the
// SCRAM verifier as a literal, which DDL requires).
var passwordLiteral = regexp.MustCompile(`(?i)(\bPASSWORD\s+)'(?:[^']|'')*'`)

// redactStatement removes password literals from sql before otelpgx records it as
// db.query.text: a SCRAM verifier is enough for an offline guess of the password.
func redactStatement(sql string) string {
	return passwordLiteral.ReplaceAllString(sql, "${1}'[redacted]'")
}

// TraceQueryStart starts the query span with password literals redacted.
func (t *redactingTracer) TraceQueryStart(ctx context.Context, conn *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
	data.SQL = redactStatement(data.SQL)
	return t.Tracer.TraceQueryStart(ctx, conn, data)
}

// TracePrepareStart starts the prepare span with password literals redacted.
func (t *redactingTracer) TracePrepareStart(ctx context.Context, conn *pgx.Conn, data pgx.TracePrepareStartData) context.Context {
	data.SQL = redactStatement(data.SQL)
	return t.Tracer.TracePrepareStart(ctx, conn, data)
}

// TraceQueryEnd ends the query span with the error redacted.
func (t *redactingTracer) TraceQueryEnd(ctx context.Context, conn *pgx.Conn, data pgx.TraceQueryEndData) {
	data.Err = redactStatementError(data.Err)
	t.Tracer.TraceQueryEnd(ctx, conn, data)
}

// TraceBatchQuery records one query of a batch with its error redacted.
func (t *redactingTracer) TraceBatchQuery(ctx context.Context, conn *pgx.Conn, data pgx.TraceBatchQueryData) {
	data.SQL = redactStatement(data.SQL)
	data.Err = redactStatementError(data.Err)
	t.Tracer.TraceBatchQuery(ctx, conn, data)
}

// TraceBatchEnd ends the batch span with the error redacted.
func (t *redactingTracer) TraceBatchEnd(ctx context.Context, conn *pgx.Conn, data pgx.TraceBatchEndData) {
	data.Err = redactStatementError(data.Err)
	t.Tracer.TraceBatchEnd(ctx, conn, data)
}

// TraceCopyFromEnd ends the copy span with the error redacted.
func (t *redactingTracer) TraceCopyFromEnd(ctx context.Context, conn *pgx.Conn, data pgx.TraceCopyFromEndData) {
	data.Err = redactStatementError(data.Err)
	t.Tracer.TraceCopyFromEnd(ctx, conn, data)
}

// TracePrepareEnd ends the prepare span with the error redacted.
func (t *redactingTracer) TracePrepareEnd(ctx context.Context, conn *pgx.Conn, data pgx.TracePrepareEndData) {
	data.Err = redactStatementError(data.Err)
	t.Tracer.TracePrepareEnd(ctx, conn, data)
}

// The interfaces pgx and pgxpool look for on ConnConfig.Tracer.
var (
	_ pgx.QueryTracer       = (*redactingTracer)(nil)
	_ pgx.BatchTracer       = (*redactingTracer)(nil)
	_ pgx.CopyFromTracer    = (*redactingTracer)(nil)
	_ pgx.PrepareTracer     = (*redactingTracer)(nil)
	_ pgx.ConnectTracer     = (*redactingTracer)(nil)
	_ pgxpool.AcquireTracer = (*redactingTracer)(nil)
)
