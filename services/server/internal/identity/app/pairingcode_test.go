// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/identity"
	"github.com/OWNER/bothy/services/server/internal/identity/app"
	"github.com/OWNER/bothy/services/server/internal/identity/domain/apitoken"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
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

// panicStore is a principal store whose every call panics: the pairing code functions must not
// read principals.
type panicStore struct{ app.Store }

// dbFreeTokens returns the token use cases over stores that count or panic on every call.
func dbFreeTokens(t *testing.T, store *countingTokens) *app.Tokens {
	t.Helper()
	cfg := app.Config{APITokenMaxTTL: time.Hour, PersonalTokenMaxTTL: time.Hour, PageSizeDefault: 1, PageSizeMax: 1}
	s, err := app.NewTokens(cfg, tokenPepper, app.TokensDeps{
		Store: store, Principals: panicStore{}, UoW: panicUoW{}, Authorizer: denyAll{}, Audit: noAudit{}, Cursors: nopCursors{},
		Switch: personalOff{},
	})
	if err != nil {
		t.Fatal(err)
	}
	return s
}

// panicUoW is a unit of work that must never run.
type panicUoW struct{}

func (panicUoW) Do(context.Context, func(context.Context) error, ...txn.Option) error {
	panic("unit of work used")
}

// TestHostPairingCode_MintAndHash (M2-41, Design note 1): HashHostPairingCode of a minted code
// equals its Hash, the HMAC-SHA-256 with the pepper over the full string.
func TestHostPairingCode_MintAndHash(t *testing.T) {
	t.Parallel()
	s := dbFreeTokens(t, &countingTokens{})
	code, err := s.NewHostPairingCode(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(code.Secret.Reveal()) != 60 || code.Secret.String() != "[redacted]" || fmt.Sprintf("%v", code.Secret) != "[redacted]" {
		t.Fatal("the code has the wrong length or prints")
	}
	h, err := s.HashHostPairingCode(context.Background(), code.Secret.Reveal())
	if err != nil || !hmac.Equal(h, code.Hash) || !hmac.Equal(h, apitoken.Hash(tokenPepper, code.Secret.Reveal())) {
		t.Fatalf("hash of the code: %v", err)
	}
}

// TestHashHostPairingCode_RejectsWithoutLookup (TM-T20): one changed checksum character, every
// other kind and a short random part are ErrTokenMalformed without any store call.
func TestHashHostPairingCode_RejectsWithoutLookup(t *testing.T) {
	t.Parallel()
	store := &countingTokens{}
	s := dbFreeTokens(t, store)
	code, err := s.NewHostPairingCode(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	good := code.Secret.Reveal()
	last := good[len(good)-1]
	other := byte('A')
	if last == other {
		other = 'B'
	}
	bad := []string{good[:len(good)-1] + string(other), good[:apitoken.Length-apitoken.ChecksumLen-2] + "_" + good[len(good)-apitoken.ChecksumLen:], "", "bothy_hpc_"}
	for _, k := range []apitoken.Kind{apitoken.KindAgent, apitoken.KindApp, apitoken.KindPersonal, apitoken.KindIncomingWebhook} {
		v, err := apitoken.Generate(nil, k)
		if err != nil {
			t.Fatal(err)
		}
		bad = append(bad, v)
	}
	for i, v := range bad {
		if _, err := s.HashHostPairingCode(context.Background(), v); !errors.Is(err, identity.ErrTokenMalformed) {
			t.Errorf("value %d: %v, want ErrTokenMalformed", i, err)
		}
	}
	if store.lookups != 0 {
		t.Fatalf("%d lookups", store.lookups)
	}
}

// TestHostPairingCode_MatchesGitleaksRule (T-TM-06, SEC-SEC-4): 100 generated codes match the
// regex of the gitleaks rule bothy-api-token and carry its keyword bothy_hpc_ (gitleaks uses Go's
// regexp, so the engine is the same).
func TestHostPairingCode_MatchesGitleaksRule(t *testing.T) {
	t.Parallel()
	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "..", ".gitleaks.toml"))
	if err != nil {
		t.Fatal(err)
	}
	re, keywords := gitleaksRule(t, string(raw), "bothy-api-token")
	if !slices.Contains(keywords, "bothy_hpc_") {
		t.Fatalf("keywords %v without bothy_hpc_", keywords)
	}
	s := dbFreeTokens(t, &countingTokens{})
	for range 100 {
		code, err := s.NewHostPairingCode(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		v := code.Secret.Reveal()
		if !re.MatchString("BOTHY_CODE="+v) || !strings.Contains(v, "bothy_hpc_") {
			sum := sha256.Sum256([]byte(v))
			t.Fatalf("code with hash prefix %x does not match the rule", sum[:4])
		}
	}
}

// gitleaksRule extracts regex and keywords of rule id from the TOML text.
func gitleaksRule(t *testing.T, toml, id string) (*regexp.Regexp, []string) {
	t.Helper()
	_, block, ok := strings.Cut(toml, `id = "`+id+`"`)
	if !ok {
		t.Fatalf("rule %s not found", id)
	}
	block, _, _ = strings.Cut(block, "[[")
	m := regexp.MustCompile(`(?m)^regex = '''(.*)'''$`).FindStringSubmatch(block)
	k := regexp.MustCompile(`(?m)^keywords = \[(.*)\]$`).FindStringSubmatch(block)
	if m == nil || k == nil {
		t.Fatalf("rule %s without regex or keywords", id)
	}
	var keywords []string
	for _, w := range strings.Split(k[1], ",") {
		keywords = append(keywords, strings.Trim(strings.TrimSpace(w), `"`))
	}
	return regexp.MustCompile(m[1]), keywords
}
