// SPDX-License-Identifier: Apache-2.0

package domain_test

import (
	"testing"

	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/notify/domain"
)

func zone(s string) *string { return &s }

func validRule() domain.RuleInput {
	return domain.RuleInput{Weekdays: 31, StartLocal: "22:00", EndLocal: "07:00", TimeZone: zone("Europe/Berlin"), Enabled: true}
}

// fieldOf returns pointer and code of the first field error of err.
func fieldOf(t *testing.T, err error) (string, string) {
	t.Helper()
	e, ok := errs.As(err)
	if !ok || e.Code != errs.CodeValidationFailed || len(e.Fields) == 0 {
		t.Fatalf("want validation_failed with a field, got %v", err)
	}
	return e.Fields[0].Pointer, e.Fields[0].Code
}

// TestValidateRuleInput: weekdays 1 to 127, local times by ParseLocalTime (start = end allowed),
// a present zone neither empty nor Local; RuleUnchanged for equal input and for each single
// differing field.
func TestValidateRuleInput(t *testing.T) {
	t.Parallel()
	for _, mod := range []func(*domain.RuleInput){
		func(*domain.RuleInput) {},
		func(r *domain.RuleInput) { r.Weekdays = 1 },
		func(r *domain.RuleInput) { r.Weekdays = 127 },
		func(r *domain.RuleInput) { r.StartLocal, r.EndLocal = "07:00", "07:00" },
		func(r *domain.RuleInput) { r.TimeZone = nil },
	} {
		in := validRule()
		mod(&in)
		if err := domain.ValidateRuleInput(in); err != nil {
			t.Errorf("%+v: %v", in, err)
		}
	}
	for _, tc := range []struct {
		mod       func(*domain.RuleInput)
		ptr, code string
	}{
		{func(r *domain.RuleInput) { r.Weekdays = 0 }, "/weekdays", domain.FieldOutOfRange},
		{func(r *domain.RuleInput) { r.Weekdays = 128 }, "/weekdays", domain.FieldOutOfRange},
		{func(r *domain.RuleInput) { r.StartLocal = "24:00" }, "/start_local", domain.FieldInvalidFormat},
		{func(r *domain.RuleInput) { r.EndLocal = "7:00" }, "/end_local", domain.FieldInvalidFormat},
		{func(r *domain.RuleInput) { r.TimeZone = zone("Local") }, "/time_zone", domain.FieldUnknownTimeZone},
		{func(r *domain.RuleInput) { r.TimeZone = zone("") }, "/time_zone", domain.FieldUnknownTimeZone},
	} {
		in := validRule()
		tc.mod(&in)
		ptr, code := fieldOf(t, domain.ValidateRuleInput(in))
		if ptr != tc.ptr || code != tc.code {
			t.Errorf("%+v: %s %s, want %s %s", in, ptr, code, tc.ptr, tc.code)
		}
	}

	if !domain.RuleUnchanged(validRule(), validRule()) {
		t.Error("equal input changed")
	}
	a, b := validRule(), validRule()
	a.TimeZone, b.TimeZone = nil, nil
	if !domain.RuleUnchanged(a, b) {
		t.Error("equal input without zone changed")
	}
	for _, mod := range []func(*domain.RuleInput){
		func(r *domain.RuleInput) { r.Weekdays = 1 },
		func(r *domain.RuleInput) { r.StartLocal = "21:00" },
		func(r *domain.RuleInput) { r.EndLocal = "06:00" },
		func(r *domain.RuleInput) { r.TimeZone = nil },
		func(r *domain.RuleInput) { r.TimeZone = zone("UTC") },
		func(r *domain.RuleInput) { r.Enabled = false },
	} {
		in := validRule()
		mod(&in)
		if domain.RuleUnchanged(validRule(), in) || domain.RuleUnchanged(in, validRule()) {
			t.Errorf("%+v counts as unchanged", in)
		}
	}
}
