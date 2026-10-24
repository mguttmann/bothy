// SPDX-License-Identifier: Apache-2.0

package domain_test

import (
	"errors"
	"math/big"
	"math/rand/v2"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/billing/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

var t0 = time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)

func ptr(n int64) *int64 { return &n }

// fields returns the field errors of a validation_failed error as pointer -> code.
func fields(t *testing.T, err error) map[string]string {
	t.Helper()
	e, ok := errs.As(err)
	if !ok || e.Code != errs.CodeValidationFailed {
		t.Fatalf("want validation_failed, got %v", err)
	}
	out := map[string]string{}
	for _, f := range e.Fields {
		out[f.Pointer] = f.Code
	}
	return out
}

func report(model string) domain.Report {
	return domain.Report{Provider: "example", Model: model, Currency: "USD"}
}

func TestValidateReports(t *testing.T) {
	cases := map[string]struct {
		r       []domain.Report
		pointer string
		code    string
	}{
		"empty provider":      {r: []domain.Report{{Provider: "  ", Model: "m", Currency: "USD"}}, pointer: "/0/provider", code: "required"},
		"empty model":         {r: []domain.Report{{Provider: "p", Model: "", Currency: "USD"}}, pointer: "/0/model", code: "required"},
		"control in model":    {r: []domain.Report{{Provider: "p", Model: "m\x07", Currency: "USD"}}, pointer: "/0/model", code: "invalid"},
		"bidi in model":       {r: []domain.Report{{Provider: "p", Model: "m\u202e", Currency: "USD"}}, pointer: "/0/model", code: "invalid"},
		"negative tokens":     {r: []domain.Report{{Provider: "p", Model: "m", Currency: "USD", Tokens: domain.Tokens{Out: -1}}}, pointer: "/0/tokens_out", code: "too_small"},
		"negative 1h":         {r: []domain.Report{{Provider: "p", Model: "m", Currency: "USD", Tokens: domain.Tokens{CacheWrite1h: -1}}}, pointer: "/0/tokens_cache_write_1h", code: "too_small"},
		"negative amount":     {r: []domain.Report{{Provider: "p", Model: "m", Currency: "USD", AmountMicros: ptr(-1)}}, pointer: "/0/amount_micros", code: "too_small"},
		"tokens at the bound": {r: []domain.Report{{Provider: "p", Model: "m", Currency: "USD", Tokens: domain.Tokens{In: 1 << 53}}}, pointer: "/0/tokens_in", code: "too_large"},
		"amount at the bound": {r: []domain.Report{{Provider: "p", Model: "m", Currency: "USD", AmountMicros: ptr(1 << 53)}}, pointer: "/0/amount_micros", code: "too_large"},
		"lower-case currency": {r: []domain.Report{{Provider: "p", Model: "m", Currency: "usd"}}, pointer: "/0/currency", code: "invalid_format"},
		"four letters":        {r: []domain.Report{{Provider: "p", Model: "m", Currency: "USDT"}}, pointer: "/0/currency", code: "invalid_format"},
		"duplicate model": {r: []domain.Report{report("model-large"), report("model-small"), report("model-large")},
			pointer: "/2/model", code: "invalid"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			got := fields(t, domain.ValidateReports(c.r))
			if got[c.pointer] != c.code {
				t.Fatalf("fields %v, want %s %s", got, c.pointer, c.code)
			}
		})
	}
	ok := []domain.Report{report("model-large"), {Provider: "example", Model: "model-small", Currency: "EUR",
		Tokens: domain.Tokens{In: 1<<53 - 1}, AmountMicros: ptr(1<<53 - 1)}}
	if err := domain.ValidateReports(ok); err != nil {
		t.Fatalf("valid reports refused (absent token fields count 0): %v", err)
	}
	if err := domain.ValidateReports(nil); err != nil {
		t.Fatalf("an empty list is valid: %v", err)
	}
}

func row(model string, unit domain.Unit, price int64, cur string, from, to time.Time) domain.PriceRow {
	return domain.PriceRow{ID: ids.New(), Provider: "example", Model: model, Unit: unit, PriceMicros: price, Currency: cur,
		ValidFrom: from, ValidTo: to}
}

