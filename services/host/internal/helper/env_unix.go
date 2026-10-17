// SPDX-License-Identifier: Apache-2.0

//go:build linux || darwin

package helper

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/OWNER/bothy/services/host/internal/hostos"
)

// loginProbe prints only whether CLAUDE_CODE_OAUTH_TOKEN is set, never its value, and the folder
// in CLAUDE_CONFIG_DIR.
const loginProbe = `if [ -n "${CLAUDE_CODE_OAUTH_TOKEN+x}" ]; then echo token=1; else echo token=0; fi; printf 'dir=%s\n' "${CLAUDE_CONFIG_DIR-}"`

// LoginEnv returns the probe of HD6 for the login shell of a: the shell runs as a login shell with
// a minimal environment and prints only whether CLAUDE_CODE_OAUTH_TOKEN is set and the folder in
// CLAUDE_CONFIG_DIR.
func LoginEnv(r hostos.Runner, a hostos.Account) func(context.Context, time.Duration) (bool, string, error) {
	return func(ctx context.Context, timeout time.Duration) (bool, string, error) {
		shell := a.Shell
		if !filepath.IsAbs(shell) {
			shell = "/bin/sh"
		}
		out, err := r.Run(ctx, hostos.Cmd{Path: shell, Args: []string{"-l", "-c", loginProbe}, Dir: a.Home, Timeout: timeout,
			Env: []string{"HOME=" + a.Home, "USER=" + a.Name, "LOGNAME=" + a.Name, "SHELL=" + shell, "PATH=/usr/bin:/bin"}})
		if err != nil || out.TimedOut || out.ExitCode != 0 {
			return false, "", errors.New("helper: login shell probe failed")
		}
		return parseProbe(string(out.Stdout))
	}
}

// parseProbe reads the last token= and dir= lines of the probe; startup files may print before.
func parseProbe(out string) (bool, string, error) {
	token, dir, seen := false, "", false
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		switch {
		case line == "token=1", line == "token=0":
			token, seen = line == "token=1", true
		case strings.HasPrefix(line, "dir="):
			dir = strings.TrimPrefix(line, "dir=")
		}
	}
	if !seen {
		return false, "", errors.New("helper: login shell probe unreadable")
	}
	return token, dir, nil
}

// MkdirPrivate creates path with mode 0700 for every created level.
func MkdirPrivate(path string) error { return os.MkdirAll(path, 0o700) }
