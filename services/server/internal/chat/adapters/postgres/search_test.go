// SPDX-License-Identifier: Apache-2.0

package postgres_test

import (
	"bytes"
	"context"
	"errors"
	"maps"
	"math"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/OWNER/bothy/services/server/internal/chat"
	"github.com/OWNER/bothy/services/server/internal/chat/adapters/postgres"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/searchkit"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
)

// searchRef is the reference time of the searches in these tests; every thread is created at or
// before it unless a test says otherwise.
var searchRef = time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)

// searchWorld is a small structure for the search reads: the space Support with the forums
// questions and archive, the space Marketing with the forum campaigns, and a dm channel.
type searchWorld struct {
	env
	alex                         ids.ID
	support, marketing           ids.ID
	questions, archive, campaign ids.ID
	dm                           ids.ID
}

func newSearchWorld(t *testing.T) searchWorld {
	t.Helper()
	w := searchWorld{env: newEnv(t)}
	w.alex, _ = w.person(t)
	w.support, w.marketing = ids.New(), ids.New()
	w.exec(t, `INSERT INTO spaces (id, name, sort_key, created_by) VALUES ($1, 'Support', 'a0', $2)`, w.support.String(), w.alex.String())
	w.exec(t, `INSERT INTO spaces (id, name, sort_key, created_by) VALUES ($1, 'Marketing', 'a1', $2)`, w.marketing.String(), w.alex.String())
	w.questions, w.archive, w.campaign, w.dm = ids.New(), ids.New(), ids.New(), ids.New()
	for _, c := range []struct {
		id, space ids.ID
		name      string
	}{{w.questions, w.support, "questions"}, {w.archive, w.support, "archive"}, {w.campaign, w.marketing, "campaigns"}} {
		w.exec(t, `INSERT INTO channels (id, space_id, kind, name, sort_key, history_backfill_limit, created_by)
			VALUES ($1, $2, 'forum', $3, 'a0', 20, $4)`, c.id.String(), c.space.String(), c.name, w.alex.String())
	}
	w.exec(t, `UPDATE channels SET archived_at = $2 WHERE id = $1`, w.archive.String(), t0)
	w.exec(t, `INSERT INTO channels (id, kind, name, sort_key, operator_inherit, history_backfill_limit, created_by)
		VALUES ($1, 'dm', '', 'a0', false, 20, $2)`, w.dm.String(), w.alex.String())
	return w
}

// threadSpec describes one thread row of a test.
type threadSpec struct {
	channel  ids.ID
	id       ids.ID // zero: a new id
	kind     string // "" is thread
	title    string
	at       time.Time
	activity time.Time // zero: at
	closed   bool
	operator ids.ID
	language string // "" is english
}

// thread inserts s and returns its id.
func (w searchWorld) thread(t *testing.T, s threadSpec) ids.ID {
	t.Helper()
	if s.id.IsZero() {
		s.id = ids.New()
	}
	if s.kind == "" {
		s.kind = "thread"
	}
	if s.activity.IsZero() {
		s.activity = s.at
	}
	if s.language == "" {
		s.language = "english"
	}
	state, closedAt := "open", any(nil)
	if s.closed {
		state, closedAt = "closed", s.at
	}
	var operator any
	if !s.operator.IsZero() {
		operator = s.operator.String()
	}
	w.exec(t, `INSERT INTO threads (id, channel_id, kind, title, state, closed_at, operator_agent_id, created_by, created_at,
		last_activity_at, search_language) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11::regconfig)`,
		s.id.String(), s.channel.String(), s.kind, s.title, state, closedAt, operator, w.alex.String(), s.at, s.activity, s.language)
	return s.id
}

// tag inserts a forum tag of channel and assigns it to the threads.
func (w searchWorld) tag(t *testing.T, channel ids.ID, name string, threads ...ids.ID) ids.ID {
	t.Helper()
	id := ids.New()
	w.exec(t, `INSERT INTO forum_tags (id, channel_id, name, sort_key) VALUES ($1, $2, $3, 'a0')`, id.String(), channel.String(), name)
	for _, th := range threads {
		w.exec(t, `INSERT INTO thread_tags (thread_id, tag_id, added_by) VALUES ($1, $2, $3)`, th.String(), id.String(), w.alex.String())
	}
	return id
}

