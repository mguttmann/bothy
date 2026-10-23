// SPDX-License-Identifier: Apache-2.0

package costs_test

import (
	"encoding/json"
	"net/http"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/api/routes"
	"github.com/OWNER/bothy/services/server/internal/authz/adapters/fake"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/httpserver/httpservertest"
)

// priceToken registers token name as a personal API token of the person id (SR5), a private copy
// of personalToken of test/integration: the token is a fake one, so creds asks the fake for it.
func (w *world) priceToken(name string, id ids.ID) {
	tok := ids.New()
	w.creds.fake.PutToken(tok, fake.Token{PrincipalID: id})
	w.creds.mu.Lock()
	w.creds.own[tok] = true
	w.creds.mu.Unlock()
	w.setActor(name, httpservertest.WithToken(id, actor.KindUser, tok))
}

// listPrices sends listPrices as name.
func (w *world) listPrices(name string) *httpservertest.Response {
	return w.do(name, http.MethodGet, "/api/v1/prices", "")
}

// priceRows counts the rows of price_table.
func (w *world) priceRows() int64 { return w.count(`SELECT count(*) FROM price_table`) }

// priceAllows returns the allow entries of price.manage, oldest first.
func (w *world) priceAllows() []auditRow {
	w.t.Helper()
	var out []auditRow
	for _, r := range w.audits("price.manage") {
		if r.Decision == "allow" {
			out = append(out, r)
		}
	}
	return out
}

// priceDenies counts the deny entries of price.manage.
func (w *world) priceDenies() int {
	w.t.Helper()
	n := 0
	for _, r := range w.audits("price.manage") {
		if r.Decision == "deny" {
			n++
		}
	}
	return n
}

// priceJSON is a PriceCreate body of the valid members of priceBody (model, unit input_mtok,
// 1 USD per million, valid and retrieved at from, open) with the members of change replaced;
// a nil value removes the member.
func priceJSON(t *testing.T, model string, from time.Time, change map[string]any) string {
	t.Helper()
	m := map[string]any{}
	must(t, json.Unmarshal([]byte(priceBody(model, "input_mtok", 1_000_000, "USD", from, time.Time{})), &m))
	for k, v := range change {
		if v == nil {
			delete(m, k)
			continue
		}
		m[k] = v
	}
	b, err := json.Marshal(m)
	must(t, err)
	return string(b)
}

// sameTime reports whether v, a time of an answer or an audit attribute, is the time want; the
// answers drop trailing zero milliseconds, so the text form is not compared.
func sameTime(v any, want time.Time) bool {
	s, _ := v.(string)
	got, err := time.Parse(time.RFC3339Nano, s)
	return err == nil && got.Equal(want)
}

// TestListPrices (M2-22 design note 10): the owner gets every row, closed ones included, in the
// order provider, model, unit, valid_from, id; a closed row carries valid_to, an open one has
// none, and no row carries created_by. Alex gets 403 forbidden owner_only, and so does the
// owner's personal API token (05 2.3 point 1).
func TestListPrices(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	now := w.clk.Now()
	// Created out of order, so the answer cannot follow the insertion order.
	outB := w.price("model-b", "output_mtok", 15_000_000, "USD", now.Add(-2*time.Hour), time.Time{})
	inB := w.price("model-b", "input_mtok", 3_000_000, "USD", now.Add(-time.Hour), time.Time{})
	oldB := w.price("model-b", "input_mtok", 2_000_000, "USD", now.Add(-3*time.Hour), now.Add(-2*time.Hour))
	inA := w.price("model-a", "input_mtok", 1_000_000, "USD", now.Add(-time.Hour), time.Time{})
	w.wantOK(w.closePrice("owner", inA, now))
	m := w.want(w.createPrice("owner", `{"provider":"alpha","model":"model-z","unit":"request","price_micros":5,"currency":"EUR",`+
		`"valid_from":"`+stamp(now.Add(-time.Hour))+`","source_url":"`+sourceURL+`","retrieved_at":"`+stamp(now)+`"}`), http.StatusCreated, "")
	alpha := m["id"].(string)

	list := w.wantOK(w.listPrices("owner"))
	items, _ := list["items"].([]any)
	want := []struct {
		id string
		to time.Time
	}{{alpha, time.Time{}}, {inA, now}, {oldB, now.Add(-2 * time.Hour)}, {inB, time.Time{}}, {outB, time.Time{}}}
	if len(items) != len(want) || len(list) != 1 {
		t.Fatalf("list %v", list)
	}
	for i, x := range want {
		it := items[i].(map[string]any)
		to, closed := it["valid_to"]
		if it["id"] != x.id || closed == x.to.IsZero() || (closed && !sameTime(to, x.to)) {
			t.Fatalf("item %d: %v, want id %s valid_to %v", i, it, x.id, x.to)
		}
		if _, ok := it["created_by"]; ok {
			t.Fatalf("item %d carries created_by: %v", i, it)
		}
	}
	if first := items[0].(map[string]any); first["provider"] != "alpha" || first["model"] != "model-z" || first["unit"] != "request" ||
		first["price_micros"] != float64(5) || first["currency"] != "EUR" || first["source_url"] != sourceURL ||
		!sameTime(first["valid_from"], now.Add(-time.Hour)) || !sameTime(first["retrieved_at"], now) {
		t.Fatalf("first item %v", first)
	}

	w.wantReason(w.listPrices("alex"), http.StatusForbidden, "forbidden", "owner_only")
	w.priceToken("owner-token", w.owner)
	w.wantReason(w.listPrices("owner-token"), http.StatusForbidden, "forbidden", "owner_only")
}

