// SPDX-License-Identifier: Apache-2.0

package actor

import (
	"context"
	"log/slog"
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
	// SessionActive reports what the authenticator read for this request: the session exists, is
	// not revoked, has not reached its end and belongs to ID and DeviceID (M1-34). authz decides
	// with it (check_active); the authenticator never rejects on it, so the denial is audited.
	SessionActive bool
	// DeviceID is the device of the session (people only).
	DeviceID ids.ID
	// DeviceState is the state of the device as the authenticator read it for this request:
	// DevicePending, DeviceActive, DeviceRevoked or DeviceUnknown; "" when the authenticator read
	// none (then authz falls back to its credential source).
	DeviceState string
	// DeviceKeyActive reports that KeyThumbprint is the device's active dpop key.
	DeviceKeyActive bool
	// KeyThumbprint is cnf.jkt of the access token, the RFC 7638 thumbprint of the DPoP key.
	KeyThumbprint string
	// StepUpAt is the last fresh confirmation of the session; zero when there was none.
	StepUpAt time.Time
	// StepUpMethod is auth_sessions.step_up_method of StepUpAt; "" for none.
	StepUpMethod string
	// StepUpConsumedAt is auth_sessions.step_up_consumed_at, the time an opening of a terminal
	// session consumed the step-up (08 section 6.3 point 4, M2-40); zero when never consumed. A
	// step-up counts as consumed while StepUpConsumedAt is not before StepUpAt.
	StepUpConsumedAt time.Time
	// AuthMethod is auth_sessions.auth_method, the login path of the session.
	AuthMethod string
	// ClientComponent is auth_sessions.client_component (app, share_ext, console).
	ClientComponent string
	// TokenID is the API token of an agent, app or personal token; zero otherwise.
	TokenID ids.ID
	// CredentialInactive reports that the authenticator found the API token of this request
	// revoked or expired (M1-44), or the session, device or device key of a person's access token
	// revoked, ended or inactive (M1-45). The authenticator does not reject it, so authorize refuses it in
	// check_active and audits token_revoked; the stages before authorize must not act for such a
	// principal: Idempotency-Key never replays a stored answer to it, the rate limiter keys it by
	// the client IP and the failed-credentials precheck counts it as a failure.
	CredentialInactive bool
	// CredentialExpiresAt is the end of the credential this actor was authenticated with: the exp
	// of a person's access token, the expires_at of an API token; zero when unknown (anonymous,
	// tests). The gateway schedules reauth_required before it and closes with 4011 at it (04
	// section 6.3 step 6, M1-64); REST requests ignore it.
	CredentialExpiresAt time.Time
	// Scopes are the scopes of the API token (api_tokens.scopes, 07 section 3 token_scopes). For
	// an app, authz requires the scope of each action here as well as in GrantedScopes (M2-24,
	// docs/DECISIONS.md D-M2-24-10, which replaces M1-28 D6); agents and people carry none.
	Scopes []string
	// InstallationID is the app installation (apps only).
	InstallationID ids.ID
	// GrantedScopes are app_installations.granted_scopes (apps only); authz checks the scope of
	// each action against them (05 section 2.6).
	GrantedScopes []string
	// ViewAs is the "view as" of an owner (07 section 9.2); nil for none. Only authz sets it, for
	// an owner session inside viewAsUser (M1-49); it is never parsed from client input in M1.
	ViewAs *ViewAs
	// ActingFor is the checked invocation token of the run an agent acts in (SR9); the zero value
	// for none. Filled from M2 on.
	ActingFor ActingFor
	// RecheckPrincipal marks the principal of the check without a request (05 section 5, R-AZ-29):
	// a person evaluated by the server itself, without a request of the person, for example
	// agent.invoke before every delivery to an agent (M2-08). Only Recheck builds it, never the
	// authentication of a request. authz takes liveness only from the principal status and the
	// owner status from the current owner list without a session, keeps its snapshot apart from
	// every request principal of the same person and refuses the marker with any other action or
	// with a session, device, token, view_as or acting_for (an internal error).
	RecheckPrincipal bool
}

