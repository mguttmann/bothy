// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/OWNER/bothy/services/server/cmd/bothy/demo"
	"github.com/OWNER/bothy/services/server/internal/platform/config"
	"github.com/OWNER/bothy/services/server/internal/wire"
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
	schema, err := os.ReadFile(filepath.Join(demoDataDir, "demo.schema.json")) //nolint:gosec // G304: the data set of the repository
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "demo.schema.json"), schema, 0o600); err != nil { //nolint:gosec // G703: a file of t.TempDir
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

// fakeBootstrap is a setupRunner whose BootstrapOwner answers with res and err.
type fakeBootstrap struct {
	setupRunner
	res wire.BootstrapResult
	err error
	got wire.BootstrapOwnerCmd
}

func (f *fakeBootstrap) BootstrapOwner(_ context.Context, cmd wire.BootstrapOwnerCmd) (wire.BootstrapResult, error) {
	f.got = cmd
	return f.res, f.err
}

// TestDemoBootstrap: the bootstrap of demo.Run creates the owner of the data set, maps a complete
// setup to demo.ErrNotEmpty and passes other errors on.
func TestDemoBootstrap(t *testing.T) {
	t.Parallel()
	owner := demo.Person{Handle: "alex", DisplayName: "Alex", Locale: "en", TimeZone: "UTC"}
	f := &fakeBootstrap{res: wire.BootstrapResult{Code: wire.IssuedEnrollmentCode{Code: "en_X"}}}
	if code, err := demoBootstrap(f)(t.Context(), owner); err != nil || code != "en_X" || f.got.Handle != "alex" || f.got.Locale != "en" {
		t.Fatalf("code %q, err %v, cmd %+v", code, err, f.got)
	}
	f.err = wire.ErrSetupCompleted
	if _, err := demoBootstrap(f)(t.Context(), owner); !errors.Is(err, demo.ErrNotEmpty) {
		t.Fatalf("complete setup: %v", err)
	}
	other := errors.New("database down")
	f.err = other
	if _, err := demoBootstrap(f)(t.Context(), owner); !errors.Is(err, other) || errors.Is(err, demo.ErrNotEmpty) {
		t.Fatalf("other error: %v", err)
	}
}

// TestDemoPaceAndAPI: --pace and the default API address of server.listen_addr.
func TestDemoPaceAndAPI(t *testing.T) {
	t.Parallel()
	if d, err := parsePace(""); err != nil || d != -1 {
		t.Fatalf("empty pace: %v %v", d, err)
	}
	if d, err := parsePace("2s"); err != nil || d.Seconds() != 2 {
		t.Fatalf("2s: %v %v", d, err)
	}
	schema, err := wire.ConfigSchema()
	if err != nil {
		t.Fatal(err)
	}
	for listen, want := range map[string]string{":8080": "http://127.0.0.1:8080", "0.0.0.0:9000": "http://127.0.0.1:9000"} {
		v, err := config.Load(schema, config.Sources{Env: []string{"BOTHY_SERVER__LISTEN_ADDR=" + listen, "BOTHY_SERVER__PUBLIC_URL=https://chat.example.com"}})
		if err != nil {
			t.Fatal(err)
		}
		if got, err := loopbackAPI(v); err != nil || got != want {
			t.Errorf("%s: %q %v, want %q", listen, got, err, want)
		}
	}
}
