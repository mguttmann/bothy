// SPDX-License-Identifier: Apache-2.0

package ratelimit

import (
	"net/http"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/reqmeta"
	"github.com/OWNER/bothy/services/server/internal/platform/httpserver"
)

// ClassPublicAuth is the class that failed credentials count against (05, 02 "Chain in
// the api node").
const ClassPublicAuth Class = "rl.public_auth"

// Install puts l into chain: Middleware(l) as httpserver.StageRateLimit, and the failed-credentials
// rule of 02 "Chain in the api node" in StageAuthenticate (SetCredentialsPrecheck). Call it once per
// chain, before the first route is mounted; it panics like Chain.Use otherwise.
func Install(chain *httpserver.Chain, l *Limiter) {
	chain.Use(httpserver.StageRateLimit, Middleware(l))
	chain.SetCredentialsPrecheck(l.credentialsPrecheck)
}

// Middleware returns the middleware of httpserver.StageRateLimit. It reads the operation of
// StageRoute, derives the key with RequestKey, takes a token from the bucket of the operation's
// class and sets RateLimit-Policy and RateLimit before calling the next handler, so every
// response of the route after the decision carries them, successes and errors alike. An
// exhausted bucket answers 429 rate_limited with Retry-After as a problem and the next handler
// never runs; because the stage runs before authorize, the answer is the same for every resource
// ID. A class the limiter does not know, a gateway class or a missing key fails closed with
// 500 internal_error (the start check of internal/wire rules these out); a webhook_id that is not
// an ID is 404 not_found. These errors come before the decision and carry no RateLimit headers.
// Rate-limit rejections are not audited (05 audits authorize decisions).
func Middleware(l *Limiter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			op, _ := httpserver.OperationFrom(r.Context())
			class := Class(op.RateLimitClass)
			key, err := RequestKey(r, class)
			if err != nil {
				httpserver.WriteProblem(w, r, err)
				return
			}
			d, err := l.Allow(class, key)
			if err != nil {
				httpserver.WriteProblem(w, r, err)
				return
			}
			policy, state := HeaderValues(class, d)
			w.Header().Set(HeaderPolicy, policy)
			w.Header().Set(HeaderState, state)
			if !d.Allowed {
				httpserver.WriteProblem(w, r, rateLimited(class, d))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// credentialsPrecheck is the failed-credentials rule of 02 "Chain in the api node": before the
// credentials of r are verified it reserves one token of rl.public_auth of the client IP. While
// the bucket is empty the answer is 429 rate_limited and nothing is verified. Reserving instead of
// looking keeps concurrent requests from all passing on the same token, so at most burst
// verifications of one client IP run per refill (red team M1-19, finding 1). The returned settle
// function keeps the token for credentials that failed with 401 other than token_expired and
// use_dpop_nonce, and gives it back otherwise: valid credentials and the normal flows of a valid
// client cost nothing, and an error of another kind (the identity store is down) is not the
// client's failure.
func (l *Limiter) credentialsPrecheck(r *http.Request) (func(error), error) {
	ip, _ := reqmeta.ClientIP(r.Context())
	key := KeyForIP(ip)
	d, err := l.Allow(ClassPublicAuth, key)
	if err != nil {
		return nil, err
	}
	if !d.Allowed {
		return nil, rateLimited(ClassPublicAuth, d)
	}
	return func(authErr error) {
		if !countsAsFailure(authErr) {
			_ = l.Refund(ClassPublicAuth, key)
		}
	}, nil
}

// countsAsFailure reports whether err is a credential failure that counts against rl.public_auth:
// 401 other than token_expired and use_dpop_nonce.
func countsAsFailure(err error) bool {
	e, ok := errs.As(err)
	if !ok || e.Kind() != errs.KindUnauthenticated {
		return false
	}
	code := e.PublicCode()
	return code != errs.CodeTokenExpired && code != errs.CodeUseDPoPNonce
}

// rateLimited is 429 rate_limited with Retry-After and the RateLimit headers of class.
func rateLimited(class Class, d Decision) *errs.Error {
	policy, state := HeaderValues(class, d)
	return errs.New(errs.CodeRateLimited, "").
		WithRetryAfter(time.Duration(d.RetryAfterSeconds)*time.Second).
		WithHeader(HeaderPolicy, policy).
		WithHeader(HeaderState, state)
}
