// SPDX-License-Identifier: Apache-2.0

package secrets

import (
	"bytes"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"io"
	"log/slog"
)

// redacted is the only rendering a Value has.
const redacted = "[redacted]"

// Value holds a secret and renders as "[redacted]" in every output path: fmt with any verb,
// slog, encoding/json and encoding.TextMarshaler (SEC-SEC-3, SEC-LOG-1). Only Bytes hands out the
// content, as a copy.
//
// The content sits behind a pointer to an unexported holder. fmt bypasses Format when it prints
// through reflection, for a Value in an unexported struct field (%v, %+v, %#v, %x) or for the
// verb %p; it then shows the address of the holder, never its bytes. The pointer also makes Value
// comparable, so it can be a map key, but == compares identity: use Equal for content.
//
// The zero Value is an empty secret. A Value is immutable and safe for concurrent use.
type Value struct {
	h *holder
}

// holder keeps the bytes out of reach of reflection on Value (see Value).
type holder struct {
	b []byte
}

// NewValue returns a Value holding a copy of b, so later changes to b do not reach it.
func NewValue(b []byte) Value {
	return Value{h: &holder{b: bytes.Clone(b)}}
}

// Bytes returns a copy of the secret, nil for the zero Value. The caller owns the copy and should
// clear it once it is no longer needed.
func (v Value) Bytes() []byte {
	if v.h == nil {
		return nil
	}
	return bytes.Clone(v.h.b)
}

// Len returns the length of the secret in bytes. A length says little about a generated secret
// and is safe to log.
func (v Value) Len() int {
	if v.h == nil {
		return 0
	}
	return len(v.h.b)
}

// Equal reports whether v and o hold the same bytes. The comparison takes the same time for
// every content of a given length (crypto/subtle), so it does not leak how much of a guess
// matched; the length itself is not hidden.
func (v Value) Equal(o Value) bool {
	return subtle.ConstantTimeCompare(v.content(), o.content()) == 1
}

func (v Value) content() []byte {
	if v.h == nil {
		return nil
	}
	return v.h.b
}

// String returns "[redacted]".
func (Value) String() string { return redacted }

// GoString returns "[redacted]". fmt calls Format even for %#v; GoString serves code that asks
// for fmt.GoStringer directly.
func (Value) GoString() string { return redacted }

// Format writes "[redacted]" for every verb and ignores flags, width and precision, so no verb
// (%x, %q, %d, ...) can render the bytes or their length.
func (Value) Format(f fmt.State, _ rune) {
	_, _ = io.WriteString(f, redacted)
}

// LogValue implements slog.LogValuer: every slog handler logs "[redacted]".
func (Value) LogValue() slog.Value { return slog.StringValue(redacted) }

// MarshalJSON returns the JSON string "[redacted]".
func (Value) MarshalJSON() ([]byte, error) { return []byte(`"` + redacted + `"`), nil }

// MarshalText returns "[redacted]"; encoding/json uses it for a Value as map key.
func (Value) MarshalText() ([]byte, error) { return []byte(redacted), nil }

// Version identifies the content of a secret without revealing it: for the file and env drivers
// the first 16 hex characters (64 bits) of the SHA-256 of the value, so equal content gives an
// equal version and a rotation gives a new one. It is a shortened hash, not a secret, but it still
// does not belong in logs: for a low-entropy value it would allow checking guesses.
type Version string

// versionOf computes the Version of content as the file and env drivers define it.
func versionOf(content []byte) Version {
	sum := sha256.Sum256(content)
	return Version(hex.EncodeToString(sum[:8]))
}
