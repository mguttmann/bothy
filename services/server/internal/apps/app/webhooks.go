// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"errors"
	"log/slog"
	"math/rand/v2"
	"slices"
	"time"

	"github.com/OWNER/bothy/services/server/internal/apps"
	"github.com/OWNER/bothy/services/server/internal/apps/domain"
	"github.com/OWNER/bothy/services/server/internal/instance"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/auditing"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
	"github.com/OWNER/bothy/services/server/internal/sync"
)

// WebhookConfig are the configuration keys of the outgoing webhooks; internal/wire binds them.
type WebhookConfig struct {
	// AllowedHosts is webhook.allowed_hosts in the form host[:port] (D-M2-26-8).
	AllowedHosts []string `config:"webhook.allowed_hosts"`
	// MaxConsecutiveFailures is webhook.max_consecutive_failures.
	MaxConsecutiveFailures int `config:"webhook.max_consecutive_failures"`
	// RetryBackoffBase and RetryBackoffCap are webhook.retry_backoff_base and
	// webhook.retry_backoff_cap.
	RetryBackoffBase time.Duration `config:"webhook.retry_backoff_base"`
	RetryBackoffCap  time.Duration `config:"webhook.retry_backoff_cap"`
	// RetryMaxAge is webhook.retry_max_age.
	RetryMaxAge time.Duration `config:"webhook.retry_max_age"`
	// MaxPendingPerSubscription is webhook.max_pending_per_subscription.
	MaxPendingPerSubscription int `config:"webhook.max_pending_per_subscription"`
	// SecretRotationOverlap is webhook.secret_rotation_overlap.
	SecretRotationOverlap time.Duration `config:"webhook.secret_rotation_overlap"`
	// DeliveryTimeout is webhook.delivery_timeout: the time limit of one attempt and the lease of
	// an attempt in flight.
	DeliveryTimeout time.Duration `config:"webhook.delivery_timeout"`
	// PageSizeDefault and PageSizeMax are api.page_size_default and api.page_size_max.
	PageSizeDefault int `config:"api.page_size_default"`
	PageSizeMax     int `config:"api.page_size_max"`
}

// InstanceState is the part of instance.State the deliveries read: the sync epoch of the row.
type InstanceState interface {
	Load(ctx context.Context) (instance.Record, error)
}

// WebhookDeps are the collaborators of the webhook use cases.
type WebhookDeps struct {
	// Store reads and writes the webhook tables; Repo the apps and installations.
	Store WebhookStore
	Repo  Repository
	UoW   txn.UnitOfWork
	// Authorizer decides every call, also webhook_out.deliver in filter mode before every attempt.
	Authorizer authorize.Authorizer
	// Audit writes the allow entries and the system transitions (webhook_out.transition).
	Audit auditing.Recorder
	// Scheduler inserts the job apps.webhook_deliver in the ambient unit of work.
	Scheduler DeliveryScheduler
	// Sender sends one attempt through the egress client.
	Sender WebhookSender
	// Master is the master key of the derived v1 secrets.
	Master HMACMasterKey
	// Keys signs v1a with the webhook_out signing set.
	Keys Signers
	// State is the instance row (sync_epoch of the deliveries).
	State InstanceState
	// Log reads the event of a delivery and the smallest retained seq.
	Log sync.LogReader
	// Cursors encodes and checks the cursors of listWebhookDeliveries.
	Cursors Cursors
	// Clock is the time of the writes and attempts; nil means clock.System().
	Clock clock.Clock
	// Rand is the random source of the jitter, rnd(n) in [0, n); nil means math/rand/v2.
	Rand func(n int64) int64
	// Logger receives the delivery and state lines; nil discards them.
	Logger *slog.Logger
}

// WebhookService implements apps.Webhooks, the consumer webhooks (Consumer), the attempt of the
// job apps.webhook_deliver (Deliver) and the hooks of install, uninstall and resume. It is safe
// for concurrent use.
type WebhookService struct {
	cfg   WebhookConfig
	d     WebhookDeps
	hosts []domain.AllowedHost
	retry domain.RetryConfig
}

