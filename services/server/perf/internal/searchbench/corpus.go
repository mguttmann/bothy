// SPDX-License-Identifier: Apache-2.0

package searchbench

import (
	"errors"
	"fmt"
	"math/rand/v2"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

// The PCG streams of the corpus: message i uses stream i, the title of post p stream
// titleStream + p, so the streams never meet.
const titleStream = 1 << 40

// Corpus is the deterministic synthetic corpus of a plan (design note 5): every message and every
// post title is a pure function of the seed and its index, so every run on every machine builds
// the same corpus and the query generator rebuilds any message without storing it. It is safe for
// concurrent use.
type Corpus struct {
	Plan  *Plan
	Vocab *Vocabulary
	// Start is the load time minus CorpusSpan; message i is created at
	// Start + (i + 1) * CorpusSpan / N, cut to the millisecond.
	Start time.Time
}

// NewCorpus returns the corpus of plan whose newest message is created at loadTime (read from the
// database clock, 03 principle 2). The error reports a vocabulary that cannot be built or a plan
// whose title or message limits cannot hold the longest generated text.
func NewCorpus(plan *Plan, loadTime time.Time) (*Corpus, error) {
	v, err := NewVocabulary(plan.Seed, plan.ContentWords)
	if err != nil {
		return nil, err
	}
	// The longest text: every word a longest form plus a space, plus the probe term and the end.
	longest := (stemMaxLetters*2+len("ungen")+1)*(plan.MessageWordsMax+1) + 1
	if longest > plan.Defaults.MaxContentBytes {
		return nil, fmt.Errorf("searchbench: a message of %d words may exceed %s = %d", plan.MessageWordsMax, KeyMaxContentBytes, plan.Defaults.MaxContentBytes)
	}
	if (stemMaxLetters+len("ungen")+1)*plan.TitleWordsMax > plan.Defaults.TitleMaxChars {
		return nil, fmt.Errorf("searchbench: a title of %d words may exceed %s = %d", plan.TitleWordsMax, KeyTitleMaxChars, plan.Defaults.TitleMaxChars)
	}
	return &Corpus{Plan: plan, Vocab: v, Start: loadTime.Add(-plan.CorpusSpan).Truncate(time.Millisecond)}, nil
}

// Meta is the part of a message that the loader needs before the texts: where it goes, who writes
// it, when, and whether it is deleted.
type Meta struct {
	Index   int
	Channel int
	// Post is the post of a forum channel, -1 for the main thread of a text channel.
	Post   int
	Author int
	// Opens marks the first message of a post (indexes 0 to Posts-1, in post order).
	Opens     bool
	Deleted   bool
	CreatedAt time.Time
}

// Message is a message of the corpus: Meta, the language of the author and the text. Text is the
// content before a deletion; the loader writes an empty content for a deleted message.
type Message struct {
	Meta
	Lang Lang
	Text string
	// Probe marks a text that carries the probe term.
	Probe bool
}

// rng returns the generator of message i.
func (c *Corpus) rng(i int) *rand.Rand {
	return rand.New(rand.NewPCG(c.Plan.Seed, uint64(i))) // #nosec G404 G115 -- a reproducible synthetic corpus; i is a non-negative index
}

// CreatedAt returns the creation time of message i: time grows strictly with the index.
func (c *Corpus) CreatedAt(i int) time.Time {
	off := time.Duration(float64(c.Plan.CorpusSpan) * float64(i+1) / float64(c.Plan.Messages))
	return c.Start.Add(off).Truncate(time.Millisecond)
}

// Meta returns the meta data of message i (0 <= i < N) without building its text.
func (c *Corpus) Meta(i int) Meta {
	m, _ := c.meta(i)
	return m
}

// meta draws the meta data of message i from its generator and returns the generator for the
// text, so Meta and Message agree.
func (c *Corpus) meta(i int) (Meta, *rand.Rand) {
	p := c.Plan
	r := c.rng(i)
	m := Meta{Index: i, Post: -1, CreatedAt: c.CreatedAt(i)}
	if i < p.Posts {
		m.Post, m.Opens = i, true
		m.Channel = p.PostChannel(i)
		m.Author = c.PostCreator(i)
		_ = r.Float64() // the deletion draw, unused for an opening message
		return m, r
	}
	m.Channel = r.IntN(p.Channels)
	if p.ChannelKind(m.Channel) == ForumChannel {
		first, count := p.ChannelPosts(m.Channel)
		m.Post = first + r.IntN(count)
	}
	m.Author = r.IntN(p.Authors)
	m.Deleted = r.Float64() < p.DeletedShare
	return m, r
}

// Message returns message i (0 <= i < N): Meta, then MessageWordsMin to MessageWordsMax words in
// the language of the author, each a function word with probability FunctionWordShare (Zipf over
// the function words) or a content stem drawn by Zipf with ZipfS over the ranks and inflected; the
// first word capitalized, ending with "." or "?". In a channel of the probe space the probe term
// replaces a word at a random position with probability ProbeTermShare.
func (c *Corpus) Message(i int) Message {
	m, r := c.meta(i)
	lang := c.Plan.AuthorLang(m.Author)
	words := c.words(r, lang, c.Plan.MessageWordsMin+r.IntN(c.Plan.MessageWordsMax-c.Plan.MessageWordsMin+1), true)
	probe := r.Float64() < c.Plan.ProbeTermShare && c.Plan.ChannelSpace(m.Channel) == c.Plan.ProbeSpace
	if probe {
		words[r.IntN(len(words))] = c.Vocab.Probe
	}
	end := "."
	if r.IntN(4) == 0 {
		end = "?"
	}
	return Message{Meta: m, Lang: lang, Text: capitalize(strings.Join(words, " ")) + end, Probe: probe}
}

// words draws n words of lang from r; function words only when withFunction.
func (c *Corpus) words(r *rand.Rand, lang Lang, n int, withFunction bool) []string {
	lv := &c.Vocab.Langs[lang]
	content := zipfOver(r, c.Plan.ZipfS, len(lv.Stems))
	function := zipfOver(r, c.Plan.ZipfS, len(lv.Function))
	out := make([]string, n)
	for k := range out {
		if withFunction && r.Float64() < c.Plan.FunctionWordShare {
			out[k] = lv.Function[function.Uint64()]
			continue
		}
		out[k] = lv.Form(int(content.Uint64()), r.IntN(len(lv.Inflections))) // #nosec G115 -- a rank below len(lv.Stems)
	}
	return out
}

// PostCreator returns the author who creates post p, drawn from the title generator of p.
func (c *Corpus) PostCreator(p int) int {
	return rand.New(rand.NewPCG(c.Plan.Seed, titleStream+uint64(p))).IntN(c.Plan.Authors) // #nosec G404 G115 -- a reproducible synthetic corpus; p is a non-negative index
}

// Title returns the title of post p: TitleWordsMin to TitleWordsMax content words in the language
// of its creator, the first capitalized.
func (c *Corpus) Title(p int) string {
	r := rand.New(rand.NewPCG(c.Plan.Seed, titleStream+uint64(p))) // #nosec G404 G115 -- a reproducible synthetic corpus; p is a non-negative index
	creator := r.IntN(c.Plan.Authors)
	n := c.Plan.TitleWordsMin + r.IntN(c.Plan.TitleWordsMax-c.Plan.TitleWordsMin+1)
	return capitalize(strings.Join(c.words(r, c.Plan.AuthorLang(creator), n, false), " "))
}

// capitalize upper-cases the first letter of s (always ASCII, see the syllables).
func capitalize(s string) string {
	r, size := utf8.DecodeRuneInString(s)
	return string(unicode.ToUpper(r)) + s[size:]
}

// Words splits a text of the corpus into its words without the end mark, lower case.
func Words(text string) []string {
	text = strings.TrimRight(text, ".?")
	out := strings.Fields(text)
	for i, w := range out {
		out[i] = strings.ToLower(w)
	}
	return out
}

// errNoVisible is the error of a searcher that sees no channel.
var errNoVisible = errors.New("searchbench: the searcher sees no channel")
