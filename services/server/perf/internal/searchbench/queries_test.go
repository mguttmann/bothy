// SPDX-License-Identifier: Apache-2.0

package searchbench_test

import (
	"math"
	"slices"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/OWNER/bothy/services/server/perf/internal/searchbench"
)

// within reports whether count of total lies within tol (absolute) of share.
func within(count, total int, share, tol float64) bool {
	return math.Abs(float64(count)/float64(total)-share) <= tol
}

// TestQueries_Mix: over ten thousand queries the class and band shares lie within two percentage
// points; phrases are adjacent words of a visible live message with a content word; negation is
// "a -b"; no query is function words only or holds a probe term; every query fits
// search.query_max_chars; the same seed gives the same sequence.
func TestQueries_Mix(t *testing.T) {
	t.Parallel()
	const n = 10000
	c := corpusOf(t, searchbench.DefaultParams(), 40000)
	pairs := visiblePairs(c, 3)
	gen, again := searchbench.NewQueries(c, 3, false), searchbench.NewQueries(c, 3, false)
	classes, bands := map[searchbench.Class]int{}, map[searchbench.Band]int{}
	banded := 0
	for range n {
		q := gen.Next()
		if q2 := again.Next(); q2 != q {
			t.Fatalf("the same seed gave %+v and %+v", q, q2)
		}
		classes[q.Class]++
		if q.Band != searchbench.BandNone {
			bands[q.Band]++
			banded++
		}
		if utf8.RuneCountInString(q.Text) > c.Plan.Defaults.QueryMaxChars {
			t.Fatalf("query %q is too long", q.Text)
		}
		if strings.Contains(q.Text, c.Vocab.Probe) || strings.Contains(q.Text, c.Vocab.Absent) {
			t.Fatalf("query %q holds a probe term", q.Text)
		}
		words := strings.Fields(strings.NewReplacer(`"`, "", "-", "").Replace(q.Text))
		if !slices.ContainsFunc(words, func(w string) bool { return !c.Vocab.IsFunction(q.Lang, w) }) {
			t.Fatalf("query %q is function words only", q.Text)
		}
		if q.Class == searchbench.ClassPhrase {
			checkPhrase(t, c, pairs, q)
		}
		if q.Class == searchbench.ClassNegation && (len(words) != 2 || q.Text != words[0]+" -"+words[1]) {
			t.Fatalf("negation %q is not a -b", q.Text)
		}
	}
	p := c.Plan
	for k, share := range map[searchbench.Class]float64{searchbench.ClassOneWord: p.ShareOneWord, searchbench.ClassTwoWords: p.ShareTwoWords,
		searchbench.ClassThreeWords: p.ShareThreeWords, searchbench.ClassPhrase: p.SharePhrase, searchbench.ClassNegation: p.ShareNegation} {
		if !within(classes[k], n, share, 0.02) {
			t.Errorf("class %s: %d of %d, want share %g", k, classes[k], n, share)
		}
	}
	for k, share := range map[searchbench.Band]float64{searchbench.BandFrequent: p.ShareFrequent, searchbench.BandMedium: p.ShareMedium, searchbench.BandRare: p.ShareRare} {
		if !within(bands[k], banded, share, 0.02) {
			t.Errorf("band %s: %d of %d, want share %g", k, bands[k], banded, share)
		}
	}
}

// visiblePairs returns every pair of adjacent words of the live messages searcher k may read.
func visiblePairs(c *searchbench.Corpus, k int) map[[2]string]bool {
	out := map[[2]string]bool{}
	for i := range c.Plan.Messages {
		meta := c.Meta(i)
		if meta.Deleted || !c.Plan.Visible(k, meta.Channel) {
			continue
		}
		words := searchbench.Words(c.Message(i).Text)
		for j := 0; j+1 < len(words); j++ {
			out[[2]string{words[j], words[j+1]}] = true
		}
	}
	return out
}

// checkPhrase fails t unless q is two adjacent words of a live message the searcher may read
// (pairs), at least one a content word.
func checkPhrase(t *testing.T, c *searchbench.Corpus, pairs map[[2]string]bool, q searchbench.Query) {
	t.Helper()
	inner, err := strconv.Unquote(q.Text)
	if err != nil {
		t.Fatalf("phrase %s is not quoted", q.Text)
	}
	pair := strings.Fields(inner)
	if len(pair) != 2 || (c.Vocab.IsFunction(q.Lang, pair[0]) && c.Vocab.IsFunction(q.Lang, pair[1])) {
		t.Fatalf("phrase %q", q.Text)
	}
	if !pairs[[2]string{pair[0], pair[1]}] {
		t.Fatalf("phrase %q is in no live message the searcher may read", q.Text)
	}
}

// TestPrefixQueries: every word has at least search.prefix_min_chars characters and is a prefix
// of a word of the title of a post the searcher may read; the one-word share lies within two
// percentage points.
func TestPrefixQueries(t *testing.T) {
	t.Parallel()
	const n = 5000
	c := corpusOf(t, searchbench.DefaultParams(), 200000)
	min := c.Plan.Defaults.PrefixMinChars
	var titles []string
	for post := range c.Plan.Posts {
		if c.Plan.Visible(5, c.Plan.PostChannel(post)) {
			titles = append(titles, strings.ToLower(c.Title(post)))
		}
	}
	gen := searchbench.NewQueries(c, 5, true)
	one := 0
	for range n {
		q, err := gen.NextPrefix()
		if err != nil {
			t.Fatal(err)
		}
		parts := strings.Fields(q.Text)
		if len(parts) == 1 {
			one++
		}
		found := slices.ContainsFunc(titles, func(title string) bool {
			for _, part := range parts {
				if utf8.RuneCountInString(part) < min || !slices.ContainsFunc(strings.Fields(title), func(w string) bool { return strings.HasPrefix(w, part) }) {
					return false
				}
			}
			return true
		})
		if !found || !q.MustFind {
			t.Fatalf("prefix query %q matches no visible title", q.Text)
		}
	}
	if !within(one, n, c.Plan.SharePrefixOneWord, 0.02) {
		t.Errorf("%d of %d prefix queries have one word, want share %g", one, n, c.Plan.SharePrefixOneWord)
	}
}
