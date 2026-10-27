// SPDX-License-Identifier: Apache-2.0

//go:build platform

package platform

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/host/internal/platform/testkit"
)

// The world of the suite: the service user bothy-host and the session users A, B and a third.
const (
	userA = "bothy-s-alex"
	userB = "bothy-s-sam"
	userC = "bothy-s-robin"
)

var (
	hostBin   = os.Getenv("BOTHY_HOST_BIN")
	fakeBin   = os.Getenv("BOTHY_FAKECLAUDE")
	fakeNode  = os.Getenv("BOTHY_FAKENODE")
	resultOut = os.Getenv("BOTHY_PLATFORM_RESULT")
	commit    = os.Getenv("BOTHY_COMMIT")
)

type result struct {
	Name     string  `json:"name"`
	Passed   bool    `json:"passed"`
	Duration float64 `json:"duration_seconds"`
}

var (
	resultsMu sync.Mutex
	results   []result
)

func TestMain(m *testing.M) {
	// The marker of a disposable machine guards the test binary itself (brief M2-48 design note
	// 11): the binary of a platform bundle runs without make, so the check of
	// BOTHY_PLATFORM_DISPOSABLE in the Makefile no longer stands in front of every run.
	if code := Guard(os.Stderr, MarkerPath(runtime.GOOS, os.Getenv("ProgramData"))); code != 0 {
		os.Exit(code)
	}
	if hostBin == "" || fakeBin == "" {
		fmt.Fprintln(os.Stderr, "platform suite: BOTHY_HOST_BIN and BOTHY_FAKECLAUDE are required (make -C services/host platform-test or run.sh of a platform bundle)")
		os.Exit(2)
	}
	if prebuilt := os.Getenv("BOTHY_SUITE_FAKECLAUDE"); prebuilt != "" {
		fakeOnce.Do(func() { errFake = installPrebuiltFake(prebuilt) })
	}
	code := m.Run()
	writeResult()
	os.Exit(code)
}

// installPrebuiltFake puts the fake claude of a platform bundle where buildFake would build it, so
// a machine without Go runs the suite. The bundle links the start log into the binary; a binary
// linked for another start log would log where no test looks, so that is an error.
func installPrebuiltFake(src string) error {
	b, err := os.ReadFile(src) //nolint:gosec // G304: the fake of the bundle that run.sh names
	if err != nil {
		return err
	}
	if !bytes.Contains(b, []byte(startLogPath())) {
		return fmt.Errorf("%s is not linked with the start log %s", src, startLogPath())
	}
	if err := os.MkdirAll(fakeDir(), 0o755); err != nil { //nolint:gosec // G301: every session user runs the programs in it
		return err
	}
	if err := os.WriteFile(filepath.Join(fakeDir(), exeName("claude")), b, 0o755); err != nil { //nolint:gosec // G306: every session user runs it
		return err
	}
	return openForAll(startLogPath())
}

// track records the outcome of t in the result file.
func track(t *testing.T) {
	start := time.Now()
	t.Cleanup(func() {
		if t.Failed() {
			// The service log of the test names the cause of a lost opening, a crash or a
			// refused start; the test output alone does not (a CI runner keeps nothing else).
			for _, l := range failureLog(start) {
				t.Log(l)
			}
		}
		resultsMu.Lock()
		defer resultsMu.Unlock()
		results = append(results, result{Name: t.Name(), Passed: !t.Failed(), Duration: time.Since(start).Seconds()})
	})
}

// maxServiceLog bounds the service log lines a failed test prints (a test constant; the terminal
// suite passes on at most 40 message lines of the run).
const maxServiceLog = 30

// failureLog returns the last maxServiceLog lines of the log of the host service since start
// (Linux: the journal of bothy-host.service; other platforms: none). The service logs codes, IDs
// and names only (24 10.1).
func failureLog(start time.Time) []string {
	if runtime.GOOS != "linux" {
		return nil
	}
	out, _ := try("/usr/bin/journalctl", "--no-pager", "-o", "short-precise", "-u", "bothy-host.service",
		"--since", fmt.Sprintf("@%.6f", float64(start.UnixMicro())/1e6))
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) > maxServiceLog {
		lines = lines[len(lines)-maxServiceLog:]
	}
	return slices.DeleteFunc(lines, func(l string) bool { return strings.TrimSpace(l) == "" })
}

