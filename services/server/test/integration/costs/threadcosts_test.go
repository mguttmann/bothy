// SPDX-License-Identifier: Apache-2.0

package costs_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/billing/adapters/billingtest"
	"github.com/OWNER/bothy/services/server/internal/billing/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/httpserver/httpservertest"
)

// sameNotFound checks that two answers are the same 404 apart from correlation_id and instance
// in the body and Correlation-Id and Date in the headers (AZ-P4).
func sameNotFound(t *testing.T, a, b *httpservertest.Response) {
	t.Helper()
	if a.StatusCode != http.StatusNotFound || b.StatusCode != http.StatusNotFound {
		t.Fatalf("statuses %d %d: %s %s", a.StatusCode, b.StatusCode, a.Body, b.Body)
	}
	ha, hb := a.Header.Clone(), b.Header.Clone()
	for _, k := range []string{"Correlation-Id", "Date"} {
		ha.Del(k)
		hb.Del(k)
	}
	if fa, fb := fmt.Sprint(ha), fmt.Sprint(hb); fa != fb {
		t.Fatalf("404 headers differ:\n%s\n%s", fa, fb)
	}
	pa, pb := a.Problem(t), b.Problem(t)
	for _, k := range []string{"correlation_id", "instance"} {
		delete(pa, k)
		delete(pb, k)
	}
	ja, _ := json.Marshal(pa)
	jb, _ := json.Marshal(pb)
	if string(ja) != string(jb) {
		t.Fatalf("404 bodies differ:\n%s\n%s", ja, jb)
	}
}

// TestGetThreadCosts_Organization: Alex reads a Support thread with three runs: runs by run_id,
// costs per currency, tokens summed, unpriced only for the run with an unpriced entry, totals per
// mode and currency, no person in the body.
func TestGetThreadCosts_Organization(t *testing.T) {
	w := costsWorld(t)
	thread, r1 := w.started()
	w.wantOK(w.costs(r1, report("model-large", 10, "USD", ""), report("model-eur", 20, "EUR", "")))
	r2 := w.startedIn(thread)
	w.wantOK(w.costs(r2, report("model-large", 30, "USD", ""), `{"provider":"example","model":"model-none","tokens_in":4,"currency":"USD"}`))
	r3 := w.startedIn(thread)
	w.wantOK(w.costs(r3, report("model-small", 5, "USD", `"amount_micros":11`)))
	r := w.threadCosts(w.h, "alex", thread)
	w.want(r, http.StatusOK, "")
	for _, person := range []string{w.alex.String(), "triggered_by"} {
		if strings.Contains(string(r.Body), person) {
			t.Fatalf("a person in the body: %s", r.Body)
		}
	}
	b := w.costsOf(w.h, "alex", thread)
	if len(b.Runs) != 3 || b.Runs[0].RunID != r1 || b.Runs[1].RunID != r2 || b.Runs[2].RunID != r3 {
		t.Fatalf("runs %+v", b.Runs)
	}
	if b.Runs[0].Unpriced || !b.Runs[1].Unpriced || b.Runs[2].Unpriced {
		t.Fatalf("unpriced %v %v %v", b.Runs[0].Unpriced, b.Runs[1].Unpriced, b.Runs[2].Unpriced)
	}
	if c := *b.Runs[0].Costs; len(c) != 2 || c[0].Currency != "EUR" || c[0].AmountMicros != large(20) || c[1].AmountMicros != large(10) {
		t.Fatalf("costs of r1 %+v", c)
	}
	if b.Runs[0].Tokens["tokens_in"] != 30 || b.Runs[1].Tokens["tokens_in"] != 34 || b.Runs[1].Tokens["tokens_out"] != 30 {
		t.Fatalf("tokens %v %v", b.Runs[0].Tokens, b.Runs[1].Tokens)
	}
	wantUSD := large(10) + large(30) + 11
	if len(b.Totals) != 2 || b.Totals[0].BillingMode != "organization" || b.Totals[0].Currency != "EUR" ||
		b.Totals[1].Currency != "USD" || b.Totals[1].AmountMicros != wantUSD || !b.Totals[1].IsEstimate {
		t.Fatalf("totals %+v (want USD %d)", b.Totals, wantUSD)
	}
}

