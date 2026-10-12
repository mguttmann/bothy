// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/identity"
	"github.com/OWNER/bothy/services/server/internal/identity/adapters/identitytest"
	"github.com/OWNER/bothy/services/server/internal/identity/adapters/replay"
	"github.com/OWNER/bothy/services/server/internal/identity/app"
	"github.com/OWNER/bothy/services/server/internal/identity/domain/dpop"
	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/jose"
)

const (
	testURL   = "https://chat.example.com/api/v1/me"
	testToken = "access.token.of-the-test"
)

var testDPoPConfig = app.Config{DPoPNonceTTL: 5 * time.Minute, DPoPProofMaxAge: 60 * time.Second, ClockLeeway: 30 * time.Second}

type dpopEnv struct {
	clk *clock.Fake
	v   *app.DPoPVerifier
	key *identitytest.Key
}

func newDPoPEnv(t *testing.T, pepper string) *dpopEnv {
	t.Helper()
	clk := clock.NewFake(time.Date(2026, 10, 12, 9, 2, 30, 0, time.UTC))
	v, err := app.NewDPoPVerifier(testDPoPConfig, []byte(pepper), replay.New(clk), clk, slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatal(err)
	}
	key, _ := identitytest.NewDeviceKey(jose.AlgES256)
	return &dpopEnv{clk: clk, v: v, key: key}
}

// claims are the claims of a valid proof for testURL with the current nonce and the test token.
func (e *dpopEnv) claims() identitytest.ProofClaims {
	return identitytest.ProofClaims{HTM: "GET", HTU: testURL, IAT: e.clk.Now(), Nonce: e.v.CurrentNonce(), ATH: dpop.ATH(testToken)}
}

func (e *dpopEnv) request(proofs ...string) identity.ProofRequest {
	return identity.ProofRequest{Proofs: proofs, Method: "GET", URL: testURL, AccessToken: testToken, ExpectedJKT: e.key.Thumbprint}
}

// codeOf returns the DPoPError code of err, or "" for none.
func codeOf(err error) string {
	var de *identity.DPoPError
	if errors.As(err, &de) {
		return de.Code
	}
	return ""
}

