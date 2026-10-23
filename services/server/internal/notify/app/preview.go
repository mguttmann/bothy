// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"errors"
	"log/slog"
	"strings"

	"github.com/OWNER/bothy/services/server/internal/files"
	"github.com/OWNER/bothy/services/server/internal/identity"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/reqmeta"
	"github.com/OWNER/bothy/services/server/internal/notify"
	"github.com/OWNER/bothy/services/server/internal/notify/domain"
)

// The challenges of a refused preview credential (06 section 3.1, RFC 9449).
const (
	headerWWWAuthenticate = "WWW-Authenticate"
	headerDPoPNonce       = "DPoP-Nonce"
	challengeDPoP         = `DPoP algs="` + identity.DPoPAlgs + `"`
	challengeInvalidToken = `DPoP error="invalid_token", algs="` + identity.DPoPAlgs + `"`
	schemeDPoP            = "dpop"
	pointerKind           = "#query/kind"
)

// msgImageTicket is the Warn line of an image ticket of a full preview that failed for another
// reason than a missing or unready thumbnail; it never names the file or the person.
const msgImageTicket = "push preview without its image: the fetch ticket failed"

// logKeyErrorKind names the kind of the failure in msgImageTicket.
const logKeyErrorKind = "error.type"

// Preview implements notify.PushRegistrations (13 section 6.4; brief M3-22 design notes 8 and 9),
// in this order; the first failure ends the request:
//
//  1. kind must point at a message (domain.PreviewKindAllowed), otherwise 400 at #query/kind;
//  2. exactly one Authorization value with the scheme DPoP, otherwise 401 unauthenticated;
//  3. the form, kind pvw and checksum of the token (Tokens.HashPushPreviewToken) without a
//     database call, then the active registration of its HMAC: otherwise 401 unauthenticated with
//     the invalid_token challenge; from here on bothy.push.fetch counts, whatever the answer;
//  4. the actor of the registration's session (identity.Auth.SessionActor): another device is 401
//     unauthenticated, a session without an active key family 401 session_revoked;
//  5. the DPoP proof against the key of that session, ath over the preview token (a DPoPError
//     answers 401 with its code and DPoP-Nonce; a stale nonce adds DPoP-Nonce to the answer);
//  6. the message (unknown, deleted, run or system: 404 without authorize), then message.read on
//     it and push.deliver on its thread with the session actor in normal mode: an unauthenticated
//     decision is 401 session_revoked, every other denial 404, identical to an unknown message;
//  7. the badge and the fields of notify.preview_mode; the image of a full preview is a fetch
//     ticket of its small thumbnail issued for the session actor, left out on any error.
func (r *Registrations) Preview(ctx context.Context, req notify.PreviewRequest) (notify.PreviewView, error) {
	if !domain.PreviewKindAllowed(req.Kind) {
		return notify.PreviewView{}, errs.New(errs.CodeValidationFailed, "the kind has no preview").WithFields(
			errs.FieldError{Pointer: pointerKind, Code: domain.FieldInvalid, Detail: "only kinds that point at a message have a preview"})
	}
	raw, ok := previewToken(req.Credentials.Authorization)
	if !ok {
		return notify.PreviewView{}, errs.New(errs.CodeUnauthenticated, "").WithHeader(headerWWWAuthenticate, challengeDPoP)
	}
	hash, err := r.d.Tokens.HashPushPreviewToken(ctx, raw)
	if errors.Is(err, identity.ErrTokenMalformed) {
		return notify.PreviewView{}, invalidPreviewToken()
	}
	if err != nil {
		return notify.PreviewView{}, err
	}
	reg, found, err := r.d.Store.ByPreviewHash(ctx, hash)
	if err != nil {
		return notify.PreviewView{}, err
	}
	if !found {
		return notify.PreviewView{}, invalidPreviewToken()
	}
	r.d.Metrics.Fetch(ctx, reg.Provider)
	sa, err := r.sessionActor(ctx, req.Credentials, raw, reg)
	if err != nil {
		return notify.PreviewView{}, err
	}
	return r.preview(ctx, sa, req)
}