// TestCreatePrice (M2-22 design note 11 and 14): 201 with the Price and one audit allow entry with
// row, source and retrieval time; the owner without fresh step-up gets 401 step_up_required; Alex
// gets 403 owner_only with a deny entry, the owner's personal token 403; an overlap of the same
// unit is 409 price_overlap, another currency overlapping the set 409 price_currency_mismatch;
// every rule of note 11 b is 400 validation_failed with its pointer; a replay with the same
// Idempotency-Key answers the stored 201 and inserts nothing.
func TestCreatePrice(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	now := w.clk.Now()
	from := now.Add(-time.Hour)

	r := w.createPrice("owner", priceBody("model-new", "input_mtok", 2_000_000, "USD", from, time.Time{}))
	p := w.want(r, http.StatusCreated, "")
	id, _ := p["id"].(string)
	if _, err := ids.Parse(id); err != nil || p["provider"] != provider || p["model"] != "model-new" || p["unit"] != "input_mtok" ||
		p["price_micros"] != float64(2_000_000) || p["currency"] != "USD" || !sameTime(p["valid_from"], from) ||
		p["source_url"] != sourceURL || !sameTime(p["retrieved_at"], from) || len(p) != 9 {
		t.Fatalf("created %v", p)
	}
	allows := w.priceAllows()
	if len(allows) != 1 {
		t.Fatalf("%d allow entries: %+v", len(allows), allows)
	}
	d := allows[0].Details
	if d["price_id"] != id || d["op"] != "create" || !sameTime(d["valid_from"], from) || !sameTime(d["retrieved_at"], from) ||
		d["source_url"] != sourceURL {
		t.Fatalf("audit details %v", d)
	}
	for _, k := range []string{"valid_to", "no_change"} {
		if _, ok := d[k]; ok {
			t.Fatalf("audit details of an open row carry %s: %v", k, d)
		}
	}
	// A row with an end: the Price and the audit entry carry valid_to.
	to := now.Add(time.Hour)
	p2 := w.want(w.createPrice("owner", priceBody("model-new", "output_mtok", 8_000_000, "USD", from, to)), http.StatusCreated, "")
	if !sameTime(p2["valid_to"], to) {
		t.Fatalf("created with an end %v", p2)
	}
	if allows = w.priceAllows(); len(allows) != 2 || allows[1].Details["price_id"] != p2["id"] || !sameTime(allows[1].Details["valid_to"], to) {
		t.Fatalf("audit of a row with an end: %+v", allows)
	}
	rows := w.priceRows()

	// Rights: step-up, non-owner (audited deny), personal token.
	w.session("owner-stale", w.owner, time.Time{})
	w.want(w.createPrice("owner-stale", priceBody("model-x", "input_mtok", 1, "USD", from, time.Time{})), http.StatusUnauthorized, "step_up_required")
	denies := w.priceDenies()
	w.wantReason(w.createPrice("alex", priceBody("model-x", "input_mtok", 1, "USD", from, time.Time{})), http.StatusForbidden, "forbidden", "owner_only")
	if n := w.priceDenies(); n != denies+1 {
		t.Fatalf("%d deny entries after Alex, want %d", n, denies+1)
	}
	w.priceToken("owner-token", w.owner)
	tp := w.want(w.createPrice("owner-token", priceBody("model-x", "input_mtok", 1, "USD", from, time.Time{})), http.StatusForbidden, "forbidden")
	if tp["reason"] != "owner_only" && tp["reason"] != "step_up_not_possible" {
		t.Fatalf("personal token: %v", tp)
	}

	// Conflicts.
	w.wantReason(w.createPrice("owner", priceBody("model-new", "input_mtok", 2_500_000, "USD", now.Add(-30*time.Minute), time.Time{})),
		http.StatusConflict, "conflict", "price_overlap")
	w.wantReason(w.createPrice("owner", priceBody("model-new", "cache_read_mtok", 300_000, "EUR", now.Add(-30*time.Minute), time.Time{})),
		http.StatusConflict, "conflict", "price_currency_mismatch")

	// Every rule of design note 11 b.
	cases := map[string]struct {
		change        map[string]any
		pointer, code string
	}{
		"empty provider":    {map[string]any{"provider": "  "}, "/provider", "required"},
		"control in model":  {map[string]any{"model": "m\u0007"}, "/model", "invalid"},
		"negative price":    {map[string]any{"price_micros": -1}, "/price_micros", "too_small"},
		"price at 2^53":     {map[string]any{"price_micros": int64(1) << 53}, "/price_micros", "too_large"},
		"lower currency":    {map[string]any{"currency": "usd"}, "/currency", "invalid_format"},
		"end at the start":  {map[string]any{"valid_to": stamp(from)}, "/valid_to", "invalid"},
		"http source":       {map[string]any{"source_url": "http://provider.example.com/pricing"}, "/source_url", "invalid_format"},
		"source user info":  {map[string]any{"source_url": "https://alex@provider.example.com/pricing"}, "/source_url", "invalid_format"},
		"source fragment":   {map[string]any{"source_url": "https://provider.example.com/pricing#large"}, "/source_url", "invalid_format"},
		"retrieval to come": {map[string]any{"retrieved_at": stamp(now.Add(time.Hour))}, "/retrieved_at", "invalid"},
	}
	for name, c := range cases {
		pr := w.want(w.createPrice("owner", priceJSON(t, "model-valid", from, c.change)), http.StatusBadRequest, "validation_failed")
		if got := fieldCodes(pr); got[c.pointer] != c.code {
			t.Fatalf("%s: errors %v, want %s %s", name, got, c.pointer, c.code)
		}
	}
	if n := w.priceRows(); n != rows {
		t.Fatalf("%d rows after refused creates, want %d", n, rows)
	}

	// A replay with the same Idempotency-Key answers the stored 201 and inserts nothing.
	k := key()
	body := priceBody("model-replay", "input_mtok", 1_000_000, "USD", from, time.Time{})
	first := w.createPrice("owner", body, k...)
	a := w.want(first, http.StatusCreated, "")
	replay := w.createPrice("owner", body, k...)
	b := w.want(replay, http.StatusCreated, "")
	if replay.Header.Get("Idempotent-Replay") != "true" || !reflect.DeepEqual(a, b) {
		t.Fatalf("replay %q: %v, first %v", replay.Header.Get("Idempotent-Replay"), b, a)
	}
	if n := w.priceRows(); n != rows+1 {
		t.Fatalf("%d rows after a replay, want %d", n, rows+1)
	}
	if n := len(w.priceAllows()); n != 3 {
		t.Fatalf("%d allow entries after a replay, want 3", n)
	}
}

