// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"io"

	"github.com/OWNER/bothy/services/host/internal/clock"
	"github.com/OWNER/bothy/services/host/internal/doctor"
	"github.com/OWNER/bothy/services/host/internal/helper"
	"github.com/OWNER/bothy/services/host/internal/hostos"
)

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
	if err := helper.Serve(ctx, helper.Options{Listener: l, ServiceUID: svc.ID, Env: env}); err != nil {
		_, _ = io.WriteString(stderr, "bothy-host pty-helper: channel closed\n")
		return exitFailed
	}
	return exitOK
}
