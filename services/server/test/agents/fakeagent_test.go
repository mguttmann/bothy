// SPDX-License-Identifier: Apache-2.0

package agents_test

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/platform/testkit"
)

// fakeAgentModule is the module of the Fake-Agent in sdk/python (brief Design note 5).
const fakeAgentModule = "bothy_sdk.testing.fake_agent"

// fakeAgentCommand returns the program and arguments that start one Fake-Agent: uv from PATH
// with the locked project sdk/python of the repository. A missing uv fails, it never skips
// (plan P-09).
func fakeAgentCommand() (string, []string, error) {
	uv, err := exec.LookPath("uv")
	if err != nil {
		return "", nil, errors.New("the conformance suite needs uv on PATH to run the Fake-Agent of sdk/python")
	}
	root, err := repoRoot()
	if err != nil {
		return "", nil, err
	}
	return uv, []string{"run", "--project", filepath.Join(root, "sdk", "python"), "--locked", "--quiet", "python", "-m", fakeAgentModule}, nil
}

// syncFakeAgentProject creates the environment of sdk/python once per test binary before the
// first Fake-Agent starts, so the parallel cases never set it up at the same time (a fresh CI
// checkout has none).
var syncFakeAgentProject = sync.OnceValue(func() error {
	uv, err := exec.LookPath("uv")
	if err != nil {
		return errors.New("the conformance suite needs uv on PATH to run the Fake-Agent of sdk/python")
	}
	root, err := repoRoot()
	if err != nil {
		return err
	}
	cmd := exec.CommandContext(context.Background(), uv, "sync", "--project", filepath.Join(root, "sdk", "python"), "--locked", "--quiet") // #nosec G204 -- uv from PATH with constant arguments and a repository path
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("uv sync of sdk/python: %w\n%s", err, out)
	}
	return nil
})

// reply is one reply line of the Fake-Agent.
type reply struct {
	ID     int            `json:"id"`
	OK     bool           `json:"ok"`
	Result map[string]any `json:"result"`
	Error  *struct {
		Kind   string `json:"kind"`
		Detail string `json:"detail"`
	} `json:"error"`
}

// fakeAgent is one running Fake-Agent process and its control pipe.
type fakeAgent struct {
	name   string
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	lines  chan []byte
	stderr *lockedBuffer
	exited chan struct{}
	mu     sync.Mutex
	nextID int
}

// lockedBuffer collects the stderr of a process for failure messages.
type lockedBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (l *lockedBuffer) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.Write(p)
}

func (l *lockedBuffer) String() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.String()
}

// startFakeAgent starts one Fake-Agent process; it is killed in t.Cleanup and its exit awaited
// (only this PID, never by name). The credentials go only over stdin in hello, never into argv
// or the environment.
func startFakeAgent(t testing.TB, name string) *fakeAgent {
	t.Helper()
	prog, args, err := fakeAgentCommand()
	if err != nil {
		t.Fatal(err)
	}
	if err := syncFakeAgentProject(); err != nil {
		t.Fatal(err)
	}
	// The process lives until t.Cleanup stops it; the test's context ending kills it too.
	cmd := exec.CommandContext(context.WithoutCancel(t.Context()), prog, args...) // #nosec G204 -- uv from PATH with constant arguments and repository paths
	root, err := repoRoot()
	if err != nil {
		t.Fatal(err)
	}
	cmd.Env = fakeAgentEnv(cmd, root)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	f := &fakeAgent{name: name, cmd: cmd, stdin: stdin, lines: make(chan []byte, 16), stderr: &lockedBuffer{}, exited: make(chan struct{})}
	cmd.Stderr = f.stderr
	// uv runs Python as its child: the Fake-Agent gets its own process group, and stop and the
	// context kill the whole group, so no Python process outlives its test or holds the stderr
	// pipe that Wait drains.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = f.killGroup
	cmd.WaitDelay = testkit.Patience
	if err := cmd.Start(); err != nil {
		t.Fatalf("start the Fake-Agent %s: %v", name, err)
	}
	go func() {
		sc := bufio.NewScanner(stdout)
		sc.Buffer(make([]byte, 0, 64*1024), 16<<20)
		for sc.Scan() {
			f.lines <- append([]byte(nil), sc.Bytes()...)
		}
		close(f.lines)
	}()
	go func() {
		_ = cmd.Wait()
		close(f.exited)
	}()
	t.Cleanup(f.stop)
	return f
}

// fakeAgentEnv is the environment of a Fake-Agent: the test's own (cmd.Environ) plus
// PYTHONPATH on sdk/python/src, because the SDK is not an installed package
// (sdk/python/pyproject.toml, M2-23/1); a later entry wins, so an inherited PYTHONPATH is
// replaced. It carries no credential; those go over stdin only.
func fakeAgentEnv(cmd *exec.Cmd, root string) []string {
	return append(cmd.Environ(), "PYTHONPATH="+filepath.Join(root, "sdk", "python", "src")) //nolint:forbidigo // environment of the uv subprocess, not server configuration
}

// stop closes stdin, kills the process (only this PID) and waits for its exit.
func (f *fakeAgent) stop() {
	_ = f.stdin.Close()
	_ = f.killGroup()
	<-f.exited
}

// killGroup kills the process group of the Fake-Agent (uv and its Python child), by the PID of
// the process this test started.
func (f *fakeAgent) killGroup() error {
	if f.cmd.Process == nil {
		return nil
	}
	return syscall.Kill(-f.cmd.Process.Pid, syscall.SIGKILL)
}

// Exited reports whether the process has exited.
func (f *fakeAgent) Exited() bool {
	select {
	case <-f.exited:
		return true
	default:
		return false
	}
}

// call sends one request and waits for its reply, the exit of the process or ctx. now is the
// harness clock.
func (f *fakeAgent) call(ctx context.Context, op string, args map[string]any, now time.Time) (reply, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.nextID++
	id := f.nextID
	if args == nil {
		args = map[string]any{}
	}
	line, err := json.Marshal(map[string]any{"id": id, "op": op, "args": args, "now": now.UTC().Format("2006-01-02T15:04:05.000Z")})
	if err != nil {
		return reply{}, err
	}
	if _, err := f.stdin.Write(append(line, '\n')); err != nil {
		return reply{}, fmt.Errorf("write to the Fake-Agent %s: %w", f.name, err)
	}
	if op == "crash" {
		select {
		case <-f.exited:
			return reply{ID: id, OK: true, Result: map[string]any{}}, nil
		case <-ctx.Done():
			return reply{}, fmt.Errorf("the Fake-Agent %s did not exit on crash", f.name)
		}
	}
	for {
		select {
		case b, ok := <-f.lines:
			if !ok {
				return reply{}, fmt.Errorf("the Fake-Agent %s exited during %s; stderr: %s", f.name, op, redact(f.stderr.String()))
			}
			var r reply
			if err := json.Unmarshal(b, &r); err != nil {
				return reply{}, fmt.Errorf("the Fake-Agent %s wrote a line that is no reply", f.name)
			}
			if r.ID != id {
				continue
			}
			return r, nil
		case <-ctx.Done():
			return reply{}, fmt.Errorf("the Fake-Agent %s did not answer %s before the deadline; stderr: %s", f.name, op, redact(f.stderr.String()))
		}
	}
}
