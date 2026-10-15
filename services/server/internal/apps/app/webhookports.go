// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"time"

	"github.com/OWNER/bothy/services/server/internal/apps/domain"
	"github.com/OWNER/bothy/services/server/internal/instance"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// SubscriptionRecord is a row of webhook_subscriptions with the app of its installation.
type SubscriptionRecord struct {
	ID, InstallationID, AppID ids.ID
	URL                       string
	EventTypes                []string
	SigningKeyKID             string
	HMACEnabled               bool
	HMACGeneration            int
	// HMACRotatedAt is zero while the secret was never rotated.
	HMACRotatedAt       time.Time
	State               domain.SubscriptionState
	ConsecutiveFailures int
	LastDeliveredSeq    int64
	CreatedBy           ids.ID
	CreatedAt           time.Time
	// DisabledAt is zero while the subscription is not disabled.
	DisabledAt time.Time
}

// DeliveryRecord is a row of webhook_deliveries; zero values stand for NULL.
type DeliveryRecord struct {
	ID, SubscriptionID, EventID ids.ID
	EventSeq                    int64
	Attempt                     int
	Status                      string
	ResponseCode                int
	LastError                   string
	NextAttemptAt, CreatedAt    time.Time
	DeliveredAt                 time.Time
}

// Candidate is a subscription a consumer round may create rows for: an app subscription in state
// active or paused whose installation is active or suspended (M2-26 design note 3 b).
type Candidate struct {
	SubscriptionID, InstallationID, AppID ids.ID
	EventTypes                            []string
	State                                 domain.SubscriptionState
	InstallationState                     domain.State
	// InstallationEvents are the event_subscriptions of the installation.
	InstallationEvents []string
}

// WebhookStore is the outgoing port of the webhook use cases over webhook_subscriptions and
// webhook_deliveries (adapters/postgres). Reads run in the ambient transaction of ctx or on the
// pool; writes and locks only inside a unit of work (else internal). Lock order: installation,
// subscription, deliveries. Errors are kernel/errs errors without SQL.
type WebhookStore interface {
	// InsertSubscription inserts r (state active, generation 0) and returns it with the times of
	// the database.
	InsertSubscription(ctx context.Context, r SubscriptionRecord) (SubscriptionRecord, error)
	// Subscription returns the app subscription id; found is false for an unknown id.
	Subscription(ctx context.Context, id ids.ID) (SubscriptionRecord, bool, error)
	// LockSubscription is Subscription with FOR UPDATE.
	LockSubscription(ctx context.Context, id ids.ID) (SubscriptionRecord, bool, error)
	// UpdateSubscription writes the mutable columns of r (url, event types, kid, generation,
	// rotation time, state, failures, last delivered seq, disabled_at).
	UpdateSubscription(ctx context.Context, r SubscriptionRecord) error
	// DeleteSubscription deletes id with its deliveries; found is false when no row existed.
	DeleteSubscription(ctx context.Context, id ids.ID) (bool, error)
	// AppSubscriptions returns the subscriptions of every installation of appID, ordered by
	// created_at, then id.
	AppSubscriptions(ctx context.Context, appID ids.ID) ([]SubscriptionRecord, error)
	// LockInstallationSubscriptions returns the subscriptions of an installation FOR UPDATE.
	LockInstallationSubscriptions(ctx context.Context, installationID ids.ID) ([]SubscriptionRecord, error)
	// LockLiveSubscriptions returns every app subscription in state active or paused whose
	// installation is active or suspended, FOR UPDATE.
	LockLiveSubscriptions(ctx context.Context) ([]SubscriptionRecord, error)
	// ShareLiveInstallation returns the installation of appID in state active or suspended, FOR
	// SHARE.
	ShareLiveInstallation(ctx context.Context, appID ids.ID) (InstallationRecord, bool, error)
	// Installation returns the installation id.
	Installation(ctx context.Context, id ids.ID) (InstallationRecord, bool, error)
	// Candidates returns the candidates of a consumer round.
	Candidates(ctx context.Context) ([]Candidate, error)
	// InsertDelivery inserts a pending row with created_at of d unless one exists for subscription
	// and seq; inserted reports a new row.
	InsertDelivery(ctx context.Context, d DeliveryRecord) (inserted bool, err error)
	// CountPending counts the pending rows of a subscription up to bound.
	CountPending(ctx context.Context, subscriptionID ids.ID, bound int) (int, error)
	// Head returns the pending row of the subscription with the lowest event_seq.
	Head(ctx context.Context, subscriptionID ids.ID) (DeliveryRecord, bool, error)
	// Delivery returns the delivery id.
	Delivery(ctx context.Context, id ids.ID) (DeliveryRecord, bool, error)
	// UpdateDelivery writes attempt, status, response code, last error, next attempt, created_at
	// and delivered_at of d.
	UpdateDelivery(ctx context.Context, d DeliveryRecord) error
	// ExpirePending makes the pending rows of a subscription below belowSeq dead with
	// event_expired and returns their seqs.
	ExpirePending(ctx context.Context, subscriptionID ids.ID, belowSeq int64) ([]int64, error)
	// Deliveries returns up to limit rows of a subscription, newest first after the keyset
	// position (afterSeq, afterID) when hasAfter, filtered by status when it is not empty.
	Deliveries(ctx context.Context, subscriptionID ids.ID, status string, hasAfter bool, afterSeq int64, afterID ids.ID, limit int) ([]DeliveryRecord, error)
}

// DeliveryScheduler inserts the job apps.webhook_deliver of a subscription in the ambient unit of
// work (adapters/jobs over jobs.Inserter.InsertTx); the job runs at or after at.
type DeliveryScheduler interface {
	Schedule(ctx context.Context, subscriptionID ids.ID, at time.Time) error
}

// SignedRequest is one prepared delivery: the address, the headers of Standard Webhooks and the
// body as signed. It lives in memory only; nothing of it is logged except the host.
type SignedRequest struct {
	// URL is the subscription address; Host its canonical host (the only part for logs and
	// spans).
	URL, Host string
	// WebhookID is the event ID; Timestamp the Unix seconds of this attempt; Signature the value
	// of webhook-signature.
	WebhookID string
	Timestamp int64
	Signature string
	// Body is the envelope as sent.
	Body []byte
	// SubscriptionID, EventSeq and EventType name the delivery in logs and spans.
	SubscriptionID ids.ID
	EventSeq       int64
	EventType      string
}

// SendResult is the answer of the receiver: its status code and the Retry-After header; the
// body is never read.
type SendResult struct {
	StatusCode int
	RetryAfter string
}

// WebhookSender sends one delivery through the egress client (adapters/webhookhttp), outside any
// transaction (02 section 8.5). The error is a transport error (refusal of the egress client,
// timeout, TLS, connection): no response arrived; a refusal is converted to *domain.BlockedError.
type WebhookSender interface {
	Send(ctx context.Context, req SignedRequest) (SendResult, error)
}

// HMACMasterKey returns the master key of the derived v1 secrets (webhook.hmac_master_key_ref,
// adapters/hmackey). Errors never contain the key.
type HMACMasterKey interface {
	Key(ctx context.Context) ([]byte, error)
}

// Signers is the part of instance.Keys the deliveries use: the signing set of webhook_out.
type Signers interface {
	Signers(ctx context.Context, p instance.Purpose) ([]instance.Signer, error)
	Signer(ctx context.Context, p instance.Purpose) (instance.Signer, error)
}