// tsq builds the tsquery text of q the way the search module does (14 4.2).
func (w searchWorld) tsq(t *testing.T, q string) string {
	t.Helper()
	var s string
	if err := w.pool.QueryRow(context.Background(), `SELECT search_tsquery(ARRAY['english']::regconfig[], $1)::text`, q).Scan(&s); err != nil {
		t.Fatal(err)
	}
	return s
}

// prefix builds the prefix tsquery text of q with the statement of 14 4.9 and a minimum of three
// characters.
func (w searchWorld) prefix(t *testing.T, q string) string {
	t.Helper()
	var s string
	if err := w.pool.QueryRow(context.Background(), `SELECT string_agg(quote_literal(lexeme) || ':*', ' & ')::tsquery::text
		FROM unnest(to_tsvector('simple', $1)) WHERE length(lexeme) >= 3`, q).Scan(&s); err != nil {
		t.Fatal(err)
	}
	return s
}

// query returns a first-page query for the term invoice over channels with roomy limits and
// random markers.
func (w searchWorld) query(t *testing.T, channels ...ids.ID) searchkit.Query {
	t.Helper()
	m, err := searchkit.NewMarkers(nil)
	if err != nil {
		t.Fatal(err)
	}
	return searchkit.Query{
		TSQuery: w.tsq(t, "invoice"), ChannelIDs: channels, Sort: searchkit.SortRelevance, RefTime: searchRef, Markers: m,
		Limits: searchkit.Limits{
			MaxCandidates: 100, RankNormalization: 1, RecencyScale: 24 * time.Hour, HeadlineInputMaxChars: 1000,
			HeadlineMaxFragments: 2, HeadlineMaxWords: 30, HeadlineMinWords: 10, PageSize: 20,
		},
	}
}

// search runs SearchThreadTitles and fails the test on an error.
func (w searchWorld) search(t *testing.T, q searchkit.Query) searchkit.Page {
	t.Helper()
	p, err := w.repo.SearchThreadTitles(context.Background(), q)
	if err != nil {
		t.Fatal(err)
	}
	if p.Hits == nil {
		t.Fatal("Hits is nil; an empty page is an empty slice")
	}
	return p
}

// hitIDs returns the ids of the hits in page order.
func hitIDs(p searchkit.Page) []ids.ID {
	out := make([]ids.ID, 0, len(p.Hits))
	for _, h := range p.Hits {
		out = append(out, h.ID)
	}
	return out
}

// sameSet reports whether a and b hold the same ids, in any order.
func sameSet(a, b []ids.ID) bool {
	x, y := slices.Clone(a), slices.Clone(b)
	slices.SortFunc(x, func(p, q ids.ID) int { return bytes.Compare(p[:], q[:]) })
	slices.SortFunc(y, func(p, q ids.ID) int { return bytes.Compare(p[:], q[:]) })
	return slices.Equal(x, y)
}

// ascending reports whether list is strictly ascending by id.
func ascending(list []ids.ID) bool {
	for i := 1; i < len(list); i++ {
		if bytes.Compare(list[i-1][:], list[i][:]) >= 0 {
			return false
		}
	}
	return true
}

