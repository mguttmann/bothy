// SPDX-License-Identifier: Apache-2.0

// Package export_test holds the full-server integration tests of brief M3-11 (exportChannel):
// the real server of internal/wire in role all through test/harness, PostgreSQL 18 from pgtest,
// the matrix world (Support/Allgemein, Support/Eskalation, Marketing/Kampagnen; Alex with G-1
// trigger and upload on Support, Sam, Robin, the agent Lotse), a steerable clock for created_at
// and the step-up age, and raw HTTP/1.1 requests with the DPoP proof of the harness clients, so
// a test reads the streamed body line by line. Two servers share the cases: one with the rate
// limits raised and small page and content limits, one with the defaults of rl.export and a
// short http.request_timeout.
package export_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"maps"
	"net/http"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/testkit"
	"github.com/OWNER/bothy/services/server/test/harness"
)

// The limits of the main test server.
const (
	pageSize       = 2
	listContentMax = 40
	stepUpMaxAge   = 10 * time.Second
	idleInTx       = 2 * time.Second
	// contentMark is in every message text of these tests, so the log check finds any content.
	contentMark = "ExportContent"
)

// mark returns a unique message text.
func mark(n int, rest string) string { return fmt.Sprintf("xq%02d%s %s", n, contentMark, rest) }

