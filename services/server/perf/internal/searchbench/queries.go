// SPDX-License-Identifier: Apache-2.0

package searchbench

import (
	"fmt"
	"math/rand/v2"
	"strings"
	"unicode/utf8"
)

// Class is the class of a query of phase messages (design note 8), or the phase name of a prefix
// or probe query.
type Class string

// The classes of the queries.
const (
	ClassOneWord    Class = "one_word"
	ClassTwoWords   Class = "two_words"
	ClassThreeWords Class = "three_words"
	ClassPhrase     Class = "phrase"
	ClassNegation   Class = "negation"
	ClassPrefix     Class = "prefix"
	ClassProbe      Class = "probe_term"
	ClassAbsent     Class = "absent_term"
)

// Band is the frequency band of the words of a query (design note 8); a phrase, prefix or probe
// query has none.
type Band string

// The frequency bands.
const (
	BandNone     Band = ""
	BandFrequent Band = "frequent"
	BandMedium   Band = "medium"
	BandRare     Band = "rare"
)

// Classes are the classes of phase messages in the order of their shares; Bands the bands.
var (
	Classes = []Class{ClassOneWord, ClassTwoWords, ClassThreeWords, ClassPhrase, ClassNegation}
	Bands   = []Band{BandFrequent, BandMedium, BandRare}
)

// Query is one request of a searcher: Text is the parameter q. MustFind marks a query built from
// a visible message or title, which must find something (design note 10).
type Query struct {
	Searcher int
	Class    Class
	Band     Band
	Lang     Lang
	Text     string
	MustFind bool
}

// The PCG streams of the query generators, apart from the streams of the corpus.
const (
	queryStream  = 1 << 50
	prefixStream = 2 << 50
)

// Queries generates the queries of one searcher; each phase has its own generator. It is not
// safe for concurrent use.
type Queries struct {
	c        *Corpus
	searcher int
	r        *rand.Rand
	visible  []int // channels the searcher may read
	forums   []int // forum channels among them
}

// NewQueries returns the query generator of searcher k for phase messages (prefix false) or
// prefix (prefix true), seeded from (Seed, stream and searcher).
func NewQueries(c *Corpus, k int, prefix bool) *Queries {
	stream := uint64(queryStream)
	if prefix {
		stream = prefixStream
	}
	q := &Queries{c: c, searcher: k, r: rand.New(rand.NewPCG(c.Plan.Seed, stream+uint64(k)))} // #nosec G404 G115 -- reproducible queries; k is a non-negative searcher number
	for ch := range c.Plan.Channels {
		if c.Plan.Visible(k, ch) {
			q.visible = append(q.visible, ch)
			if c.Plan.ChannelKind(ch) == ForumChannel {
				q.forums = append(q.forums, ch)
			}
		}
	}
	return q
}

// pick returns the index of the share in shares that u (0 <= u < 1) falls into.
func pick(u float64, shares ...float64) int {
	for i, s := range shares {
		if u < s {
			return i
		}
		u -= s
	}
	return len(shares) - 1
}

// Next returns the next query of phase messages (design note 8): the language German or English
// with equal chance, the class by the shares of the constants; the words of the word classes and
// the negation are content stems of one band chosen by share, uniform within the band, randomly
// inflected; a phrase is two adjacent words of a visible message that is not deleted, at least
// one a content word, quoted. No query is function words only or holds a probe term, and every
// query stays within search.query_max_chars.
func (q *Queries) Next() Query {
	p := q.c.Plan
	for {
		lang := Langs[q.r.IntN(2)]
		class := Classes[pick(q.r.Float64(), p.ShareOneWord, p.ShareTwoWords, p.ShareThreeWords, p.SharePhrase, p.ShareNegation)]
		out := Query{Searcher: q.searcher, Class: class, Lang: lang}
		if class == ClassPhrase {
			out.Text, out.Lang = q.phrase()
			out.MustFind = true
		} else {
			out.Band = Bands[pick(q.r.Float64(), p.ShareFrequent, p.ShareMedium, p.ShareRare)]
			words := q.distinct(lang, out.Band, wordsOf[class])
			out.Text = strings.Join(words, " ")
			if class == ClassNegation {
				out.Text = words[0] + " -" + words[1]
			}
		}
		if utf8.RuneCountInString(out.Text) <= p.Defaults.QueryMaxChars {
			return out
		}
	}
}

// wordsOf is the number of words of the word classes and the negation.
var wordsOf = map[Class]int{ClassOneWord: 1, ClassTwoWords: 2, ClassThreeWords: 3, ClassNegation: 2}

