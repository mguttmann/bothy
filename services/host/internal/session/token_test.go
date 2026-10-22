// SPDX-License-Identifier: Apache-2.0

package session

import (
	"crypto/ed25519"
	"encoding/json"
	"errors"
	"maps"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/host/internal/session/sessiontest"
)

// profileJWSHeader is a header of the profile for the kid of the signers of the tests.
const profileJWSHeader = `{"alg":"EdDSA","kid":"terminal-key-1","typ":"terminal-session+jwt"}`

// signTokenBytes signs the header and claims bytes exactly as given, so a test can put JSON the
// encoder never writes (null, trailing data, a fraction) under a valid signature.
func signTokenBytes(s *sessiontest.Signer, header, claims string) string {
	input := b64url.EncodeToString([]byte(header)) + "." + b64url.EncodeToString([]byte(claims))
	return input + "." + b64url.EncodeToString(ed25519.Sign(s.Key, []byte(input)))
}

// tokenClaimsJSON returns valid claims with the members of set replaced and those of drop removed.
func tokenClaimsJSON(t *testing.T, set map[string]any, drop ...string) string {
	t.Helper()
	m := maps.Clone(sessiontest.Claims(hostID, sessID, alexSub, alexOSU, "shell", cek, AttOpen, now, time.Minute))
	maps.Copy(m, set)
	for _, k := range drop {
		delete(m, k)
	}
	b, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// TestParseTokenProfile (ADR 0067, Design note 4 of M2-45): every token outside the profile fails
// with ErrToken even under a valid signature; within it the claims come back as signed.
func TestParseTokenProfile(t *testing.T) {
	s := sessiontest.NewSigner("terminal-key-1")
	keys := []map[string]any{s.JWK()}
	valid := tokenClaimsJSON(t, map[string]any{"own": true, "wdi": 2})
	hdr, c, err := ParseToken(signTokenBytes(s, profileJWSHeader, valid), keys)
	if err != nil || hdr.Kid != "terminal-key-1" || hdr.Typ != TokenType || !c.Own || c.Wdi == nil || *c.Wdi != 2 || c.Osu != alexOSU || c.Att != AttOpen {
		t.Fatalf("valid token %+v %+v %v", hdr, c, err)
	}
	if _, c, err := ParseToken(signTokenBytes(s, profileJWSHeader, tokenClaimsJSON(t, nil, "wdi")), keys); err != nil || c.Wdi != nil {
		t.Errorf("without wdi %+v %v", c, err)
	}
	good := signTokenBytes(s, profileJWSHeader, valid)
	cases := map[string]string{
		"padding in the base64url":    good[:len(good)-1] + "=",
		"not base64url in the header": "*" + good[1:],
		"header null":                 signTokenBytes(s, "null", valid),
		"header an array":             signTokenBytes(s, "[]", valid),
		"header with trailing data":   signTokenBytes(s, profileJWSHeader+` {}`, valid),
		"header member null":          signTokenBytes(s, `{"alg":"EdDSA","kid":"terminal-key-1","typ":null}`, valid),
		"claims with trailing data":   signTokenBytes(s, profileJWSHeader, valid+` {}`),
		"claims with trailing text":   signTokenBytes(s, profileJWSHeader, valid+`x`),
		"header with trailing text":   signTokenBytes(s, profileJWSHeader+`]`, valid),
		"own missing":                 signTokenBytes(s, profileJWSHeader, tokenClaimsJSON(t, nil, "own")),
		"own a string":                signTokenBytes(s, profileJWSHeader, tokenClaimsJSON(t, map[string]any{"own": "true"})),
		"wdi with a fraction":         signTokenBytes(s, profileJWSHeader, tokenClaimsJSON(t, map[string]any{"wdi": 1.5})),
		"wdi with an exponent":        signTokenBytes(s, profileJWSHeader, tokenClaimsJSON(t, map[string]any{"wdi": json.Number("1e2")})),
		"wdi negative":                signTokenBytes(s, profileJWSHeader, tokenClaimsJSON(t, map[string]any{"wdi": -1})),
		"exp with a fraction":         signTokenBytes(s, profileJWSHeader, tokenClaimsJSON(t, map[string]any{"exp": json.Number("1798000000.5")})),
	}
	for name, tok := range cases {
		if _, _, err := ParseToken(tok, keys); !errors.Is(err, ErrToken) {
			t.Errorf("%s: %v", name, err)
		}
	}
}

// TestParseTokenKey (ADR 0067): the key of the header kid must be an Ed25519 OKP key with a 32-byte
// x; any other key with that kid refuses the token instead of falling through to another key.
func TestParseTokenKey(t *testing.T) {
	s := sessiontest.NewSigner("terminal-key-1")
	tok := signTokenBytes(s, profileJWSHeader, tokenClaimsJSON(t, nil))
	with := func(k, v string) map[string]any {
		m := maps.Clone(s.JWK())
		m[k] = v
		return m
	}
	short := b64url.EncodeToString(make([]byte, ed25519.PublicKeySize-1))
	for name, key := range map[string]map[string]any{
		"kty RSA":            with("kty", "RSA"),
		"crv X25519":         with("crv", "X25519"),
		"x not base64url":    with("x", "**"),
		"x of 31 bytes":      with("x", short),
		"x missing":          func() map[string]any { m := maps.Clone(s.JWK()); delete(m, "x"); return m }(),
		"another kid only":   with("kid", "terminal-key-2"),
		"the key of another": sessiontest.NewSigner("terminal-key-1").JWK(),
	} {
		if _, _, err := ParseToken(tok, []map[string]any{key, s.JWK()}); name != "another kid only" && !errors.Is(err, ErrToken) {
			t.Errorf("%s first: %v", name, err)
		}
		if _, _, err := ParseToken(tok, []map[string]any{key}); !errors.Is(err, ErrToken) {
			t.Errorf("%s alone: %v", name, err)
		}
	}
	if _, _, err := ParseToken(tok, []map[string]any{with("kid", "terminal-key-2"), s.JWK()}); err != nil {
		t.Errorf("a key with another kid is skipped: %v", err)
	}
}
