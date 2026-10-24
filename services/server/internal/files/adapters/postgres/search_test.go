// SPDX-License-Identifier: Apache-2.0

package postgres_test

import (
	"bytes"
	"cmp"
	"context"
	"errors"
	"fmt"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/OWNER/bothy/services/server/internal/files"
	"github.com/OWNER/bothy/services/server/internal/files/adapters/postgres"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/searchkit"
)

// searchWorld is the world of the attachment tests with what the search reads need: more threads,
// the thread_seq of each thread, a base time for created_at and the markers of the requests.
type searchWorld struct {
	*world
	// channelOf maps every thread to its channel; seq holds its last thread_seq.
	channelOf map[ids.ID]ids.ID
	seq       map[ids.ID]int
	// eskThread is a thread of Eskalation.
	eskThread ids.ID
	base      time.Time
	markers   searchkit.Markers
}

func newSearchWorld(t *testing.T) *searchWorld {
	t.Helper()
	w := newWorld(t)
	m, err := searchkit.NewMarkers(nil)
	if err != nil {
		t.Fatal(err)
	}
	s := &searchWorld{world: w, channelOf: map[ids.ID]ids.ID{w.thread: w.allgemein}, seq: map[ids.ID]int{w.thread: w.seq},
		base: time.Date(2026, 10, 1, 9, 0, 0, 0, time.UTC), markers: m}
	s.eskThread = s.newThread(w.eskalation)
	return s
}

// newThread inserts a thread of channel and returns it.
func (s *searchWorld) newThread(channel ids.ID) ids.ID {
	s.t.Helper()
	id := ids.New()
	s.exec(`INSERT INTO threads (id, channel_id, kind, created_by) VALUES ($1, $2, 'thread', $3)`, id, channel, s.alex)
	s.channelOf[id] = channel
	return id
}

// spec is one file of the search tests; the zero value of a field is a deliverable attachment of
// Alex in the thread of Allgemein.
type spec struct {
	name, transcript string
	at               time.Time
	// thread is the thread of the message the file hangs on; zero is the thread of Allgemein.
	thread ids.ID
	// uploader is zero for Alex.
	uploader ids.ID
	// upload, scan and purpose are zero for complete, clean and attachment.
	upload, scan, purpose string
	// scope is the scope channel; zero is the channel of the thread.
	scope      ids.ID
	deleted    bool
	unattached bool
}

// placed is a file of the search tests with the message, thread and channel it hangs on.
type placed struct {
	file, message, thread, channel ids.ID
}

// add inserts the file of sp and, unless sp.unattached, a message of Alex in its thread with the
// attachment row.
func (s *searchWorld) add(sp spec) placed {
	s.t.Helper()
	if sp.thread.IsZero() {
		sp.thread = s.thread
	}
	if sp.uploader.IsZero() {
		sp.uploader = s.alex
	}
	sp.upload = cmp.Or(sp.upload, "complete")
	sp.scan = cmp.Or(sp.scan, "clean")
	sp.purpose = cmp.Or(sp.purpose, "attachment")
	channel := s.channelOf[sp.thread]
	if sp.scope.IsZero() {
		sp.scope = channel
	}
	var scope, transcript, deletedAt any
	if sp.purpose == "attachment" {
		scope = sp.scope.String()
	}
	if sp.transcript != "" {
		transcript = sp.transcript
	}
	if sp.deleted {
		deletedAt = sp.at
	}
	p := placed{file: ids.New(), thread: sp.thread, channel: channel}
	s.exec(`INSERT INTO files (id, object_key, sha256, size_bytes, mime_type, original_name, uploaded_by, purpose, scope_channel_id,
		upload_state, scan_status, transcript_text, created_at, deleted_at)
		VALUES ($1, $2, '\x01', 10, 'text/plain', $3, $4, $5, $6, $7, $8, $9, $10, $11)`,
		p.file, "f/"+p.file.String(), sp.name, sp.uploader, sp.purpose, scope, sp.upload, sp.scan, transcript, sp.at, deletedAt)
	if sp.unattached {
		return p
	}
	s.seq[sp.thread]++
	p.message = ids.New()
	s.exec(`INSERT INTO messages (id, thread_id, channel_id, thread_seq, author_principal_id, client_msg_id, client_payload_sha256, origin, content_md)
		VALUES ($1, $2, $3, $4, $5, $6, '\x00', 'human', 'see the file')`, p.message, sp.thread, channel, s.seq[sp.thread], s.alex, ids.New())
	s.exec(`INSERT INTO message_attachments (message_id, file_id, position, thread_id, channel_id) VALUES ($1, $2, 1, $3, $4)`,
		p.message, p.file, sp.thread, channel)
	return p
}

