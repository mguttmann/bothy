// SPDX-License-Identifier: Apache-2.0

package passwordhash_test

import (
	"context"
	"encoding/base64"
	"errors"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/identity/adapters/passwordhash"
	"github.com/OWNER/bothy/services/server/internal/identity/app"
)

// floor are the OWASP floor parameters the tests use to stay fast (brief M1-39 "Tests").
var floor = passwordhash.Params{MemoryKiB: 19456, Iterations: 2, Parallelism: 1}

// Compile-time check: the hasher is the outbound port.
var _ app.PasswordHasher = (*passwordhash.Hasher)(nil)

// ctx is the context of the calls that never wait long.
var ctx = context.Background()

// phcForm is the PHC string of the brief: standard Base64 without padding, 16 bytes of salt, 32 of
// hash.
var phcForm = regexp.MustCompile(`^\$argon2id\$v=19\$m=19456,t=2,p=1\$[A-Za-z0-9+/]{22}\$[A-Za-z0-9+/]{43}$`)

func TestPHCFormatAndRehash(t *testing.T) {
	t.Parallel()
	h, err := passwordhash.New(floor, 4)
	if err != nil {
		t.Fatal(err)
	}
	phc, err := h.Hash(ctx, "EXAMPLE-password")
	if err != nil {
		t.Fatal(err)
	}
	if !phcForm.MatchString(phc) {
		t.Fatalf("phc %s", phc)
	}
	parts := strings.Split(phc, "$")
	if salt, err := base64.RawStdEncoding.DecodeString(parts[4]); err != nil || len(salt) != 16 {
		t.Fatalf("salt %d bytes, err %v", len(salt), err)
	}
	if again, _ := h.Hash(ctx, "EXAMPLE-password"); again == phc {
		t.Fatal("two hashes share a salt")
	}
	ok, rehash, err := h.Verify(ctx, phc, "EXAMPLE-password")
	if !ok || rehash || err != nil {
		t.Fatalf("verify: ok %v rehash %v err %v", ok, rehash, err)
	}
	if ok, _, err := h.Verify(ctx, phc, "EXAMPLE-wrong-password"); ok || err != nil {
		t.Fatalf("wrong password: ok %v err %v", ok, err)
	}
	// Other configured parameters: the stored hash still verifies and asks for a rehash.
	other, err := passwordhash.New(passwordhash.Params{MemoryKiB: 19456, Iterations: 3, Parallelism: 1}, 4)
	if err != nil {
		t.Fatal(err)
	}
	ok, rehash, err = other.Verify(ctx, phc, "EXAMPLE-password")
	if !ok || !rehash || err != nil {
		t.Fatalf("parameter mismatch: ok %v rehash %v err %v", ok, rehash, err)
	}
	// Malformed strings are errors, never a match.
	for _, bad := range []string{"", "$argon2i$v=19$m=19456,t=2,p=1$c2FsdA$aGFzaA", "$argon2id$v=18$m=19456,t=2,p=1$" + parts[4] + "$" + parts[5],
		"$argon2id$v=19$m=x,t=2,p=1$" + parts[4] + "$" + parts[5], "$argon2id$v=19$m=19456,t=2,p=1$!!$" + parts[5],
		"$argon2id$v=19$m=19456,t=0,p=1$" + parts[4] + "$" + parts[5]} {
		if ok, _, err := h.Verify(ctx, bad, "EXAMPLE-password"); ok || err == nil {
			t.Errorf("%q: ok %v err %v", bad, ok, err)
		}
	}
	if err := h.VerifyDummy(ctx, "EXAMPLE-password"); err != nil {
		t.Fatal(err)
	}
}

