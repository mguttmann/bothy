// SPDX-License-Identifier: Apache-2.0

// Package dpop holds the pure checks of DPoP proofs (RFC 9449; docs/spec/08-identitaet.md "7.3
// DPoP", brief M1-33 "DPoP proof checks"): the htu normalization, the claim checks, the
// stateless server nonces and the replay key. Keys, times and limits are parameters; nothing here
// reads a clock, the configuration or a store. identity/app composes them into the verifier.
//
// Every rejection is a *ProofError whose Code is CodeInvalidProof or CodeUseNonce and whose Reason
// names the internal cause for the debug log; the reason never reaches a client (no oracle).
//
// Example:
//
//	c, err := dpop.ParseClaims(jws.Payload, accessToken != "")
//	htu, err := dpop.CheckRequest(c, dpop.Expect{Method: "GET", URL: url, Now: now, MaxAge: maxAge, Leeway: leeway})
//	stale, err := dpop.CheckNonce(key, c.Nonce, now, nonceTTL)
package dpop

import (
	"bytes"
	"crypto/hkdf"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"net"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// The two problem codes of a failed proof (06 section 5).
const (
	// CodeInvalidProof is invalid_dpop_proof.
	CodeInvalidProof = "invalid_dpop_proof"
	// CodeUseNonce is use_dpop_nonce: the client retries with the nonce of the DPoP-Nonce header.
	CodeUseNonce = "use_dpop_nonce"
)

// ProofError is a rejected proof. Code goes into the problem body; Reason (for example
// "htu_mismatch") only into the debug log.
type ProofError struct {
	// Code is CodeInvalidProof or CodeUseNonce.
	Code string
	// Reason is the internal cause.
	Reason string
}

// Error returns "dpop: <code> (<reason>)".
func (f *ProofError) Error() string { return "dpop: " + f.Code + " (" + f.Reason + ")" }

// invalid returns an invalid_dpop_proof failure with reason.
func invalid(reason string) *ProofError { return &ProofError{Code: CodeInvalidProof, Reason: reason} }

// useNonce returns a use_dpop_nonce failure with reason.
func useNonce(reason string) *ProofError { return &ProofError{Code: CodeUseNonce, Reason: reason} }

// NormalizeHTU normalizes an absolute http or https URL for the htu comparison (brief M1-33 check
// 5): scheme and host in lower case, the default port removed (443 for https, 80 for http), query
// and fragment dropped, an empty path becomes "/", percent-encodings normalized (upper-case hex,
// unreserved characters decoded). The path is otherwise kept exactly. A URL with user info, an
// opaque URL, a relative URL and an invalid percent-encoding are errors.
func NormalizeHTU(raw string) (string, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return "", errors.New("dpop: htu is not a URL")
	}
	scheme := strings.ToLower(u.Scheme)
	if scheme != "https" && scheme != "http" || u.Opaque != "" || u.User != nil || u.Host == "" {
		return "", errors.New("dpop: htu must be an absolute http or https URL without user info")
	}
	host, port := strings.ToLower(u.Hostname()), u.Port()
	if host == "" {
		return "", errors.New("dpop: htu has no host")
	}
	if port != "" {
		if _, err := strconv.ParseUint(port, 10, 16); err != nil {
			return "", errors.New("dpop: htu has an invalid port")
		}
	}
	if scheme == "https" && port == "443" || scheme == "http" && port == "80" {
		port = ""
	}
	authority := host
	switch {
	case port != "":
		authority = net.JoinHostPort(host, port)
	case strings.Contains(host, ":"):
		authority = "[" + host + "]"
	}
	path, err := normalizePercent(u.EscapedPath())
	if err != nil {
		return "", err
	}
	if path == "" {
		path = "/"
	}
	return scheme + "://" + authority + path, nil
}

// normalizePercent rewrites every %XX of p: unreserved characters (RFC 3986 section 2.3) are
// decoded, every other octet keeps its encoding with upper-case hex.
func normalizePercent(p string) (string, error) {
	var b strings.Builder
	for i := 0; i < len(p); i++ {
		if p[i] != '%' {
			b.WriteByte(p[i])
			continue
		}
		if i+2 >= len(p) {
			return "", errors.New("dpop: htu has an invalid percent-encoding")
		}
		v, err := strconv.ParseUint(p[i+1:i+3], 16, 8)
		if err != nil {
			return "", errors.New("dpop: htu has an invalid percent-encoding")
		}
		c := byte(v)
		if isUnreserved(c) {
			b.WriteByte(c)
		} else {
			b.WriteString("%" + strings.ToUpper(p[i+1:i+3]))
		}
		i += 2
	}
	return b.String(), nil
}

