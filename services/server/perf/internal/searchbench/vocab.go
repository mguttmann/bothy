// SPDX-License-Identifier: Apache-2.0

package searchbench

import (
	"errors"
	"fmt"
	"math/rand/v2"
	"strings"
	"unicode/utf8"
)

// Lang is a language of the corpus: German or English (14 section 7).
type Lang int

// The two languages of the corpus.
const (
	German Lang = iota
	English
)

// Langs are the languages of the corpus in order.
var Langs = []Lang{German, English}

// Locale returns the language tag a person of l sets with updateMyProfile.
func (l Lang) Locale() string {
	if l == German {
		return "de"
	}
	return "en"
}

// String returns "de" or "en".
func (l Lang) String() string { return l.Locale() }

// The function words of each language (design note 5): common words written for this test, the
// facts of the language, not a third-party list. A text draws them by a Zipf distribution in this
// order. The German words and letters below are test data of the corpus, not prose.
//
// language-check: off
var functionWords = [2][]string{
	German: {"der", "die", "und", "in", "das", "nicht", "ist", "zu", "ein", "mit", "den", "auf", "eine", "es", "sich",
		"von", "dem", "auch", "im", "noch", "wir", "aber", "nur", "oder", "wenn", "wie", "was", "dass", "bei", "nach",
		"aus", "um", "so", "am", "schon", "hier", "mehr", "des", "ich", "sie"},
	English: {"the", "and", "of", "to", "a", "in", "is", "it", "not", "that", "for", "on", "with", "this", "be",
		"are", "was", "as", "at", "by", "from", "we", "you", "they", "but", "if", "so", "no", "all", "there",
		"what", "which", "can", "will", "has", "have", "he", "she", "an", "or"},
}

// The inflections of each language (design note 5); every occurrence of a stem takes one, so the
// stemmed configurations of 14 3.2 do real work.
var inflections = [2][]string{
	German:  {"", "e", "en", "er", "es", "ung", "ungen", "lich"},
	English: {"", "s", "ed", "ing", "er", "ly"},
}

// The syllables stems are made of. No part contains q, x or y, which the probe terms carry, so
// no form of a stem can share a prefix with a probe term. German vowels and codas include a, o, u
// with diaeresis and the sharp s (14 3.3 umlaut folding), never at the start of a stem, so the
// capital first letter of a text is always ASCII.
var (
	onsets = [2][]string{
		German:  {"b", "d", "f", "g", "h", "k", "l", "m", "n", "p", "r", "s", "t", "w", "z", "sch", "br", "gr", "kl", "st", "pf", "tr"},
		English: {"b", "c", "d", "f", "g", "h", "k", "l", "m", "n", "p", "r", "s", "t", "v", "w", "br", "cr", "dr", "gr", "pl", "st", "tr"},
	}
	vowels = [2][]string{
		German:  {"a", "e", "i", "o", "u", "au", "ei", "ie", "\u00e4", "\u00f6", "\u00fc"},
		English: {"a", "e", "i", "o", "u", "ea", "oo", "ai"},
	}
	codas = [2][]string{
		German:  {"", "", "n", "r", "l", "s", "t", "m", "ch", "ng", "nd", "rt", "\u00df"},
		English: {"", "", "n", "r", "l", "s", "t", "m", "nd", "ck", "ll"},
	}
	probeOnsets = []string{"qx", "xq", "yq"}
)

// language-check: on

// The lengths of a stem in letters (design note 5).
const (
	stemMinLetters = 4
	stemMaxLetters = 10
	// probeStream is the PCG stream of the probe terms, apart from the streams of the languages.
	probeStream = 1 << 20
)

// LangVocabulary is the vocabulary of one language: the function words, the content stems in rank
// order (rank 1 at index 0) and the inflections.
type LangVocabulary struct {
	Function    []string
	Stems       []string
	Inflections []string
}

// Form returns stem rank (0-based) with inflection k.
func (v *LangVocabulary) Form(rank, k int) string { return v.Stems[rank] + v.Inflections[k] }

// Vocabulary is the generated vocabulary of both languages and the two probe terms of design note
// 11: Probe occurs only in channels of the probe space, Absent nowhere. It is a pure function of
// the seed and the stem count and is safe for concurrent reads.
type Vocabulary struct {
	Langs         [2]LangVocabulary
	Probe, Absent string
	forms         map[string]bool
}

