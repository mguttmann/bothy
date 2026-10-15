// SPDX-License-Identifier: Apache-2.0

// Package passwordhash is the Argon2id password hasher of local accounts (identity/app
// PasswordHasher, docs/spec/08-identity.md section 2.5, brief M1-39): golang.org/x/crypto/argon2
// with the parameters auth.argon2id_*, a 16-byte salt from crypto/rand and a 32-byte key, stored as
// a PHC string $argon2id$v=19$m=<kib>,t=<iterations>,p=<parallelism>$<salt>$<hash> in standard
// Base64 without padding. Verification compares in constant time; VerifyDummy spends the same work
// against a hash computed once at construction, so an unknown login name takes as long as a known
// one (ID-T21). Every verification lasts at least the average cost of the most expensive parameter
// set seen, so rows that still carry the parameters of before a change of auth.argon2id_* cost the
// same as unknown names until their next login rehashes them.
//
// Example:
//
//	h, err := passwordhash.New(passwordhash.Params{MemoryKiB: 65536, Iterations: 3, Parallelism: 1})
//	phc, err := h.Hash(nfcPassword)
//	ok, rehash, err := h.Verify(phc, nfcPassword)
package passwordhash

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/argon2"
)

// The OWASP floor of Argon2id (Password Storage Cheat Sheet, 08 section 2.5): values below stop
// the start (ID-T24).
const (
	// FloorMemoryKiB is the least memory, 19 MiB.
	FloorMemoryKiB = 19456
	// FloorIterations is the least number of passes.
	FloorIterations = 2
	// FloorParallelism is the least parallelism.
	FloorParallelism = 1
)

const (
	// saltBytes is the salt length.
	saltBytes = 16
	// keyBytes is the length of the derived key.
	keyBytes = 32
	// maxParallelism is the largest parallelism argon2 takes (a uint8).
	maxParallelism = 255
	// phcPrefix starts every string this package writes and reads.
	phcPrefix = "$argon2id$v=19$"
)

// b64 is standard Base64 without padding, the encoding of the PHC string format.
var b64 = base64.RawStdEncoding

// Params are auth.argon2id_memory_kib, auth.argon2id_iterations and auth.argon2id_parallelism.
type Params struct {
	MemoryKiB   int
	Iterations  int
	Parallelism int
}

// Hasher implements app.PasswordHasher. It is safe for concurrent use.
type Hasher struct {
	p     Params
	dummy string
	mu    sync.Mutex
	// cost is the running average duration of one verification per parameter set seen.
	cost map[Params]time.Duration
	// sleep waits out the padding; time.Sleep except in tests.
	sleep func(time.Duration)
}

// New returns the hasher with p and computes the hash VerifyDummy verifies against. Errors: a
// parameter below the OWASP floor (the error names the key) or a parallelism above 255.
func New(p Params) (*Hasher, error) {
	switch {
	case p.MemoryKiB < FloorMemoryKiB:
		return nil, fmt.Errorf("passwordhash: auth.argon2id_memory_kib must be at least %d", FloorMemoryKiB)
	case p.Iterations < FloorIterations:
		return nil, fmt.Errorf("passwordhash: auth.argon2id_iterations must be at least %d", FloorIterations)
	case p.Parallelism < FloorParallelism || p.Parallelism > maxParallelism:
		return nil, fmt.Errorf("passwordhash: auth.argon2id_parallelism must be between %d and %d", FloorParallelism, maxParallelism)
	}
	h := &Hasher{p: p, cost: map[Params]time.Duration{}, sleep: time.Sleep}
	dummy, err := h.Hash("dummy password for unknown login names")
	if err != nil {
		return nil, err
	}
	h.dummy = dummy
	// Learn the cost of the configured parameters before the first request.
	h.VerifyDummy("dummy password for unknown login names")
	return h, nil
}

// Hash implements app.PasswordHasher. Errors: a failing random source.
func (h *Hasher) Hash(password string) (string, error) {
	salt := make([]byte, saltBytes)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("passwordhash: random salt: %w", err)
	}
	key := derive(password, salt, h.p, keyBytes)
	return fmt.Sprintf("%sm=%d,t=%d,p=%d$%s$%s", phcPrefix, h.p.MemoryKiB, h.p.Iterations, h.p.Parallelism,
		b64.EncodeToString(salt), b64.EncodeToString(key)), nil
}

