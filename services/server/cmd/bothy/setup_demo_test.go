// SPDX-License-Identifier: Apache-2.0

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// demoDataDir is the data set of the repository (infra/demo), seen from this package.
var demoDataDir = filepath.Join("..", "..", "..", "..", "infra", "demo")

// TestDemoRefusesNonEmpty (brief M7-27 requirement 14): on an instance whose setup is complete,
// bothy setup demo ends with exit 3 before its first write; no row changes and stdout stays empty.
// The --api address is unreachable on purpose: the command must not get as far as a request.
func TestDemoRefusesNonEmpty(t *testing.T) {
	t.Parallel()
	db := newServeDB(t)
	env := serveTestEnv(t, db)
	if code, out, errText := runSetup(t, env, "bootstrap-owner", "--handle", "alex", "--display-name", "Alex"); code != 0 || !codeLine.MatchString(out) {
		t.Fatalf("bootstrap: exit %d, stdout %q, stderr %s", code, out, errText)
	}
	before := tableCounts(t, db)
	code, out, errText := runSetup(t, env, "demo", "--data", demoDataDir, "--api", "http://127.0.0.1:1")
	if code != exitDemoNotEmpty || out != "" || !strings.Contains(errText, "not empty") {
		t.Fatalf("demo: exit %d, stdout %q, stderr %s", code, out, errText)
	}
	if after := tableCounts(t, db); after != before {
		t.Fatalf("rows changed: %s, before %s", after, before)
	}
}

// TestDemoInvalidDataSet: a data set that does not match its schema is exit 2 before any
// connection, and the message names the place, not the value.
func TestDemoInvalidDataSet(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	schema, err := os.ReadFile(filepath.Join(demoDataDir, "demo.schema.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "demo.schema.json"), schema, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "demo.yaml"), []byte("version: 1\nvalues: {secret_value: 7}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	code, out, errText := runSetup(t, nil, "demo", "--data", dir)
	if code != exitUsage || out != "" || !strings.Contains(errText, "invalid data set") || strings.Contains(errText, "secret_value: 7") {
		t.Fatalf("exit %d, stdout %q, stderr %s", code, out, errText)
	}
}

// TestDemoFlags: wrong flags are exit 2 with the usage.
func TestDemoFlags(t *testing.T) {
	t.Parallel()
	for _, args := range [][]string{
		{"demo", "--pace", "-1s"},
		{"demo", "--pace", "soon"},
		{"demo", "--seed", "x"},
		{"demo", "extra"},
		{"demo", "--data", ""},
	} {
		if code, _, errText := runSetup(t, nil, args...); code != exitUsage || !strings.Contains(errText, "Usage: bothy setup") {
			t.Errorf("%v: exit %d, stderr %s", args, code, errText)
		}
	}
	if code, out, _ := runSetup(t, nil, "demo", "--help"); code != exitOK || !strings.Contains(out, "bothy setup demo") {
		t.Errorf("--help: exit %d, stdout %q", code, out)
	}
}
