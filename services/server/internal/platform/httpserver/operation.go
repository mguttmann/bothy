// SPDX-License-Identifier: Apache-2.0

package httpserver

import (
	"context"
	"net/http"
	"slices"
	"strings"
)

// BodyKind is the media type of an operation's request body (06 "Basic rules", row
// "Medientypen"). It decides the body limit, the deadline and the media type StageBody accepts.
type BodyKind int

// Request body kinds. The zero value is BodyNone.
const (
	BodyNone        BodyKind = iota // no request body
	BodyJSON                        // application/json
	BodyMultipart                   // multipart/form-data
	BodyOctetStream                 // application/octet-stream
	BodyReport                      // application/reports+json or application/csp-report
)

// reportMediaTypes are the media types of BodyReport: the Reporting API format and the legacy
// report-uri format of a Content Security Policy violation report (submitCspReport,
// docs/spec/19-security.md 5 rule 6). Both carry JSON, so the JSON guard and the JSON limit
// apply.
var reportMediaTypes = []string{"application/reports+json", "application/csp-report"}

// mediaType returns the media type of the kind, "" for BodyNone.
func (k BodyKind) mediaType() string {
	switch k {
	case BodyNone:
		return ""
	case BodyJSON:
		return mediaJSON
	case BodyMultipart:
		return "multipart/form-data"
	case BodyOctetStream:
		return "application/octet-stream"
	case BodyReport:
		return strings.Join(reportMediaTypes, " or ")
	}
	return ""
}

// accepts reports whether media (without parameters) is a media type of the kind.
func (k BodyKind) accepts(media string) bool {
	if k == BodyReport {
		return slices.Contains(reportMediaTypes, media)
	}
	return k != BodyNone && media == k.mediaType()
}

// json reports whether the kind carries JSON that StageBody reads whole and guards (SEC-IN-2).
func (k BodyKind) json() bool {
	return k == BodyJSON || k == BodyReport
}

// transfer reports whether the kind streams a file, so http.transfer_timeout and
// file.max_upload_bytes apply instead of the JSON limits.
func (k BodyKind) transfer() bool {
	return k == BodyMultipart || k == BodyOctetStream
}

// IfMatchMode says whether an operation takes an If-Match header (06 "Concurrency").
type IfMatchMode int

// If-Match modes. The zero value is IfMatchNone.
const (
	IfMatchNone     IfMatchMode = iota // no If-Match parameter
	IfMatchOptional                    // checked by the use case when present
	IfMatchRequired                    // missing or "*" gives 428 precondition_required
)

// Operation is what the chain needs to know about one REST operation. internal/wire fills it from
// api/routes.Operation, which the generator derives from docs/api/openapi.yaml; the platform
// package keeps its own type because it must not depend on the generated layer (module register,
// docs/DECISIONS.md M1-18/2).
type Operation struct {
	// ID is the operationId, for example getDiscovery.
	ID string
	// Method is the upper case HTTP method.
	Method string
	// Path is the template including the base URL, for example /api/v1/threads/{thread_id}.
	Path string
	// Pattern is the net/http pattern the route is mounted at, "<METHOD> <Path>".
	Pattern string
	// Action is x-authz-action; the proof (StageProof) requires a check of exactly this action.
	Action string
	// RateLimitClass is x-rate-limit-class, for example rl.read; StageRateLimit (M1-19) counts
	// the request against this class.
	RateLimitClass string
	// Public is true for operations with security: [] (anonymous allowed).
	Public bool
	// IdempotencyKey is true when the operation declares the Idempotency-Key header.
	IdempotencyKey bool
	// IfMatch is the If-Match mode.
	IfMatch IfMatchMode
	// RequestBody is the media type of the request body.
	RequestBody BodyKind
	// BodyRequired is requestBody.required.
	BodyRequired bool
	// ETagStatuses are the 2xx statuses whose response declares ETag; a replay of the
	// Idempotency-Key stage derives the ETag of these statuses from version in the stored body.
	ETagStatuses []int
	// Download is true when a success streams a body that is not JSON (an export, a file): the
	// route class of downloads, whose deadline is http.transfer_timeout (02 section 14.2).
	Download bool
	// AuthorizedInBand marks the one route whose handler authorizes its action on the connection
	// it opens instead of in the request: GET /api/v1/gateway (M1-64), which upgrades to a
	// WebSocket and authenticates and authorizes gateway.connect with the identify or resume
	// frame (04 section 6.3, R-SY-01). Chain.Wrap then runs no per-route stage for it: no
	// authentication of the upgrade request, no rate limit, body limit or deadline and no proof
	// of an authorize check for the 101 response; the outer steps 1 to 4 still apply. The
	// registration check counts the route with its action and class (AZ-P1), and a test in
	// internal/wire holds that no other route sets the marker.
	AuthorizedInBand bool
	// BodyLimit, when above zero, replaces http.max_json_body_bytes as the body limit of this
	// operation, and BodyLimitKey is the configuration key a 413 payload_too_large names in its
	// detail (the incoming webhooks use webhook_in.max_body_bytes, M2-27). For such an operation the
	// check of a declared Content-Length runs at the start of StageBody instead of StageRoute, so
	// the stages before StageBody (rate limit, webhook token) answer first (11 8.4: token before
	// size); the body is still never read beyond the limit.
	BodyLimit    int64
	BodyLimitKey string
	// AnonymousCredentials marks an operation whose credential is not an identity credential (the
	// incoming webhook token, M2-27 design note 2): StageAuthenticate neither runs the credentials
	// precheck nor calls the Authenticator and the actor stays anonymous, whatever Authorization
	// holds; a later stage checks the credential itself.
	AnonymousCredentials bool
	// OwnCredential marks an operation whose only security scheme is the preview token of a push
	// registration (getPushPreview, M3-22 design note 12): StageAuthenticate neither runs the
	// credentials precheck nor calls the Authenticator, keeps the actor anonymous and stores the
	// credentials of the request with actor.WithCredentials, so the use case checks the token and
	// its DPoP proof itself. Every other operation still hands such a token to the Authenticator,
	// which refuses it.
	OwnCredential bool
}

// transfer reports whether the route class of op is that of transfers (uploads and downloads):
// the deadline is http.transfer_timeout and it bounds the write as well.
func (op Operation) transfer() bool { return op.Download || op.RequestBody.transfer() }

// Route is one operation with the handler to mount for it; Chain.Wrap puts the per-route stages
// around Handler.
type Route struct {
	Operation Operation
	Handler   http.Handler
}

type operationKey struct{}

// withOperation stores op in ctx (StageRoute).
func withOperation(ctx context.Context, op Operation) context.Context {
	return context.WithValue(ctx, operationKey{}, op)
}

// OperationFrom returns the operation of the request and true inside the per-route stages, or the
// zero Operation and false outside them (an unmatched request, the outer stages).
func OperationFrom(ctx context.Context) (Operation, bool) {
	op, ok := ctx.Value(operationKey{}).(Operation)
	return op, ok
}