// Recheck returns the principal of the check without a request for the person id (R-AZ-29): kind
// user, the marker RecheckPrincipal and nothing else, so it never carries a session, device or
// token of the person.
//
// Example:
//
//	d, err := az.Authorize(authorize.Filter(ctx), actor.Recheck(run.TriggeredBy), authorize.ActionAgentInvoke, res)
func Recheck(id ids.ID) Actor { return Actor{ID: id, Kind: KindUser, RecheckPrincipal: true} }

// IsRecheckPrincipal reports the marker RecheckPrincipal (kernel/authorize.RecheckMarker).
func (a Actor) IsRecheckPrincipal() bool { return a.RecheckPrincipal }

// The device states of Actor.DeviceState (devices.state plus unknown for a device that does not
// exist or belongs to someone else).
const (
	DevicePending = "pending"
	DeviceActive  = "active"
	DeviceRevoked = "revoked"
	DeviceUnknown = "unknown"
)

// ActingFor names the run an agent acts in (07 section 3, SR9): the triggering person, the run,
// its thread and the jti of the invocation token.
type ActingFor struct {
	UserID, RunID, ThreadID ids.ID
	JTI                     string
}

// ViewAs is the state of "view as" (07 section 3 field view_as, section 9.2): the person whose
// view an owner looks at and the start of the view. authorize intersects the owner's rights with
// the person's, refuses every write with view_as_read_only and, from StartedAt plus
// authz.view_as_max_duration on, ends every decision with step_up_required.
//
// Example:
//
//	as := owner
//	as.ViewAs = &actor.ViewAs{UserID: alex, StartedAt: now}
type ViewAs struct {
	// UserID is the viewed person.
	UserID ids.ID
	// StartedAt is the request time the view started at.
	StartedAt time.Time
}

// IsZero reports whether f names no run.
func (f ActingFor) IsZero() bool { return f == ActingFor{} }

// PrincipalID returns a.ID (kernel/authorize.Principal).
func (a Actor) PrincipalID() ids.ID { return a.ID }

// PrincipalKind returns the kind as text (kernel/authorize.Principal).
func (a Actor) PrincipalKind() string { return string(a.Kind) }

// SessionRef returns a.SessionID (kernel/authorize.Principal).
func (a Actor) SessionRef() ids.ID { return a.SessionID }

// DeviceRef returns a.DeviceID (kernel/authorize.Principal).
func (a Actor) DeviceRef() ids.ID { return a.DeviceID }

// TokenRef returns a.TokenID (kernel/authorize.Principal).
func (a Actor) TokenRef() ids.ID { return a.TokenID }

// InstallationRef returns a.InstallationID (kernel/authorize.Principal).
func (a Actor) InstallationRef() ids.ID { return a.InstallationID }

// StepUpTime returns a.StepUpAt (kernel/authorize.Principal).
func (a Actor) StepUpTime() time.Time { return a.StepUpAt }

// StepUpMethodName returns a.StepUpMethod (kernel/authorize.Principal).
func (a Actor) StepUpMethodName() string { return a.StepUpMethod }

// StepUpConsumedTime returns a.StepUpConsumedAt (kernel/authorize.Principal).
func (a Actor) StepUpConsumedTime() time.Time { return a.StepUpConsumedAt }

// ClientComponentName returns a.ClientComponent (kernel/authorize.Principal).
func (a Actor) ClientComponentName() string { return a.ClientComponent }

// CredentialState returns the state of session, device and device key the authenticator read for
// this request and true; false when it read none (DeviceState empty), for example for an API
// token or a principal built outside the authenticator. authz prefers these fresh facts to its
// cached ones (07 section 4.2 check_active, M1-34). The signature uses plain values because the
// kernel ports do not import each other.
func (a Actor) CredentialState() (sessionActive bool, deviceState string, deviceKeyActive, observed bool) {
	if a.DeviceState == "" {
		return false, "", false, false
	}
	return a.SessionActive, a.DeviceState, a.DeviceKeyActive, true
}

// AppGrantedScopes returns a.GrantedScopes (kernel/authorize.Principal); the slice is shared.
func (a Actor) AppGrantedScopes() []string { return a.GrantedScopes }

