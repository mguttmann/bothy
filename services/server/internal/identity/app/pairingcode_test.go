// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"bytes"
	"context"
	"crypto/hmac"
	"encoding/json"
	"fmt"
	"regexp"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/identity/domain/apitoken"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
)

// pairingCodePattern is the pattern of HostPairingCode.pairing_code in openapi.yaml.
var pairingCodePattern = regexp.MustCompile(`^bothy_hpc_[0-9A-Za-z]{43}_[0-9A-Za-z]{6}$`)

// TestHostPairingCode_Mint (M2-39, T-TM-06): the pairing code of a host has the kind hpc and the
// format of ADR 0061 with a valid checksum, its hash is the HMAC-SHA-256 with the token pepper
// (like api_tokens.token_hash), two codes differ, and the secret renders as [redacted] through
// fmt and JSON. NewHostPairingCode reads no database.
func TestHostPairingCode_Mint(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	s := newTokens(t, f, clock.NewFake(time.Now()))
	code, err := s.NewHostPairingCode(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	secret := code.Secret.Reveal()
	if !pairingCodePattern.MatchString(secret) || len(secret) != apitoken.Length {
		t.Fatalf("pairing code of %d characters does not match the pattern", len(secret))
	}
	p, err := apitoken.Parse(secret)
	if err != nil || p.Kind != apitoken.KindHostPairingCode {
		t.Fatalf("parse: kind %q, %v", p.Kind, err)
	}
	if !hmac.Equal(code.Hash, apitoken.Hash(tokenPepper, secret)) {
		t.Fatal("hash is not the HMAC of the code with the pepper")
	}
	if printed := fmt.Sprintf("%v %+v %s", code.Secret, code, code.Secret); bytes.Contains([]byte(printed), []byte(secret)) {
		t.Fatal("the code leaks through fmt")
	}
	raw, err := json.Marshal(code.Secret)
	if err != nil || string(raw) != `"[redacted]"` {
		t.Fatalf("json %s, %v", raw, err)
	}
	other, err := s.NewHostPairingCode(context.Background())
	if err != nil || other.Secret.Reveal() == secret || bytes.Equal(other.Hash, code.Hash) {
		t.Fatalf("a second code equals the first: %v", err)
	}
}

// TestAuthenticator_RejectsPairingCode (M2-39): a bearer value of kind hpc is 401 unauthenticated
// without a database call, under Bearer and DPoP alike (08 section 12: the authenticator never
// accepts a pairing code).
func TestAuthenticator_RejectsPairingCode(t *testing.T) {
	t.Parallel()
	store := &countingTokens{}
	a, _ := tokenAuth(t, store)
	hpc, err := apitoken.Generate(nil, apitoken.KindHostPairingCode)
	if err != nil {
		t.Fatal(err)
	}
	for _, scheme := range []string{"Bearer ", "DPoP "} {
		_, err := a.Authenticate(context.Background(), actor.Credentials{Authorization: scheme + hpc, Method: "GET", HTU: testPublicURL + "/api/v1/me"})
		if e, ok := errs.As(err); !ok || e.Code != errs.CodeUnauthenticated {
			t.Fatalf("%spairing code: %v, want 401 unauthenticated", scheme, err)
		}
	}
	if store.lookups != 0 {
		t.Fatalf("%d lookups for a pairing code, want 0", store.lookups)
	}
}
