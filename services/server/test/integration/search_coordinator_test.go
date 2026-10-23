// SPDX-License-Identifier: Apache-2.0

package integration_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/searchkit"
	"github.com/OWNER/bothy/services/server/internal/platform/httpserver/httpservertest"
	"github.com/OWNER/bothy/services/server/internal/search"
	searchhttp "github.com/OWNER/bothy/services/server/internal/search/adapters/httpapi"
	searchpg "github.com/OWNER/bothy/services/server/internal/search/adapters/postgres"
	searchapp "github.com/OWNER/bothy/services/server/internal/search/app"
)

// searchConfig is the configuration of the search coordinator of the world: the defaults of
// docs/spec/14-search.md section 15.
func searchConfig() searchapp.Config {
	return searchapp.Config{
		QueryMaxChars: 256, MaxCandidates: 2000, RankNormalization: 1, RecencyScale: 30 * 24 * time.Hour,
		PageSizeDefault: 20, PageSizeMax: 50, HeadlineInputMaxChars: 20000, HeadlineMaxFragments: 2,
		HeadlineMaxWords: 30, HeadlineMinWords: 10, PrefixMinChars: 2, StatementTimeout: 3 * time.Second,
	}
}

// countingErrors is the ErrorCounter of a coordinator a test builds; it counts per reason.
type countingErrors struct {
	mu     sync.Mutex
	counts map[string]int
}

func (c *countingErrors) CountError(_ context.Context, reason string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.counts == nil {
		c.counts = map[string]int{}
	}
	c.counts[reason]++
}

func (c *countingErrors) count(reason string) int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.counts[reason]
}

// coordinator builds a search coordinator over the read operations of the world with cfg, the
// Authorizer az (the world's when nil) and the error counter errs (none when nil).
func (w *world) coordinator(cfg searchapp.Config, az authorize.Authorizer, errs searchapp.ErrorCounter) *searchapp.Coordinator {
	w.t.Helper()
	if az == nil {
		az = w.az
	}
	index, err := searchpg.New(w.pool, w.searchLanguages, cfg.PrefixMinChars,
		searchpg.Reads{Messages: w.searchReads.Messaging, Chat: w.searchReads.Chat, Files: w.searchReads.Files})
	must(w.t, err)
	c, err := searchapp.NewCoordinator(cfg, searchapp.Deps{
		Authorizer: az, UoW: w.uow, Index: index, Structure: w.searchReads.Chat, Cursors: searchhttp.Cursors{}, Clock: w.clk, Errors: errs,
	})
	must(w.t, err)
	return c
}

// say posts content as name into the main thread of channel and returns the message.
func (w *world) say(name string, channel ids.ID, content string) ids.ID {
	w.t.Helper()
	return (&searchWorld{world: w}).post(name, w.mainThread(channel), content)
}

// searchPage is the decoded SearchHitPage.
type searchPage struct {
	Items []struct {
		Kind     string   `json:"kind"`
		Score    *float64 `json:"score"`
		Segments []struct {
			Text  string `json:"text"`
			Match bool   `json:"match"`
		} `json:"segments"`
		ThreadTitle *string        `json:"thread_title"`
		ChannelName *string        `json:"channel_name"`
		SpaceName   *string        `json:"space_name"`
		Message     map[string]any `json:"message"`
		Thread      map[string]any `json:"thread"`
		File        map[string]any `json:"file"`
	} `json:"items"`
	NextCursor       *string `json:"next_cursor"`
	CandidatesCapped *bool   `json:"candidates_capped"`
}

// ids returns the ids of the objects of the page in order.
func (p searchPage) ids() []string {
	out := []string{}
	for _, it := range p.Items {
		for _, o := range []map[string]any{it.Message, it.Thread, it.File} {
			if o != nil {
				out = append(out, o["id"].(string))
			}
		}
	}
	return out
}

// searchRaw sends GET /search as name with the query parameters.
func (w *world) searchRaw(name string, params url.Values) *httpservertest.Response {
	w.t.Helper()
	return w.do(name, http.MethodGet, "/api/v1/search?"+params.Encode(), "")
}

// searchHTTP sends GET /search as name and decodes the 200 page.
func (w *world) searchHTTP(name string, params url.Values) searchPage {
	w.t.Helper()
	r := w.searchRaw(name, params)
	w.want(r, http.StatusOK, "")
	var p searchPage
	must(w.t, json.Unmarshal(r.Body, &p))
	if p.CandidatesCapped == nil {
		w.t.Fatalf("candidates_capped absent: %s", r.Body)
	}
	return p
}

// q returns query parameters with q and pairs of further names and values.
func q(text string, pairs ...string) url.Values {
	v := url.Values{"q": {text}}
	for i := 0; i+1 < len(pairs); i += 2 {
		v.Add(pairs[i], pairs[i+1])
	}
	return v
}

