// SPDX-License-Identifier: Apache-2.0

package domain_test

import (
	"bufio"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/notify/domain"
)

// repoRoot walks up from the package directory to the directory that holds docs/spec.
func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("working directory: %v", err)
	}
	for {
		info, err := os.Stat(filepath.Join(dir, "docs", "spec"))
		if err == nil && info.IsDir() {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("no directory with docs/spec above the working directory")
		}
		dir = parent
	}
}

func readLines(t *testing.T, rel string) []string {
	t.Helper()
	path := filepath.Join(repoRoot(t), filepath.FromSlash(rel))
	f, err := os.Open(path) //nolint:gosec // G304: a fixed path below the repository root, not input.
	if err != nil {
		t.Fatalf("open %s: %v", rel, err)
	}
	defer func() { _ = f.Close() }()
	var lines []string
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 1<<20)
	for sc.Scan() {
		lines = append(lines, sc.Text())
	}
	if err := sc.Err(); err != nil {
		t.Fatalf("read %s: %v", rel, err)
	}
	return lines
}

// openAPIEnum returns the enum line of a top-level schema of components.schemas.
func openAPIEnum(t *testing.T, lines []string, schema string) []string {
	t.Helper()
	head := "    " + schema + ":"
	for i, l := range lines {
		if l != head {
			continue
		}
		for _, next := range lines[i+1:] {
			if strings.HasPrefix(next, "    ") && !strings.HasPrefix(next, "     ") {
				break // the next schema starts
			}
			if v, ok := strings.CutPrefix(strings.TrimSpace(next), "enum: ["); ok {
				return splitList(strings.TrimSuffix(v, "]"))
			}
		}
		t.Fatalf("openapi.yaml: schema %s has no enum line", schema)
	}
	t.Fatalf("openapi.yaml: no schema %s", schema)
	return nil
}

func splitList(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		out = append(out, strings.Trim(strings.TrimSpace(p), "'\""))
	}
	return out
}

// chapter13Kinds returns the first column of the table in 13 "2. Arten von Hinweisen und
// Empfänger".
func chapter13Kinds(t *testing.T) []string {
	t.Helper()
	lines := readLines(t, "docs/spec/13-push-benachrichtigungen.md")
	var out []string
	in, body := false, false
	for _, l := range lines {
		switch {
		case strings.HasPrefix(l, "## 2. Arten von Hinweisen"):
			in = true
		case in && strings.HasPrefix(l, "#"):
			return out
		case in && strings.HasPrefix(l, "|---"):
			body = true // the header row above names the column, not a kind
		case body && strings.HasPrefix(l, "| `"):
			cells := strings.Split(l, "|")
			out = append(out, strings.Trim(strings.TrimSpace(cells[1]), "`"))
		}
	}
	t.Fatal("13: section 2 not found")
	return nil
}

var platformCheck = regexp.MustCompile(`CHECK \(platform IN \(([^)]*)\)\)`)

// devicePlatforms returns the CHECK list of devices.platform in 03, not the one of hosts.
func devicePlatforms(t *testing.T) []string {
	t.Helper()
	lines := readLines(t, "docs/spec/03-datenmodell.md")
	in := false
	for _, l := range lines {
		switch {
		case l == "#### `devices`":
			in = true
		case in && strings.HasPrefix(l, "#"):
			t.Fatal("03: table devices has no platform check")
		case in && strings.HasPrefix(l, "| `platform` |"):
			m := platformCheck.FindStringSubmatch(l)
			if m == nil {
				t.Fatalf("03: platform row of devices without a CHECK list: %q", l)
			}
			return splitList(m[1])
		}
	}
	t.Fatal("03: table devices not found")
	return nil
}

func asStrings[T ~string](vs []T) []string {
	out := make([]string, len(vs))
	for i, v := range vs {
		out[i] = string(v)
	}
	return out
}

// TestKindsLevelsPlatformsMatchContract holds the enums of this package equal to openapi.yaml, 13
// and 03; a new value in one of them fails here until the domain follows.
func TestKindsLevelsPlatformsMatchContract(t *testing.T) {
	t.Parallel()
	api := readLines(t, "docs/api/openapi.yaml")
	kinds := asStrings(domain.Kinds())
	if want := openAPIEnum(t, api, "NotificationKind"); !reflect.DeepEqual(kinds, want) {
		t.Errorf("Kinds() = %v, openapi NotificationKind = %v", kinds, want)
	}
	if want := chapter13Kinds(t); !reflect.DeepEqual(kinds, want) {
		t.Errorf("Kinds() = %v, 13 section 2 = %v", kinds, want)
	}
	if got, want := asStrings(domain.Levels()), openAPIEnum(t, api, "NotificationLevel"); !reflect.DeepEqual(got, want) {
		t.Errorf("Levels() = %v, openapi NotificationLevel = %v", got, want)
	}
	if got, want := asStrings(domain.Platforms()), devicePlatforms(t); !reflect.DeepEqual(got, want) {
		t.Errorf("Platforms() = %v, 03 devices.platform = %v", got, want)
	}
}
