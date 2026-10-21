// SPDX-License-Identifier: Apache-2.0

package suite

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/test/harness"
	"github.com/OWNER/bothy/services/server/test/terminal/device"
	"github.com/OWNER/bothy/services/server/test/terminal/stack"
)

// startWait bounds the start of a stack in TestMain, image builds included (a test constant).
const startWait = 45 * time.Minute

// hostSlots is the number of host containers that start at once (a test constant).
const hostSlots = 6

// pkg is the state of the e2e package of the test binary.
var pkg struct {
	env      *Env
	code     string
	once     sync.Once
	external *harness.Server
}

// Main runs the tests of an e2e package against one stack of profile (Design note 2): it loads
// and checks the profiles, starts the stack, builds the binaries of the host containers and the
// sidecar, creates the bootstrap owner, runs m and removes the stack. Its result is the exit code
// of the test binary. A machine without the prerequisites fails (requirement 18).
func Main(m *testing.M, name, profile string) int {
	ctx, cancel := context.WithTimeout(context.Background(), startWait)
	defer cancel()
	env, s, err := setup(ctx, name, profile)
	rc := 1
	if err != nil {
		fmt.Fprintf(os.Stderr, "terminal suite %s: %v\n", name, err)
	} else {
		pkg.env = env
		rc = m.Run()
		if rc != 0 {
			s.Diagnose(context.WithoutCancel(ctx), os.Stderr)
		}
	}
	if s != nil {
		if derr := s.Down(context.WithoutCancel(ctx)); derr != nil {
			fmt.Fprintf(os.Stderr, "terminal suite %s: removal: %v\n", name, derr)
			rc = 1
		}
		for k, v := range s.Timings {
			fmt.Fprintf(os.Stderr, "terminal suite %s: %s %s\n", name, k, v.Round(time.Second))
		}
	}
	return rc
}

// setup prepares the Env of a package.
func setup(ctx context.Context, name, profile string) (*Env, *stack.Stack, error) {
	repo, err := FindRepo()
	if err != nil {
		return nil, nil, err
	}
	profiles, err := LoadProfiles(repo.ProfilesPath())
	if err != nil {
		return nil, nil, err
	}
	keys, err := ConfigKeys(repo.Root)
	if err != nil {
		return nil, nil, err
	}
	mode, err := MultiNodeClusterMode(repo.Root)
	if err != nil {
		return nil, nil, err
	}
	if probs := profiles.Check(keys, mode); len(probs) > 0 {
		return nil, nil, fmt.Errorf("profiles.yaml: %v", probs)
	}
	prof, ok := profiles.Profiles[profile]
	if !ok {
		return nil, nil, fmt.Errorf("no profile %s", profile)
	}
	ops, err := Operations(repo.Root)
	if err != nil {
		return nil, nil, err
	}
	front, err := frontTimeouts(prof)
	if err != nil {
		return nil, nil, err
	}
	s, err := stack.Start(ctx, stack.Options{Package: name, Profile: stack.Profile(profile), ServerKeys: prof.Server, Front: front,
		Logf: func(f string, a ...any) { fmt.Fprintf(os.Stderr, f+"\n", a...) }})
	if err != nil {
		return nil, s, err
	}
	bin := filepath.Join(s.Dir, "bin")
	bins, err := stack.BuildHostBinaries(ctx, repo.Root, bin, s.Arch)
	if err != nil {
		return nil, s, err
	}
	side, err := device.BuildSidecar(ctx, repo.Root, bin)
	if err != nil {
		return nil, s, err
	}
	code, err := s.BootstrapOwnerCode(ctx, "termsuite-owner", "Owner")
	if err != nil {
		return nil, s, err
	}
	pkg.code = code
	env := &Env{Stack: s, Bins: bins, Sidecar: side, Repo: repo, ProfileName: profile, Profile: prof, Ops: ops,
		HostSlots: make(chan struct{}, hostSlots),
		Dialer:    &device.Dialer{Host: stack.InstanceHost, Port: stack.InstancePort, Mapped: s.FrontPort, Roots: s.CA.Pool()}}
	return env, s, nil
}

// frontTimeouts returns the values of the profile the TLS front computes its timeouts from.
func frontTimeouts(p Profile) (stack.FrontTimeouts, error) {
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

// Get returns the Env of the package; the first call enrols the bootstrap owner with t
// (harness.External), so the owner is shared by every test of the package.
func Get(t *testing.T) *Env {
	t.Helper()
	if pkg.env == nil {
		t.Fatal("suite: Main did not run in TestMain")
	}
	pkg.once.Do(func() {
		pkg.external = harness.External(t, harness.ExternalOptions{PublicURL: stack.InstanceOrigin, HTTPClient: pkg.env.Dialer.Client(),
			OwnerEnrollmentCode: pkg.code})
	})
	if pkg.external == nil {
		t.Fatal("suite: the bootstrap owner could not enrol")
	}
	pkg.env.Server = pkg.external
	return pkg.env
}

// RunFile loads scenarios/<id>.yaml, validates it and runs its cases.
func RunFile(t *testing.T, id string) {
	t.Helper()
	env := Get(t)
	sc, err := LoadScenario(env.Repo.ScenarioPath(id))
	if err != nil {
		t.Fatal(err)
	}
	keys, err := ConfigKeys(env.Repo.Root)
	if err != nil {
		t.Fatal(err)
	}
	profiles, err := LoadProfiles(env.Repo.ProfilesPath())
	if err != nil {
		t.Fatal(err)
	}
	if probs := Validate(sc, Rules{Operations: env.Ops, ConfigKeys: keys, Profiles: profiles}); len(probs) > 0 {
		t.Fatalf("%s is not valid: %v", id, probs)
	}
	Run(t, env, sc)
}

// StartEnv starts the stack of profile for one test outside an e2e package (TestChaosTMT16 in
// test/chaos, whose TestMain belongs to the harness) and removes it in t.Cleanup.
func StartEnv(t *testing.T, name, profile string) *Env {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), startWait)
	t.Cleanup(cancel)
	env, s, err := setup(ctx, name, profile)
	if s != nil {
		t.Cleanup(func() {
			if t.Failed() {
				s.Diagnose(context.WithoutCancel(ctx), os.Stderr)
			}
			if err := s.Down(context.WithoutCancel(ctx)); err != nil {
				t.Errorf("terminal suite %s: removal: %v", name, err)
			}
		})
	}
	if err != nil {
		t.Fatalf("terminal suite %s: %v", name, err)
	}
	pkg.env = env
	return Get(t)
}

// RunScenario validates sc and runs its cases in env.
func RunScenario(t *testing.T, env *Env, sc *Scenario) {
	t.Helper()
	keys, err := ConfigKeys(env.Repo.Root)
	if err != nil {
		t.Fatal(err)
	}
	profiles, err := LoadProfiles(env.Repo.ProfilesPath())
	if err != nil {
		t.Fatal(err)
	}
	if probs := Validate(sc, Rules{Operations: env.Ops, ConfigKeys: keys, Profiles: profiles}); len(probs) > 0 {
		t.Fatalf("%s is not valid: %v", sc.ID, probs)
	}
	Run(t, env, sc)
}