// TestAZ_M06_SearchShowsNothingFromMarketing (07 11.1 AZ-M06): three messages with "campaign" in
// Marketing/Kampagnen by Sam; Alex finds nothing and no counter, Sam finds the three.
func TestAZ_M06_SearchShowsNothingFromMarketing(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	var sams []string
	for i := range 3 {
		sams = append(sams, w.say("sam", w.kampagnen, "campaign plan "+strconv.Itoa(i)).String())
	}
	r := w.searchRaw("alex", q("campaign"))
	w.want(r, http.StatusOK, "")
	var body map[string]any
	must(t, json.Unmarshal(r.Body, &body))
	if len(body) != 2 || body["candidates_capped"] != false || len(body["items"].([]any)) != 0 {
		t.Fatalf("Alex got %s, want only items [] and candidates_capped false", r.Body)
	}
	got := w.searchHTTP("sam", q("campaign"))
	if g := got.ids(); len(g) != 3 || !sameSet(g, sams) {
		t.Fatalf("Sam found %v, want %v", g, sams)
	}
}

// sameSet reports whether a and b hold the same strings.
func sameSet(a, b []string) bool {
	x, y := slices.Clone(a), slices.Clone(b)
	slices.Sort(x)
	slices.Sort(y)
	return slices.Equal(x, y)
}

// TestSearch_TwoPrincipalsOnlyOwnChannels: the same term in Support/Eskalation (Alex reads it,
// Sam not) and Marketing/Kampagnen (Sam reads it, Alex not); each finds only the own channel and
// no response names the other channel, its name or its space.
func TestSearch_TwoPrincipalsOnlyOwnChannels(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	alexMsg := w.say("alex", w.eskalation, "invoice for the customer")
	samMsg := w.say("sam", w.kampagnen, "invoice for the campaign")
	for _, c := range []struct {
		name    string
		want    ids.ID
		foreign []string
	}{
		{"alex", alexMsg, []string{w.kampagnen.String(), w.marketing.String(), "Kampagnen", "Marketing", samMsg.String()}},
		{"sam", samMsg, []string{w.eskalation.String(), w.support.String(), "Eskalation", "Support", alexMsg.String()}},
	} {
		for _, typ := range []string{"messages", "threads", "files"} {
			r := w.searchRaw(c.name, q("invoice", "type", typ))
			w.want(r, http.StatusOK, "")
			for _, f := range c.foreign {
				if strings.Contains(string(r.Body), f) {
					t.Fatalf("%s %s names %q: %s", c.name, typ, f, r.Body)
				}
			}
		}
		if got := w.searchHTTP(c.name, q("invoice")).ids(); !slices.Equal(got, []string{c.want.String()}) {
			t.Fatalf("%s found %v, want %s", c.name, got, c.want)
		}
	}
}

// TestSearch_OwnerNoForeignDMHits: DMs arrive with V3-01, so the test inserts one channel of kind
// dm with its main thread and a message through the owner pool (the API cannot create one yet).
// The owner is no participant: no hit with any type and filter, and a channel_id filter on the
// DM answers exactly like one on a random id.
func TestSearch_OwnerNoForeignDMHits(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	dm, main, msg := ids.New(), ids.New(), ids.New()
	w.sql(`INSERT INTO channels (id, space_id, kind, name, sort_key, operator_inherit, history_backfill_limit, created_by)
		VALUES ($1, NULL, 'dm', '', 'a', false, 0, $2)`, dm.String(), w.alex.String())
	w.sql(`INSERT INTO threads (id, channel_id, kind, created_by, created_at, last_activity_at, search_language)
		VALUES ($1, $2, 'main', $3, $4, $4, 'simple')`, main.String(), dm.String(), w.alex.String(), w.clk.Now())
	w.sql(`INSERT INTO messages (id, thread_id, channel_id, thread_seq, author_principal_id, client_msg_id, client_payload_sha256,
		origin, content_md, created_at, search_language) VALUES ($1, $2, $3, 1, $4, $5, '\x00', 'human', 'secret invoice', $6, 'simple')`,
		msg.String(), main.String(), dm.String(), w.alex.String(), ids.New().String(), w.clk.Now())
	for _, typ := range []string{"messages", "threads", "files"} {
		if got := w.searchHTTP("owner", q("invoice", "type", typ)); len(got.Items) != 0 {
			t.Fatalf("owner found %v in a foreign DM (%s)", got.ids(), typ)
		}
	}
	a := w.searchRaw("owner", q("invoice", "channel_id", dm.String()))
	b := w.searchRaw("owner", q("invoice", "channel_id", ids.New().String()))
	if a.StatusCode != b.StatusCode || string(a.Body) != string(b.Body) {
		t.Fatalf("DM filter %d %s, random filter %d %s", a.StatusCode, a.Body, b.StatusCode, b.Body)
	}
	if strings.Contains(string(a.Body), msg.String()) {
		t.Fatal("the DM message leaked")
	}
}

