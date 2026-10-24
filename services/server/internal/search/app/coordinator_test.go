// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"context"
	"errors"
	"log/slog"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/searchkit"
	"github.com/OWNER/bothy/services/server/internal/search"
)

// hit returns a hit in channel ch.
func hit(ch ids.ID) searchkit.Hit {
	return searchkit.Hit{ID: ids.New(), ThreadID: ids.New(), ChannelID: ch, Score: 0.5, Segments: []searchkit.Segment{{Text: "x", Match: true}}}
}

// field returns the code and pointer of the only field of a validation error.
func field(t *testing.T, err error) (code, pointer string) {
	t.Helper()
	e, ok := errs.As(err)
	if !ok || e.Code != errs.CodeValidationFailed || len(e.Fields) != 1 {
		t.Fatalf("err = %v, want validation_failed with one field", err)
	}
	return e.Fields[0].Code, e.Fields[0].Pointer
}

func TestCoordinator_EmptyChannelSetRunsNoQuery(t *testing.T) {
	t.Parallel()
	visible := ids.New()
	e := newEnv(t, authorize.Scope{ChannelIDs: []ids.ID{visible}})
	ctx := context.Background()
	for _, r := range []search.Request{
		{Q: "invoice", ChannelIDs: []ids.ID{ids.New()}},
		{Q: "invoice", SpaceIDs: []ids.ID{ids.New()}},
	} {
		res, err := e.coord.Query(ctx, e.alex, r)
		if err != nil {
			t.Fatal(err)
		}
		if len(res.Hits) != 0 || res.Next != nil || res.NextCursor != "" || res.CandidatesCapped || res.Hits == nil {
			t.Fatalf("result %+v, want an empty page", res)
		}
	}
	empty := newEnv(t, authorize.Scope{})
	if _, err := empty.coord.Query(ctx, empty.alex, search.Request{Q: "invoice"}); err != nil {
		t.Fatal(err)
	}
	if len(e.index.queries) != 0 || len(empty.index.queries) != 0 {
		t.Fatalf("an empty channel set ran %d and %d queries", len(e.index.queries), len(empty.index.queries))
	}
}