// TestClosePrice (M2-22 design note 12): 200 with the Price and an audit entry (op close,
// valid_to); the same end again 200 with an audit entry no_change true; another end 409
// price_closed; an unknown id 404; an end not after the start 400 (invalid, /valid_to) that leaves
// the row open.
func TestClosePrice(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	now := w.clk.Now()
	from := now.Add(-2 * time.Hour)
	id := w.price("model-c", "input_mtok", 1_000_000, "USD", from, time.Time{})
	end := now.Add(-time.Hour)

	p := w.wantOK(w.closePrice("owner", id, end))
	if p["id"] != id || !sameTime(p["valid_to"], end) || !sameTime(p["valid_from"], from) {
		t.Fatalf("closed %v", p)
	}
	allows := w.priceAllows()
	last := allows[len(allows)-1].Details
	if len(allows) != 2 || last["price_id"] != id || last["op"] != "close" || !sameTime(last["valid_to"], end) ||
		!sameTime(last["valid_from"], from) || last["source_url"] != sourceURL {
		t.Fatalf("close audit %+v", allows)
	}
	if _, ok := last["no_change"]; ok {
		t.Fatalf("a close with change carries no_change: %v", last)
	}

	again := w.wantOK(w.closePrice("owner", id, end))
	if !reflect.DeepEqual(again, p) {
		t.Fatalf("the same end again: %v, first %v", again, p)
	}
	allows = w.priceAllows()
	if last = allows[len(allows)-1].Details; len(allows) != 3 || last["op"] != "close" || last["no_change"] != true || !sameTime(last["valid_to"], end) {
		t.Fatalf("no-change audit %+v", allows)
	}

	w.wantReason(w.closePrice("owner", id, now), http.StatusConflict, "conflict", "price_closed")
	w.want(w.closePrice("owner", ids.New().String(), now), http.StatusNotFound, "not_found")

	open := w.price("model-c", "output_mtok", 5_000_000, "USD", from, time.Time{})
	for _, to := range []time.Time{from, from.Add(-time.Minute)} {
		pr := w.want(w.closePrice("owner", open, to), http.StatusBadRequest, "validation_failed")
		if got := fieldCodes(pr); got["/valid_to"] != "invalid" {
			t.Fatalf("end %v: errors %v", to, got)
		}
	}
	if n := w.count(`SELECT count(*) FROM price_table WHERE id = $1 AND valid_to IS NULL`, open); n != 1 {
		t.Fatalf("a refused close changed the row")
	}
	if n := len(w.priceAllows()); n != 4 {
		t.Fatalf("%d allow entries, want 4 (two creates, a close and a no-change close)", n)
	}
	w.wantReason(w.closePrice("alex", open, now), http.StatusForbidden, "forbidden", "owner_only")
}

