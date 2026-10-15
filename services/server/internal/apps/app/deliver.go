// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"encoding/json"
	"log/slog"
	"slices"
	"time"

	"github.com/OWNER/bothy/services/server/internal/apps"
	"github.com/OWNER/bothy/services/server/internal/apps/domain"
	"github.com/OWNER/bothy/services/server/internal/instance"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/sync"
)

// claim is the result of the claim step: the request to send, or nothing to do.
type claim struct {
	send   bool
	req    SignedRequest
	head   DeliveryRecord
	signer string
}

// Deliver runs one attempt of the job apps.webhook_deliver for a subscription (M2-26 design note 4,
// D-M2-26-3): claim the due head under the subscription lock in one unit of work (with the gap,
// routing and rights checks, and a lease of webhook.delivery_timeout with its wake-up job), send
// it outside any transaction, then record the outcome in a second unit of work and chain the next
// job. Several jobs of one subscription serialize on its row lock, so at most one attempt is in
// flight. Errors (database, context) return and River retries the job; a retry that finds the
// lease ahead ends, and the lease wake-up sends the head again.
func (w *WebhookService) Deliver(ctx context.Context, subscriptionID ids.ID) error {
	var c claim
	if err := w.d.UoW.Do(ctx, func(ctx context.Context) error {
		var err error
		c, err = w.claim(ctx, subscriptionID)
		return err
	}); err != nil || !c.send {
		return err
	}
	res, sendErr := w.d.Sender.Send(ctx, c.req)
	if err := ctx.Err(); err != nil && sendErr != nil {
		// A cancelled job is no answer of the receiver: the lease wake-up sends again.
		return err
	}
	return w.d.UoW.Do(ctx, func(ctx context.Context) error {
		return w.record(ctx, c, res, sendErr)
	})
}

// claim is step a to e of design note 4 inside the unit of ctx.
func (w *WebhookService) claim(ctx context.Context, subscriptionID ids.ID) (claim, error) {
	sub, inst, head, due, err := w.dueHead(ctx, subscriptionID)
	if err != nil || !due {
		return claim{}, err
	}
	now := w.now()
	ev, found, err := w.d.Log.EventBySeq(ctx, head.EventSeq)
	if err != nil {
		return claim{}, err
	}
	if !found {
		return claim{}, w.gap(ctx, &sub)
	}
	if code, err := w.refusal(ctx, sub, inst, ev); err != nil || code != "" {
		if err != nil {
			return claim{}, err
		}
		return claim{}, w.finishWithout(ctx, sub.ID, head, code)
	}
	req, signer, err := w.sign(ctx, sub, ev, now)
	if err != nil {
		return claim{}, err
	}
	head.Attempt++
	head.NextAttemptAt = now.Add(w.cfg.DeliveryTimeout)
	if err := w.d.Store.UpdateDelivery(ctx, head); err != nil {
		return claim{}, err
	}
	if err := w.d.Scheduler.Schedule(ctx, sub.ID, head.NextAttemptAt); err != nil {
		return claim{}, err
	}
	return claim{send: true, req: req, head: head, signer: signer}, nil
}

// refusal returns the code that ends the head without a request (design note 4 c and d):
// not_subscribed when the type left the event types of the subscription or of the installation,
// not_authorized when webhook_out.deliver does not allow it; "" when the attempt may go out.
func (w *WebhookService) refusal(ctx context.Context, sub SubscriptionRecord, inst InstallationRecord, ev sync.Event) (string, error) {
	if !slices.Contains(sub.EventTypes, ev.Type) || !slices.Contains(inst.EventSubscriptions, ev.Type) {
		return apps.LastErrorNotSubscribed, nil
	}
	allowed, err := w.mayReceive(ctx, inst, ev)
	if err != nil || allowed {
		return "", err
	}
	return apps.LastErrorNotAuthorized, nil
}

// dueHead is step a of design note 4: lock the subscription, and report its head when the
// subscription is active, its installation active and the head due; due is false otherwise.
func (w *WebhookService) dueHead(ctx context.Context, subscriptionID ids.ID) (SubscriptionRecord, InstallationRecord, DeliveryRecord, bool, error) {
	sub, found, err := w.d.Store.LockSubscription(ctx, subscriptionID)
	if err != nil || !found || sub.State != domain.SubscriptionActive {
		return SubscriptionRecord{}, InstallationRecord{}, DeliveryRecord{}, false, err
	}
	inst, found, err := w.d.Store.Installation(ctx, sub.InstallationID)
	if err != nil || !found || inst.State != domain.StateActive {
		return SubscriptionRecord{}, InstallationRecord{}, DeliveryRecord{}, false, err
	}
	head, found, err := w.d.Store.Head(ctx, sub.ID)
	if err != nil || !found || head.NextAttemptAt.After(w.now()) {
		return SubscriptionRecord{}, InstallationRecord{}, DeliveryRecord{}, false, err
	}
	return sub, inst, head, true, nil
}

