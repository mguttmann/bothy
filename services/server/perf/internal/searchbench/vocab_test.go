// SPDX-License-Identifier: Apache-2.0

package searchbench_test

import (
	"math"
	"math/rand/v2"
	"slices"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/OWNER/bothy/services/server/perf/internal/searchbench"
)

func TestVocabulary_Deterministic(t *testing.T) {
	t.Parallel()
	a, err := searchbench.NewVocabulary(searchbench.Seed, 5000)
	if err != nil {
		t.Fatal(err)
	}
	b, err := searchbench.NewVocabulary(searchbench.Seed, 5000)
	if err != nil {
		t.Fatal(err)
	}
	other, err := searchbench.NewVocabulary(searchbench.Seed+1, 5000)
	if err != nil {
		t.Fatal(err)
	}
	for _, l := range searchbench.Langs {
		la, lb := a.Langs[l], b.Langs[l]
		if !slices.Equal(la.Function, lb.Function) || !slices.Equal(la.Stems, lb.Stems) || !slices.Equal(la.Inflections, lb.Inflections) {
			t.Errorf("%s: the same seed gave another vocabulary", l)
		}
		if slices.Equal(la.Stems, other.Langs[l].Stems) {
			t.Errorf("%s: another seed gave the same stems", l)
		}
	}
	if a.Probe != b.Probe || a.Absent != b.Absent || a.Probe == a.Absent {
		t.Errorf("probe terms %q/%q and %q/%q", a.Probe, a.Absent, b.Probe, b.Absent)
	}
}

func TestVocabulary_NoCollisions(t *testing.T) {
	t.Parallel()
	v, err := searchbench.NewVocabulary(searchbench.Seed, searchbench.ContentWords)
	if err != nil {
		t.Fatal(err)
	}
	owner := map[string]string{}
	for _, l := range searchbench.Langs {
		for _, f := range v.Langs[l].Function {
			owner[f] = "function word"
		}
	}
	umlauts := 0
	for _, l := range searchbench.Langs {
		lv := v.Langs[l]
		if len(lv.Stems) != searchbench.ContentWords {
			t.Fatalf("%s: %d stems", l, len(lv.Stems))
		}
		for rank, stem := range lv.Stems {
			if n := utf8.RuneCountInString(stem); n < 4 || n > 10 {
				t.Errorf("%s stem %q has %d letters", l, stem, n)
			}
			// language-check: off
			if l == searchbench.German && strings.ContainsAny(stem, "\u00e4\u00f6\u00fc\u00df") {
				// language-check: on
				umlauts++
			}
			for k := range lv.Inflections {
				form := lv.Form(rank, k)
				if o, ok := owner[form]; ok && o != stem {
					t.Fatalf("%s form %q of stem %q equals a form of %q", l, form, stem, o)
				}
				owner[form] = stem
			}
		}
	}
	if umlauts == 0 {
		t.Error("no German stem contains a, o, u with diaeresis or the sharp s")
	}
	for _, term := range []string{v.Probe, v.Absent} {
		if _, ok := owner[term]; ok || v.IsForm(term) {
			t.Errorf("probe term %q equals a form", term)
		}
	}
}

// zipfShare is the expected share of rank 1 of a Zipf distribution with exponent s over n ranks
// (weights (1 + k)^-s for k = 0 .. n-1).
func zipfShare(s float64, n int) float64 {
	sum := 0.0
	for k := range n {
		sum += math.Pow(float64(k+1), -s)
	}
	return 1 / sum
}

// TestZipf_Shape: over a large sample of the content words of the corpus the mean count per rank
// falls from rank 1 to ranks 2 to 10, 11 to 100 and 101 to 1000, and the share of rank 1 lies
// within 10 percent (relative) of the value ZipfS gives.
func TestZipf_Shape(t *testing.T) {
	t.Parallel()
	const draws = 400000
	r := rand.New(rand.NewPCG(1, 2))
	z := rand.NewZipf(r, searchbench.ZipfS, 1, searchbench.ContentWords-1)
	counts := make([]int, searchbench.ContentWords)
	for range draws {
		counts[z.Uint64()]++
	}
	mean := func(lo, hi int) float64 {
		s := 0
		for k := lo; k <= hi; k++ {
			s += counts[k]
		}
		return float64(s) / float64(hi-lo+1)
	}
	groups := []float64{mean(0, 0), mean(1, 9), mean(10, 99), mean(100, 999)}
	for i := 1; i < len(groups); i++ {
		if groups[i] >= groups[i-1] {
			t.Errorf("mean count per rank does not fall: %v", groups)
		}
	}
	want := zipfShare(searchbench.ZipfS, searchbench.ContentWords)
	if got := float64(counts[0]) / draws; math.Abs(got-want) > 0.1*want {
		t.Errorf("share of rank 1 = %.4f, want %.4f within 10 percent", got, want)
	}
}