// Compile-time check: WebhookService is the incoming port.
var _ apps.Webhooks = (*WebhookService)(nil)

// NewWebhooks returns the webhook use cases. Errors: a missing collaborator, a value outside its
// range, or an entry of webhook.allowed_hosts that is no host[:port].
func NewWebhooks(cfg WebhookConfig, d WebhookDeps) (*WebhookService, error) {
	if d.Store == nil || d.Repo == nil || d.UoW == nil || d.Authorizer == nil || d.Audit == nil || d.Scheduler == nil ||
		d.Sender == nil || d.Master == nil || d.Keys == nil || d.State == nil || d.Log == nil || d.Cursors == nil {
		return nil, errors.New("apps: NewWebhooks needs every collaborator")
	}
	switch {
	case cfg.MaxConsecutiveFailures < 1, cfg.MaxPendingPerSubscription < 1, cfg.PageSizeDefault < 1, cfg.PageSizeMax < cfg.PageSizeDefault:
		return nil, errors.New("apps: the webhook limits must be at least 1")
	case cfg.RetryBackoffBase <= 0, cfg.RetryBackoffCap < cfg.RetryBackoffBase, cfg.RetryMaxAge <= 0, cfg.SecretRotationOverlap <= 0,
		cfg.DeliveryTimeout <= 0:
		return nil, errors.New("apps: the webhook durations must be positive and the backoff base at most the cap")
	}
	hosts, err := domain.ParseAllowedHosts(cfg.AllowedHosts)
	if err != nil {
		return nil, err
	}
	if d.Clock == nil {
		d.Clock = clock.System()
	}
	if d.Rand == nil {
		d.Rand = rand.Int64N // #nosec G404 -- the jitter of a retry is no secret
	}
	if d.Logger == nil {
		d.Logger = slog.New(slog.DiscardHandler)
	}
	return &WebhookService{cfg: cfg, d: d, hosts: hosts, retry: domain.RetryConfig{
		BackoffBase: cfg.RetryBackoffBase, BackoffCap: cfg.RetryBackoffCap, MaxAge: cfg.RetryMaxAge,
	}}, nil
}

// now is the time of the clock at the precision of PostgreSQL.
func (w *WebhookService) now() time.Time { return w.d.Clock.Now().Truncate(time.Microsecond) }

// Reasons of a 409 conflict of the webhook operations (D-M2-26-18).
const (
	reasonAppNotInstalled = "app_not_installed"
	reasonEventExpired    = "event_expired"
)

// The audit attributes of the webhook actions (SEC-AUD-5): ids, codes and integers; never the URL
// path or query and never a secret.
const (
	attrSubscriptionID = "subscription_id"
	attrChange         = "change"
	attrHost           = "host"
	attrEventTypes     = "event_types"
	attrHMACEnabled    = "hmac_enabled"
	attrHMACGeneration = "hmac_generation"
	attrFromState      = "from_state"
	attrToState        = "to_state"
	attrCause          = "cause"
	attrFromSeq        = "from_seq"
	attrToSeq          = "to_seq"
	attrDeliveryID     = "delivery_id"
	attrEventSeq       = "event_seq"
	attrPreviousStatus = "previous_status"
)

// The change codes of webhook_out.manage.
const (
	changeCreated = "created"
	changeUpdated = "updated"
	changePaused  = "paused"
	changeResumed = "resumed"
	changeRotated = "rotated"
	changeDeleted = "deleted"
)

// authorizeOn authorizes act on r for a and returns the decision as an error (nil for allow).
func (w *WebhookService) authorizeOn(ctx context.Context, a actor.Actor, act authorize.Action, r authorize.Resource) error {
	d, err := w.d.Authorizer.Authorize(ctx, a, act, r)
	if err != nil {
		return err
	}
	return d.Err()
}

