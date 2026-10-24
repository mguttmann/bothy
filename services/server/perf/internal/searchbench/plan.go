// SPDX-License-Identifier: Apache-2.0

package searchbench

import (
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/searchlang"
	"github.com/OWNER/bothy/services/server/internal/platform/config"
	"github.com/OWNER/bothy/services/server/internal/wire"
)

// The configuration keys the plan reads by name (brief M3-16 design note 17).
const (
	KeySearchPerMinute  = "ratelimit.rl.search.per_minute"
	KeySearchBurst      = "ratelimit.rl.search.burst"
	KeyQueryMaxChars    = "search.query_max_chars"
	KeyPrefixMinChars   = "search.prefix_min_chars"
	KeyMaxCandidates    = "search.max_candidates"
	KeyStatementTimeout = "search.statement_timeout"
	KeyRecencyScale     = "search.recency_scale"
	KeyLanguages        = "search.languages"
	KeyLocaleMap        = "search.locale_map"
	KeyTitleMaxChars    = "thread.title_max_chars"
	KeyMaxContentBytes  = "message.max_content_bytes"
	// KeyBenchmarkMessages is the corpus size of 14 section 15 (place Load test).
	KeyBenchmarkMessages = "search.benchmark_messages"
)

// Defaults are the configuration defaults a measured server runs with, read by key name from the
// server's schema (LoadDefaults); tests change single fields.
type Defaults struct {
	SearchPerMinute, SearchBurst                 int
	QueryMaxChars, PrefixMinChars, MaxCandidates int
	TitleMaxChars, MaxContentBytes               int
	StatementTimeout, RecencyScale               time.Duration
	Languages                                    []string
	LocaleMap                                    map[string]string
}

// LoadDefaults reads the defaults of the keys of design note 17 from the schema of the server
// (wire.ConfigSchema) with no source but the mandatory public URL. The error reports a schema or
// key that cannot be read.
func LoadDefaults() (Defaults, error) {
	schema, err := wire.ConfigSchema()
	if err != nil {
		return Defaults{}, err
	}
	v, err := config.Load(schema, config.Sources{Env: []string{config.EnvName("server.public_url") + "=http://127.0.0.1:1"}})
	if err != nil {
		return Defaults{}, err
	}
	return Defaults{
		SearchPerMinute: int(v.Int(KeySearchPerMinute)), SearchBurst: int(v.Int(KeySearchBurst)),
		QueryMaxChars: int(v.Int(KeyQueryMaxChars)), PrefixMinChars: int(v.Int(KeyPrefixMinChars)),
		MaxCandidates: int(v.Int(KeyMaxCandidates)), TitleMaxChars: int(v.Int(KeyTitleMaxChars)),
		MaxContentBytes: int(v.Int(KeyMaxContentBytes)), StatementTimeout: v.Duration(KeyStatementTimeout),
		RecencyScale: v.Duration(KeyRecencyScale), Languages: v.Strings(KeyLanguages), LocaleMap: v.StringMap(KeyLocaleMap),
	}, nil
}

// Map returns the defaults by key name, for extra.search of results.json.
func (d Defaults) Map() map[string]any {
	return map[string]any{
		KeySearchPerMinute: d.SearchPerMinute, KeySearchBurst: d.SearchBurst, KeyQueryMaxChars: d.QueryMaxChars,
		KeyPrefixMinChars: d.PrefixMinChars, KeyMaxCandidates: d.MaxCandidates, KeyTitleMaxChars: d.TitleMaxChars,
		KeyMaxContentBytes: d.MaxContentBytes, KeyStatementTimeout: d.StatementTimeout.String(),
		KeyRecencyScale: d.RecencyScale.String(), KeyLanguages: d.Languages, KeyLocaleMap: d.LocaleMap,
	}
}

// ChannelKind is the kind of a channel of the world.
type ChannelKind string

// The kinds of the channels of the world.
const (
	TextChannel  ChannelKind = "text"
	ForumChannel ChannelKind = "forum"
)

// Plan is the shape of one run (design notes 3, 4 and 7), derived from Params, the corpus size
// and the defaults. Channels are numbered space by space, the text channels of a space before its
// forum channels; posts are numbered forum channel by forum channel.
type Plan struct {
	Params
	Defaults Defaults
	// Messages is the corpus size N.
	Messages int
	// ChannelsPerSpace, Channels, TextChannels, ForumChannels and Posts count the world.
	ChannelsPerSpace, Channels, TextChannels, ForumChannels, Posts int
	// ProbeSpace is the last space (design note 11); ProbeSearchers are the searchers without a
	// grant on it, in order.
	ProbeSpace     int
	ProbeSearchers []int
	// SearcherRate is the requests per second one searcher plans (BudgetShare of
	// ratelimit.rl.search.per_minute), Interval its spacing; Rate is the rate of all searchers.
	SearcherRate float64
	Interval     time.Duration
	Rate         float64
	// PlannedSamples is the requests of all searchers in Window.
	PlannedSamples float64
	// Lang maps a locale to its text search configuration (search.languages and
	// search.locale_map).
	Lang searchlang.Map
}