// TestSearchThreadTitles_VisibilityAndFilters (14 4.3, 4.4; M3-14 design notes 5 and 6): only
// threads of the given channels with a matching title are candidates (main threads with their empty
// title never match), and every filter of chat restricts them: state, any of the tags, the operator
// agent, the thread ids (nil restricts nothing, an empty list everything), the half-open interval
// After/Before on created_at and the reference time. AuthorID and HasFile do not apply to threads.
func TestSearchThreadTitles_VisibilityAndFilters(t *testing.T) {
	t.Parallel()
	w := newSearchWorld(t)
	lotse := w.agent(t, "lotse")
	day := func(n int) time.Time { return searchRef.Add(time.Duration(-n) * 24 * time.Hour) }
	open := w.thread(t, threadSpec{channel: w.questions, title: "Invoice export fails", at: day(5)})
	closed := w.thread(t, threadSpec{channel: w.questions, title: "Invoice totals wrong", at: day(4), closed: true})
	operated := w.thread(t, threadSpec{channel: w.questions, title: "Invoice run with the agent", at: day(3), operator: lotse})
	archived := w.thread(t, threadSpec{channel: w.archive, title: "Old invoice question", at: day(2)})
	w.thread(t, threadSpec{channel: w.questions, title: "Holiday plan", at: day(2)})                              // no match
	w.thread(t, threadSpec{channel: w.campaign, title: "Invoice for the campaign", at: day(1)})                   // invisible channel
	w.thread(t, threadSpec{channel: w.dm, kind: "main", at: day(1)})                                              // main thread, empty title
	w.thread(t, threadSpec{channel: w.questions, title: "Invoice from tomorrow", at: searchRef.Add(time.Second)}) // after RefTime
	bug := w.tag(t, w.questions, "bug", open)
	billing := w.tag(t, w.questions, "billing", closed)
	w.tag(t, w.questions, "other", operated)

	base := w.query(t, w.questions, w.archive, w.dm)
	cases := []struct {
		name string
		edit func(*searchkit.Query)
		want []ids.ID
	}{
		{"visible channels only", func(*searchkit.Query) {}, []ids.ID{open, closed, operated, archived}},
		{"one channel", func(q *searchkit.Query) { q.ChannelIDs = []ids.ID{w.archive} }, []ids.ID{archived}},
		{"state open", func(q *searchkit.Query) { q.Thread.State = "open" }, []ids.ID{open, operated, archived}},
		{"state closed", func(q *searchkit.Query) { q.Thread.State = "closed" }, []ids.ID{closed}},
		{"any of two tags", func(q *searchkit.Query) { q.Thread.TagIDs = []ids.ID{bug, billing} }, []ids.ID{open, closed}},
		{"one tag", func(q *searchkit.Query) { q.Thread.TagIDs = []ids.ID{billing} }, []ids.ID{closed}},
		{"unknown tag", func(q *searchkit.Query) { q.Thread.TagIDs = []ids.ID{ids.New()} }, []ids.ID{}},
		{"operator agent", func(q *searchkit.Query) { q.Thread.OperatorAgentID = &lotse }, []ids.ID{operated}},
		{"thread ids", func(q *searchkit.Query) { q.ThreadIDs = []ids.ID{closed, archived, ids.New()} }, []ids.ID{closed, archived}},
		{"empty thread ids", func(q *searchkit.Query) { q.ThreadIDs = []ids.ID{} }, []ids.ID{}},
		{"after is inclusive", func(q *searchkit.Query) { a := day(4); q.After = &a }, []ids.ID{closed, operated, archived}},
		{"before is exclusive", func(q *searchkit.Query) { b := day(4); q.Before = &b }, []ids.ID{open}},
		{"after and before", func(q *searchkit.Query) { a, b := day(4), day(2); q.After, q.Before = &a, &b }, []ids.ID{closed, operated}},
		{"reference time bounds", func(q *searchkit.Query) { q.RefTime = day(3) }, []ids.ID{open, closed, operated}},
		{"author and has_file ignored", func(q *searchkit.Query) {
			other, yes := ids.New(), true
			q.AuthorID, q.HasFile = &other, &yes
		}, []ids.ID{open, closed, operated, archived}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			q := base
			c.edit(&q)
			for _, sort := range []searchkit.Sort{searchkit.SortRelevance, searchkit.SortNewest} {
				q.Sort = sort
				p := w.search(t, q)
				if got := hitIDs(p); !sameSet(got, c.want) {
					t.Fatalf("%s: hits %v, want %v", sort, got, c.want)
				}
				if p.CandidatesCapped || p.Next != nil {
					t.Fatalf("%s: capped %v next %v", sort, p.CandidatesCapped, p.Next)
				}
				for _, h := range p.Hits {
					if h.ThreadID != h.ID || !h.MessageID.IsZero() || !slices.Contains(q.ChannelIDs, h.ChannelID) || h.CreatedAt.IsZero() {
						t.Fatalf("%s: hit %+v", sort, h)
					}
				}
			}
		})
	}
}

