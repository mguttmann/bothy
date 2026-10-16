// SPDX-License-Identifier: Apache-2.0

package identity

import (
	"context"
	"net/netip"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// OIDCBegin is the input of OIDCLogin.BeginOIDCLogin and BeginIdentityLink (schemas
// OidcBeginRequest and IdentityLinkBegin).
type OIDCBegin struct {
	// Console selects the console form (client = console, M6-01): AppRedirectURI must be the
	// console callback /console/login/callback on the origin of server.public_url.
	Console bool
	// ProviderID is an enabled provider of kind oidc.
	ProviderID ids.ID
	// AppRedirectURI is the loopback callback of the app or an entry of the provider's
	// app_redirect_uris.
	AppRedirectURI string
	// ClientChallenge is the S256 challenge of the app's client_verifier.
	ClientChallenge string
	// EnrollmentCode is the optional enrollment code (login only); "" for none.
	EnrollmentCode string
}

// OIDCBeginResult is the answer of a begin (schema OidcBeginResponse).
type OIDCBeginResult struct {
	// LoginAttemptID is auth_login_flows.id.
	LoginAttemptID ids.ID
	// AuthorizationURL is the authorization endpoint of the provider with the server's request.
	AuthorizationURL string
	// ExpiresAt is the end of the flow (auth.login_flow_ttl).
	ExpiresAt time.Time
}

// OIDCCallback is the query of the provider's redirect to handleOidcCallback. A parameter the
// provider did not send is "".
type OIDCCallback struct {
	State, Code, Iss, Error string
	// HasIss reports an iss parameter, even an empty one (RFC 9207).
	HasIss bool
}

// OIDCComplete is the input of OIDCLogin.CompleteOIDCLogin (schema OidcCompleteRequest plus the
// request facts).
type OIDCComplete struct {
	// Console selects the console form (client = console, M6-01): no Device and no Proof; the
	// login registers a web device and starts a console session with a cookie instead of tokens.
	Console bool
	// LoginCode is the one-time code of the callback redirect.
	LoginCode string
	// ClientVerifier is the PKCE verifier of the app instance that began the flow.
	ClientVerifier string
	// Device is the registration of the device; UserID, State and ApprovedBy are set by the login
	// completion.
	Device RegisterDevice
	// Proof is the DPoP proof of the request, verified as a login proof without ath.
	Proof ProofRequest
	// ClientIP is the client address of the request; the zero Addr for none.
	ClientIP netip.Addr
}

// OIDCLogin is the incoming port of the OIDC login (M1-41, 08 section 2.2): the server is the
// confidential relying party of the providers and the authorization server of its apps. Errors are
// kernel/errs errors; none carries a code, state, nonce, verifier or provider token.
type OIDCLogin interface {
	// BeginOIDCLogin authorizes auth.login.begin for the anonymous principal, checks provider,
	// redirect, challenge and the optional enrollment code (401 for every rejected code), loads the
	// provider metadata (503 when it fails) and starts a login flow. Errors: not_found for a
	// missing, disabled or non-OIDC provider; validation_failed; unauthenticated; unavailable.
	BeginOIDCLogin(ctx context.Context, in OIDCBegin) (OIDCBeginResult, error)
	// HandleOIDCCallback authorizes auth.login.begin, finds the flow by its state, exchanges the
	// code, verifies the ID token and stores the verified identity with a new login_code. It
	// returns the redirect target: app_redirect_uri with login_code and login_attempt_id, or with
	// error and login_attempt_id after a failure of a found flow. Errors: validation_failed for an
	// unknown, expired or used state (no redirect target is trusted).
	HandleOIDCCallback(ctx context.Context, in OIDCCallback) (redirect string, err error)
	// CompleteOIDCLogin authorizes auth.login.complete, verifies the DPoP proof, redeems the
	// login_code with the client_verifier and maps, links or provisions the person, registers the
	// device and starts the session. Errors: unauthenticated (every rejected code or identity),
	// invalid_dpop_proof, validation_failed or conflict for the device registration.
	CompleteOIDCLogin(ctx context.Context, in OIDCComplete) (RedeemResult, error)
	// BeginIdentityLink is BeginOIDCLogin in link mode for the person a (identity.link with
	// step-up); EnrollmentCode must be empty.
	BeginIdentityLink(ctx context.Context, a actor.Actor, in OIDCBegin) (OIDCBeginResult, error)
	// CompleteIdentityLink redeems the login_code of a link flow of a with the client_verifier and
	// links the identity (identity.link with step-up; a refusal leaves the code usable). Errors:
	// the decision of authorize, not_found for a code of another person or an unknown code,
	// unauthenticated for a wrong verifier, conflict for a subject that is linked already.
	CompleteIdentityLink(ctx context.Context, a actor.Actor, loginCode, clientVerifier string) (Identity, error)
}
