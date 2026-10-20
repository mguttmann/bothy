// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"log/slog"
	"math"
	"slices"

	"github.com/OWNER/bothy/services/server/internal/apps/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/auditing"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
	"github.com/OWNER/bothy/services/server/internal/sync"
)

// Consumer is the handler of the worker consumer webhooks (sync.ConsumerWebhooks, M2-26 design
// note 3): in the round's transaction it creates one pending delivery per matching subscription,
// pauses a subscription that crosses the delivery quota, schedules the delivery jobs and, for a
// range retention removed before the round, disables the subscriptions that could have matched it.
// It calls nothing outside the process and appends no durable event.
type Consumer struct {
	w *WebhookService
}

// Compile-time checks: Consumer is a consumer handler with the loss hook.
var (
	_ sync.ConsumerHandler     = (*Consumer)(nil)
	_ sync.ConsumerLossHandler = (*Consumer)(nil)
)

// Consumer returns the handler of the worker consumer webhooks over w.
func (w *WebhookService) Consumer() *Consumer { return &Consumer{w: w} }

// roundMemo is the state of one consumer round, kept in its unit of work: the candidates loaded
// once, the open count per subscription, the current state and the subscriptions scheduled.
type roundMemo struct {
	candidates []Candidate
	pending    map[ids.ID]int
	state      map[ids.ID]domain.SubscriptionState
	scheduled  map[ids.ID]bool
}

// memoKey names the memo of a round in its unit of work; a retried round starts with a fresh one.
var memoKey = txn.NewKey[*roundMemo]("apps.webhook_round")

// memo returns the memo of the round of ctx, loading the candidates on the first call.
func (c *Consumer) memo(ctx context.Context) (*roundMemo, error) {
	if m, ok := txn.Load(ctx, memoKey); ok {
		return m, nil
	}
	cands, err := c.w.d.Store.Candidates(ctx)
	if err != nil {
		return nil, err
	}
	m := &roundMemo{candidates: cands, pending: map[ids.ID]int{}, state: map[ids.ID]domain.SubscriptionState{}, scheduled: map[ids.ID]bool{}}
	for _, cand := range cands {
		m.state[cand.SubscriptionID] = cand.State
	}
	return m, txn.Store(ctx, memoKey, m)
}

// HandleEvent implements sync.ConsumerHandler (M2-26 design note 3 a to e): skip types apps cannot
// subscribe to; for each candidate whose event types and installation subscriptions name the
// type (a personal event only for the installation of its audience) insert the pending row; pause
// an active subscription whose open count crosses webhook.max_pending_per_subscription; schedule
// one job per active subscription with an active installation that got a row in this round.
// Rights are not checked here: they apply at delivery time.
func (c *Consumer) HandleEvent(ctx context.Context, ev sync.Event) error {
	if !domain.AppSubscribable(ev.Type) {
		return nil
	}
	m, err := c.memo(ctx)
	if err != nil {
		return err
	}
	for _, cand := range m.candidates {
		if !slices.Contains(cand.EventTypes, ev.Type) || !slices.Contains(cand.InstallationEvents, ev.Type) {
			continue
		}
		if !ev.AudiencePrincipalID.IsZero() && ev.AudiencePrincipalID != cand.AppID {
			continue
		}
		if err := c.add(ctx, m, cand, ev); err != nil {
			return err
		}
	}
	return nil
}

// add inserts the row of ev for cand and applies the quota and the job of design note 3 d and e.
func (c *Consumer) add(ctx context.Context, m *roundMemo, cand Candidate, ev sync.Event) error {
	inserted, err := c.w.d.Store.InsertDelivery(ctx, DeliveryRecord{
		ID: ids.New(), SubscriptionID: cand.SubscriptionID, EventSeq: ev.Seq, EventID: ev.ID, CreatedAt: c.w.now(),
	})
	if err != nil || !inserted {
		return err
	}
	limit := c.w.cfg.MaxPendingPerSubscription
	before, ok := m.pending[cand.SubscriptionID]
	if !ok {
		// The count includes the row just inserted; the bound keeps the read short.
		n, err := c.w.d.Store.CountPending(ctx, cand.SubscriptionID, limit+1)
		if err != nil {
			return err
		}
		before = n - 1
	}
	after := min(before+1, limit)
	m.pending[cand.SubscriptionID] = after
	if m.state[cand.SubscriptionID] == domain.SubscriptionActive && before < limit && before+1 >= limit {
		if err := c.w.systemTransition(ctx, cand.SubscriptionID, domain.EventQuota, 0, 0); err != nil {
			return err
		}
		m.state[cand.SubscriptionID] = domain.SubscriptionPaused
	}
	if m.state[cand.SubscriptionID] == domain.SubscriptionActive && cand.InstallationState == domain.StateActive && !m.scheduled[cand.SubscriptionID] {
		if err := c.w.d.Scheduler.Schedule(ctx, cand.SubscriptionID, c.w.now()); err != nil {
			return err
		}
		m.scheduled[cand.SubscriptionID] = true
	}
	return nil
}

