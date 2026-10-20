// SPDX-License-Identifier: Apache-2.0

package session

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"maps"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/host/internal/session/sessiontest"
)

const (
	hostID  = "0192f4a0-7b1c-7b22-9d4f-5a6b7c8d9e0f"
	sessID  = "0192f4a0-7b1c-7b22-9d4f-00000000a001"
	otherID = "0192f4a0-7b1c-7b22-9d4f-00000000a002"
	alexSub = "0192f4a0-7b1c-7b22-9d4f-0000000a1e00"
	samSub  = "0192f4a0-7b1c-7b22-9d4f-0000000005a0"
	alexOSU = "bothy-s-alex"
	samOSU  = "bothy-s-sam"
	cek     = "Q2VrT2ZUaGVEZXZpY2VFcGhlbWVyYWxLZXlIYXNoMDA"
)

var now = time.Date(2026, 10, 20, 9, 0, 0, 0, time.UTC)

// memStore is a BindingStore in memory; writes counts the writes, fail makes every write fail.
type memStore struct {
	mu     sync.Mutex
	m      map[string]string
	writes int
	fail   bool
}

func (s *memStore) ReadBindings() (map[string]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return maps.Clone(s.m), nil
}

func (s *memStore) WriteBindings(m map[string]string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.fail {
		return errors.New("write failed")
	}
	s.writes++
	s.m = maps.Clone(m)
	return nil
}

type fixture struct {
	signer *sessiontest.Signer
	v      *Verifier
	store  *memStore
	live   map[string]Live
	server bool
}

func newFixture() *fixture {
	st := &memStore{m: map[string]string{}}
	return &fixture{signer: sessiontest.NewSigner("terminal-key-1"), v: &Verifier{JTI: NewJTIMemory(), Bindings: NewBindings(st)}, store: st, live: map[string]Live{}}
}

// claims returns valid claims of an opening of the profile shell.
func (f *fixture) claims() map[string]any {
	return sessiontest.Claims(hostID, sessID, alexSub, alexOSU, "shell", cek, AttOpen, now, 30*time.Second)
}

func (f *fixture) check(tok string) Check {
	return Check{Token: tok, Keys: []map[string]any{f.signer.JWK()}, HostID: hostID, SessionID: sessID, Now: now, Leeway: 30 * time.Second,
		Cek: cek, Live: func(sid string) (Live, bool) { l, ok := f.live[sid]; return l, ok },
		Offered:      []OfferedProfile{{Name: "shell", Kind: "shell"}, {Name: "claude", Kind: "claude"}},
		SessionUsers: []string{alexOSU, samOSU}, Workdirs: func(osu string) int { return map[string]int{alexOSU: 2}[osu] }, ServerHost: f.server}
}

func (f *fixture) verify(c Check) string {
	_, d := f.v.Verify(c)
	return d
}

// h returns a short hash prefix of s for failure messages (never the token itself).
func h(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:4])
}

