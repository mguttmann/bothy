// SPDX-License-Identifier: Apache-2.0

package domain_test

import (
	"bytes"
	"crypto/rand"
	"encoding/json"
	"strings"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/identity/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
)

func TestPairingCodeFormatAndNormalize(t *testing.T) {
	t.Parallel()
	code, err := domain.NewPairingCode(rand.Reader, 8)
	if err != nil {
		t.Fatal(err)
	}
	if len(code) != 8 {
		t.Fatalf("canonical code %q has %d characters, want 8", code, len(code))
	}
	for _, c := range code {
		if !strings.ContainsRune(crockford, c) {
			t.Fatalf("character %q outside Crockford Base32", c)
		}
	}
	shown := domain.DisplayPairingCode(code)
	if shown != code[:4]+"-"+code[4:] {
		t.Fatalf("display %q, want two groups of four", shown)
	}
	// An odd length splits with the longer group first.
	if got := domain.DisplayPairingCode("ABCDEFGHJ"); got != "ABCDE-FGHJ" {
		t.Fatalf("display of nine characters %q", got)
	}
	for _, in := range []string{shown, code, strings.ToLower(shown), " " + code[:4] + " " + code[4:] + " "} {
		got, ok := domain.NormalizePairingCode(in, 8)
		if !ok || got != code {
			t.Fatalf("NormalizePairingCode(%q) = %q, %v; want %q", in, got, ok, code)
		}
	}
	// Crockford: O is 0, I and L are 1.
	if got, ok := domain.NormalizePairingCode("oil0-1abc", 8); !ok || got != "01101ABC" {
		t.Fatalf("Crockford mapping %q, %v", got, ok)
	}
	for _, bad := range []string{"", "ABC", "ABCD-EFGHJ", "ABCD-EFG!", "ABCU-EFGH", "en_ABCD-EFGH"} {
		if _, ok := domain.NormalizePairingCode(bad, 8); ok {
			t.Errorf("NormalizePairingCode(%q) accepted", bad)
		}
	}
	// The source is read for every character; a short source is an error, never a short code.
	if _, err := domain.NewPairingCode(bytes.NewReader([]byte{1, 2, 3}), 8); err == nil {
		t.Fatal("a short random source gave a code")
	}
	// Every byte maps to one character without bias (256 is a multiple of 32).
	got, err := domain.NewPairingCode(bytes.NewReader([]byte{0, 31, 32, 255, 64, 1, 2, 3}), 8)
	if err != nil || got != "0Z0Z0123" {
		t.Fatalf("code from fixed bytes %q, %v", got, err)
	}
}

func TestPairingCodeHash(t *testing.T) {
	t.Parallel()
	a := domain.PairingCodeHash(vectorPepper, "7KQ2M9XD")
	if len(a) != 32 || bytes.Equal(a, domain.PairingCodeHash(vectorPepper, "7KQ2M9XE")) ||
		bytes.Equal(a, domain.EnrollmentCodeHash(vectorPepper, "7KQ2M9XD")) {
		t.Fatal("pairing hash not a 32-byte HMAC separated from other codes")
	}
}

func TestPairingStateMachine(t *testing.T) {
	t.Parallel()
	all := []domain.PairingStatus{
		domain.PairingRequested, domain.PairingApproved, domain.PairingCompleted, domain.PairingRejected, domain.PairingExpired,
	}
	allowed := map[[2]domain.PairingStatus]bool{
		{domain.PairingRequested, domain.PairingApproved}:  true,
		{domain.PairingApproved, domain.PairingCompleted}:  true,
		{domain.PairingRequested, domain.PairingRejected}:  true,
		{domain.PairingRequested, domain.PairingExpired}:   true,
		{domain.PairingApproved, domain.PairingExpired}:    true,
		{domain.PairingApproved, domain.PairingRejected}:   false,
		{domain.PairingCompleted, domain.PairingExpired}:   false,
		{domain.PairingRejected, domain.PairingRequested}:  false,
		{domain.PairingExpired, domain.PairingApproved}:    false,
		{domain.PairingRequested, domain.PairingCompleted}: false,
	}
	for _, from := range all {
		for _, to := range all {
			err := from.To(to)
			if allowed[[2]domain.PairingStatus{from, to}] {
				if err != nil {
					t.Errorf("%s -> %s refused: %v", from, to, err)
				}
				continue
			}
			if e, ok := errs.As(err); !ok || e.Code != errs.CodeInvalidStateTransition {
				t.Errorf("%s -> %s = %v, want invalid_state_transition", from, to, err)
			}
		}
	}
	if !domain.PairingRequested.Open() || !domain.PairingApproved.Open() || domain.PairingCompleted.Open() {
		t.Fatal("Open is true exactly for requested and approved")
	}
}

func TestDeviceRegistrationV1JSON(t *testing.T) {
	t.Parallel()
	reg := domain.DeviceRegistrationV1{
		V: 1, DeviceID: "0192f4a0-7b1c-7848-8501-1716705c7dbe", Name: "Alex Laptop", Platform: "macos",
		DPoP: domain.RegisteredKey{JWK: json.RawMessage(`{"kty":"EC"}`), Alg: "ES256"},
	}
	b, err := json.Marshal(reg)
	if err != nil {
		t.Fatal(err)
	}
	want := `{"v":1,"device_id":"0192f4a0-7b1c-7848-8501-1716705c7dbe","name":"Alex Laptop","platform":"macos","dpop":{"jwk":{"kty":"EC"},"alg":"ES256"}}`
	if string(b) != want {
		t.Fatalf("json %s, want %s", b, want)
	}
	var back domain.DeviceRegistrationV1
	if err := json.Unmarshal(b, &back); err != nil || back.V != 1 || back.StepUp != nil {
		t.Fatalf("round trip %+v, %v", back, err)
	}
}
