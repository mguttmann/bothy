// SPDX-License-Identifier: Apache-2.0

// Package hostplatform runs the platform suite of the host service (services/host/test/platform,
// M2-42 and M2-45) in a Linux container inside make check (brief M2-48 Design note 11): it builds
// the Linux bundle with make -C services/host platform-bundle, runs it as root in a fresh host
// container of the terminal stack and fails when any platform test fails. The platform suite needs
// no server (it drives bothy-host against M2-43's controltest inside its own test binary); the
// package still starts the one stack of Design note 2, because the host containers are services
// of that Compose project.
package hostplatform

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/test/terminal/stack"
	"github.com/OWNER/bothy/services/server/test/terminal/suite"
)

// startWait bounds the start of the stack and the build of the bundle in TestMain (a test
// constant).
const startWait = 45 * time.Minute

// pkg is the state of the package: the stack and the folder of the Linux bundle.
var pkg struct {
	stack  *stack.Stack
	bundle string
}

// TestMain starts the stack of profile default, builds the Linux bundle for the architecture of
// the Docker host, runs the tests and removes the stack.
func TestMain(m *testing.M) { os.Exit(run(m)) }

// run is TestMain without the exit, so the deferred cancel runs; its result is the exit code.
func run(m *testing.M) int {
	ctx, cancel := context.WithTimeout(context.Background(), startWait)
	defer cancel()
	s, err := setup(ctx)
	rc := 1
	if err != nil {
		fmt.Fprintf(os.Stderr, "terminal suite hostplatform: %v\n", err)
	} else {
		rc = m.Run()
		if rc != 0 {
			s.Diagnose(context.WithoutCancel(ctx), os.Stderr)
		}
	}
	if s != nil {
		if derr := s.Down(context.WithoutCancel(ctx)); derr != nil {
			fmt.Fprintf(os.Stderr, "terminal suite hostplatform: removal: %v\n", derr)
			rc = 1
		}
		for k, v := range s.Timings {
			fmt.Fprintf(os.Stderr, "terminal suite hostplatform: %s %s\n", k, v.Round(time.Second))
		}
	}
	return rc
}

// setup starts the stack with the server keys and the front timeouts of profile default, as
// suite.Main computes them, and builds the bundle into the temporary folder of the stack. It
// returns the stack even on an error after the project exists, so TestMain can remove it.
func setup(ctx context.Context) (*stack.Stack, error) {
	repo, err := suite.FindRepo()
	if err != nil {
		return nil, err
	}
	profiles, err := suite.LoadProfiles(repo.ProfilesPath())
	if err != nil {
		return nil, err
	}
	prof, ok := profiles.Profiles[string(stack.ProfileDefault)]
	if !ok {
		return nil, fmt.Errorf("profiles.yaml has no profile %s", stack.ProfileDefault)
	}
	front, err := frontTimeouts(prof)
	if err != nil {
		return nil, err
	}
	s, err := stack.Start(ctx, stack.Options{Package: "hostplatform", Profile: stack.ProfileDefault, ServerKeys: prof.Server, Front: front,
		Logf: func(f string, a ...any) { fmt.Fprintf(os.Stderr, f+"\n", a...) }})
	if err != nil {
		return s, err
	}
	pkg.stack = s
	pkg.bundle = filepath.Join(s.Dir, "bundle")
	t0 := time.Now()
	if err := buildBundle(ctx, s.Root, pkg.bundle, s.Arch); err != nil {
		return s, err
	}
	s.Timings["platform_bundle"] = time.Since(t0)
	return s, nil
}

// frontTimeouts returns the values of the profile the TLS front computes its timeouts from (the
// same keys as suite.Main reads).
func frontTimeouts(p suite.Profile) (stack.FrontTimeouts, error) {
	var f stack.FrontTimeouts
	var err error
	if f.IdleTimeout, err = p.Duration("terminal.idle_timeout"); err != nil {
		return f, err
	}
	if f.IdleGrace, err = p.Duration("terminal.idle_grace"); err != nil {
		return f, err
	}
	if f.HeartbeatInterval, err = p.Duration("terminal.host_heartbeat_interval"); err != nil {
		return f, err
	}
	f.HeartbeatMissLimit, err = p.Int("terminal.host_heartbeat_miss_limit")
	return f, err
}

// buildBundle runs make -C services/host platform-bundle OUT=out GOOS=linux GOARCH=arch in root,
// so the container runs the files every other platform run gets (Design note 11).
//
// Errors: a failed build with the last part of its output (build messages, no data of a run).
func buildBundle(ctx context.Context, root, out, arch string) error {
	cmd := exec.CommandContext(ctx, "make", "-C", "services/host", "platform-bundle", "OUT="+out, "GOOS=linux", "GOARCH="+arch) //nolint:gosec // G204: fixed program, arguments of the suite
	cmd.Dir = root
	cmd.Env = os.Environ() //nolint:forbidigo // go needs PATH, HOME and the caches of the machine
	if b, err := cmd.CombinedOutput(); err != nil {
		if len(b) > 4096 {
			b = b[len(b)-4096:]
		}
		return fmt.Errorf("platform-bundle for linux/%s: %w\n%s", arch, err, b)
	}
	return nil
}