// sessionActor runs steps 4 and 5 of Preview and returns the actor of the registration's session.
func (r *Registrations) sessionActor(ctx context.Context, creds actor.Credentials, raw string, reg PreviewRegistration) (actor.Actor, error) {
	sa, err := r.d.Sessions.SessionActor(ctx, reg.UserID, reg.SessionID)
	if err != nil {
		return actor.Actor{}, err
	}
	if sa.DeviceID != reg.DeviceID {
		return actor.Actor{}, errs.New(errs.CodeUnauthenticated, "").WithHeader(headerWWWAuthenticate, challengeDPoP)
	}
	if sa.KeyThumbprint == "" {
		// No active refresh family: the session was revoked, so no key can prove it any more.
		return actor.Actor{}, errs.New(errs.CodeSessionRevoked, "")
	}
	proofs := creds.DPoPProofs
	if proofs == nil && creds.DPoP != "" {
		proofs = []string{creds.DPoP}
	}
	proof, err := r.d.DPoP.Verify(ctx, identity.ProofRequest{
		Proofs: proofs, Method: creds.Method, URL: creds.HTU, AccessToken: raw, ExpectedJKT: sa.KeyThumbprint,
	})
	if err != nil {
		return actor.Actor{}, err
	}
	if proof.NonceStale {
		reqmeta.SetResponseHeader(ctx, headerDPoPNonce, r.d.DPoP.CurrentNonce())
	}
	return sa, nil
}

// preview runs steps 6 and 7 of Preview for the session actor sa.
func (r *Registrations) preview(ctx context.Context, sa actor.Actor, req notify.PreviewRequest) (notify.PreviewView, error) {
	m, found, err := r.d.Builder.Message(ctx, req.MessageID)
	if err != nil {
		return notify.PreviewView{}, err
	}
	if !found {
		return notify.PreviewView{}, notFound()
	}
	msg := authorize.Resource{Kind: authorize.ResourceMessage, ID: m.ID, ThreadID: m.ThreadID, OwnerID: m.AuthorID}
	thread := authorize.Resource{Kind: authorize.ResourceThread, ID: m.ThreadID}
	for _, c := range []struct {
		act authorize.Action
		res authorize.Resource
	}{{authorize.ActionMessageRead, msg}, {authorize.ActionPushDeliver, thread}} {
		d, err := r.d.Authorizer.Authorize(ctx, sa, c.act, c.res)
		if err != nil {
			return notify.PreviewView{}, err
		}
		if d.Outcome == authorize.OutcomeUnauthenticated {
			return notify.PreviewView{}, errs.New(errs.CodeSessionRevoked, "")
		}
		if d.Outcome != authorize.OutcomeAllow {
			// Every other denial answers like an unknown message (13 6.4, P3).
			return notify.PreviewView{}, notFound()
		}
	}
	mode, err := r.d.Builder.Mode(ctx, sa.ID)
	if err != nil {
		return notify.PreviewView{}, err
	}
	badge, err := r.d.Badge.For(ctx, sa.ID)
	if err != nil {
		return notify.PreviewView{}, err
	}
	f, err := r.d.Builder.Fields(ctx, mode, m)
	if err != nil {
		return notify.PreviewView{}, err
	}
	out := notify.PreviewView{
		Kind: req.Kind, ThreadID: m.ThreadID, MessageID: m.ID, Badge: badge,
		SpaceName: f.SpaceName, ChannelName: f.ChannelName, ThreadTitle: f.ThreadTitle, SenderDisplayName: f.SenderDisplayName,
		Excerpt: f.Excerpt, AttachmentCount: f.AttachmentCount,
	}
	if mode.ShowsContent() && !f.FirstImage.IsZero() {
		out.ImageURL = r.imageURL(actor.WithActor(ctx, sa), f)
	}
	return out, nil
}

// imageURL returns the URL of a fetch ticket of the small thumbnail of the first image, or "" on
// any error of IssueTicket (not ready, blocked, no small thumbnail yet): the image never fails the
// preview (design note 9 c).
func (r *Registrations) imageURL(ctx context.Context, f PreviewFields) string {
	t, err := r.d.Tickets.IssueTicket(ctx, f.FirstImage, files.VariantSmall)
	if err != nil {
		if k := errs.KindOf(err); k != errs.KindNotFound && k != errs.KindConflict {
			r.d.Log.LogAttrs(ctx, slog.LevelWarn, msgImageTicket, slog.String(logKeyErrorKind, k.String()))
		}
		return ""
	}
	return t.URL
}

// previewToken returns the token of exactly one Authorization value with the scheme DPoP (the
// scheme without regard to case, RFC 9110 11.1); several values are joined with a comma by the
// chain and refused.
func previewToken(authorization string) (string, bool) {
	if strings.Contains(authorization, ",") {
		return "", false
	}
	scheme, token, ok := strings.Cut(authorization, " ")
	if !ok || !strings.EqualFold(scheme, schemeDPoP) || token == "" || strings.ContainsAny(token, " \t") {
		return "", false
	}
	return token, true
}

// invalidPreviewToken is the 401 of a malformed, unknown or deactivated preview token.
func invalidPreviewToken() error {
	return errs.New(errs.CodeUnauthenticated, "").WithHeader(headerWWWAuthenticate, challengeInvalidToken)
}