// AppTokenScopes returns a.Scopes (kernel/authorize.Principal); the slice is shared.
func (a Actor) AppTokenScopes() []string { return a.Scopes }

// ViewAsPerson returns the viewed person of a.ViewAs, zero for none (kernel/authorize.Principal).
func (a Actor) ViewAsPerson() ids.ID {
	if a.ViewAs == nil {
		return ids.ID{}
	}
	return a.ViewAs.UserID
}

// ViewAsStart returns the start of a.ViewAs, zero for none (kernel/authorize.Principal).
func (a Actor) ViewAsStart() time.Time {
	if a.ViewAs == nil {
		return time.Time{}
	}
	return a.ViewAs.StartedAt
}

// ActingForPerson returns a.ActingFor.UserID (kernel/authorize.Principal).
func (a Actor) ActingForPerson() ids.ID { return a.ActingFor.UserID }

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
	a.GrantedScopes = slices.Clone(a.GrantedScopes)
	a.ViewAs = cloneViewAs(a.ViewAs)
	return context.WithValue(ctx, actorKey{}, a)
}

// FromContext returns the principal stored with WithActor and true, or the zero Actor and false.
// A use case that finds no actor must deny: it runs outside the chain.
func FromContext(ctx context.Context) (Actor, bool) {
	a, ok := ctx.Value(actorKey{}).(Actor)
	if ok {
		a.Scopes = slices.Clone(a.Scopes)
		a.GrantedScopes = slices.Clone(a.GrantedScopes)
		a.ViewAs = cloneViewAs(a.ViewAs)
	}
	return a, ok
}

// cloneViewAs copies v, so a context never shares it with the caller.
func cloneViewAs(v *ViewAs) *ViewAs {
	if v == nil {
		return nil
	}
	c := *v
	return &c
}

// ConsoleCookieName is the name of the session cookie of the web console (docs/spec/08-identity.md
// section 7.5, M6-01). The HTTP chain reads it into Credentials.ConsoleCookie and the gateway
// refuses an upgrade that carries it without the own Origin (SEC-CORS-3).
const ConsoleCookieName = "__Host-console_session"

// redacted is the only rendering of a CookieValue.
const redacted = "[redacted]"

// CookieValue is the value of the console cookie of a request. It renders as "[redacted]" through
// fmt, slog and encoding/json, so it never reaches a log, an audit entry or an error by accident
// (SEC-LOG-1); only Reveal hands out the value.
type CookieValue struct {
	v string
}

// NewCookieValue wraps v.
func NewCookieValue(v string) CookieValue { return CookieValue{v: v} }

// Reveal returns the value; "" for none.
func (c CookieValue) Reveal() string { return c.v }

// IsZero reports a request without the cookie.
func (c CookieValue) IsZero() bool { return c.v == "" }

// String returns "[redacted]".
func (CookieValue) String() string { return redacted }

// GoString returns "[redacted]".
func (CookieValue) GoString() string { return redacted }

// LogValue implements slog.LogValuer.
func (CookieValue) LogValue() slog.Value { return slog.StringValue(redacted) }

// MarshalJSON returns the JSON string "[redacted]".
func (CookieValue) MarshalJSON() ([]byte, error) { return []byte(`"` + redacted + `"`), nil }

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
	// DPoPProofs are all values of the DPoP header in order; more than one is an invalid proof
	// (RFC 9449 section 4.3). nil when the request has none.
	DPoPProofs []string
	// ClientIP is the client IP the chain resolved (06 "Basic rules", row Client IP); the
	// identity module counts failed proofs against rl.public_auth with it.
	ClientIP netip.Addr
	// ConsoleCookie is the value of the cookie ConsoleCookieName, the second credential kind of a
	// person in the web console (M6-01); zero without the cookie.
	ConsoleCookie CookieValue
}

// Present reports whether the request carries credentials: an Authorization header or the console
// cookie. A request without either is anonymous; a DPoP header alone is not a credential.
func (c Credentials) Present() bool {
	return c.Authorization != "" || !c.ConsoleCookie.IsZero()
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

// AnonymousOnly is the Authenticator before identity is wired (internal/wire uses identity's from
// the start step sessions on, M1-34) and in tests: every request without
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