func isUnreserved(c byte) bool {
	return c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '-' || c == '.' || c == '_' || c == '~'
}

// Claims are the payload members of a proof that the checks read.
type Claims struct {
	// JTI is the unique identifier of the proof.
	JTI string
	// HTM is the HTTP method.
	HTM string
	// HTU is the HTTP target URI as the client wrote it.
	HTU string
	// IAT is the creation time (NumericDate, whole seconds).
	IAT time.Time
	// Nonce is the server nonce.
	Nonce string
	// ATH is the hash of the access token; HasATH reports its presence.
	ATH    string
	HasATH bool
}

// ParseClaims reads the payload of a proof (check 3): jti a non-empty string, htm and htu
// strings, iat an integer NumericDate, nonce a string, ath a string exactly when withToken. A
// missing nonce is CodeUseNonce; every other missing or mistyped member is CodeInvalidProof.
func ParseClaims(payload []byte, withToken bool) (Claims, error) {
	var all map[string]json.RawMessage
	dec := json.NewDecoder(bytes.NewReader(payload))
	dec.UseNumber()
	if err := dec.Decode(&all); err != nil || all == nil {
		return Claims{}, invalid("payload_not_object")
	}
	var c Claims
	for _, f := range []struct {
		name string
		dst  *string
	}{{"jti", &c.JTI}, {"htm", &c.HTM}, {"htu", &c.HTU}} {
		if !stringMember(all, f.name, f.dst) {
			return Claims{}, invalid(f.name + "_invalid")
		}
	}
	if c.JTI == "" {
		return Claims{}, invalid("jti_invalid")
	}
	iat, ok := all["iat"]
	if !ok {
		return Claims{}, invalid("iat_invalid")
	}
	secs, err := strconv.ParseInt(string(iat), 10, 64)
	if err != nil {
		return Claims{}, invalid("iat_invalid")
	}
	c.IAT = time.Unix(secs, 0).UTC()
	if _, ok := all["nonce"]; !ok {
		return Claims{}, useNonce("nonce_missing")
	}
	if !stringMember(all, "nonce", &c.Nonce) {
		return Claims{}, invalid("nonce_invalid")
	}
	_, c.HasATH = all["ath"]
	if c.HasATH != withToken {
		return Claims{}, invalid("ath_presence")
	}
	if c.HasATH && !stringMember(all, "ath", &c.ATH) {
		return Claims{}, invalid("ath_invalid")
	}
	return c, nil
}

// stringMember reads the string member name into dst; false when it is absent, null or no
// string (a null member is mistyped, not missing).
func stringMember(all map[string]json.RawMessage, name string, dst *string) bool {
	v, ok := all[name]
	return ok && len(v) > 0 && v[0] == '"' && json.Unmarshal(v, dst) == nil
}

// Expect is what the request expects of a proof.
type Expect struct {
	// Method is the request method; htm must equal it exactly.
	Method string
	// URL is the absolute expected htu (server.public_url plus the request path).
	URL string
	// Now is the current time.
	Now time.Time
	// MaxAge is auth.dpop_proof_max_age; Leeway is auth.clock_leeway.
	MaxAge, Leeway time.Duration
}

// CheckRequest applies checks 4 to 6: htm equals the method exactly, the normalized htu equals
// the normalized URL, iat is not older than MaxAge and not later than Now plus Leeway. It returns
// the normalized htu, the base of the replay key. Every failure is CodeInvalidProof.
func CheckRequest(c Claims, e Expect) (string, error) {
	if c.HTM != e.Method {
		return "", invalid("htm_mismatch")
	}
	want, err := NormalizeHTU(e.URL)
	if err != nil {
		return "", invalid("expected_htu_invalid")
	}
	got, err := NormalizeHTU(c.HTU)
	if err != nil || got != want {
		return "", invalid("htu_mismatch")
	}
	if c.IAT.Before(e.Now.Add(-e.MaxAge)) {
		return "", invalid("iat_too_old")
	}
	if c.IAT.After(e.Now.Add(e.Leeway)) {
		return "", invalid("iat_in_future")
	}
	return want, nil
}

