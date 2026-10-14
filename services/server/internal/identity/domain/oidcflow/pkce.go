// SPDX-License-Identifier: Apache-2.0

package oidcflow

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"net/netip"
	"net/url"
	"slices"
	"strconv"
	"strings"

	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
)

// The field pointers of the begin request (schemas OidcBeginRequest and IdentityLinkBegin).
const (
	pointerRedirect  = "/app_redirect_uri"
	pointerChallenge = "/client_challenge"
)

// challengeLen is the length of base64url(SHA-256) without padding.
const challengeLen = 43

// The bounds of a PKCE verifier (RFC 7636 section 4.1).
const (
	verifierMin = 43
	verifierMax = 128
)

// S256 returns base64url(SHA-256(verifier)) without padding (RFC 7636 section 4.2).
func S256(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

// ValidVerifier reports a verifier of RFC 7636 section 4.1: 43 to 128 characters of the unreserved
// set A-Z, a-z, 0-9, "-", ".", "_", "~".
func ValidVerifier(v string) bool {
	if len(v) < verifierMin || len(v) > verifierMax {
		return false
	}
	for i := 0; i < len(v); i++ {
		if !unreserved(v[i]) {
			return false
		}
	}
	return true
}

// unreserved reports a byte of the unreserved set of RFC 3986.
func unreserved(c byte) bool {
	return c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '-' || c == '.' || c == '_' || c == '~'
}

// base64url reports a byte of the base64url alphabet.
func base64url(c byte) bool {
	return c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '-' || c == '_'
}

// CheckClientChallenge checks client_challenge: exactly 43 base64url characters without padding,
// the form of an S256 challenge. Errors: validation_failed at /client_challenge.
func CheckClientChallenge(c string) error {
	ok := len(c) == challengeLen
	for i := 0; ok && i < len(c); i++ {
		ok = base64url(c[i])
	}
	if !ok {
		return fieldError(pointerChallenge, "The client challenge must be 43 base64url characters (S256).")
	}
	return nil
}

// VerifierMatches reports a valid verifier whose S256 equals challenge, compared in constant time
// (08 section 2.2 step 7).
func VerifierMatches(verifier, challenge string) bool {
	if !ValidVerifier(verifier) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(S256(verifier)), []byte(challenge)) == 1
}

// loopbackPath is the only path of a loopback redirect (RFC 8252 section 7.3, 08 section 2.2).
const loopbackPath = "/callback"

// CheckAppRedirectURI checks app_redirect_uri at begin (08 section 2.2 step 2, brief M1-41
// "Redirect URI rules"): an exact string match with an entry of allowed (private-use scheme or
// claimed https URL of the provider configuration), or http://127.0.0.1:<port>/callback or
// http://[::1]:<port>/callback with a port of 1 to 65535 written without leading zero. Refused:
// localhost, every other http host, user information, query, fragment and every other loopback
// path. An entry of allowed with user information or a fragment never matches. Errors:
// validation_failed at /app_redirect_uri.
func CheckAppRedirectURI(uri string, allowed []string) error {
	if uri != "" && slices.Contains(allowed, uri) && !strings.ContainsAny(uri, "@#") {
		return nil
	}
	if loopbackRedirect(uri) {
		return nil
	}
	return fieldError(pointerRedirect, "The redirect URI is neither a loopback callback nor an entry of the provider.")
}

// loopbackRedirect reports http://127.0.0.1:<port>/callback or http://[::1]:<port>/callback.
func loopbackRedirect(uri string) bool {
	rest, ok := strings.CutPrefix(uri, "http://")
	if !ok {
		return false
	}
	hostPort, path, ok := strings.Cut(rest, "/")
	if !ok || "/"+path != loopbackPath {
		return false
	}
	u, err := url.Parse(uri)
	if err != nil || u.User != nil || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" || u.RawPath != "" {
		return false
	}
	var host, port string
	switch {
	case strings.HasPrefix(hostPort, "127.0.0.1:"):
		host, port = "127.0.0.1", strings.TrimPrefix(hostPort, "127.0.0.1:")
	case strings.HasPrefix(hostPort, "[::1]:"):
		host, port = "::1", strings.TrimPrefix(hostPort, "[::1]:")
	default:
		return false
	}
	if a, err := netip.ParseAddr(host); err != nil || !a.IsLoopback() {
		return false
	}
	return validPort(port)
}

// validPort reports a decimal port of 1 to 65535 without sign or leading zero.
func validPort(p string) bool {
	if p == "" || p[0] == '0' || len(p) > 5 {
		return false
	}
	for i := 0; i < len(p); i++ {
		if p[i] < '0' || p[i] > '9' {
			return false
		}
	}
	n, err := strconv.Atoi(p)
	return err == nil && n >= 1 && n <= 65535
}

// fieldError is validation_failed with one field error of code invalid.
func fieldError(pointer, detail string) *errs.Error {
	return errs.New(errs.CodeValidationFailed, detail).WithFields(errs.FieldError{Pointer: pointer, Code: "invalid", Detail: detail})
}