// NewPlan derives the plan of p for a corpus of n messages under the defaults d. The error
// reports parameters outside their range (shares that do not add up to one, bands outside the
// stems, fewer messages than posts, odd or too few spaces), a locale map that does not give
// german for de and english for en, a rate whose Window holds fewer than MinSamples samples
// (naming ratelimit.rl.search.per_minute and the constants), and a budget at or above the limit.
func NewPlan(p Params, n int, d Defaults) (*Plan, error) {
	if err := p.check(); err != nil {
		return nil, err
	}
	cps := p.TextChannelsPerSpace + p.ForumChannelsPerSpace
	pl := &Plan{Params: p, Defaults: d, Messages: n, ChannelsPerSpace: cps, Channels: p.Spaces * cps,
		TextChannels: p.Spaces * p.TextChannelsPerSpace, ForumChannels: p.Spaces * p.ForumChannelsPerSpace, ProbeSpace: p.Spaces - 1}
	pl.Posts = pl.ForumChannels * p.ForumPostsPerChannel
	if n < pl.Posts || n < 1 {
		return nil, fmt.Errorf("searchbench: %d messages cannot open %d posts", n, pl.Posts)
	}
	m, err := searchlang.New(d.Languages, d.LocaleMap)
	if err != nil {
		return nil, err
	}
	for _, l := range Langs {
		if got, want := m.For(l.Locale()), configOf(l); got != want {
			return nil, fmt.Errorf("searchbench: %s maps %s to %s, the corpus needs %s", KeyLocaleMap, l.Locale(), got, want)
		}
	}
	pl.Lang = m
	if d.SearchPerMinute <= 0 || d.SearchBurst <= 0 {
		return nil, fmt.Errorf("searchbench: %s = %d and %s = %d must be positive", KeySearchPerMinute, d.SearchPerMinute, KeySearchBurst, d.SearchBurst)
	}
	pl.SearcherRate = p.BudgetShare * float64(d.SearchPerMinute) / 60
	pl.Interval = time.Duration(float64(time.Second) / pl.SearcherRate)
	pl.Rate = pl.SearcherRate * float64(p.Searchers)
	pl.PlannedSamples = pl.Rate * p.Window.Seconds()
	if pl.PlannedSamples < float64(p.MinSamples) {
		return nil, fmt.Errorf("searchbench: %d searchers at BudgetShare %g of %s = %d plan %.0f samples in Window %s, fewer than MinSamples %d",
			p.Searchers, p.BudgetShare, KeySearchPerMinute, d.SearchPerMinute, pl.PlannedSamples, p.Window, p.MinSamples)
	}
	for k := range p.Searchers {
		if !pl.SpaceVisible(k, pl.ProbeSpace) {
			pl.ProbeSearchers = append(pl.ProbeSearchers, k)
		}
	}
	if len(pl.ProbeSearchers) == 0 {
		return nil, errors.New("searchbench: every searcher sees the probe space")
	}
	return pl, nil
}

