// SPDX-License-Identifier: Apache-2.0

package instance_test

import (
	"bytes"
	"crypto/ed25519"
	"crypto/x509"
	"encoding/base64"
	"strings"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/instance"
)

// TestFormatWebhookPublicKey: whpk_ and the standard base64 of the DER SubjectPublicKeyInfo,
// which x509.ParsePKIXPublicKey reads back to the same key (D-M2-26-11).
func TestFormatWebhookPublicKey(t *testing.T) {
	t.Parallel()
	k := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{7}, ed25519.SeedSize)).Public().(ed25519.PublicKey)
	s := instance.FormatWebhookPublicKey(k)
	rest, ok := strings.CutPrefix(s, "whpk_")
	if !ok {
		t.Fatalf("no prefix: %q", s)
	}
	der, err := base64.StdEncoding.DecodeString(rest)
	if err != nil {
		t.Fatal(err)
	}
	back, err := x509.ParsePKIXPublicKey(der)
	if err != nil {
		t.Fatal(err)
	}
	if pk, ok := back.(ed25519.PublicKey); !ok || !pk.Equal(k) {
		t.Fatalf("round trip gives %v", back)
	}
	if instance.FormatWebhookPublicKey(k[:10]) != "" {
		t.Fatal("a short key was encoded")
	}
}
