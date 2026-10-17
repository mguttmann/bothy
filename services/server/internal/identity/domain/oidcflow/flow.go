// SPDX-License-Identifier: Apache-2.0

package oidcflow

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// Status is auth_login_flows.status (03 section 4.2).
type Status string

// The states of a login flow.
const (
	// StatusStarted is a flow after begin, waiting for the provider callback.
	StatusStarted Status = "started"
	// StatusCodeIssued is a flow whose callback verified the ID token and issued the login_code.
	StatusCodeIssued Status = "code_issued"
	// StatusCompleted is a flow whose login_code was redeemed.
	StatusCompleted Status = "completed"
	// StatusFailed is a flow that ended with an error; it is never used again.
	StatusFailed Status = "failed"
)

// CanTransition reports whether a flow may move from from to to: started to code_issued,
// code_issued to completed, and started or code_issued to failed. Every other move, including a
// repeated one, is refused, so a replayed callback or a second redemption cannot change a flow.
func CanTransition(from, to Status) bool {
	switch to {
	case StatusCodeIssued:
		return from == StatusStarted
	case StatusCompleted:
		return from == StatusCodeIssued
	case StatusFailed:
		return from == StatusStarted || from == StatusCodeIssued
	case StatusStarted:
		// A flow is created started; nothing moves back to it.
	}
	return false
}

// Mode is auth_login_flows.mode.
type Mode string

// The modes of a flow.
const (
	// ModeLogin is beginOidcLogin: the flow ends in a session.
	ModeLogin Mode = "login"
	// ModeLink is beginIdentityLink: the flow links another identity to the person of user_id.
	ModeLink Mode = "link"
)

// The closed list of error codes the callback appends to app_redirect_uri (08 section 2.2 step 6).
// No other detail reaches the app.
const (
	// ErrorAccessDenied is a provider that answered with an error parameter.
	ErrorAccessDenied = "access_denied"
	// ErrorLoginFailed is any failed check of the callback.
	ErrorLoginFailed = "login_failed"
	// ErrorFlowExpired is a flow past expires_at.
	ErrorFlowExpired = "flow_expired"
)

// The HMAC contexts of the values a flow stores only as hash; each separates its hashes from
// every other use of the token pepper.
const (
	// HashState is the context of oidc_state_hash.
	HashState = "oidc-state/v1"
	// HashNonce is the context of oidc_nonce_hash.
	HashNonce = "oidc-nonce/v1"
	// HashLoginCode is the context of login_code_hash.
	HashLoginCode = "oidc-login-code/v1"
)

// serverVerifierContext prefixes the flow id in the HMAC of the server PKCE verifier (brief M1-41
// "Server PKCE towards the provider").
const serverVerifierContext = "oidc-pkce-verifier:"

// Hash returns HMAC-SHA-256 with pepper over context, a 0x00 byte and value. The stores keep only
// this hash of state, nonce and login_code (03 Grundsatz 9).
func Hash(pepper []byte, context, value string) []byte {
	m := hmac.New(sha256.New, pepper)
	m.Write([]byte(context))
	m.Write([]byte{0})
	m.Write([]byte(value))
	return m.Sum(nil)
}

// HashEqual compares two hashes in constant time.
func HashEqual(a, b []byte) bool { return hmac.Equal(a, b) }

// ServerVerifier returns the PKCE verifier of the server towards the provider for the flow id:
// base64url(HMAC-SHA-256(pepper, "oidc-pkce-verifier:" || id)), 43 characters. It is derived, not
// stored, so the table holds no further secret (03 auth_login_flows).
func ServerVerifier(pepper []byte, id ids.ID) string {
	m := hmac.New(sha256.New, pepper)
	m.Write([]byte(serverVerifierContext))
	m.Write([]byte(id.String()))
	return base64.RawURLEncoding.EncodeToString(m.Sum(nil))
}

// secretBytes is the entropy of state, nonce and login_code: 256 bits like every token of 08 "7.1".
const secretBytes = 32

// errShortRandom reports a random source that delivered fewer bytes than asked.
var errShortRandom = errors.New("oidcflow: the random source delivered too few bytes")

// NewSecret reads 32 bytes from r and returns them as base64url without padding (43 characters):
// the state and the nonce of a flow. Errors: the read error or a short read.
func NewSecret(r io.Reader) (string, error) {
	var b [secretBytes]byte
	if _, err := io.ReadFull(r, b[:]); err != nil {
		if errors.Is(err, io.ErrUnexpectedEOF) || errors.Is(err, io.EOF) {
			return "", errShortRandom
		}
		return "", fmt.Errorf("oidcflow: random source: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b[:]), nil
}

// loginCodePrefix starts every login_code (brief M1-41).
const loginCodePrefix = "lc_"

// NewLoginCode returns a login_code: "lc_" and 32 bytes of r as base64url (46 characters).
// Errors: as NewSecret.
func NewLoginCode(r io.Reader) (string, error) {
	s, err := NewSecret(r)
	if err != nil {
		return "", err
	}
	return loginCodePrefix + s, nil
}