// check validates the parameters that do not depend on the configuration.
func (p Params) check() error {
	const eps = 1e-9
	switch {
	case p.Spaces < 2 || p.Spaces%2 != 0:
		return fmt.Errorf("searchbench: Spaces = %d must be even and at least 2", p.Spaces)
	case p.TextChannelsPerSpace < 1 || p.ForumChannelsPerSpace < 1 || p.ForumPostsPerChannel < 1:
		return errors.New("searchbench: every space needs a text channel and a forum channel with posts")
	case p.Authors < 2 || p.Authors%2 != 0 || p.Searchers < 1:
		return fmt.Errorf("searchbench: Authors = %d must be even and at least 2, Searchers = %d at least 1", p.Authors, p.Searchers)
	case p.ZipfS <= 1:
		return fmt.Errorf("searchbench: ZipfS = %g must be above one", p.ZipfS)
	case p.MessageWordsMin < 2 || p.MessageWordsMax < p.MessageWordsMin || p.TitleWordsMin < 1 || p.TitleWordsMax < p.TitleWordsMin:
		return errors.New("searchbench: the word counts of messages (at least 2) and titles (at least 1) are out of order")
	case p.BandFrequentMax < 1 || p.BandMediumMax <= p.BandFrequentMax || p.ContentWords <= p.BandMediumMax:
		return fmt.Errorf("searchbench: bands %d < %d < ContentWords %d", p.BandFrequentMax, p.BandMediumMax, p.ContentWords)
	case math.Abs(p.ShareOneWord+p.ShareTwoWords+p.ShareThreeWords+p.SharePhrase+p.ShareNegation-1) > eps:
		return errors.New("searchbench: the class shares do not add up to one")
	case math.Abs(p.ShareFrequent+p.ShareMedium+p.ShareRare-1) > eps:
		return errors.New("searchbench: the band shares do not add up to one")
	case p.BudgetShare <= 0 || p.BudgetShare >= 1:
		return fmt.Errorf("searchbench: BudgetShare = %g must lie between zero and one", p.BudgetShare)
	case p.CorpusSpan <= 0 || p.Window <= 0 || p.LoadBatch < 1:
		return errors.New("searchbench: CorpusSpan, Window and LoadBatch must be positive")
	}
	return nil
}

// configOf is the text search configuration a language of the corpus needs.
func configOf(l Lang) string {
	if l == German {
		return "german"
	}
	return "english"
}

// ChannelSpace returns the space of channel c.
func (p *Plan) ChannelSpace(c int) int { return c / p.ChannelsPerSpace }

// ChannelKind returns the kind of channel c.
func (p *Plan) ChannelKind(c int) ChannelKind {
	if c%p.ChannelsPerSpace < p.TextChannelsPerSpace {
		return TextChannel
	}
	return ForumChannel
}

// forumIndex returns the number of forum channel c among the forum channels.
func (p *Plan) forumIndex(c int) int {
	return p.ChannelSpace(c)*p.ForumChannelsPerSpace + c%p.ChannelsPerSpace - p.TextChannelsPerSpace
}

// ChannelPosts returns the first post of forum channel c and the number of its posts.
func (p *Plan) ChannelPosts(c int) (first, count int) {
	return p.forumIndex(c) * p.ForumPostsPerChannel, p.ForumPostsPerChannel
}

// PostChannel returns the forum channel of post.
func (p *Plan) PostChannel(post int) int {
	f := post / p.ForumPostsPerChannel
	return f/p.ForumChannelsPerSpace*p.ChannelsPerSpace + p.TextChannelsPerSpace + f%p.ForumChannelsPerSpace
}

// SearcherSpaces returns the two spaces searcher k may read (design note 4): k mod Spaces and
// (k + Spaces/2) mod Spaces.
func (p *Plan) SearcherSpaces(k int) [2]int {
	return [2]int{k % p.Spaces, (k + p.Spaces/2) % p.Spaces}
}

// SpaceVisible reports whether searcher k may read space s.
func (p *Plan) SpaceVisible(k, s int) bool {
	sp := p.SearcherSpaces(k)
	return sp[0] == s || sp[1] == s
}

// Visible reports whether searcher k may read channel c, the truth of the checks.
func (p *Plan) Visible(k, c int) bool { return p.SpaceVisible(k, p.ChannelSpace(c)) }

// AuthorLang returns the language of author a: the first half German, the second English.
func (p *Plan) AuthorLang(a int) Lang {
	if a < p.Authors/2 {
		return German
	}
	return English
}

// AuthorConfig returns the search_language the product stores for author a: searchlang.Map.For of
// the author's locale, as identity does (M3-13).
func (p *Plan) AuthorConfig(a int) string { return p.Lang.For(p.AuthorLang(a).Locale()) }

// SearcherLang returns the language of the profile of searcher k, German and English alternating.
func (p *Plan) SearcherLang(k int) Lang {
	if k%2 == 0 {
		return German
	}
	return English
}

// Map returns the counts and rates of the plan, for extra.search of results.json.
func (p *Plan) Map() map[string]any {
	return map[string]any{
		keyMessages: p.Messages, "spaces": p.Spaces, "channels": p.Channels, "text_channels": p.TextChannels,
		"forum_channels": p.ForumChannels, "posts": p.Posts, "authors": p.Authors, "searchers": p.Searchers,
		"probe_searchers": len(p.ProbeSearchers), "visible_channel_share": 2.0 / float64(p.Spaces),
		"planned_requests_per_s": p.Rate, "planned_requests_per_searcher_per_min": p.SearcherRate * 60,
		"planned_samples_per_window": p.PlannedSamples,
	}
}
