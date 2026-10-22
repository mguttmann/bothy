// SPDX-License-Identifier: Apache-2.0

package apps

import (
	"context"
	"encoding/json"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// IncomingWebhook is an incoming webhook as the API shows it (schema IncomingWebhook): never the
// token, never its hash.
type IncomingWebhook struct {
	ID, AppID, ChannelID ids.ID
	// ThreadID is the fixed thread; zero for none.
	ThreadID ids.ID
	// Name is the name the manager gave it; it never goes into logs or the audit.
	Name      string
	CreatedAt time.Time
	// LastUsedAt and RevokedAt are zero while the column is NULL.
	LastUsedAt, RevokedAt time.Time
}

// CreateIncomingCmd is the body of createIncomingWebhook (schema IncomingWebhookCreate) with the
// channel of the path.
type CreateIncomingCmd struct {
	ChannelID, AppID ids.ID
	// ThreadID is the fixed thread; nil when absent.
	ThreadID *ids.ID
	Name     string
}

// IncomingCreated is the answer of createIncomingWebhook (schema IncomingWebhookCreated): the
// webhook and its URL with the token, shown exactly once. URL is a secret: only the handler writes
// it into the one response, nothing logs it, and the caller marks the response as a one-time
// secret (R-API-26).
type IncomingCreated struct {
	Webhook IncomingWebhook
	URL     string
}

// PostIncomingCmd is one call of postIncomingWebhook or postIncomingWebhookWithBearer: the
// verified webhook and the request body as JSON with the members the caller sent (schema
// IncomingWebhookPost, already checked against the closed schema by the HTTP chain).
type PostIncomingCmd struct {
	WebhookID ids.ID
	Body      json.RawMessage
}

// IncomingPosted is the answer of a post (schema IncomingWebhookResult): the message and the
// thread it landed in, and whether it is the original of an earlier identical native request
// (200 with Idempotent-Replay).
type IncomingPosted struct {
	MessageID, ThreadID ids.ID
	Replay              bool
}

// IncomingWebhooks is the incoming port of the incoming webhooks of the module apps (M2-27, 02
// section 4.3, 11 section 8, 05 section 4.11). The management methods take the caller and
// authorize it first: webhook_in.read and webhook_in.create need manage on the channel (create
// with step-up, audited), webhook_in.revoke manage on the webhook's channel (step-up, audited). An
// unknown channel or webhook and one the caller cannot see answer the same 404. Errors are
// kernel/errs errors: 400 validation_failed (codes app_not_installed, app_cannot_write,
// not_in_channel and those of the formats), 401 step_up_required, 403 forbidden, 404 not_found,
// 413 payload_too_large, 422 idempotency_key_reuse.
type IncomingWebhooks interface {
	// ListIncoming returns the active webhooks of a channel ordered by created_at, then id.
	ListIncoming(ctx context.Context, a actor.Actor, channelID ids.ID) ([]IncomingWebhook, error)
	// CreateIncoming creates a webhook for one channel that writes as its app; the answer carries
	// the URL with the token once, only the HMAC is stored.
	CreateIncoming(ctx context.Context, a actor.Actor, cmd CreateIncomingCmd) (IncomingCreated, error)
	// RevokeIncoming revokes a webhook; a revoked one is without effect (audited with no_change).
	RevokeIncoming(ctx context.Context, a actor.Actor, webhookID ids.ID) error
	// AuthenticateIncoming checks the token of a call to webhookID (design note 2 b) and returns
	// the webhook actor: the app principal with its active installation and the granted scopes
	// limited to messages:write. An unknown id, a wrong or malformed token, a revoked webhook and
	// an installation that is not active all give the same 404 not_found; other errors are
	// internal. It reads at most one row and authorizes nothing.
	AuthenticateIncoming(ctx context.Context, webhookID ids.ID, token string) (actor.Actor, error)
	// PostIncoming writes the message of a verified call for the webhook actor a (design notes 4
	// to 6): format, body rules, target thread, authorize, then the message with origin app, no
	// mentions and no trigger. Every refusal of the app's rights is the 404 of an unknown webhook.
	PostIncoming(ctx context.Context, a actor.Actor, cmd PostIncomingCmd) (IncomingPosted, error)
}