// HandleLost implements sync.ConsumerLossHandler (M2-26 design note 3 f): the consumer itself
// fell behind sync.event_retention, so every app subscription in state active or paused whose
// installation is active or suspended becomes disabled with cause gap; its pending rows, all below
// the lost range, become dead with event_expired. The audit entry names from_seq (the lowest seq
// of those rows, else from) and to_seq. A re-enable continues with the events read after it.
func (c *Consumer) HandleLost(ctx context.Context, from, to int64) error {
	subs, err := c.w.d.Store.LockLiveSubscriptions(ctx)
	if err != nil {
		return err
	}
	for _, s := range subs {
		seqs, err := c.w.d.Store.ExpirePending(ctx, s.ID, math.MaxInt64)
		if err != nil {
			return err
		}
		first := from
		if len(seqs) > 0 {
			first = slices.Min(seqs)
		}
		if err := c.w.applyTransition(ctx, &s, domain.EventGap, first, to); err != nil {
			return err
		}
		if err := c.w.d.Store.UpdateSubscription(ctx, s); err != nil {
			return err
		}
	}
	return nil
}

// systemTransition locks the subscription id and applies the system event ev to it (M2-26 design
// note 5 b); a change the rules refuse changes nothing.
func (w *WebhookService) systemTransition(ctx context.Context, id ids.ID, ev domain.SubscriptionEvent, fromSeq, toSeq int64) error {
	rec, found, err := w.d.Store.LockSubscription(ctx, id)
	if err != nil || !found {
		return err
	}
	if err := w.applyTransition(ctx, &rec, ev, fromSeq, toSeq); err != nil {
		return err
	}
	return w.d.Store.UpdateSubscription(ctx, rec)
}

// Log lines and attributes of the deliveries (docs/observability/signals.md 5.3); the app layer
// does not import platform/otel/attr, the adapter tests hold the keys equal.
const (
	msgDeliveryFailed = "webhook delivery failed"
	msgStateChanged   = "webhook subscription state changed"
	keySubscriptionID = "bothy.webhook_subscription.id"
	keyEventSeq       = "bothy.event.seq"
	keyEventType      = "bothy.event.type"
	keyReason         = "bothy.reason"
	keyServerAddress  = "server.address"
)

// applyTransition applies the system event ev to rec in memory (disabled_at on a change to
// disabled), writes the audit entry of webhook_out.transition in the unit of ctx (R-AZ-27: no
// actor, resource the app) and the warn line; the caller writes rec. A change the rules refuse
// is no error and changes nothing.
func (w *WebhookService) applyTransition(ctx context.Context, rec *SubscriptionRecord, ev domain.SubscriptionEvent, fromSeq, toSeq int64) error {
	from := rec.State
	to, err := domain.NextSubscriptionState(from, ev)
	if err != nil {
		return nil
	}
	rec.State = to
	if to == domain.SubscriptionDisabled {
		rec.DisabledAt = w.now()
	}
	attrs := auditing.Attrs{
		attrSubscriptionID: auditing.ID(rec.ID), attrFromState: auditing.Code(string(from)), attrToState: auditing.Code(string(to)),
		attrCause: auditing.Code(ev.Cause()),
	}
	if ev == domain.EventGap {
		attrs[attrFromSeq], attrs[attrToSeq] = auditing.Int(fromSeq), auditing.Int(toSeq)
	}
	if err := w.d.Audit.Record(ctx, auditing.Entry{
		Action: string(authorize.ActionWebhookOutTransition), Decision: auditing.DecisionAllow,
		ResourceKind: string(authorize.ResourceApp), ResourceID: rec.AppID, Attrs: attrs,
	}); err != nil {
		return errs.Wrap(errs.CodeInternalError, err, "apps: audit of a subscription transition")
	}
	w.d.Logger.LogAttrs(ctx, slog.LevelWarn, msgStateChanged, slog.String(keySubscriptionID, rec.ID.String()),
		slog.String(keyReason, ev.Cause()), slog.String(keyServerAddress, hostOf(rec.URL)))
	return nil
}