// term is the tsquery text of the search tests.
const term = "invoice"

// query is a search for term in channels with generous limits; RefTime is a day after base.
func (s *searchWorld) query(sort searchkit.Sort, channels ...ids.ID) searchkit.Query {
	return searchkit.Query{TSQuery: term, ChannelIDs: channels, Sort: sort, RefTime: s.base.Add(24 * time.Hour), Markers: s.markers,
		Limits: searchkit.Limits{MaxCandidates: 100, RankNormalization: 1, RecencyScale: 24 * time.Hour, HeadlineInputMaxChars: 200,
			HeadlineMaxFragments: 2, HeadlineMaxWords: 30, HeadlineMinWords: 10, PageSize: 50}}
}

// search runs SearchFiles and fails the test on an error.
func (s *searchWorld) search(q searchkit.Query) searchkit.Page {
	s.t.Helper()
	p, err := s.svc.SearchFiles(context.Background(), q)
	if err != nil {
		s.t.Fatalf("SearchFiles: %v", err)
	}
	if p.Hits == nil {
		s.t.Fatal("SearchFiles returned nil hits")
	}
	return p
}

// pages pages through q from the first page and returns every page.
func (s *searchWorld) pages(q searchkit.Query) []searchkit.Page {
	s.t.Helper()
	var out []searchkit.Page
	for range 100 {
		p := s.search(q)
		out = append(out, p)
		if p.Next == nil {
			return out
		}
		q.From = p.Next
	}
	s.t.Fatal("more than 100 pages")
	return nil
}

// hitIDs returns the ids of the hits of pages in order.
func hitIDs(pages ...searchkit.Page) []ids.ID {
	var out []ids.ID
	for _, p := range pages {
		for _, h := range p.Hits {
			out = append(out, h.ID)
		}
	}
	return out
}

// fileIDs returns the file ids of ps.
func fileIDs(ps ...placed) []ids.ID {
	out := make([]ids.ID, 0, len(ps))
	for _, p := range ps {
		out = append(out, p.file)
	}
	return out
}

// sorts are both orders of 14 4.5.
var sorts = []searchkit.Sort{searchkit.SortRelevance, searchkit.SortNewest}

// TestSearchFiles_Visibility holds 14 4.3 item 2 for files: only files attached in ChannelIDs are
// hits, by the channel of the attachment, never by files.scope_channel_id; each hit carries the
// message, thread and channel of its attachment.
func TestSearchFiles_Visibility(t *testing.T) {
	t.Parallel()
	s := newSearchWorld(t)
	a := s.add(spec{name: "invoice march", at: s.base})
	e := s.add(spec{name: "invoice april", at: s.base.Add(time.Hour), thread: s.eskThread})
	// The scope channel is Allgemein, the attachment hangs in Eskalation (as after a move).
	moved := s.add(spec{name: "invoice may", at: s.base.Add(2 * time.Hour), thread: s.eskThread, scope: s.allgemein})
	for _, c := range []struct {
		name     string
		channels []ids.ID
		want     []placed
	}{
		{"Allgemein", []ids.ID{s.allgemein}, []placed{a}},
		{"Eskalation", []ids.ID{s.eskalation}, []placed{moved, e}},
		{"both", []ids.ID{s.allgemein, s.eskalation}, []placed{moved, e, a}},
		{"unknown channel", []ids.ID{ids.New()}, nil},
	} {
		for _, sort := range sorts {
			p := s.search(s.query(sort, c.channels...))
			if got := hitIDs(p); !slices.Equal(got, fileIDs(c.want...)) {
				t.Fatalf("%s %s: hits %v, want %v", c.name, sort, got, fileIDs(c.want...))
			}
			for i, h := range p.Hits {
				w := c.want[i]
				if h.MessageID != w.message || h.ThreadID != w.thread || h.ChannelID != w.channel {
					t.Fatalf("%s %s: hit %+v, want the place %+v", c.name, sort, h, w)
				}
			}
		}
	}
}

