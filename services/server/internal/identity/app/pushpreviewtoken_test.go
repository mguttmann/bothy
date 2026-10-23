// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"context"
	"crypto/hmac"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/identity"
	"github.com/OWNER/bothy/services/server/internal/identity/domain/apitoken"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
)

// TestPushPreviewToken_MintAndHash (M3-22 design note 3): the preview token has the kind pvw and
// the format of ADR 0061, HashPushPreviewToken of it equals its Hash (the HMAC-SHA-256 with the
// pepper over the full string), and the secret prints [redacted].
func TestPushPreviewToken_MintAndHash(t *testing.T) {
	t.Parallel()
	s := dbFreeTokens(t, &countingTokens{})
	tok, err := s.NewPushPreviewToken(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	secret := tok.Secret.Reveal()
	p, err := apitoken.Parse(secret)
	if err != nil || p.Kind != apitoken.KindPushPreview || !strings.HasPrefix(secret, "bothy_pvw_") {
		t.Fatalf("parse: kind %q, %v", p.Kind, err)
	}
	if fmt.Sprintf("%v %+v", tok.Secret, tok) != "[redacted] {Secret:[redacted] Hash:"+fmt.Sprintf("%v", tok.Hash)+"}" {
		t.Fatal("the token prints")
	}
	h, err := s.HashPushPreviewToken(context.Background(), secret)
	if err != nil || !hmac.Equal(h, tok.Hash) || !hmac.Equal(h, apitoken.Hash(tokenPepper, secret)) {
		t.Fatalf("hash of the token: %v", err)
	}
	other, err := s.NewPushPreviewToken(context.Background())
	if err != nil || other.Secret.Reveal() == secret {
		t.Fatalf("a second token equals the first: %v", err)
	}
}

// TestHashPushPreviewToken_MalformedWithoutDatabase (M3-22, Tests): a wrong prefix, the kind pat,
// a bad checksum and a wrong length are ErrTokenMalformed, and the counting store sees no call.
func TestHashPushPreviewToken_MalformedWithoutDatabase(t *testing.T) {
	t.Parallel()
	store := &countingTokens{}
	s := dbFreeTokens(t, store)
	tok, err := s.NewPushPreviewToken(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	good := tok.Secret.Reveal()
	last := good[len(good)-1]
	other := byte('A')
	if last == other {
		other = 'B'
	}
	pat, err := apitoken.Generate(nil, apitoken.KindPersonal)
	if err != nil {
		t.Fatal(err)
	}
	for name, v := range map[string]string{
		"wrong prefix": "bothx" + good[5:], "kind pat": pat, "bad checksum": good[:len(good)-1] + string(other),
		"wrong length": good[:len(good)-1], "empty": "",
	} {
		if _, err := s.HashPushPreviewToken(context.Background(), v); !errors.Is(err, identity.ErrTokenMalformed) {
			t.Errorf("%s: %v, want ErrTokenMalformed", name, err)
		}
	}
	if store.lookups != 0 {
		t.Fatalf("%d lookups", store.lookups)
	}
}

// TestAuthenticate_PreviewTokenRejected (M3-22 design note 3): a preview token under Bearer or
// DPoP on a normal operation is 401 unauthenticated without a database call.
func TestAuthenticate_PreviewTokenRejected(t *testing.T) {
	t.Parallel()
	store := &countingTokens{}
	a, _ := tokenAuth(t, store)
	pvw, err := apitoken.Generate(nil, apitoken.KindPushPreview)
	if err != nil {
		t.Fatal(err)
	}
	for _, scheme := range []string{"Bearer ", "DPoP "} {
		_, err := a.Authenticate(context.Background(), actor.Credentials{Authorization: scheme + pvw, Method: "GET", HTU: testPublicURL + "/api/v1/me/devices"})
		if e, ok := errs.As(err); !ok || e.Code != errs.CodeUnauthenticated {
			t.Fatalf("%spreview token: %v, want 401 unauthenticated", scheme, err)
		}
	}
	if store.lookups != 0 {
		t.Fatalf("%d lookups for a preview token, want 0", store.lookups)
	}
}

// TestPushPreviewToken_MatchesGitleaksRule (SEC-SEC-4): generated preview tokens match the regex
// of the gitleaks rule bothy-api-token, which carries the keyword bothy_pvw_.
func TestPushPreviewToken_MatchesGitleaksRule(t *testing.T) {
	t.Parallel()
	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "..", ".gitleaks.toml"))
	if err != nil {
		t.Fatal(err)
	}
	re, keywords := gitleaksRule(t, string(raw), "bothy-api-token")
	if !slices.Contains(keywords, "bothy_pvw_") {
		t.Fatalf("keywords %v without bothy_pvw_", keywords)
	}
	s := dbFreeTokens(t, &countingTokens{})
	for range 20 {
		tok, err := s.NewPushPreviewToken(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		if !re.MatchString("PREVIEW=" + tok.Secret.Reveal()) {
			t.Fatal("a preview token does not match the rule")
		}
	}
}
