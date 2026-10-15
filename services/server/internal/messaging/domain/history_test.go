// SPDX-License-Identifier: Apache-2.0

package domain_test

import (
	"slices"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/messaging/domain"
)

func ptr(v int64) *int64 { return &v }

// TestValidateHistoryRequest (brief M1-57 note 2 step 1): at most one anchor, after_seq >= 0,
// before_seq and around_seq >= 1, every anchor below 2^53.
func TestValidateHistoryRequest(t *testing.T) {
	t.Parallel()
	const max = int64(1)<<53 - 1
	type field struct{ pointer, code string }
	for _, c := range []struct {
		name                string
		before, after, near *int64
		mode                domain.HistoryMode
		anchor              int64
		fields              []field
	}{
		{name: "none", mode: domain.HistoryNewest},
		{name: "before", before: ptr(5), mode: domain.HistoryBefore, anchor: 5},
		{name: "after zero", after: ptr(0), mode: domain.HistoryAfter, anchor: 0},
		{name: "around one", near: ptr(1), mode: domain.HistoryAround, anchor: 1},
		{name: "before max", before: ptr(max), mode: domain.HistoryBefore, anchor: max},
		{name: "two", before: ptr(5), after: ptr(1), fields: []field{{"#query/before_seq", "mutually_exclusive"}, {"#query/after_seq", "mutually_exclusive"}}},
		{name: "three", before: ptr(5), after: ptr(1), near: ptr(3), fields: []field{
			{"#query/before_seq", "mutually_exclusive"}, {"#query/after_seq", "mutually_exclusive"}, {"#query/around_seq", "mutually_exclusive"},
		}},
		{name: "after and around", after: ptr(-1), near: ptr(0), fields: []field{{"#query/after_seq", "mutually_exclusive"}, {"#query/around_seq", "mutually_exclusive"}}},
		{name: "after negative", after: ptr(-1), fields: []field{{"#query/after_seq", "out_of_range"}}},
		{name: "before zero", before: ptr(0), fields: []field{{"#query/before_seq", "out_of_range"}}},
		{name: "before negative", before: ptr(-3), fields: []field{{"#query/before_seq", "out_of_range"}}},
		{name: "around zero", near: ptr(0), fields: []field{{"#query/around_seq", "out_of_range"}}},
		{name: "before 2^53", before: ptr(max + 1), fields: []field{{"#query/before_seq", "out_of_range"}}},
		{name: "after 2^53", after: ptr(max + 1), fields: []field{{"#query/after_seq", "out_of_range"}}},
		{name: "around 2^53", near: ptr(max + 1), fields: []field{{"#query/around_seq", "out_of_range"}}},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			mode, anchor, err := domain.ValidateHistoryRequest(c.before, c.after, c.near)
			if c.fields == nil {
				if err != nil || mode != c.mode || anchor != c.anchor {
					t.Fatalf("got %v %d %v, want %v %d", mode, anchor, err, c.mode, c.anchor)
				}
				return
			}
			e, ok := errs.As(err)
			if !ok || e.Code != errs.CodeValidationFailed {
				t.Fatalf("error %v, want validation_failed", err)
			}
			var got []field
			for _, f := range e.Fields {
				got = append(got, field{f.Pointer, f.Code})
			}
			if !slices.Equal(got, c.fields) {
				t.Fatalf("fields %v, want %v", got, c.fields)
			}
		})
	}
}

// TestPickAround (brief note 3): b0 = min(|B|, (L-1)/2), a = min(|A|, L-b0), b = min(|B|, L-a).
func TestPickAround(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name                 string
		below, above, limit  int
		wantBelow, wantAbove int
	}{
		{"middle of a long thread", 11, 11, 10, 4, 6},
		{"near the start fills from above", 2, 11, 10, 2, 8},
		{"at the start", 0, 11, 10, 0, 10},
		{"near the end fills from below", 11, 3, 10, 7, 3},
		{"anchor beyond the last message", 11, 0, 10, 10, 0},
		{"short thread", 3, 2, 10, 3, 2},
		{"L = 1", 2, 2, 1, 0, 1},
		{"L = 1 at the end", 2, 0, 1, 1, 0},
		{"L = 2", 3, 3, 2, 0, 2},
		{"L = 2 at the end", 3, 1, 2, 1, 1},
		{"L = 2 one above", 0, 1, 2, 0, 1},
		{"empty thread", 0, 0, 10, 0, 0},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			b, a := domain.PickAround(c.below, c.above, c.limit)
			if b != c.wantBelow || a != c.wantAbove {
				t.Fatalf("PickAround(%d, %d, %d) = %d, %d; want %d, %d", c.below, c.above, c.limit, b, a, c.wantBelow, c.wantAbove)
			}
		})
	}
}