// TestSearchThreadTitles_CandidateCap (14 4.5 item 1, R-SU-04, M3-14 design note 6): with more
// matches than MaxCandidates only the newest MaxCandidates are reachable over every page, for both
// sorts, and CandidatesCapped is true on every page, also on an empty one after the keyset;
// exactly MaxCandidates matches are not capped.
func TestSearchThreadTitles_CandidateCap(t *testing.T) {
	t.Parallel()
	w := newSearchWorld(t)
	var all []ids.ID // newest first
	for i := range 5 {
		all = append([]ids.ID{w.thread(t, threadSpec{channel: w.questions, title: "Invoice number " + string(rune('a'+i)),
			at: searchRef.Add(time.Duration(i-10) * time.Hour)})}, all...)
	}
	for _, sort := range []searchkit.Sort{searchkit.SortRelevance, searchkit.SortNewest} {
		q := w.query(t, w.questions)
		q.Sort, q.Limits.MaxCandidates, q.Limits.PageSize = sort, 3, 2
		var got []ids.ID
		for range 4 {
			p := w.search(t, q)
			if !p.CandidatesCapped {
				t.Fatalf("%s: page %d not capped", sort, len(got))
			}
			got = append(got, hitIDs(p)...)
			if p.Next == nil {
				break
			}
			q.From = p.Next
		}
		if !sameSet(got, all[:3]) {
			t.Fatalf("%s: reachable %v, want the newest three %v", sort, got, all[:3])
		}
		// A position after the last candidate: an empty page that still reports the cap.
		last := w.search(t, q)
		q.From = &searchkit.Position{Score: -1, CreatedAt: searchRef.Add(-100 * time.Hour), ID: ids.ID{}}
		empty := w.search(t, q)
		if len(empty.Hits) != 0 || !empty.CandidatesCapped || empty.Next != nil {
			t.Fatalf("%s: page after the end %+v (last page %+v)", sort, empty, last)
		}
		q = w.query(t, w.questions)
		q.Sort, q.Limits.MaxCandidates = sort, 5
		if p := w.search(t, q); p.CandidatesCapped || len(p.Hits) != 5 {
			t.Fatalf("%s: exactly MaxCandidates matches: %+v", sort, p)
		}
	}
}

// TestSearchThreadTitles_RelevanceOrder (14 4.5 item 2, R-SU-04): the score is ts_rank divided by
// 1 + age / RecencyScale with the age from RefTime, so an equal title one RecencyScale older has
// half the score; on an equal score the higher id comes first; Next is the position of the last
// row of the page.
func TestSearchThreadTitles_RelevanceOrder(t *testing.T) {
	t.Parallel()
	w := newSearchWorld(t)
	scale := 24 * time.Hour
	older := w.thread(t, threadSpec{channel: w.questions, title: "Invoice export", at: searchRef.Add(-scale)})
	lowID, highID := ids.New(), ids.New()
	if bytes.Compare(lowID[:], highID[:]) > 0 {
		lowID, highID = highID, lowID
	}
	w.thread(t, threadSpec{channel: w.questions, id: lowID, title: "Invoice export", at: searchRef})
	w.thread(t, threadSpec{channel: w.questions, id: highID, title: "Invoice export", at: searchRef})
	q := w.query(t, w.questions)
	q.Limits.RecencyScale = scale
	p := w.search(t, q)
	if got := hitIDs(p); !slices.Equal(got, []ids.ID{highID, lowID, older}) {
		t.Fatalf("order %v, want %v", got, []ids.ID{highID, lowID, older})
	}
	if p.Hits[0].Score <= 0 || p.Hits[0].Score != p.Hits[1].Score {
		t.Fatalf("scores %v and %v, want equal and positive", p.Hits[0].Score, p.Hits[1].Score)
	}
	if ratio := p.Hits[0].Score / p.Hits[2].Score; math.Abs(ratio-2) > 1e-9 {
		t.Fatalf("score of the new title / score of the old one = %v, want 2", ratio)
	}
	q.Limits.PageSize = 2
	first := w.search(t, q)
	if first.Next == nil || first.Next.ID != lowID || first.Next.Score != p.Hits[1].Score {
		t.Fatalf("Next %+v, want the position of the second row", first.Next)
	}
	q.From = first.Next
	if second := w.search(t, q); !slices.Equal(hitIDs(second), []ids.ID{older}) || second.Next != nil {
		t.Fatalf("second page %+v", second)
	}
}

