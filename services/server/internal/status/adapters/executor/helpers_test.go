// SPDX-License-Identifier: Apache-2.0

package executor_test

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/status/adapters/executor"
	"github.com/OWNER/bothy/services/server/internal/status/app"
	"github.com/OWNER/bothy/services/server/internal/status/domain"
)

// Throwaway secret values of the tests (EXAMPLE, never real).
const (
	exampleResticPassword = "EXAMPLE-restic-password"
	exampleCipherPass     = "EXAMPLE-cipher-pass"
	exampleAccessKey      = "EXAMPLEACCESSKEY"
	exampleSecretKey      = "EXAMPLE-secret-key"
	exampleAdminAccess    = "EXAMPLEADMINACCESS"
	exampleAdminSecret    = "EXAMPLE-admin-secret"
	exampleDBPassword     = "EXAMPLE:db\\password" // #nosec G101 -- a throwaway test value
	// parentSentinel is a variable of the test process that must never reach a child.
	parentSentinel = "BOTHY_EXAMPLE_PARENT_SECRET"
)

// call is one recorded invocation of a fake binary.
type call struct {
	args []string
	env  map[string]string
}

// fake is a fake binary: a shell script in a temporary directory that appends its argv and its
// environment to a log and then runs body. The script is a test fixture; the executor itself
// never runs a shell.
type fake struct {
	path, log string
}

// newFake writes the fake binary name into dir with body as the rest of the script.
func newFake(t *testing.T, dir, name, body string) fake {
	t.Helper()
	f := fake{path: filepath.Join(dir, name), log: filepath.Join(dir, name+".log")}
	script := "#!/bin/sh\n" +
		"{ echo call; for a in \"$@\"; do echo \"arg=$a\"; done; env | sed 's/^/env=/'; echo end; } >> '" + f.log + "'\n" +
		body + "\n"
	if err := os.WriteFile(f.path, []byte(script), 0o700); err != nil { // #nosec G306 -- an executable test fixture
		t.Fatal(err)
	}
	return f
}

// calls parses the log of f.
func (f fake) calls(t *testing.T) []call {
	t.Helper()
	b, err := os.ReadFile(f.log)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		t.Fatal(err)
	}
	var out []call
	var cur *call
	for _, line := range strings.Split(string(b), "\n") {
		switch {
		case line == "call":
			out = append(out, call{env: map[string]string{}})
			cur = &out[len(out)-1]
		case line == "end":
			cur = nil
		case cur != nil && strings.HasPrefix(line, "arg="):
			cur.args = append(cur.args, strings.TrimPrefix(line, "arg="))
		case cur != nil && strings.HasPrefix(line, "env="):
			k, v, _ := strings.Cut(strings.TrimPrefix(line, "env="), "=")
			cur.env[k] = v
		}
	}
	return out
}

// shellVars are the variables /bin/sh sets itself.
var shellVars = map[string]bool{"PWD": true, "SHLVL": true, "_": true, "OLDPWD": true}

// checkEnv fails when c's environment holds a variable outside the base list and want, or lacks
// or differs from a variable of want.
func checkEnv(t *testing.T, c call, want map[string]string) {
	t.Helper()
	base := map[string]bool{"PATH": true, "HOME": true, "LANG": true}
	for k, v := range c.env {
		if shellVars[k] || base[k] {
			continue
		}
		if _, ok := want[k]; !ok {
			t.Errorf("unexpected child variable %s=%q", k, v)
		}
	}
	for k, v := range want {
		if got, ok := c.env[k]; !ok || got != v {
			t.Errorf("child variable %s = %q (set %v), want %q", k, got, ok, v)
		}
	}
	if _, ok := c.env[parentSentinel]; ok {
		t.Error("the parent environment reached the child")
	}
}

// secretsDir writes the secret files of the tests and returns the directory.
func secretsDir(t *testing.T, withCA bool) string {
	t.Helper()
	dir := t.TempDir()
	write(t, filepath.Join(dir, "restic_password"), exampleResticPassword+"\n")
	write(t, filepath.Join(dir, "pgbackrest_cipher_pass"), exampleCipherPass+"\n")
	if err := os.Mkdir(filepath.Join(dir, "offsite"), 0o700); err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(dir, "offsite", "access_key_id"), exampleAccessKey+"\n")
	write(t, filepath.Join(dir, "offsite", "secret_access_key"), exampleSecretKey)
	if withCA {
		write(t, filepath.Join(dir, "offsite", "ca.crt"), "EXAMPLE PEM\n")
	}
	return dir
}

// adminDir writes the owner's credentials directory of the tests.
func adminDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	write(t, filepath.Join(dir, "access_key_id"), exampleAdminAccess+"\n")
	write(t, filepath.Join(dir, "secret_access_key"), exampleAdminSecret+"\n")
	return dir
}

func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

// exampleTarget is the off-site target of the tests.
func exampleTarget() *domain.OffsiteTarget {
	return &domain.OffsiteTarget{Host: "s3.example.com", Port: 9443, Bucket: "bothy-backup", Prefix: "instance-1"}
}

// syncBuffer is a bytes.Buffer for the stderr copy goroutine of exec.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// newExecutor builds an executor over cfg and fails the test on an error.
func newExecutor(t *testing.T, cfg executor.Config) *executor.Executor {
	t.Helper()
	t.Setenv(parentSentinel, "EXAMPLE")
	e, err := executor.New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	return e
}

// wantCode fails unless err is an *app.ExecError with code.
func wantCode(t *testing.T, err error, code string) {
	t.Helper()
	var ee *app.ExecError
	if !errors.As(err, &ee) || ee.Code != code {
		t.Fatalf("error %v, want ExecError %s", err, code)
	}
	if !domain.ValidErrorCode(ee.Code) {
		t.Fatalf("code %q is not a valid error code", ee.Code)
	}
}