// TestSearchFiles_Filters holds the filters of 14 4.4 for files: ThreadIDs against the thread of
// the attachment (nil restricts nothing, empty matches nothing), AuthorID on files.uploaded_by,
// After and Before half-open on files.created_at; Thread and HasFile are ignored.
func TestSearchFiles_Filters(t *testing.T) {
	t.Parallel()
	s := newSearchWorld(t)
	t2 := s.newThread(s.allgemein)
	f1 := s.add(spec{name: "invoice one", at: s.base.Add(time.Hour)})
	f2 := s.add(spec{name: "invoice two", at: s.base.Add(2 * time.Hour), thread: t2, uploader: s.sam})
	f3 := s.add(spec{name: "invoice three", at: s.base.Add(3 * time.Hour), thread: t2})
	at2 := s.base.Add(2 * time.Hour)
	no := false
	for _, c := range []struct {
		name string
		set  func(*searchkit.Query)
		want []placed
	}{
		{"none", func(*searchkit.Query) {}, []placed{f3, f2, f1}},
		{"thread", func(q *searchkit.Query) { q.ThreadIDs = []ids.ID{t2} }, []placed{f3, f2}},
		{"two threads", func(q *searchkit.Query) { q.ThreadIDs = []ids.ID{s.thread, t2} }, []placed{f3, f2, f1}},
		{"empty thread list", func(q *searchkit.Query) { q.ThreadIDs = []ids.ID{} }, nil},
		{"author", func(q *searchkit.Query) { q.AuthorID = &s.sam }, []placed{f2}},
		{"after is inclusive", func(q *searchkit.Query) { q.After = &at2 }, []placed{f3, f2}},
		{"before is exclusive", func(q *searchkit.Query) { q.Before = &at2 }, []placed{f1}},
		{"thread filter and has_file are ignored", func(q *searchkit.Query) {
			q.Thread = searchkit.ThreadFilter{State: "closed"}
			q.HasFile = &no
		}, []placed{f3, f2, f1}},
	} {
		q := s.query(searchkit.SortNewest, s.allgemein)
		c.set(&q)
		if got := hitIDs(s.search(q)); !slices.Equal(got, fileIDs(c.want...)) {
			t.Fatalf("%s: hits %v, want %v", c.name, got, fileIDs(c.want...))
		}
	}
}

// TestSearchFiles_NotSearchable holds 14 1 and 2 for files: only deliverable files (upload_state
// complete, scan_status clean or skipped, not deleted, purpose attachment) that are attached are
// hits; a file of a deleted message (DeleteForMessage removes the row and marks the file) is none.
func TestSearchFiles_NotSearchable(t *testing.T) {
	t.Parallel()
	s := newSearchWorld(t)
	at := s.base
	clean := s.add(spec{name: "invoice", at: at})
	skipped := s.add(spec{name: "invoice", at: at, scan: "skipped"})
	for _, sp := range []spec{
		{upload: "pending", scan: "pending"},
		{upload: "failed", scan: "pending"},
		{scan: "pending"},
		{scan: "infected"},
		{scan: "error"},
		{deleted: true},
		{unattached: true},
		{purpose: "avatar"},
	} {
		sp.name, sp.at = "invoice", at
		s.add(sp)
	}
	gone := s.add(spec{name: "invoice", at: at})
	if err := s.uow.Do(context.Background(), func(ctx context.Context) error {
		return s.svc.DeleteForMessage(ctx, gone.message)
	}); err != nil {
		t.Fatal(err)
	}
	want := fileIDs(clean, skipped)
	slices.SortFunc(want, func(a, b ids.ID) int { return bytes.Compare(b[:], a[:]) })
	for _, sort := range sorts {
		if got := hitIDs(s.search(s.query(sort, s.allgemein))); !slices.Equal(got, want) {
			t.Fatalf("%s: hits %v, want %v", sort, got, want)
		}
	}
}

