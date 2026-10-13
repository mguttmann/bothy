// SPDX-License-Identifier: Apache-2.0

package domain_test

import (
	"bytes"
	"context"
	"crypto/rand"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/identity/domain"
)

// crockford is the alphabet of the enrollment codes.
const crockford = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"

// countingReader returns the bytes of src and counts what was read.
type countingReader struct {
	src []byte
	n   int
}

func (r *countingReader) Read(p []byte) (int, error) {
	k := copy(p, r.src[r.n:])
	r.n += k
	return k, nil
}

func TestEnrollmentCodeFormat(t *testing.T) {
	t.Parallel()
	code, err := domain.NewEnrollmentCode(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	if len(code) != 32 {
		t.Fatalf("canonical code has %d characters, want 32", len(code))
	}
	for _, c := range code {
		if !strings.ContainsRune(crockford, c) {
			t.Fatalf("character %q outside the Crockford alphabet", c)
		}
	}
	shown := domain.DisplayEnrollmentCode(code)
	if !strings.HasPrefix(shown, "en_") {
		t.Fatalf("display form %q lacks the prefix", shown)
	}
	groups := strings.Split(strings.TrimPrefix(shown, "en_"), "-")
	if len(groups) != 8 {
		t.Fatalf("display form has %d groups, want 8", len(groups))
	}
	for _, g := range groups {
		if len(g) != 4 {
			t.Fatalf("group %q has not four characters", g)
		}
	}
	if strings.Join(groups, "") != code {
		t.Fatal("display form does not carry the canonical code")
	}
	// 160 bits: all-ones input encodes to all Z, all-zero input to all 0.
	ones, _ := domain.NewEnrollmentCode(bytes.NewReader(bytes.Repeat([]byte{0xff}, 20)))
	zeros, _ := domain.NewEnrollmentCode(bytes.NewReader(make([]byte, 20)))
	if ones != strings.Repeat("Z", 32) || zeros != strings.Repeat("0", 32) {
		t.Fatalf("encoding of the extremes: %q %q", ones, zeros)
	}
	if _, err := domain.NewEnrollmentCode(bytes.NewReader(make([]byte, 19))); err == nil {
		t.Fatal("a short random source must fail")
	}
}

func TestNormalizeEnrollmentCode(t *testing.T) {
	t.Parallel()
	canonical := "0123456789ABCDEFGHJKMNPQRSTVWXYZ"
	shown := domain.DisplayEnrollmentCode(canonical)
	cases := []struct {
		name, in string
		ok       bool
		want     string
	}{
		{"display form", shown, true, canonical},
		{"lowercase", strings.ToLower(shown), true, canonical},
		{"prefix in capitals", "EN_" + strings.TrimPrefix(shown, "en_"), true, canonical},
		{"missing prefix", strings.TrimPrefix(shown, "en_"), true, canonical},
		{"spaces and no hyphens", " en_0123 4567 89AB CDEF GHJK MNPQ RSTV WXYZ ", true, canonical},
		{"O I L mapping", "en_O123-4567-89AB-CDEF-GHJK-MNPQ-RSTV-WXYZ", true, canonical},
		{"I and L map to 1", "en_0I23-4567-89AB-CDEF-GHJK-MNPQ-RSTV-WXYZ", true, canonical},
		{"l lowercase", "en_0l23-4567-89AB-CDEF-GHJK-MNPQ-RSTV-WXYZ", true, canonical},
		{"wrong length", "en_0123-4567", false, ""},
		{"too long", shown + "0", false, ""},
		{"character U", "en_U123-4567-89AB-CDEF-GHJK-MNPQ-RSTV-WXYZ", false, ""},
		{"empty", "", false, ""},
		{"only prefix", "en_", false, ""},
		{"non ASCII", "en_0123-4567-89AB-CDEF-GHJK-MNPQ-RSTV-WXYÄ", false, ""},
	}
	for _, c := range cases {
		got, ok := domain.NormalizeEnrollmentCode(c.in)
		if ok != c.ok || got != c.want {
			t.Errorf("%s: got %q %v, want %q %v", c.name, got, ok, c.want, c.ok)
		}
	}
}

func TestIDT27_EnrollmentCodeEntropyAndSource(t *testing.T) {
	t.Parallel()
	r := &countingReader{src: bytes.Repeat([]byte{7}, 64)}
	if _, err := domain.NewEnrollmentCode(r); err != nil {
		t.Fatal(err)
	}
	if r.n != 20 {
		t.Fatalf("the generator read %d bytes, want 20 (160 bits)", r.n)
	}
	if domain.EnrollmentRandom != rand.Reader {
		t.Fatal("the production random source is not crypto/rand.Reader")
	}
	// go list -deps of these packages always contains math/rand through the standard library
	// (math/big, crypto/ecdsa, database/sql) and the drivers, so the rule is held on the imports of
	// every package under internal/identity, tests included (docs/DECISIONS.md M1-37).
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	out, err := exec.CommandContext(ctx, "go", "list", "-test", "-f", "{{.ImportPath}}: {{join .Imports \" \"}}",
		"github.com/OWNER/bothy/services/server/internal/identity/...").CombinedOutput()
	if err != nil {
		t.Fatalf("go list: %v: %s", err, out)
	}
	pkgs := 0
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		pkgs++
		for _, imp := range strings.Fields(line)[1:] {
			if imp == "math/rand" || imp == "math/rand/v2" {
				t.Fatalf("%s imports %s", strings.Fields(line)[0], imp)
			}
		}
	}
	if pkgs < 10 {
		t.Fatalf("go list named only %d packages", pkgs)
	}
}