// TestSearchThreadTitles_Paging (14 4.7, M3-14 design note 6): three pages with each sort hold
// every match once, in the order of the sort (newest: created_at, then the higher id), Next is nil
// exactly on the last page, and a page after a reference time stays the same when a newer matching
// thread appears.
func TestSearchThreadTitles_Paging(t *testing.T) {
	t.Parallel()
	w := newSearchWorld(t)
	var created []ids.ID
	for i := range 7 {
		// Pairs with the same created_at test the id tie break across a page boundary.
		at := searchRef.Add(time.Duration(-(i / 2)) * time.Hour)
		created = append(created, w.thread(t, threadSpec{channel: w.questions, title: "Invoice paging " + strings.Repeat("x", i), at: at}))
	}
	for _, sort := range []searchkit.Sort{searchkit.SortRelevance, searchkit.SortNewest} {
		q := w.query(t, w.questions)
		q.Sort, q.Limits.PageSize = sort, 3
		one := w.query(t, w.questions)
		one.Sort = sort
		whole := w.search(t, one)
		var pages [][]ids.ID
		for {
			p := w.search(t, q)
			pages = append(pages, hitIDs(p))
			if len(pages) == 2 {
				// A newer matching thread after the reference time: not a candidate of later pages.
				w.thread(t, threadSpec{channel: w.questions, title: "Invoice paging late " + string(sort), at: searchRef.Add(time.Minute)})
			}
			if p.Next == nil {
				break
			}
			if len(p.Hits) != 3 || len(pages) > 3 {
				t.Fatalf("%s: page %d has %d hits", sort, len(pages), len(p.Hits))
			}
			q.From = p.Next
		}
		if len(pages) != 3 || len(pages[2]) != 1 {
			t.Fatalf("%s: pages %v", sort, pages)
		}
		got := slices.Concat(pages...)
		if !slices.Equal(got, hitIDs(whole)) || !sameSet(got, created) {
			t.Fatalf("%s: paged %v, one page %v", sort, got, hitIDs(whole))
		}
		for i := 1; i < len(whole.Hits); i++ {
			a, b := whole.Hits[i-1], whole.Hits[i]
			var ok bool
			if sort == searchkit.SortNewest {
				ok = a.Score == 0 && b.Score == 0 && (a.CreatedAt.After(b.CreatedAt) || a.CreatedAt.Equal(b.CreatedAt) && bytes.Compare(a.ID[:], b.ID[:]) > 0)
			} else {
				ok = a.Score > b.Score || a.Score == b.Score && bytes.Compare(a.ID[:], b.ID[:]) > 0
			}
			if !ok {
				t.Fatalf("%s: %+v before %+v", sort, a, b)
			}
		}
	}
}

// TestSearchThreadTitles_Headline (14 4.6, R-SU-06): the headline of a hit is the title split at
// the random markers of the query: the matched word is a match segment, HTML and entities in the
// title stay text, the texts joined give the title, and the input is cut at HeadlineInputMaxChars.
func TestSearchThreadTitles_Headline(t *testing.T) {
	t.Parallel()
	w := newSearchWorld(t)
	title := "Fix <b>bold</b> &lt; invoice export"
	w.thread(t, threadSpec{channel: w.questions, title: title, at: searchRef})
	q := w.query(t, w.questions)
	p := w.search(t, q)
	if len(p.Hits) != 1 {
		t.Fatalf("hits %+v", p.Hits)
	}
	segs := p.Hits[0].Segments
	var joined strings.Builder
	var matches []string
	for _, s := range segs {
		joined.WriteString(s.Text)
		if s.Match {
			matches = append(matches, s.Text)
		}
		if strings.Contains(s.Text, q.Markers.Start) || strings.Contains(s.Text, q.Markers.Stop) {
			t.Fatalf("a marker left in %q", s.Text)
		}
	}
	// ts_headline drops the tag tokens of its parser and short words at the edges of a fragment,
	// but it adds nothing and decodes nothing: every word of the headline is a word of the title in
	// the same order, and the entity stays as written.
	if !slices.Equal(matches, []string{"invoice"}) || !strings.Contains(joined.String(), "&lt;") || !inOrder(joined.String(), title) {
		t.Fatalf("segments %+v, want words of the title in order with the match invoice", segs)
	}
	zero := q
	zero.Limits.PageSize = 0
	if _, err := w.repo.SearchThreadTitles(context.Background(), zero); err == nil {
		t.Fatal("a page size of zero was accepted")
	}
	q.Limits.HeadlineInputMaxChars = len("Fix <b>bold</b>")
	cut := w.search(t, q)
	var cutText strings.Builder
	for _, s := range cut.Hits[0].Segments {
		cutText.WriteString(s.Text)
		if s.Match {
			t.Fatalf("a match outside the cut input: %+v", cut.Hits[0].Segments)
		}
	}
	if got := cutText.String(); !inOrder(got, title[:q.Limits.HeadlineInputMaxChars]) || strings.Contains(got, "invoice") {
		t.Fatalf("cut headline %q, want words of the first %d characters only", got, q.Limits.HeadlineInputMaxChars)
	}
	if cut.Hits[0].Segments == nil {
		t.Fatal("segments nil")
	}
}

