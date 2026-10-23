// SPDX-License-Identifier: Apache-2.0

// Package startprices is the start file of the price table (docs/spec/10-backend-policy.md
// section 12.3, PLAN row M2-22): start_prices.json, schema start_prices.v1, with the base prices
// per unit of the five token units for every model of the main price table of the public pricing
// page, its source and the time it was read. The file is product data embedded in the binary and
// checked by tests; the server never loads it at runtime, because loading it on the host needs a
// system action that 05 does not have (spec gap, ruling D-M2-22-20). Until one exists the owner
// enters prices with createPrice.
//
// Example:
//
//	rows, err := startprices.Rows()
package startprices

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/OWNER/bothy/services/server/internal/billing/domain"
)

// Schema is the value of the member schema of the start file.
const Schema = "start_prices.v1"

//go:embed start_prices.json
var file []byte

// fileRow is one row of start_prices.v1.
type fileRow struct {
	Provider    string    `json:"provider"`
	Model       string    `json:"model"`
	Unit        string    `json:"unit"`
	PriceMicros int64     `json:"price_micros"`
	Currency    string    `json:"currency"`
	ValidFrom   time.Time `json:"valid_from"`
	SourceURL   string    `json:"source_url"`
	RetrievedAt time.Time `json:"retrieved_at"`
}

// document is start_prices.v1.
type document struct {
	Schema string    `json:"schema"`
	Rows   []fileRow `json:"rows"`
}

// Rows parses the embedded start file into price rows (open validity). Errors: those of Parse.
func Rows() ([]domain.PriceInput, error) { return Parse(file) }

// Parse parses a document of schema start_prices.v1. Unknown members, another schema and data
// after the document are rejected, so a file that does not follow the schema never passes.
func Parse(b []byte) ([]domain.PriceInput, error) {
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	var d document
	if err := dec.Decode(&d); err != nil {
		return nil, fmt.Errorf("startprices: %w", err)
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return nil, errors.New("startprices: data after the document")
	}
	if d.Schema != Schema {
		return nil, fmt.Errorf("startprices: schema %q, want %q", d.Schema, Schema)
	}
	out := make([]domain.PriceInput, 0, len(d.Rows))
	for _, r := range d.Rows {
		out = append(out, domain.PriceInput{Provider: r.Provider, Model: r.Model, Unit: r.Unit, PriceMicros: r.PriceMicros,
			Currency: r.Currency, ValidFrom: r.ValidFrom, SourceURL: r.SourceURL, RetrievedAt: r.RetrievedAt})
	}
	return out, nil
}