func TestVerifyOrder(t *testing.T) {
	type row struct {
		name   string
		mutate func(f *fixture, c map[string]any) Check
		want   string
	}
	sign := func(f *fixture, c map[string]any) Check { return f.check(f.signer.Sign(c)) }
	header := func(hdr map[string]any) func(f *fixture, c map[string]any) Check {
		return func(f *fixture, c map[string]any) Check { return f.check(f.signer.SignRaw(hdr, c)) }
	}
	base := map[string]any{"alg": "EdDSA", "kid": "terminal-key-1", "typ": TokenType}
	with := func(k string, v any) map[string]any { m := maps.Clone(base); m[k] = v; return m }
	rows := []row{
		{"valid shell", sign, ""},
		{"wrong signature", func(f *fixture, c map[string]any) Check {
			other := sessiontest.NewSigner("terminal-key-1")
			return f.check(other.Sign(c))
		}, DetailTokenInvalid},
		{"unknown kid", header(with("kid", "terminal-key-9")), DetailTokenInvalid},
		{"alg none", header(with("alg", "none")), DetailTokenInvalid},
		{"alg HS256", header(with("alg", "HS256")), DetailTokenInvalid},
		{"alg ES256", header(with("alg", "ES256")), DetailTokenInvalid},
		{"crit member", header(with("crit", []string{"exp"})), DetailTokenInvalid},
		{"jwk member", header(with("jwk", map[string]any{"kty": "OKP"})), DetailTokenInvalid},
		{"three dots", func(f *fixture, c map[string]any) Check { return f.check(f.signer.Sign(c) + ".x") }, DetailTokenInvalid},
		{"null claim", func(f *fixture, c map[string]any) Check { c["dev"] = nil; return sign(f, c) }, DetailTokenInvalid},
		{"missing osu", func(f *fixture, c map[string]any) Check { delete(c, "osu"); return sign(f, c) }, DetailTokenInvalid},
		{"typ of the invocation token", header(with("typ", "agent-invocation+jwt")), DetailTokenInvalid},
		{"aud of another host", func(f *fixture, c map[string]any) Check { c["aud"] = otherID; return sign(f, c) }, DetailTokenInvalid},
		{"sid of another session", func(f *fixture, c map[string]any) Check { c["sid"] = otherID; return sign(f, c) }, DetailTokenInvalid},
		{"now equal to exp plus leeway", func(f *fixture, c map[string]any) Check {
			c["exp"] = now.Add(-30 * time.Second).Unix()
			return sign(f, c)
		}, ""},
		{"one second later", func(f *fixture, c map[string]any) Check {
			c["exp"] = now.Add(-31 * time.Second).Unix()
			return sign(f, c)
		}, DetailTokenExpired},
		{"seen jti", func(f *fixture, c map[string]any) Check {
			f.v.JTI.Remember(c["jti"].(string), now.Add(time.Minute), now)
			return sign(f, c)
		}, DetailTokenReplayed},
		{"cek of another key", func(f *fixture, c map[string]any) Check { c["cek"] = "b3RoZXI"; return sign(f, c) }, DetailTokenInvalid},
		{"open for a running session", func(f *fixture, c map[string]any) Check {
			f.live[sessID] = Live{Sub: alexSub, Osu: alexOSU, Prf: "shell"}
			return sign(f, c)
		}, DetailTokenInvalid},
		{"attach for an unknown session", func(f *fixture, c map[string]any) Check { c["att"] = AttAttach; return sign(f, c) }, DetailTokenInvalid},
		{"attach with another sub", func(f *fixture, c map[string]any) Check {
			f.live[sessID] = Live{Sub: samSub, Osu: alexOSU, Prf: "shell"}
			c["att"] = AttAttach
			return sign(f, c)
		}, DetailTokenInvalid},
		{"attach with another osu", func(f *fixture, c map[string]any) Check {
			f.live[sessID] = Live{Sub: alexSub, Osu: samOSU, Prf: "shell"}
			c["att"] = AttAttach
			return sign(f, c)
		}, DetailTokenInvalid},
		{"attach with another prf", func(f *fixture, c map[string]any) Check {
			f.live[sessID] = Live{Sub: alexSub, Osu: alexOSU, Prf: "claude"}
			c["att"] = AttAttach
			return sign(f, c)
		}, DetailTokenInvalid},
		{"attach to a running session", func(f *fixture, c map[string]any) Check {
			f.live[sessID] = Live{Sub: alexSub, Osu: alexOSU, Prf: "shell"}
			c["att"] = AttAttach
			return sign(f, c)
		}, ""},
		{"unknown prf", func(f *fixture, c map[string]any) Check { c["prf"] = "top"; return sign(f, c) }, DetailProfileUnknown},
		{"osu not configured", func(f *fixture, c map[string]any) Check { c["osu"] = "bothy-s-robin"; return sign(f, c) }, DetailSessionUserUnknown},
		{"osu bound to another sub", func(f *fixture, c map[string]any) Check {
			f.store.m[alexOSU] = samSub
			return sign(f, c)
		}, DetailSessionUserMismatch},
		{"wdi missing for claude", func(f *fixture, c map[string]any) Check { c["prf"] = "claude"; return sign(f, c) }, DetailProfileUnknown},
		{"wdi out of range for claude", func(f *fixture, c map[string]any) Check { c["prf"] = "claude"; c["wdi"] = 2; return sign(f, c) }, DetailProfileUnknown},
		{"wdi negative", func(f *fixture, c map[string]any) Check { c["prf"] = "claude"; c["wdi"] = -1; return sign(f, c) }, DetailTokenInvalid},
		{"wdi in range for claude", func(f *fixture, c map[string]any) Check { c["prf"] = "claude"; c["wdi"] = 1; return sign(f, c) }, ""},
		{"wdi present for shell", func(f *fixture, c map[string]any) Check { c["wdi"] = 0; return sign(f, c) }, DetailProfileUnknown},
		{"own false on a detected server host", func(f *fixture, c map[string]any) Check { f.server = true; return sign(f, c) }, DetailServerHostOwnerOnly},
		{"own true on a detected server host", func(f *fixture, c map[string]any) Check { f.server = true; c["own"] = true; return sign(f, c) }, ""},
		// Order rows.
		{"wrong aud and expired", func(f *fixture, c map[string]any) Check {
			c["aud"] = otherID
			c["exp"] = now.Add(-time.Hour).Unix()
			return sign(f, c)
		}, DetailTokenInvalid},
		{"expired and wrong cek", func(f *fixture, c map[string]any) Check {
			c["cek"] = "b3RoZXI"
			c["exp"] = now.Add(-time.Hour).Unix()
			return sign(f, c)
		}, DetailTokenExpired},
		{"replayed and unknown osu", func(f *fixture, c map[string]any) Check {
			c["osu"] = "bothy-s-robin"
			f.v.JTI.Remember(c["jti"].(string), now.Add(time.Minute), now)
			return sign(f, c)
		}, DetailTokenReplayed},
	}
	for _, r := range rows {
		t.Run(r.name, func(t *testing.T) {
			f := newFixture()
			c := r.mutate(f, f.claims())
			if got := f.verify(c); got != r.want {
				t.Errorf("detail %q, want %q (token %s)", got, r.want, h(c.Token))
			}
		})
	}
}

