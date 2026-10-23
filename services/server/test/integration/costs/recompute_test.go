// SPDX-License-Identifier: Apache-2.0

package costs_test

import (
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/platform/httpserver/httpservertest"
)

// TestBookingsStayRecomputable (R-DM-23): after bookings, closePrice at a later end and a new row
// from then on leave every computed entry recomputable; closePrice with an end before a computed
// booking that used the row gives 409 price_in_use and changes nothing; the same end on a row
// whose unit had count 0 in every booking after it passes.
func TestBookingsStayRecomputable(t *testing.T) {
	w := costsWorld(t)
	_, run := w.started()
	w.wantOK(w.costs(run, report("model-large", 1000, "USD", ""),
		`{"provider":"example","model":"model-eur","tokens_in":500,"currency":"EUR"}`))
	now := w.clk.Now()
	later := now.Add(time.Hour)
	input := w.priceID("model-large", "input_mtok")
	w.wantOK(w.closePrice("owner", input, later))
	w.price("model-large", "input_mtok", 4_000_000, "USD", later, time.Time{})
	w.recompute()
	output := w.priceID("model-large", "output_mtok")
	w.wantReason(w.closePrice("owner", output, now.Add(-time.Minute)), http.StatusConflict, "conflict", "price_in_use")
	if w.count(`SELECT count(*) FROM price_table WHERE id = $1 AND valid_to IS NULL`, output) != 1 {
		t.Fatal("a refused close changed the row")
	}
	w.wantReason(w.closePrice("owner", w.priceID("model-eur", "input_mtok"), now.Add(-time.Minute)), http.StatusConflict, "conflict", "price_in_use")
	w.wantOK(w.closePrice("owner", w.priceID("model-eur", "cache_write_1h_mtok"), now.Add(-time.Minute)))
	w.recompute()
}

// TestCloseRaceWithBooking: a closePrice with an end before now and a booking at now on the same
// price set run in parallel: either the booking is computed and the close fails price_in_use, or
// the close succeeds and the booking is not computed; nothing stops recomputing.
func TestCloseRaceWithBooking(t *testing.T) {
	w := costsWorld(t)
	for i := 0; i < 3; i++ {
		_, run := w.started()
		row := w.priceID("model-large", "input_mtok")
		var wg sync.WaitGroup
		var closed, booked *httpservertest.Response
		wg.Add(2)
		go func() { defer wg.Done(); closed = w.closePrice("owner", row, w.clk.Now().Add(-time.Second)) }()
		go func() { defer wg.Done(); booked = w.costs(run, report("model-large", 10, "USD", "")) }()
		wg.Wait()
		if booked.StatusCode != http.StatusOK {
			t.Fatalf("booking: %d %s", booked.StatusCode, booked.Body)
		}
		src := w.entries(run)[0].Source
		switch {
		case closed.StatusCode == http.StatusOK && src != "computed":
		case closed.StatusCode == http.StatusConflict && src == "computed":
			w.wantReason(closed, http.StatusConflict, "conflict", "price_in_use")
		default:
			t.Fatalf("close %d %s with a %s booking", closed.StatusCode, closed.Body, src)
		}
		w.recompute()
		if closed.StatusCode == http.StatusOK {
			// A fresh complete set for the next round.
			w.price("model-large", "input_mtok", largePrices["input_mtok"], "USD", w.clk.Now(), time.Time{})
		}
		w.clk.Advance(time.Minute)
	}
}