func TestPriceSet(t *testing.T) {
	a := row("model-large", domain.UnitInput, 1, "USD", t0, time.Time{})
	b := row("model-large", domain.UnitOutput, 1, "USD", t0.Add(-time.Hour), t0)
	c := row("model-small", domain.UnitInput, 1, "USD", t0, time.Time{})
	d := row("Model-Large", domain.UnitInput, 1, "USD", t0, time.Time{})
	rows := []domain.PriceRow{a, b, c, d}
	got := domain.PriceSet(rows, "example", "model-large", t0)
	if len(got) != 1 || got[0].ID != a.ID {
		t.Fatalf("set at valid_from: %v (valid_from included, valid_to excluded, other model and case not matched)", got)
	}
	got = domain.PriceSet(rows, "example", "model-large", t0.Add(-time.Minute))
	if len(got) != 1 || got[0].ID != b.ID {
		t.Fatalf("set before t0: %v", got)
	}
	if got := domain.PriceSet(rows, "example", "model-large", t0.Add(1000*time.Hour)); len(got) != 1 || got[0].ID != a.ID {
		t.Fatalf("open end: %v", got)
	}
}

// fullSet is a complete price set of model-large in cur with distinct prices per unit.
func fullSet(cur string) []domain.PriceRow {
	return []domain.PriceRow{
		row("model-large", domain.UnitInput, 3_000_000, cur, t0, time.Time{}),
		row("model-large", domain.UnitOutput, 15_000_000, cur, t0, time.Time{}),
		row("model-large", domain.UnitCacheRead, 300_000, cur, t0, time.Time{}),
		row("model-large", domain.UnitCacheWrite, 3_750_000, cur, t0, time.Time{}),
		row("model-large", domain.UnitCacheWrite1h, 6_000_000, cur, t0, time.Time{}),
	}
}

func TestCompute(t *testing.T) {
	all := domain.Tokens{In: 1000, Out: 2000, CacheRead: 3000, CacheWrite: 4000, CacheWrite1h: 5000}
	// 1000*3 + 2000*15 + 3000*0.3 + 4000*3.75 + 5000*6 = 3000+30000+900+15000+30000 = 78900 micros
	r := domain.Report{Provider: "example", Model: "model-large", Tokens: all, AmountMicros: ptr(1), Currency: "EUR"}
	got, err := domain.Compute(r, fullSet("CHF"))
	if err != nil || got.Source != domain.SourceComputed || got.AmountMicros != 78900 || got.Currency != "CHF" {
		t.Fatalf("complete set: %+v %v (the adapter amount and currency are ignored)", got, err)
	}
	// tokens_cache_write_1h is priced with cache_write_1h_mtok, never with cache_write_mtok.
	only1h := domain.Report{Provider: "example", Model: "model-large", Tokens: domain.Tokens{CacheWrite1h: 1_000_000}, Currency: "USD"}
	if got, _ := domain.Compute(only1h, fullSet("USD")); got.AmountMicros != 6_000_000 {
		t.Fatalf("1h cache write: %d", got.AmountMicros)
	}
	missing := fullSet("USD")[:4] // no cache_write_1h_mtok
	got, _ = domain.Compute(domain.Report{Provider: "example", Model: "model-large", Tokens: all, AmountMicros: ptr(42), Currency: "EUR"}, missing)
	if got.Source != domain.SourceAdapterReported || got.AmountMicros != 42 || got.Currency != "EUR" {
		t.Fatalf("incomplete set with amount: %+v", got)
	}
	got, _ = domain.Compute(domain.Report{Provider: "example", Model: "model-large", Tokens: all, Currency: "EUR"}, missing)
	if got.Source != domain.SourceUnpriced || got.AmountMicros != 0 || got.Currency != "EUR" {
		t.Fatalf("incomplete set without amount: %+v", got)
	}
	got, _ = domain.Compute(domain.Report{Provider: "example", Model: "model-large", Currency: "USD", AmountMicros: ptr(9)}, fullSet("USD"))
	if got.Source != domain.SourceComputed || got.AmountMicros != 0 {
		t.Fatalf("zero tokens with a set: %+v", got)
	}
	got, _ = domain.Compute(domain.Report{Provider: "example", Model: "model-large", Currency: "USD"}, nil)
	if got.Source != domain.SourceUnpriced {
		t.Fatalf("empty set: %+v", got)
	}
	mixed := fullSet("USD")
	mixed[1].Currency = "EUR"
	if got, _ := domain.Compute(domain.Report{Provider: "example", Model: "model-large", Tokens: domain.Tokens{In: 1}, Currency: "USD"}, mixed); got.Source != domain.SourceUnpriced {
		t.Fatalf("two currencies are incomplete: %+v", got)
	}
	withRequest := append(fullSet("USD"), row("model-large", domain.UnitRequest, 99, "EUR", t0, time.Time{}))
	if got, _ := domain.Compute(domain.Report{Provider: "example", Model: "model-large", Tokens: domain.Tokens{In: 1_000_000}, Currency: "USD"}, withRequest); got.Source != domain.SourceComputed || got.AmountMicros != 3_000_000 {
		t.Fatalf("the request unit is ignored: %+v", got)
	}
	half := []domain.PriceRow{row("model-large", domain.UnitInput, 1, "USD", t0, time.Time{})}
	if got, _ := domain.Compute(domain.Report{Provider: "example", Model: "model-large", Tokens: domain.Tokens{In: 500_000}, Currency: "USD"}, half); got.AmountMicros != 1 {
		t.Fatalf("0.5 micro rounds half up: %d", got.AmountMicros)
	}
	if got, _ := domain.Compute(domain.Report{Provider: "example", Model: "model-large", Tokens: domain.Tokens{In: 499_999}, Currency: "USD"}, half); got.AmountMicros != 0 {
		t.Fatalf("below 0.5 micro rounds down: %d", got.AmountMicros)
	}
	huge := []domain.PriceRow{row("model-large", domain.UnitInput, 1<<53-1, "USD", t0, time.Time{})}
	if _, err := domain.Compute(domain.Report{Provider: "example", Model: "model-large", Tokens: domain.Tokens{In: 1_000_000}, Currency: "USD"}, huge); err != nil {
		t.Fatalf("result just below the bound: %v", err)
	}
	if _, err := domain.Compute(domain.Report{Provider: "example", Model: "model-large", Tokens: domain.Tokens{In: 2_000_000}, Currency: "USD"}, huge); !errors.Is(err, domain.ErrAmountTooLarge) {
		t.Fatalf("result at the bound: %v", err)
	}
}