// TestSearch_RevokeThenSearch: after the owner revokes G-1, Alex finds nothing in Support.
func TestSearch_RevokeThenSearch(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	w.say("alex", w.allgemein, "invoice of Alex")
	if got := w.searchHTTP("alex", q("invoice")); len(got.Items) != 1 {
		t.Fatalf("before the revocation Alex found %v", got.ids())
	}
	w.wantOK(w.do("owner", http.MethodPost, "/api/v1/grants/"+w.g1.String()+"/revoke", `{"reason":"Moved"}`, key()...))
	if got := w.searchHTTP("alex", q("invoice")); len(got.Items) != 0 {
		t.Fatalf("after the revocation Alex found %v", got.ids())
	}
}

// revokingAuthorizer revokes once, right after the first Scope of the wrapped Authorizer returned.
type revokingAuthorizer struct {
	authorize.Authorizer
	once   sync.Once
	revoke func()
}

func (a *revokingAuthorizer) Scope(ctx context.Context, p authorize.Principal, act authorize.Action) (authorize.Scope, error) {
	s, err := a.Authorizer.Scope(ctx, p, act)
	a.once.Do(a.revoke)
	return s, err
}

// TestSearch_RevokeBetweenScopeAndQuery: G-1 is revoked (committed in its own unit) after the
// coordinator's first Scope returned, so the read operation still returns the Support rows; the
// page check drops them, and bothy.authz.errors with filter_mismatch stays unchanged.
func TestSearch_RevokeBetweenScopeAndQuery(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	m := w.say("alex", w.allgemein, "invoice of Alex")
	var revoked atomic.Bool
	az := &revokingAuthorizer{Authorizer: w.az, revoke: func() {
		w.wantOK(w.do("owner", http.MethodPost, "/api/v1/grants/"+w.g1.String()+"/revoke", `{"reason":"Moved"}`, key()...))
		revoked.Store(true)
	}}
	// The read operation of messaging sees the row: the decorator records what the index returned.
	errsCount := &countingErrors{}
	c := w.coordinator(searchConfig(), az, errsCount)
	res, err := c.Query(context.Background(), w.actor("alex"), search.Request{Q: "invoice"})
	must(t, err)
	if !revoked.Load() {
		t.Fatal("the decorator did not revoke")
	}
	if len(res.Hits) != 0 {
		t.Fatalf("hits %+v after the revocation", res.Hits)
	}
	if n := errsCount.count("filter_mismatch"); n != 0 {
		t.Fatalf("filter_mismatch counted %d times for a race", n)
	}
	// Without the revocation the same query finds the message: the rows were there.
	page, err := w.searchReads.Messaging.SearchMessages(context.Background(), searchkit.Query{
		TSQuery: "'invoic'", ChannelIDs: []ids.ID{w.allgemein}, Sort: searchkit.SortNewest, RefTime: w.clk.Now(),
		Limits: searchkit.Limits{MaxCandidates: 10, RankNormalization: 1, RecencyScale: time.Hour, HeadlineInputMaxChars: 100,
			HeadlineMaxFragments: 1, HeadlineMaxWords: 10, HeadlineMinWords: 2, PageSize: 10},
		Markers: searchkit.Markers{Start: strings.Repeat("a", 32), Stop: strings.Repeat("b", 32)},
	})
	must(t, err)
	if len(page.Hits) != 1 || page.Hits[0].ID != m {
		t.Fatalf("the read operation returned %+v, want the message of Alex", page.Hits)
	}
}

// TestSearch_SR9_AgentInRunEqualsAlex (SR9, AZ-P11 for search): Lotse may read Engineering, Alex
// not; Engineering/Werkzeuge holds more matches than search.max_candidates, Support/Allgemein a
// few. Lotse in Alex's run gets Alex's answer (items, scores, candidates_capped, the presence of
// the cursor, and decoded the same position and reference time); Lotse alone gets
// candidates_capped, which proves the setup would leak without the cut.
func TestSearch_SR9_AgentInRunEqualsAlex(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	w.grant(`"subject_kind":"agent","subject_id":"` + w.lotse.String() + `","target_kind":"space","target_id":"` + w.engineering.String() +
		`","level":"read","reason":"Engineering reader"`)
	for i := range 4 {
		w.say("owner", w.werkzeuge, "invoice tooling "+strconv.Itoa(i))
		w.clk.Advance(time.Second)
	}
	for i := range 3 {
		w.say("alex", w.allgemein, "invoice support "+strconv.Itoa(i))
		w.clk.Advance(time.Second)
	}
	cfg := searchConfig()
	cfg.MaxCandidates, cfg.PageSizeDefault = 5, 2
	c := w.coordinator(cfg, nil, nil)
	ctx := context.Background()
	alex := w.actor("alex")
	inRun := w.actor("lotse")
	inRun.ActingFor = actor.ActingFor{Status: actor.ActingForBound, UserID: w.alex, RunID: ids.New(), ThreadID: w.mainThread(w.allgemein), JTI: "jti"}
	for _, sort := range []searchkit.Sort{searchkit.SortRelevance, searchkit.SortNewest} {
		a, err := c.Query(ctx, alex, search.Request{Q: "invoice", Sort: sort})
		must(t, err)
		b, err := c.Query(ctx, inRun, search.Request{Q: "invoice", Sort: sort})
		must(t, err)
		if !sameHits(a.Hits, b.Hits) || a.CandidatesCapped || b.CandidatesCapped || (a.Next == nil) != (b.Next == nil) ||
			a.Next == nil || *a.Next != *b.Next || !a.RefTime.Equal(b.RefTime) {
			t.Fatalf("%s: Alex %+v, Lotse in Alex's run %+v", sort, a, b)
		}
		if a.NextCursor == b.NextCursor {
			t.Fatalf("%s: the cursor strings of two principals are equal", sort)
		}
		alone, err := c.Query(ctx, w.actor("lotse"), search.Request{Q: "invoice", Sort: sort})
		must(t, err)
		if !alone.CandidatesCapped {
			t.Fatalf("%s: Lotse alone did not reach the candidate limit; the test proves nothing", sort)
		}
	}
}