// target checks an address (D-M2-26-8): 400 webhook_host_not_allowed for a host or port outside
// webhook.allowed_hosts, 400 validation_failed at /url for every other defect.
func (w *WebhookService) target(raw string) (domain.Target, error) {
	t, err := domain.ValidateTarget(raw, w.hosts)
	switch {
	case errors.Is(err, domain.ErrHostNotAllowed):
		return domain.Target{}, errs.New(errs.CodeWebhookHostNotAllowed, "The host or port is not in webhook.allowed_hosts (R-DM-17).").
			WithFields(errs.FieldError{Pointer: "/url", Code: domain.CodeHostNotAllowed, Detail: "The host or port is not in webhook.allowed_hosts."})
	case err != nil:
		return domain.Target{}, fieldErr("/url", domain.CodeInvalid, "The address is no absolute https address without user information and fragment.")
	}
	return t, nil
}

// eventTypes checks the event types against the installation (400 validation_failed).
func eventTypes(types, granted []string) error {
	fe := domain.ValidateEventTypes(types, granted)
	if len(fe) == 0 {
		return nil
	}
	return errs.New(errs.CodeValidationFailed, "The event types break the rules of webhooks.md section 1.").WithFields(fe...)
}

// view assembles the Subscription of the port.
func subscriptionView(r SubscriptionRecord) apps.Subscription {
	schemes := []string{domain.SchemeV1a}
	if r.HMACEnabled {
		schemes = append(schemes, domain.SchemeV1)
	}
	return apps.Subscription{
		ID: r.ID, AppID: r.AppID, URL: r.URL, EventTypes: nonNil(r.EventTypes), State: string(r.State),
		ConsecutiveFailures: r.ConsecutiveFailures, SignatureSchemes: schemes, SigningKeyKID: r.SigningKeyKID,
		CreatedAt: r.CreatedAt, DisabledAt: r.DisabledAt,
	}
}

// secret derives the v1 secret of a generation (D-M2-26-10) in the form shown once.
func (w *WebhookService) secret(ctx context.Context, subscription ids.ID, generation int) (string, error) {
	master, err := w.d.Master.Key(ctx)
	if err != nil {
		return "", err
	}
	return domain.FormatSecret(domain.DeriveHMACKey(master, subscription, generation)), nil
}

// manageAttrs are the details of a webhook_out.manage entry.
func manageAttrs(r SubscriptionRecord, host, change string) auditing.Attrs {
	return auditing.Attrs{
		attrSubscriptionID: auditing.ID(r.ID), attrAppID: auditing.ID(r.AppID), attrChange: auditing.Code(change),
		attrHost: manifestCode(host), attrEventTypes: codes(r.EventTypes), attrHMACEnabled: auditing.Bool(r.HMACEnabled),
		attrHMACGeneration: auditing.Int(int64(r.HMACGeneration)),
	}
}

// recordManage writes the allow entry of webhook_out.manage on the app of r.
func (w *WebhookService) recordManage(ctx context.Context, r SubscriptionRecord, host, change string) error {
	return w.d.Audit.Record(ctx, auditing.Entry{
		Action: string(authorize.ActionWebhookOutManage), Decision: auditing.DecisionAllow, ResourceKind: string(authorize.ResourceApp),
		ResourceID: r.AppID, Attrs: manageAttrs(r, host, change),
	})
}

// hostOf returns the canonical host of a stored address for the audit and the logs.
func hostOf(raw string) string { return domain.HostOf(raw) }