// TestDPoP_IDT14 holds the DPoP cases of ID-T14 (without the Bearer case, M1-34) with their codes.
func TestDPoP_IDT14(t *testing.T) {
	t.Parallel()
	const invalid, nonce, ok = identity.DPoPInvalidProof, identity.DPoPUseNonce, ""
	other, _ := identitytest.NewDeviceKey(jose.AlgEdDSA)
	cases := []struct {
		name  string
		build func(e *dpopEnv) identity.ProofRequest
		want  string
		stale bool
	}{
		{"valid", func(e *dpopEnv) identity.ProofRequest {
			return e.request(identitytest.SignDPoPProof(e.key, e.claims()))
		}, ok, false},
		{"wrong htm", func(e *dpopEnv) identity.ProofRequest {
			c := e.claims()
			c.HTM = "POST"
			return e.request(identitytest.SignDPoPProof(e.key, c))
		}, invalid, false},
		{"wrong htu host", mutate(func(c *identitytest.ProofClaims) { c.HTU = "https://other.example.com/api/v1/me" }), invalid, false},
		{"wrong htu path", mutate(func(c *identitytest.ProofClaims) { c.HTU = "https://chat.example.com/api/v1/you" }), invalid, false},
		{"wrong htu scheme", mutate(func(c *identitytest.ProofClaims) { c.HTU = "http://chat.example.com/api/v1/me" }), invalid, false},
		{"htu with query and fragment", mutate(func(c *identitytest.ProofClaims) { c.HTU = "https://Chat.example.com:443/api/v1/me?a=1#f" }), ok, false},
		{"wrong ath", mutate(func(c *identitytest.ProofClaims) { c.ATH = dpop.ATH("another token") }), invalid, false},
		{"ath without token", func(e *dpopEnv) identity.ProofRequest {
			r := e.request(identitytest.SignDPoPProof(e.key, e.claims()))
			r.AccessToken = ""
			return r
		}, invalid, false},
		{"iat older than max age", func(e *dpopEnv) identity.ProofRequest {
			c := e.claims()
			c.IAT = e.clk.Now().Add(-61 * time.Second)
			return e.request(identitytest.SignDPoPProof(e.key, c))
		}, invalid, false},
		{"iat newer than leeway", func(e *dpopEnv) identity.ProofRequest {
			c := e.claims()
			c.IAT = e.clk.Now().Add(31 * time.Second)
			return e.request(identitytest.SignDPoPProof(e.key, c))
		}, invalid, false},
		{"missing nonce", mutate(func(c *identitytest.ProofClaims) { c.Nonce = "" }), nonce, false},
		{"nonce of two windows ago", func(e *dpopEnv) identity.ProofRequest {
			c := e.claims()
			e.clk.Advance(-10 * time.Minute)
			c.Nonce = e.v.CurrentNonce()
			e.clk.Advance(10 * time.Minute)
			return e.request(identitytest.SignDPoPProof(e.key, c))
		}, nonce, false},
		{"forged nonce MAC", mutate(func(c *identitytest.ProofClaims) { c.Nonce = c.Nonce[:len(c.Nonce)-2] + "AA" }), nonce, false},
		{"previous window", func(e *dpopEnv) identity.ProofRequest {
			c := e.claims()
			e.clk.Advance(-5 * time.Minute)
			c.Nonce = e.v.CurrentNonce()
			e.clk.Advance(5 * time.Minute)
			return e.request(identitytest.SignDPoPProof(e.key, c))
		}, ok, true},
		{"proof key differs from ExpectedJKT", func(e *dpopEnv) identity.ProofRequest {
			return e.request(identitytest.SignDPoPProof(other, e.claims()))
		}, invalid, false},
		{"two DPoP headers", func(e *dpopEnv) identity.ProofRequest {
			p := identitytest.SignDPoPProof(e.key, e.claims())
			return e.request(p, p)
		}, invalid, false},
		{"no DPoP header", func(e *dpopEnv) identity.ProofRequest { return e.request() }, invalid, false},
		{"typ JWT", mutate(func(c *identitytest.ProofClaims) { c.Typ = "JWT" }), invalid, false},
		{"header jwk of another key", func(e *dpopEnv) identity.ProofRequest {
			c := e.claims()
			c.JWK = other.JWK
			return e.request(identitytest.SignDPoPProof(e.key, c))
		}, invalid, false},
		{"header without jwk", func(e *dpopEnv) identity.ProofRequest {
			p := identitytest.SignJWS(e.key, jose.Header{Typ: "dpop+jwt"}, []byte(`{"jti":"j","htm":"GET","htu":"`+testURL+`","iat":1,"nonce":"n"}`))
			return e.request(p)
		}, invalid, false},
		{"not a JWS", func(e *dpopEnv) identity.ProofRequest { return e.request("not.a.jws") }, invalid, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			e := newDPoPEnv(t, "a pepper of thirty-two bytes ...")
			got, err := e.v.Verify(context.Background(), tc.build(e))
			if c := codeOf(err); c != tc.want || (tc.want == ok && err != nil) {
				t.Fatalf("Verify = %+v, %v; want code %q", got, err, tc.want)
			}
			if tc.want != ok {
				var de *identity.DPoPError
				_ = errors.As(err, &de)
				if de.Nonce != e.v.CurrentNonce() {
					t.Errorf("error nonce %q, want the current one", de.Nonce)
				}
				if e, ok := errs.As(err); !ok || e.Kind() != errs.KindUnauthenticated || string(e.PublicCode()) != tc.want {
					t.Errorf("errs view %v", err)
				}
				return
			}
			if got.NonceStale != tc.stale || got.JKT != e.key.Thumbprint || got.Alg != jose.AlgES256 || got.IssuedAt.IsZero() || len(got.PublicJWK) == 0 {
				t.Errorf("verified %+v", got)
			}
		})
	}
}

// mutate builds a request from the valid claims changed by f.
func mutate(f func(*identitytest.ProofClaims)) func(e *dpopEnv) identity.ProofRequest {
	return func(e *dpopEnv) identity.ProofRequest {
		c := e.claims()
		f(&c)
		return e.request(identitytest.SignDPoPProof(e.key, c))
	}
}

