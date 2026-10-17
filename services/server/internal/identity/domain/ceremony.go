// SPDX-License-Identifier: Apache-2.0

package domain

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// WebAuthn ceremonies (docs/spec/08-identity.md "2.5 Local accounts", docs/spec/03-data-model.md
// auth_challenges, brief M1-40 "Design notes"): the state of a ceremony between begin and complete
// lives in a row of auth_challenges, never in process memory, so a ceremony begun on one node
// completes on any other. The row holds the HMAC of the challenge value and the schema
// webauthn_session.v1 below; it never holds the challenge itself.

// WebAuthnSessionSchema is the schema name of auth_challenges.webauthn_session.
const WebAuthnSessionSchema = "webauthn_session.v1"

// UserVerificationRequired is the only user verification a ceremony of this server asks for
// (08 "2.5": userVerification = required).
const UserVerificationRequired = "required"

// WebAuthnSession is auth_challenges.webauthn_session (schema webauthn_session.v1): the relying
// party ID the options named, the required user verification, the credential IDs the options
// allowed (empty for a discoverable login and for a creation) and the user handle of a creation.
// It never carries the challenge or a secret.
type WebAuthnSession struct {
	// Schema is always WebAuthnSessionSchema.
	Schema string `json:"schema"`
	// RPID is the relying party ID of the options.
	RPID string `json:"rp_id"`
	// UserVerification is always UserVerificationRequired.
	UserVerification string `json:"user_verification"`
	// AllowedCredentialIDs are the credential IDs of allowCredentials, base64url without padding.
	AllowedCredentialIDs []string `json:"allowed_credential_ids,omitempty"`
	// UserHandle is user.id of a creation, base64url without padding; "" for an assertion.
	UserHandle string `json:"user_handle,omitempty"`
}

// errWebAuthnSession reports a stored webauthn_session that is not schema webauthn_session.v1.
var errWebAuthnSession = errors.New("identity: the stored webauthn_session is not webauthn_session.v1")

// NewWebAuthnSession returns the session of a ceremony with rpID, the allowed credential IDs and
// the user handle (nil for none); user verification is always required.
//
// Example:
//
//	raw, _ := domain.NewWebAuthnSession("chat.example.com", allowed, nil).Marshal()
func NewWebAuthnSession(rpID string, allowed [][]byte, userHandle []byte) WebAuthnSession {
	s := WebAuthnSession{Schema: WebAuthnSessionSchema, RPID: rpID, UserVerification: UserVerificationRequired}
	for _, id := range allowed {
		s.AllowedCredentialIDs = append(s.AllowedCredentialIDs, base64.RawURLEncoding.EncodeToString(id))
	}
	if len(userHandle) > 0 {
		s.UserHandle = base64.RawURLEncoding.EncodeToString(userHandle)
	}
	return s
}

// Marshal encodes s as JSON for auth_challenges.webauthn_session.
func (s WebAuthnSession) Marshal() ([]byte, error) { return json.Marshal(s) }

// Allows reports whether credentialID may complete the ceremony: every ID when the options named
// none (a discoverable login), otherwise only one of AllowedCredentialIDs.
func (s WebAuthnSession) Allows(credentialID []byte) bool {
	if len(s.AllowedCredentialIDs) == 0 {
		return true
	}
	want := base64.RawURLEncoding.EncodeToString(credentialID)
	for _, id := range s.AllowedCredentialIDs {
		if id == want {
			return true
		}
	}
	return false
}

// ParseWebAuthnSession decodes a stored webauthn_session. Errors: invalid JSON, another schema or
// a user verification other than required; the caller treats them as a failed ceremony.
func ParseWebAuthnSession(raw []byte) (WebAuthnSession, error) {
	var s WebAuthnSession
	if err := json.Unmarshal(raw, &s); err != nil {
		return WebAuthnSession{}, errWebAuthnSession
	}
	if s.Schema != WebAuthnSessionSchema || s.UserVerification != UserVerificationRequired || s.RPID == "" {
		return WebAuthnSession{}, errWebAuthnSession
	}
	return s, nil
}

// CeremonyOutcome is the decision about a stored challenge row before its confirmation is
// checked.
type CeremonyOutcome int

// The outcomes of CheckCeremony.
const (
	// CeremonyOpen: the row is issued and not expired; it may be completed now, once.
	CeremonyOpen CeremonyOutcome = iota + 1
	// CeremonyFinished: the row is used or failed already (single use).
	CeremonyFinished
	// CeremonyExpired: expires_at has passed.
	CeremonyExpired
)

// StatusIssued is auth_challenges.status of an open challenge.
const StatusIssued = "issued"