// CreateSubscription implements apps.Webhooks (M2-26 design note 2 a): webhook_out.manage on the
// app, then one unit of work that locks the current installation FOR SHARE (none active or
// suspended: 409 conflict app_not_installed), checks address and event types, inserts the
// subscription signed by the active webhook_out key and writes the allow entry. With HMAC the
// derived secret comes back once.
func (w *WebhookService) CreateSubscription(ctx context.Context, a actor.Actor, cmd apps.CreateSubscription) (apps.SubscriptionCreated, error) {
	ctx, err := withCaller(ctx, a)
	if err != nil {
		return apps.SubscriptionCreated{}, err
	}
	if err := w.authorizeOn(ctx, a, authorize.ActionWebhookOutManage, appResource(cmd.AppID)); err != nil {
		return apps.SubscriptionCreated{}, err
	}
	var rec SubscriptionRecord
	err = w.d.UoW.Do(ctx, func(ctx context.Context) error {
		if _, found, err := w.d.Repo.App(ctx, cmd.AppID); err != nil || !found {
			return orNotFound(err)
		}
		inst, found, err := w.d.Store.ShareLiveInstallation(ctx, cmd.AppID)
		if err != nil {
			return err
		}
		if !found {
			return errs.New(errs.CodeConflict, "The app has no active or suspended installation.").WithReason(reasonAppNotInstalled)
		}
		rec, err = w.insert(ctx, a, inst, cmd.URL, cmd.EventTypes, cmd.EnableHMAC)
		return err
	})
	if err != nil {
		return apps.SubscriptionCreated{}, err
	}
	out := apps.SubscriptionCreated{Subscription: subscriptionView(rec)}
	if rec.HMACEnabled {
		if out.HMACSecret, err = w.secret(ctx, rec.ID, rec.HMACGeneration); err != nil {
			return apps.SubscriptionCreated{}, err
		}
	}
	return out, nil
}

// insert checks address and event types against the installation, inserts the subscription and
// writes the allow entry with change created, inside the unit of ctx.
func (w *WebhookService) insert(ctx context.Context, a actor.Actor, inst InstallationRecord, rawURL string, types []string, hmac bool) (SubscriptionRecord, error) {
	t, err := w.target(rawURL)
	if err != nil {
		return SubscriptionRecord{}, err
	}
	if err := eventTypes(types, inst.EventSubscriptions); err != nil {
		return SubscriptionRecord{}, err
	}
	signer, err := w.d.Keys.Signer(ctx, instance.PurposeWebhookOut)
	if err != nil {
		return SubscriptionRecord{}, err
	}
	rec, err := w.d.Store.InsertSubscription(ctx, SubscriptionRecord{
		ID: ids.New(), InstallationID: inst.ID, AppID: inst.AppID, URL: rawURL, EventTypes: slices.Clone(types),
		SigningKeyKID: signer.KeyID(), HMACEnabled: hmac, State: domain.SubscriptionActive, CreatedBy: a.ID,
	})
	if err != nil {
		return SubscriptionRecord{}, err
	}
	return rec, w.recordManage(ctx, rec, t.Host, changeCreated)
}

// loadVisible reads the subscription id and authorizes act on its app: an unknown id is 404
// before authorize, and a caller who cannot see the app gets the same 404 (AZ-P4).
func (w *WebhookService) loadVisible(ctx context.Context, a actor.Actor, act authorize.Action, id ids.ID) (SubscriptionRecord, error) {
	rec, found, err := w.d.Store.Subscription(ctx, id)
	if err != nil {
		return SubscriptionRecord{}, err
	}
	if !found {
		return SubscriptionRecord{}, notFound()
	}
	return rec, w.authorizeOn(ctx, a, act, appResource(rec.AppID))
}

