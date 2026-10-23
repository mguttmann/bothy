// SPDX-License-Identifier: Apache-2.0

package postgres_test

import (
	"context"
	"errors"
	"os"
	"slices"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/OWNER/bothy/services/server/internal/chat"
	"github.com/OWNER/bothy/services/server/internal/files"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/searchkit"
	"github.com/OWNER/bothy/services/server/internal/messaging"
	"github.com/OWNER/bothy/services/server/internal/platform/pg"
	"github.com/OWNER/bothy/services/server/internal/platform/pg/pgtest"
	"github.com/OWNER/bothy/services/server/internal/search/adapters/postgres"
	"github.com/OWNER/bothy/services/server/internal/search/domain"
)

func TestMain(m *testing.M) { os.Exit(pgtest.Main(m)) }

// The read ports under distinct names, so fakeReads can embed all three.
type (
	messageReads = messaging.SearchReads
	chatReads    = chat.SearchReads
	fileReads    = files.SearchReads
)

// fakeReads records the calls of the index to the read operations of the modules; methods it does
// not override panic through the nil embedded interfaces.
type fakeReads struct {
	messageReads
	chatReads
	fileReads

	matching        []ids.ID
	matchingAsked   []searchkit.ThreadFilter
	messageQueries  []searchkit.Query
	fileQueries     []searchkit.Query
	threadQueries   []searchkit.Query
	matchingChannel [][]ids.ID
}

func (f *fakeReads) SearchMessages(_ context.Context, q searchkit.Query) (searchkit.Page, error) {
	f.messageQueries = append(f.messageQueries, q)
	return searchkit.Page{Hits: []searchkit.Hit{}}, nil
}

func (f *fakeReads) SearchFiles(_ context.Context, q searchkit.Query) (searchkit.Page, error) {
	f.fileQueries = append(f.fileQueries, q)
	return searchkit.Page{Hits: []searchkit.Hit{}}, nil
}

func (f *fakeReads) SearchThreadTitles(_ context.Context, q searchkit.Query) (searchkit.Page, error) {
	f.threadQueries = append(f.threadQueries, q)
	return searchkit.Page{Hits: []searchkit.Hit{}}, nil
}

func (f *fakeReads) ThreadsMatching(_ context.Context, channels []ids.ID, tf searchkit.ThreadFilter) ([]ids.ID, error) {
	f.matchingAsked = append(f.matchingAsked, tf)
	f.matchingChannel = append(f.matchingChannel, channels)
	return slices.Clone(f.matching), nil
}

// newIndex returns an index over a fresh database with the chapter languages.
func newIndex(t testing.TB, r *fakeReads) *postgres.Index {
	t.Helper()
	db := pgtest.New(t)
	pool := db.OpenPool(t, db.Config(pg.RoleApp))
	x, err := postgres.New(pool, []string{"german", "english"}, 2, postgres.Reads{Messages: r, Chat: r, Files: r})
	if err != nil {
		t.Fatal(err)
	}
	return x
}

// TestBuildQueryAndPrefix holds the query construction of 14 4.2 and 4.9 on PostgreSQL.
func TestBuildQueryAndPrefix(t *testing.T) {
	t.Parallel()
	x := newIndex(t, &fakeReads{})
	ctx := context.Background()
	tsq, ok, err := x.BuildQuery(ctx, "Rechnungen")
	if err != nil || !ok || !strings.Contains(tsq, "'rechnung'") || !strings.Contains(tsq, "'rechnungen'") {
		t.Fatalf("BuildQuery(Rechnungen) = %q, %v, %v", tsq, ok, err)
	}
	if _, ok, err := x.BuildQuery(ctx, "!!! ..."); err != nil || ok {
		t.Fatalf("punctuation only: ok %v err %v, want no term", ok, err)
	}
	p, ok, err := x.BuildPrefix(ctx, "rechnungs okt")
	if err != nil || !ok || p != "'okt':* & 'rechnungs':*" {
		t.Fatalf("BuildPrefix(rechnungs okt) = %q, %v, %v", p, ok, err)
	}
	if _, ok, err := x.BuildPrefix(ctx, "r o"); err != nil || ok {
		t.Fatalf("only short words: ok %v err %v, want an empty answer", ok, err)
	}
}