// TestSearchFiles_CandidateCap holds 14 4.5 item 1 for both orders: only the newest MaxCandidates
// matches are ranked and reachable, CandidatesCapped reports the limit on every page, also on an
// empty page after the keyset, and is false when the matches fit.
func TestSearchFiles_CandidateCap(t *testing.T) {
	t.Parallel()
	s := newSearchWorld(t)
	var all []placed
	for i := range 5 {
		all = append(all, s.add(spec{name: "invoice", transcript: strings.Repeat("invoice ", i), at: s.base.Add(time.Duration(i) * time.Hour)}))
	}
	newest := fileIDs(all[2:]...)
	for _, sort := range sorts {
		q := s.query(sort, s.allgemein)
		q.Limits.MaxCandidates, q.Limits.PageSize = 3, 2
		pages := s.pages(q)
		got := hitIDs(pages...)
		if len(got) != 3 || !sameSet(got, newest) {
			t.Fatalf("%s: reachable %v, want the newest three %v", sort, got, newest)
		}
		for i, p := range pages {
			if !p.CandidatesCapped {
				t.Fatalf("%s: page %d does not report the cap", sort, i)
			}
		}
		last := pages[len(pages)-1].Hits
		q.From = position(sort, last[len(last)-1])
		p := s.search(q)
		if len(p.Hits) != 0 || p.Next != nil || !p.CandidatesCapped {
			t.Fatalf("%s: the page after the last = %+v, want empty and capped", sort, p)
		}
		for _, c := range []struct {
			max    int
			capped bool
		}{{4, true}, {5, false}, {6, false}} {
			q := s.query(sort, s.allgemein)
			q.Limits.MaxCandidates = c.max
			if p := s.search(q); p.CandidatesCapped != c.capped || len(p.Hits) != min(c.max, 5) {
				t.Fatalf("%s max %d: capped %v with %d hits", sort, c.max, p.CandidatesCapped, len(p.Hits))
			}
		}
	}
}

// sameSet reports whether a and b hold the same ids.
func sameSet(a, b []ids.ID) bool {
	x, y := slices.Clone(a), slices.Clone(b)
	order := func(a, b ids.ID) int { return bytes.Compare(a[:], b[:]) }
	slices.SortFunc(x, order)
	slices.SortFunc(y, order)
	return slices.Equal(x, y)
}

// position is the keyset of h in sort.
func position(sort searchkit.Sort, h searchkit.Hit) *searchkit.Position {
	if sort == searchkit.SortNewest {
		return &searchkit.Position{CreatedAt: h.CreatedAt, ID: h.ID}
	}
	return &searchkit.Position{Score: h.Score, ID: h.ID}
}

// TestSearchFiles_Ranking holds 14 4.5 items 2 and 3: with equal texts the newer file ranks first
// (age from RefTime over search.recency_scale), equal scores rank the higher id first, newest
// orders by created_at then id with score 0.
func TestSearchFiles_Ranking(t *testing.T) {
	t.Parallel()
	s := newSearchWorld(t)
	old := s.add(spec{name: "invoice", at: s.base})
	newer := s.add(spec{name: "invoice", at: s.base.Add(10 * time.Hour)})
	twinA := s.add(spec{name: "invoice", at: s.base.Add(5 * time.Hour)})
	twinB := s.add(spec{name: "invoice", at: s.base.Add(5 * time.Hour)})
	hi, lo := twinA, twinB
	if bytes.Compare(twinA.file[:], twinB.file[:]) < 0 {
		hi, lo = twinB, twinA
	}
	want := fileIDs(newer, hi, lo, old)
	for _, sort := range sorts {
		p := s.search(s.query(sort, s.allgemein))
		if got := hitIDs(p); !slices.Equal(got, want) {
			t.Fatalf("%s: order %v, want %v", sort, got, want)
		}
		for i, h := range p.Hits {
			switch {
			case sort == searchkit.SortNewest && h.Score != 0:
				t.Fatalf("newest: hit %d has score %v", i, h.Score)
			case sort == searchkit.SortRelevance && h.Score <= 0:
				t.Fatalf("relevance: hit %d has score %v", i, h.Score)
			case sort == searchkit.SortRelevance && i > 0 && h.Score > p.Hits[i-1].Score:
				t.Fatalf("relevance: score rises at hit %d", i)
			}
		}
		if sort == searchkit.SortRelevance && (p.Hits[1].Score != p.Hits[2].Score || p.Hits[0].Score == p.Hits[1].Score) {
			t.Fatalf("relevance: scores %v %v %v", p.Hits[0].Score, p.Hits[1].Score, p.Hits[2].Score)
		}
		if !p.Hits[0].CreatedAt.Equal(s.base.Add(10 * time.Hour)) {
			t.Fatalf("%s: CreatedAt %v", sort, p.Hits[0].CreatedAt)
		}
	}
}