// TestPrefixThreadTitles (14 4.9): the prefix tsquery "rechnungs okt" finds "Rechnungslauf
// Oktober"; only threads of the given channels; newest last_activity_at first, then the higher id;
// at most Limit.
func TestPrefixThreadTitles(t *testing.T) {
	t.Parallel()
	w := newSearchWorld(t)
	old := w.thread(t, threadSpec{channel: w.questions, title: "Rechnungslauf Oktober", at: t0, activity: t0})
	lowID, highID := ids.New(), ids.New()
	if bytes.Compare(lowID[:], highID[:]) > 0 {
		lowID, highID = highID, lowID
	}
	later := t0.Add(time.Hour)
	w.thread(t, threadSpec{channel: w.archive, id: lowID, title: "Rechnungsplanung Oktober", at: t0, activity: later})
	w.thread(t, threadSpec{channel: w.archive, id: highID, title: "Oktober Rechnungsabschluss", at: t0, activity: later})
	w.thread(t, threadSpec{channel: w.campaign, title: "Rechnungslauf Oktober Marketing", at: t0, activity: later.Add(time.Hour)})
	w.thread(t, threadSpec{channel: w.questions, title: "Rechnungslauf November", at: t0})
	q := searchkit.PrefixQuery{TSQuery: w.prefix(t, "rechnungs okt"), ChannelIDs: []ids.ID{w.questions, w.archive}, Limit: 10}
	got, err := w.repo.PrefixThreadTitles(context.Background(), q)
	if err != nil {
		t.Fatal(err)
	}
	want := []searchkit.TitleHit{
		{ThreadID: highID, ChannelID: w.archive, Title: "Oktober Rechnungsabschluss", LastActivityAt: later},
		{ThreadID: lowID, ChannelID: w.archive, Title: "Rechnungsplanung Oktober", LastActivityAt: later},
		{ThreadID: old, ChannelID: w.questions, Title: "Rechnungslauf Oktober", LastActivityAt: t0},
	}
	if len(got) != len(want) {
		t.Fatalf("titles %+v, want %+v", got, want)
	}
	for i := range want {
		if got[i].ThreadID != want[i].ThreadID || got[i].ChannelID != want[i].ChannelID || got[i].Title != want[i].Title ||
			!got[i].LastActivityAt.Equal(want[i].LastActivityAt) {
			t.Fatalf("title %d = %+v, want %+v", i, got[i], want[i])
		}
	}
	q.Limit = 1
	if one, err := w.repo.PrefixThreadTitles(context.Background(), q); err != nil || len(one) != 1 || one[0].ThreadID != highID {
		t.Fatalf("limit 1: %+v %v", one, err)
	}
}

