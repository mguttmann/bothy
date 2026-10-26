// SPDX-License-Identifier: Apache-2.0

package hostplatform

import (
	"context"
	"encoding/json"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"
)

// bundleDir is where the bundle lies in the host container: a folder owned by root, as the suite
// runs as root.
const bundleDir = "/opt/termsuite/platform"

// resultFile is the result file run.sh writes for Linux (BOTHY_PLATFORM_RESULT unset).
const resultFile = bundleDir + "/result-linux.json"

// runMargin is the time the test keeps after the run for fetching the result and removing the
// container (a test constant).
const runMargin = 45 * time.Second

// platformResult is the part of M2-42's result file the test reads.
type platformResult struct {
	Platform string `json:"platform"`
	Tests    []struct {
		Name   string `json:"name"`
		Passed bool   `json:"passed"`
	} `json:"tests"`
}

// failRE finds the names of failed tests in the output of a test binary, the only part of the
// output the test passes on when the result file is missing.
var failRE = regexp.MustCompile(`(?m)^\s*--- FAIL: (\S+)`)

// messageRE finds the message lines of the platform tests (file:line: text). They carry no
// session content (the suite reports output only as length and hash, 24 10.1), so the test
// passes them on with a failure; without them a failure on a CI runner names no cause.
var messageRE = regexp.MustCompile(`(?m)^\s+[\w.-]+_test\.go:\d+: .*$`)

// Bounds of the message lines passed on (test constants).
const (
	maxMessages    = 40
	maxMessageLine = 300
)

// messages returns the message lines of out, at most maxMessages, each cut to maxMessageLine bytes.
func messages(out string) []string {
	var lines []string
	for _, m := range messageRE.FindAllString(out, maxMessages) {
		m = strings.TrimSpace(m)
		if len(m) > maxMessageLine {
			m = m[:maxMessageLine]
		}
		lines = append(lines, m)
	}
	return lines
}

// TestPlatformSuiteLinux runs the Linux platform bundle as root in a fresh host container (Design
// note 11): run.sh suite runs M2-45's TestSessionCgroup and then the platform suite, which writes
// result-linux.json. The test fails when the run exits other than 0, the result file is missing or
// empty, or any test in it failed. It prints the counts and the names of failed tests, never any
// other output of the run than the message lines of the platform tests when one failed.
func TestPlatformSuiteLinux(t *testing.T) {
	ctx := context.Background()
	if d, ok := t.Deadline(); ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithDeadline(ctx, d.Add(-runMargin/3))
		defer cancel()
	}
	h, err := pkg.stack.NewHost(ctx, "suite", "linux")
	t.Cleanup(func() {
		if rerr := h.Remove(context.WithoutCancel(ctx)); rerr != nil {
			t.Errorf("remove host container: %v", rerr)
		}
	})
	if err != nil {
		t.Fatalf("host container: %v", err)
	}
	if _, err := h.Shell(ctx, "install -d -o root -g root -m 0755 "+bundleDir); err != nil {
		t.Fatal(err)
	}
	if err := h.CopyIn(ctx, pkg.bundle+"/.", bundleDir); err != nil {
		t.Fatalf("copy bundle: %v", err)
	}
	if _, err := h.Shell(ctx, "chown -R root:root "+bundleDir+" && chmod 0755 "+bundleDir+"/run.sh "+bundleDir+"/bin/*"); err != nil {
		t.Fatal(err)
	}
	args := []string{"bash", bundleDir + "/run.sh", "suite"}
	if d, ok := ctx.Deadline(); ok {
		// The inner timeout ends before the context, so a hanging test ends the binary with the
		// names of the running tests instead of a killed docker exec.
		args = append(args, "-test.timeout="+time.Until(d.Add(-runMargin)).Round(time.Second).String())
	}
	t0 := time.Now()
	run, err := h.Exec(ctx, nil, args...)
	wall := time.Since(t0).Round(time.Second)
	if err != nil {
		t.Fatalf("run.sh suite: %v", err)
	}
	t.Logf("platform run: exit %d after %s", run.Code, wall)
	res, rerr := h.Exec(ctx, nil, "cat", resultFile)
	if rerr != nil || res.Code != 0 {
		names := failRE.FindAllStringSubmatch(string(run.Stdout)+string(run.Stderr), -1)
		var failed []string
		for _, m := range names {
			failed = append(failed, m[1])
		}
		for _, m := range messages(string(run.Stdout) + string(run.Stderr)) {
			t.Log(m)
		}
		t.Fatalf("no result file %s (run.sh exit %d); failed tests in the output: %v", resultFile, run.Code, failed)
	}
	var r platformResult
	if err := json.Unmarshal(res.Stdout, &r); err != nil {
		t.Fatalf("result file: %v", err)
	}
	var failed []string
	for _, x := range r.Tests {
		if !x.Passed {
			failed = append(failed, x.Name)
		}
	}
	slices.Sort(failed)
	t.Logf("platform %s: %d passed, %d failed, %d total", r.Platform, len(r.Tests)-len(failed), len(failed), len(r.Tests))
	if len(failed) > 0 {
		t.Errorf("failed platform tests: %s", strings.Join(failed, ", "))
		for _, m := range messages(string(run.Stdout) + string(run.Stderr)) {
			t.Log(m)
		}
	}
	if len(r.Tests) == 0 {
		t.Error("the result file lists no test")
	}
	if r.Platform != "linux" {
		t.Errorf("result of platform %q, want linux", r.Platform)
	}
	if run.Code != 0 && len(failed) == 0 {
		t.Errorf("run.sh suite exited %d although the result file lists no failed test", run.Code)
	}
}
