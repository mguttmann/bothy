// SPDX-License-Identifier: Apache-2.0

package hostos

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"time"
)

// waitDelay bounds how long Run waits for the output pipes after the program ended or was
// killed (a protocol constant of the runner, not a product window).
const waitDelay = time.Second

// ExecRunner is the Runner of the operating system.
type ExecRunner struct{}

// ErrRelativeProgram is returned for a Cmd whose Path is not absolute.
var ErrRelativeProgram = errors.New("hostos: program path must be absolute")

// Run implements Runner. The environment is exactly c.Env (never the caller's), so a program
// started for a check sees no variable it was not given.
func (ExecRunner) Run(ctx context.Context, c Cmd) (Output, error) {
	if !filepath.IsAbs(c.Path) {
		return Output{}, ErrRelativeProgram
	}
	if c.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.Timeout)
		defer cancel()
	}
	cmd := exec.CommandContext(ctx, c.Path, c.Args...) //nolint:gosec // absolute path from the fixed lists or the checked configuration
	cmd.Env = c.Env
	if cmd.Env == nil {
		cmd.Env = []string{}
	}
	cmd.Dir = c.Dir
	// A child that keeps the output open must not stretch the bound: the whole process group ends
	// with the run, and Wait stops waiting for the pipes shortly after.
	cmd.WaitDelay = waitDelay
	ownGroup(cmd)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if c.Stdin != nil {
		cmd.Stdin = bytes.NewReader(c.Stdin)
	}
	err := cmd.Run()
	out := Output{Stdout: stdout.Bytes(), Stderr: stderr.Bytes()}
	if ctx.Err() != nil {
		out.TimedOut = true
		out.ExitCode = -1
		return out, nil
	}
	var exit *exec.ExitError
	switch {
	case err == nil:
	case errors.As(err, &exit):
		out.ExitCode = exit.ExitCode()
	default:
		return out, fmt.Errorf("run %s: %w", filepath.Base(c.Path), err)
	}
	return out, nil
}