// ATH returns base64url (no padding) of SHA-256 over the ASCII bytes of an access token.
func ATH(accessToken string) string {
	sum := sha256.Sum256([]byte(accessToken))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

// CheckBinding applies check 8: with accessToken the proof's ath equals ATH(accessToken) (and
// without one it has none); with expectedJKT the proof key's thumbprint jkt equals it. Failures are
// CodeInvalidProof. The comparisons are constant time.
func CheckBinding(c Claims, accessToken, jkt, expectedJKT string) error {
	if (accessToken != "") != c.HasATH {
		return invalid("ath_presence")
	}
	if accessToken != "" && !hmac.Equal([]byte(c.ATH), []byte(ATH(accessToken))) {
		return invalid("ath_mismatch")
	}
	if expectedJKT != "" && !hmac.Equal([]byte(jkt), []byte(expectedJKT)) {
		return invalid("jkt_mismatch")
	}
	return nil
}

// nonceInfo is the HKDF info of the nonce key. It carries no product name (02 "6. Code-Layout").
const nonceInfo = "dpop-nonce/v1"

// The layout of a nonce: the window as uint64 big-endian, then the first 16 bytes of the MAC.
const (
	windowSize = 8
	macSize    = 16
	nonceSize  = windowSize + macSize
	keySize    = 32
)

// NonceKey derives the nonce key from the token pepper: HKDF-SHA256 with empty salt and info
// "dpop-nonce/v1", 32 bytes. Every process with the same pepper derives the same key, so nonces
// hold across processes and restarts. An empty pepper is an error.
func NonceKey(pepper []byte) ([]byte, error) {
	if len(pepper) == 0 {
		return nil, errors.New("dpop: empty token pepper")
	}
	return hkdf.Key(sha256.New, pepper, nil, nonceInfo, keySize)
}

// NonceWindow returns floor(unix seconds of now / seconds of ttl); ttl below one second counts as
// one second, a time before 1970 as window 0.
func NonceWindow(now time.Time, ttl time.Duration) uint64 {
	secs := max(int64(ttl/time.Second), 1)
	unix := max(now.Unix(), 0)
	return uint64(unix / secs)
}

// EncodeNonce returns the nonce of window w: base64url without padding of
// uint64be(w) || first 16 bytes of HMAC-SHA256(key, uint64be(w)).
func EncodeNonce(key []byte, w uint64) string {
	var buf [nonceSize]byte
	binary.BigEndian.PutUint64(buf[:windowSize], w)
	copy(buf[windowSize:], nonceMAC(key, buf[:windowSize]))
	return base64.RawURLEncoding.EncodeToString(buf[:])
}

func nonceMAC(key, window []byte) []byte {
	m := hmac.New(sha256.New, key)
	m.Write(window)
	return m.Sum(nil)[:macSize]
}

// CheckNonce applies check 7: nonce decodes to 24 bytes, its MAC is equal (hmac.Equal) and its
// window is the current one or the previous one; stale is true for the previous window, so the
// response carries the current nonce. Every failure is CodeUseNonce.
func CheckNonce(key []byte, nonce string, now time.Time, ttl time.Duration) (stale bool, err error) {
	if strings.ContainsAny(nonce, "=+/ \r\n\t") {
		return false, useNonce("nonce_malformed")
	}
	raw, decErr := base64.RawURLEncoding.Strict().DecodeString(nonce)
	if decErr != nil || len(raw) != nonceSize {
		return false, useNonce("nonce_malformed")
	}
	if !hmac.Equal(raw[windowSize:], nonceMAC(key, raw[:windowSize])) {
		return false, useNonce("nonce_forged")
	}
	w, cur := binary.BigEndian.Uint64(raw[:windowSize]), NonceWindow(now, ttl)
	switch {
	case w == cur:
		return false, nil
	case cur > 0 && w == cur-1:
		return true, nil
	}
	return false, useNonce("nonce_expired")
}

// ReplayKey is the key of the jti store: SHA-256(normalized htu || 0x00 || jti), so a proof is
// accepted once per htu and the store holds no client text.
func ReplayKey(normalizedHTU, jti string) [32]byte {
	return sha256.Sum256([]byte(normalizedHTU + "\x00" + jti))
}