// Verify implements app.PasswordHasher: the key is derived with the stored parameters and salt
// and compared in constant time; needsRehash reports stored parameters other than the configured
// ones. Every verification lasts at least the average cost of the most expensive parameter set
// seen (pad): after a change of auth.argon2id_* the rows keep their old parameters until the next
// login rehashes them, and without the padding a wrong password on such a row would take another
// time than an unknown name, which verifies with the configured parameters (ID-T21). Errors: a
// string that is not an Argon2id v19 PHC string of this format; never a match.
func (h *Hasher) Verify(phc, password string) (bool, bool, error) {
	start := time.Now()
	p, salt, key, err := parse(phc)
	if err != nil {
		return false, false, err
	}
	got := derive(password, salt, p, len(key))
	h.pad(p, time.Since(start))
	return subtle.ConstantTimeCompare(got, key) == 1, p != h.p || len(key) != keyBytes, nil
}

// pad records took as one verification with p in the running average (weight 1/4) and sleeps
// until the verification has lasted the largest average of all parameter sets seen.
func (h *Hasher) pad(p Params, took time.Duration) {
	h.mu.Lock()
	if avg, ok := h.cost[p]; ok {
		h.cost[p] = avg + (took-avg)/4
	} else {
		h.cost[p] = took
	}
	var target time.Duration
	for _, c := range h.cost {
		target = max(target, c)
	}
	h.mu.Unlock()
	if rest := target - took; rest > 0 {
		h.sleep(rest)
	}
}

// VerifyDummy implements app.PasswordHasher: the work of Verify against the hash of New.
func (h *Hasher) VerifyDummy(password string) {
	_, _, _ = h.Verify(h.dummy, password)
}

// derive runs Argon2id; the parameters were checked by New or parse.
func derive(password string, salt []byte, p Params, n int) []byte {
	return argon2.IDKey([]byte(password), salt, uint32(p.Iterations), uint32(p.MemoryKiB), uint8(p.Parallelism), uint32(n)) //nolint:gosec // G115: New and parse bound every value.
}

// errFormat reports a stored string this package cannot read; it never repeats the string.
var errFormat = errors.New("passwordhash: not an Argon2id v19 PHC string")

// maxStoredMemoryKiB bounds the memory a stored string may ask for (4 GiB), so a corrupt row
// cannot exhaust the server.
const maxStoredMemoryKiB = 4 << 20

// parse reads a PHC string: parameters, salt and key.
func parse(phc string) (Params, []byte, []byte, error) {
	rest, ok := strings.CutPrefix(phc, phcPrefix)
	if !ok {
		return Params{}, nil, nil, errFormat
	}
	parts := strings.Split(rest, "$")
	if len(parts) != 3 {
		return Params{}, nil, nil, errFormat
	}
	var p Params
	fields := strings.Split(parts[0], ",")
	if len(fields) != 3 {
		return Params{}, nil, nil, errFormat
	}
	for i, dst := range []*int{&p.MemoryKiB, &p.Iterations, &p.Parallelism} {
		name, value, ok := strings.Cut(fields[i], "=")
		n, err := strconv.Atoi(value)
		if !ok || name != []string{"m", "t", "p"}[i] || err != nil {
			return Params{}, nil, nil, errFormat
		}
		*dst = n
	}
	if p.MemoryKiB < 8*p.Parallelism || p.MemoryKiB > maxStoredMemoryKiB || p.Iterations < 1 || p.Parallelism < 1 || p.Parallelism > maxParallelism {
		return Params{}, nil, nil, errFormat
	}
	salt, err := b64.DecodeString(parts[1])
	if err != nil || len(salt) < 8 {
		return Params{}, nil, nil, errFormat
	}
	key, err := b64.DecodeString(parts[2])
	if err != nil || len(key) < 16 || len(key) > 64 {
		return Params{}, nil, nil, errFormat
	}
	return p, salt, key, nil
}
