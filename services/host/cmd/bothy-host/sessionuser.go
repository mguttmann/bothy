// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/OWNER/bothy/services/host/internal/install"
	"github.com/OWNER/bothy/services/host/internal/state"
)

// cmdSessionUser is "session-user add <name>" and "session-user remove <name>" (D-M2-42-13).
func cmdSessionUser(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	fl := newFlags("session-user", stderr)
	cfgPath := fl.String("config", "", "path of the configuration file")
	if err := fl.Parse(args); err != nil || fl.NArg() != 2 || (fl.Arg(0) != "add" && fl.Arg(0) != "remove") {
		_, _ = fmt.Fprintln(stderr, "usage: bothy-host session-user add|remove <name>")
		return exitUsage
	}
	if !sys.isAdmin() {
		return adminRequired(stderr, "session-user")
	}
	c, err := sys.loadConfig(*cfgPath)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "bothy-host session-user: %v\n", err)
		return exitUsage
	}
	tg, err := sys.target(c, true)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "bothy-host session-user: %v\n", err)
		return exitFailed
	}
	name := fl.Arg(1)
	if fl.Arg(0) == "remove" {
		steps, err := install.SessionUserRemove(tg, name, c.SessionUsers)
		if err != nil {
			_, _ = fmt.Fprintln(stderr, err)
			return exitUsage
		}
		if code := applyOrFail(ctx, stderr, "session-user", steps); code != 0 {
			return code
		}
		_, _ = fmt.Fprint(stdout, install.OperatorStepsAfterRemove(name))
		return exitOK
	}
	bindings, err := state.Open(c.StateDir, sys.protector).ReadBindings()
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "bothy-host session-user: %v\n", err)
		return exitFailed
	}
	steps, err := install.SessionUserAdd(tg, name, c.SessionUsers, bindings)
	var bound *install.BoundError
	if errors.As(err, &bound) {
		_, _ = fmt.Fprintln(stderr, err)
		return exitFailed
	}
	if err != nil {
		_, _ = fmt.Fprintln(stderr, err)
		return exitUsage
	}
	if code := applyOrFail(ctx, stderr, "session-user", steps); code != 0 {
		return code
	}
	_, _ = fmt.Fprintf(stdout, "session-user add: %s created; the host service restarted\n", name)
	return exitOK
}

// cmdRestart is "restart": the host service only, so a changed configuration takes effect.
func cmdRestart(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	fl := newFlags("restart", stderr)
	if err := fl.Parse(args); err != nil || fl.NArg() != 0 {
		return exitUsage
	}
	if !sys.isAdmin() {
		return adminRequired(stderr, "restart")
	}
	steps, err := install.Restart(install.Target{GOOS: sys.goos})
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "bothy-host restart: %v\n", err)
		return exitUsage
	}
	if code := applyOrFail(ctx, stderr, "restart", steps); code != 0 {
		return code
	}
	_, _ = fmt.Fprintln(stdout, "restart: done")
	return exitOK
}
