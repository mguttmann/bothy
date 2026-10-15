// SPDX-License-Identifier: Apache-2.0

package httpapi_test

import (
	"bytes"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	messagingapp "github.com/OWNER/bothy/services/server/internal/messaging/app"
	mw "github.com/OWNER/bothy/services/server/test/messagingworld"
)

// page is a decoded MessagePage.
type page struct {
	items                       []map[string]any
	hasMoreBefore, hasMoreAfter bool
	raw                         []byte
}

// history requests listThreadMessages of thread with query as token name and decodes the 200.
func history(t *testing.T, w *mw.World, name string, thread ids.ID, query string) page {
	t.Helper()
	path := mw.MessagesPath(thread)
	if query != "" {
		path += "?" + query
	}
	r := w.Do(name, http.MethodGet, path, "")
	body := w.Want(r, http.StatusOK, "")
	var p page
	for _, it := range body["items"].([]any) {
		p.items = append(p.items, it.(map[string]any))
	}
	p.hasMoreBefore, p.hasMoreAfter = body["has_more_before"].(bool), body["has_more_after"].(bool)
	p.raw = r.Body
	return p
}

// seqs returns the thread_seq of the items.
func (p page) seqs() []int64 {
	out := make([]int64, 0, len(p.items))
	for _, it := range p.items {
		out = append(out, int64(it["thread_seq"].(float64)))
	}
	return out
}

// span is the list from..to inclusive; empty when from > to.
func span(from, to int64) []int64 {
	out := []int64{}
	for i := from; i <= to; i++ {
		out = append(out, i)
	}
	return out
}

// fill sends n messages "message <i>" into thread as the owner.
func fill(t *testing.T, w *mw.World, thread ids.ID, n int) {
	t.Helper()
	for i := range n {
		w.Send("owner", thread, fmt.Sprintf("message %d", i+1), http.StatusCreated)
	}
}

// wantPage checks the items and both flags of p.
func wantPage(t *testing.T, label string, p page, seqs []int64, before, after bool) {
	t.Helper()
	got := p.seqs()
	if fmt.Sprint(got) != fmt.Sprint(seqs) || p.hasMoreBefore != before || p.hasMoreAfter != after {
		t.Errorf("%s: items %v before %v after %v; want %v %v %v", label, got, p.hasMoreBefore, p.hasMoreAfter, seqs, before, after)
	}
}

// noSecretsInLogs is the captured-logger assertion of the brief (SEC-LOG-3, SEC-LOG-4): no
// message content, query string or display name in any record of the modules or the chain.
func noSecretsInLogs(t *testing.T, w *mw.World, secrets ...string) {
	t.Helper()
	records := append(w.Logs.Records(), w.H.Logs.Records()...)
	for _, r := range records {
		text := r.Message + fmt.Sprint(r.Attrs)
		for _, s := range secrets {
			if strings.Contains(text, s) {
				t.Errorf("log record %q carries %q: %v", r.Message, s, r.Attrs)
			}
		}
	}
}

// TestListThreadMessages_NewestPage (06 section 6 item 5): 30 messages, limit=10 gives 21 to 30.
func TestListThreadMessages_NewestPage(t *testing.T) {
	t.Parallel()
	w := mw.New(t, subscribe)
	_, _, main := w.TextChannel()
	fill(t, w, main, 30)
	p := history(t, w, "owner", main, "limit=10")
	wantPage(t, "newest", p, span(21, 30), true, false)
	if p.items[0]["content_md"] != "message 21" {
		t.Errorf("first item %v", p.items[0]["content_md"])
	}
	noSecretsInLogs(t, w, "message 2", "limit=10")
}

