// SPDX-License-Identifier: Apache-2.0

package definition_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/agents/domain/definition"
)

// corpusDir is the shared corpus of 26 4.3 at the repository root (tests/agent-definitions).
const corpusDir = "../../../../../../tests/agent-definitions"

// fixture is one file of the corpus.
type fixture struct {
	ToolClass  string          `json:"tool_class"`
	Definition json.RawMessage `json:"definition"`
	Expect     []finding       `json:"expect"`
}

// finding is one expected field error.
type finding struct {
	Pointer string `json:"pointer"`
	Code    string `json:"code"`
}

// corpusLimits reads limits.json of the corpus into Limits by the config tags, so the corpus and
// the server name the same keys (the wire test holds the defaults equal to it).
func corpusLimits(t *testing.T) definition.Limits {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(corpusDir, "limits.json"))
	if err != nil {
		t.Fatal(err)
	}
	var byKey map[string]json.RawMessage
	if err := json.Unmarshal(raw, &byKey); err != nil {
		t.Fatal(err)
	}
	var lim definition.Limits
	v := reflect.ValueOf(&lim).Elem()
	for i := range v.NumField() {
		key := v.Type().Field(i).Tag.Get("config")
		val, ok := byKey[key]
		if !ok {
			t.Fatalf("limits.json misses %s", key)
		}
		if err := json.Unmarshal(val, v.Field(i).Addr().Interface()); err != nil {
			t.Fatalf("limits.json %s: %v", key, err)
		}
		delete(byKey, key)
	}
	if len(byKey) != 0 {
		t.Fatalf("limits.json has keys Limits does not know: %v", byKey)
	}
	return lim
}

// sorted returns the findings as sorted "pointer code" lines, so the comparison is order free.
func sorted(fs []finding) []string {
	out := make([]string, 0, len(fs))
	for _, f := range fs {
		out = append(out, f.Pointer+" "+f.Code)
	}
	slices.Sort(out)
	return out
}

// TestCorpus: every fixture of tests/agent-definitions gives exactly its expected findings,
// pointer and code, order free; a valid fixture gives none and parses.
func TestCorpus(t *testing.T) {
	t.Parallel()
	lim := corpusLimits(t)
	for _, kind := range []string{"valid", "invalid"} {
		files, err := filepath.Glob(filepath.Join(corpusDir, kind, "*.json"))
		if err != nil || len(files) == 0 {
			t.Fatalf("no %s fixtures: %v", kind, err)
		}
		for _, path := range files {
			name := kind + "/" + filepath.Base(path)
			t.Run(name, func(t *testing.T) {
				t.Parallel()
				raw, err := os.ReadFile(path) //nolint:gosec // a fixture path from the Glob of the corpus folder
				if err != nil {
					t.Fatal(err)
				}
				var fx fixture
				dec := json.NewDecoder(strings.NewReader(string(raw)))
				dec.DisallowUnknownFields()
				if err := dec.Decode(&fx); err != nil {
					t.Fatalf("fixture format: %v", err)
				}
				if (kind == "valid") != (len(fx.Expect) == 0) {
					t.Fatalf("a valid fixture expects nothing, an invalid one at least one finding")
				}
				_, got := definition.Parse(fx.Definition, lim, definition.ToolClass(fx.ToolClass))
				gotF := make([]finding, 0, len(got))
				for _, g := range got {
					gotF = append(gotF, finding{Pointer: g.Pointer, Code: g.Code})
				}
				if g, w := sorted(gotF), sorted(fx.Expect); !slices.Equal(g, w) {
					t.Errorf("findings\n got  %q\n want %q", g, w)
				}
			})
		}
	}
}

// TestCorpusCoversEveryCode: the corpus holds at least one invalid fixture per field code of
// 26 4.3 (A1-02 requirement 13), named <code>-<name>.json.
func TestCorpusCoversEveryCode(t *testing.T) {
	t.Parallel()
	for _, code := range []string{
		definition.CodeUnknownMember, definition.CodeSkillFrontmatter, definition.CodeInlineShell,
		definition.CodeImportOutsideFolder, definition.CodeSecretValueNotAllowed, definition.CodeEnvNameReserved,
		definition.CodeMCPCommandNotAllowed, definition.CodeMCPPackageUnpinned, definition.CodeToolClassTooLow,
		definition.CodeTooLong, definition.CodeTooMany, definition.CodeInvalidName, definition.CodeDuplicateName,
	} {
		files, err := filepath.Glob(filepath.Join(corpusDir, "invalid", code+"-*.json"))
		if err != nil || len(files) == 0 {
			t.Errorf("no invalid fixture for %s", code)
		}
	}
}