// TestSearchFiles_Paging holds 14 4.7 for both orders: pages strictly after From, PageSize rows
// each, Next the position of the last row while a further row exists and nil on the last page; the
// pages joined equal the single page without gaps or duplicates.
func TestSearchFiles_Paging(t *testing.T) {
	t.Parallel()
	s := newSearchWorld(t)
	for i := range 7 {
		s.add(spec{name: "report", transcript: strings.Repeat("invoice ", 1+i%3), at: s.base.Add(time.Duration(i) * time.Hour)})
	}
	for _, sort := range sorts {
		whole := hitIDs(s.search(s.query(sort, s.allgemein)))
		if len(whole) != 7 {
			t.Fatalf("%s: %d hits", sort, len(whole))
		}
		q := s.query(sort, s.allgemein)
		q.Limits.PageSize = 3
		pages := s.pages(q)
		if len(pages) != 3 || len(pages[0].Hits) != 3 || len(pages[1].Hits) != 3 || len(pages[2].Hits) != 1 {
			t.Fatalf("%s: %d pages", sort, len(pages))
		}
		if got := hitIDs(pages...); !slices.Equal(got, whole) {
			t.Fatalf("%s: pages %v, want %v", sort, got, whole)
		}
		for i, p := range pages[:2] {
			if want := position(sort, p.Hits[len(p.Hits)-1]); *p.Next != *want {
				t.Fatalf("%s: Next of page %d = %+v, want %+v", sort, i, *p.Next, *want)
			}
		}
	}
}

// TestSearchFiles_RefTime holds 14 4.7 and brief M3-14 design note 7: a file created after RefTime
// is no candidate, one created at RefTime is.
func TestSearchFiles_RefTime(t *testing.T) {
	t.Parallel()
	s := newSearchWorld(t)
	q := s.query(searchkit.SortRelevance, s.allgemein)
	at := s.add(spec{name: "invoice", at: q.RefTime})
	s.add(spec{name: "invoice", at: q.RefTime.Add(time.Millisecond)})
	for _, sort := range sorts {
		q.Sort = sort
		if got := hitIDs(s.search(q)); !slices.Equal(got, fileIDs(at)) {
			t.Fatalf("%s: hits %v, want only the file at RefTime", sort, got)
		}
	}
}

// TestSearchFiles_Headline holds 14 4.6 for files: the headline over the name followed by the
// transcript, split at the random markers, only the term matches, and the input is cut at
// search.headline_input_max_chars. HTML stays text: an entity such as &lt; comes back verbatim,
// and ts_headline replaces an XML tag (a tag token of its parser) by a space, so a tag never
// reaches a segment as markup.
func TestSearchFiles_Headline(t *testing.T) {
	t.Parallel()
	s := newSearchWorld(t)
	name, transcript := "<b>invoice</b> &lt; draft", "the invoice was paid in march"
	s.add(spec{name: name, transcript: transcript, at: s.base.Add(time.Hour)})
	s.add(spec{name: "notes", transcript: strings.Repeat("filler ", 100) + "invoice", at: s.base})
	q := s.query(searchkit.SortNewest, s.allgemein)
	p := s.search(q)
	if len(p.Hits) != 2 {
		t.Fatalf("%d hits", len(p.Hits))
	}
	content := strings.NewReplacer("<b>", " ", "</b>", " ").Replace(name + " " + transcript)
	segs := p.Hits[0].Segments
	matches, rest := 0, content
	for _, sg := range segs {
		if strings.Contains(sg.Text, s.markers.Start) || strings.Contains(sg.Text, s.markers.Stop) {
			t.Fatalf("segment %q carries a marker", sg.Text)
		}
		i := strings.Index(rest, sg.Text)
		if i < 0 {
			t.Fatalf("segment %q is not in the content in order: %+v", sg.Text, segs)
		}
		rest = rest[i+len(sg.Text):]
		if sg.Match {
			matches++
			if sg.Text != "invoice" {
				t.Fatalf("match %q, want only the term", sg.Text)
			}
		}
	}
	if joined := fmt.Sprint(segs); matches == 0 || !strings.Contains(joined, "&lt; draft") || strings.Contains(joined, "<") {
		t.Fatalf("segments %+v: no match or not over the name", segs)
	}
	cut := p.Hits[1].Segments
	if cut == nil {
		t.Fatal("nil segments")
	}
	n := 0
	for _, sg := range cut {
		n += len(sg.Text)
		if sg.Match {
			t.Fatalf("the term beyond search.headline_input_max_chars was highlighted: %+v", cut)
		}
	}
	if n > q.Limits.HeadlineInputMaxChars {
		t.Fatalf("headline of %d bytes from an input cut at %d", n, q.Limits.HeadlineInputMaxChars)
	}
}