func TestCoordinator_SpaceAndChannelFilterIntersect(t *testing.T) {
	t.Parallel()
	a, b, c, d := ids.New(), ids.New(), ids.New(), ids.New()
	spaceVisible, spaceInvisible := ids.New(), ids.New()
	e := newEnv(t, authorize.Scope{ChannelIDs: sorted(a, b, c), SpaceIDs: []ids.ID{spaceVisible}})
	e.str.spaces[spaceVisible] = []ids.ID{b, c}
	e.str.spaces[spaceInvisible] = []ids.ID{d}
	_, err := e.coord.Query(context.Background(), e.alex, search.Request{
		Q: "invoice", ChannelIDs: []ids.ID{c, a, d, c}, SpaceIDs: []ids.ID{spaceVisible, spaceInvisible},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(e.index.queries) != 1 || !slices.Equal(e.index.queries[0].ChannelIDs, []ids.ID{c}) {
		t.Fatalf("queries %+v, want one over channel c only", e.index.queries)
	}
	if len(e.str.asked) != 1 || !slices.Equal(e.str.asked[0], []ids.ID{spaceVisible}) {
		t.Fatalf("ChannelsInSpaces asked for %v, want only the visible space", e.str.asked)
	}
}

func TestCoordinator_PageCheckRaceDropsSilently(t *testing.T) {
	t.Parallel()
	kept, revoked := ids.New(), ids.New()
	first := authorize.Scope{ChannelIDs: sorted(kept, revoked)}
	fresh := authorize.Scope{ChannelIDs: []ids.ID{kept}}
	e := newEnv(t, first, fresh)
	h1, h2 := hit(kept), hit(revoked)
	e.index.page = searchkit.Page{Hits: []searchkit.Hit{h1, h2}}
	res, err := e.coord.Query(context.Background(), e.alex, search.Request{Q: "invoice"})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Hits) != 1 || res.Hits[0].ID != h1.ID {
		t.Fatalf("hits %+v, want only the hit of the kept channel", res.Hits)
	}
	if len(e.errors.counts) != 0 || len(e.log.records) != 0 {
		t.Fatalf("a race counted %v and logged %d records", e.errors.counts, len(e.log.records))
	}
	if len(e.az.memos) != 2 || e.az.memos[1] == nil || e.az.memos[0] == e.az.memos[1] {
		t.Fatal("the page check did not read the rights in a fresh request scope")
	}
}

func TestCoordinator_PageCheckMismatchCounts(t *testing.T) {
	t.Parallel()
	ch := ids.New()
	e := newEnv(t, authorize.Scope{ChannelIDs: []ids.ID{ch}})
	h1, h2 := hit(ch), hit(ch)
	e.az.deny[h2.ID] = true
	e.index.page = searchkit.Page{Hits: []searchkit.Hit{h1, h2}}
	res, err := e.coord.Query(context.Background(), e.alex, search.Request{Q: "invoice"})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Hits) != 1 || res.Hits[0].ID != h1.ID {
		t.Fatalf("hits %+v, want the allowed hit only", res.Hits)
	}
	if e.errors.counts["filter_mismatch"] != 1 {
		t.Fatalf("counts %v, want filter_mismatch 1", e.errors.counts)
	}
	if len(e.log.records) != 1 {
		t.Fatalf("%d log records, want 1", len(e.log.records))
	}
	r := e.log.records[0]
	var text strings.Builder
	text.WriteString(r.Message)
	r.Attrs(func(a slog.Attr) bool { text.WriteString(" " + a.Key + "=" + a.Value.String()); return true })
	for _, id := range []ids.ID{h2.ID, h2.ThreadID, ch} {
		if strings.Contains(text.String(), id.String()) {
			t.Fatalf("the log record names an id: %s", text.String())
		}
	}
	if !strings.Contains(text.String(), "filter_mismatch") || !strings.Contains(text.String(), "message.read") {
		t.Fatalf("the log record %q lacks the action or the reason", text.String())
	}
}

func TestCoordinator_NextFollowsLastRow(t *testing.T) {
	t.Parallel()
	ch := ids.New()
	e := newEnv(t, authorize.Scope{ChannelIDs: []ids.ID{ch}})
	h1, h2 := hit(ch), hit(ch)
	e.az.deny[h2.ID] = true // the last row drops; the next page still starts after it
	next := &searchkit.Position{Score: 0.25, ID: h2.ID}
	e.index.page = searchkit.Page{Hits: []searchkit.Hit{h1, h2}, Next: next}
	ctx := context.Background()
	req := search.Request{Q: "invoice"}
	res, err := e.coord.Query(ctx, e.alex, req)
	if err != nil {
		t.Fatal(err)
	}
	if res.Next == nil || *res.Next != *next || res.NextCursor == "" || !res.RefTime.Equal(e.clock.Now()) {
		t.Fatalf("result %+v, want next at the last returned row and the request time", res)
	}
	// The second page: the decoded position and reference time reach the index, also after the
	// clock moved on.
	e.clock.Advance(time.Hour)
	req.Cursor = res.NextCursor
	res2, err := e.coord.Query(ctx, e.alex, req)
	if err != nil {
		t.Fatal(err)
	}
	q := e.index.queries[1]
	if q.From == nil || *q.From != *next || !q.RefTime.Equal(res.RefTime) || !res2.RefTime.Equal(res.RefTime) {
		t.Fatalf("second query from %+v at %v, want %+v at %v", q.From, q.RefTime, next, res.RefTime)
	}
	for name, r := range map[string]search.Request{
		"other q":    {Q: "invoices", Cursor: res.NextCursor},
		"other sort": {Q: "invoice", Sort: searchkit.SortNewest, Cursor: res.NextCursor},
		"other type": {Q: "invoice", Type: search.TypeThreads, Cursor: res.NextCursor},
	} {
		if _, err := e.coord.Query(ctx, e.alex, r); codeOf(err) != errs.CodeCursorInvalid {
			t.Errorf("%s: err = %v, want cursor_invalid", name, err)
		}
	}
	other := e.alex
	other.ID = ids.New()
	if _, err := e.coord.Query(ctx, other, req); codeOf(err) != errs.CodeCursorInvalid {
		t.Errorf("another principal: err = %v, want cursor_invalid", err)
	}
	req.Limit = 7
	if _, err := e.coord.Query(ctx, e.alex, req); err != nil {
		t.Errorf("another limit: %v", err)
	}
}

