// SPDX-License-Identifier: Apache-2.0

package identity

import (
	"context"
	"errors"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// TokenKind is the kind part of an API token (bothy_<kind>_..., 08 section 12, ADR 0061); it
// follows the principal.
type TokenKind string

// The kinds createToken issues; the incoming webhook kind whi belongs to M2-27.
const (
	// TokenKindAgent is the token of an agent principal.
	TokenKindAgent TokenKind = "agt"
	// TokenKindApp is the token of an app principal (M2-24 issues it).
	TokenKindApp TokenKind = "app"
	// TokenKindPersonal is the personal token of a person (switch tokens.personal_enabled).
	TokenKindPersonal TokenKind = "pat"
)

// APIToken is an API token as the API shows it (schema ApiToken): never the secret, never the
// hash. Zero values are absent members.
type APIToken struct {
	ID, PrincipalID ids.ID
	// Kind is the kind of the token.
	Kind TokenKind
	// Name is the label of the token.
	Name string
	// TokenPrefix is bothy_<kind>_ plus the first 6 characters of the random part.
	TokenPrefix string
	// Scopes are the scopes of an app token; empty for agent and personal tokens.
	Scopes []string
	// DPoPJKT is the RFC 7638 thumbprint the token is bound to; "" for an unbound token.
	DPoPJKT string
	// ExpiresAt is the mandatory expiry.
	ExpiresAt time.Time
	// CreatedBy is the principal that created the token.
	CreatedBy ids.ID
	CreatedAt time.Time
	// LastUsedAt is zero for a token never used; RevokedAt zero for an active token.
	LastUsedAt, RevokedAt time.Time
}

// TokenPage is one page of API tokens, newest first; NextCursor is "" on the last page.
type TokenPage struct {
	Items      []APIToken
	NextCursor string
}

// IssueTokenCmd is the input of Tokens.Issue: a new token for a principal, after the caller
// authorized it (createToken, later the app installation of M2-24).
type IssueTokenCmd struct {
	// PrincipalID is the holder; its kind decides the token kind (agent agt, app app, user pat).
	PrincipalID ids.ID
	// Name is the label: at least one character, no control, format or bidi characters.
	Name string
	// Scopes must be empty for agent and personal tokens (08 section 12).
	Scopes []string
	// DPoPJKT is the optional RFC 7638 SHA-256 thumbprint (43 base64url characters); required
	// when the kind is in auth.dpop_required_for.
	DPoPJKT string
	// ExpiresAt is required: after now and at most auth.api_token_max_ttl (agents, apps) or
	// auth.personal_token_max_ttl (personal tokens) ahead.
	ExpiresAt time.Time
	// CreatedBy is the principal that creates the token.
	CreatedBy ids.ID
}

// IssuedToken is the result of Issue: the stored token and its secret. The secret is a Token,
// which prints "[redacted]" through fmt, slog and JSON; only Token.Reveal hands it out, and only
// the createToken handler calls it, once (SEC-LOG-1).
type IssuedToken struct {
	Token  APIToken
	Secret Token
}

// PairingCode is a new pairing code of a terminal host (bothy_hpc_..., 24-terminal.md section 3.2,
// ADR 0061): the secret, shown exactly once by createHost, and its HMAC-SHA-256 with the token
// pepper of auth.token_pepper_ref, the only form the terminal module stores
// (hosts.pairing_code_hash). The secret prints "[redacted]" through fmt, slog and JSON.
type PairingCode struct {
	// Secret is the code; only the createHost handler reveals it, once.
	Secret Token
	// Hash is HMAC-SHA-256(pepper, code).
	Hash []byte
}

// IncomingWebhookToken is a new token of an incoming webhook (bothy_whi_..., 08 section 12, ADR
// 0061, M2-27): the secret, shown exactly once by createIncomingWebhook inside its URL, and its
// HMAC-SHA-256 with the token pepper of auth.token_pepper_ref, the only form the module apps
// stores (incoming_webhooks.token_hash). The secret prints "[redacted]" through fmt, slog and JSON.
type IncomingWebhookToken struct {
	// Secret is the token; only the createIncomingWebhook handler reveals it, once.
	Secret Token
	// Hash is HMAC-SHA-256(pepper, token).
	Hash []byte
}

// Tokens is the API token port of identity (02 section 4.3, brief M1-44). Its methods authorize
// nothing: the calling use case did. Each joins the ambient unit of work of ctx, or runs its own.
// Every error is a kernel/errs error without the secret, the hash or SQL.
type Tokens interface {
	// Issue validates cmd, generates the token from crypto/rand, stores only its HMAC and prefix
	// and returns it with the secret. Errors: not_found for an unknown principal;
	// validation_failed with the pointer of the field (/name, /scopes, /expires_at, /dpop_jkt).
	Issue(ctx context.Context, cmd IssueTokenCmd) (IssuedToken, error)
	// List returns the tokens of principalID with revoked_at IS NULL (expired ones included),
	// newest first. Errors: cursor_invalid.
	List(ctx context.Context, principalID ids.ID, page Page) (TokenPage, error)
	// Revoke sets revoked_at and revoked_by of an active token; an already revoked token stays
	// unchanged. Inside a unit of work it requires a declared rights change (the caller owns it);
	// alone it runs its own rights change for the token's principal. Errors: not_found for an
	// unknown token; internal_error inside a unit that is no rights change.
	Revoke(ctx context.Context, tokenID, by ids.ID) error
	// RevokeAllForPrincipal revokes every active token of principalID and returns their number
	// (M1-45 on user.disable, M1-51 on agent.disable). The caller owns the rights change and the
	// audit; outside a unit of work that is a rights change it fails with internal_error.
	RevokeAllForPrincipal(ctx context.Context, principalID, by ids.ID) (int, error)
	// NewHostPairingCode returns a new pairing code of kind hpc from crypto/rand with its HMAC
	// (M2-39). It reads and writes no database and authorizes nothing: host.manage did. The
	// authenticator never accepts the code; host.pair redeems it (M2-41). Errors: internal_error
	// when the random source fails.
	NewHostPairingCode(ctx context.Context) (PairingCode, error)
	// HashHostPairingCode returns the HMAC of raw exactly as NewHostPairingCode computes it, the
	// value host.pair looks up in hosts.pairing_code_hash (M2-41). A value that is not a
	// well-formed token of kind hpc (form, kind, checksum) is ErrTokenMalformed before any HMAC is
	// computed; it reads and writes no database and authorizes nothing.
	HashHostPairingCode(ctx context.Context, raw string) ([]byte, error)
	// NewIncomingWebhookToken returns a new token of kind whi from crypto/rand with its HMAC
	// (M2-27). It reads and writes no database and authorizes nothing: webhook_in.create did. The
	// authenticator never accepts the token; the token check of the incoming webhooks does.
	// Errors: internal_error when the random source fails.
	NewIncomingWebhookToken(ctx context.Context) (IncomingWebhookToken, error)
	// HashIncomingWebhookToken returns the HMAC of raw exactly as NewIncomingWebhookToken computes
	// it, the value compared with incoming_webhooks.token_hash (M2-27 design note 2 b). A value
	// that is not a well-formed token of kind whi (form, kind, checksum) is ErrTokenMalformed
	// before any HMAC is computed; it reads and writes no database and authorizes nothing.
	HashIncomingWebhookToken(ctx context.Context, raw string) ([]byte, error)
}

// ErrTokenMalformed reports a value that is not a well-formed token of the expected kind (ADR
// 0061: form, kind and checksum), found without any database access. It never names the value;
// callers answer 401 unauthenticated with the body of every other failed redemption (M2-41,
// TM-T20).
var ErrTokenMalformed = errors.New("identity: malformed token")

// CreateTokenCmd is the body of createToken (schema ApiTokenCreate).
type CreateTokenCmd struct {
	Name      string
	Scopes    []string
	DPoPJKT   string
	ExpiresAt time.Time
}

// TokenAdmin are the use cases of the three token operations (05 token.list, token.create,
// token.revoke): they authorize the caller a, then use Tokens.
type TokenAdmin interface {
	// CreateToken issues a token for principalID (token.create, step-up). Errors: the decision of
	// authorize (not_self for a personal token of another person, step_up_not_possible for a
	// token, owner_only for a non-owner on an agent); not_found for an unknown principal; conflict
	// app_not_installed for an app; feature_disabled for a personal token while
	// tokens.personal_enabled is off; validation_failed of the body.
	CreateToken(ctx context.Context, a actor.Actor, principalID ids.ID, cmd CreateTokenCmd) (IssuedToken, error)
	// ListTokens lists the active tokens of principalID (token.list, owner or the principal
	// itself). Errors: not_found for an unknown principal or one the caller may not read;
	// cursor_invalid.
	ListTokens(ctx context.Context, a actor.Actor, principalID ids.ID, page Page) (TokenPage, error)
	// RevokeToken revokes tokenID (token.revoke) in a rights change; an already revoked token is a
	// call without effect. Errors: not_found for an unknown token or one the caller may not see.
	RevokeToken(ctx context.Context, a actor.Actor, tokenID ids.ID) error
}