// UpdateSubscription implements apps.Webhooks (M2-26 design note 2 b): webhook_out.manage on the
// app of the subscription, then one unit of work under the row lock: a new address or event types
// are checked as at creation; state paused pauses, state active resumes or re-enables (failures
// reset, the head due now, a job scheduled); rotate_hmac_secret raises the generation and returns
// the new secret once. An empty body changes nothing and writes no entry.
func (w *WebhookService) UpdateSubscription(ctx context.Context, a actor.Actor, cmd apps.UpdateSubscription) (apps.SubscriptionCreated, error) {
	ctx, err := withCaller(ctx, a)
	if err != nil {
		return apps.SubscriptionCreated{}, err
	}
	if _, err := w.loadVisible(ctx, a, authorize.ActionWebhookOutManage, cmd.ID); err != nil {
		return apps.SubscriptionCreated{}, err
	}
	var rec SubscriptionRecord
	err = w.d.UoW.Do(ctx, func(ctx context.Context) error {
		var err error
		rec, err = w.update(ctx, cmd)
		return err
	})
	if err != nil {
		return apps.SubscriptionCreated{}, err
	}
	out := apps.SubscriptionCreated{Subscription: subscriptionView(rec)}
	if cmd.RotateHMACSecret {
		if out.HMACSecret, err = w.secret(ctx, rec.ID, rec.HMACGeneration); err != nil {
			return apps.SubscriptionCreated{}, err
		}
	}
	return out, nil
}

// update is the work of UpdateSubscription inside the unit of ctx.
func (w *WebhookService) update(ctx context.Context, cmd apps.UpdateSubscription) (SubscriptionRecord, error) {
	rec, found, err := w.d.Store.LockSubscription(ctx, cmd.ID)
	if err != nil {
		return SubscriptionRecord{}, err
	}
	if !found {
		return SubscriptionRecord{}, notFound()
	}
	host, changed, err := w.applyFields(ctx, &rec, cmd)
	if err != nil {
		return SubscriptionRecord{}, err
	}
	change := changeUpdated
	if cmd.RotateHMACSecret {
		if !rec.HMACEnabled {
			return SubscriptionRecord{}, fieldErr("/rotate_hmac_secret", domain.CodeHMACNotEnabled, "The subscription signs without HMAC; there is no secret to rotate.")
		}
		rec.HMACGeneration++
		rec.HMACRotatedAt, change, changed = w.now(), changeRotated, true
	}
	if cmd.State != nil && *cmd.State != string(rec.State) {
		c, err := w.ownerState(ctx, &rec, domain.SubscriptionState(*cmd.State))
		if err != nil {
			return SubscriptionRecord{}, err
		}
		if change != changeRotated {
			change = c
		}
		changed = true
	}
	if !changed {
		return rec, nil
	}
	if err := w.d.Store.UpdateSubscription(ctx, rec); err != nil {
		return SubscriptionRecord{}, err
	}
	return rec, w.recordManage(ctx, rec, host, change)
}

// applyFields applies a new address and new event types of cmd to rec, checked as at creation; it
// returns the host for the audit entry and whether anything changed.
func (w *WebhookService) applyFields(ctx context.Context, rec *SubscriptionRecord, cmd apps.UpdateSubscription) (string, bool, error) {
	host, changed := hostOf(rec.URL), false
	if cmd.URL != nil {
		t, err := w.target(*cmd.URL)
		if err != nil {
			return "", false, err
		}
		rec.URL, host, changed = *cmd.URL, t.Host, true
	}
	if cmd.EventTypes != nil {
		inst, found, err := w.d.Store.Installation(ctx, rec.InstallationID)
		if err != nil || !found {
			return "", false, orNotFound(err)
		}
		if err := eventTypes(*cmd.EventTypes, inst.EventSubscriptions); err != nil {
			return "", false, err
		}
		rec.EventTypes, changed = slices.Clone(*cmd.EventTypes), true
	}
	return host, changed, nil
}

