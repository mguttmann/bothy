// SPDX-License-Identifier: Apache-2.0

package designkeys_test

import (
	"encoding/json"
	"os"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/kernel/designkeys"
)

// root is the repository root relative to this package.
const root = "../../../../../"

// adrAccents are the accent keys of ADR 0044 and docs/spec/01-produkt.md section 12.
var adrAccents = []string{"amber", "gorse", "heather", "loch", "moss", "rowan", "spruce"}

func TestAccentColorsMatchADR0044(t *testing.T) {
	t.Parallel()
	got := designkeys.AccentColors()
	if !slices.Equal(got, adrAccents) {
		t.Fatalf("AccentColors() = %v, want %v", got, adrAccents)
	}
	for _, k := range got {
		if !designkeys.IsCategoryColor(k) {
			t.Errorf("accent %q is not a generated category colour", k)
		}
		if !designkeys.IsAccentColor(k) {
			t.Errorf("IsAccentColor(%q) = false", k)
		}
	}
	// The list in 01 section 12 and in ADR 0044 names exactly these keys.
	for _, doc := range []string{"docs/spec/01-produkt.md", "docs/adr/0044-arbeitsname-und-branding-als-konfiguration.md"} {
		b, err := os.ReadFile(root + doc) //nolint:gosec // G304: a fixed path below the repository root, not input.
		if err != nil {
			t.Fatal(err)
		}
		text := string(b)
		for _, k := range adrAccents {
			if !strings.Contains(text, "`"+k+"`") {
				t.Errorf("%s does not name the accent %q", doc, k)
			}
		}
	}
}

func TestIsAccentColorRejects(t *testing.T) {
	t.Parallel()
	for _, s := range []string{"#5865f2", "rgb(0,0,0)", "Spruce", " spruce", "spruce ", "granite", "\U0001F535", "", "red", "loch.fg"} {
		if designkeys.IsAccentColor(s) {
			t.Errorf("IsAccentColor(%q) = true", s)
		}
	}
	if designkeys.IsCategoryColor("Spruce") || designkeys.IsCategoryColor("#5865f2") {
		t.Error("a free value passed as category colour")
	}
	if !designkeys.IsCategoryColor("granite") {
		t.Error("granite is a category colour of the tokens")
	}
}

func TestCategoryColorsFromTokens(t *testing.T) {
	t.Parallel()
	b, err := os.ReadFile(root + "design/build/tokens.resolved.json")
	if err != nil {
		t.Fatal(err)
	}
	var f struct {
		Contexts struct {
			Theme map[string]map[string]json.RawMessage `json:"theme"`
		} `json:"contexts"`
	}
	if err := json.Unmarshal(b, &f); err != nil {
		t.Fatal(err)
	}
	re := regexp.MustCompile(`^color\.category\.([^.]+)\.`)
	var want []string
	for k := range f.Contexts.Theme["light"] {
		if m := re.FindStringSubmatch(k); m != nil {
			want = append(want, m[1])
		}
	}
	slices.Sort(want)
	want = slices.Compact(want)
	if got := designkeys.CategoryColors(); !slices.Equal(got, want) {
		t.Errorf("CategoryColors() = %v, want %v", got, want)
	}
}

func TestSpaceIconsFromTokens(t *testing.T) {
	t.Parallel()
	b, err := os.ReadFile(root + "design/build/tokens.resolved.json")
	if err != nil {
		t.Fatal(err)
	}
	var f struct {
		Icons struct {
			SpaceIcons []string `json:"space-icons"`
		} `json:"icons"`
	}
	if err := json.Unmarshal(b, &f); err != nil {
		t.Fatal(err)
	}
	want := slices.Clone(f.Icons.SpaceIcons)
	slices.Sort(want)
	want = slices.Compact(want)
	got := designkeys.SpaceIcons()
	if len(got) == 0 || !slices.Equal(got, want) {
		t.Fatalf("SpaceIcons() = %v, want %v", got, want)
	}
	for _, icon := range got {
		if !designkeys.IsSpaceIcon(icon) {
			t.Errorf("IsSpaceIcon(%q) = false", icon)
		}
	}
	for _, s := range []string{"", "Code", "code ", "\U0001F535", "folder"} {
		if designkeys.IsSpaceIcon(s) {
			t.Errorf("IsSpaceIcon(%q) = true", s)
		}
	}
}

func TestListsAreCopies(t *testing.T) {
	t.Parallel()
	for name, f := range map[string]func() []string{
		"CategoryColors": designkeys.CategoryColors, "SpaceIcons": designkeys.SpaceIcons, "AccentColors": designkeys.AccentColors,
	} {
		a := f()
		a[0] = "changed"
		if f()[0] == "changed" {
			t.Errorf("%s returns the package list itself", name)
		}
	}
}