// sameHits compares two hit lists by id, kind, score and segments.
func sameHits(a, b []search.ResultHit) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].ID != b[i].ID || a[i].Kind != b[i].Kind || (a[i].Score == nil) != (b[i].Score == nil) ||
			(a[i].Score != nil && *a[i].Score != *b[i].Score) || !slices.Equal(a[i].Segments, b[i].Segments) {
			return false
		}
	}
	return true
}

// TestSearch_ViewAsAlexEqualsAlex: the owner viewing as Alex gets Alex's answer.
func TestSearch_ViewAsAlexEqualsAlex(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	w.say("alex", w.allgemein, "invoice support")
	w.say("sam", w.kampagnen, "invoice marketing")
	w.say("owner", w.werkzeuge, "invoice tooling")
	c := w.coordinator(searchConfig(), nil, nil)
	owner := w.actor("owner")
	owner.ViewAs = &actor.ViewAs{UserID: w.alex, StartedAt: w.clk.Now()}
	a, err := c.Query(context.Background(), w.actor("alex"), search.Request{Q: "invoice"})
	must(t, err)
	b, err := c.Query(context.Background(), owner, search.Request{Q: "invoice"})
	must(t, err)
	if len(a.Hits) != 1 || !sameHits(a.Hits, b.Hits) || a.CandidatesCapped != b.CandidatesCapped {
		t.Fatalf("Alex %+v, owner as Alex %+v", a, b)
	}
}

// TestSearch_CursorWithOtherRequestIs400: the next_cursor of a first page fails with another q,
// filter, sort, type, principal or one changed byte; another limit works.
func TestSearch_CursorWithOtherRequestIs400(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	for i := range 3 {
		w.say("alex", w.allgemein, "invoice "+strconv.Itoa(i))
	}
	first := w.searchHTTP("alex", q("invoice", "limit", "1"))
	if first.NextCursor == nil {
		t.Fatal("no next_cursor")
	}
	cur := *first.NextCursor
	flipped := []byte(cur)
	flipped[len(flipped)/2] ^= 0x01
	for name, c := range map[string]struct {
		who    string
		params url.Values
	}{
		"other q":         {"alex", q("invoices", "limit", "1", "cursor", cur)},
		"other filter":    {"alex", q("invoice", "limit", "1", "cursor", cur, "channel_id", w.allgemein.String())},
		"other sort":      {"alex", q("invoice", "limit", "1", "cursor", cur, "sort", "newest")},
		"other type":      {"alex", q("invoice", "limit", "1", "cursor", cur, "type", "threads")},
		"other principal": {"sam", q("invoice", "limit", "1", "cursor", cur)},
		"changed byte":    {"alex", q("invoice", "limit", "1", "cursor", string(flipped))},
	} {
		w.want(w.searchRaw(c.who, c.params), http.StatusBadRequest, "cursor_invalid")
		_ = name
	}
	if got := w.searchHTTP("alex", q("invoice", "limit", "2", "cursor", cur)); len(got.Items) != 2 {
		t.Fatalf("another limit gave %v", got.ids())
	}
}

