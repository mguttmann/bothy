// SPDX-License-Identifier: Apache-2.0

package searchlang_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/kernel/searchlang"
)

// chapterLanguages and chapterLocales are the defaults of docs/spec/14-search.md section 15.
var (
	chapterLanguages = []string{"german", "english"}
	chapterLocales   = map[string]string{"de": "german", "en": "english"}
)

func TestNew(t *testing.T) {
	t.Parallel()
	long := strings.Repeat("a", 64)
	cases := []struct {
		name      string
		languages []string
		locales   map[string]string
		// wantKey and wantPos are the key and the position the error names; "" means valid.
		wantKey string
		wantPos string
		// hidden is a configured value that must not appear in the error.
		hidden string
	}{
		{name: "chapter defaults", languages: chapterLanguages, locales: chapterLocales},
		{name: "empty", languages: nil, locales: nil},
		{name: "simple as map value", languages: []string{"german"}, locales: map[string]string{"de": "german", "fr": "simple"}},
		{name: "longest identifier", languages: []string{long[:63]}, locales: map[string]string{}},
		{name: "underscore and digits", languages: []string{"_x1", "de_2"}, locales: map[string]string{}},
		{
			name: "map value missing from languages", languages: chapterLanguages,
			locales: map[string]string{"de": "german", "fr": "french"}, wantKey: "search.locale_map", wantPos: "entry 2", hidden: "french",
		},
		{name: "simple in languages", languages: []string{"german", "simple"}, wantKey: "search.languages", wantPos: "entry 2"},
		{name: "duplicate language", languages: []string{"german", "english", "german"}, wantKey: "search.languages", wantPos: "entry 3", hidden: "german"},
		{name: "upper case", languages: []string{"German"}, wantKey: "search.languages", wantPos: "entry 1", hidden: "German"},
		{name: "dash", languages: []string{"english", "pt-br"}, wantKey: "search.languages", wantPos: "entry 2", hidden: "pt-br"},
		{name: "leading digit", languages: []string{"1german"}, wantKey: "search.languages", wantPos: "entry 1", hidden: "1german"},
		{name: "empty name", languages: []string{""}, wantKey: "search.languages", wantPos: "entry 1"},
		{name: "too long", languages: []string{long}, wantKey: "search.languages", wantPos: "entry 1", hidden: long},
		{name: "schema qualified", languages: []string{"pg_catalog.german"}, wantKey: "search.languages", wantPos: "entry 1", hidden: "pg_catalog.german"},
		{
			name: "map value not an identifier", languages: chapterLanguages,
			locales: map[string]string{"de": "German"}, wantKey: "search.locale_map", wantPos: "entry 1", hidden: "German",
		},
		{name: "map key with region", languages: chapterLanguages, locales: map[string]string{"de-AT": "german"}, wantKey: "search.locale_map", wantPos: "entry 1", hidden: "de-AT"},
		{name: "map key upper case", languages: chapterLanguages, locales: map[string]string{"DE": "german", "en": "english"}, wantKey: "search.locale_map", wantPos: "entry 1", hidden: "DE"},
		{name: "empty map key", languages: chapterLanguages, locales: map[string]string{"": "german"}, wantKey: "search.locale_map", wantPos: "entry 1"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := searchlang.New(tc.languages, tc.locales)
			if tc.wantKey == "" {
				if err != nil {
					t.Fatalf("New: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatal("New accepted the configuration")
			}
			msg := err.Error()
			if !strings.Contains(msg, tc.wantKey) || !strings.Contains(msg, tc.wantPos) {
				t.Errorf("error %q does not name %s and %s", msg, tc.wantKey, tc.wantPos)
			}
			if tc.hidden != "" && strings.Contains(msg, tc.hidden) {
				t.Errorf("error %q repeats the configured value %q", msg, tc.hidden)
			}
		})
	}
}

func TestNewCopiesInput(t *testing.T) {
	t.Parallel()
	langs := slices.Clone(chapterLanguages)
	locales := map[string]string{"de": "german", "en": "english"}
	m, err := searchlang.New(langs, locales)
	if err != nil {
		t.Fatal(err)
	}
	langs[0], locales["de"] = "x", "x"
	if got := m.For("de"); got != "german" {
		t.Errorf("For(de) = %q after the caller changed its map", got)
	}
	if got := m.Languages(); !slices.Equal(got, chapterLanguages) {
		t.Errorf("Languages() = %v after the caller changed its slice", got)
	}
}

func TestFor(t *testing.T) {
	t.Parallel()
	m, err := searchlang.New(chapterLanguages, chapterLocales)
	if err != nil {
		t.Fatal(err)
	}
	for locale, want := range map[string]string{
		"de": "german", "de-AT": "german", "DE-at": "german", "en-US": "english", "en": "english",
		"fr": "simple", "": "simple", "-": "simple", "-de": "simple", "dé": "simple",
	} {
		if got := m.For(locale); got != want {
			t.Errorf("For(%q) = %q, want %q", locale, got, want)
		}
	}
	var zero searchlang.Map
	if got := zero.For("de"); got != searchlang.Simple {
		t.Errorf("zero Map: For(de) = %q, want simple", got)
	}
}

func TestConfigs(t *testing.T) {
	t.Parallel()
	m, err := searchlang.New([]string{"german", "english", "french"}, map[string]string{"de": "german", "fr": "simple", "en": "english"})
	if err != nil {
		t.Fatal(err)
	}
	got := m.Configs()
	if want := []string{"english", "french", "german", "simple"}; !slices.Equal(got, want) {
		t.Errorf("Configs() = %v, want %v", got, want)
	}
	var zero searchlang.Map
	if got := zero.Configs(); !slices.Equal(got, []string{"simple"}) {
		t.Errorf("zero Map: Configs() = %v, want [simple]", got)
	}
}

func TestLanguages(t *testing.T) {
	t.Parallel()
	m, err := searchlang.New([]string{"english", "german"}, map[string]string{"fr": "simple"})
	if err != nil {
		t.Fatal(err)
	}
	got := m.Languages()
	if want := []string{"english", "german"}; !slices.Equal(got, want) {
		t.Errorf("Languages() = %v, want %v", got, want)
	}
	got[0] = "x"
	if m.Languages()[0] != "english" {
		t.Error("Languages() returns the internal slice")
	}
	var zero searchlang.Map
	if got := zero.Languages(); got == nil || len(got) != 0 {
		t.Errorf("zero Map: Languages() = %#v, want an empty slice", got)
	}
}

func TestLocaleIndex(t *testing.T) {
	t.Parallel()
	m, err := searchlang.New([]string{"german", "english"}, map[string]string{"en": "english", "de": "german", "at": "german"})
	if err != nil {
		t.Fatal(err)
	}
	for config, want := range map[string]int{"german": 1, "english": 3, "simple": 0, "french": 0} {
		if got := m.LocaleIndex(config); got != want {
			t.Errorf("LocaleIndex(%s) = %d, want %d", config, got, want)
		}
	}
}
