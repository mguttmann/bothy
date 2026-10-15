// SPDX-License-Identifier: Apache-2.0

package httpapi

import (
	"context"
	"time"

	"github.com/OWNER/bothy/services/server/api/rest"
	"github.com/OWNER/bothy/services/server/internal/apps"
	"github.com/OWNER/bothy/services/server/internal/kernel/idempotency"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// Webhooks implements listWebhookSubscriptions, createWebhookSubscription,
// updateWebhookSubscription, deleteWebhookSubscription, listWebhookDeliveries and redeliverWebhook
// of rest.StrictServerInterface over the port apps.Webhooks (M2-26). The use cases authorize; the
// handlers convert, and mark a response with a v1 secret as a one-time secret (R-API-26).
type Webhooks struct {
	hooks apps.Webhooks
}

// NewWebhooks returns the handlers over the use cases.
func NewWebhooks(w apps.Webhooks) *Webhooks { return &Webhooks{hooks: w} }

// ListWebhookSubscriptions implements rest.StrictServerInterface (GET /apps/{app_id}/webhooks).
func (h *Webhooks) ListWebhookSubscriptions(ctx context.Context, req rest.ListWebhookSubscriptionsRequestObject) (rest.ListWebhookSubscriptionsResponseObject, error) {
	list, err := h.hooks.ListSubscriptions(ctx, principal(ctx), ids.ID(req.AppId))
	if err != nil {
		return nil, err
	}
	out := rest.WebhookSubscriptionList{Items: make([]rest.WebhookSubscription, 0, len(list))}
	for _, s := range list {
		out.Items = append(out.Items, subscriptionREST(s))
	}
	return rest.ListWebhookSubscriptions200JSONResponse{Body: out}, nil
}

// CreateWebhookSubscription implements rest.StrictServerInterface (POST /apps/{app_id}/webhooks).
func (h *Webhooks) CreateWebhookSubscription(ctx context.Context, req rest.CreateWebhookSubscriptionRequestObject) (rest.CreateWebhookSubscriptionResponseObject, error) {
	cmd := apps.CreateSubscription{AppID: ids.ID(req.AppId)}
	if req.Body != nil {
		cmd.URL, cmd.EventTypes = req.Body.Url, append([]string{}, req.Body.EventTypes...)
		cmd.EnableHMAC = req.Body.EnableHmac != nil && *req.Body.EnableHmac
	}
	res, err := h.hooks.CreateSubscription(ctx, principal(ctx), cmd)
	if err != nil {
		return nil, err
	}
	return rest.CreateWebhookSubscription201JSONResponse{Body: createdREST(ctx, res)}, nil
}

// UpdateWebhookSubscription implements rest.StrictServerInterface (PATCH
// /webhooks/{subscription_id}).
func (h *Webhooks) UpdateWebhookSubscription(ctx context.Context, req rest.UpdateWebhookSubscriptionRequestObject) (rest.UpdateWebhookSubscriptionResponseObject, error) {
	cmd := apps.UpdateSubscription{ID: ids.ID(req.SubscriptionId)}
	if b := req.Body; b != nil {
		cmd.URL, cmd.EventTypes = b.Url, b.EventTypes
		if b.State != nil {
			s := string(*b.State)
			cmd.State = &s
		}
		cmd.RotateHMACSecret = b.RotateHmacSecret != nil && *b.RotateHmacSecret
	}
	res, err := h.hooks.UpdateSubscription(ctx, principal(ctx), cmd)
	if err != nil {
		return nil, err
	}
	return rest.UpdateWebhookSubscription200JSONResponse{Body: createdREST(ctx, res)}, nil
}

// DeleteWebhookSubscription implements rest.StrictServerInterface (DELETE
// /webhooks/{subscription_id}).
func (h *Webhooks) DeleteWebhookSubscription(ctx context.Context, req rest.DeleteWebhookSubscriptionRequestObject) (rest.DeleteWebhookSubscriptionResponseObject, error) {
	if err := h.hooks.DeleteSubscription(ctx, principal(ctx), ids.ID(req.SubscriptionId)); err != nil {
		return nil, err
	}
	return rest.DeleteWebhookSubscription204Response{}, nil
}

// ListWebhookDeliveries implements rest.StrictServerInterface (GET
// /webhooks/{subscription_id}/deliveries).
func (h *Webhooks) ListWebhookDeliveries(ctx context.Context, req rest.ListWebhookDeliveriesRequestObject) (rest.ListWebhookDeliveriesResponseObject, error) {
	q := apps.DeliveryQuery{SubscriptionID: ids.ID(req.SubscriptionId)}
	if req.Params.Cursor != nil {
		q.Cursor = *req.Params.Cursor
	}
	if req.Params.Limit != nil {
		q.Limit = *req.Params.Limit
	}
	if req.Params.Status != nil {
		q.Status = string(*req.Params.Status)
	}
	p, err := h.hooks.ListDeliveries(ctx, principal(ctx), q)
	if err != nil {
		return nil, err
	}
	out := rest.WebhookDeliveryPage{Items: make([]rest.WebhookDelivery, 0, len(p.Items))}
	for _, d := range p.Items {
		out.Items = append(out.Items, deliveryREST(d))
	}
	if p.NextCursor != "" {
		out.NextCursor = &p.NextCursor
	}
	return rest.ListWebhookDeliveries200JSONResponse{Body: out}, nil
}

// RedeliverWebhook implements rest.StrictServerInterface (POST
// /webhook-deliveries/{delivery_id}/redeliver).
func (h *Webhooks) RedeliverWebhook(ctx context.Context, req rest.RedeliverWebhookRequestObject) (rest.RedeliverWebhookResponseObject, error) {
	d, err := h.hooks.Redeliver(ctx, principal(ctx), ids.ID(req.DeliveryId))
	if err != nil {
		return nil, err
	}
	return rest.RedeliverWebhook202JSONResponse{Body: deliveryREST(d)}, nil
}

// createdREST converts a create or update answer; a response with a secret is marked as a
// one-time secret, so the Idempotency-Key stage stores no replay of it (R-API-26).
func createdREST(ctx context.Context, res apps.SubscriptionCreated) rest.WebhookSubscriptionCreated {
	out := rest.WebhookSubscriptionCreated{Subscription: subscriptionREST(res.Subscription)}
	if res.HMACSecret != "" {
		idempotency.MarkOneTimeSecret(ctx)
		secret := res.HMACSecret
		out.HmacSecret = &secret
	}
	return out
}

// subscriptionREST converts a subscription; absent values are absent members (no JSON null).
func subscriptionREST(s apps.Subscription) rest.WebhookSubscription {
	schemes := make([]rest.WebhookSubscriptionSignatureSchemes, 0, len(s.SignatureSchemes))
	for _, sc := range s.SignatureSchemes {
		schemes = append(schemes, rest.WebhookSubscriptionSignatureSchemes(sc))
	}
	out := rest.WebhookSubscription{
		Id: rest.Uuid(s.ID), Url: s.URL, EventTypes: append([]string{}, s.EventTypes...), State: rest.WebhookSubscriptionState(s.State),
		ConsecutiveFailures: s.ConsecutiveFailures, SignatureSchemes: schemes, SigningKeyKid: s.SigningKeyKID, CreatedAt: millis(s.CreatedAt),
	}
	if !s.AppID.IsZero() {
		app := rest.Uuid(s.AppID)
		out.AppId = &app
	}
	if !s.DisabledAt.IsZero() {
		at := millis(s.DisabledAt)
		out.DisabledAt = &at
	}
	return out
}

// deliveryREST converts a delivery; event_type is absent once the event left the log.
func deliveryREST(d apps.Delivery) rest.WebhookDelivery {
	out := rest.WebhookDelivery{
		Id: rest.Uuid(d.ID), SubscriptionId: rest.Uuid(d.SubscriptionID), EventId: rest.Uuid(d.EventID), EventSeq: d.EventSeq,
		Attempt: d.Attempt, Status: rest.WebhookDeliveryStatus(d.Status), CreatedAt: millis(d.CreatedAt),
	}
	if d.EventType != "" {
		t := d.EventType
		out.EventType = &t
	}
	if d.ResponseCode != 0 {
		c := d.ResponseCode
		out.ResponseCode = &c
	}
	if d.LastError != "" {
		e := d.LastError
		out.LastError = &e
	}
	if !d.NextAttemptAt.IsZero() {
		at := millis(d.NextAttemptAt)
		out.NextAttemptAt = &at
	}
	if !d.DeliveredAt.IsZero() {
		at := millis(d.DeliveredAt)
		out.DeliveredAt = &at
	}
	return out
}

// millis is t in UTC at millisecond precision (schema Timestamp).
func millis(t time.Time) time.Time { return t.UTC().Truncate(time.Millisecond) }
