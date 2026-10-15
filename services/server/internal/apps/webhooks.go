// SPDX-License-Identifier: Apache-2.0

package apps

import (
	"context"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// The states of an outgoing subscription (webhook_subscriptions.state, 03 section 4.11).
const (
	SubscriptionActive   = "active"
	SubscriptionPaused   = "paused"
	SubscriptionDisabled = "disabled"
)

// The statuses of a delivery (webhook_deliveries.status, docs/DECISIONS.md D-M2-26-6): pending
// waits for its first or next attempt; succeeded got a 2xx; failed ended with 410 (terminal, the
// owner may redeliver); dead ended after webhook.retry_max_age or without a right, a subscription
// or its event.
const (
	DeliveryPending   = "pending"
	DeliverySucceeded = "succeeded"
	DeliveryFailed    = "failed"
	DeliveryDead      = "dead"
)

// The last_error codes of a delivery (docs/DECISIONS.md D-M2-26-6), never a response body. The
// domain of apps holds the same list (TestLastErrorCodesMatchDomain).
const (
	LastErrorTimeout          = "timeout"
	LastErrorConnectionFailed = "connection_failed"
	LastErrorTLSFailed        = "tls_failed"
	LastErrorAddressBlocked   = "address_blocked"
	LastErrorHostNotAllowed   = "host_not_allowed"
	LastErrorHTTPStatus       = "http_status"
	LastErrorThrottled        = "throttled"
	LastErrorGone             = "gone"
	LastErrorNotAuthorized    = "not_authorized"
	LastErrorNotSubscribed    = "not_subscribed"
	LastErrorEventExpired     = "event_expired"
)

// LastErrorCodes returns the closed list of last_error codes in a fresh slice.
func LastErrorCodes() []string {
	return []string{LastErrorTimeout, LastErrorConnectionFailed, LastErrorTLSFailed, LastErrorAddressBlocked,
		LastErrorHostNotAllowed, LastErrorHTTPStatus, LastErrorThrottled, LastErrorGone, LastErrorNotAuthorized,
		LastErrorNotSubscribed, LastErrorEventExpired}
}

// Subscription is an outgoing subscription as the API shows it (schema WebhookSubscription).
type Subscription struct {
	ID, AppID ids.ID
	// URL is the address; it appears in this response only, never in logs or the audit.
	URL string
	// EventTypes are the subscribed event types in the stored order.
	EventTypes []string
	// State is SubscriptionActive, SubscriptionPaused or SubscriptionDisabled.
	State               string
	ConsecutiveFailures int
	// SignatureSchemes is [v1a] or [v1a, v1].
	SignatureSchemes []string
	// SigningKeyKID names the key that signed the last accepted delivery, at creation the active
	// key (docs/DECISIONS.md D-M2-26-4).
	SigningKeyKID string
	CreatedAt     time.Time
	// DisabledAt is zero while the subscription was never disabled.
	DisabledAt time.Time
}

// SubscriptionCreated is the answer of createWebhookSubscription and updateWebhookSubscription
// (schema WebhookSubscriptionCreated): the subscription and, after a creation with HMAC or a
// rotation, the v1 secret shown once (whsec_); the caller marks the response as a one-time
// secret (R-API-26).
type SubscriptionCreated struct {
	Subscription Subscription
	// HMACSecret is empty when the response carries no secret.
	HMACSecret string
}

// CreateSubscription is the body of createWebhookSubscription (schema WebhookSubscriptionCreate).
type CreateSubscription struct {
	AppID      ids.ID
	URL        string
	EventTypes []string
	EnableHMAC bool
}

// UpdateSubscription is the body of updateWebhookSubscription (schema WebhookSubscriptionUpdate);
// nil members are absent and change nothing.
type UpdateSubscription struct {
	ID         ids.ID
	URL        *string
	EventTypes *[]string
	// State is SubscriptionActive (resume, re-enable) or SubscriptionPaused.
	State *string
	// RotateHMACSecret raises hmac_generation and returns the new secret once.
	RotateHMACSecret bool
}

// Delivery is one delivery as the API shows it (schema WebhookDelivery).
type Delivery struct {
	ID, SubscriptionID, EventID ids.ID
	EventSeq                    int64
	// EventType is empty once the event left the event log (sync.event_retention): it is read from
	// the log, not stored (docs/DECISIONS.md D-M2-26-13).
	EventType string
	Attempt   int
	Status    string
	// ResponseCode is 0 when no response arrived.
	ResponseCode int
	// LastError is a code of LastErrorCodes or empty.
	LastError string
	// NextAttemptAt, CreatedAt and DeliveredAt; zero where the column is NULL.
	NextAttemptAt, CreatedAt, DeliveredAt time.Time
}

// DeliveryQuery selects one page of listWebhookDeliveries, newest first ((event_seq, id)
// descending, docs/DECISIONS.md D-M2-26-20).
type DeliveryQuery struct {
	SubscriptionID ids.ID
	// Status filters by status; empty for all.
	Status string
	Cursor string
	Limit  int
}

// DeliveryPage is one page of deliveries; NextCursor is empty on the last page.
type DeliveryPage struct {
	Items      []Delivery
	NextCursor string
}

// Webhooks is the incoming port of the outgoing webhooks of the module apps (M2-26, 05 section
// 4.11): the subscription operations of an owner and the redelivery. Every method takes the caller
// and authorizes it first: webhook_out.manage (owner, step-up, audited) for create, update and
// delete, webhook_out.read (owner) for the lists, webhook_out.redeliver (owner, audited) for a
// redelivery. A subscription or delivery that is unknown or belongs to an app the caller cannot
// see answers the same 404. Errors are kernel/errs errors: 400 validation_failed,
// webhook_host_not_allowed or cursor_invalid, 401 step_up_required, 403 forbidden, 404 not_found,
// 409 conflict (reasons app_not_installed and event_expired).
type Webhooks interface {
	// CreateSubscription subscribes an installed app (active or suspended) to event types.
	CreateSubscription(ctx context.Context, a actor.Actor, cmd CreateSubscription) (SubscriptionCreated, error)
	// UpdateSubscription changes address or event types, pauses, resumes, re-enables or rotates
	// the HMAC secret.
	UpdateSubscription(ctx context.Context, a actor.Actor, cmd UpdateSubscription) (SubscriptionCreated, error)
	// DeleteSubscription deletes a subscription with its deliveries.
	DeleteSubscription(ctx context.Context, a actor.Actor, id ids.ID) error
	// ListSubscriptions returns every subscription of every installation of the app, ordered by
	// created_at, then id.
	ListSubscriptions(ctx context.Context, a actor.Actor, appID ids.ID) ([]Subscription, error)
	// ListDeliveries returns one page of the deliveries of a subscription.
	ListDeliveries(ctx context.Context, a actor.Actor, q DeliveryQuery) (DeliveryPage, error)
	// Redeliver makes a delivery due again with the same webhook-id.
	Redeliver(ctx context.Context, a actor.Actor, deliveryID ids.ID) (Delivery, error)
	// ResumeInstallation makes the head of every active subscription of the installation due now
	// and schedules its delivery, in the caller's unit of work (app.resume, design note 14).
	ResumeInstallation(ctx context.Context, installationID ids.ID) error
}
