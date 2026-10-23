// SPDX-License-Identifier: Apache-2.0

package jobs

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
	"github.com/OWNER/bothy/services/server/internal/platform/pg"
)

// inserter is the Inserter of a Runtime.
type inserter struct{ rt *Runtime }

// insertPhase reports whether a job may be inserted in phase p: in the work phase and the two
// rights phases, where the use case and authz write domain rows. In sync and audit the unit
// only numbers events and audits what happened, and outside a unit there is no transaction.
func insertPhase(p txn.Phase) bool {
	switch p {
	case txn.PhaseRightsBegin, txn.PhaseWork, txn.PhaseRightsCheck:
		return true
	case txn.PhaseNone, txn.PhaseSync, txn.PhaseAudit:
		return false
	default:
		return false
	}
}

// InsertTx implements Inserter.
func (i inserter) InsertTx(ctx context.Context, args Args, opts *InsertOptions) error {
	tx, e, err := i.prepare(ctx, "InsertTx", args)
	if err != nil {
		return err
	}
	if _, err := i.rt.client.InsertTx(ctx, tx, args, e.insertOpts(scheduledAt(opts))); err != nil {
		return fmt.Errorf("jobs: insert %s: %w", args.Kind(), err)
	}
	return nil
}

// prepare runs the checks every operation of the inserter shares and returns the ambient
// transaction and the registered entry of args: op is the operation name for the error texts.
// The errors are those Inserter.InsertTx documents, all with code internal_error.
func (i inserter) prepare(ctx context.Context, op string, args Args) (pgx.Tx, *entry, error) {
	if args == nil {
		return nil, nil, errs.New(errs.CodeInternalError, "jobs: "+op+" without arguments")
	}
	kind := args.Kind()
	if p := txn.CurrentPhase(ctx); !insertPhase(p) {
		return nil, nil, errs.New(errs.CodeInternalError, fmt.Sprintf("jobs: %s of %s in phase %s; jobs are inserted in the work phase of a unit of work", op, kind, p))
	}
	tx, err := pg.Tx(ctx)
	if err != nil {
		return nil, nil, err
	}
	e, ok := i.rt.kinds[kind]
	if !ok {
		return nil, nil, errs.New(errs.CodeInternalError, fmt.Sprintf("jobs: %s of the unregistered kind %s", op, kind))
	}
	if t := reflect.TypeOf(args); t != e.typ {
		return nil, nil, errs.New(errs.CodeInternalError, fmt.Sprintf("jobs: %s of %s with arguments of type %s, registered is %s", op, kind, t, e.typ))
	}
	if err := roundTrip(args, e.typ); err != nil {
		return nil, nil, errs.Wrap(errs.CodeInternalError, err, fmt.Sprintf("jobs: %s of %s with arguments that cannot be read back", op, kind))
	}
	return tx, e, nil
}

// scheduledAt returns the time of opts; the zero time (now) for nil.
func scheduledAt(opts *InsertOptions) time.Time {
	if opts == nil {
		return time.Time{}
	}
	return opts.ScheduledAt
}

// roundTrip encodes args as River stores them and decodes them into a fresh value of t, as the
// worker will. Arguments that fail here (a zero ids.ID is no valid UUIDv7) would fail every
// attempt of the job; the insert refuses them inside the unit of work of the caller instead.
func roundTrip(args Args, t reflect.Type) error {
	b, err := json.Marshal(args)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, reflect.New(t).Interface())
}