// BandRanks returns the content ranks (0-based, inclusive) of band b; every band other than
// frequent and medium is rare.
func (p *Plan) BandRanks(b Band) (lo, hi int) {
	if b == BandFrequent {
		return 0, p.BandFrequentMax - 1
	}
	if b == BandMedium {
		return p.BandFrequentMax, p.BandMediumMax - 1
	}
	return p.BandMediumMax, p.ContentWords - 1
}

// BandOf returns the band of a content rank (0-based).
func (p *Plan) BandOf(rank int) Band {
	switch {
	case rank < p.BandFrequentMax:
		return BandFrequent
	case rank < p.BandMediumMax:
		return BandMedium
	default:
		return BandRare
	}
}

// distinct returns n inflected forms of n different stems of band b in lang.
func (q *Queries) distinct(lang Lang, b Band, n int) []string {
	lv := &q.c.Vocab.Langs[lang]
	lo, hi := q.c.Plan.BandRanks(b)
	seen := map[int]bool{}
	out := make([]string, 0, n)
	for len(out) < n {
		rank := lo + q.r.IntN(hi-lo+1)
		if seen[rank] {
			continue
		}
		seen[rank] = true
		out = append(out, lv.Form(rank, q.r.IntN(len(lv.Inflections))))
	}
	return out
}

// phrase returns a quoted pair of adjacent words of a random visible message that is not deleted,
// at least one of them a content word and neither the probe term, with the message's language.
func (q *Queries) phrase() (string, Lang) {
	n := q.c.Plan.Messages
	for {
		i := q.r.IntN(n)
		meta := q.c.Meta(i)
		if meta.Deleted || !q.c.Plan.Visible(q.searcher, meta.Channel) {
			continue
		}
		m := q.c.Message(i)
		words := Words(m.Text)
		pairs := q.c.Vocab.phrasePairs(m.Lang, words)
		if len(pairs) == 0 {
			continue
		}
		j := pairs[q.r.IntN(len(pairs))]
		return fmt.Sprintf("%q", words[j]+" "+words[j+1]), m.Lang
	}
}

// phrasePairs returns the positions j of words where words[j] and words[j+1] can form a phrase:
// neither is the probe term and at least one is a content word of l.
func (v *Vocabulary) phrasePairs(l Lang, words []string) []int {
	var out []int
	for j := 0; j+1 < len(words); j++ {
		a, b := words[j], words[j+1]
		if a != v.Probe && b != v.Probe && (!v.IsFunction(l, a) || !v.IsFunction(l, b)) {
			out = append(out, j)
		}
	}
	return out
}

// NextPrefix returns the next query of phase prefix (design note 9): a random post in a forum
// channel the searcher may read, one word of its title (share SharePrefixOneWord) or two, each cut
// to a random length between search.prefix_min_chars and its full length, joined by a space.
func (q *Queries) NextPrefix() (Query, error) {
	p := q.c.Plan
	if len(q.forums) == 0 {
		return Query{}, errNoVisible
	}
	for {
		ch := q.forums[q.r.IntN(len(q.forums))]
		first, count := p.ChannelPosts(ch)
		post := first + q.r.IntN(count)
		var words []string
		for _, w := range Words(q.c.Title(post)) {
			if utf8.RuneCountInString(w) >= p.Defaults.PrefixMinChars {
				words = append(words, w)
			}
		}
		if len(words) == 0 {
			continue
		}
		n := 2
		if q.r.Float64() < p.SharePrefixOneWord || len(words) == 1 {
			n = 1
		}
		parts := make([]string, 0, n)
		for _, k := range q.r.Perm(len(words))[:n] {
			runes := []rune(words[k])
			cut := p.Defaults.PrefixMinChars + q.r.IntN(len(runes)-p.Defaults.PrefixMinChars+1)
			parts = append(parts, string(runes[:cut]))
		}
		return Query{Searcher: q.searcher, Class: ClassPrefix, Lang: p.AuthorLang(q.c.PostCreator(post)), Text: strings.Join(parts, " "), MustFind: true}, nil
	}
}

// ProbeQuery returns the probe query of searcher k: the probe term (absent false), which occurs
// only in channels of the probe space, or the absent term, which occurs nowhere (design note 11).
func (c *Corpus) ProbeQuery(k int, absent bool) Query {
	if absent {
		return Query{Searcher: k, Class: ClassAbsent, Text: c.Vocab.Absent}
	}
	return Query{Searcher: k, Class: ClassProbe, Text: c.Vocab.Probe}
}