// TestThreadsMatching (07 7.4 rule 6, M3-14 design note 5): the ids of the threads of the given
// channels that match the thread filter, ascending; a zero filter matches every thread, main
// threads included; the operator filter never matches a main thread.
func TestThreadsMatching(t *testing.T) {
	t.Parallel()
	w := newSearchWorld(t)
	lotse := w.agent(t, "lotse")
	main := w.thread(t, threadSpec{channel: w.dm, kind: "main", at: t0})
	a := w.thread(t, threadSpec{channel: w.questions, title: "A", at: t0, operator: lotse})
	b := w.thread(t, threadSpec{channel: w.questions, title: "B", at: t0, closed: true})
	c := w.thread(t, threadSpec{channel: w.archive, title: "C", at: t0})
	w.thread(t, threadSpec{channel: w.campaign, title: "D", at: t0, operator: lotse})
	bug, billing := w.tag(t, w.questions, "bug", a), w.tag(t, w.questions, "billing", b)
	channels := []ids.ID{w.questions, w.archive, w.dm}
	for _, x := range []struct {
		name string
		f    searchkit.ThreadFilter
		want []ids.ID
	}{
		{"zero filter", searchkit.ThreadFilter{}, []ids.ID{main, a, b, c}},
		{"closed", searchkit.ThreadFilter{State: "closed"}, []ids.ID{b}},
		{"any of the tags", searchkit.ThreadFilter{TagIDs: []ids.ID{bug, billing}}, []ids.ID{a, b}},
		{"operator", searchkit.ThreadFilter{OperatorAgentID: &lotse}, []ids.ID{a}},
		{"tag and state", searchkit.ThreadFilter{TagIDs: []ids.ID{bug, billing}, State: "open"}, []ids.ID{a}},
	} {
		got, err := w.repo.ThreadsMatching(context.Background(), channels, x.f)
		if err != nil {
			t.Fatal(err)
		}
		if !sameSet(got, x.want) || !ascending(got) {
			t.Fatalf("%s: %v, want %v ascending", x.name, got, x.want)
		}
	}
}

// TestChannelsInSpaces: the channels of the given spaces, archived ones included, ascending; other
// spaces and the dm channel (no space) are absent.
func TestChannelsInSpaces(t *testing.T) {
	t.Parallel()
	w := newSearchWorld(t)
	got, err := w.repo.ChannelsInSpaces(context.Background(), []ids.ID{w.support, ids.New()})
	if err != nil {
		t.Fatal(err)
	}
	if !sameSet(got, []ids.ID{w.questions, w.archive}) || !ascending(got) {
		t.Fatalf("channels %v", got)
	}
	both, err := w.repo.ChannelsInSpaces(context.Background(), []ids.ID{w.support, w.marketing})
	if err != nil || !sameSet(both, []ids.ID{w.questions, w.archive, w.campaign}) {
		t.Fatalf("channels %v %v", both, err)
	}
}

// TestLabels (14 11, M3-14 design note 9): channel and space names only for channels whose space
// is visible (a dm channel has none), thread titles only for threads of the given channels with a
// title; the maps are never nil.
func TestLabels(t *testing.T) {
	t.Parallel()
	w := newSearchWorld(t)
	inQuestions := w.thread(t, threadSpec{channel: w.questions, title: "Invoice export", at: t0})
	inCampaign := w.thread(t, threadSpec{channel: w.campaign, title: "Campaign plan", at: t0})
	main := w.thread(t, threadSpec{channel: w.dm, kind: "main", at: t0})
	l, err := w.repo.Labels(context.Background(), []ids.ID{w.support}, []ids.ID{w.questions, w.campaign, w.dm},
		[]ids.ID{inQuestions, inCampaign, main, ids.New()})
	if err != nil {
		t.Fatal(err)
	}
	wantChannels := map[ids.ID]chat.ChannelLabel{w.questions: {Name: "questions", SpaceName: "Support"}}
	if !maps.Equal(l.Channels, wantChannels) {
		t.Fatalf("channels %+v, want %+v", l.Channels, wantChannels)
	}
	wantTitles := map[ids.ID]string{inQuestions: "Invoice export", inCampaign: "Campaign plan"}
	if !maps.Equal(l.ThreadTitles, wantTitles) {
		t.Fatalf("titles %+v, want %+v", l.ThreadTitles, wantTitles)
	}
	// A thread outside the given channels has no title label.
	l, err = w.repo.Labels(context.Background(), []ids.ID{w.support, w.marketing}, []ids.ID{w.questions}, []ids.ID{inCampaign})
	if err != nil || len(l.ThreadTitles) != 0 || l.ThreadTitles == nil || len(l.Channels) != 1 {
		t.Fatalf("labels %+v %v", l, err)
	}
	none, err := w.repo.Labels(context.Background(), nil, nil, nil)
	if err != nil || none.Channels == nil || none.ThreadTitles == nil {
		t.Fatalf("empty labels %+v %v", none, err)
	}
}