// TestListThreadMessages_BeforeAfterAround: each mode against brief note 3, with both flags and
// the empty cases of note 4.
func TestListThreadMessages_BeforeAfterAround(t *testing.T) {
	t.Parallel()
	w := mw.New(t, subscribe)
	space, _, main := w.TextChannel()
	fill(t, w, main, 30)
	for _, c := range []struct {
		query         string
		seqs          []int64
		before, after bool
	}{
		{"before_seq=21&limit=10", span(11, 20), true, true},
		{"before_seq=5&limit=10", span(1, 4), false, true},
		{"before_seq=1&limit=10", span(1, 0), false, true},
		{"before_seq=100&limit=10", span(21, 30), true, false},
		{"after_seq=0&limit=10", span(1, 10), false, true},
		{"after_seq=25&limit=10", span(26, 30), true, false},
		{"after_seq=30&limit=10", span(1, 0), true, false},
		{"after_seq=100&limit=10", span(1, 0), true, false},
		{"around_seq=15&limit=10", span(11, 20), true, true},
		{"around_seq=1&limit=10", span(1, 10), false, true},
		{"around_seq=2&limit=10", span(1, 10), false, true},
		{"around_seq=8&limit=10", span(4, 13), true, true},
		{"around_seq=21&limit=10", span(17, 26), true, true},
		{"around_seq=29&limit=10", span(21, 30), true, false},
		{"around_seq=100&limit=10", span(21, 30), true, false},
		{"around_seq=15&limit=1", span(15, 15), true, true},
		{"around_seq=15&limit=2", span(15, 16), true, true},
		{"around_seq=30&limit=2", span(29, 30), true, false},
	} {
		wantPage(t, c.query, history(t, w, "owner", main, c.query), c.seqs, c.before, c.after)
	}
	empty := w.MainThread(w.Channel(space, "text", "empty"))
	for _, q := range []string{"", "before_seq=1", "after_seq=0", "around_seq=1", "before_seq=5", "after_seq=5"} {
		wantPage(t, "empty thread "+q, history(t, w, "owner", empty, q), span(1, 0), false, false)
	}
}

// TestListThreadMessages_PagesStableAgainstNewMessages (ADR 0015): a keyset page over thread_seq
// does not change when messages arrive.
func TestListThreadMessages_PagesStableAgainstNewMessages(t *testing.T) {
	t.Parallel()
	w := mw.New(t, subscribe)
	_, _, main := w.TextChannel()
	fill(t, w, main, 30)
	first := history(t, w, "owner", main, "before_seq=21&limit=10")
	wantPage(t, "before", first, span(11, 20), true, true)
	fill(t, w, main, 5)
	again := history(t, w, "owner", main, "before_seq=21&limit=10")
	if !bytes.Equal(first.raw, again.raw) {
		t.Fatalf("the page changed:\n%s\n%s", first.raw, again.raw)
	}
}

// TestListThreadMessages_PagingCoversThreadExactlyOnce: walking back from the newest page with
// before_seq yields every thread_seq once, each page contiguous.
func TestListThreadMessages_PagingCoversThreadExactlyOnce(t *testing.T) {
	t.Parallel()
	w := mw.New(t, subscribe)
	_, _, main := w.TextChannel()
	const n = 23
	fill(t, w, main, n)
	seen := map[int64]int{}
	p := history(t, w, "owner", main, "limit=5")
	for range n {
		s := p.seqs()
		for i := range s {
			seen[s[i]]++
			if i > 0 && s[i] != s[i-1]+1 {
				t.Fatalf("page not contiguous: %v", s)
			}
		}
		if !p.hasMoreBefore {
			break
		}
		p = history(t, w, "owner", main, fmt.Sprintf("limit=5&before_seq=%d", s[0]))
	}
	for i := int64(1); i <= n; i++ {
		if seen[i] != 1 {
			t.Errorf("thread_seq %d seen %d times", i, seen[i])
		}
	}
	if len(seen) != n {
		t.Errorf("%d distinct thread_seq, want %d", len(seen), n)
	}
}

// TestListThreadMessages_TombstonesKeepThreadSeq (03 Grundsatz 7, brief note 6): a deleted message
// stays at its position without content and mentions; the page stays contiguous.
func TestListThreadMessages_TombstonesKeepThreadSeq(t *testing.T) {
	t.Parallel()
	w := mw.New(t, subscribe)
	_, _, main := w.TextChannel()
	w.Send("owner", main, "first", http.StatusCreated)
	gone := w.Send("owner", main, "secret plan @owner", http.StatusCreated)
	w.Send("owner", main, "third", http.StatusCreated)
	w.Tombstone(ids.MustParse(gone["id"].(string)))
	p := history(t, w, "owner", main, "")
	wantPage(t, "with tombstone", p, span(1, 3), false, false)
	ts := p.items[1]
	if ts["content_md"] != "" || ts["deleted_at"] == nil || ts["version"].(float64) != 2 || ts["pinned"] != false {
		t.Errorf("tombstone %v", ts)
	}
	for _, k := range []string{"attachments", "mention_ids", "reactions", "components"} {
		if l, ok := ts[k].([]any); !ok || len(l) != 0 {
			t.Errorf("tombstone %s = %v", k, ts[k])
		}
	}
	if _, ok := ts["content_truncated"]; ok {
		t.Error("tombstone with content_truncated")
	}
	if ts["id"] != gone["id"] || ts["client_msg_id"] != gone["client_msg_id"] || ts["author"].(map[string]any)["id"] != w.OwnerID.String() {
		t.Errorf("tombstone lost its stored fields: %v", ts)
	}
	if strings.Contains(string(p.raw), "secret plan") {
		t.Error("tombstone content in the page")
	}
}

