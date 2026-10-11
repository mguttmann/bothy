// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/platform/config"
	"github.com/OWNER/bothy/services/server/internal/wire"
)

// committedFile is the exported schema relative to this package (the go:generate line in main.go
// writes it).
const committedFile = "../../architecture/config-schema.json"

func serverKeys(t *testing.T) []config.Key {
	t.Helper()
	s, err := wire.ConfigSchema()
	if err != nil {
		t.Fatalf("wire.ConfigSchema: %v", err)
	}
	return s.Keys()
}

// TestExportMatchesCommittedFile holds the committed JSON equal to the schema in the code, so the
// documentation gate (make config-doc-check), which reads only the JSON, sees the schema of this
// server version.
func TestExportMatchesCommittedFile(t *testing.T) {
	t.Parallel()
	got, err := export(serverKeys(t))
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	want, err := os.ReadFile(committedFile)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("%s differs from the schema in internal/wire; run `go generate ./tools/configschema` in services/server\ngot:\n%s", committedFile, got)
	}
}

// TestExportIsSortedAndDeterministic pins the file format: two-space indent, final newline, keys
// sorted by name, every field present in a fixed order, enum as a list, no HTML escaping (a
// secret_ref default contains "<").
func TestExportIsSortedAndDeterministic(t *testing.T) {
	t.Parallel()
	keys := []config.Key{
		{
			Name: "z.secret_ref", Kind: config.KindSecretRef, Default: "file:<a.dir>/z",
			Doc: "Reference & key.", Chapter: "16-betrieb.md",
		},
		{
			Name: "a.mode", Kind: config.KindEnum, Default: "x", Enum: []string{"x", "y"},
			Doc: "Mode.", Chapter: "02-architektur.md",
		},
		{
			Name: "m.limit", Kind: config.KindInt, Required: true, Min: "1", Max: "9",
			Doc: "Limit.", Chapter: "03-datenmodell.md",
		},
	}
	want := `{
  "version": 1,
  "keys": [
    {
      "name": "a.mode",
      "env": "BOTHY_A__MODE",
      "kind": "enum",
      "default": "x",
      "required": false,
      "enum": [
        "x",
        "y"
      ],
      "min": "",
      "max": "",
      "chapter": "02-architektur.md",
      "doc": "Mode."
    },
    {
      "name": "m.limit",
      "env": "BOTHY_M__LIMIT",
      "kind": "int",
      "default": "",
      "required": true,
      "enum": [],
      "min": "1",
      "max": "9",
      "chapter": "03-datenmodell.md",
      "doc": "Limit."
    },
    {
      "name": "z.secret_ref",
      "env": "BOTHY_Z__SECRET_REF",
      "kind": "secret_ref",
      "default": "file:<a.dir>/z",
      "required": false,
      "enum": [],
      "min": "",
      "max": "",
      "chapter": "16-betrieb.md",
      "doc": "Reference & key."
    }
  ]
}
`
	first, err := export(keys)
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	if string(first) != want {
		t.Errorf("export =\n%s\nwant\n%s", first, want)
	}
	reversed := []config.Key{keys[2], keys[1], keys[0]}
	second, err := export(reversed)
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	if !bytes.Equal(first, second) {
		t.Errorf("export depends on the input order:\n%s\n---\n%s", first, second)
	}
	if keys[0].Name != "z.secret_ref" {
		t.Errorf("export reordered the caller's slice")
	}
}

func TestExportEmptySchema(t *testing.T) {
	t.Parallel()
	got, err := export(nil)
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	if want := "{\n  \"version\": 1,\n  \"keys\": []\n}\n"; string(got) != want {
		t.Errorf("export(nil) = %q, want %q", got, want)
	}
}

func runTool(args ...string) (code int, stdout, stderr string) {
	var out, errOut bytes.Buffer
	code = run(args, &out, &errOut)
	return code, out.String(), errOut.String()
}

func TestRunWritesSchema(t *testing.T) {
	t.Parallel()
	out := filepath.Join(t.TempDir(), "config-schema.json")
	code, stdout, stderr := runTool("-out", out)
	if code != 0 || stdout != "" || stderr != "" {
		t.Fatalf("exit %d, stdout %q, stderr %q", code, stdout, stderr)
	}
	got, err := os.ReadFile(filepath.Clean(out))
	if err != nil {
		t.Fatal(err)
	}
	want, err := export(serverKeys(t))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("written file differs from export")
	}
}

func TestRunUsageErrorExits2(t *testing.T) {
	t.Parallel()
	for _, args := range [][]string{{"-unknown"}, {"extra"}, {"-out"}} {
		code, stdout, stderr := runTool(args...)
		if code != 2 {
			t.Errorf("run(%q) = %d, want 2", args, code)
		}
		if stdout != "" || !strings.Contains(stderr, "Usage: configschema") {
			t.Errorf("run(%q): want usage on stderr only, stdout %q, stderr %q", args, stdout, stderr)
		}
	}
}

func TestRunHelpExits0(t *testing.T) {
	t.Parallel()
	for _, arg := range []string{"-h", "-help", "--help"} {
		code, stdout, stderr := runTool(arg)
		if code != 0 {
			t.Errorf("run(%s) = %d, want 0", arg, code)
		}
		if !strings.Contains(stdout, "Usage: configschema") || !strings.Contains(stdout, "-out") || stderr != "" {
			t.Errorf("run(%s): want usage with flags on stdout, stdout %q, stderr %q", arg, stdout, stderr)
		}
	}
}

func TestRunUnwritableOutputExits1(t *testing.T) {
	t.Parallel()
	out := filepath.Join(t.TempDir(), "missing-dir", "config-schema.json")
	code, _, stderr := runTool("-out", out)
	if code != 1 || !strings.Contains(stderr, "missing-dir") {
		t.Errorf("exit %d, stderr %q; want 1 and the path", code, stderr)
	}
}

func TestRunInvalidSchemaExits1(t *testing.T) {
	t.Parallel()
	var out, errOut bytes.Buffer
	broken := func() (*config.Schema, error) { return nil, errors.New("config schema: key \"x\": broken") }
	target := filepath.Join(t.TempDir(), "config-schema.json")
	code := runWith([]string{"-out", target}, &out, &errOut, broken)
	if code != 1 || !strings.Contains(errOut.String(), "broken") {
		t.Errorf("exit %d, stderr %q; want 1 and the schema error", code, errOut.String())
	}
	if _, err := os.Stat(target); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("output written despite the schema error")
	}
}