func TestHasherDefaultParameters(t *testing.T) {
	t.Parallel()
	// The standard values of 08 section 18: 65536 KiB, 3 iterations, parallelism 1.
	h, err := passwordhash.New(passwordhash.Params{MemoryKiB: 65536, Iterations: 3, Parallelism: 1}, 4)
	if err != nil {
		t.Fatal(err)
	}
	phc, err := h.Hash(ctx, "EXAMPLE-default")
	if err != nil || !strings.HasPrefix(phc, "$argon2id$v=19$m=65536,t=3,p=1$") {
		t.Fatalf("phc %s err %v", phc, err)
	}
	if ok, rehash, err := h.Verify(ctx, phc, "EXAMPLE-default"); !ok || rehash || err != nil {
		t.Fatalf("verify: %v %v %v", ok, rehash, err)
	}
}

func TestHasherRefusesBelowFloor(t *testing.T) {
	t.Parallel()
	for _, p := range []passwordhash.Params{
		{MemoryKiB: 19455, Iterations: 2, Parallelism: 1},
		{MemoryKiB: 19456, Iterations: 1, Parallelism: 1},
		{MemoryKiB: 19456, Iterations: 2, Parallelism: 0},
		{MemoryKiB: 19456, Iterations: 2, Parallelism: 256},
	} {
		if _, err := passwordhash.New(p, 4); err == nil {
			t.Errorf("%+v accepted", p)
		}
	}
	if _, err := passwordhash.New(floor, 0); err == nil || !strings.Contains(err.Error(), "auth.argon2id_max_concurrent") {
		t.Errorf("no computation at a time accepted: %v", err)
	}
}

// TestConcurrentComputationsBounded (auth.argon2id_max_concurrent): however many logins arrive at
// once, for known names (Verify), unknown ones (VerifyDummy) and password changes (Hash), at most
// the configured number of Argon2id computations run at the same time, each with
// auth.argon2id_memory_kib of memory; the others wait for a free one and all of them finish.
func TestConcurrentComputationsBounded(t *testing.T) {
	t.Parallel()
	const limit, callers = 2, 12
	h, err := passwordhash.New(floor, limit)
	if err != nil {
		t.Fatal(err)
	}
	phc, err := h.Hash(ctx, "EXAMPLE-password")
	if err != nil {
		t.Fatal(err)
	}
	peak := passwordhash.WatchComputations(h, 20*time.Millisecond)
	var wg sync.WaitGroup
	failed := make(chan error, callers)
	for i := range callers {
		wg.Go(func() {
			var err error
			switch i % 3 {
			case 0:
				var ok bool
				if ok, _, err = h.Verify(ctx, phc, "EXAMPLE-password"); err == nil && !ok {
					err = errors.New("the right password did not verify")
				}
			case 1:
				err = h.VerifyDummy(ctx, "EXAMPLE-unknown")
			default:
				_, err = h.Hash(ctx, "EXAMPLE-new-password")
			}
			failed <- err
		})
	}
	wg.Wait()
	close(failed)
	for err := range failed {
		if err != nil {
			t.Fatal(err)
		}
	}
	if got := peak(); got != limit {
		t.Fatalf("%d computations ran at the same time, want the limit %d", got, limit)
	}
}

// TestWaitForAComputationEndsWithTheContext: a caller that waits for a free computation leaves
// when its context ends and computes nothing, the way a request leaves at its deadline; the
// computation that holds the slot finishes, and the slot is free again afterwards.
func TestWaitForAComputationEndsWithTheContext(t *testing.T) {
	t.Parallel()
	h, err := passwordhash.New(floor, 1)
	if err != nil {
		t.Fatal(err)
	}
	started, release := make(chan struct{}), make(chan struct{})
	count := passwordhash.BlockComputations(h, started, release)
	first := make(chan error, 1)
	go func() { first <- h.VerifyDummy(ctx, "EXAMPLE-first") }()
	<-started
	waiting, cancel := context.WithCancel(ctx)
	second := make(chan error, 1)
	go func() { second <- h.VerifyDummy(waiting, "EXAMPLE-second") }()
	select {
	case err := <-second:
		t.Fatalf("the second computation did not wait: %v", err)
	case <-time.After(50 * time.Millisecond):
	}
	cancel()
	if err := <-second; !errors.Is(err, context.Canceled) {
		t.Fatalf("the waiting caller got %v, want its context error", err)
	}
	if _, err := h.Hash(waiting, "EXAMPLE-third"); !errors.Is(err, context.Canceled) {
		t.Fatalf("a caller with an ended context got %v, want its context error", err)
	}
	close(release)
	if err := <-first; err != nil {
		t.Fatal(err)
	}
	if got := count(); got != 1 {
		t.Fatalf("%d computations started, want only the first", got)
	}
	// The slot is free again.
	go func() { <-started }()
	if err := h.VerifyDummy(ctx, "EXAMPLE-fourth"); err != nil {
		t.Fatal(err)
	}
}

