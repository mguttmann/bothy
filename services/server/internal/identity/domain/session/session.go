// SPDX-License-Identifier: Apache-2.0

// Package session holds the pure rules of sessions, refresh families and access tokens of the
// identity module (docs/spec/08-identitaet.md "7. Token-Modell", docs/spec/03-datenmodell.md
// tables auth_sessions, refresh_token_families and refresh_tokens): the rotation decision of a
// refresh token, the claims of an access token with their validity capped at the session end, the
// refresh token itself (256 random bits) and its HMAC hash. It reads no clock and no database.
//
// Example:
//
//	switch session.Decide(family, token, now, grace) {
//	case session.OutcomeRotate, session.OutcomeGraceRotate:
//		// issue generation family.CurrentGeneration + 1
//	case session.OutcomeReuse:
//		// revoke family and session (refresh_reuse)
//	}
package session

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"net/url"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// Outcome is the result of Decide.
type Outcome string

// The outcomes of the rotation decision (03 refresh_tokens, "Rotation" items 1 to 3).
const (
	// OutcomeRotate: the token is the current generation and unused; rotate (item 1).
	OutcomeRotate Outcome = "rotate"
	// OutcomeGraceRotate: the token is the generation just before the current one and the last
	// rotation lies less than auth.refresh_reuse_grace back; rotate again from the current
	// generation, whose never received token is marked used (item 2).
	OutcomeGraceRotate Outcome = "grace_rotate"
	// OutcomeReuse: any other used or older token; revoke family and session (item 3).
	OutcomeReuse Outcome = "reuse"
	// OutcomeExpired: the token's expires_at has passed; nothing changes.
	OutcomeExpired Outcome = "expired"
	// OutcomeFamilyRevoked: the family is revoked; nothing changes.
	OutcomeFamilyRevoked Outcome = "family_revoked"
)

// Family is the state of a refresh_token_families row that the decision reads.
type Family struct {
	// CurrentGeneration is the number of the only valid token.
	CurrentGeneration int
	// RotatedAt is the time of the last rotation; zero when the family never rotated.
	RotatedAt time.Time
	// Revoked reports revoked_at IS NOT NULL.
	Revoked bool
}

// Token is the state of a refresh_tokens row that the decision reads.
type Token struct {
	// Generation is the generation of the token.
	Generation int
	// UsedAt is set when the token was rotated; zero while unused.
	UsedAt time.Time
	// ExpiresAt is the end of the token.
	ExpiresAt time.Time
}

// Decide returns what a refresh with token t of family f at now does, with grace the length of
// auth.refresh_reuse_grace. A revoked family wins, then expiry, then items 1 to 3 of 03. The grace
// holds strictly below grace: a rotation exactly grace ago is reuse.
func Decide(f Family, t Token, now time.Time, grace time.Duration) Outcome {
	switch {
	case f.Revoked:
		return OutcomeFamilyRevoked
	case !now.Before(t.ExpiresAt):
		return OutcomeExpired
	case t.Generation == f.CurrentGeneration && t.UsedAt.IsZero():
		return OutcomeRotate
	case t.Generation == f.CurrentGeneration-1 && !f.RotatedAt.IsZero() && now.Sub(f.RotatedAt) < grace:
		return OutcomeGraceRotate
	}
	return OutcomeReuse
}

// RefreshExpiry returns the expires_at of a new refresh token: now + ttl
// (auth.refresh_token_ttl), never beyond sessionEnd (08 "7.1").
func RefreshExpiry(now time.Time, ttl time.Duration, sessionEnd time.Time) time.Time {
	end := now.Add(ttl)
	if end.After(sessionEnd) {
		return sessionEnd
	}
	return end
}

// apiBase is the server base path of the REST API (docs/api/openapi.yaml servers).
const apiBase = "/api/v1"

// Audience returns the aud claim of access tokens: the origin of publicURL (server.public_url)
// plus /api/v1, for example https://chat.example.com/api/v1. Errors: a value without scheme or
// host.
func Audience(publicURL string) (string, error) {
	u, err := url.Parse(publicURL)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return "", errors.New("session: server.public_url must be an absolute origin")
	}
	return u.Scheme + "://" + u.Host + apiBase, nil
}

// ClaimsInput are the facts of one access token.
type ClaimsInput struct {
	// InstanceID is instance.instance_id, the issuer.
	InstanceID ids.ID
	// Audience is the result of Audience.
	Audience string
	// UserID, SessionID and DeviceID are sub, sid and did.
	UserID, SessionID, DeviceID ids.ID
	// JTI is a new UUIDv7.
	JTI ids.ID
	// Now is the time of issue; TTL is auth.access_token_ttl.
	Now time.Time
	TTL time.Duration
	// SessionEnd is auth_sessions.expires_at; exp never lies after it.
	SessionEnd time.Time
	// JKT is the dpop_jkt of the session's family, the cnf.jkt claim.
	JKT string
	// AuthMethod is auth_sessions.auth_method, the only member of amr.
	AuthMethod string
}

// Confirmation is the cnf claim (RFC 7800, RFC 9449 section 6).
type Confirmation struct {
	// JKT is the RFC 7638 thumbprint of the DPoP key.
	JKT string `json:"jkt"`
}

// Claims are the claims of an access token, exactly the set of 08 "7.2"; no rights.
type Claims struct {
	Issuer    string       `json:"iss"`
	Audience  string       `json:"aud"`
	Subject   string       `json:"sub"`
	SessionID string       `json:"sid"`
	DeviceID  string       `json:"did"`
	JTI       string       `json:"jti"`
	IssuedAt  int64        `json:"iat"`
	NotBefore int64        `json:"nbf"`
	Expiry    int64        `json:"exp"`
	Cnf       Confirmation `json:"cnf"`
	AMR       []string     `json:"amr"`
}

// BuildClaims returns the claims of in: iat = nbf = now in whole seconds, exp = min(now + TTL,
// session end) rounded down to a whole second, so it never lies after the session end.
func BuildClaims(in ClaimsInput) Claims {
	iat := in.Now.Unix()
	exp := in.Now.Add(in.TTL)
	if exp.After(in.SessionEnd) {
		exp = in.SessionEnd
	}
	return Claims{
		Issuer: in.InstanceID.String(), Audience: in.Audience, Subject: in.UserID.String(), SessionID: in.SessionID.String(),
		DeviceID: in.DeviceID.String(), JTI: in.JTI.String(), IssuedAt: iat, NotBefore: iat, Expiry: exp.Unix(),
		Cnf: Confirmation{JKT: in.JKT}, AMR: []string{in.AuthMethod},
	}
}

// ExpiresIn returns TokenResponse.expires_in: whole seconds from now to exp, at least 0.
func (c Claims) ExpiresIn(now time.Time) int {
	return int(max(c.Expiry-now.Unix(), 0))
}

// refreshTokenBytes is the size of the random part of a refresh token: 256 bits (08 "7.1", ID-T27).
const refreshTokenBytes = 32

// NewRefreshToken returns a refresh token: 32 bytes from crypto/rand, base64url without padding
// (43 characters). Errors: those of the random source.
func NewRefreshToken() (string, error) {
	b := make([]byte, refreshTokenBytes)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// TokenHash returns HMAC-SHA-256(pepper, token), the only form a refresh token is stored in (03
// Grundsatz 9).
func TokenHash(pepper []byte, token string) []byte {
	m := hmac.New(sha256.New, pepper)
	m.Write([]byte(token))
	return m.Sum(nil)
}