func TestAmountProperty(t *testing.T) {
	rng := rand.New(rand.NewPCG(1, 2)) // #nosec G404 -- reproducible test data, not a secret
	million := big.NewInt(1_000_000)
	for i := 0; i < 5000; i++ {
		set := fullSet("USD")
		for j := range set {
			set[j].PriceMicros = rng.Int64N(100_000_000)
		}
		tok := domain.Tokens{In: rng.Int64N(1 << 30), Out: rng.Int64N(1 << 30), CacheRead: rng.Int64N(1 << 30),
			CacheWrite: rng.Int64N(1 << 30), CacheWrite1h: rng.Int64N(1 << 30)}
		got, err := domain.Compute(domain.Report{Provider: "example", Model: "model-large", Tokens: tok, Currency: "USD"}, set)
		counts := []int64{tok.In, tok.Out, tok.CacheRead, tok.CacheWrite, tok.CacheWrite1h}
		sum := new(big.Int)
		for j, c := range counts {
			sum.Add(sum, new(big.Int).Mul(big.NewInt(c), big.NewInt(set[j].PriceMicros)))
		}
		q, r := new(big.Int).QuoRem(sum, million, new(big.Int))
		if new(big.Int).Mul(r, big.NewInt(2)).Cmp(million) >= 0 {
			q.Add(q, big.NewInt(1))
		}
		if !q.IsInt64() || q.Int64() > domain.MaxAmount {
			if !errors.Is(err, domain.ErrAmountTooLarge) {
				t.Fatalf("over the bound must fail: %v", err)
			}
			continue
		}
		if err != nil || got.AmountMicros != q.Int64() || got.AmountMicros < 0 {
			t.Fatalf("amount %d, reference %s, err %v", got.AmountMicros, q, err)
		}
	}
}