// CheckCeremony decides whether the challenge row with status and expiresAt may be completed at
// now: only an issued row before its expiry (03 auth_challenges "Redemption"). The caller holds the
// row lock, so two completions see the result of each other.
//
// Example:
//
//	if domain.CheckCeremony(row.Status, row.ExpiresAt, now) != domain.CeremonyOpen { reject }
func CheckCeremony(status string, expiresAt, now time.Time) CeremonyOutcome {
	switch {
	case status != StatusIssued:
		return CeremonyFinished
	case !now.Before(expiresAt):
		return CeremonyExpired
	}
	return CeremonyOpen
}

// clientData is the part of the collected client data a ceremony reads before verification.
type clientData struct {
	Challenge string `json:"challenge"`
}

// credentialResponse is the part of a PublicKeyCredential in JSON form that carries the client
// data.
type credentialResponse struct {
	Response struct {
		ClientDataJSON string `json:"clientDataJSON"`
	} `json:"response"`
}

// ClientDataChallenge returns the challenge member of the clientDataJSON inside credentialJSON, a
// PublicKeyCredential in JSON form (WebAuthn Level 3 "toJSON"): base64url as the client wrote it.
// ok is false when the structure or the encoding is broken; the caller rejects such a response
// like a wrong confirmation.
func ClientDataChallenge(credentialJSON []byte) (challenge string, ok bool) {
	var c credentialResponse
	if err := json.Unmarshal(credentialJSON, &c); err != nil || c.Response.ClientDataJSON == "" {
		return "", false
	}
	raw, err := decodeBase64URL(c.Response.ClientDataJSON)
	if err != nil {
		return "", false
	}
	var cd clientData
	if err := json.Unmarshal(raw, &cd); err != nil || cd.Challenge == "" {
		return "", false
	}
	return cd.Challenge, true
}

// ChallengeMatches reports whether the base64url challenge value hashes to stored under pepper
// (HMAC-SHA-256 compared in constant time, 03 Grundsatz 9). The value is normalised to base64url
// without padding first, because a client may add padding.
func ChallengeMatches(pepper, stored []byte, challenge string) bool {
	raw, err := decodeBase64URL(challenge)
	if err != nil {
		return false
	}
	m := hmac.New(sha256.New, pepper)
	m.Write([]byte(base64.RawURLEncoding.EncodeToString(raw)))
	return hmac.Equal(m.Sum(nil), stored)
}

// DecodeChallenge returns the raw bytes of a base64url challenge value (with or without padding).
func DecodeChallenge(value string) ([]byte, error) { return decodeBase64URL(value) }

// decodeBase64URL decodes base64url with or without padding.
func decodeBase64URL(s string) ([]byte, error) {
	return base64.RawURLEncoding.DecodeString(strings.TrimRight(s, "="))
}

// SignCount applies the sign counter rule (03 passkey_credentials, brief M1-40 "Sign counter"):
// when stored and received are both above zero and received is not greater than stored, it is a
// regression; the ceremony continues and the higher value is kept. Counters that stay zero
// (synced passkeys) are no regression. keep is the value to store.
//
// Example:
//
//	keep, regression := domain.SignCount(7, 5) // 7, true: audit, continue
func SignCount(stored, received uint32) (keep uint32, regression bool) {
	if stored > 0 && received > 0 && received <= stored {
		return stored, true
	}
	return max(stored, received), false
}

// decoyContext separates the HMAC input of decoy credential IDs from every other use of the
// pepper (docs/DECISIONS.md M1-40).
const decoyContext = "passkey-decoy/v1"

// DecoyCredentialID returns the decoy credential ID beginPasskeyLogin names for a login name
// without passkeys: the first 32 bytes of HMAC-SHA-256(pepper, "passkey-decoy/v1" || 0x00 ||
// lower case name). It is stable for a name and differs between names, so the answer has the
// same shape as the answer for a real person and reveals no account (08 "2.5").
func DecoyCredentialID(pepper []byte, loginName string) []byte {
	m := hmac.New(sha256.New, pepper)
	m.Write([]byte(decoyContext))
	m.Write([]byte{0})
	m.Write([]byte(strings.ToLower(loginName)))
	return m.Sum(nil)[:sha256.Size]
}

// UserHandle returns the WebAuthn user handle of a person: the 16 bytes of the principal ID
// (brief M1-40 "Enrollment with passkey"; no personal data in the handle).
func UserHandle(principalID ids.ID) []byte {
	b := [16]byte(principalID)
	return b[:]
}

// UserOfHandle returns the principal ID of a user handle; ok is false for a handle that is not 16
// bytes.
func UserOfHandle(handle []byte) (ids.ID, bool) {
	if len(handle) != 16 {
		return ids.ID{}, false
	}
	return ids.ID([16]byte(handle)), true
}

// SameBytes reports whether a and b are equal; a helper for credential IDs, which are no secret.
func SameBytes(a, b []byte) bool { return bytes.Equal(a, b) }
