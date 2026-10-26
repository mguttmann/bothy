// SPDX-License-Identifier: Apache-2.0

package demo

import (
	"errors"
	"strings"
	"testing"
	"time"
)

// TestZeroKey: the zero key is unusable and says so instead of signing.
func TestZeroKey(t *testing.T) {
	t.Parallel()
	var k key
	if k.publicJWK() != nil || k.privateJWK() != nil || k.thumbprint() != "" {
		t.Fatal("the zero key exported something")
	}
	if _, err := k.sign(nil, nil); !errors.Is(err, errNoKey) {
		t.Fatalf("sign: %v", err)
	}
}

// TestKeyForms: the JWKs carry exactly their members, a proof and a step-up signature are compact
// JWS of three parts, and a UUIDv7 has its version and variant bits.
func TestKeyForms(t *testing.T) {
	t.Parallel()
	k, err := newKey()
	if err != nil {
		t.Fatal(err)
	}
	pub, priv := k.publicJWK(), k.privateJWK()
	if len(pub) != 4 || len(priv) != 5 || priv["d"] == "" || pub["x"] != priv["x"] {
		t.Fatalf("jwks %v %v", pub, priv)
	}
	if tp := k.thumbprint(); len(tp) != 43 {
		t.Fatalf("thumbprint %q", tp)
	}
	for _, f := range []func() (string, error){
		func() (string, error) {
			return k.proof("GET", "https://chat.example.com/api/v1/me", "n", "token", time.Now())
		},
		func() (string, error) { return k.stepUpSignature("c", "s", time.Now()) },
	} {
		jws, err := f()
		if err != nil || strings.Count(jws, ".") != 2 {
			t.Fatalf("jws %q %v", jws, err)
		}
	}
	u := newUUIDv7()
	if u[6]>>4 != 7 || u[8]>>6 != 2 {
		t.Fatalf("uuid %s", u)
	}
}
