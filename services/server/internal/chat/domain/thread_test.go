// SPDX-License-Identifier: Apache-2.0

package domain_test

import (
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/chat/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// TestTitleLength_CountsRunes: the limit counts Unicode characters, not bytes; empty is allowed.
func TestTitleLength_CountsRunes(t *testing.T) {
	t.Parallel()
	if err := domain.CheckThreadTitle(strings.Repeat("ä", 5), 5); err != nil {
		t.Fatalf("five runes: %v", err)
	}
	if err := domain.CheckThreadTitle("", 5); err != nil {
		t.Fatalf("empty: %v", err)
	}
	wantCode(t, domain.CheckThreadTitle(strings.Repeat("ä", 6), 5), "/title", domain.FieldTooLong)
}

// TestSEC_T07_ThreadTitleRejectsControlAndBidi: every C0 and C1 control and every bidi control
// is refused in a title (SEC-IN-4); emoji with a zero width joiner pass.
func TestSEC_T07_ThreadTitleRejectsControlAndBidi(t *testing.T) {
	t.Parallel()
	var bad []rune
	for r := rune(0); r < 0x20; r++ {
		bad = append(bad, r)
	}
	for r := rune(0x7f); r <= 0x9f; r++ {
		bad = append(bad, r)
	}
	bad = append(bad, 0x061c, 0x200e, 0x200f, 0x202a, 0x202b, 0x202c, 0x202d, 0x202e, 0x2066, 0x2067, 0x2068, 0x2069)
	for _, r := range bad {
		err := domain.CheckThreadTitle("ab"+string(r)+"c", 100)
		if err == nil {
			t.Errorf("title with %U accepted", r)
			continue
		}
		wantPointer(t, err, "/title")
	}
	if err := domain.CheckThreadTitle("Team \U0001F468\u200d\U0001F469", 100); err != nil {
		t.Fatalf("emoji: %v", err)
	}
}

// TestThreadStateTransitions: close open, close closed (no-op), reopen closed, reopen open
// (no-op), close main (rejected).
func TestThreadStateTransitions(t *testing.T) {
	t.Parallel()
	at, by := time.Date(2026, 10, 14, 9, 0, 0, 0, time.UTC), ids.New()
	th := domain.Thread{ID: ids.New(), Kind: domain.ThreadKindThread, State: domain.ThreadOpen, Version: 1}
	if changed, err := th.Close(by, at); err != nil || !changed || th.State != domain.ThreadClosed || th.ClosedBy != by ||
		!th.ClosedAt.Equal(at) || th.Version != 2 {
		t.Fatalf("close open: %v %v %+v", changed, err, th)
	}
	if changed, err := th.Close(ids.New(), at.Add(time.Hour)); err != nil || changed || th.Version != 2 || th.ClosedBy != by {
		t.Fatalf("close closed: %v %v %+v", changed, err, th)
	}
	if !th.Reopen() || th.State != domain.ThreadOpen || !th.ClosedAt.IsZero() || !th.ClosedBy.IsZero() || th.Version != 3 {
		t.Fatalf("reopen closed: %+v", th)
	}
	if th.Reopen() || th.Version != 3 {
		t.Fatalf("reopen open: %+v", th)
	}
	main := domain.NewMainThread(ids.New(), ids.New(), by, at)
	_, err := main.Close(by, at)
	if e, ok := errs.As(err); !ok || e.Code != errs.CodeInvalidStateTransition {
		t.Fatalf("close main: %v", err)
	}
	if main.State != domain.ThreadOpen || main.Version != 1 {
		t.Fatalf("main changed: %+v", main)
	}
}

// TestTagDiff_RestrictedFlag: the diff names added and removed tags; the restricted flag is set
// exactly when an added or removed tag is restricted, never for a kept one.
func TestTagDiff_RestrictedFlag(t *testing.T) {
	t.Parallel()
	a, b, r, k := ids.New(), ids.New(), ids.New(), ids.New()
	restricted := map[ids.ID]bool{r: true, k: true}
	isR := func(id ids.ID) bool { return restricted[id] }
	for _, c := range []struct {
		name          string
		current, next []ids.ID
		added, gone   []ids.ID
		flag, changed bool
	}{
		{"same", []ids.ID{a, k}, []ids.ID{k, a}, nil, nil, false, false},
		{"add plain", []ids.ID{k}, []ids.ID{k, a}, []ids.ID{a}, nil, false, true},
		{"remove plain", []ids.ID{a, b, k}, []ids.ID{b, k}, nil, []ids.ID{a}, false, true},
		{"add restricted", []ids.ID{a}, []ids.ID{a, r}, []ids.ID{r}, nil, true, true},
		{"remove restricted", []ids.ID{a, r}, []ids.ID{a}, nil, []ids.ID{r}, true, true},
		{"empty", nil, nil, nil, nil, false, false},
	} {
		d := domain.DiffTags(c.current, c.next, isR)
		if !slices.Equal(d.Added, c.added) || !slices.Equal(d.Removed, c.gone) || d.Restricted != c.flag || d.Changed() != c.changed {
			t.Errorf("%s: %+v", c.name, d)
		}
	}
}

// TestCheckTagSet: duplicates and a set above the limit are refused on /tag_ids.
func TestCheckTagSet(t *testing.T) {
	t.Parallel()
	a, b := ids.New(), ids.New()
	if err := domain.CheckTagSet([]ids.ID{a, b}, 2); err != nil {
		t.Fatal(err)
	}
	wantCode(t, domain.CheckTagSet([]ids.ID{a, a}, 5), "/tag_ids", domain.FieldDuplicate)
	wantCode(t, domain.CheckTagSet([]ids.ID{a, b}, 1), "/tag_ids", domain.FieldLimitReached)
}
