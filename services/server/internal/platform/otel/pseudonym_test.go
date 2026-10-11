// SPDX-License-Identifier: Apache-2.0

package otel

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strconv"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/platform/otel/attr"
	"github.com/OWNER/bothy/services/server/internal/platform/secrets"
)

// fixedKeys is a KeySource with one key for every reference.
type fixedKeys []byte

func (k fixedKeys) Resolve(context.Context, string) (secrets.Value, secrets.Version, error) {
	return secrets.NewValue(k), "v1", nil
}

const principal = "0b6f2a4e-3c1d-4e5f-8a9b-0c1d2e3f4a5b"

// wantPseudonym computes privacy.md "Pseudonyme" item 1 step by step.
func wantPseudonym(master []byte, period int64, id string) string {
	m := hmac.New(sha256.New, master)
	m.Write([]byte("bothy/telemetry-pseudonym/v1/" + strconv.FormatInt(period, 10)))
	kp := m.Sum(nil)
	m = hmac.New(sha256.New, kp)
	m.Write([]byte(id))
	return hex.EncodeToString(m.Sum(nil))[:32]
}

func newTestPseudonymizer(t *testing.T, key []byte, rotation time.Duration) *Pseudonymizer {
	t.Helper()
	p, err := NewPseudonymizer(context.Background(), fixedKeys(key), "env:"+testKeyVar, rotation, nil)
	if err != nil {
		t.Fatalf("NewPseudonymizer: %v", err)
	}
	return p
}

func TestPseudonymStableAndRotating(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	const rotation = 30 * 24 * time.Hour
	at := time.Date(2026, 10, 11, 12, 0, 0, 0, time.UTC)
	period := at.Unix() / int64(rotation/time.Second)
	p := newTestPseudonymizer(t, testKey, rotation)

	got, start, err := p.PseudonymWithPeriod(ctx, principal, at)
	if err != nil {
		t.Fatal(err)
	}
	if want := wantPseudonym(testKey, period, principal); got != want || len(got) != 32 {
		t.Errorf("pseudonym = %q, want %q", got, want)
	}
	if want := time.Unix(period*int64(rotation/time.Second), 0).UTC(); !start.Equal(want) || start.Location() != time.UTC {
		t.Errorf("period start = %v, want %v", start, want)
	}
	same, _ := p.Pseudonym(ctx, principal, start.Add(rotation-time.Second))
	upper, _ := p.Pseudonym(ctx, "0B6F2A4E-3C1D-4E5F-8A9B-0C1D2E3F4A5B", at)
	if same != got || upper != got {
		t.Errorf("same period %q, upper case id %q; want %q", same, upper, got)
	}
	next, _ := p.Pseudonym(ctx, principal, start.Add(rotation))
	other, _ := newTestPseudonymizer(t, []byte("fedcba9876543210fedcba9876543210"), rotation).Pseudonym(ctx, principal, at)
	if next == got || other == got {
		t.Errorf("next period %q, other key %q; both must differ from %q", next, other, got)
	}

	if _, err := NewPseudonymizer(ctx, fixedKeys(testKey[:31]), "env:K", rotation, nil); !errors.Is(err, ErrKeyTooShort) {
		t.Errorf("31-byte key = %v, want ErrKeyTooShort", err)
	}
	if _, err := NewPseudonymizer(ctx, fixedKeys(testKey), "env:K", 500*time.Millisecond, nil); !errors.Is(err, ErrRotation) {
		t.Errorf("rotation 500ms = %v, want ErrRotation", err)
	}
	for _, bad := range []string{"", "not-a-uuid", "0b6f2a4e3c1d4e5f8a9b0c1d2e3f4a5b", "0b6f2a4e-3c1d-4e5f-8a9b-0c1d2e3f4a5g"} {
		if _, err := p.Pseudonym(ctx, bad, at); !errors.Is(err, ErrPrincipalID) {
			t.Errorf("Pseudonym(%q) = %v, want ErrPrincipalID", bad, err)
		}
	}
}

// TestPseudonymPeriodBeforeEpoch: periods round toward negative infinity.
func TestPseudonymPeriodBeforeEpoch(t *testing.T) {
	t.Parallel()
	p := newTestPseudonymizer(t, testKey, 10*time.Second)
	got, start, err := p.PseudonymWithPeriod(context.Background(), principal, time.Unix(-1, 0))
	if err != nil {
		t.Fatal(err)
	}
	if start.Unix() != -10 || got != wantPseudonym(testKey, -1, principal) {
		t.Errorf("at -1s: start %d, pseudonym %q; want start -10 and period -1", start.Unix(), got)
	}
}

func TestPseudonymAttrUsesClock(t *testing.T) {
	t.Parallel()
	at := time.Date(2026, 10, 11, 12, 0, 0, 0, time.UTC)
	p, err := NewPseudonymizer(context.Background(), fixedKeys(testKey), "env:K", time.Hour, clock.NewFake(at))
	if err != nil {
		t.Fatal(err)
	}
	kv, err := p.Attr(context.Background(), principal)
	if err != nil {
		t.Fatal(err)
	}
	want, _ := p.Pseudonym(context.Background(), principal, at)
	if string(kv.Key) != attr.PrincipalPseudoID || kv.Value.AsString() != want {
		t.Errorf("Attr = %v, want %s=%s", kv, attr.PrincipalPseudoID, want)
	}
}