// gap is the gap rule at the head (M2-26 design note 5 e, D-M2-26-5): the subscription becomes
// disabled with cause gap, every pending row below the smallest retained seq becomes dead with
// event_expired, and the audit entry names the first and last lost seq.
func (w *WebhookService) gap(ctx context.Context, sub *SubscriptionRecord) error {
	minSeq, err := w.d.Log.MinRetainedSeq(ctx)
	if err != nil {
		return err
	}
	below := minSeq
	if minSeq == 0 {
		// An empty log retains nothing: every pending row is lost.
		below = 1<<63 - 1
	}
	seqs, err := w.d.Store.ExpirePending(ctx, sub.ID, below)
	if err != nil {
		return err
	}
	var first, last int64
	if len(seqs) > 0 {
		first, last = slices.Min(seqs), slices.Max(seqs)
	}
	if err := w.applyTransition(ctx, sub, domain.EventGap, first, last); err != nil {
		return err
	}
	return w.d.Store.UpdateSubscription(ctx, *sub)
}

// finishWithout ends the head as dead with code without a request: no attempt, no failure
// counted; the next head gets a job (design note 4 c and d, AZ-M49).
func (w *WebhookService) finishWithout(ctx context.Context, subscription ids.ID, head DeliveryRecord, code string) error {
	head.Status, head.LastError, head.NextAttemptAt = apps.DeliveryDead, code, time.Time{}
	if err := w.d.Store.UpdateDelivery(ctx, head); err != nil {
		return err
	}
	return w.d.Scheduler.Schedule(ctx, subscription, w.now())
}

// mayReceive decides webhook_out.deliver for the app of inst on ev in filter mode with the current
// grants and scopes (design note 4 d): allow is true; any other decision is false; an error is
// no decision and returns.
func (w *WebhookService) mayReceive(ctx context.Context, inst InstallationRecord, ev sync.Event) (bool, error) {
	p := actor.Actor{ID: inst.AppID, Kind: actor.KindApp, InstallationID: inst.ID, GrantedScopes: slices.Clone(inst.GrantedScopes)}
	r := authorize.Resource{
		Kind: authorize.ResourceKind(ev.ResourceKind), ID: ev.ResourceID, SpaceID: ev.SpaceID, ChannelID: ev.ChannelID, ThreadID: ev.ThreadID,
		DeliveryOf: authorize.Action(ev.ReceiveAction), Audience: ev.AudiencePrincipalID,
	}
	d, err := w.d.Authorizer.Authorize(authorize.Filter(ctx), p, authorize.ActionWebhookOutDeliver, r)
	if err != nil {
		return false, err
	}
	return d.Allowed(), nil
}

// sign builds the body and the headers of one attempt (requirement 15, design note 11): the
// envelope with sync_epoch, the Unix seconds of now, v1a with every signer of webhook_out and v1
// with the derived keys of the current and, inside webhook.secret_rotation_overlap, the previous
// generation. It returns the request and the kid of the active signer.
func (w *WebhookService) sign(ctx context.Context, sub SubscriptionRecord, ev sync.Event, now time.Time) (SignedRequest, string, error) {
	rec, err := w.d.State.Load(ctx)
	if err != nil {
		return SignedRequest{}, "", err
	}
	body, err := json.Marshal(ev.Envelope(sync.DeliveryFields{SyncEpoch: rec.SyncEpoch}))
	if err != nil {
		return SignedRequest{}, "", errs.Wrap(errs.CodeInternalError, err, "apps: envelope of a delivery")
	}
	ts := now.Unix()
	content := domain.SignedContent(ev.ID, ts, body)
	signers, err := w.d.Keys.Signers(ctx, instance.PurposeWebhookOut)
	if err != nil {
		return SignedRequest{}, "", err
	}
	v1a := make([][]byte, 0, len(signers))
	for _, s := range signers {
		sig, err := s.Sign(ctx, content)
		if err != nil {
			return SignedRequest{}, "", err
		}
		v1a = append(v1a, sig)
	}
	var v1 [][]byte
	if sub.HMACEnabled {
		master, err := w.d.Master.Key(ctx)
		if err != nil {
			return SignedRequest{}, "", err
		}
		for _, g := range domain.HMACGenerations(sub.HMACGeneration, sub.HMACRotatedAt, w.cfg.SecretRotationOverlap, now) {
			v1 = append(v1, domain.HMACSignature(domain.DeriveHMACKey(master, sub.ID, g), content))
		}
	}
	return SignedRequest{
		URL: sub.URL, Host: hostOf(sub.URL), WebhookID: ev.ID.String(), Timestamp: ts, Signature: domain.SignatureHeader(v1a, v1),
		Body: body, SubscriptionID: sub.ID, EventSeq: ev.Seq, EventType: ev.Type,
	}, signers[0].KeyID(), nil
}