// TestListThreadMessages_ListContentCut (06 section 6 item 5): lists cut content_md above
// message.list_content_max_bytes at a UTF-8 boundary; getMessage returns it whole.
func TestListThreadMessages_ListContentCut(t *testing.T) {
	t.Parallel()
	w := mw.New(t, subscribe, func(c *messagingapp.Config) { c.ListContentMaxBytes = 10 })
	_, _, main := w.TextChannel()
	long := w.Send("owner", main, "abc€€€", http.StatusCreated) // 12 bytes
	w.Send("owner", main, "short", http.StatusCreated)
	p := history(t, w, "owner", main, "")
	if p.items[0]["content_md"] != "abc€€" || p.items[0]["content_truncated"] != true {
		t.Errorf("cut item %q %v", p.items[0]["content_md"], p.items[0]["content_truncated"])
	}
	if _, ok := p.items[1]["content_truncated"]; ok || p.items[1]["content_md"] != "short" {
		t.Errorf("short item %v", p.items[1])
	}
	full := w.Want(w.Do("owner", http.MethodGet, mw.MessagePath(ids.MustParse(long["id"].(string))), ""), http.StatusOK, "")
	if full["content_md"] != "abc€€€" {
		t.Errorf("getMessage content %q", full["content_md"])
	}
	if _, ok := full["content_truncated"]; ok {
		t.Error("getMessage with content_truncated")
	}
}

// TestListThreadMessages_LimitDefaultAndCap (06 section 6 item 3): no limit gives
// api.page_size_default, a larger limit is capped at api.page_size_max.
func TestListThreadMessages_LimitDefaultAndCap(t *testing.T) {
	t.Parallel()
	w := mw.New(t, subscribe, func(c *messagingapp.Config) { c.PageSizeDefault, c.PageSizeMax = 3, 5 })
	_, _, main := w.TextChannel()
	fill(t, w, main, 8)
	wantPage(t, "default", history(t, w, "owner", main, ""), span(6, 8), true, false)
	wantPage(t, "capped", history(t, w, "owner", main, "limit=100"), span(4, 8), true, false)
	wantPage(t, "capped after", history(t, w, "owner", main, "after_seq=0&limit=100"), span(1, 5), false, true)
}

// TestListThreadMessages_ParamErrors (brief note 2 step 1): 400 validation_failed with one entry per
// given anchor, before any lookup, so an unknown thread gets the same 400.
func TestListThreadMessages_ParamErrors(t *testing.T) {
	t.Parallel()
	w := mw.New(t, subscribe)
	_, _, main := w.TextChannel()
	for _, thread := range []ids.ID{main, ids.New()} {
		for _, c := range []struct{ query, errors string }{
			{"before_seq=5&after_seq=1", `[{"code":"mutually_exclusive","detail":"Give at most one of before_seq, after_seq and around_seq.","pointer":"#query/before_seq"},` +
				`{"code":"mutually_exclusive","detail":"Give at most one of before_seq, after_seq and around_seq.","pointer":"#query/after_seq"}]`},
			{"before_seq=5&after_seq=1&around_seq=3", `[{"code":"mutually_exclusive","detail":"Give at most one of before_seq, after_seq and around_seq.","pointer":"#query/before_seq"},` +
				`{"code":"mutually_exclusive","detail":"Give at most one of before_seq, after_seq and around_seq.","pointer":"#query/after_seq"},` +
				`{"code":"mutually_exclusive","detail":"Give at most one of before_seq, after_seq and around_seq.","pointer":"#query/around_seq"}]`},
			{"after_seq=-1", `[{"code":"out_of_range","detail":"The sequence number is out of range.","pointer":"#query/after_seq"}]`},
			{"before_seq=0", `[{"code":"out_of_range","detail":"The sequence number is out of range.","pointer":"#query/before_seq"}]`},
			{"around_seq=0", `[{"code":"out_of_range","detail":"The sequence number is out of range.","pointer":"#query/around_seq"}]`},
			{"before_seq=9007199254740992", `[{"code":"out_of_range","detail":"The sequence number is out of range.","pointer":"#query/before_seq"}]`},
		} {
			p := w.Want(w.Do("owner", http.MethodGet, mw.MessagesPath(thread)+"?"+c.query, ""), http.StatusBadRequest, "validation_failed")
			if got := mw.JSONErrors(p); got != c.errors {
				t.Errorf("%s: errors %s\nwant %s", c.query, got, c.errors)
			}
		}
	}
}