// TestSearchFiles_ContextEnded: a cancelled context gives the context error, not ErrTimeout.
func TestSearchFiles_ContextEnded(t *testing.T) {
	t.Parallel()
	s := newSearchWorld(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := s.svc.SearchFiles(ctx, s.query(searchkit.SortRelevance, s.allgemein)); !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
}

// TestSearchReads_MapStatementTimeout holds brief M3-14 design note 6 for SearchFiles: SQLSTATE
// 57014 while the context is alive is searchkit.ErrTimeout, with an ended context the context
// error, any other error the internal error of the module.
func TestSearchReads_MapStatementTimeout(t *testing.T) {
	t.Parallel()
	timeout := &pgconn.PgError{Code: "57014"}
	live := context.Background()
	ended, cancel := context.WithCancel(context.Background())
	cancel()
	if err := postgres.SearchError(live, nil, "searching files"); err != nil {
		t.Fatalf("nil error mapped to %v", err)
	}
	for _, err := range []error{timeout, fmt.Errorf("query: %w", timeout)} {
		if got := postgres.SearchError(live, err, "searching files"); !errors.Is(got, searchkit.ErrTimeout) {
			t.Fatalf("live context, %v: %v, want searchkit.ErrTimeout", err, got)
		}
		if got := postgres.SearchError(ended, err, "searching files"); !errors.Is(got, context.Canceled) || errors.Is(got, searchkit.ErrTimeout) {
			t.Fatalf("ended context, %v: %v, want context.Canceled", err, got)
		}
	}
	got := postgres.SearchError(live, &pgconn.PgError{Code: "42P01"}, "searching files")
	if e, ok := errs.As(got); !ok || e.Code != errs.CodeInternalError || errors.Is(got, searchkit.ErrTimeout) {
		t.Fatalf("another database error: %v, want the internal error", got)
	}
}

// TestHitFiles holds files.SearchReads.HitFiles: the representation of getFile in the order of
// fileIDs; missing, not deliverable, unattached and files attached outside channelIDs are absent.
func TestHitFiles(t *testing.T) {
	t.Parallel()
	s := newSearchWorld(t)
	a := s.add(spec{name: "invoice", at: s.base})
	s.exec(`INSERT INTO file_thumbnails (file_id, variant, object_key, mime_type, width, height, size_bytes)
		VALUES ($1, 'small', $2, 'image/jpeg', 2, 1, 3)`, a.file, "t/"+a.file.String())
	b := s.add(spec{name: "invoice", at: s.base, thread: s.eskThread})
	c := s.add(spec{name: "invoice", at: s.base, scan: "skipped"})
	pending := s.add(spec{name: "invoice", at: s.base, scan: "pending"})
	unattached := s.add(spec{name: "invoice", at: s.base, unattached: true})
	deleted := s.add(spec{name: "invoice", at: s.base, deleted: true})
	ctx := context.Background()
	// getFile answers an anonymous caller with 401 before any read (M3-08), so the expected
	// representations are read as Alex.
	asAlex := actor.WithActor(ctx, actor.Actor{ID: s.alex, Kind: actor.KindUser})
	want := map[ids.ID]files.File{}
	for _, p := range []placed{a, b, c} {
		f, err := s.svc.Get(asAlex, p.file)
		if err != nil {
			t.Fatal(err)
		}
		want[p.file] = f
	}
	if len(want[a.file].Thumbnails) != 1 {
		t.Fatalf("getFile of a: %+v", want[a.file])
	}
	for _, tc := range []struct {
		name     string
		in       []ids.ID
		channels []ids.ID
		out      []placed
	}{
		{"input order", fileIDs(c, b, a), []ids.ID{s.allgemein, s.eskalation}, []placed{c, b, a}},
		{"outside the channels", fileIDs(b, a, c), []ids.ID{s.allgemein}, []placed{a, c}},
		{"absent", append(fileIDs(pending, unattached, deleted, a), ids.New()), []ids.ID{s.allgemein}, []placed{a}},
		{"no ids", nil, []ids.ID{s.allgemein}, nil},
	} {
		got, err := s.svc.HitFiles(ctx, tc.in, tc.channels)
		if err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		if got == nil || len(got) != len(tc.out) {
			t.Fatalf("%s: %+v, want %d files", tc.name, got, len(tc.out))
		}
		for i, p := range tc.out {
			if !reflect.DeepEqual(got[i], want[p.file]) {
				t.Fatalf("%s: file %d = %+v, want getFile %+v", tc.name, i, got[i], want[p.file])
			}
		}
	}
}
