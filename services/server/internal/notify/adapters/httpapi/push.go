// SPDX-License-Identifier: Apache-2.0

package httpapi

import (
	"context"
	"time"

	"github.com/OWNER/bothy/services/server/api/rest"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/idempotency"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/notify"
)

// Push implements registerPush, unregisterPush and getPushPreview (M3-22) of
// rest.StrictServerInterface over notify.PushRegistrations. The use cases authorize; the handlers
// convert. The preview token leaves the server only in the answer of RegisterPush, which is
// marked as a one-time secret, so a replay with the same Idempotency-Key answers 409 conflict with
// the reason one_time_secret and the idempotency row holds no part of it (06 section 7, R-API-26).
type Push struct {
	regs notify.PushRegistrations
}

// NewPush returns the handlers over the port.
//
// Example:
//
//	h := httpapi.NewPush(registrations)
func NewPush(regs notify.PushRegistrations) *Push { return &Push{regs: regs} }

// caller returns the actor of the request; outside the chain it is the anonymous principal,
// which authorize refuses.
func caller(ctx context.Context) actor.Actor {
	if a, ok := actor.FromContext(ctx); ok {
		return a
	}
	return actor.Anonymous()
}

// RegisterPush implements rest.StrictServerInterface (POST
// /me/devices/current/push-registrations).
func (h *Push) RegisterPush(ctx context.Context, req rest.RegisterPushRequestObject) (rest.RegisterPushResponseObject, error) {
	idempotency.MarkOneTimeSecret(ctx)
	if req.Body == nil {
		return nil, errs.New(errs.CodeValidationFailed, "")
	}
	cmd := notify.RegisterCmd{Provider: string(req.Body.Provider), Token: req.Body.Token, AppID: req.Body.AppId}
	if req.Body.Environment != nil {
		cmd.Environment = string(*req.Body.Environment)
	}
	out, err := h.regs.Register(ctx, caller(ctx), cmd)
	if err != nil {
		return nil, err
	}
	r := out.Registration
	return rest.RegisterPush201JSONResponse{Body: rest.PushRegistrationCreated{
		Id: rest.Uuid(r.ID), DeviceId: rest.Uuid(r.DeviceID), Provider: rest.PushProvider(r.Provider),
		Environment: rest.PushEnvironment(r.Environment), AppId: r.AppID, CreatedAt: ms(r.CreatedAt),
		PreviewToken: out.PreviewToken.Reveal(),
	}}, nil
}

// UnregisterPush implements rest.StrictServerInterface (DELETE
// /push-registrations/{registration_id}).
func (h *Push) UnregisterPush(ctx context.Context, req rest.UnregisterPushRequestObject) (rest.UnregisterPushResponseObject, error) {
	if err := h.regs.Unregister(ctx, caller(ctx), ids.ID(req.RegistrationId)); err != nil {
		return nil, err
	}
	return rest.UnregisterPush204Response{}, nil
}

// GetPushPreview implements rest.StrictServerInterface (GET /push/preview). The chain stored the
// credentials of the request for this route (route flag OwnCredential); without them the request
// is refused like one without a preview token.
func (h *Push) GetPushPreview(ctx context.Context, req rest.GetPushPreviewRequestObject) (rest.GetPushPreviewResponseObject, error) {
	creds, _ := actor.CredentialsFrom(ctx)
	v, err := h.regs.Preview(ctx, notify.PreviewRequest{
		Credentials: creds, NotificationID: ids.ID(req.Params.Nid), MessageID: ids.ID(req.Params.MessageId), Kind: string(req.Params.Kind),
	})
	if err != nil {
		return nil, err
	}
	out := rest.PushPreview{
		Kind: rest.NotificationKind(v.Kind), Badge: int(v.Badge),
		SpaceName: optional(v.SpaceName), ChannelName: optional(v.ChannelName), ThreadTitle: optional(v.ThreadTitle),
		SenderDisplayName: optional(v.SenderDisplayName), Excerpt: optional(v.Excerpt), ImageUrl: optional(v.ImageURL),
	}
	if !v.ThreadID.IsZero() {
		t := rest.Uuid(v.ThreadID)
		out.ThreadId = &t
	}
	if !v.MessageID.IsZero() {
		m := rest.Uuid(v.MessageID)
		out.MessageId = &m
	}
	if v.AttachmentCount > 0 {
		n := v.AttachmentCount
		out.AttachmentCount = &n
	}
	return rest.GetPushPreview200JSONResponse{Body: out}, nil
}

// optional returns nil for "", so an absent member is omitted, never null (ADR 0053).
func optional(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// ms returns t in UTC with millisecond precision, the form of every API timestamp.
func ms(t time.Time) time.Time { return t.UTC().Truncate(time.Millisecond) }