// NewVocabulary generates contentWords stems per language from syllables with PCG seeded from
// (seed, language), rejecting every stem outside four to ten letters or one of whose forms equals
// a function word or a form of another stem; then the two probe terms, which equal no form. The
// error reports a stem count below one or a syllable set that cannot give enough stems.
func NewVocabulary(seed uint64, contentWords int) (*Vocabulary, error) {
	if contentWords < 1 {
		return nil, errors.New("searchbench: ContentWords must be at least 1")
	}
	v := &Vocabulary{forms: map[string]bool{}}
	for _, l := range Langs {
		for _, w := range functionWords[l] {
			v.forms[w] = true
		}
	}
	for _, l := range Langs {
		lv, err := v.generate(seed, l, contentWords)
		if err != nil {
			return nil, err
		}
		v.Langs[l] = lv
	}
	v.probeTerms(seed)
	return v, nil
}

// generate draws the stems of l from PCG seeded from (seed, l) until contentWords are free, and
// takes their forms.
func (v *Vocabulary) generate(seed uint64, l Lang, contentWords int) (LangVocabulary, error) {
	r := rand.New(rand.NewPCG(seed, uint64(l)+1)) // #nosec G404 G115 -- a reproducible synthetic corpus; l is 0 or 1
	lv := LangVocabulary{Function: functionWords[l], Inflections: inflections[l]}
	for attempts := 0; len(lv.Stems) < contentWords; attempts++ {
		if attempts > 50*contentWords {
			return lv, fmt.Errorf("searchbench: only %d of %d %s stems after %d attempts", len(lv.Stems), contentWords, l, attempts)
		}
		stem := syllables(r, l)
		if !v.free(stem, lv.Inflections) {
			continue
		}
		for _, in := range lv.Inflections {
			v.forms[stem+in] = true
		}
		lv.Stems = append(lv.Stems, stem)
	}
	return lv, nil
}

// probeTerms draws the probe and the absent term: a probe onset (q, x and y occur in no stem) and
// English syllables, four to ten letters, equal to no form and to each other.
func (v *Vocabulary) probeTerms(seed uint64) {
	r := rand.New(rand.NewPCG(seed, probeStream)) // #nosec G404 -- a reproducible synthetic corpus, not a secret
	for v.Probe == "" || v.Absent == "" {
		term := probeOnsets[r.IntN(len(probeOnsets))] + syllables(r, English)
		if n := utf8.RuneCountInString(term); n < stemMinLetters || n > stemMaxLetters || v.forms[term] || term == v.Probe {
			continue
		}
		if v.Probe == "" {
			v.Probe = term
		} else {
			v.Absent = term
		}
	}
}

// syllables returns two or three syllables of l.
func syllables(r *rand.Rand, l Lang) string {
	var b strings.Builder
	for range 2 + r.IntN(2) {
		b.WriteString(onsets[l][r.IntN(len(onsets[l]))])
		b.WriteString(vowels[l][r.IntN(len(vowels[l]))])
		b.WriteString(codas[l][r.IntN(len(codas[l]))])
	}
	return b.String()
}

// free reports whether stem has four to ten letters and none of its forms is taken yet.
func (v *Vocabulary) free(stem string, infl []string) bool {
	if n := utf8.RuneCountInString(stem); n < stemMinLetters || n > stemMaxLetters {
		return false
	}
	for _, in := range infl {
		if v.forms[stem+in] {
			return false
		}
	}
	return true
}

// IsForm reports whether w is a function word or a form of a stem of either language.
func (v *Vocabulary) IsForm(w string) bool { return v.forms[w] }

// IsFunction reports whether w is a function word of l.
func (v *Vocabulary) IsFunction(l Lang, w string) bool {
	for _, f := range v.Langs[l].Function {
		if f == w {
			return true
		}
	}
	return false
}

// zipfOver returns a Zipf sampler over n ranks (0 to n-1) with exponent s on r: rank k has a
// weight proportional to (1 + k)^-s (math/rand/v2.NewZipf with v = 1).
func zipfOver(r *rand.Rand, s float64, n int) *rand.Zipf {
	return rand.NewZipf(r, s, 1, uint64(n-1)) // #nosec G115 -- n is a list length of at least one
}