// TestSearch_PagesAreStable: three pages with relevance and with newest; no item twice, none
// missing, order of 14 4.5; a matching message created between page one and two does not appear
// and moves nothing.
func TestSearch_PagesAreStable(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	var all []string
	for i := range 7 {
		all = append(all, w.say("alex", w.allgemein, "invoice "+strings.Repeat("run ", i)).String())
		w.clk.Advance(time.Minute)
	}
	walk := func(sort string, between func()) []string {
		var got []string
		params := q("invoice", "limit", "3", "sort", sort)
		for page := 0; ; page++ {
			p := w.searchHTTP("alex", params)
			got = append(got, p.ids()...)
			if page == 0 && between != nil {
				between()
			}
			if p.NextCursor == nil {
				return got
			}
			params.Set("cursor", *p.NextCursor)
		}
	}
	for _, sort := range []string{"relevance", "newest"} {
		before := walk(sort, nil)
		if len(before) != 7 || !sameSet(before, all) {
			t.Fatalf("%s: pages %v, want each of %v once", sort, before, all)
		}
		if sort == "newest" && !slices.Equal(before, reversed(all)) {
			t.Fatalf("newest: %v, want %v", before, reversed(all))
		}
		var added string
		during := walk(sort, func() {
			w.clk.Advance(time.Minute)
			added = w.say("alex", w.allgemein, "invoice").String()
		})
		if !slices.Equal(during, before) || slices.Contains(during, added) {
			t.Fatalf("%s: with a message between the pages %v, want %v", sort, during, before)
		}
		all = append(all, added)
		if got := walk(sort, nil); len(got) != len(all) {
			t.Fatalf("%s: a new search found %d of %d", sort, len(got), len(all))
		}
		all = all[:len(all)-1]
		w.wantOK(w.do("alex", http.MethodDelete, "/api/v1/messages/"+added, "", key()...))
	}
}

// reversed returns s in reverse order.
func reversed(s []string) []string {
	out := slices.Clone(s)
	slices.Reverse(out)
	return out
}

// TestSearch_FilterInvisibleChannelEqualsUnknown (14 12): channel_id and space_id of an
// invisible and of a random id give the same status, headers and body after normalising the
// per-request headers.
func TestSearch_FilterInvisibleChannelEqualsUnknown(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	w.say("sam", w.kampagnen, "invoice")
	w.say("alex", w.allgemein, "invoice")
	for _, c := range []struct{ param, invisible string }{
		{"channel_id", w.kampagnen.String()}, {"space_id", w.marketing.String()},
	} {
		a := w.searchRaw("alex", q("invoice", c.param, c.invisible))
		b := w.searchRaw("alex", q("invoice", c.param, ids.New().String()))
		if a.StatusCode != http.StatusOK || a.StatusCode != b.StatusCode || string(a.Body) != string(b.Body) ||
			!sameSearchHeaders(a.Header, b.Header) {
			t.Fatalf("%s: invisible %d %v %s, unknown %d %v %s", c.param, a.StatusCode, a.Header, a.Body, b.StatusCode, b.Header, b.Body)
		}
	}
}

// sameSearchHeaders compares two response headers without the per-request values.
func sameSearchHeaders(a, b http.Header) bool {
	norm := func(h http.Header) http.Header {
		h = h.Clone()
		for _, k := range []string{"Correlation-Id", "Date", "Ratelimit", "Ratelimit-Policy"} {
			h.Del(k)
		}
		return h
	}
	x, y := norm(a), norm(b)
	if len(x) != len(y) {
		return false
	}
	for k, v := range x {
		if !slices.Equal(v, y[k]) {
			return false
		}
	}
	return true
}

