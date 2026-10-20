// SPDX-License-Identifier: Apache-2.0

package httpserver

import (
	"bytes"
	"context"
	"io"
	"mime"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/reqmeta"
	"github.com/OWNER/bothy/services/server/internal/platform/otel"
)

// CORS values (SEC-CORS-1, SEC-CORS-2, 06 "Grundregeln" row CORS).
const (
	actionDiscovery      = "instance.discovery.read"
	headerACAO           = "Access-Control-Allow-Origin"
	headerACAM           = "Access-Control-Allow-Methods"
	headerACAH           = "Access-Control-Allow-Headers"
	headerACEH           = "Access-Control-Expose-Headers"
	corsExposeHeaders    = "Correlation-Id, ETag, RateLimit, RateLimit-Policy, Retry-After, DPoP-Nonce, Idempotent-Replay"
	corsAllowHeaders     = "Authorization, DPoP, Content-Type, Idempotency-Key, If-Match"
	detailUnexpectedBody = "This operation takes no request body."
)

// corsStage is StageCORS. By default no Access-Control-* header is sent (SEC-CORS-1). Operations
// with the action instance.discovery.read answer with Access-Control-Allow-Origin: * and never
// with credentials (SEC-CORS-2). A request whose Origin equals an entry of
// api.cors_allowed_origins exactly gets that origin, Vary: Origin and the exposed headers.
func (c *Chain) corsStage(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		op, _ := OperationFrom(r.Context())
		h := w.Header()
		switch origin := r.Header.Get(headerOrigin); {
		case op.Action == actionDiscovery:
			h.Set(headerACAO, "*")
		case origin != "" && c.origins[origin]:
			h.Set(headerACAO, origin)
			h.Add(headerVary, headerOrigin)
			h.Set(headerACEH, corsExposeHeaders)
		}
		next.ServeHTTP(w, r)
	})
}

// Preflight returns the handler of OPTIONS <path> for the route template path whose enabled operations use
// methods: 204 with Access-Control-Allow-Methods, -Headers and the origin when the Origin is in
// api.cors_allowed_origins, 204 without any CORS header otherwise. internal/wire mounts it for
// every enabled path, only when api.cors_allowed_origins is not empty. Preflights carry no
// credentials and run no action, so they bypass the per-route stages; the outer steps (headers,
// correlation, recover) still apply.
func (c *Chain) Preflight(path string, methods []string) http.Handler {
	allow := strings.Join(slices.Sorted(slices.Values(methods)), ", ")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		otel.RecordRoute(r, path)
		h := w.Header()
		h.Set(headerCacheControl, valueNoStore)
		h.Set(headerCORP, valueSameOrigin)
		h.Add(headerVary, headerOrigin)
		if origin := r.Header.Get(headerOrigin); origin != "" && c.origins[origin] {
			h.Set(headerACAO, origin)
			h.Set(headerACAM, allow)
			h.Set(headerACAH, corsAllowHeaders)
		}
		w.WriteHeader(http.StatusNoContent)
	})
}

// CORSEnabled reports whether api.cors_allowed_origins names at least one origin, so internal/wire
// mounts the preflight routes.
func (c *Chain) CORSEnabled() bool { return len(c.origins) > 0 }

