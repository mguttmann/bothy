// SPDX-License-Identifier: Apache-2.0

package searchbench_test

import (
	"math"
	"slices"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/OWNER/bothy/services/server/perf/internal/report"
	"github.com/OWNER/bothy/services/server/perf/internal/searchbench"
)

// loadTime is the fixed load time of the unit tests.
var loadTime = time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)

// defaults returns the configuration defaults of the server.
func defaults(t *testing.T) searchbench.Defaults {
	t.Helper()
	d, err := searchbench.LoadDefaults()
	if err != nil {
		t.Fatal(err)
	}
	return d
}

// corpusOf builds the plan of p for n messages under the defaults and its corpus.
func corpusOf(t *testing.T, p searchbench.Params, n int) *searchbench.Corpus {
	t.Helper()
	plan, err := searchbench.NewPlan(p, n, defaults(t))
	if err != nil {
		t.Fatal(err)
	}
	c, err := searchbench.NewCorpus(plan, loadTime)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

// TestCorpus_PureByIndex: message i built twice is equal; created_at grows strictly; the first
// messages open the posts in order; the deleted share lies within tolerance and no first message
// is deleted; the probe term occurs only in the probe space and the absent term nowhere; every
// text and title stays within its limit; the language follows the author's locale.
func TestCorpus_PureByIndex(t *testing.T) {
	t.Parallel()
	const n = 60000
	p := searchbench.DefaultParams()
	c := corpusOf(t, p, n)
	d := c.Plan.Defaults
	deleted := 0
	prev := time.Time{}
	for i := range n {
		m := c.Message(i)
		if again := c.Message(i); again != m {
			t.Fatalf("message %d differs when built twice", i)
		}
		if meta := c.Meta(i); meta != m.Meta {
			t.Fatalf("message %d: Meta and Message disagree", i)
		}
		if !m.CreatedAt.After(prev) {
			t.Fatalf("message %d: created_at %s does not grow after %s", i, m.CreatedAt, prev)
		}
		prev = m.CreatedAt
		if i < c.Plan.Posts {
			if !m.Opens || m.Post != i || m.Deleted || m.Author != c.PostCreator(i) || m.Channel != c.Plan.PostChannel(i) {
				t.Fatalf("message %d does not open post %d: %+v", i, i, m.Meta)
			}
		} else if m.Opens {
			t.Fatalf("message %d opens a post", i)
		}
		if m.Deleted {
			deleted++
		}
		words := searchbench.Words(m.Text)
		if slices.Contains(words, c.Vocab.Probe) != m.Probe || (m.Probe && c.Plan.ChannelSpace(m.Channel) != c.Plan.ProbeSpace) {
			t.Fatalf("message %d: probe term outside the probe space or unmarked", i)
		}
		if slices.Contains(words, c.Vocab.Absent) {
			t.Fatalf("message %d holds the absent term", i)
		}
		if len(m.Text) > d.MaxContentBytes {
			t.Fatalf("message %d has %d bytes", i, len(m.Text))
		}
		if got, want := c.Plan.AuthorConfig(m.Author), c.Plan.Lang.For(c.Plan.AuthorLang(m.Author).Locale()); got != want || m.Lang != c.Plan.AuthorLang(m.Author) {
			t.Fatalf("message %d: language %s, want %s", i, got, want)
		}
	}
	if want, got := p.DeletedShare*float64(n-c.Plan.Posts), float64(deleted); math.Abs(got-want) > 0.2*want {
		t.Errorf("%d deleted messages, want %.0f within 20 percent", deleted, want)
	}
	if c.CreatedAt(n-1).After(loadTime) || c.CreatedAt(0).Before(loadTime.Add(-p.CorpusSpan)) {
		t.Errorf("created_at outside the span: %s .. %s", c.CreatedAt(0), c.CreatedAt(n-1))
	}
	for post := range c.Plan.Posts {
		if title := c.Title(post); utf8.RuneCountInString(title) > d.TitleMaxChars || title == "" || strings.Contains(title, c.Vocab.Probe) {
			t.Fatalf("title of post %d: %d characters", post, utf8.RuneCountInString(title))
		}
	}
}

// TestPlan_FromDefaults: the counts follow the constants, N is search.benchmark_messages of the
// real chapter 14, every searcher sees two spaces, every space is seen by the same number of
// searchers, the probe searchers are exactly those without the probe space, the planned rate is
// BudgetShare of ratelimit.rl.search.per_minute, and the window reaches MinSamples.
func TestPlan_FromDefaults(t *testing.T) {
	t.Parallel()
	targets, err := report.LoadAll()
	if err != nil {
		t.Fatal(err)
	}
	n, err := targets.Count(searchbench.KeyBenchmarkMessages)
	if err != nil {
		t.Fatal(err)
	}
	d := defaults(t)
	plan, err := searchbench.NewPlan(searchbench.DefaultParams(), n, d)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Messages != n || plan.Channels != searchbench.Spaces*(searchbench.TextChannelsPerSpace+searchbench.ForumChannelsPerSpace) ||
		plan.Posts != searchbench.Spaces*searchbench.ForumChannelsPerSpace*searchbench.ForumPostsPerChannel {
		t.Errorf("plan counts %+v", plan.Map())
	}
	seen := make([]int, plan.Spaces)
	for k := range plan.Searchers {
		sp := plan.SearcherSpaces(k)
		if sp[0] == sp[1] {
			t.Errorf("searcher %d sees one space only", k)
		}
		seen[sp[0]]++
		seen[sp[1]]++
		visible := 0
		for c := range plan.Channels {
			if plan.Visible(k, c) {
				visible++
			}
		}
		if visible*4 != plan.Channels {
			t.Errorf("searcher %d sees %d of %d channels, want a quarter", k, visible, plan.Channels)
		}
		if slices.Contains(plan.ProbeSearchers, k) == plan.SpaceVisible(k, plan.ProbeSpace) {
			t.Errorf("searcher %d: probe searcher iff without the probe space", k)
		}
	}
	for s, v := range seen {
		if v != seen[0] {
			t.Errorf("space %d seen by %d searchers, space 0 by %d", s, v, seen[0])
		}
	}
	if want := searchbench.BudgetShare * float64(d.SearchPerMinute) / 60; plan.SearcherRate != want {
		t.Errorf("planned rate %g per second, want %g", plan.SearcherRate, want)
	}
	if plan.PlannedSamples < searchbench.MinSamples {
		t.Errorf("planned samples %g below MinSamples", plan.PlannedSamples)
	}
	for c := range plan.Channels {
		if plan.ChannelKind(c) == searchbench.ForumChannel {
			first, count := plan.ChannelPosts(c)
			for post := first; post < first+count; post++ {
				if plan.PostChannel(post) != c {
					t.Fatalf("post %d is in channel %d, not %d", post, plan.PostChannel(post), c)
				}
			}
		}
	}
}

// TestPlan_FollowsKeys: halving ratelimit.rl.search.per_minute halves the planned rate; a value
// too low for MinSamples fails naming the key.
func TestPlan_FollowsKeys(t *testing.T) {
	t.Parallel()
	d := defaults(t)
	full, err := searchbench.NewPlan(searchbench.DefaultParams(), searchbench.SmallMessages*1000, d)
	if err != nil {
		t.Fatal(err)
	}
	d.SearchPerMinute /= 2
	half, err := searchbench.NewPlan(searchbench.DefaultParams(), searchbench.SmallMessages*1000, d)
	if err != nil {
		t.Fatal(err)
	}
	if half.SearcherRate*2 != full.SearcherRate {
		t.Errorf("halved limit: rate %g, full %g", half.SearcherRate, full.SearcherRate)
	}
	d.SearchPerMinute = 1
	if _, err := searchbench.NewPlan(searchbench.DefaultParams(), searchbench.SmallMessages*1000, d); err == nil ||
		!strings.Contains(err.Error(), searchbench.KeySearchPerMinute) || !strings.Contains(err.Error(), "MinSamples") {
		t.Errorf("too low a limit: %v", err)
	}
}
