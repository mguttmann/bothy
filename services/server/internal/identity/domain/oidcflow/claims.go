// SPDX-License-Identifier: Apache-2.0

package oidcflow

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"slices"
	"strings"
	"time"
	"unicode/utf8"
)

// ErrClaims is wrapped by every error of ParseClaims and CheckIDToken. The wrapping error names
// the rule, never a claim value.
var ErrClaims = errors.New("oidcflow: ID token claims rejected")

// TenantPlaceholder is the placeholder of a multi-tenant issuer (08 section 2.3).
const TenantPlaceholder = "{tenantid}"

// Claims are the claims of an ID token whose signature the provider client verified (08 section
// 2.2 step 5). Times without their Has flag were absent.
type Claims struct {
	Issuer, Subject   string
	Audience          []string
	AZP               string
	Expiry, IssuedAt  time.Time
	HasExpiry         bool
	HasIssuedAt       bool
	Nonce             string
	TenantID          string
	Name              string
	PreferredUsername string
	Email             string
	AuthTime          time.Time
	AMR               []string
}

// maxNumericDate bounds a NumericDate to the year 9999, so a time always converts.
const maxNumericDate = 253402300799

// ParseClaims reads the claims of an ID token payload (a JSON object, already checked by
// kernel/jose): the strings iss, sub, azp, nonce, tid, name, preferred_username and email; aud as
// a string or an array of strings; exp, iat and auth_time as NumericDate between 0 and the year
// 9999 (fractions are cut); amr as an array of strings. Other members are ignored. Errors wrap
// ErrClaims for a member of another type.
func ParseClaims(payload []byte) (Claims, error) {
	var all map[string]json.RawMessage
	dec := json.NewDecoder(bytes.NewReader(payload))
	dec.UseNumber()
	if err := dec.Decode(&all); err != nil || all == nil {
		return Claims{}, fmt.Errorf("%w: the payload is not an object", ErrClaims)
	}
	var c Claims
	for _, step := range []func(map[string]json.RawMessage, *Claims) error{parseStrings, parseAudience, parseTimes, parseAMR} {
		if err := step(all, &c); err != nil {
			return Claims{}, err
		}
	}
	return c, nil
}

// parseStrings reads the string claims; null reads as absent.
func parseStrings(all map[string]json.RawMessage, c *Claims) error {
	for name, dst := range map[string]*string{
		"iss": &c.Issuer, "sub": &c.Subject, "azp": &c.AZP, "nonce": &c.Nonce, "tid": &c.TenantID, "name": &c.Name,
		"preferred_username": &c.PreferredUsername, "email": &c.Email,
	} {
		if raw, ok := all[name]; ok && json.Unmarshal(raw, dst) != nil {
			return fmt.Errorf("%w: %s is not a string", ErrClaims, name)
		}
	}
	return nil
}

// parseAudience reads aud.
func parseAudience(all map[string]json.RawMessage, c *Claims) error {
	raw, ok := all["aud"]
	if !ok {
		return nil
	}
	aud, err := audience(raw)
	c.Audience = aud
	return err
}

// parseTimes reads exp, iat and auth_time and records which of exp and iat were present.
func parseTimes(all map[string]json.RawMessage, c *Claims) error {
	for name, dst := range map[string]*time.Time{"exp": &c.Expiry, "iat": &c.IssuedAt, "auth_time": &c.AuthTime} {
		raw, ok := all[name]
		if !ok {
			continue
		}
		t, err := numericDate(raw)
		if err != nil {
			return fmt.Errorf("%w: %s is not a NumericDate", ErrClaims, name)
		}
		*dst = t
	}
	_, c.HasExpiry = all["exp"]
	_, c.HasIssuedAt = all["iat"]
	return nil
}

// parseAMR reads amr as an array of strings.
func parseAMR(all map[string]json.RawMessage, c *Claims) error {
	raw, ok := all["amr"]
	if !ok {
		return nil
	}
	if err := json.Unmarshal(raw, &c.AMR); err != nil || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return fmt.Errorf("%w: amr is not an array of strings", ErrClaims)
	}
	return nil
}

// audience reads aud as one string or an array of strings.
func audience(raw json.RawMessage) ([]string, error) {
	var one string
	if json.Unmarshal(raw, &one) == nil {
		return []string{one}, nil
	}
	var list []string
	if err := json.Unmarshal(raw, &list); err != nil || list == nil {
		return nil, fmt.Errorf("%w: aud is neither a string nor an array of strings", ErrClaims)
	}
	return list, nil
}

// numericDate reads a JSON number of seconds since the epoch.
func numericDate(raw json.RawMessage) (time.Time, error) {
	var n json.Number
	if t := bytes.TrimSpace(raw); len(t) == 0 || t[0] == '"' {
		return time.Time{}, errors.New("not a number")
	}
	if err := json.Unmarshal(raw, &n); err != nil {
		return time.Time{}, err
	}
	f, err := n.Float64()
	if err != nil || math.IsNaN(f) || f < 0 || f > maxNumericDate {
		return time.Time{}, errors.New("out of range")
	}
	return time.Unix(int64(f), 0).UTC(), nil
}

