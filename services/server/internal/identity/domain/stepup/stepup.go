// SPDX-License-Identifier: Apache-2.0

// Package stepup holds the pure rules of the step-up with the device key (docs/spec/08-identity.md
// "6.2 Step-up flow" and "13. Login protection", brief M1-35 "Design notes"): the random
// challenge value and its HMAC, the check of the compact JWS the step_up key signs over
// {challenge, sid, iat}, and the failure-limit decision. Keys, times and limits are parameters;
// nothing here reads a clock, the configuration or a store. identity/app composes them into the
// use cases.
//
// Example:
//
//	if cause := stepup.Check(signature, stepup.Expect{Key: k, Alg: alg, ChallengeHash: h, Pepper: p,
//		SessionID: sid, IssuedAt: created, Now: now, Leeway: leeway}); cause != "" {
//		// count the failure; revoke when stepup.LimitReached(failedAt, now, window, limit)
//	}
package stepup

import (
	"bytes"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"strconv"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/jose"
)

// challengeBytes is the size of a challenge value: 256 random bits, like every token of 08 "7.1".
const challengeBytes = 32

// NewChallengeValue returns a challenge value: 32 bytes from crypto/rand, base64url without padding
// (43 characters). The value appears only in the response; the store keeps ChallengeHash. Errors:
// those of the random source.
func NewChallengeValue() (string, error) {
	b := make([]byte, challengeBytes)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// ChallengeHash returns HMAC-SHA-256(pepper, value), the only form a challenge is stored in
// (auth_challenges.challenge_hash, 03 Grundsatz 9).
func ChallengeHash(pepper []byte, value string) []byte {
	m := hmac.New(sha256.New, pepper)
	m.Write([]byte(value))
	return m.Sum(nil)
}

// Cause is the reason a confirmation failed, a code for the audit details (SEC-AUD-5); "" means the
// confirmation holds. Every cause counts as a failed attempt.
type Cause string

// The causes of Check.
const (
	// CauseInvalidJWS: not a compact JWS of the profile (kernel/jose), an algorithm outside the
	// profile, or a crit, jku, x5u, x5c or zip header member.
	CauseInvalidJWS Cause = "invalid_jws"
	// CauseForbiddenHeader: a jwk header member; the key comes only from device_keys.
	CauseForbiddenHeader Cause = "forbidden_header"
	// CauseAlgMismatch: alg differs from the alg of the active step_up key.
	CauseAlgMismatch Cause = "alg_mismatch"
	// CauseBadSignature: the signature does not verify with the active step_up key (for example a
	// signature by the dpop key, ID-T18).
	CauseBadSignature Cause = "bad_signature"
	// CausePayloadInvalid: a member challenge, sid or iat is missing or of the wrong type.
	CausePayloadInvalid Cause = "payload_invalid"
	// CauseChallengeMismatch: the HMAC of challenge differs from challenge_hash.
	CauseChallengeMismatch Cause = "challenge_mismatch"
	// CauseSessionMismatch: sid is not the session of the request.
	CauseSessionMismatch Cause = "session_mismatch"
	// CauseIATOutOfRange: iat lies before the issue of the challenge minus the leeway or after now
	// plus the leeway.
	CauseIATOutOfRange Cause = "iat_out_of_range"
)

// Expect are the facts a confirmation is checked against.
type Expect struct {
	// Key is the active step_up key of the session's device; Alg its device_keys.alg, the only
	// algorithm allowed.
	Key jose.JWK
	Alg string
	// ChallengeHash is auth_challenges.challenge_hash; Pepper the HMAC key it was made with.
	ChallengeHash, Pepper []byte
	// SessionID is the session of the request, the only accepted sid.
	SessionID ids.ID
	// IssuedAt is auth_challenges.created_at; Now the time of the check; Leeway auth.clock_leeway.
	IssuedAt, Now time.Time
	Leeway        time.Duration
}

// supported are the algorithms a step_up key may have (kernel/jose profile); the alg of the key
// narrows them to one after parsing, so a differing alg is reported as such.
var supported = []string{jose.AlgES256, jose.AlgEdDSA}

// Check verifies signature against e and returns "" when it holds, otherwise the first failing
// cause in this order: form and header, alg, signature, payload members, challenge, sid, iat. iat is
// an integer NumericDate not before IssuedAt minus Leeway (cut to the whole second, because iat has
// whole seconds) and not after Now plus Leeway; unknown payload members are ignored.
func Check(signature string, e Expect) Cause {
	j, err := jose.ParseCompactJWS(signature, supported, "")
	if err != nil {
		return CauseInvalidJWS
	}
	if j.Header.JWK != nil {
		return CauseForbiddenHeader
	}
	if j.Header.Alg != e.Alg {
		return CauseAlgMismatch
	}
	if j.Verify(e.Key) != nil {
		return CauseBadSignature
	}
	p, ok := parsePayload(j.Payload)
	if !ok {
		return CausePayloadInvalid
	}
	if !hmac.Equal(ChallengeHash(e.Pepper, p.challenge), e.ChallengeHash) {
		return CauseChallengeMismatch
	}
	if p.sid != e.SessionID.String() {
		return CauseSessionMismatch
	}
	iat := time.Unix(p.iat, 0)
	if iat.Before(e.IssuedAt.Add(-e.Leeway).Truncate(time.Second)) || iat.After(e.Now.Add(e.Leeway)) {
		return CauseIATOutOfRange
	}
	return ""
}

// payload are the three members of a confirmation.
type payload struct {
	challenge, sid string
	iat            int64
}

// parsePayload reads challenge and sid as strings and iat as an integer; false when one is missing
// or of another type.
func parsePayload(raw []byte) (payload, bool) {
	var m map[string]json.RawMessage
	if json.Unmarshal(raw, &m) != nil {
		return payload{}, false
	}
	var p payload
	if !str(m["challenge"], &p.challenge) || !str(m["sid"], &p.sid) {
		return payload{}, false
	}
	n, ok := m["iat"]
	if !ok {
		return payload{}, false
	}
	iat, err := strconv.ParseInt(string(bytes.TrimSpace(n)), 10, 64)
	if err != nil {
		return payload{}, false
	}
	p.iat = iat
	return p, true
}

// str unmarshals a JSON string into dst; false for a missing member or another type.
func str(raw json.RawMessage, dst *string) bool {
	if len(raw) == 0 || raw[0] != '"' {
		return false
	}
	return json.Unmarshal(raw, dst) == nil
}

// Failures returns how many of failedAt (finished_at of the failed challenges of a session) lie
// strictly after now minus window (auth.login_fail_window).
func Failures(failedAt []time.Time, now time.Time, window time.Duration) int {
	from := now.Add(-window)
	n := 0
	for _, t := range failedAt {
		if t.After(from) {
			n++
		}
	}
	return n
}

// LimitReached reports whether the failures inside the window reach limit
// (auth.step_up_fail_limit); then the session is revoked with step_up_failed (08 "13").
func LimitReached(failedAt []time.Time, now time.Time, window time.Duration, limit int) bool {
	return Failures(failedAt, now, window) >= limit
}