// TestPageFlags (brief note 4): every mode with full, partial and empty results. below and above
// are the rows the statement returned per side (at most L + 1), opposite its EXISTS.
func TestPageFlags(t *testing.T) {
	t.Parallel()
	const l = 10
	for _, c := range []struct {
		name         string
		mode         domain.HistoryMode
		below, above int
		opposite     bool
		want         domain.Window
	}{
		{"newest full", domain.HistoryNewest, l + 1, 0, false, domain.Window{Below: l, HasMoreBefore: true}},
		{"newest exactly L", domain.HistoryNewest, l, 0, false, domain.Window{Below: l}},
		{"newest partial", domain.HistoryNewest, 3, 0, false, domain.Window{Below: 3}},
		{"newest empty", domain.HistoryNewest, 0, 0, false, domain.Window{}},
		{"before full", domain.HistoryBefore, l + 1, 0, true, domain.Window{Below: l, HasMoreBefore: true, HasMoreAfter: true}},
		{"before partial", domain.HistoryBefore, 4, 0, true, domain.Window{Below: 4, HasMoreAfter: true}},
		{"before partial at the end", domain.HistoryBefore, 4, 0, false, domain.Window{Below: 4}},
		{"before empty with later", domain.HistoryBefore, 0, 0, true, domain.Window{HasMoreAfter: true}},
		{"before empty", domain.HistoryBefore, 0, 0, false, domain.Window{}},
		{"after full", domain.HistoryAfter, 0, l + 1, true, domain.Window{Above: l, HasMoreBefore: true, HasMoreAfter: true}},
		{"after partial", domain.HistoryAfter, 0, 2, true, domain.Window{Above: 2, HasMoreBefore: true}},
		{"after from zero", domain.HistoryAfter, 0, 2, false, domain.Window{Above: 2}},
		{"after empty with earlier", domain.HistoryAfter, 0, 0, true, domain.Window{HasMoreBefore: true}},
		{"after empty", domain.HistoryAfter, 0, 0, false, domain.Window{}},
		{"around full", domain.HistoryAround, l + 1, l + 1, false, domain.Window{Below: 4, Above: 6, HasMoreBefore: true, HasMoreAfter: true}},
		{"around near the start", domain.HistoryAround, 2, l + 1, false, domain.Window{Below: 2, Above: 8, HasMoreAfter: true}},
		{"around near the end", domain.HistoryAround, l + 1, 3, false, domain.Window{Below: 7, Above: 3, HasMoreBefore: true}},
		{"around partial below", domain.HistoryAround, 7, l + 1, false, domain.Window{Below: 4, Above: 6, HasMoreBefore: true, HasMoreAfter: true}},
		{"around partial above", domain.HistoryAround, l + 1, 7, false, domain.Window{Below: 4, Above: 6, HasMoreBefore: true, HasMoreAfter: true}},
		{"around short", domain.HistoryAround, 3, 2, false, domain.Window{Below: 3, Above: 2}},
		{"around empty", domain.HistoryAround, 0, 0, false, domain.Window{}},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			if got := domain.PageFlags(c.mode, c.below, c.above, l, c.opposite); got != c.want {
				t.Fatalf("PageFlags = %+v, want %+v", got, c.want)
			}
		})
	}
}

// TestCutForList (brief note 5): content above the limit is cut at a UTF-8 boundary.
func TestCutForList(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		in    string
		max   int
		want  string
		wantT bool
	}{
		{"hello", 5, "hello", false},
		{"hello", 4, "hell", true},
		{"aé", 2, "a", true},
		{"aé", 3, "aé", false},
		{"€€", 4, "€", true},
		{"€", 2, "", true},
		{"", 1, "", false},
	} {
		got, cut := domain.CutForList(c.in, c.max)
		if got != c.want || cut != c.wantT {
			t.Errorf("CutForList(%q, %d) = %q, %v; want %q, %v", c.in, c.max, got, cut, c.want, c.wantT)
		}
	}
}