// ownerState applies the owner transition to state on rec (M2-26 design note 5 a): pause, or
// resume and re-enable with the failures reset, disabled_at cleared, the head due now and a job.
func (w *WebhookService) ownerState(ctx context.Context, rec *SubscriptionRecord, state domain.SubscriptionState) (string, error) {
	ev, change := domain.EventOwnerPause, changePaused
	if state == domain.SubscriptionActive {
		ev, change = domain.EventOwnerResume, changeResumed
	}
	next, err := domain.NextSubscriptionState(rec.State, ev)
	if err != nil || next != state {
		return "", fieldErr("/state", domain.CodeInvalid, "The subscription cannot change to this state.")
	}
	rec.State = next
	if ev == domain.EventOwnerResume {
		rec.ConsecutiveFailures, rec.DisabledAt = 0, time.Time{}
		if err := w.headDue(ctx, rec.ID); err != nil {
			return "", err
		}
	}
	return change, nil
}

// headDue makes the head of a subscription due now and schedules its delivery.
func (w *WebhookService) headDue(ctx context.Context, subscription ids.ID) error {
	head, found, err := w.d.Store.Head(ctx, subscription)
	if err != nil || !found {
		return err
	}
	now := w.now()
	head.NextAttemptAt = now
	if err := w.d.Store.UpdateDelivery(ctx, head); err != nil {
		return err
	}
	return w.d.Scheduler.Schedule(ctx, subscription, now)
}

// DeleteSubscription implements apps.Webhooks (M2-26 design note 2 c): webhook_out.manage on the
// app, then the row goes with its deliveries; a second delete is 404.
func (w *WebhookService) DeleteSubscription(ctx context.Context, a actor.Actor, id ids.ID) error {
	ctx, err := withCaller(ctx, a)
	if err != nil {
		return err
	}
	rec, err := w.loadVisible(ctx, a, authorize.ActionWebhookOutManage, id)
	if err != nil {
		return err
	}
	return w.d.UoW.Do(ctx, func(ctx context.Context) error {
		found, err := w.d.Store.DeleteSubscription(ctx, id)
		if err != nil {
			return err
		}
		if !found {
			return notFound()
		}
		return w.recordManage(ctx, rec, hostOf(rec.URL), changeDeleted)
	})
}

// ListSubscriptions implements apps.Webhooks (M2-26 design note 2 d): webhook_out.read on the app,
// then every subscription of every installation of the app.
func (w *WebhookService) ListSubscriptions(ctx context.Context, a actor.Actor, appID ids.ID) ([]apps.Subscription, error) {
	if err := w.authorizeOn(ctx, a, authorize.ActionWebhookOutRead, appResource(appID)); err != nil {
		return nil, err
	}
	if _, found, err := w.d.Repo.App(ctx, appID); err != nil || !found {
		return nil, orNotFound(err)
	}
	rows, err := w.d.Store.AppSubscriptions(ctx, appID)
	if err != nil {
		return nil, err
	}
	out := make([]apps.Subscription, 0, len(rows))
	for _, r := range rows {
		out = append(out, subscriptionView(r))
	}
	return out, nil
}

// deliveriesList is the list name of the cursors of listWebhookDeliveries.
const deliveriesList = "webhook_deliveries"

// deliveryCursor is the keyset position of a delivery page: event_seq and id of its last row.
type deliveryCursor struct {
	Seq int64  `json:"seq"`
	ID  ids.ID `json:"id"`
}