// TestExportChannel runs the cases of brief M3-11 that share one server: api.page_size_max and
// message.list_content_max_bytes small, rate limits raised, database.idle_in_tx_timeout small,
// database.lock_timeout and database.statement_timeout raised for the gate of the stream tests.
func TestExportChannel(t *testing.T) {
	e := start(t, map[string]string{
		"api.page_size_max":              strconv.Itoa(pageSize),
		"api.page_size_default":          strconv.Itoa(pageSize),
		"message.list_content_max_bytes": strconv.Itoa(listContentMax),
		"auth.step_up_max_age":           stepUpMaxAge.String(),
		"database.idle_in_tx_timeout":    idleInTx.String(),
		"database.lock_timeout":          "90s",
		"database.statement_timeout":     "90s",
		"audit.deny_aggregate_window":    "45s",
	})
	o := e.w.Owner
	alex, sam, robin := e.w.People["Alex"], e.w.People["Sam"], e.w.People["Robin"]
	allgemein, eskalation, kampagnen := e.w.Channels["Support/Allgemein"], e.w.Channels["Support/Eskalation"], e.w.Channels["Marketing/Kampagnen"]
	main := e.mainThread(t, allgemein)
	worldThread := e.w.Threads["Support/Allgemein"]

	// The range [from, to) of Support/Allgemein: messages before, at the borders and after it, a
	// long message, one that becomes a tombstone, one with characters JSON must escape, a thread
	// created inside the range and messages of other channels inside the range.
	from := e.clk.Now().Truncate(time.Millisecond).Add(time.Second)
	to := from.Add(100 * time.Millisecond)
	ms := func(n int) time.Duration { return time.Duration(n) * time.Millisecond }
	longText := mark(2, strings.Repeat("long ", 3*listContentMax))
	specialText := mark(5, "line one\nline two \u2028 \u2029 \"quoted\" back\\slash \t end")
	before := e.post(t, o, main, mark(0, "before the range"), from.Add(-ms(1)))
	atFrom := e.post(t, o, main, mark(1, "at the start"), from)
	long := e.post(t, o, main, longText, from.Add(ms(5)))
	gone := e.post(t, o, main, mark(3, "to be deleted"), from.Add(ms(10)))
	parent, thread2, first2 := e.newThread(t, o, allgemein, main, mark(4, "question"), mark(40, "first answer"), from.Add(ms(15)))
	inWorld := e.post(t, o, worldThread, mark(6, "in the old thread"), from.Add(ms(20)))
	special := e.post(t, o, worldThread, specialText, from.Add(ms(25)))
	second2 := e.post(t, o, thread2, mark(7, "second answer"), from.Add(ms(30)))
	otherEsk := e.post(t, o, e.w.Threads["Support/Eskalation"], mark(8, "other channel"), from.Add(ms(35)))
	otherKam := e.post(t, o, e.w.Threads["Marketing/Kampagnen"], mark(9, "other space"), from.Add(ms(40)))
	lastIn := e.post(t, o, main, mark(10, "just before the end"), to.Add(-ms(1)))
	atTo := e.post(t, o, worldThread, mark(11, "at the end"), to)
	after := e.post(t, o, main, mark(12, "after the range"), to.Add(ms(1)))
	e.clk.Advance(2 * time.Second)
	harness.MustDo(t, o, harness.Request{Method: http.MethodDelete, Path: "/messages/" + gone.ID}, http.StatusNoContent)
	inRange := []msg{atFrom, long, gone, parent, first2, inWorld, special, second2, lastIn}
	// The first answer of the new thread triggers the agent of the world, so the run of M2-05
	// writes its silent anchor (kind = run) into the thread; it is a message of the channel in
	// the range and the export streams it like any other (INT-W51/12).
	inRange = append(inRange, e.runAnchors(t, thread2)...)
	if got, want := sortedIDs(e.selection(t, allgemein, from, to)), sortedIDs(idsOfMsgs(inRange)); !slices.Equal(got, want) {
		t.Fatalf("selection of the range %v, want the posted messages %v", got, want)
	}
	if !atFrom.Created.Equal(from) || !atTo.Created.Equal(to) {
		t.Fatalf("created_at %v and %v, want the pinned %v and %v", atFrom.Created, atTo.Created, from, to)
	}

	t.Run("DenialsAggregated", func(t *testing.T) {
		// The aggregate is written once its window has passed, so the wait for it below needs a
		// window well inside testkit.Patience.
		if window := e.duration(t, "audit.deny_aggregate_window"); 2*window >= testkit.Patience {
			t.Fatalf("audit.deny_aggregate_window %v leaves no room within testkit.Patience %v", window, testkit.Patience)
		}
		stepUp(t, alex)
		for range 2 {
			wantProblem(t, e.get(t, e.callerOf(t, alex), exportPath(eskalation, from, to)), http.StatusForbidden, "forbidden", "capability_missing")
		}
		denies := func() []auditRow {
			var out []auditRow
			// The aggregation key is principal, action and reason (audit/domain.AggregateKey), so
			// this case runs first and alone in its window.
			for _, r := range e.audits(t, exportAction, alex.PrincipalID) {
				if r.Decision == "deny" {
					out = append(out, r)
				}
			}
			return out
		}
		// The aggregate is written once its window has passed; the window is far longer than the
		// two requests take even on a loaded runner, and later denials of Alex start a new
		// aggregate after this one is written.
		testkit.Until(t, "the deny aggregate of the export", func() bool { return len(denies()) > 0 })
		rows := denies()
		if len(rows) != 1 {
			var all []string
			r2, err := e.db.Query(t.Context(), `SELECT row_to_json(a)::text FROM (SELECT action, decision, deny_reason, resource_kind, resource_id, actor_principal_id, details
				FROM audit_events WHERE decision = 'deny' OR action = 'export.channel' ORDER BY seq) a`)
			if err == nil {
				for r2.Next() {
					var s string
					_ = r2.Scan(&s)
					all = append(all, s)
				}
				r2.Close()
			}
			t.Fatalf("%d deny entries of export.channel on Eskalation, want 1: %+v\nall deny and export rows:\n%s", len(rows), rows, strings.Join(all, "\n"))
		}
		agg, _ := rows[0].Details["aggregate"].(map[string]any)
		if rows[0].DenyReason != "capability_missing" || rows[0].ResourceKind != "channel" || rows[0].ResourceID != eskalation || fmt.Sprint(agg["count"]) != "2" {
			t.Errorf("deny entry %+v", rows[0])
		}
	})

	t.Run("WithoutExportIs403", func(t *testing.T) {
		// G-1 gives Alex trigger and upload on Support, no export.
		r := e.export(t, alex, allgemein, from, to)
		wantProblem(t, r, http.StatusForbidden, "forbidden", "capability_missing")
		if strings.Contains(string(r.body), contentMark) {
			t.Errorf("a refused export carries content: %s", r.body)
		}
		if n := len(e.allows(t, alex.PrincipalID)); n != 0 {
			t.Errorf("%d allow entries after a refusal", n)
		}
	})

	e.grantExport(t, alex, e.w.Spaces["Support"])

	t.Run("StreamsVisibleHistory", func(t *testing.T) {
		r := e.export(t, alex, allgemein, from, to)
		ls := lines(t, r)
		if got, want := idsOf(ls), e.selection(t, allgemein, from, to); !slices.Equal(got, want) {
			t.Fatalf("export %v, want %v", got, want)
		}
		checkOrder(t, ls)
		if strings.Count(string(r.body), "\n") != len(inRange) {
			t.Errorf("%d newlines for %d messages", strings.Count(string(r.body), "\n"), len(inRange))
		}
		byID := map[string]line{}
		for _, l := range ls {
			byID[l.str("id")] = l
			if l.str("channel_id") != allgemein {
				t.Errorf("line of channel %s", l.str("channel_id"))
			}
			for _, k := range []string{"content_truncated", "my_reactions"} {
				if _, ok := l[k]; ok {
					t.Errorf("line %s has %s", l.str("id"), k)
				}
			}
		}
		for _, m := range inRange {
			l := byID[m.ID]
			if l.seq(t) != m.Seq || l.str("thread_id") != m.Thread {
				t.Errorf("message %s at %s/%d, want %s/%d", m.ID, l.str("thread_id"), l.seq(t), m.Thread, m.Seq)
			}
		}
		if got := byID[long.ID].str("content_md"); got != longText {
			t.Errorf("long message has %d bytes, want the whole %d", len(got), len(longText))
		}
		if got := byID[special.ID].str("content_md"); got != specialText {
			t.Errorf("special message %q, want %q", got, specialText)
		}
		tomb := byID[gone.ID]
		if tomb.str("content_md") != "" || tomb.str("deleted_at") == "" || tomb.seq(t) != gone.Seq {
			t.Errorf("tombstone %v", tomb)
		}
		// The list of a thread cuts the same text, so message.list_content_max_bytes is in effect;
		// the export does not cut it.
		_, th, _ := e.newThread(t, o, eskalation, e.mainThread(t, eskalation), mark(13, "list"), longText, e.clk.Now())
		var list struct {
			Items []map[string]any `json:"items"`
		}
		harness.MustDo(t, alex, harness.Request{Method: http.MethodGet, Path: "/threads/" + th + "/messages"}, http.StatusOK).JSON(t, &list)
		// The run anchor of M2-05 follows the first message in the thread (INT-W51/12).
		if len(list.Items) != 1+len(e.runAnchors(t, th)) || list.Items[0]["content_truncated"] != true {
			t.Errorf("the list does not cut the long text: %v", list.Items)
		}
	})

	t.Run("OnlyThisChannelAndRange", func(t *testing.T) {
		got := idsOf(lines(t, e.export(t, alex, allgemein, from, to)))
		for name, m := range map[string]msg{"before": before, "at to": atTo, "after": after, "Eskalation": otherEsk, "Kampagnen": otherKam} {
			if slices.Contains(got, m.ID) {
				t.Errorf("the export holds the message %s", name)
			}
		}
		for name, m := range map[string]msg{"at from": atFrom, "1ms before to": lastIn} {
			if !slices.Contains(got, m.ID) {
				t.Errorf("the export lacks the message %s", name)
			}
		}
		// One millisecond more on each side moves exactly the border messages.
		wider := idsOf(lines(t, e.export(t, alex, allgemein, from.Add(-ms(1)), to.Add(ms(1)))))
		if want := append([]string{before.ID, atTo.ID}, got...); !slices.Equal(sortedIDs(wider), sortedIDs(want)) {
			t.Errorf("range widened by 1ms: %v, want %v", wider, want)
		}
		narrow := idsOf(lines(t, e.export(t, alex, allgemein, from.Add(ms(1)), to.Add(-ms(1)))))
		if slices.Contains(narrow, atFrom.ID) || slices.Contains(narrow, lastIn.ID) || len(narrow) != len(got)-2 {
			t.Errorf("range narrowed by 1ms: %v", narrow)
		}
		esk := idsOf(lines(t, e.export(t, alex, eskalation, from, to)))
		if !slices.Equal(esk, []string{otherEsk.ID}) {
			t.Errorf("export of Eskalation %v, want only %s", esk, otherEsk.ID)
		}
	})

	t.Run("EmptyRange", func(t *testing.T) {
		n := len(e.allows(t, alex.PrincipalID))
		f, l := time.Date(2001, 2, 3, 4, 5, 6, 7e6, time.UTC), time.Date(2001, 2, 3, 5, 0, 0, 0, time.UTC)
		r := e.export(t, alex, allgemein, f, l)
		if ls := lines(t, r); len(ls) != 0 || len(r.body) != 0 {
			t.Errorf("empty range: %d lines, %d bytes", len(ls), len(r.body))
		}
		rows := e.allows(t, alex.PrincipalID)
		if len(rows) != n+1 || rows[len(rows)-1].Details["from"] != ts(f) || rows[len(rows)-1].Details["to"] != ts(l) {
			t.Errorf("allow entries %d -> %d: %+v", n, len(rows), rows[len(rows)-1:])
		}
	})

	t.Run("RangeValidation", func(t *testing.T) {
		stepUp(t, alex)
		cl := e.callerOf(t, alex)
		base := "/channels/" + allgemein + "/export"
		cases := []struct {
			name, query, want string
		}{
			{"missing from", "?to=" + ts(to), "#query/from required"},
			{"missing to", "?from=" + ts(from), "#query/to required"},
			{"malformed from", "?from=yesterday&to=" + ts(to), "#query/from invalid_format"},
			{"malformed to", "?from=" + ts(from) + "&to=2026-13-45T25:61:00Z", "#query/to invalid_format"},
			{"from = to", "?from=" + ts(from) + "&to=" + ts(from), "#query/to invalid"},
			{"from > to", "?from=" + ts(to) + "&to=" + ts(from), "#query/to invalid"},
		}
		for _, c := range cases {
			r := e.get(t, cl, base+c.query)
			p := wantProblem(t, r, http.StatusBadRequest, "validation_failed", "")
			if got := fieldErrors(p); !slices.Equal(got, []string{c.want}) {
				t.Errorf("%s: errors %v, want [%s]", c.name, got, c.want)
			}
			// The same request on a channel Alex cannot see and on an unknown one answers alike.
			for _, other := range []string{kampagnen, ids.New().String()} {
				o := e.get(t, cl, "/channels/"+other+"/export"+c.query)
				if o.status != r.status || problemOf(t, o) != problemOf(t, r) {
					t.Errorf("%s on %s: %d %s, want %d %s", c.name, other, o.status, o.body, r.status, r.body)
				}
			}
		}
	})

	t.Run("InvisibleEqualsUnknown", func(t *testing.T) {
		ref := e.export(t, alex, kampagnen, from, to)
		wantProblem(t, ref, http.StatusNotFound, "not_found", "")
		for i := range 3 {
			for name, ch := range map[string]string{"Kampagnen": kampagnen, "unknown": ids.New().String()} {
				r := e.export(t, alex, ch, from, to)
				if r.status != http.StatusNotFound || problemOf(t, r) != problemOf(t, ref) || headerSet(r) != headerSet(ref) {
					t.Errorf("round %d %s: %d %s\n%s\nwant\n%s", i, name, r.status, r.body, headerSet(r), headerSet(ref))
				}
				validate(t, r)
			}
		}
	})

	t.Run("Headers", func(t *testing.T) {
		stepUp(t, alex)
		g := e.lockMessages(t)
		ctx, cancel := context.WithTimeout(t.Context(), streamWait)
		defer cancel()
		req, resp := e.open(t, ctx, e.callerOf(t, alex), exportPath(allgemein, from, to))
		defer func() { _ = resp.Body.Close() }()
		want := map[string]string{
			"X-Content-Type-Options": "nosniff", "Cache-Control": "no-store", "Cross-Origin-Resource-Policy": "same-origin", "Content-Type": mediaNDJSON,
		}
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("status %d", resp.StatusCode)
		}
		for k, v := range want {
			if got := resp.Header.Get(k); got != v {
				t.Errorf("%s = %q, want %q", k, got, v)
			}
		}
		for _, k := range []string{"Correlation-Id", "RateLimit", "RateLimit-Policy"} {
			if resp.Header.Get(k) == "" {
				t.Errorf("no %s", k)
			}
		}
		if p := resp.Header.Get("RateLimit-Policy"); !strings.HasPrefix(p, `"rl.export";`) {
			t.Errorf("RateLimit-Policy %q", p)
		}
		// The first batch reaches the client while the handler waits for the gate in its next
		// read, so it was flushed before the handler returned.
		lr := readStream(resp.Body)
		for lr.count() == 0 {
			g.step(t)
			lr.settle()
		}
		if !g.isWaiting(t) {
			g.waitFor(t)
		}
		first := lr.count()
		g.release(t)
		body, tail, err := lr.finish(t)
		if !errors.Is(err, io.EOF) || len(tail) != 0 || first < 1 || first > pageSize {
			t.Errorf("first batch %d lines; end %v tail %q", first, err, tail)
		}
		r := result{req: req, status: resp.StatusCode, header: resp.Header, body: body}
		if got := idsOf(lines(t, r)); !slices.Equal(got, e.selection(t, allgemein, from, to)) {
			t.Errorf("export %v", got)
		}
	})

	t.Run("Audit", func(t *testing.T) {
		n := len(e.allows(t, alex.PrincipalID))
		stepUp(t, alex)
		g := e.lockMessages(t)
		ctx, cancel := context.WithTimeout(t.Context(), streamWait)
		defer cancel()
		_, resp := e.open(t, ctx, e.callerOf(t, alex), exportPath(allgemein, from, to))
		defer func() { _ = resp.Body.Close() }()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("status %d", resp.StatusCode)
		}
		// Only the headers have arrived and the handler waits for its first read: the entry is
		// committed already.
		g.waitFor(t)
		rows := e.allows(t, alex.PrincipalID)
		g.release(t)
		_, _ = io.Copy(io.Discard, resp.Body)
		if len(rows) != n+1 {
			t.Fatalf("%d allow entries before the first line, want %d", len(rows), n+1)
		}
		a := rows[len(rows)-1]
		if a.ResourceKind != "channel" || a.ResourceID != allgemein || a.Actor != alex.PrincipalID || a.Session != alex.SessionID ||
			a.Device != alex.DeviceID || !strings.EqualFold(a.Correlation, resp.Header.Get("Correlation-Id")) {
			t.Errorf("allow entry %+v, want session %s device %s correlation %s", a, alex.SessionID, alex.DeviceID, resp.Header.Get("Correlation-Id"))
		}
		keys := slices.Sorted(maps.Keys(a.Details))
		keys = slices.DeleteFunc(keys, func(k string) bool { return k == "ip_hmac" || k == "ip_key_id" })
		if !slices.Equal(keys, []string{"from", "to"}) || a.Details["from"] != ts(from) || a.Details["to"] != ts(to) {
			t.Errorf("details %v, want exactly from %s and to %s", a.Details, ts(from), ts(to))
		}
		if got := len(e.allows(t, alex.PrincipalID)); got != n+1 {
			t.Errorf("%d allow entries after one export, want %d", got, n+1)
		}
	})

	// Support/Batches: more messages than three batches over four threads, so batch borders fall
	// inside threads, at their ends, and the thread pages end exactly at the page size.
	batches, batchesMain := e.newChannel(t, "Support", "Batches")
	var batchesMsgs []msg
	for i := range 3 {
		batchesMsgs = append(batchesMsgs, e.post(t, o, batchesMain, mark(20+i, "batch"), time.Time{}))
	}
	for i, extra := range []int{2, 0, 1} {
		p, th, f := e.newThread(t, o, batches, batchesMain, mark(30+i, "batch question"), mark(40+i, "batch first"), e.clk.Now())
		batchesMsgs = append(batchesMsgs, p, f)
		for j := range extra {
			batchesMsgs = append(batchesMsgs, e.post(t, o, th, mark(50+3*i+j, "batch more"), time.Time{}))
		}
		batchesMsgs = append(batchesMsgs, e.runAnchors(t, th)...) // INT-W51/12
	}
	all0, all1 := time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC), time.Date(2100, 1, 1, 0, 0, 0, 0, time.UTC)

	t.Run("BatchBorders", func(t *testing.T) {
		want := e.selection(t, batches, all0, all1)
		if len(want) != len(batchesMsgs) || len(want) <= 3*pageSize {
			t.Fatalf("selection %d messages, posted %d", len(want), len(batchesMsgs))
		}
		ls := lines(t, e.export(t, alex, batches, all0, all1))
		if got := idsOf(ls); !slices.Equal(got, want) {
			t.Fatalf("export %v, want %v", got, want)
		}
		checkOrder(t, ls)
		threads := map[string]bool{}
		for _, l := range ls {
			threads[l.str("thread_id")] = true
		}
		if len(threads) != 4 {
			t.Errorf("%d threads, want 4", len(threads))
		}
	})

	t.Run("OwnerExportsAnyChannel", func(t *testing.T) {
		got := idsOf(lines(t, e.export(t, o, kampagnen, from, to)))
		if !slices.Equal(got, []string{otherKam.ID}) || !slices.Equal(got, e.selection(t, kampagnen, from, to)) {
			t.Errorf("owner export of Kampagnen %v, want [%s]", got, otherKam.ID)
		}
	})

	t.Run("AgentAndTokens", func(t *testing.T) {
		lotse := e.w.Agents["Lotse"]
		r := e.get(t, e.callerOf(t, lotse), exportPath(allgemein, from, to))
		wantProblem(t, r, http.StatusForbidden, "forbidden", "capability_missing")
		if n := len(e.allows(t, lotse.PrincipalID)); n != 0 {
			t.Errorf("%d allow entries of the agent", n)
		}
		harness.MustDo(t, o, harness.Request{Method: http.MethodPut, Path: "/feature-flags/tokens.personal_enabled",
			Body: map[string]any{"enabled": true, "reason": "personal tokens for the export tests"}}, http.StatusOK)
		var tok struct {
			Secret string `json:"secret"`
		}
		harness.MustDo(t, alex, harness.Request{Method: http.MethodPost, Path: "/principals/" + alex.PrincipalID + "/tokens",
			Body: map[string]any{"name": "alex export", "expires_at": ts(e.clk.Now().Add(time.Hour))}}, http.StatusCreated).JSON(t, &tok)
		n := len(e.allows(t, alex.PrincipalID))
		r = e.get(t, &caller{bearer: tok.Secret}, exportPath(allgemein, from, to))
		wantProblem(t, r, http.StatusForbidden, "forbidden", "step_up_not_possible")
		if strings.Contains(string(r.body), contentMark) || len(e.allows(t, alex.PrincipalID)) != n {
			t.Errorf("personal token: body %s, allow entries %d -> %d", r.body, n, len(e.allows(t, alex.PrincipalID)))
		}
	})

	t.Run("StepUp", func(t *testing.T) {
		fresh, err := e.s.UserSession(t.Context(), alex.PrincipalID)
		if err != nil {
			t.Fatal(err)
		}
		cl := e.callerOf(t, fresh)
		path := exportPath(allgemein, from, to)
		wantProblem(t, e.get(t, cl, path), http.StatusUnauthorized, "step_up_required", "")
		stepUp(t, fresh)
		e.clk.Advance(e.duration(t, "auth.step_up_max_age") + time.Second)
		wantProblem(t, e.get(t, cl, path), http.StatusUnauthorized, "step_up_required", "")
		stepUp(t, fresh)
		if got := idsOf(lines(t, e.get(t, cl, path))); !slices.Equal(got, e.selection(t, allgemein, from, to)) {
			t.Errorf("export after a fresh step-up %v", got)
		}
	})

	// stopAtSecondThreadPage starts an export of Batches as c behind the gate and lets it run until
	// it waits for its second thread page, after all lines of the first two threads went out.
	stopAtSecondThreadPage := func(t *testing.T, c *harness.Client) (*gate, *lineReader, []string, func()) {
		t.Helper()
		// The first thread page holds the main thread and thread A (page size 2): their messages go
		// out before the second thread page is read. Read before the gate locks messages.
		var firstTwo, threadsSeen []string
		for _, id := range e.selection(t, batches, all0, all1) {
			var th string
			if err := e.db.QueryRow(t.Context(), `SELECT thread_id::text FROM messages WHERE id = $1`, id).Scan(&th); err != nil {
				t.Fatal(err)
			}
			if !slices.Contains(threadsSeen, th) {
				threadsSeen = append(threadsSeen, th)
			}
			if len(threadsSeen) > 2 {
				break
			}
			firstTwo = append(firstTwo, id)
		}
		stepUp(t, c)
		g := e.lockMessages(t)
		ctx, cancel := context.WithTimeout(t.Context(), streamWait)
		_, resp := e.open(t, ctx, e.callerOf(t, c), exportPath(batches, all0, all1)) //nolint:bodyclose // the returned done function closes it
		if resp.StatusCode != http.StatusOK {
			cancel()
			t.Fatalf("status %d", resp.StatusCode)
		}
		lr := readStream(resp.Body)
		// A statement can wait twice at the gate (pgx prepares it on the first use of a
		// connection, and parse and execute each take the lock outside a transaction), so the
		// second thread page is the thread page the export waits for after its first line.
		for g.waitFor(t) != "ListExportThreadIDs" || lr.count() == 0 {
			g.step(t)
			lr.settle()
		}
		lr.waitLines(t, len(firstTwo))
		return g, lr, firstTwo, func() { cancel(); _ = resp.Body.Close() }
	}

	// abortedAfter checks that the stream ended cut (io.ErrUnexpectedEOF), with exactly the lines
	// sent before the revocation and no problem document after them.
	abortedAfter := func(t *testing.T, lr *lineReader, want []string) {
		t.Helper()
		body, tail, err := lr.finish(t)
		if !isUnexpectedEOF(err) {
			t.Errorf("the stream ended with %v, want io.ErrUnexpectedEOF", err)
		}
		if len(tail) != 0 {
			t.Errorf("bytes after the last line: %q", tail)
		}
		if got := idsOf(decodeLines(t, body)); !slices.Equal(got, want) {
			t.Errorf("lines %v, want exactly those before the revocation %v", got, want)
		}
	}

	t.Run("RevocationBetweenBatches", func(t *testing.T) {
		grant := e.grantExport(t, robin, e.w.Spaces["Support"])
		g, lr, sent, done := stopAtSecondThreadPage(t, robin)
		defer done()
		harness.MustDo(t, o, harness.Request{Method: http.MethodPost, Path: "/grants/" + grant + "/revoke", Body: map[string]any{"reason": "export test"}}, http.StatusOK)
		g.release(t)
		abortedAfter(t, lr, sent)
		if !regexp.MustCompile(`"msg":"messaging: channel export aborted after the first byte".*"export_aborted"`).MatchString(e.logs.String()) {
			var rel []string
			for _, l := range strings.Split(e.logs.String(), "\n") {
				if strings.Contains(l, "export") || strings.Contains(l, "abort") {
					rel = append(rel, l)
				}
			}
			t.Errorf("no abort record with the cause export_aborted; related lines:\n%s", strings.Join(rel, "\n"))
		}
	})

	t.Run("SessionRevokedBetweenBatches", func(t *testing.T) {
		e.grantExport(t, robin, e.w.Spaces["Support"])
		other, err := e.s.UserSession(t.Context(), robin.PrincipalID)
		if err != nil {
			t.Fatal(err)
		}
		mark0 := len(e.logs.String())
		defer func() {
			if t.Failed() {
				for _, l := range strings.Split(e.logs.String()[mark0:], "\n") {
					if strings.Contains(l, `"level":"ERROR"`) || strings.Contains(l, "WARN") || strings.Contains(l, "mismatch") || strings.Contains(l, "/export") {
						t.Logf("server log %.500s", l)
					}
				}
			}
		}()
		g, lr, sent, done := stopAtSecondThreadPage(t, other)
		defer done()
		harness.MustDo(t, o, harness.Request{Method: http.MethodPost, Path: "/sessions/" + other.SessionID + "/revoke"}, http.StatusNoContent)
		g.release(t)
		abortedAfter(t, lr, sent)
		if !regexp.MustCompile(`"msg":"messaging: channel export aborted after the first byte".*"export_aborted"`).MatchString(e.logs.String()[mark0:]) {
			t.Error("no abort record with the cause export_aborted after the session revocation")
		}
	})

	t.Run("NoTransactionBetweenBatches", func(t *testing.T) {
		// The export of Batches is held between two batches: its message pages of the first two
		// threads went out and their units of work ended, and it waits for its next thread page
		// behind the gate (a lock of the test's own owner connection, not of the server's role).
		g, lr, sent, done := stopAtSecondThreadPage(t, alex)
		defer done()
		// Only a transaction that was open and idle before the pause began counts: the server role
		// also runs short units of work of its background jobs (runs, presence) during the pause,
		// which are idle in a transaction for a moment between two statements (INT-W51/16).
		var pauseStart time.Time
		if err := e.db.QueryRow(t.Context(), `SELECT clock_timestamp()`).Scan(&pauseStart); err != nil {
			t.Fatal(err)
		}
		pause := e.duration(t, "database.idle_in_tx_timeout") + time.Second
		for end := time.Now().Add(pause); time.Now().Before(end); {
			time.Sleep(100 * time.Millisecond)
			var n int
			if err := e.db.QueryRow(t.Context(), `SELECT count(*) FROM pg_stat_activity WHERE usename = 'bothy_app'
				AND state IN ('idle in transaction', 'idle in transaction (aborted)')
				AND xact_start < $1 AND state_change < $1`, pauseStart).Scan(&n); err != nil {
				t.Fatal(err)
			}
			if n != 0 {
				t.Fatalf("%d connections of the server idle in a transaction between two batches", n)
			}
		}
		select {
		case <-lr.done:
			t.Fatal("the export ended during the pause, so the pause did not fall between two batches")
		default:
		}
		if n := lr.count(); n != len(sent) {
			t.Fatalf("%d lines during the pause, want the %d of the first two threads", n, len(sent))
		}
		g.release(t)
		body, tail, err := lr.finish(t)
		if !errors.Is(err, io.EOF) || len(tail) != 0 {
			t.Fatalf("after a pause longer than database.idle_in_tx_timeout the stream ended with %v", err)
		}
		if got := idsOf(decodeLines(t, body)); !slices.Equal(got, e.selection(t, batches, all0, all1)) {
			t.Errorf("export after the pause %v, want the whole selection", got)
		}
	})

	t.Run("ArchivedChannel", func(t *testing.T) {
		harness.MustDo(t, o, harness.Request{Method: http.MethodPost, Path: "/channels/" + eskalation + "/archive", Body: map[string]any{"reason": "export test"}}, http.StatusOK)
		if got := idsOf(lines(t, e.export(t, alex, eskalation, from, to))); !slices.Equal(got, []string{otherEsk.ID}) {
			t.Errorf("export of the archived channel %v, want [%s]", got, otherEsk.ID)
		}
	})

	t.Run("NoContentInLogs", func(t *testing.T) {
		logs := e.logs.String()
		if !strings.Contains(logs, `"msg"`) {
			t.Fatal("no log lines captured")
		}
		names := regexp.MustCompile(`\b(Alex|Sam|Robin|Lotse)\b`)
		query := regexp.MustCompile(`[?&](from|to)=|/export\?|2100-01-01|2001-02-03`)
		for i, l := range strings.Split(logs, "\n") {
			switch {
			case strings.Contains(l, contentMark) || strings.Contains(l, "line one"):
				t.Errorf("log line %d holds message content: %.300s", i, l)
			case names.MatchString(l):
				t.Errorf("log line %d holds a display name: %.300s", i, l)
			case query.MatchString(l):
				t.Errorf("log line %d holds a query string: %.300s", i, l)
			}
		}
		for _, who := range []*harness.Client{alex, sam, robin, o} {
			for _, a := range e.allows(t, who.PrincipalID) {
				keys := slices.DeleteFunc(slices.Sorted(maps.Keys(a.Details)), func(k string) bool { return k == "ip_hmac" || k == "ip_key_id" })
				if !slices.Equal(keys, []string{"from", "to"}) {
					t.Errorf("audit details %v", a.Details)
				}
			}
		}
	})
}

