// SPDX-License-Identifier: Apache-2.0

//go:build unix

package httpapi_test

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/gowebpki/jcs"

	"github.com/OWNER/bothy/services/server/api/rest"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/httpserver/httpservertest"
	"github.com/OWNER/bothy/services/server/internal/platform/testkit"
)

// TestAuditOperationsOwnerAndStepUp: all three operations refuse a person without the owner role
// (403), an agent token (403), an owner without a fresh step-up (401 step_up_required) and the
// anonymous caller (401), and serve an owner with a fresh step-up (200).
func TestAuditOperationsOwnerAndStepUp(t *testing.T) {
	t.Parallel()
	e := newEnv(t, true)
	now := e.clk.Now()
	e.owner("owner")
	e.person("member", actor.KindUser, false, now)
	e.person("stale", actor.KindUser, true, now.Add(-10*time.Minute))
	e.person("agent", actor.KindAgent, false, time.Time{})
	from, to := url.QueryEscape(now.Add(-time.Hour).Format("2006-01-02T15:04:05.000Z")), url.QueryEscape(now.Add(time.Hour).Format("2006-01-02T15:04:05.000Z"))
	ops := map[string]func(token string) *httpResult{
		"listAuditEvents":   func(tok string) *httpResult { return wrap(e.get(tok, "/api/v1/audit/events")) },
		"exportAuditEvents": func(tok string) *httpResult { return wrap(e.get(tok, "/api/v1/audit/export?from="+from+"&to="+to)) },
		"verifyAuditChain": func(tok string) *httpResult {
			resp, _ := e.verify(tok, "{}")
			return wrap(resp)
		},
	}
	for name, op := range ops {
		for _, c := range []struct {
			token  string
			status int
			code   string
		}{
			{"member", http.StatusForbidden, "forbidden"},
			{"agent", http.StatusForbidden, "forbidden"},
			{"stale", http.StatusUnauthorized, "step_up_required"},
			{"", http.StatusUnauthorized, "unauthenticated"},
			{"owner", http.StatusOK, ""},
		} {
			r := op(c.token)
			if r.status != c.status || c.code != "" && r.code != c.code {
				t.Errorf("%s as %q: %d %s, want %d %s", name, c.token, r.status, r.body, c.status, c.code)
			}
		}
	}
}

// httpResult is the status, problem code and body of a response.
type httpResult struct {
	status int
	code   string
	body   string
}

// wrap reads the problem code of resp, if any.
func wrap(resp *httpservertest.Response) *httpResult {
	r := &httpResult{status: resp.StatusCode, body: string(resp.Body)}
	if resp.Header.Get("Content-Type") == "application/problem+json" {
		var p struct {
			Code string `json:"code"`
		}
		_ = json.Unmarshal(resp.Body, &p)
		r.code = p.Code
	}
	return r
}

// TestAuditReadIsItselfAudited: one list call adds exactly one audit.read allow entry of the
// caller with the filters as IDs and codes (20 "jeder Lesezugriff steht selbst im Audit-Log"), and
// so do an export and a verification.
func TestAuditReadIsItselfAudited(t *testing.T) {
	t.Parallel()
	e := newEnv(t, false)
	o := e.owner("owner")
	e.write(2, actor.Actor{}, "grant.create", "grant", ids.New())
	before := e.count("action = 'audit.read'")
	if resp := e.get("owner", "/api/v1/audit/events?action=grant.create&decision=allow&actor_id="+o.ID.String()); resp.StatusCode != http.StatusOK {
		t.Fatalf("list: %d %s", resp.StatusCode, resp.Body)
	}
	if n := e.count("action = 'audit.read'"); n != before+1 {
		t.Fatalf("%d audit.read entries after one list, want %d", n, before+1)
	}
	if n := e.count(`action = 'audit.read' AND decision = 'allow' AND actor_principal_id = $1
		AND details - 'ip_hmac' - 'ip_key_id' = jsonb_build_object('action', 'grant.create', 'decision', 'allow', 'actor_id', $1::text)`, o.ID.String()); n != 1 {
		t.Errorf("the entry does not name the caller and the filters (%d)", n)
	}
	now := e.clk.Now()
	from, to := now.Add(-time.Hour).Format("2006-01-02T15:04:05.000Z"), now.Add(time.Hour).Format("2006-01-02T15:04:05.000Z")
	e.get("owner", "/api/v1/audit/export?from="+url.QueryEscape(from)+"&to="+url.QueryEscape(to))
	if n := e.count(`action = 'audit.read' AND details - 'ip_hmac' - 'ip_key_id' = jsonb_build_object('from', $1::text, 'to', $2::text)`, from, to); n != 1 {
		t.Errorf("%d export entries, want 1", n)
	}
	e.verify("owner", "{}")
	if n := e.count(`action = 'audit.verify' AND decision = 'allow' AND actor_principal_id = $1 AND details ? 'ok'`, o.ID.String()); n != 1 {
		t.Errorf("%d verify entries, want 1", n)
	}
}

