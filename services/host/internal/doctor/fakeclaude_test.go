// SPDX-License-Identifier: Apache-2.0

//go:build linux || darwin

package doctor

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/host/internal/clock"
	"github.com/OWNER/bothy/services/host/internal/hostos"
)

// TestHD6WithFakeClaude runs the real program testdata/fakeclaude through the helper-side check:
// only host_service.claude_bin starts, with the fixed environment, once per CLAUDE_CONFIG_DIR.
func TestHD6WithFakeClaude(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "bin", "claude")
	logFile := filepath.Join(dir, "starts.log")
	build := exec.CommandContext(context.Background(), "go", "build", "-ldflags", "-X main.startLog="+logFile, "-o", bin, "./testdata/fakeclaude") //nolint:gosec // the test builds its fake
	build.Dir = filepath.Join("..", "..")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build fakeclaude: %v\n%s", err, out)
	}
	home := filepath.Join(dir, "home")
	org := filepath.Join(home, OrgConfigDir)
	if err := os.MkdirAll(org, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(home, ".claude"), 0o700); err != nil {
		t.Fatal(err)
	}
	env := HelperEnv{GOOS: "linux", Self: hostos.Account{Name: "bothy-s-alex", Home: home}, Runner: hostos.ExecRunner{}, Clock: clock.Real(),
		Security: osSecurity{}}
	req := Request{ClaudeBin: bin, SearchList: []string{filepath.Dir(bin), "/usr/bin", "/bin"}, AuthStatusTimeout: 30 * time.Second}
	for _, tc := range []struct {
		where, method string
		want          LLMFacts
	}{
		{"", "none", LLMFacts{}},
		{".claude", "claude.ai", LLMFacts{Finding: true}},
		{"org", "oauth_token", LLMFacts{Finding: true}},
		{"org", "api_key", LLMFacts{}},
		{".claude", "garbage", LLMFacts{Unknown: true}},
		{"org", "unknown_method", LLMFacts{Unknown: true}},
	} {
		_ = os.Remove(filepath.Join(home, ".claude", "fake-auth"))
		_ = os.Remove(filepath.Join(org, "fake-auth"))
		switch tc.where {
		case ".claude":
			_ = os.WriteFile(filepath.Join(home, ".claude", "fake-auth"), []byte(tc.method), 0o600)
		case "org":
			_ = os.WriteFile(filepath.Join(org, "fake-auth"), []byte(tc.method), 0o600)
		}
		if got := checkLLM(context.Background(), env, req); got != tc.want {
			t.Errorf("%s in %q: %+v", tc.method, tc.where, got)
		}
	}
	// Timeout at host_service.auth_status_timeout.
	_ = os.WriteFile(filepath.Join(org, "fake-auth"), []byte("hang"), 0o600)
	req.AuthStatusTimeout = 200 * time.Millisecond
	if got := checkLLM(context.Background(), env, req); !got.Unknown {
		t.Errorf("hang: %+v", got)
	}
	b, _ := os.ReadFile(logFile) //nolint:gosec // the log of this test
	for _, line := range strings.Split(strings.TrimSpace(string(b)), "\n") {
		if line != bin {
			t.Errorf("started %q", line)
		}
	}
}

// osSecurity is a Security over os.Lstat that reports only existence (the login files).
type osSecurity struct{}

func (osSecurity) Lstat(p string) (hostos.FileSec, error) {
	_, err := os.Lstat(p)
	return hostos.FileSec{}, err
}