// TestListThreadMessages_InvisibleThreadIs404 (AZ-M52, 05 rule 2): Alex on a Marketing thread and
// a random UUIDv7 give the same 404.
func TestListThreadMessages_InvisibleThreadIs404(t *testing.T) {
	t.Parallel()
	w := mw.New(t, subscribe)
	support, _, _ := w.TextChannel()
	marketing := w.Space("Marketing")
	mainMarketing := w.MainThread(w.Channel(marketing, "text", "campaigns"))
	w.Send("owner", mainMarketing, "launch plan", http.StatusCreated)
	alex := w.Person("alex", false)
	w.Grant("user", alex, support, "read")
	mw.Same404(t, w.Do("alex", http.MethodGet, mw.MessagesPath(mainMarketing), ""), w.Do("alex", http.MethodGet, mw.MessagesPath(ids.New()), ""))
	mw.Same404(t, w.Do("alex", http.MethodGet, mw.MessagesPath(mainMarketing)+"?around_seq=1", ""),
		w.Do("alex", http.MethodGet, mw.MessagesPath(ids.New())+"?around_seq=1", ""))
	noSecretsInLogs(t, w, "launch plan", "around_seq")
}

// TestListThreadMessages_OperatorRuleReads (05 section 2.3 item 4): the operator agent of a
// channel reads its thread without a grant.
func TestListThreadMessages_OperatorRuleReads(t *testing.T) {
	t.Parallel()
	w := mw.New(t, subscribe)
	_, ch, main := w.TextChannel()
	fill(t, w, main, 3)
	op := w.Agent("operator-agent")
	w.SetOperator(ch, op)
	wantPage(t, "operator", history(t, w, "operator-agent", main, ""), span(1, 3), false, false)
	w.Agent("other-agent")
	w.Want(w.Do("other-agent", http.MethodGet, mw.MessagesPath(main), ""), http.StatusNotFound, "not_found")
}

// TestGetMessage_FullContentWhenEventTruncated (04 payload rule 2): the message.created row of a
// long message is cut, getMessage returns the whole content without content_truncated.
func TestGetMessage_FullContentWhenEventTruncated(t *testing.T) {
	t.Parallel()
	w := mw.New(t, subscribe)
	_, _, main := w.TextChannel()
	content := strings.Repeat("ü", mw.PayloadMax)
	m := w.Send("owner", main, content, http.StatusCreated)
	ev := w.MessageEvents(main)
	if len(ev) != 1 || !ev[0].Truncated {
		t.Fatalf("events %v", ev)
	}
	got := w.Want(w.Do("owner", http.MethodGet, mw.MessagePath(ids.MustParse(m["id"].(string))), ""), http.StatusOK, "")
	if got["content_md"] != content {
		t.Errorf("content of %d bytes, want %d", len(got["content_md"].(string)), len(content))
	}
	if _, ok := got["content_truncated"]; ok {
		t.Error("getMessage with content_truncated")
	}
}

// TestGetMessage_ETagMatchesVersion (06 section 8 item 1): ETag "<version>".
func TestGetMessage_ETagMatchesVersion(t *testing.T) {
	t.Parallel()
	w := mw.New(t, subscribe)
	_, _, main := w.TextChannel()
	id := ids.MustParse(w.Send("owner", main, "hello", http.StatusCreated)["id"].(string))
	r := w.Do("owner", http.MethodGet, mw.MessagePath(id), "")
	w.Want(r, http.StatusOK, "")
	if r.Header.Get("ETag") != `"1"` {
		t.Errorf("ETag %q", r.Header.Get("ETag"))
	}
	w.SQL(`UPDATE messages SET version = version + 1 WHERE id = $1`, id.String())
	r = w.Do("owner", http.MethodGet, mw.MessagePath(id), "")
	if b := w.Want(r, http.StatusOK, ""); r.Header.Get("ETag") != `"2"` || b["version"].(float64) != 2 {
		t.Errorf("ETag %q version %v", r.Header.Get("ETag"), b["version"])
	}
}

