// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"context"
	"crypto/hmac"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/identity"
	"github.com/OWNER/bothy/services/server/internal/identity/domain/apitoken"
)

// TestIncomingWebhookToken_MintAndHash (M2-27, ADR 0061, 08 section 12): the token of an incoming
// webhook has the kind whi, the length and a valid checksum of the format, its hash is the
// HMAC-SHA-256 with the token pepper (as api_tokens.token_hash), HashIncomingWebhookToken of the
// secret gives the same hash, two tokens differ and the secret prints [redacted]. Neither call
// reads a store or runs a unit of work.
func TestIncomingWebhookToken_MintAndHash(t *testing.T) {
	t.Parallel()
	store := &countingTokens{}
	s := dbFreeTokens(t, store)
	tok, err := s.NewIncomingWebhookToken(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	secret := tok.Secret.Reveal()
	p, err := apitoken.Parse(secret)
	if err != nil || p.Kind != apitoken.KindIncomingWebhook || len(secret) != apitoken.Length || !strings.HasPrefix(secret, "bothy_whi_") {
		t.Fatalf("the token has kind %q and length %d: %v", p.Kind, len(secret), err)
	}
	if !hmac.Equal(tok.Hash, apitoken.Hash(tokenPepper, secret)) {
		t.Fatal("the hash is not the HMAC of the token with the pepper")
	}
	h, err := s.HashIncomingWebhookToken(context.Background(), secret)
	if err != nil || !hmac.Equal(h, tok.Hash) {
		t.Fatalf("HashIncomingWebhookToken of the minted token: %v", err)
	}
	if printed := fmt.Sprintf("%v %+v %s", tok.Secret, tok, tok.Secret); strings.Contains(printed, secret) {
		t.Fatal("the token leaks through fmt")
	}
	if raw, err := json.Marshal(tok.Secret); err != nil || string(raw) != `"[redacted]"` {
		t.Fatalf("json %s, %v", raw, err)
	}
	other, err := s.NewIncomingWebhookToken(context.Background())
	if err != nil || other.Secret.Reveal() == secret || hmac.Equal(other.Hash, tok.Hash) {
		t.Fatalf("a second token equals the first: %v", err)
	}
	if store.lookups != 0 {
		t.Fatalf("%d lookups", store.lookups)
	}
}

// TestHashIncomingWebhookToken_RejectsWithoutLookup (M2-27 design note 2 b): a changed checksum,
// the kinds agt, app, pat and hpc, a wrong length and the empty value are ErrTokenMalformed
// without any store call.
func TestHashIncomingWebhookToken_RejectsWithoutLookup(t *testing.T) {
	t.Parallel()
	store := &countingTokens{}
	s := dbFreeTokens(t, store)
	tok, err := s.NewIncomingWebhookToken(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	good := tok.Secret.Reveal()
	other := byte('A')
	if good[len(good)-1] == other {
		other = 'B'
	}
	bad := []string{good[:len(good)-1] + string(other), good[:len(good)-1], good + "A", "", "bothy_whi_"}
	for _, k := range []apitoken.Kind{apitoken.KindAgent, apitoken.KindApp, apitoken.KindPersonal, apitoken.KindHostPairingCode} {
		v, err := apitoken.Generate(nil, k)
		if err != nil {
			t.Fatal(err)
		}
		bad = append(bad, v)
	}
	for i, v := range bad {
		if _, err := s.HashIncomingWebhookToken(context.Background(), v); !errors.Is(err, identity.ErrTokenMalformed) {
			t.Errorf("value %d: %v, want ErrTokenMalformed", i, err)
		}
	}
	if store.lookups != 0 {
		t.Fatalf("%d lookups", store.lookups)
	}
}
