// SPDX-License-Identifier: Apache-2.0

package domain_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/sync/domain"
)

// TestSettingValues: per key valid and invalid values, null, numbers for booleans and nested
// objects in layout.column_widths; an unknown key is not_found.
func TestSettingValues(t *testing.T) {
	t.Parallel()
	valid := map[string][]string{
		"appearance.theme":                       {`"system"`, `"light"`, `"dark"`},
		"appearance.density":                     {`"comfortable"`, `"compact"`},
		"appearance.reduce_motion":               {`"system"`, `"on"`, `"off"`},
		"composer.enter_sends":                   {`true`, `false`},
		"composer.paste_originals":               {`true`, `false`},
		"privacy.crash_reports":                  {`true`},
		"privacy.client_metrics":                 {`false`},
		"layout.column_widths":                   {`{}`, `{"sidebar":240,"post_list":0}`, ` {"a9_b": 12} `},
		"notify.default_level":                   {`"all"`, `"mentions"`, `"mute"`},
		"notify.preview_mode":                    {`"full"`, `"names"`, `"none"`},
		"notify.input_needed_breaks_quiet_hours": {`true`},
		"notify.admin_enabled":                   {`false`},
	}
	invalid := map[string][]string{
		"appearance.theme":         {`null`, `"Light"`, `"touch"`, `1`, `["dark"]`, `""`, `"dark" "dark"`},
		"appearance.density":       {`"touch"`},
		"composer.enter_sends":     {`null`, `1`, `0`, `"true"`, `{}`},
		"composer.paste_originals": {`"false"`, `0`},
		"layout.column_widths": {`null`, `[]`, `{"sidebar":{"w":1}}`, `{"sidebar":-1}`, `{"sidebar":1.5}`, `{"sidebar":1e3}`,
			`{"Sidebar":1}`, `{"1a":1}`, `{"sidebar":"240"}`, `{"sidebar":null}`, `{"sidebar":1,"sidebar":2}`, `{"a":` + strings.Repeat("9", 20) + `}`},
		"notify.preview_mode":  {`"all"`},
		"notify.admin_enabled": {`"yes"`},
	}
	for key, list := range valid {
		if !domain.KnownSetting(key) {
			t.Errorf("%s unknown", key)
		}
		for _, v := range list {
			if err := domain.ValidateSetting(key, json.RawMessage(v)); err != nil {
				t.Errorf("%s = %s: %v", key, v, err)
			}
		}
	}
	for key, list := range invalid {
		for _, v := range list {
			code, ptr, _ := fieldOf(t, domain.ValidateSetting(key, json.RawMessage(v)))
			if code != errs.CodeValidationFailed || ptr != "/value" {
				t.Errorf("%s = %s: %s %s", key, v, code, ptr)
			}
		}
	}
	if domain.KnownSetting("appearance.font") {
		t.Fatal("unknown key known")
	}
	if code, _, _ := fieldOf(t, domain.ValidateSetting("appearance.font", json.RawMessage(`"x"`))); code != errs.CodeNotFound {
		t.Fatalf("unknown key: %s", code)
	}
}

// repoRoot returns the repository root from the path of this file.
func repoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("no caller")
	}
	return filepath.Join(filepath.Dir(file), "..", "..", "..", "..", "..")
}

// specSection returns the lines of the section that starts with heading in the one chapter file
// docs/spec/<prefix>-*.md (the chapter names differ between the languages of the spec).
func specSection(t *testing.T, prefix, heading string) []string {
	t.Helper()
	files, err := filepath.Glob(filepath.Join(repoRoot(t), "docs", "spec", prefix+"-*.md"))
	if err != nil || len(files) != 1 {
		t.Fatalf("chapter %s: %v %v", prefix, files, err)
	}
	b, err := os.ReadFile(files[0])
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	in := false
	level := strings.Count(strings.Fields(heading)[0], "#")
	for _, line := range strings.Split(string(b), "\n") {
		if strings.HasPrefix(line, heading) {
			in = true
			continue
		}
		if in && strings.HasPrefix(line, "#") && strings.Count(strings.Fields(line)[0], "#") <= level {
			break
		}
		if in {
			out = append(out, line)
		}
	}
	if len(out) == 0 {
		t.Fatalf("section %s of chapter %s not found", heading, prefix)
	}
	return out
}

// specRow is one key of a settings table: its key and the backticked values of the column
// "Values" ("bool" for a boolean of 13, "object" for the object of 01).
type specRow struct {
	key    string
	values []string
}

var backticked = regexp.MustCompile("`([^`]*)`")

// specRows reads the table rows of lines whose first cell is a backticked key.
func specRows(lines []string) []specRow {
	var out []specRow
	for _, l := range lines {
		cells := strings.Split(l, "|")
		if len(cells) < 4 || !strings.HasPrefix(strings.TrimSpace(cells[1]), "`") {
			continue
		}
		r := specRow{key: strings.Trim(strings.TrimSpace(cells[1]), "`")}
		valuesCell := strings.TrimSpace(cells[2])
		switch {
		case valuesCell == "bool":
			r.values = []string{"bool"}
		case !strings.HasPrefix(valuesCell, "`"):
			r.values = []string{"object"}
		default:
			for _, m := range backticked.FindAllStringSubmatch(valuesCell, -1) {
				r.values = append(r.values, m[1])
			}
			if slices.Equal(r.values, []string{"true", "false"}) {
				r.values = []string{"bool"}
			}
		}
		out = append(out, r)
	}
	return out
}

// TestSettingRegistryMatchesSpec: the registry holds exactly the keys of 01 section 11 and 13
// section 3.3 with their enumerated values, booleans and the object of layout.column_widths.
func TestSettingRegistryMatchesSpec(t *testing.T) {
	t.Parallel()
	rows := append(specRows(specSection(t, "01", "## 11.")), specRows(specSection(t, "13", "### 3.3"))...)
	if len(rows) == 0 {
		t.Fatal("no rows read")
	}
	want := map[string][]string{}
	for _, r := range rows {
		want[r.key] = r.values
	}
	got := map[string][]string{}
	for _, s := range domain.Settings() {
		switch s.Kind {
		case domain.SettingEnum:
			got[s.Key] = s.Values
		case domain.SettingBool:
			got[s.Key] = []string{"bool"}
		case domain.SettingObject:
			got[s.Key] = []string{"object"}
		}
	}
	for k, v := range want {
		if !slices.Equal(got[k], v) {
			t.Errorf("%s: registry %v, spec %v", k, got[k], v)
		}
	}
	for k := range got {
		if _, ok := want[k]; !ok {
			t.Errorf("%s is in the registry but not in the spec", k)
		}
	}
}
