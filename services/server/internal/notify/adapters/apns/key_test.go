// SPDX-License-Identifier: Apache-2.0

package apns_test

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"strings"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/notify/adapters/apns"
	"github.com/OWNER/bothy/services/server/internal/platform/secrets"
)

func TestParsePrivateKey(t *testing.T) {
	t.Parallel()
	good := newKey(t)
	k, err := apns.ParsePrivateKey(secrets.NewValue(pemPKCS8(t, good)))
	if err != nil || !k.Equal(good) {
		t.Fatalf("P-256 PKCS #8: %v", err)
	}
	p384, err := ecdsa.GenerateKey(elliptic.P384(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	rsaKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	sec1, err := x509.MarshalECPrivateKey(good)
	if err != nil {
		t.Fatal(err)
	}
	cases := map[string][]byte{
		"P-384":          pemPKCS8(t, p384),
		"RSA":            pemPKCS8(t, rsaKey),
		"SEC 1":          pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: sec1}),
		"two blocks":     append(pemPKCS8(t, good), pemPKCS8(t, good)...),
		"garbage":        []byte("not a key at all"),
		"empty":          nil,
		"broken PKCS #8": pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: []byte("broken")}),
	}
	for name, in := range cases {
		_, err := apns.ParsePrivateKey(secrets.NewValue(in))
		if err == nil {
			t.Errorf("%s: parsed", name)
			continue
		}
		for _, line := range strings.Split(string(in), "\n") {
			if len(line) > 8 && strings.Contains(err.Error(), line) {
				t.Errorf("%s: error %q contains input", name, err)
			}
		}
	}
}