// TestSearch_Filters: the filters of 14 4.4 and thread_id per type with a world where exactly the
// expected hits match; a filter outside its types is not_applicable; tag_id with two tags matches
// either. has_file is proven with real attachments in test/search.
func TestSearch_Filters(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	sw := &searchWorld{world: w}
	fragen := w.channel(w.support, "forum", "Fragen")
	tag := func(name string) string {
		m := w.want(w.do("owner", http.MethodPost, "/api/v1/channels/"+fragen.String()+"/tags", `{"name":"`+name+`"}`, key()...), http.StatusCreated, "")
		return m["id"].(string)
	}
	tagA, tagB := tag("Billing"), tag("Access")
	postA, firstA := sw.forumPost("alex", fragen, "Invoice question", "invoice details A")
	w.clk.Advance(time.Minute)
	postB, firstB := sw.forumPost("owner", fragen, "Invoice problem", "invoice details B")
	w.clk.Advance(time.Minute)
	postC, firstC := sw.forumPost("alex", fragen, "Invoice idea", "invoice details C")
	w.wantOK(w.do("owner", http.MethodPut, "/api/v1/threads/"+postA.String()+"/tags", `{"tag_ids":["`+tagA+`"]}`, key()...))
	w.wantOK(w.do("owner", http.MethodPut, "/api/v1/threads/"+postB.String()+"/tags", `{"tag_ids":["`+tagB+`"]}`, key()...))
	w.wantOK(w.do("owner", http.MethodPost, "/api/v1/threads/"+postC.String()+"/close", "", key()...))
	plain := w.say("alex", w.allgemein, "invoice in the main thread")
	mid := w.clk.Now().Add(-90 * time.Second).UTC().Format(time.RFC3339Nano)

	check := func(name string, params url.Values, want ...ids.ID) {
		t.Helper()
		got := w.searchHTTP("alex", params).ids()
		var ws []string
		for _, id := range want {
			ws = append(ws, id.String())
		}
		if !sameSet(got, ws) {
			t.Errorf("%s: got %v, want %v", name, got, ws)
		}
	}
	check("messages, no filter", q("invoice"), firstA, firstB, firstC, plain)
	check("messages, channel", q("invoice", "channel_id", fragen.String()), firstA, firstB, firstC)
	check("messages, space", q("invoice", "space_id", w.support.String()), firstA, firstB, firstC, plain)
	check("messages, thread", q("invoice", "thread_id", postB.String()), firstB)
	check("messages, author", q("invoice", "author_id", w.owner.String()), firstB)
	check("messages, after", q("invoice", "after", mid), firstB, firstC, plain)
	check("messages, before", q("invoice", "before", mid), firstA)
	check("messages, state closed", q("invoice", "thread_state", "closed"), firstC)
	check("messages, state open", q("invoice", "thread_state", "open"), firstA, firstB, plain)
	check("messages, tag", q("invoice", "tag_id", tagA), firstA)
	check("messages, two tags", q("invoice", "tag_id", tagA, "tag_id", tagB), firstA, firstB)
	check("messages, tag and thread", q("invoice", "tag_id", tagA, "thread_id", postB.String()))
	check("messages, has_file false", q("invoice", "has_file", "false"), firstA, firstB, firstC, plain)
	check("messages, has_file true", q("invoice", "has_file", "true"))
	check("threads, no filter", q("invoice", "type", "threads"), postA, postB, postC)
	check("threads, thread", q("invoice", "type", "threads", "thread_id", postA.String()), postA)
	check("threads, state", q("invoice", "type", "threads", "thread_state", "closed"), postC)
	check("threads, two tags", q("invoice", "type", "threads", "tag_id", tagA, "tag_id", tagB), postA, postB)
	check("threads, after", q("invoice", "type", "threads", "after", mid), postB, postC)
	check("threads, unknown tag", q("invoice", "type", "threads", "tag_id", ids.New().String()))
	check("threads, agent", q("invoice", "type", "threads", "agent_id", ids.New().String()))
	for _, c := range []struct{ typ, param, value string }{
		{"threads", "author_id", w.alex.String()}, {"threads", "has_file", "true"}, {"files", "has_file", "false"},
		{"files", "thread_state", "open"}, {"files", "tag_id", tagA},
	} {
		p := w.want(w.searchRaw("alex", q("invoice", "type", c.typ, c.param, c.value)), http.StatusBadRequest, "validation_failed")
		fields, _ := p["errors"].([]any)
		if len(fields) != 1 || fields[0].(map[string]any)["code"] != "not_applicable" || fields[0].(map[string]any)["pointer"] != "#query/"+c.param {
			t.Errorf("%s with %s: %v", c.typ, c.param, p)
		}
	}
}

// TestSearch_SegmentsNoHTML (14 12): content with <b>, </b>, an entity and a hex string that looks
// like a marker gives segments whose texts appear in the content in order, no added markup, and
// matches only on the term. The parser of ts_headline reads a tag as one token and puts a space
// in its place, so the texts are compared with the content whose tags are spaces; an entity and
// the look-alike stay text.
func TestSearch_SegmentsNoHTML(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	content := "Before <b>invoice</b> &lt; 0123456789abcdef0123456789abcdef after"
	w.say("alex", w.allgemein, content)
	got := w.searchHTTP("alex", q("invoice"))
	if len(got.Items) != 1 {
		t.Fatalf("items %v", got.ids())
	}
	rest := tagPattern.ReplaceAllString(content, " ")
	matches := 0
	for _, s := range got.Items[0].Segments {
		i := strings.Index(rest, s.Text)
		if i < 0 {
			t.Fatalf("segment %q is not in the content in order (rest %q)", s.Text, rest)
		}
		rest = rest[i+len(s.Text):]
		if s.Match {
			matches++
			if !strings.EqualFold(s.Text, "invoice") {
				t.Fatalf("match segment %q", s.Text)
			}
		}
	}
	if matches != 1 {
		t.Fatalf("%d match segments, want 1: %+v", matches, got.Items[0].Segments)
	}
	var joined strings.Builder
	for _, s := range got.Items[0].Segments {
		joined.WriteString(s.Text)
	}
	if !strings.Contains(joined.String(), "&lt;") || !strings.Contains(joined.String(), "0123456789abcdef0123456789abcdef") {
		t.Fatalf("the entity or the look-alike did not stay text: %q", joined.String())
	}
}

// tagPattern matches an HTML tag as the parser of ts_headline reads it.
var tagPattern = regexp.MustCompile(`<[^>]*>`)

