// SPDX-License-Identifier: Apache-2.0

package domain

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/textcheck"
)

// MaxAmount is the largest integer the API carries exactly (06 "Basic rules", row "Integers":
// amounts stay below 2^53, so JavaScript represents them exactly).
const MaxAmount int64 = 1<<53 - 1

// The errors[].code values of this package (the list of M1-18).
const (
	codeRequired      = "required"
	codeInvalid       = "invalid"
	codeInvalidFormat = "invalid_format"
	codeInvalidEnum   = "invalid_enum"
	codeTooSmall      = "too_small"
	codeTooLarge      = "too_large"
)

// currencyForm is the form of an ISO 4217 code. It also bounds the metric attribute
// bothy.currency (signals.md section 2).
var currencyForm = regexp.MustCompile(`^[A-Z]{3}$`)

// ValidCurrency reports whether c has the ISO 4217 form of three upper-case letters.
func ValidCurrency(c string) bool { return currencyForm.MatchString(c) }

// Tokens are the consumed tokens of one report or entry (03 cost_entries).
type Tokens struct {
	In, Out, CacheRead, CacheWrite, CacheWrite1h int64
}

// Add returns the field-wise sum of t and o.
func (t Tokens) Add(o Tokens) Tokens {
	return Tokens{In: t.In + o.In, Out: t.Out + o.Out, CacheRead: t.CacheRead + o.CacheRead,
		CacheWrite: t.CacheWrite + o.CacheWrite, CacheWrite1h: t.CacheWrite1h + o.CacheWrite1h}
}

// fields returns the token counts with their member names in the order of the schema.
func (t Tokens) fields() []struct {
	name string
	n    int64
} {
	return []struct {
		name string
		n    int64
	}{
		{"tokens_in", t.In}, {"tokens_out", t.Out}, {"tokens_cache_read", t.CacheRead},
		{"tokens_cache_write", t.CacheWrite}, {"tokens_cache_write_1h", t.CacheWrite1h},
	}
}

// Report is one cost report of a finished run (schema CostReport). AmountMicros nil lets the
// server compute.
type Report struct {
	Provider, Model string
	Tokens          Tokens
	AmountMicros    *int64
	Currency        string
}

// ValidateReports checks the cost reports of a finishRun body (M2-22 design note 3). Pointers are
// relative to the list ("/0/provider"); the caller prefixes its member. Rules: provider and model
// non-empty after trimming (required) and passing textcheck.DisplayText (invalid); every token
// count and amount_micros not negative (too_small) and not above MaxAmount (too_large); currency
// of the form ^[A-Z]{3}$ (invalid_format); a second report with the same provider and model
// (invalid, pointer to the second's model), because one entry per model is what 10 section 12.1
// books. All violations are collected into one validation_failed; nil when the list is valid,
// also for an empty list.
//
// Example:
//
//	err := domain.ValidateReports([]domain.Report{{Provider: "example", Model: "model-large", Currency: "USD"}})
func ValidateReports(reports []Report) error {
	var fs []errs.FieldError
	seen := make(map[[2]string]bool, len(reports))
	for i, r := range reports {
		p := "/" + strconv.Itoa(i)
		fs = append(fs, checkName(p+"/provider", r.Provider)...)
		fs = append(fs, checkName(p+"/model", r.Model)...)
		for _, f := range r.Tokens.fields() {
			fs = append(fs, checkCount(p+"/"+f.name, f.n)...)
		}
		if r.AmountMicros != nil {
			fs = append(fs, checkCount(p+"/amount_micros", *r.AmountMicros)...)
		}
		if !ValidCurrency(r.Currency) {
			fs = append(fs, field(p+"/currency", codeInvalidFormat, "the currency is not an ISO 4217 code"))
		}
		key := [2]string{r.Provider, r.Model}
		if seen[key] {
			fs = append(fs, field(p+"/model", codeInvalid, "a second report for the same provider and model"))
		}
		seen[key] = true
	}
	return failed(fs)
}

// checkName checks a provider or model name.
func checkName(pointer, s string) []errs.FieldError {
	if strings.TrimSpace(s) == "" {
		return []errs.FieldError{field(pointer, codeRequired, "the value is required")}
	}
	if textcheck.DisplayText(s) != nil {
		return []errs.FieldError{field(pointer, codeInvalid, "the value contains characters that are not allowed")}
	}
	return nil
}

// checkCount checks a non-negative integer below the bound of 06.
func checkCount(pointer string, n int64) []errs.FieldError {
	switch {
	case n < 0:
		return []errs.FieldError{field(pointer, codeTooSmall, "the value is negative")}
	case n > MaxAmount:
		return []errs.FieldError{field(pointer, codeTooLarge, "the value is at or above 2^53")}
	}
	return nil
}

// field builds one field error.
func field(pointer, code, detail string) errs.FieldError {
	return errs.FieldError{Pointer: pointer, Code: code, Detail: detail}
}

// failed returns validation_failed with fs, nil without field errors.
func failed(fs []errs.FieldError) error {
	if len(fs) == 0 {
		return nil
	}
	return errs.New(errs.CodeValidationFailed, "the request violates a rule").WithFields(fs...)
}
