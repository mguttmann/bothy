// SPDX-License-Identifier: Apache-2.0

package postgres

import (
	"bytes"
	"context"
	"encoding/json"

	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/eventlog"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/pg"
	"github.com/OWNER/bothy/services/server/internal/sync/adapters/postgres/sqlcgen"
)

// Redactor implements the kernel port eventlog.Redactor over event_log (M1-58, 03 section 6.5,
// R-DM-18): it locks the stored events of one resource FOR UPDATE in seq order, through the index
// (resource_kind, resource_id), and writes the payloads fn returns in one statement. bothy_app
// holds the column right UPDATE (payload, redacted_at) only (migration 00037), so the envelope
// cannot change. It is safe for concurrent use.
//
// Example:
//
//	r := postgres.NewRedactor(pool)
//	n, err := r.Redact(ctx, "message", id, fn) // inside the unit of work of the deletion
type Redactor struct {
	pool *pg.Pool
}

// Compile-time check: the kernel port.
var _ eventlog.Redactor = (*Redactor)(nil)

// NewRedactor returns the redactor over pool; it opens no connection.
func NewRedactor(pool *pg.Pool) *Redactor { return &Redactor{pool: pool} }

// Redact implements eventlog.Redactor. Errors: internal outside a unit of work and for a result
// of fn that is not a JSON object, the error of fn unchanged, internal for a database error
// (never with a payload in the message).
func (r *Redactor) Redact(ctx context.Context, kind string, id ids.ID, fn eventlog.RedactFunc) (int, error) {
	tx, err := pg.Tx(ctx)
	if err != nil {
		return 0, err
	}
	q := sqlcgen.New(tx)
	rows, err := q.LockResourceEvents(ctx, sqlcgen.LockResourceEventsParams{ResourceKind: kind, ResourceID: uuid(id)})
	if err != nil {
		return 0, errs.Wrap(errs.CodeInternalError, err, "sync: locking the events of a resource failed")
	}
	if len(rows) == 0 {
		return 0, nil
	}
	p := sqlcgen.RedactEventsParams{Seqs: make([]int64, 0, len(rows)), Payloads: make([]string, 0, len(rows))}
	for _, row := range rows {
		out, err := fn(row.Type, int(row.SchemaVersion), json.RawMessage(row.Payload))
		if err != nil {
			return 0, err
		}
		if !isObject(out) {
			return 0, errs.New(errs.CodeInternalError, "sync: a redacted payload is not a JSON object")
		}
		p.Seqs = append(p.Seqs, row.Seq)
		p.Payloads = append(p.Payloads, string(out))
	}
	n, err := q.RedactEvents(ctx, p)
	if err != nil {
		return 0, errs.Wrap(errs.CodeInternalError, err, "sync: writing redacted payloads failed")
	}
	return int(n), nil
}

// isObject reports whether b is one valid JSON object.
func isObject(b json.RawMessage) bool {
	t := bytes.TrimSpace(b)
	return len(t) > 0 && t[0] == '{' && json.Valid(t)
}
