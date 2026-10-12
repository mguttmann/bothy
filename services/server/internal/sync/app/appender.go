// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"errors"
	"slices"
	stdsync "sync"

	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/eventlog"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
	"github.com/OWNER/bothy/services/server/internal/sync/domain"
)

// pending is the per-unit buffer of durable events: the appender fills it, the sync hook writes
// it. kernel/txn drops it with a rollback and starts every attempt empty.
var pending = txn.NewKey[[]PendingEvent]("durable events")

// pendingMu makes read and write of the buffer one step, so subscribers that append from two
// goroutines of the same unit never lose an event.
var pendingMu stdsync.Mutex

// AppenderConfig is the configuration of the appender.
type AppenderConfig struct {
	// PayloadMaxBytes is sync.event_payload_max_bytes: a payload must be smaller.
	PayloadMaxBytes int `config:"sync.event_payload_max_bytes"`
}

// Appender implements kernel/eventlog.Appender: it validates durable events and buffers them in
// the unit of work. It is safe for concurrent use.
type Appender struct {
	maxBytes int
	corr     Correlator
}

// Compile-time check: Appender is the kernel port.
var _ eventlog.Appender = (*Appender)(nil)

// NewAppender returns the appender with cfg and the correlation source corr. The error reports
// a payload limit below 1 or a nil corr.
func NewAppender(cfg AppenderConfig, corr Correlator) (*Appender, error) {
	if cfg.PayloadMaxBytes < 1 {
		return nil, errors.New("sync: sync.event_payload_max_bytes must be at least 1")
	}
	if corr == nil {
		return nil, errors.New("sync: the appender needs a correlation source")
	}
	return &Appender{maxBytes: cfg.PayloadMaxBytes, corr: corr}, nil
}

// Append implements eventlog.Appender. It checks the unit and the phase (work or rights_check,
// 02 "Flow": durable events arise only in phase 2 and 3.1), validates every event with
// domain.ValidateEvent, and only when all pass gives each event without ID a UUIDv7, attaches the
// correlation of ctx and appends them to the buffer of the unit. Errors are kind internal; a
// rejected call buffers nothing.
func (a *Appender) Append(ctx context.Context, events ...eventlog.Event) error {
	if !txn.InUnit(ctx) {
		return errs.New(errs.CodeInternalError, "sync: Append outside a unit of work")
	}
	if p := txn.CurrentPhase(ctx); p != txn.PhaseWork && p != txn.PhaseRightsCheck {
		return errs.New(errs.CodeInternalError, "sync: Append in phase "+p.String()+"; durable events arise only in the work and rights_check phases")
	}
	for _, e := range events {
		if err := domain.ValidateEvent(fields(e), a.maxBytes); err != nil {
			return err
		}
	}
	if len(events) == 0 {
		return nil
	}
	corr, trace := a.corr.Correlation(ctx)
	rows := make([]PendingEvent, len(events))
	for i, e := range events {
		if e.ID.IsZero() {
			e.ID = ids.New()
		}
		e.Payload = slices.Clone(e.Payload)
		rows[i] = PendingEvent{Event: e, CorrelationID: corr, TraceContext: trace}
	}
	pendingMu.Lock()
	defer pendingMu.Unlock()
	have, _ := txn.Load(ctx, pending)
	return txn.Store(ctx, pending, append(slices.Clip(have), rows...))
}

// fields maps an event to the fields the domain rules check.
func fields(e eventlog.Event) domain.EventFields {
	return domain.EventFields{
		Type: e.Type, SchemaVersion: e.SchemaVersion, ActorKind: string(e.ActorKind),
		HasActorPrincipal: !e.ActorPrincipalID.IsZero(), ResourceKind: e.ResourceKind,
		ReceiveAction: e.ReceiveAction, HasAudience: !e.AudiencePrincipalID.IsZero(), Payload: e.Payload,
	}
}