// TestIndex_ResolvesThreadFacts holds design note 5: thread facts become thread ids through
// chat.ThreadsMatching for messages and files, intersected with a given thread; an empty result
// runs no further query; threads keep their filter for chat.
func TestIndex_ResolvesThreadFacts(t *testing.T) {
	t.Parallel()
	a, b, c := ids.New(), ids.New(), ids.New()
	r := &fakeReads{matching: []ids.ID{a, b}}
	x := newIndex(t, r)
	ctx := context.Background()
	ch := []ids.ID{ids.New()}
	f := searchkit.ThreadFilter{State: "open"}
	if _, err := x.Messages(ctx, searchkit.Query{ChannelIDs: ch, Thread: f}); err != nil {
		t.Fatal(err)
	}
	if len(r.messageQueries) != 1 || !slices.Equal(r.messageQueries[0].ThreadIDs, []ids.ID{a, b}) || !r.messageQueries[0].Thread.IsZero() ||
		!slices.Equal(r.matchingChannel[0], ch) {
		t.Fatalf("messages query %+v", r.messageQueries)
	}
	if _, err := x.Files(ctx, searchkit.Query{ChannelIDs: ch, Thread: f, ThreadIDs: []ids.ID{b, c}}); err != nil {
		t.Fatal(err)
	}
	if len(r.fileQueries) != 1 || !slices.Equal(r.fileQueries[0].ThreadIDs, []ids.ID{b}) {
		t.Fatalf("files query %+v", r.fileQueries)
	}
	page, err := x.Messages(ctx, searchkit.Query{ChannelIDs: ch, Thread: f, ThreadIDs: []ids.ID{c}})
	if err != nil || len(page.Hits) != 0 || page.Hits == nil || len(r.messageQueries) != 1 {
		t.Fatalf("an empty thread list ran a query: %+v %v", r.messageQueries, err)
	}
	if _, err := x.Threads(ctx, searchkit.Query{ChannelIDs: ch, Thread: f}); err != nil {
		t.Fatal(err)
	}
	if len(r.threadQueries) != 1 || r.threadQueries[0].Thread.State != "open" || len(r.matchingAsked) != 3 {
		t.Fatalf("threads query %+v, ThreadsMatching asked %d times", r.threadQueries, len(r.matchingAsked))
	}
	if _, err := x.Messages(ctx, searchkit.Query{ChannelIDs: ch}); err != nil || len(r.messageQueries) != 2 || r.messageQueries[1].ThreadIDs != nil {
		t.Fatalf("no thread facts: %+v %v", r.messageQueries, err)
	}
}

// fuzzSeeds are hostile inputs of 14 12: punctuation, quotes, negation, the operators of
// to_tsquery, backslashes, NUL, invalid UTF-8 and very long input.
var fuzzSeeds = []string{
	"!!!", "...", "\"", "\"invoice", "-", "-invoice", "or", "invoice or", ":*", "a:*", "&|!()", "(a & b) | !c", "\\", "a\\b",
	"\x00", "a\x00b", "\xff\xfe", "invoice\xff", "'", "''", "o'neil", "<b>", "%", strings.Repeat("a", 256), strings.Repeat("a", 257),
	strings.Repeat("ä", 300), strings.Repeat("-a ", 85), "\t\n", "", "the", "a:B*", "\u202e", "\ufffd", "x'y'z", "e'\\x00'",
}

// FuzzSearchQuery (14 12): every q gives a page or 400 validation_failed on #query/q with code
// empty or too_long: the cleaning of domain never fails but on the length, and the query and the
// prefix construction on PostgreSQL never fail; no term is "empty".
func FuzzSearchQuery(f *testing.F) {
	for _, s := range fuzzSeeds {
		f.Add(s)
	}
	x := newIndex(f, &fakeReads{})
	f.Fuzz(func(t *testing.T, raw string) {
		q, err := domain.ValidateQ(raw, 256)
		if err != nil {
			e, ok := errs.As(err)
			if !ok || e.Code != errs.CodeValidationFailed || len(e.Fields) != 1 || e.Fields[0].Code != "too_long" ||
				utf8.RuneCountInString(raw) <= 256 {
				t.Fatalf("ValidateQ(%q) = %v", raw, err)
			}
			return
		}
		if !utf8.ValidString(q) || strings.ContainsRune(q, 0) {
			t.Fatalf("ValidateQ(%q) = %q keeps an invalid byte or NUL", raw, q)
		}
		ctx := context.Background()
		if _, _, err := x.BuildQuery(ctx, q); err != nil {
			t.Fatalf("BuildQuery(%q): %v", q, err)
		}
		if _, _, err := x.BuildPrefix(ctx, q); err != nil {
			t.Fatalf("BuildPrefix(%q): %v", q, err)
		}
	})
}

// TestBuildQuery_StatementTimeout maps SQLSTATE 57014 with a live context to searchkit.ErrTimeout
// and a cancelled context to its error.
func TestBuildQuery_StatementTimeout(t *testing.T) {
	t.Parallel()
	x := newIndex(t, &fakeReads{})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, _, err := x.BuildQuery(ctx, "invoice"); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled: %v", err)
	}
}