func TestEnrollmentCodeHash(t *testing.T) {
	t.Parallel()
	pepper := []byte("0123456789abcdef0123456789abcdef")
	a := domain.EnrollmentCodeHash(pepper, "0123456789ABCDEFGHJKMNPQRSTVWXYZ")
	if len(a) != 32 {
		t.Fatalf("hash has %d bytes", len(a))
	}
	if bytes.Equal(a, domain.EnrollmentCodeHash([]byte("other pepper of thirty-two bytes"), "0123456789ABCDEFGHJKMNPQRSTVWXYZ")) {
		t.Fatal("the hash does not depend on the pepper")
	}
	if bytes.Equal(a, domain.EnrollmentCodeHash(pepper, "1123456789ABCDEFGHJKMNPQRSTVWXYZ")) {
		t.Fatal("the hash does not depend on the code")
	}
}

func TestEnrollmentPurposeRules(t *testing.T) {
	t.Parallel()
	issue := []struct {
		local, device bool
		want          domain.EnrollmentPurpose
	}{
		{true, false, domain.PurposeFirstLogin},
		{true, true, domain.PurposeRecovery},
		{false, false, domain.PurposeIdentityLink},
		{false, true, domain.PurposeIdentityLink},
	}
	for _, c := range issue {
		if got := domain.IssuePurpose(c.local, c.device); got != c.want {
			t.Errorf("IssuePurpose(%v, %v) = %s, want %s", c.local, c.device, got, c.want)
		}
	}
	consume := map[domain.EnrollmentConsumer][]domain.EnrollmentPurpose{
		domain.ConsumerRedeem:       {domain.PurposeFirstLogin, domain.PurposeRecovery},
		domain.ConsumerPasskey:      {domain.PurposeFirstLogin, domain.PurposeRecovery},
		domain.ConsumerIdentityLink: {domain.PurposeFirstLogin, domain.PurposeRecovery, domain.PurposeIdentityLink},
	}
	all := []domain.EnrollmentPurpose{domain.PurposeFirstLogin, domain.PurposeRecovery, domain.PurposeIdentityLink, "other"}
	for consumer, allowed := range consume {
		for _, p := range all {
			want := false
			for _, a := range allowed {
				want = want || a == p
			}
			if got := domain.PurposeConsumableBy(p, consumer); got != want {
				t.Errorf("PurposeConsumableBy(%s, %s) = %v, want %v", p, consumer, got, want)
			}
		}
		if got := domain.ConsumablePurposes(consumer); len(got) != len(allowed) {
			t.Errorf("ConsumablePurposes(%s) = %v, want %v", consumer, got, allowed)
		}
	}
	for _, p := range all[:3] {
		if !p.Valid() {
			t.Errorf("%s not valid", p)
		}
	}
	if domain.EnrollmentPurpose("other").Valid() {
		t.Error("unknown purpose valid")
	}
}

func TestEnrollmentCodeUsable(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 10, 13, 9, 0, 0, 0, time.UTC)
	cases := []struct {
		name   string
		st     domain.EnrollmentCodeState
		usable bool
	}{
		{"fresh", domain.EnrollmentCodeState{ExpiresAt: now.Add(time.Minute)}, true},
		{"used", domain.EnrollmentCodeState{ExpiresAt: now.Add(time.Minute), UsedAt: now.Add(-time.Second)}, false},
		{"expires now", domain.EnrollmentCodeState{ExpiresAt: now}, false},
		{"expired", domain.EnrollmentCodeState{ExpiresAt: now.Add(-time.Second)}, false},
		{"one attempt left", domain.EnrollmentCodeState{ExpiresAt: now.Add(time.Minute), Attempts: 4}, true},
		{"exhausted", domain.EnrollmentCodeState{ExpiresAt: now.Add(time.Minute), Attempts: 5}, false},
	}
	for _, c := range cases {
		if got := c.st.Usable(now, 5); got != c.usable {
			t.Errorf("%s: usable %v, want %v", c.name, got, c.usable)
		}
	}
}