func writeResult() {
	if resultOut == "" {
		return
	}
	osv, _ := exec.Command(osVersionCmd()[0], osVersionCmd()[1:]...).Output() //nolint:gosec // fixed command
	b, _ := json.MarshalIndent(map[string]any{
		"platform": runtime.GOOS, "os_version": strings.TrimSpace(string(osv)), "go_version": runtime.Version(), "commit": commit,
		"tests": results, "entry_scan": entryScanMeasurement,
	}, "", "  ")
	_ = os.WriteFile(resultOut, b, 0o644)
}

func osVersionCmd() []string {
	switch runtime.GOOS {
	case "darwin":
		return []string{"/usr/bin/sw_vers", "-productVersion"}
	case "windows":
		return []string{"cmd", "/c", "ver"}
	}
	return []string{"/bin/uname", "-sr"}
}

// sh runs a command as the suite (administrator) and fails the test on an error.
func sh(t *testing.T, name string, args ...string) string {
	t.Helper()
	out, err := exec.Command(name, args...).CombinedOutput() //nolint:gosec // commands of the suite
	if err != nil {
		t.Fatalf("%s %v: %v\n%s", name, args, err, out)
	}
	return string(out)
}

// try runs a command and returns its output and exit code.
func try(name string, args ...string) (string, int) {
	cmd := exec.Command(name, args...) //nolint:gosec // commands of the suite
	out, err := cmd.CombinedOutput()
	if ee, ok := err.(*exec.ExitError); ok {
		return string(out), ee.ExitCode()
	}
	if err != nil {
		return err.Error(), -1
	}
	return string(out), 0
}

// host runs bothy-host with args and returns output and exit code.
func host(args ...string) (string, int) { return try(hostBin, args...) }

// configPath is the configuration file of the platform.
func configPath() string {
	switch runtime.GOOS {
	case "darwin":
		return "/Library/Application Support/bothy-host/config.yaml"
	case "windows":
		return os.Getenv("ProgramFiles") + `\bothy-host\config.yaml`
	}
	return "/etc/bothy-host/config.yaml"
}

func stateDir() string {
	if runtime.GOOS == "windows" {
		return os.Getenv("ProgramData") + `\bothy-host`
	}
	return "/var/lib/bothy-host"
}

// writeConfig writes the configuration file (owned by the administrator).
func writeConfig(t *testing.T, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(configPath()), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(configPath(), []byte("host_service:\n"+body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// installed installs the host service with the session users of users and returns a cleanup
// that removes everything again.
func installed(t *testing.T, extra string, users ...string) {
	t.Helper()
	writeConfig(t, "  session_users: ["+strings.Join(users, ", ")+"]\n"+extra)
	// The cleanup comes before install: a failed install may have created users, groups or
	// services, and every later test would trip over them.
	t.Cleanup(func() { uninstall(t, users) })
	if out, code := host("install", "--drive-root-confirm", "yes"); code != 0 {
		t.Fatalf("install: %d\n%s", code, out)
	}
	// Folders a test created for session users before install made them (mkdirOwned) get
	// their owner now; otherwise HD7 drops them as unresolvable and the test of a doctor
	// finding sees a list without them.
	applyPendingOwners(t)
}

// doctorEntries runs doctor and returns output and exit code.
func doctor(t *testing.T) (string, int) {
	t.Helper()
	return host("doctor")
}

// doctorAnswer runs doctor until it answers: a service that refuses its start while doctor asks it
// closes the admin endpoint before the answer, and the next call runs the checks without it.
func doctorAnswer(t *testing.T) (string, int) {
	t.Helper()
	deadline := testkit.Deadline()
	for {
		out, c := doctor(t)
		if !strings.Contains(out, "the running service did not answer") || time.Now().After(deadline) {
			return out, c
		}
		time.Sleep(500 * time.Millisecond)
	}
}

func hasLine(out, line string) bool {
	return slices.ContainsFunc(strings.Split(out, "\n"), func(l string) bool { return strings.HasPrefix(l, line) })
}

// waitExit waits until the host service is stopped and returns whether it stopped with 78.
func waitRefused(t *testing.T) bool {
	t.Helper()
	ctx, cancel := context.WithDeadline(context.Background(), testkit.Deadline())
	defer cancel()
	for ctx.Err() == nil {
		if code, stopped := serviceExit(); stopped {
			return code == 78
		}
		time.Sleep(500 * time.Millisecond)
	}
	return false
}

var _ = bytes.Equal
