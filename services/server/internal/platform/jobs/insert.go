// SPDX-License-Identifier: Apache-2.0

package jobs

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"time"

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
	if args == nil {
		return errs.New(errs.CodeInternalError, "jobs: InsertTx without arguments")
	}
	kind := args.Kind()
	if p := txn.CurrentPhase(ctx); !insertPhase(p) {
		return errs.New(errs.CodeInternalError, fmt.Sprintf("jobs: InsertTx of %s in phase %s; jobs are inserted in the work phase of a unit of work", kind, p))
	}
	tx, err := pg.Tx(ctx)
	if err != nil {
		return err
	}
	e, ok := i.rt.kinds[kind]
	if !ok {
		return errs.New(errs.CodeInternalError, fmt.Sprintf("jobs: InsertTx of the unregistered kind %s", kind))
	}
	if t := reflect.TypeOf(args); t != e.typ {
		return errs.New(errs.CodeInternalError, fmt.Sprintf("jobs: InsertTx of %s with arguments of type %s, registered is %s", kind, t, e.typ))
	}
	if err := roundTrip(args, e.typ); err != nil {
		return errs.Wrap(errs.CodeInternalError, err, fmt.Sprintf("jobs: InsertTx of %s with arguments that cannot be read back", kind))
	}
	var at time.Time
	if opts != nil {
		at = opts.ScheduledAt
	}
	if _, err := i.rt.client.InsertTx(ctx, tx, args, e.insertOpts(at)); err != nil {
		return fmt.Errorf("jobs: insert %s: %w", kind, err)
	}
	return nil
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
