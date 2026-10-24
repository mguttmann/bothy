// SPDX-License-Identifier: Apache-2.0

package agents_test

import (
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/platform/testkit"
)

// canaryCredential stands for an API token in the process tests.
const canaryCredential = "bothy_agt_canarycanarycanarycanary0000" //nolint:gosec // G101: a canary that must never leak, no credential

// TestFakeAgentProcess_CredentialsOnlyOnStdin: the credentials reach the process in hello over
// stdin; its argv and environment hold none (brief Design note 6).
func TestFakeAgentProcess_CredentialsOnlyOnStdin(t *testing.T) {
	t.Parallel()
	f := startFakeAgent(t, "canary")
	for _, part := range append(append([]string{}, f.cmd.Args...), f.cmd.Env...) {
		if strings.Contains(part, canaryCredential) {
			t.Fatalf("argv or environment holds the credential: %s", redact(part))
		}
	}
	// hello with a key that is not P-256 fails as a usage error; the process keeps serving.
	r, err := f.call(testkit.Context(t), "hello", map[string]any{
		"base_url": "http://127.0.0.1:9", "gateway_url": "/api/v1/gateway", "instance_id": "0192f4a0-7b1c-7000-8000-000000000001",
		"agent_id": "0192f4a0-7b1c-7000-8000-000000000002", "credentials": map[string]any{"token": canaryCredential, "dpop_private_jwk": map[string]any{"kty": "OKP"}},
	}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if r.OK || r.Error == nil || r.Error.Kind != "usage" || strings.Contains(r.Error.Detail, canaryCredential) {
		t.Fatalf("hello with a bad key: %+v", r)
	}
	if strings.Contains(f.stderr.String(), canaryCredential) {
		t.Fatal("stderr holds the credential")
	}
}

// TestFakeAgentProcess_KilledOnCleanup: the process of a finished test is gone (only its PID is
// stopped), and crash exits at once with a non-zero status.
func TestFakeAgentProcess_KilledOnCleanup(t *testing.T) {
	t.Parallel()
	var pid int
	t.Run("process", func(t *testing.T) {
		f := startFakeAgent(t, "cleanup")
		pid = f.cmd.Process.Pid
		if f.Exited() {
			t.Fatal("the process ended at once")
		}
	})
	if pid == 0 {
		t.Fatal("no process")
	}
	if err := syscall.Kill(pid, 0); err == nil {
		t.Fatalf("the process %d still runs after its test ended", pid)
	}
	// uv runs Python as its child: nothing of the process group outlives the test.
	// The kernel reaps the killed group asynchronously: wait until no member is left.
	testkit.Until(t, "the process group ends", func() bool { return syscall.Kill(-pid, 0) != nil })
	f := startFakeAgent(t, "crash")
	if _, err := f.call(testkit.Context(t), "crash", nil, time.Now()); err != nil {
		t.Fatal(err)
	}
	// call returned after the exit: the waiting goroutine set ProcessState.
	if code := f.cmd.ProcessState.ExitCode(); code == 0 {
		t.Fatalf("crash exited with %d", code)
	}
}