// TestExportChannelLimits runs the cases that need the default rate limit of rl.export and a short
// http.request_timeout.
func TestExportChannelLimits(t *testing.T) {
	cfg := map[string]string{
		"ratelimit.rl.export.per_minute": "2",
		"ratelimit.rl.export.burst":      "2",
		"http.request_timeout":           "3s",
		"http.transfer_timeout":          "2m",
		"database.lock_timeout":          "90s",
		"database.statement_timeout":     "90s",
	}
	e := start(t, cfg)
	alex, sam, robin := e.w.People["Alex"], e.w.People["Sam"], e.w.People["Robin"]
	allgemein := e.w.Channels["Support/Allgemein"]
	for _, c := range []*harness.Client{alex, sam, robin} {
		e.grantExport(t, c, e.w.Spaces["Support"])
	}
	all0, all1 := time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC), time.Date(2100, 1, 1, 0, 0, 0, 0, time.UTC)
	want := e.selection(t, allgemein, all0, all1)
	if len(want) == 0 {
		t.Fatal("no messages in Support/Allgemein")
	}

	t.Run("RateLimit", func(t *testing.T) {
		burst, err := strconv.Atoi(e.s.ConfigValue("ratelimit.rl.export.burst"))
		if err != nil {
			t.Fatal(err)
		}
		stepUp(t, alex)
		cl := e.callerOf(t, alex)
		if r := e.get(t, cl, "/me"); r.status != http.StatusOK {
			t.Fatalf("getMe: %d %s", r.status, r.body)
		}
		for i := range burst {
			if r := e.get(t, cl, exportPath(allgemein, all0, all1)); r.status != http.StatusOK {
				t.Fatalf("export %d of the burst: %d %s", i, r.status, r.body)
			}
		}
		r := e.get(t, cl, exportPath(allgemein, all0, all1))
		wantProblem(t, r, http.StatusTooManyRequests, "rate_limited", "")
		if ra, err := strconv.Atoi(r.header.Get("Retry-After")); err != nil || ra < 1 {
			t.Errorf("Retry-After %q", r.header.Get("Retry-After"))
		}
		if p := r.header.Get("RateLimit-Policy"); p != `"rl.export";q=`+e.s.ConfigValue("ratelimit.rl.export.per_minute")+";w=60" {
			t.Errorf("RateLimit-Policy %q", p)
		}
		if s := r.header.Get("RateLimit"); !strings.HasPrefix(s, `"rl.export";r=0;`) {
			t.Errorf("RateLimit %q", s)
		}
		if got := idsOf(lines(t, e.export(t, sam, allgemein, all0, all1))); !slices.Equal(got, want) {
			t.Errorf("export by Sam right after: %v", got)
		}
	})

	t.Run("TransferDeadline", func(t *testing.T) {
		request := e.duration(t, "http.request_timeout")
		if transfer := e.duration(t, "http.transfer_timeout"); transfer <= 2*request {
			t.Fatalf("http.transfer_timeout %v must be well above http.request_timeout %v", transfer, request)
		}
		stepUp(t, robin)
		g := e.lockMessages(t)
		ctx, cancel := context.WithTimeout(t.Context(), streamWait)
		defer cancel()
		req, resp := e.open(t, ctx, e.callerOf(t, robin), exportPath(allgemein, all0, all1))
		defer func() { _ = resp.Body.Close() }()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("status %d", resp.StatusCode)
		}
		lr := readStream(resp.Body)
		g.waitFor(t)
		started := time.Now()
		time.Sleep(request + time.Second)
		// The export still waits in its read: its context has not ended at http.request_timeout.
		select {
		case <-lr.done:
			t.Fatal("the stream ended at http.request_timeout")
		default:
		}
		if !g.isWaiting(t) || time.Since(started) < request {
			t.Fatal("the export no longer waits after http.request_timeout")
		}
		g.release(t)
		body, tail, err := lr.finish(t)
		if !errors.Is(err, io.EOF) || len(tail) != 0 {
			t.Fatalf("an export longer than http.request_timeout ended with %v", err)
		}
		r := result{req: req, status: resp.StatusCode, header: resp.Header, body: body}
		if got := idsOf(lines(t, r)); !slices.Equal(got, want) {
			t.Errorf("export %v, want %v", got, want)
		}
	})
}

// sortedIDs returns ids sorted.
func sortedIDs(s []string) []string { return slices.Sorted(slices.Values(s)) }

// idsOfMsgs returns the ids of ms.
func idsOfMsgs(ms []msg) []string {
	out := make([]string, 0, len(ms))
	for _, m := range ms {
		out = append(out, m.ID)
	}
	return out
}
