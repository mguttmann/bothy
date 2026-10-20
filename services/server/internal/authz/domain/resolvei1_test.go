// SPDX-License-Identifier: Apache-2.0

package domain_test

import (
	"strings"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/authz/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// TestResolveI1Validate (M2-21 design note 2): the rules of resolve_i1 beyond the schema, each
// rejection with its pointer and code; the limit audit.reason_max_chars is passed in.
func TestResolveI1Validate(t *testing.T) {
	const limit = 10
	a, b := ids.New(), ids.New()
	cases := []struct {
		name    string
		agents  []ids.ID
		reason  string
		pointer string
		code    string
	}{
		{name: "valid", agents: []ids.ID{a, b}, reason: "Deputy"},
		{name: "empty list", agents: nil, reason: "x", pointer: "/resolve_i1/switch_to_organization", code: "too_few_items"},
		{name: "repeated id", agents: []ids.ID{a, b, a}, reason: "x", pointer: "/resolve_i1/switch_to_organization/2", code: "invalid"},
		{name: "blank reason", agents: []ids.ID{a}, reason: " \t ", pointer: "/resolve_i1/reason", code: "required"},
		{name: "control character", agents: []ids.ID{a}, reason: "bad\x07", pointer: "/resolve_i1/reason", code: "control_character"},
		{name: "bidi character", agents: []ids.ID{a}, reason: "bad\u202e", pointer: "/resolve_i1/reason", code: "bidi_control"},
		{name: "exactly the limit", agents: []ids.ID{a}, reason: strings.Repeat("ä", limit)},
		{name: "one above the limit", agents: []ids.ID{a}, reason: strings.Repeat("ä", limit+1), pointer: "/resolve_i1/reason", code: "too_long"},
		{name: "zero id", agents: []ids.ID{{}}, reason: "x", pointer: "/resolve_i1/switch_to_organization/0", code: "invalid"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := domain.CheckResolveI1(c.agents, c.reason, limit)
			if c.pointer == "" {
				if err != nil {
					t.Fatalf("CheckResolveI1: %v", err)
				}
				return
			}
			e, ok := errs.As(err)
			if !ok || e.Code != errs.CodeValidationFailed || len(e.Fields) != 1 {
				t.Fatalf("CheckResolveI1: %v, want one field error", err)
			}
			if f := e.Fields[0]; f.Pointer != c.pointer || f.Code != c.code {
				t.Fatalf("field %+v, want %s %s", f, c.pointer, c.code)
			}
		})
	}
}

// TestResolveI1ValidateCollectsAll: an empty list and a bad reason come in one answer (06
// section 5 point 3).
func TestResolveI1ValidateCollectsAll(t *testing.T) {
	e, ok := errs.As(domain.CheckResolveI1(nil, "", 10))
	if !ok || len(e.Fields) != 2 {
		t.Fatalf("got %v, want two field errors", e)
	}
}

// TestResolveI1ValidateNeedsLimit: a limit below 1 is a wiring error, never "no limit".
func TestResolveI1ValidateNeedsLimit(t *testing.T) {
	if e, ok := errs.As(domain.CheckResolveI1([]ids.ID{ids.New()}, "x", 0)); !ok || e.Code != errs.CodeInternalError {
		t.Fatalf("got %v, want internal_error", e)
	}
}
