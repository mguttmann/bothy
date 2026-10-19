// SPDX-License-Identifier: Apache-2.0

package secretbox_test

import (
	"bytes"
	"context"
	"encoding/hex"
	"errors"
	"strings"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/identity/adapters/secretbox"
	"github.com/OWNER/bothy/services/server/internal/identity/app"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// Compile-time check: the box is the outbound port.
var _ app.SecretSealer = (*secretbox.Box)(nil)

// keys is a fake secret provider: reference to the stored value (32 bytes as hex).
type keys map[string]string

// Resolve implements secretbox.Resolver.
func (k keys) Resolve(_ context.Context, ref string) ([]byte, error) {
	v, ok := k[ref]
	if !ok {
		return nil, errors.New("unknown reference")
	}
	return []byte(v), nil
}

func hexKey(b byte) string { return hex.EncodeToString(bytes.Repeat([]byte{b}, 32)) }

func TestSecretBoxBindsIdentity(t *testing.T) {
	t.Parallel()
	src := keys{"file:a": hexKey(1), "file:b": hexKey(2)}
	box, err := secretbox.New(context.Background(), src, "file:a")
	if err != nil {
		t.Fatal(err)
	}
	alex, sam := ids.New(), ids.New()
	secret := bytes.Repeat([]byte("1234567890"), 2)
	ct, ref, err := box.Seal(alex, secret)
	if err != nil || ref != "file:a" {
		t.Fatalf("seal: ref %q err %v", ref, err)
	}
	if bytes.Contains(ct, secret) || ct[0] != 0x01 || len(ct) != 1+12+len(secret)+16 {
		t.Fatalf("layout: %d bytes, version %#x", len(ct), ct[0])
	}
	if again, _, _ := box.Seal(alex, secret); bytes.Equal(again, ct) {
		t.Fatal("two seals share a nonce")
	}
	got, err := box.Open(alex, ct, ref)
	if err != nil || !bytes.Equal(got, secret) {
		t.Fatalf("open: %q %v", got, err)
	}
	if _, err := box.Open(sam, ct, ref); err == nil {
		t.Fatal("opened with another identity")
	}
	wrongVersion := bytes.Clone(ct)
	wrongVersion[0] = 0x02
	if _, err := box.Open(alex, wrongVersion, ref); err == nil {
		t.Fatal("opened with a wrong version byte")
	}
	tampered := bytes.Clone(ct)
	tampered[len(tampered)-1] ^= 0x01
	if _, err := box.Open(alex, tampered, ref); err == nil {
		t.Fatal("opened a tampered ciphertext")
	}
	if _, err := box.Open(alex, ct[:10], ref); err == nil {
		t.Fatal("opened a truncated ciphertext")
	}
	// A new key applies to new rows; old rows stay readable through their own reference.
	next, err := secretbox.New(context.Background(), src, "file:b")
	if err != nil {
		t.Fatal(err)
	}
	if got, err := next.Open(alex, ct, "file:a"); err != nil || !bytes.Equal(got, secret) {
		t.Fatalf("old row with new key: %v", err)
	}
	if _, err := next.Open(alex, ct, "file:b"); err == nil {
		t.Fatal("opened with the wrong key")
	}
	if _, err := next.Open(alex, ct, "file:missing"); err == nil {
		t.Fatal("opened with an unknown reference")
	}
}

func TestSecretBoxKeyLength(t *testing.T) {
	t.Parallel()
	for name, v := range map[string]string{"short": hex.EncodeToString(make([]byte, 16)), "not hex": strings.Repeat("z", 64), "long": hexKey(3) + "00"} {
		_, err := secretbox.New(context.Background(), keys{"file:k": v}, "file:k")
		if err == nil {
			t.Errorf("%s key accepted", name)
		} else if strings.Contains(err.Error(), v) {
			t.Errorf("%s: the error repeats the key", name)
		}
	}
	if _, err := secretbox.New(context.Background(), keys{}, "file:k"); err == nil {
		t.Fatal("missing key accepted")
	}
	// Surrounding whitespace of a key file (a trailing newline) is allowed.
	if _, err := secretbox.New(context.Background(), keys{"file:k": hexKey(4) + "\n"}, "file:k"); err != nil {
		t.Fatal(err)
	}
}