// listPage is one page of listAuditEvents.
func (e *env) listPage(query string) (rest.AuditEventPage, int, []byte) {
	e.t.Helper()
	resp := e.get("owner", "/api/v1/audit/events"+query)
	var page rest.AuditEventPage
	if resp.StatusCode == http.StatusOK {
		if err := json.Unmarshal(resp.Body, &page); err != nil {
			e.t.Fatal(err)
		}
	}
	return page, resp.StatusCode, resp.Body
}

// TestListAuditEventsFiltersAndCursor: each filter narrows the result; a cursor used with
// another filter is 400 cursor_invalid; pages are stable while new rows arrive; newest first.
func TestListAuditEventsFiltersAndCursor(t *testing.T) {
	t.Parallel()
	e := newEnv(t, false)
	e.owner("owner")
	alex, sam := e.owner("alex"), e.owner("sam")
	res := ids.New()
	e.write(3, alex, "grant.create", "grant", res)
	e.write(2, sam, "grant.revoke", "grant", ids.New())
	e.write(1, sam, "group.create", "group", ids.New())

	countOf := func(q string) int {
		t.Helper()
		total, cursor := 0, ""
		for {
			page, status, body := e.listPage(q + cursor)
			if status != http.StatusOK {
				t.Fatalf("%s: %d %s", q, status, body)
			}
			for i := 1; i < len(page.Items); i++ {
				if page.Items[i].Seq >= page.Items[i-1].Seq {
					t.Fatalf("%s: not newest first", q)
				}
			}
			total += len(page.Items)
			if page.NextCursor == nil {
				return total
			}
			cursor = "&cursor=" + url.QueryEscape(*page.NextCursor)
		}
	}
	for q, want := range map[string]int{
		"?actor_id=" + alex.ID.String():                         3,
		"?action=grant.revoke":                                  2,
		"?resource_kind=group":                                  1,
		"?resource_id=" + res.String():                          3,
		"?actor_id=" + sam.ID.String() + "&action=grant.revoke": 2,
		"?decision=deny":                                        0,
	} {
		if got := countOf(q + "&limit=2"); got != want {
			t.Errorf("%s: %d rows, want %d", q, got, want)
		}
	}
	now := e.clk.Now()
	future := url.QueryEscape(now.Add(time.Hour).Format("2006-01-02T15:04:05.000Z"))
	if got := countOf("?from=" + future + "&limit=2"); got != 0 {
		t.Errorf("from in the future: %d rows", got)
	}

	page, _, _ := e.listPage("?action=grant.create&limit=2")
	if page.NextCursor == nil || len(page.Items) != 2 {
		t.Fatalf("first page %+v", page)
	}
	cursor := url.QueryEscape(*page.NextCursor)
	if _, status, body := e.listPage("?action=grant.revoke&limit=2&cursor=" + cursor); status != http.StatusBadRequest ||
		!strings.Contains(string(body), "cursor_invalid") {
		t.Errorf("cursor with another filter: %d %s", status, body)
	}
	e.write(2, alex, "grant.create", "grant", res)
	next, _, _ := e.listPage("?action=grant.create&limit=2&cursor=" + cursor)
	if len(next.Items) != 1 || next.Items[0].Seq >= page.Items[1].Seq {
		t.Errorf("the second page moved with new rows: %+v after %+v", next.Items, page.Items)
	}
	for _, bad := range []string{"?action=Not%20A%20Code", "?resource_kind=%40x"} {
		if _, status, body := e.listPage(bad); status != http.StatusBadRequest {
			t.Errorf("%s: %d %s", bad, status, body)
		}
	}
}