// TestGetThreadCosts_Personal_BR11: the owner (Robin of the brief, credential holder of the
// personal agent Assistent that operates Research/Notizen) triggers Assistent, which starts with
// personal and finishes with costs: getThreadCosts shows the mode personal and the tokens, no
// costs and no personal total; on the second instance with billing.show_personal_estimates on the
// amounts appear with is_estimate.
func TestGetThreadCosts_Personal_BR11(t *testing.T) {
	w := costsWorld(t)
	thread := w.newThread(w.notizen)
	m := w.post("owner", thread, "Summarise my notes")
	rr, ok := w.runOf(m["id"].(string))
	if !ok {
		t.Fatal("no run of Assistent")
	}
	w.wantOK(w.do("assistent", http.MethodPost, "/api/v1/runs/"+rr.ID+"/start", startBody(w.deliver(rr.ID), "personal", "chat")))
	w.wantOK(w.finish("assistent", rr.ID, `"state":"completed","costs":[`+report("model-large", 100, "USD", "")+`]`))
	if es := w.entries(rr.ID); len(es) != 1 || es[0].Mode != "personal" || es[0].Amount != large(100) {
		t.Fatalf("entries %+v", es)
	}
	r := w.threadCosts(w.h, "owner", thread)
	w.want(r, http.StatusOK, "")
	if strings.Contains(string(r.Body), "amount_micros") {
		t.Fatalf("a personal amount without the switch: %s", r.Body)
	}
	b := w.costsOf(w.h, "owner", thread)
	if len(b.Runs) != 1 || b.Runs[0].BillingMode != "personal" || b.Runs[0].Costs != nil || b.Runs[0].Tokens["tokens_in"] != 100 ||
		len(b.Totals) != 0 {
		t.Fatalf("personal without the switch: %+v", b)
	}
	s := w.costsOf(w.hShow, "owner", thread)
	if s.Runs[0].Costs == nil || len(*s.Runs[0].Costs) != 1 || (*s.Runs[0].Costs)[0].AmountMicros != large(100) ||
		!(*s.Runs[0].Costs)[0].IsEstimate || len(s.Totals) != 1 || s.Totals[0].BillingMode != "personal" {
		t.Fatalf("personal with the switch: %+v", s)
	}
}

// TestGetThreadCosts_SupersededExcluded: a reconciled entry with supersedes_id replaces the
// estimate in every sum; its is_estimate false makes the sum non-estimate only when no estimate
// remains.
func TestGetThreadCosts_SupersededExcluded(t *testing.T) {
	w := costsWorld(t)
	thread, run := w.started()
	w.wantOK(w.costs(run, report("model-large", 10, "USD", "")))
	e := w.entries(run)[0]
	corr := billingtest.Entry{RunID: ids.MustParse(run), AgentID: w.lotse, ThreadID: thread, ChannelID: w.allgemein, SpaceID: w.support,
		BillingMode: "organization", Provider: provider, Model: "model-large", Tokens: domain.Tokens{In: 10, Out: 10, CacheRead: 10, CacheWrite: 10, CacheWrite1h: 10},
		AmountMicros: 777, Currency: "USD", Source: "reconciled", OccurredAt: e.Occurred, ReconciledAt: e.Occurred.Add(time.Hour),
		SupersedesID: ids.MustParse(e.ID)}
	_, err := billingtest.New(w.db).InsertEntry(context.Background(), corr)
	must(t, err)
	b := w.costsOf(w.h, "alex", thread)
	if len(b.Totals) != 1 || b.Totals[0].AmountMicros != 777 || b.Totals[0].IsEstimate {
		t.Fatalf("totals after the correction %+v", b.Totals)
	}
	if c := *b.Runs[0].Costs; len(c) != 1 || c[0].AmountMicros != 777 || c[0].IsEstimate || b.Runs[0].Tokens["tokens_in"] != 10 {
		t.Fatalf("run %+v", b.Runs[0])
	}
	run2 := w.startedIn(thread)
	w.wantOK(w.costs(run2, report("model-large", 1, "USD", "")))
	b = w.costsOf(w.h, "alex", thread)
	if len(b.Totals) != 1 || b.Totals[0].AmountMicros != 777+large(1) || !b.Totals[0].IsEstimate {
		t.Fatalf("an estimate remains: %+v", b.Totals)
	}
}

// TestGetThreadCosts_Rights_AZP4: Sam without read gets the same 404 as for an unknown thread;
// Sam with G-4 and Lotse's agent token get 200; an empty thread gives 200 with empty lists.
func TestGetThreadCosts_Rights_AZP4(t *testing.T) {
	w := costsWorld(t)
	thread, run := w.started()
	w.wantOK(w.costs(run, report("model-large", 10, "USD", "")))
	hidden := w.newThread(w.eskalation)
	sameNotFound(t, w.threadCosts(w.h, "sam", hidden), w.threadCosts(w.h, "sam", ids.New()))
	if b := w.costsOf(w.h, "sam", thread); len(b.Runs) != 1 {
		t.Fatalf("Sam with G-4: %+v", b)
	}
	if b := w.costsOf(w.h, "lotse", thread); len(b.Runs) != 1 {
		t.Fatalf("Lotse: %+v", b)
	}
	empty := w.costsOf(w.h, "alex", w.newThread(w.allgemein))
	if len(empty.Runs) != 0 || len(empty.Totals) != 0 || empty.ThreadID == "" {
		t.Fatalf("empty thread %+v", empty)
	}
}
