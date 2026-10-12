// SPDX-License-Identifier: Apache-2.0

package txn

import (
	"context"
	"reflect"

	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
)

// spanPrefix starts the name of a subscriber span (docs/observability/signals.md "Spans").
const spanPrefix = "txn.subscribe "

// Publish delivers ev synchronously to the subscribers of type E in registration order, inside
// the transaction of ctx (02 "In-Tx events"). A subscriber may write, append durable events
// and publish further events; a nested Publish runs in the same phase, and its depth is bounded
// only by the call stack. Each delivery gets its own span "txn.subscribe <module>" when the
// registry has a tracer.
//
// Errors: kind internal outside a unit, with an unsealed registry, or in a phase other than
// PhaseWork and PhaseRightsCheck; otherwise the first subscriber error unchanged (veto), and the
// later subscribers do not run.
func Publish[E any](ctx context.Context, ev E) error {
	u, ok := UnitFrom(ctx)
	if !ok {
		return errs.New(errs.CodeInternalError, "txn: Publish outside a unit of work")
	}
	if p := u.Phase(); !eventPhase(p) {
		return errs.New(errs.CodeInternalError, "txn: Publish in phase "+p.String()+"; events arise only in the work and rights_check phases")
	}
	subs, tracer, sealed := u.Registry().subscribers(reflect.TypeFor[E]())
	if !sealed {
		return errs.New(errs.CodeInternalError, "txn: Publish with an unsealed registry")
	}
	for _, s := range subs {
		if err := deliver(ctx, tracer, s, ev); err != nil {
			return err
		}
	}
	return nil
}

// deliver runs one subscriber, inside its span when tracer is set.
func deliver(ctx context.Context, tracer Tracer, s subscriber, ev any) error {
	if tracer == nil {
		return s.deliver(ctx, ev)
	}
	sctx, end := tracer(ctx, spanPrefix+s.module)
	err := s.deliver(sctx, ev)
	end(err)
	return err
}
