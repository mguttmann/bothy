// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"errors"
	"net/netip"
	"time"

	"github.com/OWNER/bothy/services/server/internal/audit/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/auditing"
	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/reqmeta"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
)

// Deps are the collaborators of the recorder.
type Deps struct {
	// UoW is the unit of work whose registry holds Chain.Hook in the audit phase.
	UoW txn.UnitOfWork
	// Store is the storage of the module.
	Store Store
	// KEK resolves privacy.audit_ip_kek_ref.
	KEK KEKSource
	// Clock is the source of occurred_at and of the aggregation windows; nil means
	// clock.System().
	Clock clock.Clock
	// Reporter receives the reports of the module; required.
	Reporter Reporter
}

// Recorder implements auditing.Recorder (ADR 0021). It is safe for concurrent use.
type Recorder struct {
	uow      txn.UnitOfWork
	clock    clock.Clock
	reporter Reporter
	keyring  *Keyring
	agg      *Aggregator
}

// Compile-time check: Recorder is the kernel port.
var _ auditing.Recorder = (*Recorder)(nil)

// NewRecorder returns the recorder for cfg with its keyring and aggregator. It does no I/O; the
// caller runs Aggregator().Run with the lifecycle of the process and calls Aggregator().Flush at
// shutdown, before the pool closes.
//
// Errors: a nil UoW, Store, KEK or Reporter, a window, a key cap or a key period that is not
// positive (the configuration schema already refuses them).
func NewRecorder(cfg Config, d Deps) (*Recorder, error) {
	if d.UoW == nil || d.Store == nil || d.KEK == nil || d.Reporter == nil {
		return nil, errors.New("audit: the recorder needs a unit of work, a store, a KEK source and a reporter")
	}
	if cfg.DenyAggregateWindow <= 0 || cfg.DenyAggregateMaxKeys <= 0 || cfg.IPKeyPeriod < time.Millisecond {
		return nil, errors.New("audit: audit.deny_aggregate_window, audit.deny_aggregate_max_keys and privacy.audit_ip_key_period must be positive")
	}
	if d.Clock == nil {
		d.Clock = clock.System()
	}
	r := &Recorder{
		uow: d.UoW, clock: d.Clock, reporter: d.Reporter,
		keyring: NewKeyring(d.Store, d.UoW, d.KEK, cfg.IPKEKRef, cfg.IPKeyPeriod),
	}
	r.agg = newAggregator(cfg.DenyAggregateWindow, cfg.DenyAggregateMaxKeys, d.Clock, r.writeRows, d.Reporter)
	return r, nil
}

// Aggregator returns the aggregator of the denials of RecordDenial.
func (r *Recorder) Aggregator() *Aggregator { return r.agg }

// Keyring returns the keyring of the IP pseudonyms.
func (r *Recorder) Keyring() *Keyring { return r.keyring }

// Record adds e to the unit of work of ctx; Chain.Hook writes it before the commit
// (auditing.Recorder.Record).
func (r *Recorder) Record(ctx context.Context, e auditing.Entry) error {
	if !txn.InUnit(ctx) {
		return errs.New(errs.CodeInternalError, "audit: Record outside a unit of work")
	}
	if p := txn.CurrentPhase(ctx); p == txn.PhaseAudit || p == txn.PhaseNone {
		return errs.New(errs.CodeInternalError, "audit: Record in phase "+p.String()+"; the audit hook already ran")
	}
	row, err := r.row(ctx, e)
	if err != nil {
		return err
	}
	return internalError(appendPending(ctx, row))
}

// RecordDetached writes e at once in an own unit of work (auditing.Recorder.RecordDetached).
func (r *Recorder) RecordDetached(ctx context.Context, e auditing.Entry) error {
	row, err := r.row(ctx, e)
	if err != nil {
		return err
	}
	return r.writeRows(ctx, []domain.Row{row})
}

// RecordDenial aggregates a denial (auditing.Recorder.RecordDenial). A failure of the IP key is
// reported and the denial is aggregated without IP fields; it never returns a storage error.
func (r *Recorder) RecordDenial(ctx context.Context, e auditing.Entry) error {
	if e.Decision != auditing.DecisionDeny {
		return internalError(invalid("RecordDenial takes only denials"))
	}
	now := r.clock.Now()
	row, err := buildRow(ctx, e, now)
	if err != nil {
		return internalError(err)
	}
	if ip, ok := reqmeta.ClientIP(ctx); ok {
		if err := r.addIP(ctx, &row, ip, now); err != nil {
			r.reporter.IPKeyFailed(ctx, err)
		}
	}
	r.reportUnknown(ctx, e)
	key := domain.AggregateKey{Principal: row.ActorPrincipalID, Action: row.Action, Reason: row.DenyReason}
	if key.Principal.IsZero() {
		key.IPHMAC, _ = row.Details[domain.DetailIPHMAC].(string)
	}
	r.agg.add(ctx, key, row.OccurredAt, row)
	return nil
}

// row builds and checks the row of e with the IP fields of ctx.
func (r *Recorder) row(ctx context.Context, e auditing.Entry) (domain.Row, error) {
	now := r.clock.Now()
	row, err := buildRow(ctx, e, now)
	if err != nil {
		return domain.Row{}, internalError(err)
	}
	if ip, ok := reqmeta.ClientIP(ctx); ok {
		if err := r.addIP(ctx, &row, ip, now); err != nil {
			return domain.Row{}, err
		}
	}
	r.reportUnknown(ctx, e)
	return row, nil
}

// addIP sets ip_hmac and ip_key_id of row from the key of the period of now.
func (r *Recorder) addIP(ctx context.Context, row *domain.Row, ip netip.Addr, now time.Time) error {
	id, key, err := r.keyring.Current(ctx, now)
	if err != nil {
		return err
	}
	row.Details[domain.DetailIPHMAC] = domain.IPHMAC(key, ip)
	row.Details[domain.DetailIPKeyID] = id.String()
	return nil
}

// reportUnknown reports a reason outside 07 "4.1"; the entry is written anyway (R-DM-28).
func (r *Recorder) reportUnknown(ctx context.Context, e auditing.Entry) {
	if e.Reason != "" && !e.Reason.Known() {
		r.reporter.UnknownReason(ctx, e.Action, string(e.Reason))
	}
}

// writeRows writes rows in an own unit of work that never joins an ambient one: the audit hook
// numbers and chains them before its commit.
func (r *Recorder) writeRows(ctx context.Context, rows []domain.Row) error {
	return r.uow.Do(txn.Detach(ctx), func(ctx context.Context) error { return appendPending(ctx, rows...) })
}