// record is step g and h of design note 4 inside the unit of ctx: lock the subscription, reload
// the head and discard the result when it is no longer pending with the claimed attempt; else
// apply the outcome with the timing of D-M2-26-7 and chain the next job.
func (w *WebhookService) record(ctx context.Context, c claim, res SendResult, sendErr error) error {
	sub, found, err := w.d.Store.LockSubscription(ctx, c.req.SubscriptionID)
	if err != nil || !found {
		return err
	}
	d, found, err := w.d.Store.Delivery(ctx, c.head.ID)
	if err != nil || !found || d.Status != apps.DeliveryPending || d.Attempt != c.head.Attempt {
		return err
	}
	o := domain.Classify(res.StatusCode, sendErr)
	now := w.now()
	if sendErr == nil {
		d.ResponseCode = res.StatusCode
	}
	switch o.Kind {
	case domain.OutcomeSucceeded:
		d.Status, d.DeliveredAt, d.NextAttemptAt, d.LastError = apps.DeliverySucceeded, now, time.Time{}, ""
		sub.LastDeliveredSeq = max(sub.LastDeliveredSeq, d.EventSeq)
		sub.ConsecutiveFailures, sub.SigningKeyKID = 0, c.signer
	case domain.OutcomeGone:
		d.Status, d.LastError, d.NextAttemptAt = apps.DeliveryFailed, o.Error, time.Time{}
		if err := w.applyTransition(ctx, &sub, domain.EventGone, 0, 0); err != nil {
			return err
		}
	case domain.OutcomeThrottled, domain.OutcomeFailed:
		if err := w.applyRetry(ctx, &sub, &d, o, res, now); err != nil {
			return err
		}
	}
	if o.Kind != domain.OutcomeSucceeded {
		w.d.Logger.LogAttrs(ctx, slog.LevelInfo, msgDeliveryFailed, slog.String(keySubscriptionID, sub.ID.String()),
			slog.Int64(keyEventSeq, d.EventSeq), slog.String(keyEventType, c.req.EventType), slog.String(keyReason, o.Error),
			slog.String(keyServerAddress, c.req.Host))
	}
	if err := w.d.Store.UpdateDelivery(ctx, d); err != nil {
		return err
	}
	if err := w.d.Store.UpdateSubscription(ctx, sub); err != nil {
		return err
	}
	if sub.State != domain.SubscriptionActive {
		return nil
	}
	at := now
	if d.Status == apps.DeliveryPending {
		at = d.NextAttemptAt
	}
	return w.d.Scheduler.Schedule(ctx, sub.ID, at)
}

// applyRetry applies a throttle or a counted failure (D-M2-26-6, D-M2-26-7): the next attempt after
// backoff and Retry-After, cut at the deadline; at or after created_at + webhook.retry_max_age the
// delivery is dead and the subscription disabled (cause retry_max_age); after
// webhook.max_consecutive_failures counted failures the subscription is disabled and the head stays
// pending.
func (w *WebhookService) applyRetry(ctx context.Context, sub *SubscriptionRecord, d *DeliveryRecord, o domain.Outcome, res SendResult, now time.Time) error {
	if o.Counted() {
		sub.ConsecutiveFailures++
	}
	retryAfter, has := domain.ParseRetryAfter(res.RetryAfter, now)
	next, dead := domain.NextAttempt(o, d.Attempt, retryAfter, has, d.CreatedAt, now, w.retry, w.d.Rand)
	d.LastError = o.Error
	if dead {
		d.Status, d.NextAttemptAt = apps.DeliveryDead, time.Time{}
		return w.applyTransition(ctx, sub, domain.EventRetryMaxAge, 0, 0)
	}
	d.NextAttemptAt = next
	if o.Counted() && sub.ConsecutiveFailures >= w.cfg.MaxConsecutiveFailures {
		return w.applyTransition(ctx, sub, domain.EventMaxFailures, 0, 0)
	}
	return nil
}