// TestThreadsIn: the store read of HitThreads returns the threads among the ids that lie in the
// given channels, with every column of the thread representation.
func TestThreadsIn(t *testing.T) {
	t.Parallel()
	w := newSearchWorld(t)
	lotse := w.agent(t, "lotse")
	a := w.thread(t, threadSpec{channel: w.questions, title: "Invoice export", at: t0, operator: lotse, closed: true})
	b := w.thread(t, threadSpec{channel: w.campaign, title: "Campaign plan", at: t0})
	got, err := w.repo.ThreadsIn(context.Background(), []ids.ID{b, a, ids.New()}, []ids.ID{w.questions})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("threads %+v", got)
	}
	want, found, err := w.repo.Thread(context.Background(), a)
	if err != nil || !found || got[0] != want {
		t.Fatalf("thread %+v, want the row of Thread %+v (%v)", got[0], want, err)
	}
}

// TestSearchReads_MapStatementTimeout (M3-14 Tests; 14 7): a search read that PostgreSQL cancels
// at the statement_timeout of the unit gives searchkit.ErrTimeout, a cancelled context gives the
// context error, and the mapping itself turns SQLSTATE 57014 into ErrTimeout only while the context
// is alive. The read waits on an ACCESS EXCLUSIVE lock another transaction holds, so the deadline
// fires deterministically without pg_sleep in the module's SQL.
func TestSearchReads_MapStatementTimeout(t *testing.T) {
	t.Parallel()
	w := newSearchWorld(t)
	ctx := context.Background()
	q := w.query(t, w.questions)
	reads := map[string]func(ctx context.Context) error{
		"SearchThreadTitles": func(ctx context.Context) error { _, err := w.repo.SearchThreadTitles(ctx, q); return err },
		"PrefixThreadTitles": func(ctx context.Context) error {
			_, err := w.repo.PrefixThreadTitles(ctx, searchkit.PrefixQuery{TSQuery: w.prefix(t, "invoice"), ChannelIDs: q.ChannelIDs, Limit: 5})
			return err
		},
		"ThreadsMatching": func(ctx context.Context) error {
			_, err := w.repo.ThreadsMatching(ctx, q.ChannelIDs, searchkit.ThreadFilter{State: "open"})
			return err
		},
	}
	for name, read := range reads {
		t.Run(name, func(t *testing.T) {
			tx, err := w.pool.Begin(ctx, pgx.TxOptions{})
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = tx.Rollback(ctx) }()
			if _, err := tx.Exec(ctx, "LOCK TABLE threads IN ACCESS EXCLUSIVE MODE"); err != nil {
				t.Fatal(err)
			}
			var readErr error
			_ = w.uow.Do(ctx, func(ctx context.Context) error {
				readErr = read(ctx)
				return readErr
			}, txn.StatementTimeout(50*time.Millisecond))
			if !errors.Is(readErr, searchkit.ErrTimeout) {
				t.Fatalf("err = %v, want searchkit.ErrTimeout", readErr)
			}
			if err := tx.Rollback(ctx); err != nil {
				t.Fatal(err)
			}
			cctx, cancel := context.WithCancel(ctx)
			cancel()
			if err := read(cctx); !errors.Is(err, context.Canceled) || errors.Is(err, searchkit.ErrTimeout) {
				t.Fatalf("cancelled context: %v, want context.Canceled", err)
			}
		})
	}

	timeout := &pgconn.PgError{Code: "57014"}
	if err := postgres.SearchError(ctx, timeout, "x"); !errors.Is(err, searchkit.ErrTimeout) {
		t.Fatalf("57014 with a live context: %v", err)
	}
	cctx, cancel := context.WithCancel(ctx)
	cancel()
	if err := postgres.SearchError(cctx, timeout, "x"); !errors.Is(err, context.Canceled) {
		t.Fatalf("57014 with a cancelled context: %v", err)
	}
	if err := postgres.SearchError(ctx, &pgconn.PgError{Code: "42P01"}, "x"); err == nil || errors.Is(err, searchkit.ErrTimeout) {
		t.Fatalf("another database error: %v, want internal", err)
	} else if e, ok := errs.As(err); !ok || e.Code != errs.CodeInternalError {
		t.Fatalf("another database error: %v, want internal", err)
	}
	if postgres.SearchError(ctx, nil, "x") != nil {
		t.Fatal("nil error mapped")
	}
}

// inOrder reports whether the words of got appear in want in the same order.
func inOrder(got, want string) bool {
	for _, w := range strings.Fields(got) {
		i := strings.Index(want, w)
		if i < 0 {
			return false
		}
		want = want[i+len(w):]
	}
	return true
}
