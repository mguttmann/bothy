// SPDX-License-Identifier: Apache-2.0

package httpapi

import (
	"context"
	"net/http"
	"strings"

	"github.com/OWNER/bothy/services/server/internal/apps"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/httpserver"
)

// ActionWebhookInPost is the action of the two post operations; the token check acts only on
// operations with it.
const ActionWebhookInPost = "webhook_in.post"

// OperationPostWithToken is the post operation whose token is the last path segment; the other
// post operation (postIncomingWebhookWithBearer) takes it from Authorization.
const OperationPostWithToken = "postIncomingWebhook"

// pathWebhookID and pathToken are the path parameters of the post operations.
const (
	pathWebhookID = "webhook_id"
	pathToken     = "token"
)

// CredentialGuard reserves one token of rl.public_auth for the client IP of r before a credential
// is checked: while the bucket is empty it returns 429 rate_limited and the credential must not be
// checked; otherwise settle(failed) must be called exactly once (ratelimit.Limiter
// FailedCredentialGuard, 02 section 9.1).
type CredentialGuard func(r *http.Request) (settle func(failed bool), err error)

// verifiedKey is the context key of the webhook id the token check verified.
type verifiedKey struct{}

// VerifiedWebhook returns the id of the incoming webhook whose token the stage verified for this
// request; ok is false when no check passed.
func VerifiedWebhook(ctx context.Context) (ids.ID, bool) {
	id, ok := ctx.Value(verifiedKey{}).(ids.ID)
	return id, ok
}

// TokenStage returns the middleware of httpserver.StageWebhookToken (M2-27 design note 2): for an
// operation with action webhook_in.post it checks the token of the incoming webhook after the
// rate limit and before any body is read (11 section 8.4). postIncomingWebhook takes the path
// value token and ignores Authorization; postIncomingWebhookWithBearer takes exactly
// "Authorization: Bearer <token>", and a missing or other scheme counts as a wrong token. A failed
// check answers the 404 of an unknown webhook and keeps one token of rl.public_auth of the client
// IP through guard; while that bucket is empty the stage answers 429 rate_limited without checking
// the token. A failed check writes no audit entry. Success replaces the anonymous actor with the
// webhook actor of AuthenticateIncoming and stores the verified webhook id for the handler.
// Operations with another action pass through. The token is never logged.
func TokenStage(hooks apps.IncomingWebhooks, guard CredentialGuard) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			op, _ := httpserver.OperationFrom(r.Context())
			if op.Action != ActionWebhookInPost {
				next.ServeHTTP(w, r)
				return
			}
			settle, err := guard(r)
			if err != nil {
				httpserver.WriteProblem(w, r, err)
				return
			}
			a, id, err := check(r, op, hooks)
			if err != nil {
				settle(errs.KindOf(err) == errs.KindNotFound)
				httpserver.WriteProblem(w, r, err)
				return
			}
			settle(false)
			ctx := context.WithValue(actor.WithActor(r.Context(), a), verifiedKey{}, id)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// check reads the webhook id and the token of r and verifies them.
func check(r *http.Request, op httpserver.Operation, hooks apps.IncomingWebhooks) (actor.Actor, ids.ID, error) {
	id, err := ids.Parse(r.PathValue(pathWebhookID))
	token := tokenOf(r, op)
	if err != nil || token == "" {
		return actor.Actor{}, ids.ID{}, errs.New(errs.CodeNotFound, "")
	}
	a, err := hooks.AuthenticateIncoming(r.Context(), id, token)
	return a, id, err
}

// tokenOf returns the token of the call, "" when the request carries none in its place.
func tokenOf(r *http.Request, op httpserver.Operation) string {
	if op.ID == OperationPostWithToken {
		return r.PathValue(pathToken)
	}
	values := r.Header.Values("Authorization")
	if len(values) != 1 {
		return ""
	}
	token, ok := strings.CutPrefix(values[0], "Bearer ")
	if !ok || token == "" || strings.ContainsAny(token, " \t,") {
		return ""
	}
	return token
}