// ListDeliveries implements apps.Webhooks (M2-26 design note 2 d, D-M2-26-20): webhook_out.read
// on the app of the subscription, then one page newest first ((event_seq, id) descending) bound to
// subscription and status filter; event_type is read from the event log while the event is
// retained.
func (w *WebhookService) ListDeliveries(ctx context.Context, a actor.Actor, q apps.DeliveryQuery) (apps.DeliveryPage, error) {
	if _, err := w.loadVisible(ctx, a, authorize.ActionWebhookOutRead, q.SubscriptionID); err != nil {
		return apps.DeliveryPage{}, err
	}
	binding := []byte(q.SubscriptionID.String() + "\x00status=" + q.Status)
	var after deliveryCursor
	if q.Cursor != "" {
		if err := w.d.Cursors.Decode(deliveriesList, binding, q.Cursor, &after); err != nil {
			return apps.DeliveryPage{}, err
		}
		if after.ID.IsZero() {
			return apps.DeliveryPage{}, errs.New(errs.CodeCursorInvalid, "")
		}
	}
	limit := q.Limit
	if limit < 1 {
		limit = w.cfg.PageSizeDefault
	}
	limit = min(limit, w.cfg.PageSizeMax)
	rows, err := w.d.Store.Deliveries(ctx, q.SubscriptionID, q.Status, q.Cursor != "", after.Seq, after.ID, limit+1)
	if err != nil {
		return apps.DeliveryPage{}, err
	}
	var out apps.DeliveryPage
	if len(rows) > limit {
		rows = rows[:limit]
		last := rows[limit-1]
		if out.NextCursor, err = w.d.Cursors.Encode(deliveriesList, binding, deliveryCursor{Seq: last.EventSeq, ID: last.ID}); err != nil {
			return apps.DeliveryPage{}, errs.Wrap(errs.CodeInternalError, err, "apps: cursor")
		}
	}
	out.Items = make([]apps.Delivery, 0, len(rows))
	for _, r := range rows {
		v, err := w.deliveryView(ctx, r)
		if err != nil {
			return apps.DeliveryPage{}, err
		}
		out.Items = append(out.Items, v)
	}
	return out, nil
}

// deliveryView assembles the Delivery of the port with event_type from the log (D-M2-26-13).
func (w *WebhookService) deliveryView(ctx context.Context, r DeliveryRecord) (apps.Delivery, error) {
	ev, found, err := w.d.Log.EventBySeq(ctx, r.EventSeq)
	if err != nil {
		return apps.Delivery{}, err
	}
	v := apps.Delivery{
		ID: r.ID, SubscriptionID: r.SubscriptionID, EventID: r.EventID, EventSeq: r.EventSeq, Attempt: r.Attempt, Status: r.Status,
		ResponseCode: r.ResponseCode, LastError: r.LastError, NextAttemptAt: r.NextAttemptAt, CreatedAt: r.CreatedAt,
		DeliveredAt: r.DeliveredAt,
	}
	if found {
		v.EventType = ev.Type
	}
	return v, nil
}

// Redeliver implements apps.Webhooks (M2-26 design note 13 b, D-M2-26-14): webhook_out.redeliver
// on the delivery, then under the subscription lock a pending row becomes due now; a finished row
// becomes pending again with created_at now (a new retry window), last_error and response_code
// cleared and attempt kept; a job is scheduled; an event no longer in the log is 409 conflict
// event_expired. The audit entry names delivery, subscription, seq and the previous status.
func (w *WebhookService) Redeliver(ctx context.Context, a actor.Actor, deliveryID ids.ID) (apps.Delivery, error) {
	ctx, err := withCaller(ctx, a)
	if err != nil {
		return apps.Delivery{}, err
	}
	d, found, err := w.d.Store.Delivery(ctx, deliveryID)
	if err != nil {
		return apps.Delivery{}, err
	}
	if !found {
		return apps.Delivery{}, notFound()
	}
	r := authorize.Resource{Kind: authorize.ResourceWebhookDelivery, ID: deliveryID}
	if err := w.authorizeOn(ctx, a, authorize.ActionWebhookOutRedeliver, r); err != nil {
		return apps.Delivery{}, err
	}
	var out apps.Delivery
	err = w.d.UoW.Do(ctx, func(ctx context.Context) error {
		var err error
		out, err = w.redeliver(ctx, d.SubscriptionID, deliveryID)
		return err
	})
	return out, err
}

