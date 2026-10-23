// SPDX-License-Identifier: Apache-2.0

package startprices_test

import (
	"strings"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/billing/domain"
	"github.com/OWNER/bothy/services/server/internal/billing/domain/startprices"
)

// TestStartPrices_EveryRowHasSourceAndRetrievedAt is the acceptance test of the plan (row M2-22,
// 10 section 12.3): every row of the start file has a source and a retrieval date.
func TestStartPrices_EveryRowHasSourceAndRetrievedAt(t *testing.T) {
	rows, err := startprices.Rows()
	if err != nil {
		t.Fatal(err)
	}
	for i, r := range rows {
		if strings.TrimSpace(r.SourceURL) == "" || r.RetrievedAt.IsZero() {
			t.Errorf("row %d (%s %s %s) lacks source_url or retrieved_at", i, r.Provider, r.Model, r.Unit)
		}
	}
}

// TestStartPrices_Valid: at least one row; every row passes ValidatePrice; only the five token
// units; one currency per provider and model; no overlapping validity of one provider, model and
// unit; valid_from equals retrieved_at.
func TestStartPrices_Valid(t *testing.T) {
	rows, err := startprices.Rows()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) == 0 {
		t.Fatal("the start file has no row")
	}
	token := map[string]bool{}
	for _, u := range domain.TokenUnits() {
		token[string(u)] = true
	}
	currency := map[[2]string]string{}
	seen := map[[3]string][]domain.PriceInput{}
	far := time.Date(9999, 1, 1, 0, 0, 0, 0, time.UTC)
	for i, r := range rows {
		if err := domain.ValidatePrice(r, r.RetrievedAt); err != nil {
			t.Errorf("row %d: %v", i, err)
		}
		if !token[r.Unit] {
			t.Errorf("row %d: unit %s is not a token unit", i, r.Unit)
		}
		if !r.ValidFrom.Equal(r.RetrievedAt) {
			t.Errorf("row %d: valid_from differs from retrieved_at", i)
		}
		pm := [2]string{r.Provider, r.Model}
		if c, ok := currency[pm]; ok && c != r.Currency {
			t.Errorf("row %d: two currencies for %v", i, pm)
		}
		currency[pm] = r.Currency
		k := [3]string{r.Provider, r.Model, r.Unit}
		for _, o := range seen[k] {
			endR, endO := r.ValidTo, o.ValidTo
			if endR.IsZero() {
				endR = far
			}
			if endO.IsZero() {
				endO = far
			}
			if r.ValidFrom.Before(endO) && o.ValidFrom.Before(endR) {
				t.Errorf("row %d overlaps another row of %v", i, k)
			}
		}
		seen[k] = append(seen[k], r)
	}
}

// TestStartPrices_UnknownMemberRejected: the parser rejects members outside start_prices.v1.
func TestStartPrices_UnknownMemberRejected(t *testing.T) {
	for name, doc := range map[string]string{
		"unknown row member": `{"schema":"start_prices.v1","rows":[{"provider":"example","model":"model-large","unit":"input_mtok",` +
			`"price_micros":1,"currency":"USD","valid_from":"2026-10-01T00:00:00Z","source_url":"https://provider.example.com/pricing",` +
			`"retrieved_at":"2026-10-01T00:00:00Z","discount":1}]}`,
		"unknown top member": `{"schema":"start_prices.v1","rows":[],"note":"x"}`,
		"other schema":       `{"schema":"start_prices.v2","rows":[]}`,
		"trailing data":      `{"schema":"start_prices.v1","rows":[]} {}`,
	} {
		if _, err := startprices.Parse([]byte(doc)); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}