// TestDummyUsesConfiguredParameters: VerifyDummy verifies against a hash with the configured
// parameters, so an unknown login name costs what a known one costs (ID-T21).
func TestDummyUsesConfiguredParameters(t *testing.T) {
	t.Parallel()
	for _, p := range []passwordhash.Params{floor, {MemoryKiB: 20000, Iterations: 3, Parallelism: 2}} {
		h, err := passwordhash.New(p, 4)
		if err != nil {
			t.Fatal(err)
		}
		if got := passwordhash.DummyParams(h); got != p {
			t.Fatalf("dummy parameters %+v, want %+v", got, p)
		}
	}
}

// TestStaleParametersCostLikeUnknownNames (ID-T21 during a parameter change): every
// verification lasts at least the average cost of the most expensive parameter set seen, so a row
// hashed with other parameters than the configured ones costs the same time for a wrong password
// as an unknown name, in both directions (08 section 2.5: the rehash happens only at the next
// login). The padding is checked with recorded costs and a captured sleep, not with wall time.
func TestStaleParametersCostLikeUnknownNames(t *testing.T) {
	t.Parallel()
	cheap := passwordhash.Params{MemoryKiB: 19456, Iterations: 2, Parallelism: 1}
	dear := passwordhash.Params{MemoryKiB: 19456, Iterations: 4, Parallelism: 1}
	h, err := passwordhash.New(cheap, 4)
	if err != nil {
		t.Fatal(err)
	}
	ms := time.Millisecond
	passwordhash.SetCost(h, cheap, 10*ms)
	passwordhash.SetCost(h, dear, 40*ms)
	// A cheap verification (configured or stale) is padded up to the dear average.
	if got := passwordhash.Pad(h, cheap, 12*ms); got != 40*ms-12*ms {
		t.Fatalf("cheap verification slept %v, want %v", got, 28*ms)
	}
	// A dear verification above every average is not padded; its average moves by a quarter.
	if got := passwordhash.Pad(h, dear, 60*ms); got != 0 {
		t.Fatalf("dear verification slept %v", got)
	}
	if got := passwordhash.Pad(h, cheap, 10*ms); got != 45*ms-10*ms {
		t.Fatalf("after the dear sample: slept %v, want %v", got, 35*ms)
	}
	// Verify pads a stale row and VerifyDummy pads an unknown name to the same target.
	old, err := passwordhash.New(dear, 4)
	if err != nil {
		t.Fatal(err)
	}
	phc, err := old.Hash(ctx, "EXAMPLE-stored")
	if err != nil {
		t.Fatal(err)
	}
	passwordhash.SetCost(h, dear, time.Hour)
	var slept time.Duration
	passwordhash.CaptureSleep(h, &slept)
	if ok, rehash, err := h.Verify(ctx, phc, "EXAMPLE-wrong"); ok || !rehash || err != nil {
		t.Fatalf("stale row: ok %v rehash %v err %v", ok, rehash, err)
	}
	if slept < 40*time.Minute {
		t.Fatalf("a stale row slept %v, want the dear average", slept)
	}
	slept = 0
	if err := h.VerifyDummy(ctx, "EXAMPLE-wrong"); err != nil {
		t.Fatal(err)
	}
	if slept < 40*time.Minute {
		t.Fatalf("an unknown name slept %v, want the dear average", slept)
	}
}
