// SPDX-License-Identifier: Apache-2.0

package domain

import (
	"regexp"
	"slices"
	"strconv"
	"time"
)

// The field error codes of the token rules besides FieldRequired and FieldPattern.
const (
	// FieldNotAllowed marks a member the token kind does not take (scopes of agent and personal
	// tokens).
	FieldNotAllowed = "not_allowed"
	// FieldOutOfRange marks an expiry not after now or beyond the configured maximum.
	FieldOutOfRange = "out_of_range"
)

// The JSON pointers of ApiTokenCreate (docs/api/openapi.yaml).
const (
	pointerTokenName    = "/name"
	pointerTokenScopes  = "/scopes"
	pointerTokenExpires = "/expires_at"
	pointerTokenJKT     = "/dpop_jkt" //nolint:gosec // G101: a JSON pointer, no credential
)

// jktPattern is an RFC 7638 SHA-256 thumbprint: 32 bytes as unpadded base64url, 43 characters.
// Go's regexp runs in linear time (SEC-IN-6).
var jktPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{43}$`)

// CheckTokenName validates api_tokens.name: at least one visible character, no control, format
// or bidi characters (SEC-IN-4). Errors: validation_failed on /name.
func CheckTokenName(s string) error {
	return displayText(pointerTokenName, "name", s)
}

// CheckTokenScopes requires an empty scope list for the kinds that take none: agent tokens hold
// their scopes implicitly (05 section 2.6) and personal tokens act with the grants of the person
// (08 section 12). takesScopes is true for app tokens only. Errors: validation_failed on
// /scopes.
func CheckTokenScopes(takesScopes bool, scopes []string) error {
	if !takesScopes && len(scopes) > 0 {
		return fieldError(pointerTokenScopes, FieldNotAllowed, "this token kind carries no scopes")
	}
	return nil
}

// AppTokenScopes returns the scopes of a new app token (M2-24 design note 10 a): the granted
// scopes of the installation at this moment when requested is empty, else requested sorted, each
// of which must be granted and named once. Errors: validation_failed on /scopes/<i> with code
// not_allowed.
func AppTokenScopes(requested, granted []string) ([]string, error) {
	if len(requested) == 0 {
		out := append([]string{}, granted...)
		slices.Sort(out)
		return slices.Compact(out), nil
	}
	seen := map[string]bool{}
	for i, s := range requested {
		p := pointerTokenScopes + "/" + strconv.Itoa(i)
		switch {
		case seen[s]:
			return nil, fieldError(p, FieldNotAllowed, "the scope is named twice")
		case !slices.Contains(granted, s):
			return nil, fieldError(p, FieldNotAllowed, "the installation does not grant this scope")
		}
		seen[s] = true
	}
	out := slices.Clone(requested)
	slices.Sort(out)
	return out, nil
}

// CheckTokenExpiry requires an expiry after now and at most maxTTL ahead (08 section 12:
// auth.api_token_max_ttl for agents and apps, auth.personal_token_max_ttl for personal tokens).
// Errors: validation_failed on /expires_at.
func CheckTokenExpiry(expiresAt, now time.Time, maxTTL time.Duration) error {
	switch {
	case expiresAt.IsZero():
		return fieldError(pointerTokenExpires, FieldRequired, "an API token needs an expiry")
	case !expiresAt.After(now), expiresAt.After(now.Add(maxTTL)):
		return fieldError(pointerTokenExpires, FieldOutOfRange, "the expiry must lie after now and within the configured maximum")
	}
	return nil
}

// CheckTokenJKT validates the optional DPoP binding: an RFC 7638 SHA-256 thumbprint of 43
// base64url characters, required when the token kind is in auth.dpop_required_for. Errors:
// validation_failed on /dpop_jkt.
func CheckTokenJKT(jkt string, required bool) error {
	switch {
	case jkt == "" && required:
		return fieldError(pointerTokenJKT, FieldRequired, "this token kind must be bound to a DPoP key")
	case jkt != "" && !jktPattern.MatchString(jkt):
		return fieldError(pointerTokenJKT, FieldPattern, "the thumbprint must be 43 base64url characters")
	}
	return nil
}