// authenticateStage is StageAuthenticate: the Authenticator gets the method, the htu built from
// server.public_url and the escaped path (never from the Host header), Authorization, DPoP and the
// client IP; the TCP peer, the header and the route's action go into the context as
// reqmeta.Transport (M1-43), and the console cookie (M6-01). Without credentials it returns the
// anonymous actor; a request the cookie authenticated then passes ConsoleGuard (consoleGuard). Its error becomes the problem
// (after the failed-credentials hook, when one is set). A request with credentials passes the precheck
// of M1-19 first, whose settle function gets the result of the Authenticator. An operation with
// AnonymousCredentials skips the precheck, the Authenticator and ConsoleGuard and stays anonymous.
// The actor, the DPoP facts of the request (reqmeta.Proof) and a setter of response headers go
// into the context.
func (c *Chain) authenticateStage(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip, _ := reqmeta.ClientIP(r.Context())
		creds := actor.Credentials{
			Method:        r.Method,
			HTU:           c.HTU(r),
			Authorization: authorization(r.Header),
			DPoP:          r.Header.Get(headerDPoP),
			DPoPProofs:    r.Header.Values(headerDPoP),
			ClientIP:      ip,
			ConsoleCookie: consoleCookie(r),
		}
		// The DPoP facts go to the handlers of the login and refresh endpoints, and the
		// Authenticator and the use cases set DPoP-Nonce without the response writer (M1-34).
		ctx := reqmeta.WithProof(r.Context(), reqmeta.Proof{Method: creds.Method, HTU: creds.HTU, Values: creds.DPoPProofs})
		// The TCP peer, the header and the action go to the trusted header driver of identity,
		// which must never use a forwarded address (M1-43, 08 section 2.4 rule 2).
		op, _ := OperationFrom(ctx)
		peer, _ := parseHop(r.RemoteAddr)
		ctx = reqmeta.WithTransport(ctx, reqmeta.Transport{Peer: peer, Header: r.Header, Action: op.Action})
		ctx = reqmeta.WithResponseHeaders(ctx, w.Header().Set)
		r = r.WithContext(ctx)
		c.mu.Lock()
		hook, precheck := c.failedCreds, c.precheck
		c.mu.Unlock()
		if op.AnonymousCredentials {
			// The credential of this operation (the incoming webhook token, M2-27) is not one the
			// Authenticator knows; a later stage checks it, and counts its failures itself.
			next.ServeHTTP(w, r.WithContext(actor.WithActor(r.Context(), actor.Anonymous())))
			return
		}
		var settle func(error)
		if precheck != nil && creds.Present() {
			var err error
			if settle, err = precheck(r); err != nil {
				WriteProblem(w, r, err)
				return
			}
		}
		a, err := authenticateSettled(r, c.auth, creds, settle)
		if err != nil {
			if hook != nil {
				err = hook(r, err)
			}
			WriteProblem(w, r, err)
			return
		}
		// ConsoleGuard (M6-01): a request the cookie authenticated passes the CSRF check before
		// any later stage and the handler see it.
		if err := c.consoleGuard(r, creds, a); err != nil {
			WriteProblem(w, r, err)
			return
		}
		// The request span names the principal kind and, for a person, its pseudonym (DP-T09).
		otel.AnnotatePrincipal(r.Context(), string(a.Kind), a.ID.String())
		next.ServeHTTP(w, r.WithContext(actor.WithActor(r.Context(), a)))
	})
}

// authenticateSettled runs the Authenticator and then settle (when not nil) exactly once with its
// error, or with errInactiveCredential for an actor whose API token is revoked or expired. A panic of the Authenticator settles with errAuthenticatorPanic, a server failure that is
// not a failed credential, and then panics on to the recover step, so a bug never keeps a token
// the precheck reserved (M1-19).
func authenticateSettled(r *http.Request, auth actor.Authenticator, creds actor.Credentials, settle func(error)) (a actor.Actor, err error) {
	if settle == nil {
		return auth.Authenticate(r.Context(), creds)
	}
	settled := false
	defer func() {
		if !settled {
			settle(errAuthenticatorPanic)
		}
	}()
	a, err = auth.Authenticate(r.Context(), creds)
	settled = true
	if err == nil && a.CredentialInactive {
		// A revoked or expired API token goes on to authorize, which refuses and audits it
		// (M1-44/5), but for the failed-credentials rule it is a dead credential like any other.
		settle(errInactiveCredential)
	} else {
		settle(err)
	}
	return a, err
}

// errInactiveCredential is what settle sees for an actor with CredentialInactive.
var errInactiveCredential = errs.New(errs.CodeUnauthenticated, "inactive credential")

// errAuthenticatorPanic is what settle sees when the Authenticator panicked.
var errAuthenticatorPanic = errs.New(errs.CodeInternalError, "authenticator panicked")

// authorization returns the Authorization header. Several lines are joined, so a request whose
// first line is empty is never anonymous; the Authenticator refuses the joined value.
func authorization(h http.Header) string {
	values := h.Values(headerAuthorization)
	if len(values) > 1 {
		return strings.Join(values, ", ")
	}
	return h.Get(headerAuthorization)
}

// HTU returns the DPoP htu of r: server.public_url plus the escaped request path, without query
// and fragment (RFC 9449, 06 section 3.1). The Host header of the client never contributes.
func (c *Chain) HTU(r *http.Request) string {
	return c.publicURL.Scheme + "://" + c.publicURL.Host + r.URL.EscapedPath()
}

