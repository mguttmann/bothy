// SPDX-License-Identifier: Apache-2.0

package postgres_test

import (
	"bytes"
	"context"
	"errors"
	"math"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/searchkit"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
	"github.com/OWNER/bothy/services/server/internal/messaging/adapters/postgres"
	"github.com/OWNER/bothy/services/server/internal/messaging/domain"
	"github.com/OWNER/bothy/services/server/internal/platform/pg"
	"github.com/OWNER/bothy/services/server/internal/platform/testkit"
	mw "github.com/OWNER/bothy/services/server/test/messagingworld"
)

// The search tests of M3-14 (docs/spec/14-search.md sections 4.3 to 4.8) run SearchMessages of
// the repository on a real database. The rows are written with InsertMessage, so created_at,
// author, language and has_attachments are under the test's control.

// searchStart is the creation time of the first row of a search test.
var searchStart = time.Date(2026, 10, 1, 9, 0, 0, 0, time.UTC)

// searchWorld is a world with the repository and its unit of work.
type searchWorld struct {
	w    *mw.World
	repo *postgres.Repository
	uow  *pg.UnitOfWork
	seq  map[ids.ID]int64
}

func newSearchWorld(t *testing.T) *searchWorld {
	t.Helper()
	w := mw.New(t, func(reg *txn.Registry, rec *mw.Recorder) error { return txn.Subscribe(reg, posted{rec}.onPosted) })
	reg := txn.NewRegistry()
	reg.Seal()
	uow := pg.NewUnitOfWork(w.Pool, pg.UnitConfig{StatementTimeout: 30 * time.Second, LockTimeout: 10 * time.Second, IdleInTxTimeout: time.Minute}, reg)
	return &searchWorld{w: w, repo: postgres.NewRepository(w.Pool), uow: uow, seq: map[ids.ID]int64{}}
}

// row describes one message of a search test.
type row struct {
	id              ids.ID // zero: a new id
	thread, channel ids.ID
	author          ids.ID // zero: the owner
	content         string
	at              time.Time
	hasFile         bool
}

