// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/cmd/bothy/importtool/bothyapi"
	"github.com/OWNER/bothy/services/server/cmd/bothy/importtool/bothyapi/bothytest"
)

// testImportDeps returns dependencies that approve the pairing of fake at the first poll and
// never wait, with the session file in a temporary directory.
func testImportDeps(t *testing.T, fake *bothytest.Server) (importDeps, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "bothy", "import-session.json")
	return importDeps{
		sessionPath: func() (string, error) { return path, nil },
		hostname:    func() (string, error) { return "workstation", nil },
		limits:      bothyapi.DefaultLimits(),
		deps: bothyapi.Deps{Sleep: func(ctx context.Context, _ time.Duration) error {
			fake.Approve()
			return ctx.Err()
		}},
	}, path
}

// TestImportLoginLogout: login prints the code alone on stdout and writes the session file;
// logout deletes it; a second logout is exit 2.
func TestImportLoginLogout(t *testing.T) {
	fake := bothytest.New(t, bothytest.Options{})
	deps, path := testImportDeps(t, fake)
	var out, errw bytes.Buffer
	if code := importCommand(t.Context(), []string{"login", "--server", fake.URL, "--name", "Robin laptop"}, &out, &errw, deps); code != exitOK {
		t.Fatalf("login = %d: %s", code, errw.String())
	}
	if out.String() != fake.Code()+"\n" {
		t.Errorf("stdout = %q", out.String())
	}
	if reg, _ := fake.Registration(); reg.Name != "Robin laptop" {
		t.Errorf("name = %q", reg.Name)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal(err)
	}
	if code := importCommand(t.Context(), []string{"login", "--server", fake.URL}, &out, &errw, deps); code != exitUsage {
		t.Errorf("second login = %d, want 2", code)
	}
	if code := importCommand(t.Context(), []string{"logout"}, &out, &errw, deps); code != exitOK {
		t.Fatalf("logout = %d: %s", code, errw.String())
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("session file after logout: %v", err)
	}
	errw.Reset()
	if code := importCommand(t.Context(), []string{"logout"}, &out, &errw, deps); code != exitUsage {
		t.Errorf("logout without session = %d, want 2", code)
	}
	if !strings.Contains(errw.String(), "bothy import login") {
		t.Errorf("stderr = %q, want the login hint", errw.String())
	}
}

// TestImportSessionFileFlag: --session-file overrides the default path.
func TestImportSessionFileFlag(t *testing.T) {
	fake := bothytest.New(t, bothytest.Options{})
	deps, def := testImportDeps(t, fake)
	other := filepath.Join(t.TempDir(), "s", "session.json")
	var out, errw bytes.Buffer
	if code := importCommand(t.Context(), []string{"login", "--server", fake.URL, "--session-file", other}, &out, &errw, deps); code != exitOK {
		t.Fatalf("login = %d: %s", code, errw.String())
	}
	if _, err := os.Stat(other); err != nil {
		t.Errorf("--session-file not written: %v", err)
	}
	if _, err := os.Stat(def); !os.IsNotExist(err) {
		t.Errorf("default path written: %v", err)
	}
}

// TestImportUsage: help is exit 0, wrong calls exit 2 without any request.
func TestImportUsage(t *testing.T) {
	fake := bothytest.New(t, bothytest.Options{})
	deps, _ := testImportDeps(t, fake)
	var out, errw bytes.Buffer
	if code := importCommand(t.Context(), []string{"--help"}, &out, &errw, deps); code != exitOK || !strings.Contains(out.String(), "bothy import login") {
		t.Errorf("--help = %d %q", code, out.String())
	}
	for _, args := range [][]string{nil, {"discord"}, {"login"}, {"login", "--server", "http://example.com"},
		{"login", "--server", fake.URL, "extra"}, {"logout", "--bogus"}, {"login", "--help"}} {
		errw.Reset()
		out.Reset()
		code := importCommand(t.Context(), args, &out, &errw, deps)
		want := exitUsage
		if len(args) == 2 && args[1] == "--help" {
			want = exitOK
		}
		if code != want {
			t.Errorf("%v = %d, want %d (%s)", args, code, want, errw.String())
		}
	}
	if n := len(fake.Requests()); n != 0 {
		t.Errorf("requests = %d, want 0", n)
	}
}

// TestImportLoginFailureIsExit1: a rejected pairing is exit 1 and writes nothing.
func TestImportLoginFailureIsExit1(t *testing.T) {
	fake := bothytest.New(t, bothytest.Options{})
	deps, path := testImportDeps(t, fake)
	deps.deps.Sleep = func(ctx context.Context, _ time.Duration) error {
		fake.Reject()
		return ctx.Err()
	}
	var out, errw bytes.Buffer
	if code := importCommand(t.Context(), []string{"login", "--server", fake.URL}, &out, &errw, deps); code != exitFailure {
		t.Errorf("login = %d, want 1", code)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("session file written: %v", err)
	}
}
