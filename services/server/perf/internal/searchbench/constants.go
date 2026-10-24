// SPDX-License-Identifier: Apache-2.0

package searchbench

import "time"

// The named parameters of perf/search (brief M3-16 design note 3, docs/DECISIONS.md D-M3-16-3).
// Chapter 14 section 7 fixes the languages, the Zipf distribution, several spaces and channels
// and a partial view of each person; the values below fix everything else. They are parameters of
// a test, not limits of the product; perf/README.md lists them and results.json copies them under
// extra.search.
const (
	// Seed seeds every generator of the vocabulary, the corpus and the queries.
	Seed = 1

	// Spaces, TextChannelsPerSpace and ForumChannelsPerSpace shape the world; every forum channel
	// holds ForumPostsPerChannel posts.
	Spaces                = 8
	TextChannelsPerSpace  = 4
	ForumChannelsPerSpace = 4
	ForumPostsPerChannel  = 600
	// Authors write the corpus (write on every space); Searchers search, each with read on two of
	// the spaces, so a searcher sees a quarter of the channels.
	Authors   = 32
	Searchers = 24

	// ContentWords is the number of generated stems per language.
	ContentWords = 50000
	// ZipfS is the exponent of the word frequencies: natural text is close to one, and
	// math/rand/v2.NewZipf needs a value above one.
	ZipfS = 1.07
	// FunctionWordShare is the share of function words in running text.
	FunctionWordShare = 0.4
	// MessageWordsMin and MessageWordsMax bound the words of a message, TitleWordsMin and
	// TitleWordsMax the content words of a post title.
	MessageWordsMin = 3
	MessageWordsMax = 40
	TitleWordsMin   = 2
	TitleWordsMax   = 6
	// CorpusSpan is the age of the oldest message: a year, so the recency of
	// search.recency_scale matters.
	CorpusSpan = 365 * 24 * time.Hour
	// DeletedShare is the share of deleted messages (never the first message of a post).
	DeletedShare = 0.01
	// ProbeTermShare is the share of the messages of the probe space that carry the probe term.
	ProbeTermShare = 0.2
	// LoadBatch is the number of message rows per COPY transaction.
	LoadBatch = 100000
	// ConsistencySamples is the number of threads the product checks compare with the loader.
	ConsistencySamples = 50

	// BudgetShare is the share of ratelimit.rl.search.per_minute a searcher plans at most.
	BudgetShare = 0.5
	// Warmup is sent without samples, Window with samples, Drain is the longest wait for the
	// answers still open after the window.
	Warmup = 30 * time.Second
	Window = 3 * time.Minute
	Drain  = 30 * time.Second
	// MinSamples is the fewest samples a phase may rest on.
	MinSamples = 1000
	// ProbeWarmup queries of each probe term are unmeasured, then ProbeQueries of each are
	// measured.
	ProbeWarmup  = 20
	ProbeQueries = 200

	// The query classes of phase messages and their shares.
	ShareOneWord    = 0.40
	ShareTwoWords   = 0.25
	ShareThreeWords = 0.15
	SharePhrase     = 0.10
	ShareNegation   = 0.10
	// The frequency bands of a query word: frequent are the content ranks 1 to BandFrequentMax,
	// medium up to BandMediumMax, rare the rest; with their shares.
	BandFrequentMax = 100
	BandMediumMax   = 5000
	ShareFrequent   = 0.2
	ShareMedium     = 0.5
	ShareRare       = 0.3
	// SharePrefixOneWord is the share of prefix queries with one word; the rest have two.
	SharePrefixOneWord = 0.7
)

// Params are the parameters of one run: DefaultParams for perf/search, a small world for the
// small-corpus tests of perf/internal/searchworld, the same code for both.
type Params struct {
	Seed                                                           uint64
	Spaces, TextChannelsPerSpace, ForumChannelsPerSpace            int
	ForumPostsPerChannel, Authors, Searchers                       int
	ContentWords                                                   int
	ZipfS, FunctionWordShare                                       float64
	MessageWordsMin, MessageWordsMax, TitleWordsMin, TitleWordsMax int
	CorpusSpan                                                     time.Duration
	DeletedShare, ProbeTermShare                                   float64
	LoadBatch, ConsistencySamples                                  int
	BudgetShare                                                    float64
	Warmup, Window, Drain                                          time.Duration
	MinSamples, ProbeWarmup, ProbeQueries                          int
	ShareOneWord, ShareTwoWords, ShareThreeWords                   float64
	SharePhrase, ShareNegation                                     float64
	BandFrequentMax, BandMediumMax                                 int
	ShareFrequent, ShareMedium, ShareRare                          float64
	SharePrefixOneWord                                             float64
}