// add writes r as a text message in English and returns its id.
func (s *searchWorld) add(t *testing.T, r row) ids.ID {
	t.Helper()
	if r.id.IsZero() {
		r.id = ids.New()
	}
	if r.author.IsZero() {
		r.author = s.w.OwnerID
	}
	s.seq[r.thread]++
	m := domain.Message{
		ID: r.id, ThreadID: r.thread, ChannelID: r.channel, ThreadSeq: s.seq[r.thread], AuthorID: r.author, ClientMsgID: ids.New(),
		PayloadSHA256: []byte{1}, Origin: domain.OriginHuman, Kind: domain.KindText, ContentMD: r.content,
		ContentFormat: domain.ContentFormat, HasAttachments: r.hasFile, Version: 1, CreatedAt: r.at, SearchLanguage: "english",
	}
	if err := s.uow.Do(context.Background(), func(ctx context.Context) error {
		_, err := s.repo.InsertMessage(ctx, m)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	return r.id
}

// tsquery builds the query text the search module builds for "invoice" (14 4.2): search_tsquery
// over the languages of the chapter defaults.
func (s *searchWorld) tsquery(t *testing.T) string {
	t.Helper()
	var tsq string
	if err := s.w.Pool.QueryRow(context.Background(), `SELECT search_tsquery(ARRAY['german','english']::regconfig[], $1)::text`, "invoice").Scan(&tsq); err != nil {
		t.Fatal(err)
	}
	return tsq
}

// search runs q and fails the test on an error.
func (s *searchWorld) search(t *testing.T, q searchkit.Query) searchkit.Page {
	t.Helper()
	page, err := s.repo.SearchMessages(context.Background(), q)
	if err != nil {
		t.Fatalf("SearchMessages: %v", err)
	}
	if page.Hits == nil {
		t.Fatal("SearchMessages returned nil hits")
	}
	return page
}

// query returns a search for tsq in channels, sorted newest, with the chapter defaults as limits
// and a page large enough for every row of a test.
func query(t *testing.T, tsq string, ref time.Time, channels ...ids.ID) searchkit.Query {
	t.Helper()
	m, err := searchkit.NewMarkers(nil)
	if err != nil {
		t.Fatal(err)
	}
	return searchkit.Query{
		TSQuery: tsq, ChannelIDs: channels, Sort: searchkit.SortNewest, RefTime: ref, Markers: m,
		Limits: searchkit.Limits{
			MaxCandidates: 100, RankNormalization: 1, RecencyScale: 30 * 24 * time.Hour, HeadlineInputMaxChars: 20000,
			HeadlineMaxFragments: 2, HeadlineMaxWords: 30, HeadlineMinWords: 10, PageSize: 50,
		},
	}
}

// hitIDs returns the ids of the hits in page order.
func hitIDs(p searchkit.Page) []ids.ID {
	out := make([]ids.ID, 0, len(p.Hits))
	for _, h := range p.Hits {
		out = append(out, h.ID)
	}
	return out
}

// positionOf is the keyset position of h under sort.
func positionOf(h searchkit.Hit, sort searchkit.Sort) searchkit.Position {
	if sort == searchkit.SortNewest {
		return searchkit.Position{CreatedAt: h.CreatedAt, ID: h.ID}
	}
	return searchkit.Position{Score: h.Score, ID: h.ID}
}

// TestSearchMessages_VisibilityAndFilters (14 4.3 item 2, 4.4, brief design notes 6 and 15): only
// rows of q.ChannelIDs appear, never a deleted one and never one created after the reference
// time; each filter (ThreadIDs, AuthorID, the half-open After and Before, HasFile both ways)
// keeps exactly its rows; a non-nil empty ThreadIDs keeps none and an empty channel set answers
// an empty page without an error.
func TestSearchMessages_VisibilityAndFilters(t *testing.T) {
	t.Parallel()
	s := newSearchWorld(t)
	space, chA, mainA := s.w.TextChannel()
	chB := s.w.Channel(space, "text", "billing")
	mainB := s.w.MainThread(chB)
	chC := s.w.Channel(space, "text", "hidden")
	mainC := s.w.MainThread(chC)
	sam := s.w.Person("sam", false)
	ref := searchStart.Add(6 * time.Hour)
	h := func(n int) time.Time { return searchStart.Add(time.Duration(n) * time.Hour) }

	a1 := s.add(t, row{thread: mainA, channel: chA, content: "the invoice run of Alex", at: h(0)})
	a2 := s.add(t, row{thread: mainA, channel: chA, author: sam, content: "an invoice for Robin", at: h(1), hasFile: true})
	deleted := s.add(t, row{thread: mainA, channel: chA, content: "a deleted invoice", at: h(2)})
	b1 := s.add(t, row{thread: mainB, channel: chB, author: sam, content: "invoice in billing", at: h(3)})
	c1 := s.add(t, row{thread: mainC, channel: chC, content: "invoice in a hidden channel", at: h(4)})
	s.add(t, row{thread: mainA, channel: chA, content: "nothing to find here", at: h(5)})
	atRef := s.add(t, row{thread: mainA, channel: chA, content: "invoice at the reference time", at: ref})
	s.add(t, row{thread: mainA, channel: chA, content: "invoice after the reference time", at: ref.Add(time.Millisecond)})
	// A deleted message keeps no content (the messages check of M1-58), so it never matches.
	s.w.Tombstone(deleted)

	tsq := s.tsquery(t)
	yes, no := true, false
	after, before := h(1), h(1)
	for _, c := range []struct {
		name string
		edit func(*searchkit.Query)
		want []ids.ID
	}{
		{"visible channels only", func(*searchkit.Query) {}, []ids.ID{atRef, b1, a2, a1}},
		{"the hidden channel alone", func(q *searchkit.Query) { q.ChannelIDs = []ids.ID{chC} }, []ids.ID{c1}},
		{"no channel", func(q *searchkit.Query) { q.ChannelIDs = nil }, []ids.ID{}},
		{"one thread", func(q *searchkit.Query) { q.ThreadIDs = []ids.ID{mainA} }, []ids.ID{atRef, a2, a1}},
		{"a thread of a hidden channel", func(q *searchkit.Query) { q.ThreadIDs = []ids.ID{mainC} }, []ids.ID{}},
		{"an empty thread list", func(q *searchkit.Query) { q.ThreadIDs = []ids.ID{} }, []ids.ID{}},
		{"author", func(q *searchkit.Query) { q.AuthorID = &sam }, []ids.ID{b1, a2}},
		{"after is inclusive", func(q *searchkit.Query) { q.After = &after }, []ids.ID{atRef, b1, a2}},
		{"before is exclusive", func(q *searchkit.Query) { q.Before = &before }, []ids.ID{a1}},
		{"with a file", func(q *searchkit.Query) { q.HasFile = &yes }, []ids.ID{a2}},
		{"without a file", func(q *searchkit.Query) { q.HasFile = &no }, []ids.ID{atRef, b1, a1}},
	} {
		for _, sort := range []searchkit.Sort{searchkit.SortNewest, searchkit.SortRelevance} {
			q := query(t, tsq, ref, chA, chB)
			q.Sort = sort
			c.edit(&q)
			page := s.search(t, q)
			got := hitIDs(page)
			want := c.want
			if sort == searchkit.SortRelevance {
				// Equal ranks: the score orders like newest, so the sets compare in the same order.
				slices.SortFunc(got, func(a, b ids.ID) int { return bytes.Compare(a[:], b[:]) })
				want = slices.Clone(want)
				slices.SortFunc(want, func(a, b ids.ID) int { return bytes.Compare(a[:], b[:]) })
			}
			if !slices.Equal(got, want) || page.CandidatesCapped || page.Next != nil {
				t.Errorf("%s (%s): hits %v capped %v next %v, want %v", c.name, sort, got, page.CandidatesCapped, page.Next, want)
			}
			for _, hit := range page.Hits {
				if !slices.Contains(q.ChannelIDs, hit.ChannelID) || !hit.MessageID.IsZero() || hit.CreatedAt.After(ref) {
					t.Errorf("%s (%s): hit %+v outside the query", c.name, sort, hit)
				}
			}
		}
	}
}

// TestSearchMessages_CandidateCap (14 4.5 item 1, R-SU-04): with more matches than
// search.max_candidates only the newest are reachable, for both sorts, and CandidatesCapped is
// true, also on an empty page after the last candidate; exactly the limit is not capped.
func TestSearchMessages_CandidateCap(t *testing.T) {
	t.Parallel()
	s := newSearchWorld(t)
	_, ch, main := s.w.TextChannel()
	var rows []ids.ID // oldest first
	for i := range 5 {
		rows = append(rows, s.add(t, row{thread: main, channel: ch, content: "invoice", at: searchStart.Add(time.Duration(i) * time.Minute)}))
	}
	ref := searchStart.Add(time.Hour)
	tsq := s.tsquery(t)
	newest3 := []ids.ID{rows[4], rows[3], rows[2]}
	for _, sort := range []searchkit.Sort{searchkit.SortNewest, searchkit.SortRelevance} {
		for _, c := range []struct {
			max    int
			want   []ids.ID
			capped bool
		}{
			{3, newest3, true},
			{4, []ids.ID{rows[4], rows[3], rows[2], rows[1]}, true},
			{5, []ids.ID{rows[4], rows[3], rows[2], rows[1], rows[0]}, false},
		} {
			q := query(t, tsq, ref, ch)
			q.Sort, q.Limits.MaxCandidates = sort, c.max
			page := s.search(t, q)
			if got := hitIDs(page); !slices.Equal(got, c.want) || page.CandidatesCapped != c.capped {
				t.Errorf("%s with %d candidates: hits %v capped %v, want %v capped %v", sort, c.max, got, page.CandidatesCapped, c.want, c.capped)
			}
		}
		// After the last candidate the page is empty and still reports the limit.
		q := query(t, tsq, ref, ch)
		q.Sort, q.Limits.MaxCandidates, q.Limits.PageSize = sort, 3, 3
		first := s.search(t, q)
		last := positionOf(first.Hits[len(first.Hits)-1], sort)
		q.From = &last
		empty := s.search(t, q)
		if len(empty.Hits) != 0 || empty.Next != nil || !empty.CandidatesCapped || first.Next != nil {
			t.Errorf("%s after the last candidate: %+v (first page next %v), want an empty capped page", sort, empty, first.Next)
		}
	}
}

// TestSearchMessages_RelevanceOrder (14 4.5 item 2): equal texts rank the newer first by the age
// from the reference time (score = rank / (1 + age / search.recency_scale)), equal scores rank the
// higher id first, and newest orders by created_at, then id, with score 0.
func TestSearchMessages_RelevanceOrder(t *testing.T) {
	t.Parallel()
	s := newSearchWorld(t)
	_, ch, main := s.w.TextChannel()
	day := 24 * time.Hour
	old := s.add(t, row{thread: main, channel: ch, content: "invoice", at: searchStart})
	recent := s.add(t, row{thread: main, channel: ch, content: "invoice", at: searchStart.Add(day)})
	tieA, tieB := ids.New(), ids.New()
	if bytes.Compare(tieA[:], tieB[:]) > 0 {
		tieA, tieB = tieB, tieA
	}
	// tieB is the higher id; both rows have the same text and time, so the same score.
	s.add(t, row{id: tieB, thread: main, channel: ch, content: "invoice", at: searchStart.Add(2 * day)})
	s.add(t, row{id: tieA, thread: main, channel: ch, content: "invoice", at: searchStart.Add(2 * day)})
	ref := searchStart.Add(3 * day)
	tsq := s.tsquery(t)

	q := query(t, tsq, ref, ch)
	q.Sort = searchkit.SortRelevance
	page := s.search(t, q)
	if got, want := hitIDs(page), []ids.ID{tieB, tieA, recent, old}; !slices.Equal(got, want) {
		t.Fatalf("relevance order %v, want %v", got, want)
	}
	sc := func(i int) float64 { return page.Hits[i].Score }
	if sc(0) != sc(1) || sc(3) <= 0 || sc(2) <= sc(3) || sc(1) <= sc(2) {
		t.Errorf("scores %v %v %v %v: want equal ties and falling with age", sc(0), sc(1), sc(2), sc(3))
	}
	// The same rank divided by 1 + age / scale: the ratio depends only on the two ages.
	scale := q.Limits.RecencyScale.Hours()
	wantRatio := (1 + ref.Sub(searchStart.Add(day)).Hours()/scale) / (1 + ref.Sub(searchStart).Hours()/scale)
	if ratio := sc(3) / sc(2); math.Abs(ratio-wantRatio) > 1e-9 {
		t.Errorf("score(old)/score(recent) = %v, want %v from the reference time", ratio, wantRatio)
	}
	// A later reference time lowers every score: the age is measured from it, never from now().
	later := q
	later.RefTime = ref.Add(30 * day)
	if p := s.search(t, later); p.Hits[3].Score >= sc(3) {
		t.Errorf("score with a later reference time %v, want below %v", p.Hits[3].Score, sc(3))
	}

	q.Sort = searchkit.SortNewest
	page = s.search(t, q)
	if got, want := hitIDs(page), []ids.ID{tieB, tieA, recent, old}; !slices.Equal(got, want) {
		t.Fatalf("newest order %v, want %v", got, want)
	}
	for _, h := range page.Hits {
		if h.Score != 0 {
			t.Errorf("newest hit %s has score %v, want 0", h.ID, h.Score)
		}
	}
}

// TestSearchMessages_KeysetPaging (14 4.7): three pages of both sorts over rows with equal times
// and scores give every row once, in the order of one large page; Next is the position of the
// last hit of a page and nil on the last page.
func TestSearchMessages_KeysetPaging(t *testing.T) {
	t.Parallel()
	s := newSearchWorld(t)
	_, ch, main := s.w.TextChannel()
	for _, minute := range []int{0, 1, 1, 2, 3, 3, 4} {
		s.add(t, row{thread: main, channel: ch, content: "invoice", at: searchStart.Add(time.Duration(minute) * time.Minute)})
	}
	ref := searchStart.Add(time.Hour)
	tsq := s.tsquery(t)
	for _, sort := range []searchkit.Sort{searchkit.SortNewest, searchkit.SortRelevance} {
		q := query(t, tsq, ref, ch)
		q.Sort = sort
		want := hitIDs(s.search(t, q))
		if len(want) != 7 {
			t.Fatalf("%s: one page has %d hits, want 7", sort, len(want))
		}
		q.Limits.PageSize = 3
		var got []ids.ID
		for i, size := range []int{3, 3, 1} {
			page := s.search(t, q)
			if len(page.Hits) != size {
				t.Fatalf("%s page %d: %d hits, want %d", sort, i+1, len(page.Hits), size)
			}
			got = append(got, hitIDs(page)...)
			if i == 2 {
				if page.Next != nil {
					t.Errorf("%s: the last page has next %+v", sort, page.Next)
				}
				break
			}
			if page.Next == nil || *page.Next != positionOf(page.Hits[len(page.Hits)-1], sort) {
				t.Fatalf("%s page %d: next %+v, want the position of its last hit", sort, i+1, page.Next)
			}
			q.From = page.Next
		}
		if !slices.Equal(got, want) {
			t.Errorf("%s: pages %v, want %v", sort, got, want)
		}
	}
}

// TestSearchMessages_Headline (14 4.6, R-SU-06): the page carries segments split at the random
// markers. Markup in the content never becomes markup of the excerpt: ts_headline drops the HTML
// tags of the content (its parser reads them as tags) and keeps entities and a marker look-alike
// as text; the words of the segments are the words of the content in order, only the term
// matches, and a term behind search.headline_input_max_chars gives no match segment.
func TestSearchMessages_Headline(t *testing.T) {
	t.Parallel()
	s := newSearchWorld(t)
	_, ch, main := s.w.TextChannel()
	lookAlike := strings.Repeat("ab", 16)
	html := "Sam wrote <b>bold</b> &lt;tag&gt; " + lookAlike + " about the invoice of Alex"
	filler := strings.Repeat("filler ", 10) + "invoice"
	htmlID := s.add(t, row{thread: main, channel: ch, content: html, at: searchStart})
	cutID := s.add(t, row{thread: main, channel: ch, content: filler, at: searchStart.Add(time.Minute)})
	q := query(t, s.tsquery(t), searchStart.Add(time.Hour), ch)

	// segments returns the segments of the hit id of a search with the headline input max.
	segments := func(id ids.ID, inputMax int) []searchkit.Segment {
		t.Helper()
		q.Limits.HeadlineInputMaxChars = inputMax
		for _, h := range s.search(t, q).Hits {
			if h.ID == id {
				return h.Segments
			}
		}
		t.Fatalf("no hit %s", id)
		return nil
	}
	check := func(what, content string, segs []searchkit.Segment, wantMatch bool) {
		t.Helper()
		if len(segs) == 0 {
			t.Fatalf("%s: no segments", what)
		}
		rest, matches := content, 0
		var joined strings.Builder
		for _, seg := range segs {
			joined.WriteString(seg.Text)
			if strings.Contains(seg.Text, q.Markers.Start) || strings.Contains(seg.Text, q.Markers.Stop) ||
				strings.ContainsAny(seg.Text, "<>") {
				t.Errorf("%s: segment %q carries a marker or markup", what, seg.Text)
			}
			for _, word := range strings.Fields(seg.Text) {
				i := strings.Index(rest, word)
				if i < 0 {
					t.Errorf("%s: word %q of the segments is not the content in order (%q)", what, word, content)
					break
				}
				rest = rest[i+len(word):]
			}
			if seg.Match {
				matches++
				if !strings.EqualFold(seg.Text, "invoice") {
					t.Errorf("%s: match %q is not the term", what, seg.Text)
				}
			}
		}
		if (matches > 0) != wantMatch {
			t.Errorf("%s: %d match segments in %+v, want a match %v", what, matches, segs, wantMatch)
		}
		if wantMatch && content == html && (!strings.Contains(joined.String(), "&lt;tag&gt;") || !strings.Contains(joined.String(), lookAlike)) {
			t.Errorf("%s: excerpt %q lost the entity or the look-alike", what, joined.String())
		}
	}
	check("HTML content", html, segments(htmlID, 20000), true)
	check("term behind the headline input", filler, segments(cutID, 30), false)
	check("term inside the headline input", filler, segments(cutID, 20000), true)
}

// TestSearchMessages_StatementTimeout (14 section 7, brief design note 6): a real statement that
// waits on a lock beyond the statement timeout of its unit gives searchkit.ErrTimeout, and a
// caller that cancels the waiting statement gets the context error, not ErrTimeout.
func TestSearchMessages_StatementTimeout(t *testing.T) {
	t.Parallel()
	s := newSearchWorld(t)
	_, ch, main := s.w.TextChannel()
	s.add(t, row{thread: main, channel: ch, content: "invoice", at: searchStart})
	q := query(t, s.tsquery(t), searchStart.Add(time.Hour), ch)
	ctx := context.Background()
	// The owner holds ACCESS EXCLUSIVE on messages, so the search statement waits until its
	// statement timeout (the lock timeout of the unit is longer).
	lock, err := s.w.Owner.Begin(ctx, pgx.TxOptions{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = lock.Rollback(context.Background()) })
	if _, err := lock.Exec(ctx, `LOCK TABLE messages IN ACCESS EXCLUSIVE MODE`); err != nil {
		t.Fatal(err)
	}
	err = s.uow.Do(ctx, func(ctx context.Context) error {
		_, err := s.repo.SearchMessages(ctx, q)
		return err
	}, txn.StatementTimeout(50*time.Millisecond))
	if !errors.Is(err, searchkit.ErrTimeout) {
		t.Fatalf("search under the lock = %v, want searchkit.ErrTimeout", err)
	}

	cctx, cancel := context.WithCancel(ctx)
	t.Cleanup(cancel)
	done := make(chan error, 1)
	go func() { _, err := s.repo.SearchMessages(cctx, q); done <- err }()
	testkit.Until(t, "the search waits on the lock", func() bool {
		return s.w.Count(`SELECT count(*)::int FROM pg_locks l JOIN pg_class c ON c.oid = l.relation
			WHERE c.relname = 'messages' AND NOT l.granted
			  AND l.database = (SELECT oid FROM pg_database WHERE datname = current_database())`) > 0
	})
	cancel()
	if err := testkit.Receive(t, "the cancelled search", done); !errors.Is(err, context.Canceled) || errors.Is(err, searchkit.ErrTimeout) {
		t.Fatalf("cancelled search = %v, want context.Canceled", err)
	}
}