func TestDPoP_Replay(t *testing.T) {
	t.Parallel()
	e := newDPoPEnv(t, "a pepper of thirty-two bytes ...")
	c := e.claims()
	c.JTI = "same-jti"
	p := identitytest.SignDPoPProof(e.key, c)
	if _, err := e.v.Verify(context.Background(), e.request(p)); err != nil {
		t.Fatal(err)
	}
	if _, err := e.v.Verify(context.Background(), e.request(p)); codeOf(err) != identity.DPoPInvalidProof {
		t.Fatalf("repeated jti: %v", err)
	}
	// The same jti for another htu passes: the store keys per htu.
	c.HTU = "https://chat.example.com/api/v1/devices"
	r := e.request(identitytest.SignDPoPProof(e.key, c))
	r.URL = c.HTU
	if _, err := e.v.Verify(context.Background(), r); err != nil {
		t.Fatalf("same jti, other htu: %v", err)
	}
	// A rejected proof is not remembered: fixing the nonce lets the same jti pass.
	c = e.claims()
	c.JTI, c.Nonce = "fresh", ""
	if _, err := e.v.Verify(context.Background(), e.request(identitytest.SignDPoPProof(e.key, c))); codeOf(err) != identity.DPoPUseNonce {
		t.Fatalf("without nonce: %v", err)
	}
	c.Nonce = e.v.CurrentNonce()
	if _, err := e.v.Verify(context.Background(), e.request(identitytest.SignDPoPProof(e.key, c))); err != nil {
		t.Fatalf("after a rejection the jti was remembered: %v", err)
	}
}

func TestDPoP_NonceAcrossVerifiers(t *testing.T) {
	t.Parallel()
	a := newDPoPEnv(t, "a pepper of thirty-two bytes ...")
	b := newDPoPEnv(t, "a pepper of thirty-two bytes ...")
	c := newDPoPEnv(t, "another pepper, thirty-two bytes")
	if a.v.CurrentNonce() != b.v.CurrentNonce() || a.v.CurrentNonce() == c.v.CurrentNonce() {
		t.Fatal("nonces do not follow the pepper")
	}
	claims := a.claims()
	claims.Nonce = b.v.CurrentNonce()
	if _, err := a.v.Verify(context.Background(), a.request(identitytest.SignDPoPProof(a.key, claims))); err != nil {
		t.Fatalf("nonce of another verifier with the same pepper: %v", err)
	}
	claims = c.claims()
	claims.Nonce = a.v.CurrentNonce()
	if _, err := c.v.Verify(context.Background(), c.request(identitytest.SignDPoPProof(c.key, claims))); codeOf(err) != identity.DPoPUseNonce {
		t.Fatalf("nonce of a verifier with another pepper: %v", err)
	}
}

func TestDPoP_ReplayConcurrent(t *testing.T) {
	t.Parallel()
	e := newDPoPEnv(t, "a pepper of thirty-two bytes ...")
	p := identitytest.SignDPoPProof(e.key, e.claims())
	var passed atomic.Int64
	var wg sync.WaitGroup
	for range 64 {
		wg.Go(func() {
			if _, err := e.v.Verify(context.Background(), e.request(p)); err == nil {
				passed.Add(1)
			}
		})
	}
	wg.Wait()
	if passed.Load() != 1 {
		t.Fatalf("%d of 64 uses passed, want exactly 1", passed.Load())
	}
}

func TestDPoP_ConfigAndContext(t *testing.T) {
	t.Parallel()
	clk := clock.NewFake(time.Now())
	if _, err := app.NewDPoPVerifier(testDPoPConfig, nil, replay.New(clk), clk, nil); err == nil {
		t.Error("empty pepper accepted")
	}
	if _, err := app.NewDPoPVerifier(app.Config{DPoPNonceTTL: 0, DPoPProofMaxAge: time.Second}, []byte("p"), replay.New(clk), clk, nil); err == nil {
		t.Error("nonce TTL below one second accepted")
	}
	e := newDPoPEnv(t, "a pepper of thirty-two bytes ...")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := e.v.Verify(ctx, e.request(identitytest.SignDPoPProof(e.key, e.claims()))); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled context: %v", err)
	}
}
