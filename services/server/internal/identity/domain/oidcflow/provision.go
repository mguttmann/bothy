// SPDX-License-Identifier: Apache-2.0

package oidcflow

import (
	"errors"
	"fmt"
	"io"
	"strings"
	"unicode"
	"unicode/utf8"
)

// fallbackHandle is the handle of a provisioned person whose claims yield none.
const fallbackHandle = "user"

// ProvisionedHandle derives the handle of a provisioned person (brief M1-41 "Provisioned handle and
// display name"): from the local part of preferredUsername, else from name, else "user";
// lower-cased; every character outside [a-z0-9._-] becomes "-"; leading characters up to the first
// [a-z0-9] are dropped; the result is cut to maxChars (principal.handle_max_chars). The result
// always matches the handle pattern of 03.
func ProvisionedHandle(preferredUsername, name string, maxChars int) string {
	local, _, _ := strings.Cut(preferredUsername, "@")
	for _, src := range []string{local, name} {
		if h := handleFrom(src, maxChars); h != "" {
			return h
		}
	}
	return cut(fallbackHandle, maxChars)
}

// handleFrom applies the character rules to s; "" when nothing remains.
func handleFrom(s string, maxChars int) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '.' || r == '_' || r == '-' {
			b.WriteRune(r)
		} else {
			b.WriteByte('-')
		}
	}
	h := strings.TrimLeftFunc(b.String(), func(r rune) bool { return (r < 'a' || r > 'z') && (r < '0' || r > '9') })
	return cut(h, maxChars)
}

// cut returns the first n bytes of an ASCII string s.
func cut(s string, n int) string {
	if n < 0 {
		n = 0
	}
	if len(s) > n {
		return s[:n]
	}
	return s
}

// suffixAlphabet are the characters of a collision suffix.
const suffixAlphabet = "abcdefghijklmnopqrstuvwxyz0123456789"

// suffixLen is the length of a collision suffix.
const suffixLen = 4

// errSuffixRandom reports a random source that delivered no usable bytes.
var errSuffixRandom = errors.New("oidcflow: the random source delivered too few bytes for a handle suffix")

// NewHandleSuffix returns four characters of [a-z0-9] from r, with rejection sampling so that
// every character is equally likely. Errors: the read error or a source that runs dry.
func NewHandleSuffix(r io.Reader) (string, error) {
	out := make([]byte, 0, suffixLen)
	buf := make([]byte, 16)
	for len(out) < suffixLen {
		if _, err := io.ReadFull(r, buf); err != nil {
			if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
				return "", errSuffixRandom
			}
			return "", fmt.Errorf("oidcflow: random source: %w", err)
		}
		out = appendSuffixChars(out, buf)
	}
	return string(out), nil
}

// appendSuffixChars appends to out the characters of the bytes of buf below the largest multiple
// of the alphabet size (the others are rejected), until out holds suffixLen characters.
func appendSuffixChars(out, buf []byte) []byte {
	limit := byte(256 - 256%len(suffixAlphabet))
	for _, c := range buf {
		if c < limit && len(out) < suffixLen {
			out = append(out, suffixAlphabet[int(c)%len(suffixAlphabet)])
		}
	}
	return out
}

// HandleWithSuffix appends "-" and suffix to base within maxChars: base is shortened so that the
// whole fits; below six characters the suffix alone (cut to maxChars) is the handle.
func HandleWithSuffix(base, suffix string, maxChars int) string {
	if maxChars < len(suffix)+2 {
		return cut(suffix, maxChars)
	}
	return cut(base, maxChars-len(suffix)-1) + "-" + suffix
}

// SanitizeDisplayName removes from s every invalid UTF-8 byte, control character (Cc) and format
// character (Cf, which includes the bidi controls), trims white space and cuts the result to
// maxChars runes, so that a provider value cannot block the login (brief M1-41). An empty result
// means the caller uses the handle.
func SanitizeDisplayName(s string, maxChars int) string {
	var b strings.Builder
	for len(s) > 0 {
		r, size := utf8.DecodeRuneInString(s)
		s = s[size:]
		if r == utf8.RuneError && size <= 1 || unicode.Is(unicode.Cc, r) || unicode.Is(unicode.Cf, r) {
			continue
		}
		b.WriteRune(r)
	}
	out := strings.TrimSpace(b.String())
	if utf8.RuneCountInString(out) > maxChars {
		out = strings.TrimSpace(string([]rune(out)[:maxChars]))
	}
	return out
}