// TestSearch_RankingAndCap: equal texts of different age rank newer first; equal score ranks the
// higher id first; with more matches than search.max_candidates only the newest are reachable and
// candidates_capped is true.
func TestSearch_RankingAndCap(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	old := w.say("alex", w.allgemein, "invoice")
	w.clk.Advance(48 * time.Hour)
	young := w.say("alex", w.allgemein, "invoice")
	twin := w.say("alex", w.allgemein, "invoice")
	got := w.searchHTTP("alex", q("invoice"))
	if want := []string{twin.String(), young.String(), old.String()}; !slices.Equal(got.ids(), want) {
		t.Fatalf("relevance order %v, want %v", got.ids(), want)
	}
	if *got.Items[0].Score != *got.Items[1].Score || *got.Items[1].Score <= *got.Items[2].Score {
		t.Fatalf("scores %v %v %v", *got.Items[0].Score, *got.Items[1].Score, *got.Items[2].Score)
	}
	newer := []ids.ID{}
	for i := range 3 {
		w.clk.Advance(time.Minute)
		newer = append(newer, w.say("alex", w.allgemein, "invoice "+strconv.Itoa(i)))
	}
	cfg := searchConfig()
	cfg.MaxCandidates = 3
	c := w.coordinator(cfg, nil, nil)
	for _, sort := range []searchkit.Sort{searchkit.SortRelevance, searchkit.SortNewest} {
		res, err := c.Query(context.Background(), w.actor("alex"), search.Request{Q: "invoice", Sort: sort})
		must(t, err)
		var gotIDs []ids.ID
		for _, h := range res.Hits {
			gotIDs = append(gotIDs, h.ID)
		}
		if !res.CandidatesCapped || len(gotIDs) != 3 || !sameIDSet(gotIDs, newer) {
			t.Fatalf("%s: capped %v hits %v, want the newest %v", sort, res.CandidatesCapped, gotIDs, newer)
		}
	}
}

// sameIDSet reports whether a and b hold the same ids.
func sameIDSet(a, b []ids.ID) bool {
	var x, y []string
	for _, id := range a {
		x = append(x, id.String())
	}
	for _, id := range b {
		y = append(y, id.String())
	}
	return sameSet(x, y)
}

// TestSearch_TypesThreads: type threads finds a forum post by its title with thread_title,
// channel_name and space_name; a message hit in a main thread has no thread_title.
func TestSearch_TypesThreads(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	fragen := w.channel(w.support, "forum", "Fragen")
	post, first := (&searchWorld{world: w}).forumPost("alex", fragen, "Rechnungslauf Oktober", "invoice run")
	got := w.searchHTTP("alex", q("Rechnungslauf", "type", "threads"))
	if len(got.Items) != 1 || got.Items[0].Kind != "thread" || got.Items[0].Thread["id"] != post.String() ||
		deref(got.Items[0].ThreadTitle) != "Rechnungslauf Oktober" || deref(got.Items[0].ChannelName) != "Fragen" ||
		deref(got.Items[0].SpaceName) != "Support" {
		t.Fatalf("threads: %+v", got.Items)
	}
	msgs := w.searchHTTP("alex", q("invoice"))
	if len(msgs.Items) != 1 || msgs.Items[0].Message["id"] != first.String() || deref(msgs.Items[0].ThreadTitle) != "Rechnungslauf Oktober" {
		t.Fatalf("messages: %+v", msgs.Items)
	}
	w.say("alex", w.allgemein, "invoice main")
	for _, it := range w.searchHTTP("alex", q("main")).Items {
		if it.ThreadTitle != nil {
			t.Fatalf("a hit of a main thread has thread_title %q", *it.ThreadTitle)
		}
	}
}

// deref returns *s or "".
func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// TestSearchThreadTitles (14 4.9): "rechnungs okt" finds "Rechnungslauf Oktober"; words below
// search.prefix_min_chars drop; only short words give 200 with no items; newest activity first;
// invisible channels never appear; a too long q is too_long.
func TestSearchThreadTitles(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	sw := &searchWorld{world: w}
	fragen := w.channel(w.support, "forum", "Fragen")
	ideen := w.channel(w.marketing, "forum", "Ideen")
	first, _ := sw.forumPost("alex", fragen, "Rechnungslauf Oktober", "text")
	w.clk.Advance(time.Minute)
	second, _ := sw.forumPost("alex", fragen, "Rechnungslauf Oktober Nachtrag", "text")
	sw.forumPost("sam", ideen, "Rechnungslauf Oktober Marketing", "text")
	titles := func(qs string) []string {
		r := w.do("alex", http.MethodGet, "/api/v1/search/thread-titles?"+url.Values{"q": {qs}}.Encode(), "")
		m := w.want(r, http.StatusOK, "")
		var out []string
		for _, it := range m["items"].([]any) {
			out = append(out, it.(map[string]any)["thread_id"].(string))
		}
		return out
	}
	if got := titles("rechnungs okt"); !slices.Equal(got, []string{second.String(), first.String()}) {
		t.Fatalf("rechnungs okt: %v", got)
	}
	if got := titles("rechnungs o"); !slices.Equal(got, []string{second.String(), first.String()}) {
		t.Fatalf("a short word did not drop: %v", got)
	}
	if got := titles("r o"); len(got) != 0 {
		t.Fatalf("only short words: %v", got)
	}
	w.want(w.do("alex", http.MethodGet, "/api/v1/search/thread-titles?q="+strings.Repeat("a", 257), ""), http.StatusBadRequest, "validation_failed")
}