func TestBook(t *testing.T) {
	reps := []domain.Report{
		{Provider: "example", Model: "model-large", Tokens: domain.Tokens{In: 1_000_000}, Currency: "USD"},
		{Provider: "example", Model: "model-eur", AmountMicros: ptr(5), Currency: "EUR"},
		{Provider: "example", Model: "model-none", Currency: "USD"},
	}
	out, sums, err := domain.Book(reps, [][]domain.PriceRow{fullSet("USD"), nil, nil})
	if err != nil || len(out) != 3 || out[0].AmountMicros != 3_000_000 || out[1].Source != domain.SourceAdapterReported ||
		out[2].Source != domain.SourceUnpriced {
		t.Fatalf("book: %+v %v", out, err)
	}
	if len(sums) != 2 || sums[0].Currency != "USD" || sums[0].AmountMicros != 3_000_000 || sums[1].Currency != "EUR" || sums[1].AmountMicros != 5 {
		t.Fatalf("sums in the order of first appearance: %+v", sums)
	}
	huge := []domain.PriceRow{row("model-large", domain.UnitInput, 1<<53-1, "USD", t0, time.Time{})}
	_, _, err = domain.Book([]domain.Report{reps[2], {Provider: "example", Model: "model-large", Tokens: domain.Tokens{In: 2_000_000}, Currency: "USD"}},
		[][]domain.PriceRow{nil, huge})
	if f := fields(t, err); f["/1/amount_micros"] != "too_large" {
		t.Fatalf("too large: %v", f)
	}
	_, _, err = domain.Book([]domain.Report{{Provider: "example", Model: "a", AmountMicros: ptr(1<<53 - 1), Currency: "USD"},
		{Provider: "example", Model: "b", AmountMicros: ptr(1), Currency: "USD"}}, [][]domain.PriceRow{nil, nil})
	if f := fields(t, err); f["/1/amount_micros"] != "too_large" {
		t.Fatalf("a currency sum at the bound: %v", f)
	}
}

func TestValidatePrice(t *testing.T) {
	now := t0
	good := domain.PriceInput{Provider: "example", Model: "model-large", Unit: "input_mtok", PriceMicros: 3_000_000, Currency: "USD",
		ValidFrom: t0, SourceURL: "https://provider.example.com/pricing", RetrievedAt: t0}
	if err := domain.ValidatePrice(good, now); err != nil {
		t.Fatalf("valid: %v", err)
	}
	cases := map[string]struct {
		mod     func(p *domain.PriceInput)
		pointer string
		code    string
	}{
		"empty provider":     {func(p *domain.PriceInput) { p.Provider = " " }, "/provider", "required"},
		"control in model":   {func(p *domain.PriceInput) { p.Model = "m\x00" }, "/model", "invalid"},
		"unknown unit":       {func(p *domain.PriceInput) { p.Unit = "tokens" }, "/unit", "invalid_enum"},
		"negative price":     {func(p *domain.PriceInput) { p.PriceMicros = -1 }, "/price_micros", "too_small"},
		"price at the bound": {func(p *domain.PriceInput) { p.PriceMicros = 1 << 53 }, "/price_micros", "too_large"},
		"currency":           {func(p *domain.PriceInput) { p.Currency = "Usd" }, "/currency", "invalid_format"},
		"end before start":   {func(p *domain.PriceInput) { p.ValidTo = t0 }, "/valid_to", "invalid"},
		"http":               {func(p *domain.PriceInput) { p.SourceURL = "http://provider.example.com/pricing" }, "/source_url", "invalid_format"},
		"relative":           {func(p *domain.PriceInput) { p.SourceURL = "/pricing" }, "/source_url", "invalid_format"},
		"no host":            {func(p *domain.PriceInput) { p.SourceURL = "https:///pricing" }, "/source_url", "invalid_format"},
		"user info":          {func(p *domain.PriceInput) { p.SourceURL = "https://alex@provider.example.com/pricing" }, "/source_url", "invalid_format"},
		"fragment":           {func(p *domain.PriceInput) { p.SourceURL = "https://provider.example.com/pricing#top" }, "/source_url", "invalid_format"},
		"future retrieval":   {func(p *domain.PriceInput) { p.RetrievedAt = now.Add(time.Second) }, "/retrieved_at", "invalid"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			p := good
			c.mod(&p)
			if got := fields(t, domain.ValidatePrice(p, now)); got[c.pointer] != c.code {
				t.Fatalf("fields %v, want %s %s", got, c.pointer, c.code)
			}
		})
	}
}