func TestVerifyJTIRememberedBeforeLaterSteps(t *testing.T) {
	f := newFixture()
	c := f.claims()
	c["osu"] = "bothy-s-robin"
	tok := f.signer.Sign(c)
	if d := f.verify(f.check(tok)); d != DetailSessionUserUnknown {
		t.Fatalf("first %q", d)
	}
	if d := f.verify(f.check(tok)); d != DetailTokenReplayed {
		t.Errorf("a replay of a token that failed later is token_replayed, got %q", d)
	}
}

func TestBindingStaysWhenALaterStepFails(t *testing.T) {
	f := newFixture()
	c := f.claims()
	c["wdi"] = 0 // step 12 fails after the binding
	if d := f.verify(f.check(f.signer.Sign(c))); d != DetailProfileUnknown {
		t.Fatalf("detail %q", d)
	}
	if f.store.m[alexOSU] != alexSub {
		t.Error("the binding of step 11 stays")
	}
}

// TestVerifyRFC8037Vector checks the Ed25519 key handling with the example of RFC 8037 A.4 and a
// token of the profile signed with the same key (typ set by the test).
func TestVerifyRFC8037Vector(t *testing.T) {
	seed, _ := hex.DecodeString("9d61b19deffd5a60ba844af492ec2cc44449c5697b326919703bac031cae7f60")
	key := ed25519.NewKeyFromSeed(seed)
	jwk := map[string]any{"kty": "OKP", "crv": "Ed25519", "x": "11qYAYKxCrfVS_7TyWQHOg7hcvPapiMlrwIaaPcHURo", "kid": "rfc8037"}
	pub, err := keyOf([]map[string]any{jwk}, "rfc8037")
	if err != nil || !pub.Equal(key.Public()) {
		t.Fatalf("the JWK of A.2 is not the key of A.1: %v", err)
	}
	const input = "eyJhbGciOiJFZERTQSJ9.RXhhbXBsZSBvZiBFZDI1NTE5IHNpZ25pbmc"
	sig, _ := b64url.DecodeString("hgyY0il_MGCjP0JzlnLWG1PPOt7-09PGcvMg3AIbQR6dWbhijcNR4ki4iylGjg5BhVsPt9g7sVvpAr_MuM0KAg")
	if !ed25519.Verify(pub, []byte(input), sig) {
		t.Fatal("the signature of A.4 does not verify")
	}
	// The A.4 JWS itself lacks kid and typ, which the profile requires.
	if _, _, err := ParseToken(input+"."+b64url.EncodeToString(sig), []map[string]any{jwk}); !errors.Is(err, ErrToken) {
		t.Error("a JWS without kid and typ passed the profile")
	}
	s := &sessiontest.Signer{Kid: "rfc8037", Key: key}
	tok := s.Sign(sessiontest.Claims(hostID, sessID, alexSub, alexOSU, "shell", cek, AttOpen, now, time.Minute))
	if hdr, _, err := ParseToken(tok, []map[string]any{jwk}); err != nil || hdr.Typ != TokenType {
		t.Errorf("a token of the profile with the A.1 key: %v", err)
	}
}

