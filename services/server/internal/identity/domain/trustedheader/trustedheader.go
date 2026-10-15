// SPDX-License-Identifier: Apache-2.0

// Package trustedheader holds the pure rules of the identity driver trusted_header
// (docs/spec/08-identity.md section 2.4, brief M1-43): whether the bound public listener is
// loopback, whether the TCP peer may set identity headers, how a header value is decoded (RFC 2047)
// and validated, the configuration identity_provider.trusted_header.v1 and the handle a provisioned
// person gets from the login.
//
// The package reads no request, no configuration key and no database. It never logs and never
// puts a header value into an error, because the values are personal data (19 SEC-LOG-2).
//
// Example:
//
//	login, outcome := trustedheader.Evaluate(peer, trustedProxies, r.Header.Values(cfg.LoginHeader))
//	if outcome != trustedheader.Present {
//		// anonymous: the login answers 401
//	}
package trustedheader

import (
	"encoding/json"
	"mime"
	"net"
	"net/netip"
	"regexp"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/OWNER/bothy/services/server/internal/identity/domain/provider"
	"github.com/OWNER/bothy/services/server/internal/kernel/textcheck"
)

// Config is identity_provider.trusted_header.v1 after normalization: LoginHeader carries the
// login, NameHeader ("" for none) the display name at provisioning, both in canonical form.
type Config = provider.TrustedHeaderConfig

// ParseConfig validates raw strictly as identity_provider.trusted_header.v1 (08 section 2.6) and
// returns it with canonical header names. Errors: validation_failed with field errors under
// /config for an unknown member, a missing or invalid header name and a name header equal to the
// login header.
func ParseConfig(raw json.RawMessage) (Config, error) {
	norm, err := provider.ValidateTrustedHeader(raw)
	if err != nil {
		return Config{}, err
	}
	var c Config
	if err := json.Unmarshal(norm, &c); err != nil {
		return Config{}, err
	}
	return c, nil
}

// IsLoopbackListener reports whether addr, the address a listener is bound to, is a loopback
// address (127.0.0.0/8 or ::1). The unspecified addresses 0.0.0.0 and ::, every other address,
// a non-TCP address and nil are not: only a loopback listener keeps everyone but the local proxy
// from setting the identity headers (08 section 2.4 rule 1). The bound address is the fact; a
// configured host name is not.
func IsLoopbackListener(addr net.Addr) bool {
	tcp, ok := addr.(*net.TCPAddr)
	if !ok || tcp == nil {
		return false
	}
	return tcp.IP.IsLoopback()
}

// Outcome is the result of evaluating the login header of one request.
type Outcome int

// The outcomes of Evaluate. Every outcome but Present means the header counts as absent.
const (
	// Present is exactly one valid value from a trusted peer.
	Present Outcome = iota + 1
	// UntrustedPeer is a peer outside server.trusted_proxies (or no peer): headers are ignored.
	UntrustedPeer
	// Missing is no value.
	Missing
	// Multiple is more than one field line; the caller logs a warning without the values.
	Multiple
	// Invalid is a value that does not decode or breaks the character rules.
	Invalid
)

// Evaluate returns the decoded value of the header lines values when peer, the IP of the TCP
// connection (never a forwarded address), lies in trusted, there is exactly one line and it
// decodes (DecodeValue). Otherwise it returns "" and the reason (08 section 2.4 rules 2, 3 and 6).
func Evaluate(peer netip.Addr, trusted []netip.Prefix, values []string) (string, Outcome) {
	if !peer.IsValid() || !slices.ContainsFunc(trusted, func(p netip.Prefix) bool { return p.Contains(peer.Unmap()) }) {
		return "", UntrustedPeer
	}
	switch len(values) {
	case 0:
		return "", Missing
	case 1:
	default:
		return "", Multiple
	}
	v, ok := DecodeValue(values[0])
	if !ok {
		return "", Invalid
	}
	return v, Present
}

// encodedWord finds the charset of every RFC 2047 encoded word (with an optional RFC 2231
// language after "*"). The text runs to the first "?=", as package mime reads it, so a "?" inside
// the text cannot hide a word from the charset check.
var encodedWord = regexp.MustCompile(`=\?([^?*]*)(?:\*[^?]*)?\?[bBqQ]\?.*?\?=`)

// allowedCharset reports the two charsets 08 section 2.4 rule 6 admits.
func allowedCharset(cs string) bool {
	return strings.EqualFold(cs, "utf-8") || strings.EqualFold(cs, "us-ascii")
}

// DecodeValue decodes the RFC 2047 encoded words of v (charsets utf-8 and us-ascii only; the
// decoder of package mime also knows iso-8859-1, which is refused first) and reports whether the
// result is a usable identity value: not empty, valid UTF-8, no control (Cc), format (Cf) or bidi
// control character. Any other charset or a malformed encoded word makes the value absent.
func DecodeValue(v string) (string, bool) {
	dec := new(mime.WordDecoder)
	for _, m := range encodedWord.FindAllStringSubmatch(v, -1) {
		// DecodeHeader leaves a malformed word as it is; decoding each word alone reports it.
		if _, err := dec.Decode(m[0]); err != nil || !allowedCharset(m[1]) {
			return "", false
		}
	}
	out, err := dec.DecodeHeader(v)
	if err != nil || out == "" || !utf8.ValidString(out) || textcheck.DisplayText(out) != nil {
		return "", false
	}
	for _, r := range out {
		if unicode.Is(unicode.Cf, r) {
			return "", false
		}
	}
	return out, true
}

// fallbackHandle is the handle of a login whose part before "@" keeps no allowed character.
const fallbackHandle = "user"

// HandleFromLogin derives the handle of a provisioned person from login (08 section 3.1 item 3):
// the part before the first "@", lower case, only a-z, 0-9, ".", "_" and "-", starting with a
// letter or digit, cut to maxChars bytes (every kept character is one byte). A login that keeps
// nothing gives "user". The rule is the one brief M1-41 names for OIDC; M1-41 is not merged, so it
// lives here until both drivers share one helper.
func HandleFromLogin(login string, maxChars int) string {
	local, _, _ := strings.Cut(login, "@")
	var b strings.Builder
	for _, r := range strings.ToLower(local) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case (r == '.' || r == '_' || r == '-') && b.Len() > 0:
			b.WriteRune(r)
		}
	}
	h := b.String()
	if h == "" {
		h = fallbackHandle
	}
	if maxChars > 0 && len(h) > maxChars {
		h = h[:maxChars]
	}
	return h
}

// HandleWithSuffix returns base with "-" and suffix appended, base cut so the result has at most
// maxChars bytes; a taken handle gets a random suffix this way (08 section 3.1 item 3). suffix
// must consist of a-z and 0-9.
func HandleWithSuffix(base, suffix string, maxChars int) string {
	keep := maxChars - len(suffix) - 1
	if keep < 1 {
		return suffix[:min(len(suffix), max(maxChars, 1))]
	}
	if len(base) > keep {
		base = base[:keep]
	}
	return base + "-" + suffix
}