// TestAZP1_CostRoutes (AZ-P1, 05 rule 3): the four cost routes are enabled, each with its
// x-authz-action, its rate-limit class known to the catalogue and Idempotency-Key on the writes;
// the server of the world starts with them registered (routes.Build verifies the mounted
// patterns) and each answers through its handler.
func TestAZP1_CostRoutes(t *testing.T) {
	t.Parallel()
	want := map[string]struct {
		action, method, class string
		write                 bool
	}{
		"getThreadCosts": {"cost.read.thread", http.MethodGet, "rl.read", false},
		"listPrices":     {"price.read", http.MethodGet, "rl.read", false},
		"createPrice":    {"price.manage", http.MethodPost, "rl.admin", true},
		"closePrice":     {"price.manage", http.MethodPost, "rl.admin", true},
	}
	var found []string
	cat := routes.SpecCatalog()
	for _, op := range routes.Enabled() {
		x, ok := want[op.ID]
		if !ok {
			continue
		}
		found = append(found, op.ID)
		if op.Action != x.action || op.Method != x.method || op.RateLimitClass != x.class || op.IdempotencyKey != x.write ||
			op.IfMatch != routes.IfMatchNone {
			t.Fatalf("route %s: %+v", op.ID, op)
		}
		if class, ok := cat.Class(op.Action); !ok || class != op.RateLimitClass {
			t.Fatalf("catalogue class of %s: %q %v", op.Action, class, ok)
		}
		if got, ok := routes.ByPattern(op.Pattern); !ok || got.ID != op.ID {
			t.Fatalf("pattern %s not registered", op.Pattern)
		}
	}
	slices.Sort(found)
	if strings.Join(found, ",") != "closePrice,createPrice,getThreadCosts,listPrices" {
		t.Fatalf("cost routes enabled: %v", found)
	}

	// The world mounts the routes through routes.Build (which verifies them); each answers.
	w := newWorld(t)
	thread := w.newThread(w.allgemein)
	if b := w.costsOf(w.h, "alex", thread); b.ThreadID != thread.String() || len(b.Totals) != 0 || len(b.Runs) != 0 {
		t.Fatalf("costs of a thread without entries %+v", b)
	}
	id := w.price("model-route", "input_mtok", 1_000_000, "USD", w.clk.Now().Add(-time.Hour), time.Time{})
	if l := w.wantOK(w.listPrices("owner")); len(l["items"].([]any)) != 1 {
		t.Fatalf("list %v", l)
	}
	if p := w.wantOK(w.closePrice("owner", id, w.clk.Now())); !sameTime(p["valid_to"], w.clk.Now()) {
		t.Fatalf("close %v", p)
	}
}