// TestExportStreamsNdjsonInSeqOrder: a missing from or to is 400; every line parses as
// AuditEvent, seq ascends, each prev_hash equals the previous line's hash, no line holds null,
// and every hash recomputes offline with RFC 8785 (an independent implementation) and SHA-256.
func TestExportStreamsNdjsonInSeqOrder(t *testing.T) {
	t.Parallel()
	e := newEnv(t, false)
	e.owner("owner")
	e.write(9, e.owner("alex"), "grant.create", "grant", ids.New())
	now := e.clk.Now()
	from := url.QueryEscape(now.Add(-time.Hour).Format("2006-01-02T15:04:05.000Z"))
	to := url.QueryEscape(now.Add(time.Hour).Format("2006-01-02T15:04:05.000Z"))
	for _, q := range []string{"", "?from=" + from, "?to=" + to} {
		if resp := e.get("owner", "/api/v1/audit/export"+q); resp.StatusCode != http.StatusBadRequest {
			t.Errorf("%q: %d %s", q, resp.StatusCode, resp.Body)
		}
	}
	resp := e.get("owner", "/api/v1/audit/export?from="+from+"&to="+to)
	if resp.StatusCode != http.StatusOK || resp.Header.Get("Content-Type") != "application/x-ndjson" {
		t.Fatalf("export: %d %q %s", resp.StatusCode, resp.Header.Get("Content-Type"), resp.Body)
	}
	if testkit.HasJSONNull(resp.Body) {
		t.Error("the stream holds null")
	}
	sc := bufio.NewScanner(bytes.NewReader(resp.Body))
	var prev *rest.AuditEvent
	lines := 0
	for sc.Scan() {
		lines++
		var ev rest.AuditEvent
		dec := json.NewDecoder(bytes.NewReader(sc.Bytes()))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&ev); err != nil {
			t.Fatalf("line %d is no AuditEvent: %v", lines, err)
		}
		if prev != nil && (ev.Seq != prev.Seq+1 || ev.PrevHash != prev.Hash) {
			t.Fatalf("line %d: seq %d prev_hash %s after seq %d hash %s", lines, ev.Seq, ev.PrevHash, prev.Seq, prev.Hash)
		}
		// Offline: drop hash, canonicalise after RFC 8785, hash with the previous hash.
		var m map[string]any
		if err := json.Unmarshal(sc.Bytes(), &m); err != nil {
			t.Fatal(err)
		}
		delete(m, "hash")
		raw, _ := json.Marshal(m)
		canonical, err := jcs.Transform(raw)
		if err != nil {
			t.Fatal(err)
		}
		prevHash, _ := hex.DecodeString(ev.PrevHash)
		sum := sha256.Sum256(append(prevHash, canonical...))
		if hex.EncodeToString(sum[:]) != ev.Hash {
			t.Fatalf("line %d: the hash does not recompute offline", lines)
		}
		prev = &ev
	}
	if lines != int(e.head()) || lines <= pageMax {
		t.Errorf("%d lines for a head of %d (batches of %d)", lines, e.head(), pageMax)
	}
}

// recomputes reports whether the AuditEvent object raw verifies offline: without hash, after RFC
// 8785 (an independent implementation), SHA-256 with prev_hash gives hash.
func recomputes(t *testing.T, raw []byte) bool {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	hash, _ := m["hash"].(string)
	delete(m, "hash")
	b, _ := json.Marshal(m)
	canonical, err := jcs.Transform(b)
	if err != nil {
		t.Fatal(err)
	}
	prev, _ := hex.DecodeString(m["prev_hash"].(string))
	sum := sha256.Sum256(append(prev, canonical...))
	return hex.EncodeToString(sum[:]) == hash
}

// TestListItemsAreCanonical: every item of listAuditEvents is the stored row in exactly the
// canonical form plus hash (19 SEC-AUD-4), so it verifies offline like an export line; times
// keep three fractional digits.
func TestListItemsAreCanonical(t *testing.T) {
	t.Parallel()
	e := newEnv(t, false)
	e.owner("owner")
	e.write(3, e.owner("alex"), "grant.create", "grant", ids.New())
	// Times whose milliseconds end in zero: a JSON encoder of time.Time drops the zeros.
	for _, ms := range []int{120, 100, 0} {
		e.appendRow(time.Date(2026, 10, 13, 8, 0, 0, ms*int(time.Millisecond), time.UTC))
	}
	var items []json.RawMessage
	for cursor := ""; ; {
		resp := e.get("owner", "/api/v1/audit/events?limit=4"+cursor)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("list: %d %s", resp.StatusCode, resp.Body)
		}
		var page struct {
			Items      []json.RawMessage `json:"items"`
			NextCursor *string           `json:"next_cursor"`
		}
		if err := json.Unmarshal(resp.Body, &page); err != nil {
			t.Fatal(err)
		}
		items = append(items, page.Items...)
		if page.NextCursor == nil {
			break
		}
		cursor = "&cursor=" + url.QueryEscape(*page.NextCursor)
	}
	if len(items) < 6 {
		t.Fatalf("%d items", len(items))
	}
	for i, it := range items {
		if !recomputes(t, it) {
			t.Errorf("item %d does not recompute offline: %s", i, it)
		}
	}
}

// TestListToFilter: to (exclusive) narrows the result like from.
func TestListToFilter(t *testing.T) {
	t.Parallel()
	e := newEnv(t, false)
	e.owner("owner")
	e.write(2, actor.Actor{}, "grant.create", "grant", ids.New())
	past := url.QueryEscape(e.clk.Now().Add(-24 * time.Hour).Format("2006-01-02T15:04:05.000Z"))
	if page, status, body := e.listPage("?to=" + past); status != http.StatusOK || len(page.Items) != 0 {
		t.Errorf("to in the past: %d %s", status, body)
	}
	future := url.QueryEscape(e.clk.Now().Add(24 * time.Hour).Format("2006-01-02T15:04:05.000Z"))
	if page, status, body := e.listPage("?to=" + future + "&action=grant.create"); status != http.StatusOK || len(page.Items) != 2 {
		t.Errorf("to in the future: %d %s", status, body)
	}
}