// redeliver is the work of Redeliver inside the unit of ctx.
func (w *WebhookService) redeliver(ctx context.Context, subscription, deliveryID ids.ID) (apps.Delivery, error) {
	if _, found, err := w.d.Store.LockSubscription(ctx, subscription); err != nil || !found {
		return apps.Delivery{}, orNotFound(err)
	}
	d, found, err := w.d.Store.Delivery(ctx, deliveryID)
	if err != nil || !found {
		return apps.Delivery{}, orNotFound(err)
	}
	ev, found, err := w.d.Log.EventBySeq(ctx, d.EventSeq)
	if err != nil {
		return apps.Delivery{}, err
	}
	if !found {
		return apps.Delivery{}, errs.New(errs.CodeConflict, "The event is no longer in the event log.").WithReason(reasonEventExpired)
	}
	previous, now := d.Status, w.now()
	if d.Status != apps.DeliveryPending {
		d.Status, d.LastError, d.ResponseCode, d.CreatedAt, d.DeliveredAt = apps.DeliveryPending, "", 0, now, time.Time{}
	}
	d.NextAttemptAt = now
	if err := w.d.Store.UpdateDelivery(ctx, d); err != nil {
		return apps.Delivery{}, err
	}
	if err := w.d.Scheduler.Schedule(ctx, subscription, now); err != nil {
		return apps.Delivery{}, err
	}
	if err := w.d.Audit.Record(ctx, auditing.Entry{
		Action: string(authorize.ActionWebhookOutRedeliver), Decision: auditing.DecisionAllow,
		ResourceKind: string(authorize.ResourceWebhookDelivery), ResourceID: d.ID,
		Attrs: auditing.Attrs{
			attrDeliveryID: auditing.ID(d.ID), attrSubscriptionID: auditing.ID(subscription), attrEventSeq: auditing.Int(d.EventSeq),
			attrPreviousStatus: auditing.Code(previous),
		},
	}); err != nil {
		return apps.Delivery{}, err
	}
	v, err := w.deliveryView(ctx, d)
	v.EventType = ev.Type
	return v, err
}

// ResumeInstallation implements apps.Webhooks (M2-26 design note 14): the head of every active
// subscription of the installation becomes due now and gets a job, in the caller's unit of work.
func (w *WebhookService) ResumeInstallation(ctx context.Context, installationID ids.ID) error {
	subs, err := w.d.Store.LockInstallationSubscriptions(ctx, installationID)
	if err != nil {
		return err
	}
	for _, s := range subs {
		if s.State != domain.SubscriptionActive {
			continue
		}
		if err := w.headDue(ctx, s.ID); err != nil {
			return err
		}
	}
	return nil
}

// disableInstallation sets every subscription of the installation disabled in the caller's unit
// of work (app.uninstall, 11 section 4.3; covered by the entry of app.uninstall).
func (w *WebhookService) disableInstallation(ctx context.Context, installationID ids.ID) error {
	subs, err := w.d.Store.LockInstallationSubscriptions(ctx, installationID)
	if err != nil {
		return err
	}
	now := w.now()
	for _, s := range subs {
		if s.State == domain.SubscriptionDisabled {
			continue
		}
		s.State, s.DisabledAt = domain.SubscriptionDisabled, now
		if err := w.d.Store.UpdateSubscription(ctx, s); err != nil {
			return err
		}
	}
	return nil
}

// createManifestSubscription creates the subscription of a manifest with delivery.mode webhook in
// the install transaction (M2-26 design note 14): webhook_out.manage first, url the events_url,
// event types the installation's subscribable event subscriptions, no HMAC, created by the
// installing owner, with its own audit entry. Without subscribable events nothing is created.
func (w *WebhookService) createManifestSubscription(ctx context.Context, a actor.Actor, inst InstallationRecord, eventsURL string) error {
	var types []string
	for _, t := range inst.EventSubscriptions {
		if domain.AppSubscribable(t) {
			types = append(types, t)
		}
	}
	if len(types) == 0 {
		return nil
	}
	if err := w.authorizeOn(ctx, a, authorize.ActionWebhookOutManage, appResource(inst.AppID)); err != nil {
		return err
	}
	_, err := w.insert(ctx, a, inst, eventsURL, types, false)
	return err
}
