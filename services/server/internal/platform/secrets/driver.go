// SPDX-License-Identifier: Apache-2.0

package secrets

import "context"

// Driver reads secrets of one scheme (docs/spec/16-betrieb.md 5.2: Get(ref) -> (value, version)).
//
// Get returns the current value and its Version, or a *RefError wrapping ErrInvalidRef,
// ErrUnsupportedScheme, ErrNotFound, ErrInsecureFile or ErrReadFailed, or the context error. It
// never caches: caching and rotation belong to Provider. Implementations are safe for concurrent
// use.
type Driver interface {
	// Scheme returns the scheme the driver reads, for example "file".
	Scheme() string
	// Get reads the secret ref names; ref.Scheme equals Scheme.
	Get(ctx context.Context, ref Ref) (Value, Version, error)
}

// Writer is a Driver that can also store and remove a secret (16 5.2: optional Put for writable
// drivers). The file driver implements it for the signing keys (ADR 0066, M1-32); the vault drivers
// follow with V3-12. A Writer never limits where it writes: Provider.Put and Provider.Delete allow
// only references directly below secrets.signing_key_dir and refuse every other one with
// ErrReadOnly before the driver is called.
type Writer interface {
	Driver
	// Put stores v under ref and returns the Version of the stored value. It never overwrites:
	// an existing secret under ref is ErrExists. Other errors: ErrWriteFailed, the context error.
	Put(ctx context.Context, ref Ref, v Value) (Version, error)
	// Delete removes the secret ref names. A missing secret is ErrNotFound; other errors:
	// ErrWriteFailed, the context error.
	Delete(ctx context.Context, ref Ref) error
}

// checkDriverRef is the common start of every Get: the context first, so a canceled caller
// causes no I/O, then the scheme and grammar of ref.
func checkDriverRef(ctx context.Context, scheme string, ref Ref) error {
	if err := ctx.Err(); err != nil {
		return &RefError{Ref: displayRef(ref), Err: err}
	}
	if ref.Scheme != scheme {
		return &RefError{Err: ErrUnsupportedScheme, Detail: "the " + scheme + " driver reads only " + scheme + " references"}
	}
	return checkRef(ref)
}
