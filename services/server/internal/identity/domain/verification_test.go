// SPDX-License-Identifier: Apache-2.0

package domain_test

import (
	"testing"

	"github.com/OWNER/bothy/services/server/internal/identity/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// The vector of the verification code (brief M1-38 "Pending devices and the verification code");
// the expected values were computed independently with Python's hmac module.
var (
	vectorPepper = []byte("0123456789abcdef0123456789abcdef")
	vectorDevice = ids.MustParse("0192f4a0-7b1c-7848-8501-1716705c7dbe")
	vectorJKT    = "0ZcOCORZNYy-DWpqq30jZyJGHTN0d2HglBV3uiguA4I"
)

func TestDeriveVerificationCode(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name   string
		device ids.ID
		jkt    string
		digits int
		want   string
	}{
		// The vector keeps its leading zero.
		{"six digits", vectorDevice, vectorJKT, 6, "051265"},
		{"eight digits", vectorDevice, vectorJKT, 8, "05051265"},
		{"other key", vectorDevice, "other", 6, "742700"},
		{"other device", ids.MustParse("0192f4a0-7b1c-7848-8501-1716705c7dbf"), vectorJKT, 6, "734554"},
		{"leading zero", ids.MustParse("0192f4a0-7b1c-7848-8501-000000000000"), vectorJKT, 6, "038446"},
	}
	for _, c := range cases {
		got := domain.DeriveVerificationCode(vectorPepper, c.device, c.jkt, c.digits)
		if got != c.want || len(got) != c.digits {
			t.Errorf("%s: code %q, want %q", c.name, got, c.want)
		}
	}
}

func TestVerificationCodeEqual(t *testing.T) {
	t.Parallel()
	cases := []struct {
		expected, typed string
		want            bool
	}{
		{"051265", "051265", true},
		{"051265", "051 265", true},
		{"051265", " 0 5 1 2 6 5 ", true},
		{"051265", "051266", false},
		{"051265", "51265", false},
		{"051265", "0512650", false},
		{"051265", "", false},
		{"051265", "05126\t5", false},
	}
	for _, c := range cases {
		if got := domain.VerificationCodeEqual(c.expected, c.typed); got != c.want {
			t.Errorf("VerificationCodeEqual(%q, %q) = %v, want %v", c.expected, c.typed, got, c.want)
		}
	}
}
