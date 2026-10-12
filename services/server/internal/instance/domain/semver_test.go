// SPDX-License-Identifier: Apache-2.0

package domain_test

import (
	"testing"

	"github.com/OWNER/bothy/services/server/internal/instance/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
)

// TestCompareSemVer follows SemVer 2.0.0 section 11 (https://semver.org/spec/v2.0.0.html):
// numeric core first, a pre-release sorts before its release, identifiers compare numerically or
// in ASCII order, a longer set wins on a common prefix, build metadata does not count.
func TestCompareSemVer(t *testing.T) {
	t.Parallel()
	// The chain of section 11 item 4, each element smaller than the next.
	chain := []string{
		"1.0.0-alpha", "1.0.0-alpha.1", "1.0.0-alpha.beta", "1.0.0-beta", "1.0.0-beta.2",
		"1.0.0-beta.11", "1.0.0-rc.1", "1.0.0", "2.0.0", "2.1.0", "2.1.1", "10.0.0",
	}
	for i, a := range chain {
		for j, b := range chain {
			want := 0
			switch {
			case i < j:
				want = -1
			case i > j:
				want = 1
			}
			got, err := domain.CompareSemVer(a, b)
			if err != nil || got != want {
				t.Errorf("CompareSemVer(%q, %q) = %d, %v; want %d", a, b, got, err, want)
			}
		}
	}
	equal := [][2]string{{"1.0.0+build.1", "1.0.0+build.2"}, {"1.0.0-rc.1+x", "1.0.0-rc.1"}, {"0.0.0", "0.0.0"}}
	for _, p := range equal {
		if got, err := domain.CompareSemVer(p[0], p[1]); err != nil || got != 0 {
			t.Errorf("CompareSemVer(%q, %q) = %d, %v; want 0", p[0], p[1], got, err)
		}
	}
	// Numeric identifiers sort before alphanumeric ones; numbers compare as numbers, also
	// beyond 64 bits.
	less := [][2]string{
		{"1.0.0-1", "1.0.0-a"}, {"1.0.0-9", "1.0.0-10"}, {"1.0.0-a", "1.0.0-a-b"},
		// 23 and 24 digits: beyond 64 bits (and beyond the 17 to 20 digit rule of the deny list).
		{"1.0.0-99999999999999999999999", "1.0.0-100000000000000000000000"},
		{"1.0.0-99999999999999999999998", "1.0.0-99999999999999999999999"},
		{"99999999999999999999998.0.0", "99999999999999999999999.0.0"},
		{"99999999999999999999999.0.0", "100000000000000000000000.0.0"},
	}
	for _, p := range less {
		if got, err := domain.CompareSemVer(p[0], p[1]); err != nil || got != -1 {
			t.Errorf("CompareSemVer(%q, %q) = %d, %v; want -1", p[0], p[1], got, err)
		}
	}
	invalid := []string{
		"", "1", "1.0", "1.0.0.0", "v1.0.0", "01.0.0", "1.00.0", "1.0.00", "1.0.0-", "1.0.0-01",
		"1.0.0-a..b", "1.0.0+", "1.0.0+a..b", "1.0.0-a_b", " 1.0.0", "1.0.0 ", "-1.0.0", "1.0.0-é",
		"a.b.c",
	}
	for _, s := range invalid {
		if _, err := domain.CompareSemVer(s, "1.0.0"); err == nil {
			t.Errorf("CompareSemVer(%q, 1.0.0) accepted an invalid version", s)
		}
		if _, err := domain.CompareSemVer("1.0.0", s); err == nil {
			t.Errorf("CompareSemVer(1.0.0, %q) accepted an invalid version", s)
		}
	}
	valid := []string{"1.0.0-0A.is.legal", "1.0.0-x-y-z.--", "1.0.0+21AF26D3----117B344092BD", "0.0.0-0"}
	for _, s := range valid {
		if _, err := domain.CompareSemVer(s, s); err != nil {
			t.Errorf("CompareSemVer(%q) refused a valid version: %v", s, err)
		}
	}
}

func TestCheckServerVersion(t *testing.T) {
	t.Parallel()
	cases := []struct {
		own, min string
		ok       bool
	}{
		{"1.2.3", "", true},       // no lock (a schema of a binary before M1-15, rollback path)
		{"1.2.3", "1.2.3", true},  // equal
		{"1.2.3", "1.2.2", true},  // older minimum
		{"1.2.3", "1.2.4", false}, // newer minimum
		{"1.2.3-rc.1", "1.2.3", false},
		{"1.2.3", "not-semver", false}, // a broken minimum fails closed
		{"", "1.0.0", false},
	}
	for _, c := range cases {
		err := domain.CheckServerVersion(c.own, c.min)
		if (err == nil) != c.ok {
			t.Errorf("CheckServerVersion(%q, %q) = %v, want ok %v", c.own, c.min, err, c.ok)
		}
		if err != nil && errs.KindOf(err) != errs.KindUnavailable {
			t.Errorf("CheckServerVersion(%q, %q) kind = %s, want unavailable", c.own, c.min, errs.KindOf(err))
		}
	}
}

func TestValidateSemVer(t *testing.T) {
	t.Parallel()
	for _, s := range []string{"0.0.0", "1.2.3-rc.1+b"} {
		if err := domain.ValidateSemVer(s); err != nil {
			t.Errorf("ValidateSemVer(%q) = %v", s, err)
		}
	}
	for _, s := range []string{"", "v1.0.0", "1.0"} {
		if err := domain.ValidateSemVer(s); err == nil {
			t.Errorf("ValidateSemVer(%q) accepted", s)
		}
	}
}