// Expectation is what an ID token must match (08 section 2.2 step 5, brief M1-41 item 5).
type Expectation struct {
	// Issuer is the configured issuer; it may contain TenantPlaceholder.
	Issuer string
	// AllowedTenants is allowed_tenant_ids; when set, tid must be in it.
	AllowedTenants []string
	// ClientID is the client_id at the provider.
	ClientID string
	// NonceHash is oidc_nonce_hash of the flow; Pepper the token pepper of its HMAC.
	NonceHash, Pepper []byte
	// FlowCreatedAt is created_at of the flow; Now the time of the check.
	FlowCreatedAt, Now time.Time
	// Leeway is auth.clock_leeway.
	Leeway time.Duration
}

// claimError is the error of one failed rule.
func claimError(rule string) error { return fmt.Errorf("%w: %s", ErrClaims, rule) }

// CheckIDToken applies the claim rules of brief M1-41 "handleOidcCallback" item 5 and returns the
// issuer the token was matched against: iss equals the expected issuer (the placeholder replaced
// with the token's tid, which must then be present and in AllowedTenants); with AllowedTenants
// set, tid is in it; aud contains ClientID; with more than one audience azp is required; a present
// azp equals ClientID; exp is present and after Now minus Leeway; iat is present, at most Now plus
// Leeway and at least FlowCreatedAt minus Leeway; the HMAC of nonce equals NonceHash; sub is a
// non-empty valid UTF-8 text without NUL. Errors wrap ErrClaims and name the rule only.
func CheckIDToken(c Claims, e Expectation) (string, error) {
	issuer, err := checkIssuerAndAudience(c, e)
	if err != nil {
		return "", err
	}
	if err := checkTimesNonceSubject(c, e); err != nil {
		return "", err
	}
	return issuer, nil
}

// checkIssuerAndAudience applies the rules of iss, tid, aud and azp and returns the matched
// issuer.
func checkIssuerAndAudience(c Claims, e Expectation) (string, error) {
	issuer, ok := expectedIssuer(e.Issuer, e.AllowedTenants, c.TenantID)
	switch {
	case !ok || c.Issuer != issuer:
		return "", claimError("iss")
	case len(e.AllowedTenants) > 0 && !slices.Contains(e.AllowedTenants, c.TenantID):
		return "", claimError("tid")
	case !slices.Contains(c.Audience, e.ClientID):
		return "", claimError("aud")
	case len(c.Audience) > 1 && c.AZP == "" || c.AZP != "" && c.AZP != e.ClientID:
		return "", claimError("azp")
	}
	return issuer, nil
}

// checkTimesNonceSubject applies the rules of exp, iat, nonce and sub.
func checkTimesNonceSubject(c Claims, e Expectation) error {
	switch {
	case !c.HasExpiry || !c.Expiry.After(e.Now.Add(-e.Leeway)):
		return claimError("exp")
	case !c.HasIssuedAt || c.IssuedAt.After(e.Now.Add(e.Leeway)) || c.IssuedAt.Before(e.FlowCreatedAt.Add(-e.Leeway)):
		return claimError("iat")
	case c.Nonce == "" || !HashEqual(Hash(e.Pepper, HashNonce, c.Nonce), e.NonceHash):
		return claimError("nonce")
	case c.Subject == "" || !utf8.ValidString(c.Subject) || strings.IndexByte(c.Subject, 0) >= 0:
		return claimError("sub")
	}
	return nil
}

// expectedIssuer returns the issuer a token of tenant tid must name: configured itself without
// placeholder, else configured with the placeholder replaced by tid, which must be non-empty,
// free of the placeholder characters and a "/" and in allowed when allowed is set.
func expectedIssuer(configured string, allowed []string, tid string) (string, bool) {
	if !strings.Contains(configured, TenantPlaceholder) {
		return configured, true
	}
	if tid == "" || strings.ContainsAny(tid, "{}/?#") || len(allowed) > 0 && !slices.Contains(allowed, tid) {
		return "", false
	}
	return strings.Replace(configured, TenantPlaceholder, tid, 1), true
}

// IssuerMatches reports whether iss (the iss response parameter of RFC 9207) names the configured
// issuer: equal to it, or for a multi-tenant issuer equal to it with the placeholder replaced by
// one of allowed.
func IssuerMatches(configured string, allowed []string, iss string) bool {
	if !strings.Contains(configured, TenantPlaceholder) {
		return iss == configured
	}
	for _, tid := range allowed {
		if want, ok := expectedIssuer(configured, allowed, tid); ok && want == iss {
			return true
		}
	}
	return false
}
