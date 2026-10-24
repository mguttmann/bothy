// SPDX-License-Identifier: Apache-2.0

package costs_test

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/billing"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// handSum sums the cost entries of the SQL condition cond per billing mode and currency, the
// reference of TestTotals_Levels.
func (w *world) handSum(cond string, args ...any) map[[2]string]int64 {
	w.t.Helper()
	rs, err := w.db.Query(context.Background(), `SELECT billing_mode, currency, sum(amount_micros)::bigint FROM cost_entries WHERE `+cond+
		` GROUP BY billing_mode, currency`, args...)
	must(w.t, err)
	defer rs.Close()
	out := map[[2]string]int64{}
	for rs.Next() {
		var mode, cur string
		var n int64
		must(w.t, rs.Scan(&mode, &cur, &n))
		out[[2]string{mode, cur}] = n
	}
	return out
}

// sums converts totals to mode and currency -> amount (hidden amounts as -1).
func sums(ts []billing.Total) map[[2]string]int64 {
	out := map[[2]string]int64{}
	for _, t := range ts {
		n := int64(-1)
		if t.AmountMicros != nil {
			n = *t.AmountMicros
		}
		out[[2]string{t.BillingMode, t.Currency}] = n
	}
	return out
}

// equalSums reports whether a and b hold the same keys and amounts.
func equalSums(a, b map[[2]string]int64) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}

// TestTotals_Levels: entries in two currencies and both modes across two threads, channels,
// spaces and agents: Totals per run, thread, session, agent, space and instance equal hand sums;
// a period cuts at From inclusive and To exclusive; a channel filter keeps only listed channels;
// an empty filter returns nothing; ForDisplay hides personal amounts unless the switch.
func TestTotals_Levels(t *testing.T) {
	w := costsWorld(t)
	ctx := context.Background()
	thread := w.newThread(w.allgemein)
	org := w.startedIn(thread, `"external_session_ref":"session-a"`)
	w.wantOK(w.costs(org, report("model-large", 10, "USD", ""), report("model-eur", 20, "EUR", "")))
	session := w.runSession(org)
	if session == "" {
		t.Fatal("the run has no session")
	}
	w.clk.Advance(time.Minute)
	notes := w.newThread(w.notizen)
	m := w.post("owner", notes, "Summarise my notes")
	pr, _ := w.runOf(m["id"].(string))
	w.wantOK(w.do("assistent", http.MethodPost, "/api/v1/runs/"+pr.ID+"/start", startBody(w.deliver(pr.ID), "personal", "chat")))
	w.wantOK(w.finish("assistent", pr.ID, `"state":"completed","costs":[`+report("model-large", 5, "USD", "")+`]`))
	personalAt := w.runCostOf(pr.ID).Finished
	all := billing.ChannelFilter{All: true}
	cases := []struct {
		name  string
		level billing.Level
		id    ids.ID
		cond  string
		arg   any
	}{
		{"run", billing.LevelRun, ids.MustParse(org), "run_id = $1", org},
		{"thread", billing.LevelThread, thread, "thread_id = $1", thread.String()},
		{"session", billing.LevelSession, ids.MustParse(session), "agent_session_id = $1", session},
		{"agent", billing.LevelAgent, w.assistent, "agent_id = $1", w.assistent.String()},
		{"space", billing.LevelSpace, w.support, "space_id = $1", w.support.String()},
		{"instance", billing.LevelInstance, ids.ID{}, "$1::text <> ''", "all"},
	}
	for _, c := range cases {
		got, err := w.billing.Service.Totals(ctx, billing.TotalsQuery{Level: c.level, ID: c.id, Channels: all})
		must(t, err)
		if want := w.handSum(c.cond, c.arg); len(want) == 0 || !equalSums(sums(got), want) {
			t.Fatalf("%s: %v, want %v", c.name, sums(got), want)
		}
	}
	from, err := w.billing.Service.Totals(ctx, billing.TotalsQuery{Level: billing.LevelInstance, Channels: all, From: personalAt})
	must(t, err)
	if !equalSums(sums(from), w.handSum("run_id = $1", pr.ID)) {
		t.Fatalf("From inclusive: %v", sums(from))
	}
	to, err := w.billing.Service.Totals(ctx, billing.TotalsQuery{Level: billing.LevelInstance, Channels: all, To: personalAt})
	must(t, err)
	if !equalSums(sums(to), w.handSum("run_id = $1", org)) {
		t.Fatalf("To exclusive: %v", sums(to))
	}
	only, err := w.billing.Service.Totals(ctx, billing.TotalsQuery{Level: billing.LevelInstance, Channels: billing.ChannelFilter{IDs: []ids.ID{w.notizen}}})
	must(t, err)
	if !equalSums(sums(only), w.handSum("channel_id = $1", w.notizen.String())) {
		t.Fatalf("channel filter: %v", sums(only))
	}
	none, err := w.billing.Service.Totals(ctx, billing.TotalsQuery{Level: billing.LevelInstance})
	must(t, err)
	if len(none) != 0 {
		t.Fatalf("an empty filter returned %v", none)
	}
	hidden, err := w.billing.Service.Totals(ctx, billing.TotalsQuery{Level: billing.LevelInstance, Channels: all, ForDisplay: true})
	must(t, err)
	if s := sums(hidden); s[[2]string{"personal", "USD"}] != -1 || s[[2]string{"organization", "USD"}] != large(10) {
		t.Fatalf("display without the switch: %v", s)
	}
	shown, err := w.billingShow.Service.Totals(ctx, billing.TotalsQuery{Level: billing.LevelInstance, Channels: all, ForDisplay: true})
	must(t, err)
	if s := sums(shown); s[[2]string{"personal", "USD"}] != large(5) {
		t.Fatalf("display with the switch: %v", s)
	}
}