func TestCloseDecision(t *testing.T) {
	open := row("model-large", domain.UnitInput, 1, "USD", t0, time.Time{})
	if got, err := domain.CloseDecision(open, t0.Add(time.Hour)); err != nil || got != domain.CloseSet {
		t.Fatalf("open row: %v %v", got, err)
	}
	closed := row("model-large", domain.UnitInput, 1, "USD", t0, t0.Add(time.Hour))
	if got, err := domain.CloseDecision(closed, t0.Add(time.Hour)); err != nil || got != domain.CloseNoChange {
		t.Fatalf("same end: %v %v", got, err)
	}
	if _, err := domain.CloseDecision(closed, t0.Add(2*time.Hour)); !errors.Is(err, domain.ErrPriceClosed) {
		t.Fatalf("other end on a closed row: %v", err)
	}
	if f := fields(t, func() error { _, err := domain.CloseDecision(open, t0); return err }()); f["/valid_to"] != "invalid" {
		t.Fatalf("end not after start: %v", f)
	}
}

func entry(run ids.ID, mode, cur, src string, amount int64, est bool) domain.EntryRow {
	return domain.EntryRow{ID: ids.New(), RunID: run, BillingMode: mode, Currency: cur, Source: src, AmountMicros: amount,
		IsEstimate: est, Tokens: domain.Tokens{In: 10, Out: 1}}
}

func TestSums(t *testing.T) {
	r1, r2 := ids.New(), ids.New()
	a := entry(r1, "organization", "USD", domain.SourceComputed, 100, true)
	b := entry(r1, "organization", "EUR", domain.SourceComputed, 7, true)
	c := entry(r2, "personal", "USD", domain.SourceComputed, 50, true)
	d := entry(r2, "organization", "USD", domain.SourceAdapterReported, 30, true)
	corr := entry(r2, "organization", "USD", domain.SourceReconciled, 31, false)
	corr.SupersedesID = d.ID
	got := domain.Sum([]domain.EntryRow{a, b, c, d, corr})
	want := []domain.Total{
		{BillingMode: "organization", Currency: "EUR", AmountMicros: 7, IsEstimate: true, Tokens: domain.Tokens{In: 10, Out: 1}, Entries: 1},
		{BillingMode: "organization", Currency: "USD", AmountMicros: 131, IsEstimate: true, Tokens: domain.Tokens{In: 20, Out: 2}, Entries: 2},
		{BillingMode: "personal", Currency: "USD", AmountMicros: 50, IsEstimate: true, Tokens: domain.Tokens{In: 10, Out: 1}, Entries: 1},
	}
	if len(got) != len(want) {
		t.Fatalf("sums %+v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("sum %d: %+v, want %+v", i, got[i], want[i])
		}
	}
	only := domain.Sum([]domain.EntryRow{d, corr})
	if len(only) != 1 || only[0].IsEstimate || only[0].AmountMicros != 31 {
		t.Fatalf("a reconciled entry alone is no estimate: %+v", only)
	}
}

func TestDisplayRule(t *testing.T) {
	r1, r2 := ids.New(), ids.New()
	if r2.String() < r1.String() {
		r1, r2 = r2, r1
	}
	org := entry(r1, "organization", "USD", domain.SourceComputed, 100, true)
	pers := entry(r2, "personal", "USD", domain.SourceUnpriced, 0, true)
	off := domain.Breakdown([]domain.EntryRow{pers, org}, false)
	if len(off.Runs) != 2 || off.Runs[0].RunID != r1 || off.Runs[1].RunID != r2 {
		t.Fatalf("runs by run id: %+v", off.Runs)
	}
	if off.Runs[0].Costs == nil || off.Runs[1].Costs != nil || off.Runs[1].Tokens.In != 10 || !off.Runs[1].Unpriced || off.Runs[0].Unpriced {
		t.Fatalf("personal without the switch: %+v", off.Runs)
	}
	if len(off.Totals) != 1 || off.Totals[0].BillingMode != "organization" {
		t.Fatalf("no personal total without the switch: %+v", off.Totals)
	}
	on := domain.Breakdown([]domain.EntryRow{pers, org}, true)
	if on.Runs[1].Costs == nil || len(on.Totals) != 2 {
		t.Fatalf("with the switch: %+v", on)
	}
	if !domain.ShowAmounts("organization", false) || domain.ShowAmounts("personal", false) || !domain.ShowAmounts("personal", true) {
		t.Fatal("ShowAmounts")
	}
}
