// SPDX-License-Identifier: Apache-2.0

package httpapi

import (
	"context"
	"encoding/json"

	"github.com/OWNER/bothy/services/server/api/rest"
	"github.com/OWNER/bothy/services/server/internal/apps"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/idempotency"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// Incoming implements listIncomingWebhooks, createIncomingWebhook, revokeIncomingWebhook,
// postIncomingWebhook and postIncomingWebhookWithBearer of rest.StrictServerInterface over the
// port apps.IncomingWebhooks (M2-27). The use cases authorize; the handlers convert. The two post
// operations run behind the token check of StageWebhookToken (TokenStage), which put the webhook
// actor and the verified webhook id into the context.
type Incoming struct {
	hooks apps.IncomingWebhooks
}

// NewIncoming returns the handlers over the use cases.
func NewIncoming(h apps.IncomingWebhooks) *Incoming { return &Incoming{hooks: h} }

// ListIncomingWebhooks implements rest.StrictServerInterface (GET
// /channels/{channel_id}/incoming-webhooks).
func (h *Incoming) ListIncomingWebhooks(ctx context.Context, req rest.ListIncomingWebhooksRequestObject) (rest.ListIncomingWebhooksResponseObject, error) {
	list, err := h.hooks.ListIncoming(ctx, principal(ctx), ids.ID(req.ChannelId))
	if err != nil {
		return nil, err
	}
	out := rest.IncomingWebhookList{Items: make([]rest.IncomingWebhook, 0, len(list))}
	for _, w := range list {
		out.Items = append(out.Items, incomingREST(w))
	}
	return rest.ListIncomingWebhooks200JSONResponse{Body: out}, nil
}

// CreateIncomingWebhook implements rest.StrictServerInterface (POST
// /channels/{channel_id}/incoming-webhooks). The answer carries the token once, so it is marked as
// a one-time secret: a replay of the Idempotency-Key answers 409 one_time_secret (06 section 7).
func (h *Incoming) CreateIncomingWebhook(ctx context.Context, req rest.CreateIncomingWebhookRequestObject) (rest.CreateIncomingWebhookResponseObject, error) {
	cmd := apps.CreateIncomingCmd{ChannelID: ids.ID(req.ChannelId)}
	if b := req.Body; b != nil {
		cmd.AppID, cmd.Name = ids.ID(b.AppId), b.Name
		if b.ThreadId != nil {
			t := ids.ID(*b.ThreadId)
			cmd.ThreadID = &t
		}
	}
	res, err := h.hooks.CreateIncoming(ctx, principal(ctx), cmd)
	if err != nil {
		return nil, err
	}
	idempotency.MarkOneTimeSecret(ctx)
	return rest.CreateIncomingWebhook201JSONResponse{Body: rest.IncomingWebhookCreated{Webhook: incomingREST(res.Webhook), Url: res.URL}}, nil
}

// RevokeIncomingWebhook implements rest.StrictServerInterface (DELETE
// /incoming-webhooks/{webhook_id}).
func (h *Incoming) RevokeIncomingWebhook(ctx context.Context, req rest.RevokeIncomingWebhookRequestObject) (rest.RevokeIncomingWebhookResponseObject, error) {
	if err := h.hooks.RevokeIncoming(ctx, principal(ctx), ids.ID(req.WebhookId)); err != nil {
		return nil, err
	}
	return rest.RevokeIncomingWebhook204Response{}, nil
}

// PostIncomingWebhook implements rest.StrictServerInterface (POST
// /hooks/{webhook_id}/{token}).
func (h *Incoming) PostIncomingWebhook(ctx context.Context, req rest.PostIncomingWebhookRequestObject) (rest.PostIncomingWebhookResponseObject, error) {
	res, err := h.post(ctx, ids.ID(req.WebhookId), req.Body)
	if err != nil {
		return nil, err
	}
	body := rest.IncomingWebhookResult{MessageId: rest.Uuid(res.MessageID), ThreadId: rest.Uuid(res.ThreadID)}
	if res.Replay {
		replay := true
		return rest.PostIncomingWebhook200JSONResponse{Body: body, Headers: rest.PostIncomingWebhook200ResponseHeaders{IdempotentReplay: &replay}}, nil
	}
	return rest.PostIncomingWebhook201JSONResponse{Body: body}, nil
}

// PostIncomingWebhookWithBearer implements rest.StrictServerInterface (POST /hooks/{webhook_id}
// with the token in Authorization).
func (h *Incoming) PostIncomingWebhookWithBearer(ctx context.Context, req rest.PostIncomingWebhookWithBearerRequestObject) (rest.PostIncomingWebhookWithBearerResponseObject, error) {
	res, err := h.post(ctx, ids.ID(req.WebhookId), req.Body)
	if err != nil {
		return nil, err
	}
	body := rest.IncomingWebhookResult{MessageId: rest.Uuid(res.MessageID), ThreadId: rest.Uuid(res.ThreadID)}
	if res.Replay {
		replay := true
		return rest.PostIncomingWebhookWithBearer200JSONResponse{Body: body, Headers: rest.PostIncomingWebhookWithBearer200ResponseHeaders{IdempotentReplay: &replay}}, nil
	}
	return rest.PostIncomingWebhookWithBearer201JSONResponse{Body: body}, nil
}

// post runs one call: the path value is used only after it equals the id the token check
// verified (design note 2 d), and the body goes to the use case as the JSON of the members the
// sender sent (the generated type omits absent members).
func (h *Incoming) post(ctx context.Context, webhookID ids.ID, body *rest.IncomingWebhookPost) (apps.IncomingPosted, error) {
	verified, ok := VerifiedWebhook(ctx)
	if !ok || verified != webhookID {
		return apps.IncomingPosted{}, errs.New(errs.CodeNotFound, "")
	}
	raw := []byte("{}")
	if body != nil {
		var err error
		if raw, err = json.Marshal(body); err != nil {
			return apps.IncomingPosted{}, errs.Wrap(errs.CodeInternalError, err, "")
		}
	}
	return h.hooks.PostIncoming(ctx, principal(ctx), apps.PostIncomingCmd{WebhookID: webhookID, Body: raw})
}

// incomingREST converts a webhook; absent values are absent members (no JSON null), never a token.
func incomingREST(w apps.IncomingWebhook) rest.IncomingWebhook {
	out := rest.IncomingWebhook{
		Id: rest.Uuid(w.ID), AppId: rest.Uuid(w.AppID), ChannelId: rest.Uuid(w.ChannelID), Name: w.Name, CreatedAt: millis(w.CreatedAt),
	}
	if !w.ThreadID.IsZero() {
		t := rest.Uuid(w.ThreadID)
		out.ThreadId = &t
	}
	if !w.LastUsedAt.IsZero() {
		t := millis(w.LastUsedAt)
		out.LastUsedAt = &t
	}
	if !w.RevokedAt.IsZero() {
		t := millis(w.RevokedAt)
		out.RevokedAt = &t
	}
	return out
}
