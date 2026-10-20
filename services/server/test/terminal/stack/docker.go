// SPDX-License-Identifier: Apache-2.0

package stack

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
)

// Docker runs the docker command line of the machine that runs the tests. Every call passes the
// environment of the Docker struct, never the variables of the calling shell that would change a
// Compose project (Design note 3: BOTHY_, COMPOSE_, OTEL_, SMOKE_ and TERMSUITE_ are dropped).
type Docker struct {
	// Bin is the docker executable; empty means "docker" from PATH.
	Bin string
	// Env is the environment of every call; nil means Sanitized(os.Environ()).
	Env []string
	// Dir is the working directory of every call; empty means the one of the test process.
	Dir string
}

// result is the outcome of one docker call.
type result struct {
	stdout, stderr []byte
	code           int
}

// ErrDocker reports a docker call that exited with a code other than 0.
var ErrDocker = errors.New("stack: docker call failed")

// Sanitized returns env without the variables of the prefixes BOTHY_, COMPOSE_, OTEL_, SMOKE_
// and TERMSUITE_, as smoke.sh unsets them: a developer's shell must not change the stack.
//
// Example: Sanitized([]string{"PATH=/bin", "BOTHY_X=1"}) returns []string{"PATH=/bin"}.
func Sanitized(env []string) []string {
	out := make([]string, 0, len(env))
	for _, kv := range env {
		name, _, _ := strings.Cut(kv, "=")
		if hasAnyPrefix(name, "BOTHY_", "COMPOSE_", "OTEL_", "SMOKE_", "TERMSUITE_") {
			continue
		}
		out = append(out, kv)
	}
	return out
}

// hasAnyPrefix reports whether s starts with one of the prefixes.
func hasAnyPrefix(s string, prefixes ...string) bool {
	for _, p := range prefixes {
		if strings.HasPrefix(s, p) {
			return true
		}
	}
	return false
}

// bin returns the executable of d.
func (d *Docker) bin() string {
	if d.Bin != "" {
		return d.Bin
	}
	return "docker"
}

// env returns the environment of d.
func (d *Docker) env() []string {
	if d.Env != nil {
		return d.Env
	}
	return Sanitized(os.Environ()) //nolint:forbidigo // the docker CLI needs PATH, HOME and DOCKER_* of the machine; Sanitized drops the variables of a stack
}

// run calls docker with args and extra environment entries, stdin from in (nil for none). It
// returns stdout, stderr and the exit code; err is set only when docker could not run at all.
func (d *Docker) run(ctx context.Context, in io.Reader, extra []string, args ...string) (result, error) {
	cmd := exec.CommandContext(ctx, d.bin(), args...) //nolint:gosec // G204: the arguments are built by the suite, never from input
	cmd.Env = append(append([]string{}, d.env()...), extra...)
	cmd.Dir = d.Dir
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr, cmd.Stdin = &stdout, &stderr, in
	err := cmd.Run()
	r := result{stdout: stdout.Bytes(), stderr: stderr.Bytes()}
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		r.code = exit.ExitCode()
		return r, nil
	}
	if err != nil {
		return r, err
	}
	return r, nil
}

// must calls docker like run and turns a code other than 0 into an error that names the first
// argument and the last line of stderr (codes and messages of docker, never data of the suite).
func (d *Docker) must(ctx context.Context, extra []string, args ...string) ([]byte, error) {
	r, err := d.run(ctx, nil, extra, args...)
	if err != nil {
		return nil, fmt.Errorf("stack: docker %s: %w", firstArgs(args), err)
	}
	if r.code != 0 {
		return r.stdout, fmt.Errorf("%w: docker %s exited %d: %s", ErrDocker, firstArgs(args), r.code, lastLine(r.stderr))
	}
	return r.stdout, nil
}

// firstArgs returns the first two arguments of a docker call, for error messages.
func firstArgs(args []string) string {
	if len(args) > 2 {
		args = args[:2]
	}
	return strings.Join(args, " ")
}

// lastLine returns the last non-empty line of b, at most 300 bytes.
func lastLine(b []byte) string {
	lines := strings.Split(strings.TrimSpace(string(b)), "\n")
	l := strings.TrimSpace(lines[len(lines)-1])
	if len(l) > 300 {
		l = l[:300]
	}
	return l
}
