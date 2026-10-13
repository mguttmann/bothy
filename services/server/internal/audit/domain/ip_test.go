// SPDX-License-Identifier: Apache-2.0

package domain_test

import (
	"bytes"
	"crypto/rand"
	"net/netip"
	"strings"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/audit/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// vectorKey is the fixed key of the HMAC vectors: the bytes 0x00 to 0x1f.
func vectorKey() []byte {
	k := make([]byte, 32)
	for i := range k {
		k[i] = byte(i)
	}
	return k
}

func TestIPHMAC_Vectors(t *testing.T) {
	t.Parallel()
	// Expected values computed independently: hmac.new(bytes(range(32)), text, sha256) in Python.
	for addr, want := range map[string]string{
		"192.0.2.44":   "484cdb33c0435b92fae328daf767daf7799188e7d228e7a6269ad66533a11ef1",
		"2001:db8::44": "48506ba070f824cc4bbbcfbe7f91a0a735278adc416a14e0e0615afb4e8b63a1",
	} {
		if got := domain.IPHMAC(vectorKey(), netip.MustParseAddr(addr)); got != want {
			t.Errorf("IPHMAC(%s) = %s, want %s", addr, got, want)
		}
	}
}

func TestIPHMAC_MappedEqualsIPv4(t *testing.T) {
	t.Parallel()
	v4 := domain.IPHMAC(vectorKey(), netip.MustParseAddr("192.0.2.44"))
	if got := domain.IPHMAC(vectorKey(), netip.MustParseAddr("::ffff:192.0.2.44")); got != v4 {
		t.Fatalf("mapped address %s differs from IPv4 %s", got, v4)
	}
}

func TestIPHMAC_ZoneIgnored(t *testing.T) {
	t.Parallel()
	plain := domain.IPHMAC(vectorKey(), netip.MustParseAddr("fe80::1"))
	if got := domain.IPHMAC(vectorKey(), netip.MustParseAddr("fe80::1%eth0")); got != plain {
		t.Fatalf("zone changed the pseudonym: %s vs %s", got, plain)
	}
	other := make([]byte, 32)
	if domain.IPHMAC(other, netip.MustParseAddr("fe80::1")) == plain {
		t.Fatal("another key gave the same pseudonym")
	}
}

func TestKeyPeriod_AlignedToUnixTime(t *testing.T) {
	t.Parallel()
	period := 30 * 24 * time.Hour
	now := time.Date(2026, 10, 12, 8, 0, 0, 0, time.UTC)
	from, until := domain.KeyPeriod(now, period)
	if from.UnixMilli()%period.Milliseconds() != 0 || until.Sub(from) != period {
		t.Fatalf("period %v .. %v is not aligned to %v", from, until, period)
	}
	if now.Before(from) || !now.Before(until) {
		t.Fatalf("%v is not inside %v .. %v", now, from, until)
	}
	if f, _ := domain.KeyPeriod(until, period); !f.Equal(until) {
		t.Fatalf("the next period starts at %v, want %v", f, until)
	}
	if f, _ := domain.KeyPeriod(until.Add(-time.Millisecond), period); !f.Equal(from) {
		t.Fatalf("the last millisecond belongs to %v", f)
	}
}

func TestIPKey_SealOpen(t *testing.T) {
	t.Parallel()
	kek := bytes.Repeat([]byte{7}, 32)
	key := vectorKey()
	id := ids.New()
	sealed, err := domain.SealIPKey(kek, key, id, rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(sealed, key) || len(sealed) != 12+32+16 {
		t.Fatalf("sealed key has length %d or contains the key", len(sealed))
	}
	got, err := domain.OpenIPKey(kek, sealed, id)
	if err != nil || !bytes.Equal(got, key) {
		t.Fatalf("OpenIPKey = %x, %v", got, err)
	}
	if _, err := domain.OpenIPKey(bytes.Repeat([]byte{8}, 32), sealed, id); err == nil {
		t.Fatal("another KEK opened the key")
	}
	if _, err := domain.OpenIPKey(kek, sealed, ids.New()); err == nil {
		t.Fatal("the key opened under another row id (additional data)")
	}
	if _, err := domain.SealIPKey(kek[:16], key, id, rand.Reader); err == nil {
		t.Fatal("a 16-byte KEK was accepted")
	}
}

func TestDecodeKEK(t *testing.T) {
	t.Parallel()
	hexKey := strings.Repeat("0a", 32)
	for _, raw := range []string{hexKey, hexKey + "\n"} {
		k, err := domain.DecodeKEK([]byte(raw))
		if err != nil || len(k) != 32 || k[0] != 0x0a {
			t.Fatalf("DecodeKEK(%q) = %x, %v", raw, k, err)
		}
	}
	for _, bad := range []string{"", strings.Repeat("0a", 16), strings.Repeat("zz", 32), strings.Repeat("0a", 33)} {
		if _, err := domain.DecodeKEK([]byte(bad)); err == nil {
			t.Errorf("DecodeKEK(%q) accepted", bad)
		}
	}
}
