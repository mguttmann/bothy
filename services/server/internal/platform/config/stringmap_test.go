// SPDX-License-Identifier: Apache-2.0

package config_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/platform/config"
)

// mapSchema has one map(string) key with the default of search.locale_map.
func mapSchema(t *testing.T) *config.Schema {
	t.Helper()
	return mustSchema(t, key("search.locale_map", config.KindStringMap, `{"de":"german","en":"english"}`))
}

func TestLoadStringMapDefault(t *testing.T) {
	t.Parallel()
	v := mustLoad(t, mapSchema(t), config.Sources{})
	if got, want := v.StringMap("search.locale_map"), map[string]string{"de": "german", "en": "english"}; !reflect.DeepEqual(got, want) {
		t.Errorf("StringMap = %v, want %v", got, want)
	}
	if got := v.Text("search.locale_map"); got != `{"de":"german","en":"english"}` {
		t.Errorf("Text = %q", got)
	}
}

func TestLoadStringMapFromFile(t *testing.T) {
	t.Parallel()
	path := writeFile(t, "search:\n  locale_map:\n    fr: french\n    de: \"german\"\n")
	v := mustLoad(t, mapSchema(t), config.Sources{File: path})
	if got, want := v.StringMap("search.locale_map"), map[string]string{"de": "german", "fr": "french"}; !reflect.DeepEqual(got, want) {
		t.Errorf("StringMap = %v, want %v", got, want)
	}
	if got := v.Text("search.locale_map"); got != `{"de":"german","fr":"french"}` {
		t.Errorf("Text = %q", got)
	}
	if got := v.Source("search.locale_map"); got.Kind != config.SourceFile {
		t.Errorf("Source = %v", got)
	}

	// A flow mapping is a mapping too; a scalar holds the JSON text of the environment form.
	for doc, want := range map[string]map[string]string{
		"search:\n  locale_map: {de: german}\n":           {"de": "german"},
		"search:\n  locale_map: '{\"en\":\"english\"}'\n": {"en": "english"},
		"search:\n  locale_map: {}\n":                     {},
	} {
		v := mustLoad(t, mapSchema(t), config.Sources{File: writeFile(t, doc)})
		if got := v.StringMap("search.locale_map"); !reflect.DeepEqual(got, want) {
			t.Errorf("%q: StringMap = %v, want %v", doc, got, want)
		}
	}

	// Nested mappings, sequences, aliases, tags, duplicates and empty keys are problems of the
	// key, and no message repeats a value.
	for _, doc := range []string{
		"search:\n  locale_map:\n    de:\n      x: secretvalue\n",
		"search:\n  locale_map:\n    de: [secretvalue]\n",
		"search:\n  locale_map:\n    - secretvalue\n",
		"search:\n  locale_map:\n    de: &a secretvalue\n    en: *a\n",
		"search:\n  locale_map:\n    de: !!str secretvalue\n",
		"search:\n  locale_map:\n    de: secretvalue\n    de: secretvalue\n",
		"search:\n  locale_map:\n    \"\": secretvalue\n",
		"search:\n  locale_map:\n    [secretvalue]: german\n",
		"search:\n  locale_map: secretvalue\n",
	} {
		got := problems(t, mapSchema(t), config.Sources{File: writeFile(t, doc)})
		if len(got) == 0 {
			t.Errorf("%q: no problem", doc)
		}
		for _, p := range got {
			if strings.Contains(p.String(), "secretvalue") {
				t.Errorf("%q: problem %q repeats a value", doc, p)
			}
			if p.Key != "search.locale_map" && p.Key != "" {
				t.Errorf("%q: problem of key %q", doc, p.Key)
			}
		}
	}
}

func TestLoadStringMapFromEnv(t *testing.T) {
	t.Parallel()
	env := []string{`BOTHY_SEARCH__LOCALE_MAP={"de":"german","fr":"simple"}`}
	v := mustLoad(t, mapSchema(t), config.Sources{Env: env})
	if got, want := v.StringMap("search.locale_map"), map[string]string{"de": "german", "fr": "simple"}; !reflect.DeepEqual(got, want) {
		t.Errorf("StringMap = %v, want %v", got, want)
	}
	if got := v.Source("search.locale_map"); got.Kind != config.SourceEnv {
		t.Errorf("Source = %v", got)
	}
	// The environment overrides the file.
	path := writeFile(t, "search:\n  locale_map:\n    en: english\n")
	v = mustLoad(t, mapSchema(t), config.Sources{Env: env, File: path})
	if got := v.Text("search.locale_map"); got != `{"de":"german","fr":"simple"}` {
		t.Errorf("Text = %q", got)
	}
	got := problems(t, mapSchema(t), config.Sources{Env: []string{`BOTHY_SEARCH__LOCALE_MAP={"de":"secretvalue","de":"x"}`}})
	if len(got) != 1 || got[0].Key != "search.locale_map" || strings.Contains(got[0].String(), "secretvalue") {
		t.Errorf("problems = %v", got)
	}
}

func TestBindStringMap(t *testing.T) {
	t.Parallel()
	v := mustLoad(t, mapSchema(t), config.Sources{})
	var c struct {
		Locales map[string]string `config:"search.locale_map"`
	}
	if err := config.Bind(v, &c); err != nil {
		t.Fatalf("Bind: %v", err)
	}
	if want := map[string]string{"de": "german", "en": "english"}; !reflect.DeepEqual(c.Locales, want) {
		t.Errorf("Locales = %v, want %v", c.Locales, want)
	}
	c.Locales["de"] = "changed"
	if v.StringMap("search.locale_map")["de"] != "german" {
		t.Error("Bind shares the map of Values")
	}
	var wrong struct {
		Locales map[string]int `config:"search.locale_map"`
	}
	if err := config.Bind(v, &wrong); err == nil {
		t.Error("Bind filled a map[string]int from a map(string) key")
	}
}

func TestValuesStringMapReturnsCopy(t *testing.T) {
	t.Parallel()
	v := mustLoad(t, mapSchema(t), config.Sources{})
	v.StringMap("search.locale_map")["de"] = "changed"
	delete(v.StringMap("search.locale_map"), "en")
	if got := v.StringMap("search.locale_map"); got["de"] != "german" || got["en"] != "english" {
		t.Errorf("StringMap = %v after changing a returned map", got)
	}
}
