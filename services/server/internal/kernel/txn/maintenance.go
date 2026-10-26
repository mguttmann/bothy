// SPDX-License-Identifier: Apache-2.0

package txn

import "context"

// writeKey is the context key of the maintenance exemption.
type writeKey struct{}

// WithWriteInMaintenance returns a copy of ctx whose units of work may write while the server
// runs in maintenance mode (server.maintenance_mode = read_only, ADR 0065 item 5). Without it,
// platform/pg opens every outermost transaction READ ONLY in that mode, so PostgreSQL rejects an
// overlooked write with SQLSTATE 25006. Only the exceptions of ADR 0065 item 4 set it: the HTTP
// stage of maintenance mode for a request whose action is an exception, the gateway for the
// fencing column of an agent consumer, and the worker for the consumers push and webhooks, the
// queue delivery and the job runs.cancel_timeout. Outside maintenance mode it changes nothing.
// The mark survives Detach, so a denial written after a rollback stays exempt too.
//
// Example:
//
//	err := uow.Do(txn.WithWriteInMaintenance(ctx), func(ctx context.Context) error { ... })
func WithWriteInMaintenance(ctx context.Context) context.Context {
	return context.WithValue(ctx, writeKey{}, true)
}

// WritesInMaintenance reports whether ctx carries the exemption of WithWriteInMaintenance.
func WritesInMaintenance(ctx context.Context) bool {
	ok, _ := ctx.Value(writeKey{}).(bool)
	return ok
}

// ExemptInMaintenance returns a unit of work over uow whose every Do runs with
// WithWriteInMaintenance: the unit of work of a worker part that ADR 0065 item 4 lets write in
// maintenance mode as a whole (the consumers push and webhooks of the event log, which
// internal/wire builds with it).
//
// Example:
//
//	consumers, err := syncapp.NewConsumerService(cfg, txn.ExemptInMaintenance(uow), repo, repo, log)
func ExemptInMaintenance(uow UnitOfWork) UnitOfWork { return exemptUnit{inner: uow} }

// exemptUnit is the unit of work of ExemptInMaintenance.
type exemptUnit struct{ inner UnitOfWork }

// Do runs inner.Do with the exemption.
func (e exemptUnit) Do(ctx context.Context, fn func(ctx context.Context) error, opts ...Option) error {
	return e.inner.Do(WithWriteInMaintenance(ctx), fn, opts...)
}
