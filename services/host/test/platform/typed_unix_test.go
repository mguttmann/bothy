// SPDX-License-Identifier: Apache-2.0

//go:build linux || darwin

package platform

import (
	"context"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/host/internal/platform/testkit"
)

// TestTypedCommandsInInteractiveShells: every command the suite types into a PTY prints what its
// expression waits for in an interactive zsh and bash. Interactive zsh expands history: "!"
// followed by a double quote turns that off for the rest of the line and is removed, so
// `echo "BG=$!"` left an open quote and the shell of a macOS session user waited for more input
// (run 46103059338, docs/decisions.d/V1-13.md). The shells read the commands from standard input,
// which keeps them interactive with -i.
func TestTypedCommandsInInteractiveShells(t *testing.T) {
	cases := []struct {
		cmd string
		re  *regexp.Regexp
		bg  bool
	}{
		{identityCmd, identityRe, false},
		{parentCmd, parentRe, false},
		{shellPIDCmd, shellPIDRe, false},
	}
	for i, c := range backgroundCmds {
		cases = append(cases, struct {
			cmd string
			re  *regexp.Regexp
			bg  bool
		}{c, backgroundRes[i], true})
	}
	_, perlErr := exec.LookPath("perl")
	for _, sh := range [][]string{{"/bin/zsh", "-f", "-i"}, {"/bin/bash", "--norc", "--noprofile", "-i"}} {
		if _, err := os.Stat(sh[0]); err != nil {
			continue
		}
		for _, c := range cases {
			if strings.HasPrefix(c.cmd, "perl ") && perlErr != nil {
				continue
			}
			ctx, cancel := context.WithDeadline(context.Background(), testkit.Deadline())
			cmd := exec.CommandContext(ctx, sh[0], sh[1:]...) //nolint:gosec // fixed shells of the test
			cmd.Env = []string{"PATH=/usr/bin:/bin", "HOME=" + t.TempDir(), "TERM=dumb"}
			cmd.Stdin = strings.NewReader(c.cmd + enter + "exit" + enter)
			// The setsid child keeps the output open after the shell ended; stop reading then.
			cmd.WaitDelay = 2 * time.Second
			out, _ := cmd.CombinedOutput()
			cancel()
			// A match is a number; 0 is a valid user ID (the suite runs as root), a process ID of a
			// background process is never 0.
			m := c.re.FindSubmatch(out)
			if m == nil {
				t.Errorf("%s: %s printed no match of %s:\n%s", sh[0], c.cmd, c.re, out)
				continue
			}
			n, _ := strconv.Atoi(string(m[1]))
			if c.bg && n > 0 {
				_ = syscall.Kill(n, syscall.SIGKILL)
			}
			if c.bg && n <= 0 {
				t.Errorf("%s: %s printed the process ID %d", sh[0], c.cmd, n)
			}
		}
	}
}