func TestJTIMemory(t *testing.T) {
	j := NewJTIMemory()
	if !j.Remember("a", now.Add(time.Minute), now) || j.Remember("a", now.Add(time.Minute), now) {
		t.Fatal("first use new, second use seen")
	}
	if !j.Remember("b", now.Add(2*time.Minute), now.Add(30*time.Second)) || j.Len() != 2 {
		t.Fatal("two entries")
	}
	// After the time of "a" it is pruned.
	j.Remember("c", now.Add(3*time.Minute), now.Add(90*time.Second))
	if j.Len() != 2 {
		t.Errorf("pruned: %d entries", j.Len())
	}
	var wg sync.WaitGroup
	news := make(chan bool, 64)
	for range 64 {
		wg.Go(func() { news <- j.Remember("race", now.Add(time.Hour), now) })
	}
	wg.Wait()
	close(news)
	n := 0
	for ok := range news {
		if ok {
			n++
		}
	}
	if n != 1 {
		t.Errorf("concurrent first uses: %d new", n)
	}
}

func TestBindingConcurrentFirstUse(t *testing.T) {
	st := &memStore{m: map[string]string{}}
	b := NewBindings(st)
	var wg sync.WaitGroup
	res := make([]bool, 2)
	for i, sub := range []string{alexSub, samSub} {
		wg.Go(func() { res[i] = b.Bind(alexOSU, sub) })
	}
	wg.Wait()
	if res[0] == res[1] {
		t.Fatalf("exactly one binds: %v", res)
	}
	// A simulated crash right after step 11: a new Bindings over the same store sees the binding.
	winner := map[bool]string{true: alexSub, false: samSub}[res[0]]
	if st.m[alexOSU] != winner || !NewBindings(st).Bind(alexOSU, winner) || NewBindings(st).Bind(alexOSU, map[string]string{alexSub: samSub, samSub: alexSub}[winner]) {
		t.Errorf("binding after restart: %v", st.m)
	}
	failing := &memStore{m: map[string]string{}, fail: true}
	if NewBindings(failing).Bind(alexOSU, alexSub) {
		t.Error("a binding that cannot be written must refuse")
	}
}

func FuzzVerifySessionToken(f *testing.F) {
	fx := newFixture()
	valid := fx.signer.Sign(fx.claims())
	f.Add(valid)
	f.Add(strings.Replace(valid, ".", "..", 1))
	f.Add("e30.e30.")
	f.Add("")
	f.Fuzz(func(t *testing.T, tok string) {
		fx := newFixture()
		c := fx.check(tok)
		cl, d := fx.v.Verify(c)
		if d == "" {
			// A pass needs a signature of the key: only the seeded valid token can pass.
			if tok != valid {
				parts := strings.Split(tok, ".")
				if len(parts) != 3 {
					t.Fatalf("pass without three parts (%s)", h(tok))
				}
				sig, _ := b64url.DecodeString(parts[2])
				pub, _ := fx.signer.Key.Public().(ed25519.PublicKey)
				if !ed25519.Verify(pub, []byte(parts[0]+"."+parts[1]), sig) {
					t.Fatalf("pass without a valid signature (%s)", h(tok))
				}
			}
			if cl.Sid != sessID {
				t.Fatalf("pass with sid of length %d", len(cl.Sid))
			}
		}
	})
}
