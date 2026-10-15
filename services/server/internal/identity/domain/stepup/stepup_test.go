// SPDX-License-Identifier: Apache-2.0

package stepup_test

import (
	"encoding/base64"
	"encoding/json"
	"strconv"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/identity/adapters/identitytest"
	"github.com/OWNER/bothy/services/server/internal/identity/domain/stepup"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/jose"
)

// pepper is the HMAC key of the tests (generated for the tests, never a secret).
var pepper = []byte("0123456789abcdef0123456789abcdef")

// signed returns a compact JWS over payload with header h, signed by key.
func signed(key *identitytest.Key, h jose.Header, payload map[string]any) string {
	body, err := json.Marshal(payload)
	if err != nil {
		panic(err)
	}
	return identitytest.SignJWS(key, h, body)
}

// numeric returns iat as a JSON integer.
func numeric(t time.Time) json.Number { return json.Number(strconv.FormatInt(t.Unix(), 10)) }

// rawJWS builds a compact JWS with a raw header (members the jose builder never writes), signed by
// key over payload.
func rawJWS(key *identitytest.Key, header string, payload map[string]any) string {
	body, err := json.Marshal(payload)
	if err != nil {
		panic(err)
	}
	input := base64.RawURLEncoding.EncodeToString([]byte(header)) + "." + base64.RawURLEncoding.EncodeToString(body)
	sig, err := key.Sign([]byte(input))
	if err != nil {
		panic(err)
	}
	return input + "." + base64.RawURLEncoding.EncodeToString(sig)
}

func TestStepUpPayload_Check(t *testing.T) {
	t.Parallel()
	stepKey, stepPub := identitytest.NewDeviceKey(jose.AlgES256)
	dpopKey, _ := identitytest.NewDeviceKey(jose.AlgES256)
	edKey, _ := identitytest.NewDeviceKey(jose.AlgEdDSA)
	jwk, err := jose.ParseJWK(stepPub.JWK)
	if err != nil {
		t.Fatal(err)
	}
	created := time.Date(2026, 10, 13, 9, 0, 0, 500_000_000, time.UTC)
	now := created.Add(10 * time.Second)
	leeway := 30 * time.Second
	sid := ids.New()
	value := "c2lnbi1tZS1wbGVhc2Utc2lnbi1tZS1wbGVhc2UtMDE"
	expect := stepup.Expect{
		Key: jwk, Alg: jose.AlgES256, ChallengeHash: stepup.ChallengeHash(pepper, value), Pepper: pepper,
		SessionID: sid, IssuedAt: created, Now: now, Leeway: leeway,
	}
	good := map[string]any{"challenge": value, "sid": sid.String(), "iat": numeric(now)}
	with := func(k string, v any) map[string]any {
		m := map[string]any{}
		for kk, vv := range good {
			m[kk] = vv
		}
		if v == nil {
			delete(m, k)
		} else {
			m[k] = v
		}
		return m
	}
	es := jose.Header{Alg: jose.AlgES256}
	cases := []struct {
		name string
		sig  string
		want stepup.Cause
	}{
		{"correct payload", signed(stepKey, es, good), ""},
		{"unknown members are ignored", signed(stepKey, es, with("extra", "x")), ""},
		{"iat at the lower edge", signed(stepKey, es, with("iat", numeric(created.Add(-leeway)))), ""},
		{"iat at the upper edge", signed(stepKey, es, with("iat", numeric(now.Add(leeway)))), ""},
		{"wrong challenge", signed(stepKey, es, with("challenge", "b3RoZXI")), stepup.CauseChallengeMismatch},
		{"challenge not a string", signed(stepKey, es, with("challenge", 7)), stepup.CausePayloadInvalid},
		{"wrong sid", signed(stepKey, es, with("sid", ids.New().String())), stepup.CauseSessionMismatch},
		{"iat before created_at minus leeway", signed(stepKey, es, with("iat", numeric(created.Add(-leeway-2*time.Second)))), stepup.CauseIATOutOfRange},
		{"iat after now plus leeway", signed(stepKey, es, with("iat", numeric(now.Add(leeway+time.Second)))), stepup.CauseIATOutOfRange},
		{"iat not an integer", signed(stepKey, es, with("iat", json.Number("1.5"))), stepup.CausePayloadInvalid},
		{"iat a string", signed(stepKey, es, with("iat", "1760000000")), stepup.CausePayloadInvalid},
		{"missing challenge", signed(stepKey, es, with("challenge", nil)), stepup.CausePayloadInvalid},
		{"missing sid", signed(stepKey, es, with("sid", nil)), stepup.CausePayloadInvalid},
		{"missing iat", signed(stepKey, es, with("iat", nil)), stepup.CausePayloadInvalid},
		{"crit header", rawJWS(stepKey, `{"alg":"ES256","crit":["exp"]}`, good), stepup.CauseInvalidJWS},
		{"jku header", rawJWS(stepKey, `{"alg":"ES256","jku":"https://example.com/k"}`, good), stepup.CauseInvalidJWS},
		{"x5c header", rawJWS(stepKey, `{"alg":"ES256","x5c":["AA"]}`, good), stepup.CauseInvalidJWS},
		{"jwk header", signed(stepKey, jose.Header{Alg: jose.AlgES256, JWK: stepKey.JWK}, good), stepup.CauseForbiddenHeader},
		{"x5u header", rawJWS(stepKey, `{"alg":"ES256","x5u":"https://example.com/c"}`, good), stepup.CauseInvalidJWS},
		{"alg differing from the key", signed(edKey, jose.Header{Alg: jose.AlgEdDSA}, good), stepup.CauseAlgMismatch},
		{"alg none", rawJWS(stepKey, `{"alg":"none"}`, good), stepup.CauseInvalidJWS},
		{"signed by the dpop key", signed(dpopKey, es, good), stepup.CauseBadSignature},
		{"not a JWS", "not.a.jws", stepup.CauseInvalidJWS},
		{"empty", "", stepup.CauseInvalidJWS},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			if got := stepup.Check(c.sig, expect); got != c.want {
				t.Fatalf("cause %q, want %q", got, c.want)
			}
		})
	}
	t.Run("leeway zero accepts iat in the second of issue", func(t *testing.T) {
		t.Parallel()
		e := expect
		e.Leeway, e.Now = 0, created
		if got := stepup.Check(signed(stepKey, es, with("iat", numeric(created))), e); got != "" {
			t.Fatalf("cause %q, want none", got)
		}
	})
}

