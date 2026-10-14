// SPDX-License-Identifier: Apache-2.0

package domain_test

import (
	"strings"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/chat/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// wantField checks that err is validation_failed with one field error on pointer.
func wantField(t *testing.T, err error, pointer string) {
	t.Helper()
	e, ok := errs.As(err)
	if !ok || e.Code != errs.CodeValidationFailed || len(e.Fields) != 1 || e.Fields[0].Pointer != pointer {
		t.Fatalf("error %v, want validation_failed on %s", err, pointer)
	}
}

// TestSpaceName: trimmed, required, at most the limit in Unicode characters, no control or bidi
// characters (SEC-IN-4, SEC-T07).
func TestSpaceName(t *testing.T) {
	t.Parallel()
	if got, err := domain.SpaceName("  Support ", 7); err != nil || got != "Support" {
		t.Fatalf("SpaceName = %q, %v", got, err)
	}
	if got, err := domain.SpaceName("Forschüng", 9); err != nil || got != "Forschüng" {
		t.Fatalf("SpaceName counts runes: %q, %v", got, err)
	}
	for _, n := range []string{"", "   ", "Supports", "Sup\u202eport", "Sup\u0007", "\x00"} {
		_, err := domain.SpaceName(n, 7)
		wantField(t, err, "/name")
	}
}

// TestSpaceColorAndIcon: only keys of the generated lists; hex values, emoji and unknown keys are
// refused with the pointer of the field.
func TestSpaceColorAndIcon(t *testing.T) {
	t.Parallel()
	if err := domain.CheckColor("moss"); err != nil {
		t.Fatal(err)
	}
	if err := domain.CheckIcon("headset"); err != nil {
		t.Fatal(err)
	}
	for _, c := range []string{"#3366ff", "\U0001F600", "teal", "", "Moss"} {
		wantField(t, domain.CheckColor(c), "/color")
		wantField(t, domain.CheckIcon(c), "/icon")
	}
}

// TestSpaceUpdate: only sent fields that differ change; one version step for any change.
func TestSpaceUpdate(t *testing.T) {
	t.Parallel()
	s := domain.NewSpace(ids.New(), "Support", "headset", "moss", "a0", ids.New())
	if s.Version != 1 || s.Archived() {
		t.Fatalf("new space %+v", s)
	}
	same, other := "Support", "rowan"
	if changed := s.Update(&same, nil, nil); len(changed) != 0 || s.Version != 1 {
		t.Fatalf("no change: %v %d", changed, s.Version)
	}
	if changed := s.Update(nil, nil, &other); strings.Join(changed, ",") != "color" || s.Version != 2 || s.Color != "rowan" {
		t.Fatalf("color: %v %+v", changed, s)
	}
	name, icon := "Research", "flask"
	if changed := s.Update(&name, &icon, nil); strings.Join(changed, ",") != "name,icon" || s.Version != 3 {
		t.Fatalf("name and icon: %v %+v", changed, s)
	}
}

// TestSidebarPut: a personal key must be a fractional index key; use_global_order removes it and
// may not come with a key; neither keeps the stored key.
func TestSidebarPut(t *testing.T) {
	t.Parallel()
	key, bad := "a0V", "a00"
	for _, c := range []struct {
		stored  string
		key     *string
		global  bool
		want    string
		pointer string
	}{
		{"a1", nil, false, "a1", ""},
		{"a1", &key, false, "a0V", ""},
		{"a1", nil, true, "", ""},
		{"a1", &bad, false, "", "/sort_key"},
		{"a1", &key, true, "", "/use_global_order"},
	} {
		got, err := domain.SidebarKey(c.stored, c.key, c.global)
		if c.pointer != "" {
			wantField(t, err, c.pointer)
			continue
		}
		if err != nil || got != c.want {
			t.Errorf("SidebarKey(%q) = %q, %v; want %q", c.stored, got, err, c.want)
		}
	}
}

// TestCheckReason: required and free of control and bidi characters.
func TestCheckReason(t *testing.T) {
	t.Parallel()
	if err := domain.CheckReason("Team left"); err != nil {
		t.Fatal(err)
	}
	wantField(t, domain.CheckReason(" "), "/reason")
	wantField(t, domain.CheckReason("x\u202e"), "/reason")
}
