// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"io"
	"os"
	"path/filepath"

	"github.com/OWNER/bothy/services/host/internal/clock"
	"github.com/OWNER/bothy/services/host/internal/doctor"
	"github.com/OWNER/bothy/services/host/internal/helper"
	"github.com/OWNER/bothy/services/host/internal/hostos"
	"github.com/OWNER/bothy/services/host/internal/profiles"
	"github.com/OWNER/bothy/services/host/internal/pty"
)

// ptyParts prepares the PTYs of the helper (M2-45): the supervisor of the session trees (Linux:
// the delegated cgroup of the helper unit; macOS: the run file in the home of the session user,
// whose recorded trees it ends first) and the environment of the profiles. An error means no PTY
// may start, so the helper does not serve.
func ptyParts(self hostos.Account, env doctor.HelperEnv) (*pty.Supervisor, profiles.Env, error) {
	o := pty.Options{}
	if sys.goos == hostos.Darwin {
		o.RunDir = filepath.Join(self.Home, ".config", "bothy-host", "run")
		if err := helper.MkdirPrivate(o.RunDir); err != nil {
			return nil, profiles.Env{}, err
		}
	}
	sup, err := pty.New(o)
	if err != nil {
		return nil, profiles.Env{}, err
	}
	shells, block, err := profileExtras()
	if err != nil {
		_ = sup.Close()
		return nil, profiles.Env{}, err
	}
	return sup, profiles.Env{GOOS: sys.goos, Self: self, Environ: os.Environ(), Resolver: env.Resolver, Security: env.Security, Shells: shells,
		LoginEnv: env.LoginEnv, EnvBlock: block, SystemRoot: sys.getenv("SystemRoot")}, nil
}

// cmdPtyHelper is "pty-helper --session-user <name>", started by the service manager as that
// session user, never setuid (24 2.2 PTY helper points 1 to 4). On Windows it runs under the
// service control manager.
func cmdPtyHelper(ctx context.Context, args []string, _, stderr io.Writer) int {
	fl := newFlags("pty-helper", stderr)
	name := fl.String("session-user", "", "the session user this helper serves")
	if err := fl.Parse(args); err != nil || fl.NArg() != 0 || *name == "" {
		return exitUsage
	}
	body := func(ctx context.Context, _ bool) int { return servePtyHelper(ctx, *name, stderr) }
	if code, ok := serviceMode(body); ok {
		return code
	}
	return body(ctx, false)
}

func servePtyHelper(ctx context.Context, name string, stderr io.Writer) int {
	users, sec, probe := sys.platform(sys.runner)
	self, err := users.Lookup(name)
	if err != nil || self.ID != sys.callerID() {
		_, _ = io.WriteString(stderr, "bothy-host pty-helper: must run as its session user\n")
		return exitUsage
	}
	svc, err := users.Lookup(sys.serviceAccount())
	if err != nil {
		_, _ = io.WriteString(stderr, "bothy-host pty-helper: service user missing\n")
		return exitFailed
	}
	if err := helper.Protect(svc.ID); err != nil {
		_, _ = io.WriteString(stderr, "bothy-host pty-helper: memory protection failed\n")
		return exitFailed
	}
	l, err := helperListener(name, svc.ID)
	if err != nil {
		_, _ = io.WriteString(stderr, "bothy-host pty-helper: no channel endpoint\n")
		return exitFailed
	}
	env := doctor.HelperEnv{GOOS: sys.goos, Self: self, Users: users, Security: sec, Resolver: hostos.OSResolver{}, Files: hostos.OSLister{},
		Probe: probe, Runner: sys.runner, Clock: clock.Real(), LoginEnv: helper.LoginEnv(sys.runner, self), MkdirPrivate: helper.MkdirPrivate}
	sup, penv, err := ptyParts(self, env)
	if err != nil {
		_, _ = io.WriteString(stderr, "bothy-host pty-helper: no session tree\n")
		return exitFailed
	}
	defer func() { _ = sup.Close() }()
	sess := &helper.SessionEnv{Check: env, Profiles: penv, PTY: sup, Log: sys.log(stderr)}
	if err := helper.Serve(ctx, helper.Options{Listener: l, ServiceUID: svc.ID, Env: env, Session: sess}); err != nil {
		_, _ = io.WriteString(stderr, "bothy-host pty-helper: channel closed\n")
		return exitFailed
	}
	return exitOK
}