// TestGetMessage_Tombstone (brief note 7): a tombstone is 200 with the representation of note 6.
func TestGetMessage_Tombstone(t *testing.T) {
	t.Parallel()
	w := mw.New(t, subscribe)
	_, _, main := w.TextChannel()
	id := ids.MustParse(w.Send("owner", main, "to be removed @owner", http.StatusCreated)["id"].(string))
	w.Tombstone(id)
	r := w.Do("owner", http.MethodGet, mw.MessagePath(id), "")
	b := w.Want(r, http.StatusOK, "")
	if b["content_md"] != "" || b["deleted_at"] == nil || len(b["mention_ids"].([]any)) != 0 || r.Header.Get("ETag") != `"2"` {
		t.Errorf("tombstone %v ETag %q", b, r.Header.Get("ETag"))
	}
	if _, ok := b["content_truncated"]; ok {
		t.Error("tombstone with content_truncated")
	}
}

// TestAZM52_GetMessageForeignEqualsUnknown (AZ-M52): Alex requests a Marketing message and a random
// UUIDv7; the answers are equal.
func TestAZM52_GetMessageForeignEqualsUnknown(t *testing.T) {
	t.Parallel()
	w := mw.New(t, subscribe)
	support, _, supportMain := w.TextChannel()
	marketing := w.Space("Marketing")
	mainMarketing := w.MainThread(w.Channel(marketing, "text", "campaigns"))
	foreign := ids.MustParse(w.Send("owner", mainMarketing, "budget numbers", http.StatusCreated)["id"].(string))
	own := ids.MustParse(w.Send("owner", supportMain, "visible note", http.StatusCreated)["id"].(string))
	alex := w.Person("alex", false)
	w.Grant("user", alex, support, "read")
	mw.Same404(t, w.Do("alex", http.MethodGet, mw.MessagePath(foreign), ""), w.Do("alex", http.MethodGet, mw.MessagePath(ids.New()), ""))
	w.Want(w.Do("alex", http.MethodGet, mw.MessagePath(own), ""), http.StatusOK, "")
	noSecretsInLogs(t, w, "budget numbers", "visible note")
}

// TestGetMessage_AppWithoutReadScopeIs403 (06 section 4 item 4, 07 section 2.10, AZ-M33): an app
// with a grant on the channel but without messages:read gets 403 scope_missing from getMessage as
// from listThreadMessages; with the scope it reads both. A foreign and an unknown id stay 404.
func TestGetMessage_AppWithoutReadScopeIs403(t *testing.T) {
	t.Parallel()
	w := mw.New(t, subscribe)
	space, _, main := w.TextChannel()
	id := ids.MustParse(w.Send("owner", main, "hello", http.StatusCreated)["id"].(string))
	writer := w.App("writer-app", "messages:write")
	w.Grant("app", writer, space, "write")
	reader := w.App("reader-app", "messages:read")
	w.Grant("app", reader, space, "read")
	w.Want(w.Do("writer-app", http.MethodGet, mw.MessagesPath(main), ""), http.StatusForbidden, "scope_missing")
	w.Want(w.Do("writer-app", http.MethodGet, mw.MessagePath(id), ""), http.StatusForbidden, "scope_missing")
	w.Want(w.Do("reader-app", http.MethodGet, mw.MessagePath(id), ""), http.StatusOK, "")
	foreign := ids.MustParse(w.Send("owner", w.MainThread(w.Channel(w.Space("Marketing"), "text", "campaigns")), "plan", http.StatusCreated)["id"].(string))
	mw.Same404(t, w.Do("writer-app", http.MethodGet, mw.MessagePath(foreign), ""), w.Do("writer-app", http.MethodGet, mw.MessagePath(ids.New()), ""))
	mw.Same404(t, w.Do("reader-app", http.MethodGet, mw.MessagePath(foreign), ""), w.Do("reader-app", http.MethodGet, mw.MessagePath(ids.New()), ""))
}
