// SPDX-License-Identifier: Apache-2.0

package actor

import (
	"context"
	"net/netip"
	"slices"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// Kind is the kind of a principal (docs/spec/07-permissions.md section 3).
type Kind string

// The principal kinds of 07 section 3.
const (
	// KindUser is a person.
	KindUser Kind = "user"
	// KindAgent is an agent with an API token.
	KindAgent Kind = "agent"
	// KindApp is an installed app with an API token and scopes.
	KindApp Kind = "app"
	// KindHost is a computer logged in with its host key (24-terminal.md section 4.1).
	KindHost Kind = "host"
	// KindAnonymous is a request without credentials.
	KindAnonymous Kind = "anonymous"
)

// Actor is the checked principal context of a request (07 section 3). The authentication builds
// it; authz trusts only it. Fields that do not apply to the kind stay zero.
type Actor struct {
	// ID is the principal; zero for KindAnonymous.
	ID ids.ID
	// Kind is the principal kind.
	Kind Kind
	// SessionID is auth_sessions.id (people only).
	SessionID ids.ID
	// DeviceID is the device of the session (people only).
	DeviceID ids.ID
	// StepUpAt is the last fresh confirmation of the session; zero when there was none.
	StepUpAt time.Time
	// TokenID is the API token of an agent, app or personal token; zero otherwise.
	TokenID ids.ID
	// Scopes are the scopes of the API token or the granted scopes of an app installation.
	Scopes []string
}

// Anonymous returns the actor of a request without credentials. Anonymous requests still run
// through authorize with this principal (06 section 3, 05 section 2.1).
func Anonymous() Actor {
	return Actor{Kind: KindAnonymous}
}

// IsAnonymous reports whether a is the anonymous principal.
func (a Actor) IsAnonymous() bool {
	return a.Kind == KindAnonymous
}

type actorKey struct{}

// WithActor returns a copy of ctx that carries a as the principal of the request. The HTTP chain
// (stage StageAuthenticate) and the gateway call it after the Authenticator accepted the request.
func WithActor(ctx context.Context, a Actor) context.Context {
	a.Scopes = slices.Clone(a.Scopes)
	return context.WithValue(ctx, actorKey{}, a)
}

// FromContext returns the principal stored with WithActor and true, or the zero Actor and false.
// A use case that finds no actor must deny: it runs outside the chain.
func FromContext(ctx context.Context) (Actor, bool) {
	a, ok := ctx.Value(actorKey{}).(Actor)
	if ok {
		a.Scopes = slices.Clone(a.Scopes)
	}
	return a, ok
}

// Credentials is what the HTTP chain hands to the Authenticator for one request
// (docs/spec/02-architecture.md 9.1 step 7, 06 section 3.1).
type Credentials struct {
	// Method is the request method, the DPoP claim htm.
	Method string
	// HTU is the DPoP claim htu the proof must name: server.public_url plus the escaped request
	// path, without query and fragment. It is never built from the Host header of the client.
	HTU string
	// Authorization is the value of the Authorization header ("DPoP <token>" or "Bearer <token>").
	Authorization string
	// DPoP is the value of the DPoP header (the proof), or "".
	DPoP string
	// ClientIP is the client IP the chain resolved (06 "Basic rules", row Client IP); the
	// identity module counts failed proofs against rl.public_auth with it.
	ClientIP netip.Addr
}

// Present reports whether the request carries credentials: an Authorization header. A request
// without one is anonymous; a DPoP header alone is not a credential.
func (c Credentials) Present() bool {
	return c.Authorization != ""
}

// Authenticator checks the credentials of a request and the principal behind them. The identity
// module implements it (M1-34, M1-44); platform/httpserver and the gateway use it.
type Authenticator interface {
	// Authenticate returns the principal for creds. Without credentials (creds.Present() false) it
	// returns Anonymous(). Failed credentials give an *errs.Error with one of the 401 codes of 06
	// (unauthenticated, token_expired, session_revoked, use_dpop_nonce, invalid_dpop_proof) and
	// the headers the contract asks for (WWW-Authenticate, DPoP-Nonce).
	Authenticate(ctx context.Context, creds Credentials) (Actor, error)
	// Revalidate checks an authenticated principal again without a new proof: session, device,
	// token and principal status. The gateway calls it to close connections after a revocation.
	// It returns nil when a is still valid and a 401 *errs.Error otherwise.
	Revalidate(ctx context.Context, a Actor) error
}

// AnonymousOnly is the Authenticator until identity is wired (M1-34): every request without
// credentials is anonymous, and every request with an Authorization header fails with
// 401 unauthenticated, so no credential is ever accepted unchecked (fail closed).
type AnonymousOnly struct{}

// Compile-time check that AnonymousOnly implements Authenticator.
var _ Authenticator = AnonymousOnly{}

// wwwAuthenticate is the challenge of AnonymousOnly: both schemes of 06 section 3.
const wwwAuthenticate = "DPoP, Bearer"

// Authenticate returns Anonymous() without credentials and 401 unauthenticated with them.
func (AnonymousOnly) Authenticate(_ context.Context, creds Credentials) (Actor, error) {
	if !creds.Present() {
		return Anonymous(), nil
	}
	return Actor{}, errs.New(errs.CodeUnauthenticated, "").WithHeader("WWW-Authenticate", wwwAuthenticate)
}

// Revalidate accepts the anonymous principal and rejects every other one with 401 unauthenticated,
// because AnonymousOnly never authenticates one.
func (AnonymousOnly) Revalidate(_ context.Context, a Actor) error {
	if a.IsAnonymous() {
		return nil
	}
	return errs.New(errs.CodeUnauthenticated, "").WithHeader("WWW-Authenticate", wwwAuthenticate)
}