func TestStepUpFailLimit_Decision(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 10, 13, 9, 0, 0, 0, time.UTC)
	window := 15 * time.Minute
	inside := []time.Time{now.Add(-time.Minute), now.Add(-14 * time.Minute), now}
	outside := []time.Time{now.Add(-window), now.Add(-time.Hour)}
	if n := stepup.Failures(append(append([]time.Time{}, inside...), outside...), now, window); n != len(inside) {
		t.Fatalf("failures %d, want %d (a failure exactly window ago is outside)", n, len(inside))
	}
	cases := []struct {
		name   string
		failed []time.Time
		limit  int
		want   bool
	}{
		{"one below the limit", inside[:2], 3, false},
		{"exactly at the limit", inside, 3, true},
		{"above the limit", inside, 2, true},
		{"old failures do not count", append(append([]time.Time{}, inside[:2]...), outside...), 3, false},
		{"none", nil, 1, false},
	}
	for _, c := range cases {
		if got := stepup.LimitReached(c.failed, now, window, c.limit); got != c.want {
			t.Errorf("%s: %v, want %v", c.name, got, c.want)
		}
	}
	// A later clock moves failures out of the window.
	if stepup.LimitReached(inside, now.Add(14*time.Minute+time.Second), window, 3) {
		t.Error("failures that left the window still count")
	}
}

// TestChallengeValue_IDT27Like: 32 bytes from crypto/rand as base64url without padding (43
// characters), distinct across many draws; the hash is HMAC-SHA-256 with the pepper.
func TestChallengeValue_IDT27Like(t *testing.T) {
	t.Parallel()
	seen := map[string]bool{}
	for range 2000 {
		v, err := stepup.NewChallengeValue()
		if err != nil {
			t.Fatal(err)
		}
		if len(v) != 43 {
			t.Fatalf("length %d, want 43", len(v))
		}
		b, err := base64.RawURLEncoding.DecodeString(v)
		if err != nil || len(b) != 32 {
			t.Fatalf("decoded %d bytes, %v", len(b), err)
		}
		if seen[v] {
			t.Fatal("value drawn twice")
		}
		seen[v] = true
	}
	h := stepup.ChallengeHash(pepper, "a")
	if len(h) != 32 || string(h) == string(stepup.ChallengeHash(pepper, "b")) ||
		string(h) == string(stepup.ChallengeHash([]byte("another pepper of thirty two b!!"), "a")) {
		t.Fatal("hash must be 32 bytes and depend on pepper and value")
	}
}