// TestSearch_LimitCapped: a limit above search.page_size_max returns at most that many.
func TestSearch_LimitCapped(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	for i := range 4 {
		w.say("alex", w.allgemein, "invoice "+strconv.Itoa(i))
	}
	cfg := searchConfig()
	cfg.PageSizeMax = 2
	res, err := w.coordinator(cfg, nil, nil).Query(context.Background(), w.actor("alex"), search.Request{Q: "invoice", Limit: 500})
	must(t, err)
	if len(res.Hits) != 2 || res.Next == nil {
		t.Fatalf("hits %d next %v, want 2 and a next page", len(res.Hits), res.Next)
	}
}

// fuzzSeeds are the hostile inputs of 14 12 for the query construction (the seed corpus of
// FuzzSearchQuery in internal/search/adapters/postgres, here through the real modules and HTTP).
var fuzzSeeds = []string{
	"!!!", "...", "\"", "\"\"", "\"invoice", "-", "--", "-invoice", "or", "or or", "invoice or", ":*", "a:*", "&|!()",
	"(a & b) | !c", "\\", "\\\\", "a\\b", "\x00", "a\x00b", "\xff\xfe", "invoice\xff", "'", "''", "o'neil", "<b>", "%", "_",
	strings.Repeat("a", 256), strings.Repeat("a", 257), strings.Repeat("ä", 256), strings.Repeat("-a ", 85), "\t\n\r", " ",
	"", "the", "Rechnung's", "a:b", "a:B*", "@@", "$$", "‮", "�",
}

// TestSearch_HTTPNeverServerError runs the seeds of FuzzSearchQuery through the port and through
// HTTP for search and searchThreadTitles: a page or 400 validation_failed on q with empty or
// too_long; over HTTP 200 or 400, never 5xx.
func TestSearch_HTTPNeverServerError(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	w.say("alex", w.allgemein, "invoice o'neil a\\b <b>x</b>")
	c := w.coordinator(searchConfig(), nil, nil)
	for _, in := range fuzzSeeds {
		checkQueryInput(t, w, c, in)
	}
}

// checkQueryInput holds the fuzz property for one input.
func checkQueryInput(t *testing.T, w *world, c *searchapp.Coordinator, in string) {
	t.Helper()
	_, err := c.Query(context.Background(), w.actor("alex"), search.Request{Q: in})
	if err != nil {
		e, ok := errs.As(err)
		if !ok || e.Code != errs.CodeValidationFailed || len(e.Fields) != 1 || e.Fields[0].Pointer != "#query/q" ||
			(e.Fields[0].Code != "empty" && e.Fields[0].Code != "too_long") {
			t.Fatalf("q %q: %v", in, err)
		}
	}
	if _, err := c.ThreadTitles(context.Background(), w.actor("alex"), search.PrefixRequest{Q: in}); err != nil {
		e, ok := errs.As(err)
		if !ok || e.Code != errs.CodeValidationFailed || e.Fields[0].Code != "too_long" {
			t.Fatalf("prefix q %q: %v", in, err)
		}
	}
	for _, path := range []string{"/api/v1/search?", "/api/v1/search/thread-titles?"} {
		r := w.do("alex", http.MethodGet, path+url.Values{"q": {in}}.Encode(), "")
		if r.StatusCode != http.StatusOK && r.StatusCode != http.StatusBadRequest {
			t.Fatalf("%s q %q: %d %s", path, in, r.StatusCode, r.Body)
		}
	}
}

// TestSearchConfig_HeadlineRejectedByPostgres proves the start check search_headline_words
// (D-M3-14-14) against a real ts_headline: PostgreSQL refuses MinWords at and above MaxWords, so
// every search with such a pair would fail, and accepts a minimum below the maximum.
func TestSearchConfig_HeadlineRejectedByPostgres(t *testing.T) {
	t.Parallel()
	w := newEmptyWorld(t)
	headline := func(minWords, maxWords int) error {
		var s string
		return w.pool.QueryRow(context.Background(), `SELECT ts_headline('simple', 'one two three', 'two'::tsquery, $1)`,
			"MinWords="+strconv.Itoa(minWords)+",MaxWords="+strconv.Itoa(maxWords)).Scan(&s)
	}
	for _, c := range []struct {
		minWords, maxWords int
		ok                 bool
	}{{10, 30, true}, {29, 30, true}, {30, 30, false}, {31, 30, false}} {
		if err := headline(c.minWords, c.maxWords); (err == nil) != c.ok {
			t.Errorf("MinWords %d MaxWords %d: err %v, want ok %v", c.minWords, c.maxWords, err, c.ok)
		}
	}
}
