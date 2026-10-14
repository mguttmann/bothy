// SPDX-License-Identifier: Apache-2.0

package fracindex_test

import (
	"math/rand/v2"
	"strings"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/chat/domain/fracindex"
)

// TestFracindexVectors holds the normative vectors of the brief M1-52 and the compatible base-62
// scheme with integer part.
func TestFracindexVectors(t *testing.T) {
	t.Parallel()
	for _, c := range []struct{ a, b, want string }{
		{"", "", "a0"},
		{"a0", "", "a1"},
		{"", "a0", "Zz"},
		{"a0", "a1", "a0V"},
		{"a1", "", "a2"},
		{"az", "", "b00"},
		{"Zz", "a0", "Zz" + "V"},
		{"a0V", "a1", "a0l"},
		{"a0", "a0V", "a0G"},
		{"a0", "a0G", "a08"},
		{"b00", "", "b01"},
		{"", "Zz", "Zy"},
		{"", "a0V", "a0"},
		{"", "A" + strings.Repeat("0", 26) + "1", "A" + strings.Repeat("0", 26) + "0V"},
		{"A" + strings.Repeat("0", 26) + "1", "A" + strings.Repeat("0", 25) + "1", "A" + strings.Repeat("0", 26) + "W"},
		{"zzzzzzzzzzzzzzzzzzzzzzzzzzz", "", "zzzzzzzzzzzzzzzzzzzzzzzzzzzV"},
	} {
		got, err := fracindex.KeyBetween(c.a, c.b)
		if err != nil || got != c.want {
			t.Errorf("KeyBetween(%q, %q) = %q, %v; want %q", c.a, c.b, got, err, c.want)
		}
	}
}

// TestFracindexErrors: a not below b, and invalid keys on either side, are refused.
func TestFracindexErrors(t *testing.T) {
	t.Parallel()
	for _, c := range []struct{ a, b string }{
		{"a1", "a0"}, {"a0", "a0"}, {"a00", ""}, {"", "a10"}, {"!", ""}, {"a", ""},
		{"A" + strings.Repeat("0", 26), ""}, {"a0", "b"},
	} {
		if got, err := fracindex.KeyBetween(c.a, c.b); err == nil {
			t.Errorf("KeyBetween(%q, %q) = %q, want an error", c.a, c.b, got)
		}
	}
}

// TestFracindexValid: the key grammar.
func TestFracindexValid(t *testing.T) {
	t.Parallel()
	for _, k := range []string{"a0", "a0V", "Zz", "b00", "zzzzzzzzzzzzzzzzzzzzzzzzzzz", "A" + strings.Repeat("0", 25) + "1", "a0" + "1",
		"A" + strings.Repeat("0", 26) + "1"} {
		if !fracindex.Valid(k) {
			t.Errorf("Valid(%q) = false", k)
		}
	}
	for _, k := range []string{"", "a", "a00", "a0V0", "b0", "Z", "A" + strings.Repeat("0", 26), "a-", "aä0", "0", "a0 "} {
		if fracindex.Valid(k) {
			t.Errorf("Valid(%q) = true", k)
		}
	}
}

// TestFracindexProperties: for random valid pairs the result lies strictly between and is
// valid; repeated inserts at the end and the start keep short keys.
func TestFracindexProperties(t *testing.T) {
	t.Parallel()
	r := rand.New(rand.NewPCG(52, 1)) //nolint:gosec // G404: reproducible test data from a fixed seed, not security
	keys := []string{"a0"}
	for range 2000 {
		i := r.IntN(len(keys) + 1)
		a, b := "", ""
		if i > 0 {
			a = keys[i-1]
		}
		if i < len(keys) {
			b = keys[i]
		}
		k, err := fracindex.KeyBetween(a, b)
		if err != nil {
			t.Fatalf("KeyBetween(%q, %q): %v", a, b, err)
		}
		if !fracindex.Valid(k) || (a != "" && k <= a) || (b != "" && k >= b) {
			t.Fatalf("KeyBetween(%q, %q) = %q is not strictly between or invalid", a, b, k)
		}
		keys = append(keys[:i], append([]string{k}, keys[i:]...)...)
	}
	last, first := "", ""
	for range 10000 {
		var err error
		if last, err = fracindex.KeyBetween(last, ""); err != nil {
			t.Fatal(err)
		}
		if first, err = fracindex.KeyBetween("", first); err != nil {
			t.Fatal(err)
		}
	}
	if len(last) > 4 || len(first) > 4 {
		t.Fatalf("10000 inserts at the ends gave %q and %q", last, first)
	}
}