func TestCoordinator_TimeoutIs503SearchTimeout(t *testing.T) {
	t.Parallel()
	e := newEnv(t, authorize.Scope{ChannelIDs: []ids.ID{ids.New()}})
	e.index.err = searchkit.ErrTimeout
	_, err := e.coord.Query(context.Background(), e.alex, search.Request{Q: "invoice"})
	ee, ok := errs.As(err)
	if !ok || ee.Code != errs.CodeUnavailable || ee.Reason != "search_timeout" || errs.KindOf(err).Status() != 503 {
		t.Fatalf("err = %v, want 503 unavailable search_timeout", err)
	}
	if len(e.uow.opts) != 1 || e.uow.opts[0].StatementTimeout != testConfig().StatementTimeout {
		t.Fatalf("unit options %+v, want search.statement_timeout", e.uow.opts)
	}
	if _, err := e.coord.ThreadTitles(context.Background(), e.alex, search.PrefixRequest{Q: "rech"}); codeOf(err) != errs.CodeUnavailable {
		t.Fatalf("prefix err = %v, want unavailable", err)
	}
}

func TestCoordinator_MetricOutcomes(t *testing.T) {
	t.Parallel()
	ch := ids.New()
	cases := []struct {
		name string
		set  func(e *env)
		req  search.Request
		want string
	}{
		{name: "ok", req: search.Request{Q: "invoice"}, want: "ok"},
		{name: "client error", req: search.Request{Q: strings.Repeat("a", 257)}, want: "client_error"},
		{name: "denied", set: func(e *env) { e.az.denyQuery = true }, req: search.Request{Q: "invoice"}, want: "client_error"},
		{name: "timeout", set: func(e *env) { e.index.err = searchkit.ErrTimeout }, req: search.Request{Q: "invoice"}, want: "timeout"},
		{name: "server error", set: func(e *env) { e.index.err = errors.New("boom") }, req: search.Request{Q: "invoice"}, want: "server_error"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			e := newEnv(t, authorize.Scope{ChannelIDs: []ids.ID{ch}})
			if c.set != nil {
				c.set(e)
			}
			_, _ = e.coord.Query(context.Background(), e.alex, c.req)
			if !slices.Equal(e.metrics.outcomes, []string{c.want}) {
				t.Fatalf("outcomes %v, want [%s]", e.metrics.outcomes, c.want)
			}
		})
	}
	// The prefix search is measured by the HTTP metric only.
	e := newEnv(t, authorize.Scope{ChannelIDs: []ids.ID{ch}})
	if _, err := e.coord.ThreadTitles(context.Background(), e.alex, search.PrefixRequest{Q: "rech"}); err != nil {
		t.Fatal(err)
	}
	if len(e.metrics.outcomes) != 0 {
		t.Fatalf("the prefix search recorded %v", e.metrics.outcomes)
	}
}

func TestCoordinator_EmptyQueryIs400Empty(t *testing.T) {
	t.Parallel()
	e := newEnv(t, authorize.Scope{ChannelIDs: []ids.ID{ids.New()}})
	e.index.noTerm = true
	_, err := e.coord.Query(context.Background(), e.alex, search.Request{Q: "!!!"})
	if code, ptr := field(t, err); code != "empty" || ptr != "#query/q" {
		t.Fatalf("field %s %s, want empty on #query/q", code, ptr)
	}
	titles, err := e.coord.ThreadTitles(context.Background(), e.alex, search.PrefixRequest{Q: "a"})
	if err != nil || titles == nil || len(titles) != 0 {
		t.Fatalf("prefix search without a term = %v, %v; want an empty list", titles, err)
	}
}