// bodyStage is StageBody: a declared length above Operation.BodyLimit is 413 (StageRoute left it
// to this stage), a body on an operation without one is 415, a body with another media type than
// the operation's is 415, a missing required body is 400 validation_failed, and a JSON body passes
// the guard of SEC-IN-2 before it is restored for the next stages.
func (c *Chain) bodyStage(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		op, _ := OperationFrom(r.Context())
		if op.BodyLimit > 0 && r.ContentLength > op.BodyLimit {
			WriteProblem(w, r, payloadTooLarge(r.Context()))
			return
		}
		body, err := c.checkBody(r, op)
		if !op.RequestBody.transfer() {
			// The body is read (or refused); the read deadline of StageRoute has done its job.
			_ = http.NewResponseController(w).SetReadDeadline(time.Time{})
		}
		if err != nil {
			WriteProblem(w, r, err)
			return
		}
		if body != nil {
			r = r.WithContext(context.WithValue(r.Context(), rawJSONKey{}, &rawJSON{set: true, body: body}))
		}
		next.ServeHTTP(w, r)
	})
}

// rawJSONKey keys the JSON body of the request in its context (RawJSONBody).
type rawJSONKey struct{}

// rawJSON holds the JSON body bytes StageBody read.
type rawJSON struct {
	set  bool
	body []byte
}

// RawJSONBody returns the bytes of the JSON request body exactly as received, after StageBody
// checked them (SEC-IN-2, SEC-IN-3), and true; false for a request without a JSON body or outside
// the chain. A handler that must keep or hash what the client sent uses it, because the generated
// server hands it only the decoded value: the apps module stores the SHA-256 of a manifest as
// received (docs/spec/11-erweiterungen.md section 3.1, docs/DECISIONS.md D-M2-24-17). The slice
// must not be changed.
func RawJSONBody(ctx context.Context) ([]byte, bool) {
	b, ok := ctx.Value(rawJSONKey{}).(*rawJSON)
	if !ok || !b.set {
		return nil, false
	}
	return b.body, true
}

// checkBody applies the rules of bodyStage to r and returns the problem, or nil, and the JSON body
// it read (nil for another media type or no body).
func (c *Chain) checkBody(r *http.Request, op Operation) ([]byte, error) {
	if r.ContentLength == 0 || r.Body == nil || r.Body == http.NoBody {
		return nil, requiredBody(op)
	}
	if op.RequestBody == BodyNone {
		return nil, errs.New(errs.CodeUnsupportedMediaType, detailUnexpectedBody)
	}
	media, _, err := mime.ParseMediaType(r.Header.Get(headerContentType))
	if err != nil || !op.RequestBody.accepts(media) {
		return nil, errs.New(errs.CodeUnsupportedMediaType, "Expected "+op.RequestBody.mediaType()+".")
	}
	if !op.RequestBody.json() {
		return nil, nil
	}
	return c.guardJSON(r, op)
}

// guardJSON reads the JSON body within the limit of StageRoute (*http.MaxBytesError is 413),
// checks it with CheckJSON, puts the same bytes back as the body and returns them.
func (c *Chain) guardJSON(r *http.Request, op Operation) ([]byte, error) {
	body, err := io.ReadAll(r.Body)
	_ = r.Body.Close()
	if err != nil {
		return nil, err
	}
	if len(body) == 0 {
		return nil, requiredBody(op)
	}
	if err := CheckJSON(body, c.cfg.MaxJSONDepth); err != nil {
		return nil, err
	}
	r.Body = io.NopCloser(bytes.NewReader(body))
	r.ContentLength = int64(len(body))
	r.GetBody = func() (io.ReadCloser, error) { return io.NopCloser(bytes.NewReader(body)), nil }
	return body, nil
}

// requiredBody is 400 validation_failed for an operation whose body is required and missing.
func requiredBody(op Operation) error {
	if op.RequestBody == BodyNone || !op.BodyRequired {
		return nil
	}
	return errs.New(errs.CodeValidationFailed, detailEmptyRequired).
		WithFields(errs.FieldError{Pointer: "", Code: fieldCodeRequired, Detail: detailEmptyRequired})
}

// preconditionsStage is StagePreconditions: an operation with IfMatchRequired answers
// 428 precondition_required without If-Match or with the wildcard *, because the wildcard would
// skip the version check the contract demands (06 "Nebenläufigkeit"; docs/DECISIONS.md M1-18).
func (c *Chain) preconditionsStage(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		op, _ := OperationFrom(r.Context())
		if op.IfMatch == IfMatchRequired {
			if v := strings.TrimSpace(r.Header.Get(headerIfMatch)); v == "" || v == "*" {
				WriteProblem(w, r, errs.PreconditionRequired())
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

// validateStage is StageValidate: path, query and header parameters and a JSON body are checked
// against the contract before the generated wrapper runs (SEC-IN-1, W7).
func (c *Chain) validateStage(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		op, _ := OperationFrom(r.Context())
		if err := c.validator.Validate(r, op); err != nil {
			WriteProblem(w, r, err)
			return
		}
		next.ServeHTTP(w, r)
	})
}
