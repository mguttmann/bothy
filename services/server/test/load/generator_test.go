// SPDX-License-Identifier: Apache-2.0

package load

import (
	"regexp"
	"strings"
	"testing"
	"unicode/utf8"
)

// handlePattern is the pattern of UserCreate.handle in the contract.
var handlePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]*$`)

func TestGenerator_PeopleAreWhiteLabelAndUnique(t *testing.T) {
	t.Parallel()
	g := NewGenerator(Seed)
	if p := g.Person(0); p.Handle != "sam" || p.DisplayName != "Sam" || p.Email != "sam@example.com" {
		t.Errorf("person 0 = %+v, want Sam", p)
	}
	if p := g.Person(1); p.Handle != "robin" || p.DisplayName != "Robin" || p.Email != "robin@example.com" {
		t.Errorf("person 1 = %+v, want Robin", p)
	}
	if p := g.Person(2); p.Handle != "person-1" || p.Email != "person-1@example.com" {
		t.Errorf("person 2 = %+v, want person-1", p)
	}
	seen := map[string]bool{"alex": true} // the owner of the installation
	for i := range 5000 {
		p := g.Person(i)
		if !handlePattern.MatchString(p.Handle) {
			t.Fatalf("person %d: handle %q breaks the contract pattern", i, p.Handle)
		}
		if seen[p.Handle] {
			t.Fatalf("person %d: handle %q twice", i, p.Handle)
		}
		seen[p.Handle] = true
		if !strings.HasSuffix(p.Email, "@example.com") || p.DisplayName == "" {
			t.Fatalf("person %d = %+v: not white label", i, p)
		}
	}
}

func TestGenerator_ContentIsSeededAndBounded(t *testing.T) {
	t.Parallel()
	a, b := NewGenerator(Seed), NewGenerator(Seed)
	other := NewGenerator(Seed + 1)
	same, differs := 0, 0
	for step := range 3 {
		for i := range 2000 {
			x := a.Content(step, i)
			if x != b.Content(step, i) {
				t.Fatalf("content (%d, %d) differs between two generators of one seed", step, i)
			}
			if n := utf8.RuneCountInString(x); n < MinContentRunes || n > MaxContentRunes {
				t.Fatalf("content (%d, %d) has %d runes, want %d to %d", step, i, n, MinContentRunes, MaxContentRunes)
			}
			if strings.TrimSpace(x) != x || x == "" {
				t.Fatalf("content (%d, %d) = %q: empty or padded", step, i, x)
			}
			if x == other.Content(step, i) {
				same++
			} else {
				differs++
			}
		}
	}
	if differs == 0 || same > differs {
		t.Errorf("another seed gave %d equal and %d different texts; the seed must matter", same, differs)
	}
}

func TestGenerator_ContentLengthsSpreadOverTheRange(t *testing.T) {
	t.Parallel()
	g := NewGenerator(Seed)
	short, long := 0, 0
	for i := range 5000 {
		n := utf8.RuneCountInString(g.Content(0, i))
		if n < (MinContentRunes+MaxContentRunes)/4 {
			short++
		}
		if n > (MinContentRunes+MaxContentRunes)*3/4 {
			long++
		}
	}
	if short == 0 || long == 0 {
		t.Errorf("lengths do not spread: %d short, %d long of 5000", short, long)
	}
}