func TestCoordinator_OrderOfChecks(t *testing.T) {
	t.Parallel()
	e := newEnv(t, authorize.Scope{ChannelIDs: []ids.ID{ids.New()}})
	e.az.denyQuery = true
	ctx := context.Background()
	// Parameter rules come before the cursor and the cursor before authorize (design note 2).
	_, err := e.coord.Query(ctx, e.alex, search.Request{Q: strings.Repeat("a", 257), Cursor: "junk"})
	if code, _ := field(t, err); code != "too_long" {
		t.Fatalf("code %s, want too_long", code)
	}
	_, err = e.coord.Query(ctx, e.alex, search.Request{Q: "a", Type: search.TypeThreads, HasFile: new(bool), Cursor: "junk"})
	if code, ptr := field(t, err); code != "not_applicable" || ptr != "#query/has_file" {
		t.Fatalf("field %s %s, want not_applicable on has_file", code, ptr)
	}
	if _, err := e.coord.Query(ctx, e.alex, search.Request{Q: "a", Cursor: "junk"}); codeOf(err) != errs.CodeCursorInvalid {
		t.Fatalf("err = %v, want cursor_invalid", err)
	}
	if _, err := e.coord.Query(ctx, e.alex, search.Request{Q: "a"}); codeOf(err) != errs.CodeScopeMissing {
		t.Fatalf("err = %v, want scope_missing", err)
	}
	if len(e.index.queries) != 0 {
		t.Fatal("a refused request reached the index")
	}
}

func TestCoordinator_QueryCarriesConfigAndFilters(t *testing.T) {
	t.Parallel()
	ch, thread, author, agent, tag := ids.New(), ids.New(), ids.New(), ids.New(), ids.New()
	e := newEnv(t, authorize.Scope{ChannelIDs: []ids.ID{ch}})
	after, before, yes := e.clock.Now().Add(-time.Hour), e.clock.Now(), true
	_, err := e.coord.Query(context.Background(), e.alex, search.Request{
		Q: "inv\x00oice", ThreadID: &thread, AuthorID: &author, AgentID: &agent, After: &after, Before: &before,
		HasFile: &yes, ThreadState: "open", TagIDs: []ids.ID{tag}, Limit: 500,
	})
	if err != nil {
		t.Fatal(err)
	}
	q := e.index.queries[0]
	if q.TSQuery != "'inv oice'" || !slices.Equal(q.ThreadIDs, []ids.ID{thread}) || *q.AuthorID != author ||
		*q.Thread.OperatorAgentID != agent || q.Thread.State != "open" || !slices.Equal(q.Thread.TagIDs, []ids.ID{tag}) ||
		!q.After.Equal(after) || !q.Before.Equal(before) || !*q.HasFile || q.Sort != searchkit.SortRelevance || q.From != nil {
		t.Fatalf("query %+v", q)
	}
	c := testConfig()
	if q.Limits.PageSize != c.PageSizeMax || q.Limits.MaxCandidates != c.MaxCandidates || q.Limits.HeadlineMaxWords != c.HeadlineMaxWords ||
		q.Limits.RecencyScale != c.RecencyScale || q.Markers.Start == "" || q.Markers.Start == q.Markers.Stop {
		t.Fatalf("limits %+v markers %+v", q.Limits, q.Markers)
	}
}

func sorted(in ...ids.ID) []ids.ID {
	out := slices.Clone(in)
	slices.SortFunc(out, func(a, b ids.ID) int { return strings.Compare(a.String(), b.String()) })
	return out
}

// codeOf returns the public code of err, "" for nil or a foreign error.
func codeOf(err error) errs.Code {
	if e, ok := errs.As(err); ok {
		return e.Code
	}
	return ""
}
