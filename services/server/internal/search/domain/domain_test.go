// SPDX-License-Identifier: Apache-2.0

package domain_test

import (
	"bytes"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/search/domain"
)

// fieldCodes returns the pointer and code of every field error of err, or fails.
func fieldCodes(t *testing.T, err error) []string {
	t.Helper()
	var e *errs.Error
	if !errors.As(err, &e) || e.Code != errs.CodeValidationFailed {
		t.Fatalf("err = %v, want validation_failed", err)
	}
	var out []string
	for _, f := range e.Fields {
		out = append(out, f.Pointer+" "+f.Code)
	}
	return out
}

func TestValidateQ(t *testing.T) {
	t.Parallel()
	const maxChars = 5
	if _, err := domain.ValidateQ("abcde", maxChars); err != nil {
		t.Fatalf("five runes: %v", err)
	}
	if got := fieldCodes(t, mustErr(t, "abcdef", maxChars)); len(got) != 1 || got[0] != "#query/q too_long" {
		t.Fatalf("six runes: %v", got)
	}
	// Multi-byte runes count once each: five umlauts are ten bytes but five runes.
	if _, err := domain.ValidateQ("äöüßé", maxChars); err != nil {
		t.Fatalf("five multi-byte runes: %v", err)
	}
	// An invalid byte counts as one rune.
	if _, err := domain.ValidateQ("ab\xffcd", maxChars); err != nil {
		t.Fatalf("invalid byte: %v", err)
	}
	if _, err := domain.ValidateQ("ab\xff\xfecd", maxChars); err == nil {
		t.Fatal("two invalid bytes and four runes: want too_long")
	}
	cases := map[string]string{
		"a\x00b":        "a b",
		"a\xffb":        "a b",
		"a\tb\nc":       "a b c",
		"a\u0085b":      "a b",
		"Rechnung":      "Rechnung",
		"\xe2\x82":      "  ",
		"\"phrase\" -x": "\"phrase\" -x",
	}
	for in, want := range cases {
		got, err := domain.ValidateQ(in, 100)
		if err != nil || got != want {
			t.Errorf("CleanQ(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	if got := fieldCodes(t, domain.EmptyQuery()); len(got) != 1 || got[0] != "#query/q empty" {
		t.Fatalf("EmptyQ: %v", got)
	}
}

// mustErr returns the error of CleanQ or fails.
func mustErr(t *testing.T, q string, maxChars int) error {
	t.Helper()
	_, err := domain.ValidateQ(q, maxChars)
	if err == nil {
		t.Fatalf("CleanQ(%q) passed", q)
	}
	return err
}

func TestFilterApplicability(t *testing.T) {
	t.Parallel()
	// The column "Types" of 14 4.4 and thread_id for every type.
	want := map[domain.Filter]map[domain.Type]bool{
		"space_id":     {"messages": true, "threads": true, "files": true},
		"channel_id":   {"messages": true, "threads": true, "files": true},
		"thread_id":    {"messages": true, "threads": true, "files": true},
		"author_id":    {"messages": true, "files": true},
		"agent_id":     {"messages": true, "threads": true, "files": true},
		"after":        {"messages": true, "threads": true, "files": true},
		"before":       {"messages": true, "threads": true, "files": true},
		"has_file":     {"messages": true},
		"thread_state": {"messages": true, "threads": true},
		"tag_id":       {"messages": true, "threads": true},
	}
	if got := domain.Filters(); len(got) != len(want) {
		t.Fatalf("Filters() = %v, want the %d filters of 14 4.4", got, len(want))
	}
	for _, f := range domain.Filters() {
		for _, typ := range []domain.Type{domain.TypeMessages, domain.TypeThreads, domain.TypeFiles} {
			err := domain.CheckFilter(f, typ)
			if want[f][typ] {
				if err != nil {
					t.Errorf("CheckFilter(%s, %s) = %v", f, typ, err)
				}
				continue
			}
			if got := fieldCodes(t, err); len(got) != 1 || got[0] != "#query/"+string(f)+" not_applicable" {
				t.Errorf("CheckFilter(%s, %s) = %v", f, typ, got)
			}
		}
	}
}

func TestBinding(t *testing.T) {
	t.Parallel()
	a, b, c := ids.New(), ids.New(), ids.New()
	at := time.Date(2026, 10, 1, 12, 0, 0, 123_456_000, time.UTC)
	yes := true
	base := domain.Binding{Type: domain.TypeMessages, Q: "invoice", Sort: "relevance", SpaceIDs: []ids.ID{a, b}, ChannelIDs: []ids.ID{c},
		TagIDs: []ids.ID{a, b}, After: &at, HasFile: &yes}
	perm := base
	perm.SpaceIDs, perm.TagIDs = []ids.ID{b, a, b}, []ids.ID{b, b, a}
	local := at.In(time.FixedZone("x", 3600)).Add(400 * time.Microsecond)
	perm.After = &local
	if !bytes.Equal(base.Encode(), perm.Encode()) {
		t.Fatalf("permuted, duplicated ids and the same millisecond in another zone changed the binding:\n%s\n%s",
			base.Encode(), perm.Encode())
	}
	no := false
	later := at.Add(time.Millisecond)
	thread := ids.New()
	changes := map[string]func(*domain.Binding){
		"type":         func(x *domain.Binding) { x.Type = domain.TypeFiles },
		"q":            func(x *domain.Binding) { x.Q = "invoices" },
		"q split":      func(x *domain.Binding) { x.Q, x.Sort = "invoicer", "elevance" },
		"sort":         func(x *domain.Binding) { x.Sort = "newest" },
		"space":        func(x *domain.Binding) { x.SpaceIDs = []ids.ID{a} },
		"channel":      func(x *domain.Binding) { x.ChannelIDs = nil },
		"thread":       func(x *domain.Binding) { x.ThreadID = &thread },
		"author":       func(x *domain.Binding) { x.AuthorID = &a },
		"agent":        func(x *domain.Binding) { x.AgentID = &a },
		"after":        func(x *domain.Binding) { x.After = &later },
		"after absent": func(x *domain.Binding) { x.After = nil },
		"before":       func(x *domain.Binding) { x.Before = &at },
		"has_file":     func(x *domain.Binding) { x.HasFile = &no },
		"has_file nil": func(x *domain.Binding) { x.HasFile = nil },
		"state":        func(x *domain.Binding) { x.ThreadState = "open" },
		"tags":         func(x *domain.Binding) { x.TagIDs = []ids.ID{c} },
	}
	for name, change := range changes {
		x := base
		change(&x)
		if bytes.Equal(base.Encode(), x.Encode()) {
			t.Errorf("changing %s kept the binding", name)
		}
	}
	if strings.Contains(string(base.Encode()), "limit") {
		t.Fatal("the page size is part of the binding")
	}
}

func TestLimitCap(t *testing.T) {
	t.Parallel()
	for _, c := range []struct{ limit, want int }{{0, 20}, {-3, 20}, {1, 1}, {50, 50}, {51, 50}, {1 << 30, 50}} {
		if got := domain.PageSize(c.limit, 20, 50); got != c.want {
			t.Errorf("PageSize(%d) = %d, want %d", c.limit, got, c.want)
		}
	}
}