// DefaultParams returns the constants of this file as Params.
func DefaultParams() Params {
	return Params{
		Seed: Seed, Spaces: Spaces, TextChannelsPerSpace: TextChannelsPerSpace, ForumChannelsPerSpace: ForumChannelsPerSpace,
		ForumPostsPerChannel: ForumPostsPerChannel, Authors: Authors, Searchers: Searchers, ContentWords: ContentWords,
		ZipfS: ZipfS, FunctionWordShare: FunctionWordShare, MessageWordsMin: MessageWordsMin, MessageWordsMax: MessageWordsMax,
		TitleWordsMin: TitleWordsMin, TitleWordsMax: TitleWordsMax, CorpusSpan: CorpusSpan, DeletedShare: DeletedShare,
		ProbeTermShare: ProbeTermShare, LoadBatch: LoadBatch, ConsistencySamples: ConsistencySamples, BudgetShare: BudgetShare,
		Warmup: Warmup, Window: Window, Drain: Drain, MinSamples: MinSamples, ProbeWarmup: ProbeWarmup, ProbeQueries: ProbeQueries,
		ShareOneWord: ShareOneWord, ShareTwoWords: ShareTwoWords, ShareThreeWords: ShareThreeWords, SharePhrase: SharePhrase,
		ShareNegation: ShareNegation, BandFrequentMax: BandFrequentMax, BandMediumMax: BandMediumMax, ShareFrequent: ShareFrequent,
		ShareMedium: ShareMedium, ShareRare: ShareRare, SharePrefixOneWord: SharePrefixOneWord,
	}
}

// Map returns the parameters by their constant names, for extra.search of results.json.
func (p Params) Map() map[string]any {
	return map[string]any{
		"Seed": p.Seed, "Spaces": p.Spaces, "TextChannelsPerSpace": p.TextChannelsPerSpace, "ForumChannelsPerSpace": p.ForumChannelsPerSpace,
		"ForumPostsPerChannel": p.ForumPostsPerChannel, "Authors": p.Authors, "Searchers": p.Searchers, "ContentWords": p.ContentWords,
		"ZipfS": p.ZipfS, "FunctionWordShare": p.FunctionWordShare, "MessageWordsMin": p.MessageWordsMin, "MessageWordsMax": p.MessageWordsMax,
		"TitleWordsMin": p.TitleWordsMin, "TitleWordsMax": p.TitleWordsMax, "CorpusSpan": p.CorpusSpan.String(), "DeletedShare": p.DeletedShare,
		"ProbeTermShare": p.ProbeTermShare, "LoadBatch": p.LoadBatch, "ConsistencySamples": p.ConsistencySamples, "BudgetShare": p.BudgetShare,
		"Warmup": p.Warmup.String(), "Window": p.Window.String(), "Drain": p.Drain.String(), "MinSamples": p.MinSamples,
		"ProbeWarmup": p.ProbeWarmup, "ProbeQueries": p.ProbeQueries, "ShareOneWord": p.ShareOneWord, "ShareTwoWords": p.ShareTwoWords,
		"ShareThreeWords": p.ShareThreeWords, "SharePhrase": p.SharePhrase, "ShareNegation": p.ShareNegation,
		"BandFrequentMax": p.BandFrequentMax, "BandMediumMax": p.BandMediumMax, "ShareFrequent": p.ShareFrequent,
		"ShareMedium": p.ShareMedium, "ShareRare": p.ShareRare, "SharePrefixOneWord": p.SharePrefixOneWord,
	}
}

// SmallParams returns the small world of the small-corpus tests (brief M3-16 "Tests"): four spaces
// with one text and one forum channel each, five posts per forum, four authors and four searchers,
// so every searcher sees half of the spaces, two thousand stems per language with bands of
// matching size, and a short window.
func SmallParams() Params {
	p := DefaultParams()
	p.Spaces, p.TextChannelsPerSpace, p.ForumChannelsPerSpace, p.ForumPostsPerChannel = 4, 1, 1, 5
	p.Authors, p.Searchers = 4, 4
	p.ContentWords, p.BandFrequentMax, p.BandMediumMax = 2000, 50, 500
	p.CorpusSpan, p.LoadBatch, p.ConsistencySamples = 30*24*time.Hour, 500, 10
	p.Warmup, p.Window, p.Drain, p.MinSamples, p.ProbeWarmup, p.ProbeQueries = time.Second, 10*time.Second, time.Second, 10, 2, 5
	return p
}

// SmallMessages is the corpus size of SmallParams.
const SmallMessages = 2000
