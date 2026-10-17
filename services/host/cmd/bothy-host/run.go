// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"os/signal"
	"syscall"

	"github.com/OWNER/bothy/services/host/internal/buildinfo"
	"github.com/OWNER/bothy/services/host/internal/config"
	"github.com/OWNER/bothy/services/host/internal/startup"
	"github.com/OWNER/bothy/services/host/internal/state"
)

// Log attribute keys of run: codes and configuration keys, never values (24 2.2).
const (
	attrCode = "code"
	attrKey  = "key"
)

// cmdRun is "run [--config <path>]", started by the service manager as the service user. An
// invalid configuration logs config_invalid with the key and exits 78, so no service manager
// loops on it (D-M2-42-3).
func cmdRun(ctx context.Context, args []string, _, stderr io.Writer) int {
	fl := newFlags("run", stderr)
	cfgPath := fl.String("config", "", "path of the configuration file")
	once := fl.Bool("doctor-once", false, "Windows: one doctor run for a waiting doctor command, then exit")
	if err := fl.Parse(args); err != nil || fl.NArg() != 0 {
		return exitUsage
	}
	// On Windows everything runs under the service control manager, so a refused start (also an
	// invalid configuration) stops with the service-specific code 78, not like a crash.
	run := func(ctx context.Context, svcOnce bool) int {
		return runService(ctx, *cfgPath, *once || svcOnce, stderr)
	}
	if code, ok := serviceMode(run); ok {
		return code
	}
	return run(ctx, false)
}

// runService is the body of run: configuration, state, admin endpoint, then startup.Run or
// startup.DoctorOnce.
func runService(ctx context.Context, cfgPath string, once bool, stderr io.Writer) int {
	log := sys.log(stderr)
	c, err := sys.loadConfig(cfgPath)
	if err != nil {
		var ke *config.KeyError
		attrs := []slog.Attr{slog.String(attrCode, "config_invalid")}
		if errors.As(err, &ke) {
			attrs = append(attrs, slog.String(attrKey, ke.Key))
		}
		log.LogAttrs(ctx, slog.LevelError, "start refused", attrs...)
		return exitRefused
	}
	ctx, stop := signal.NotifyContext(ctx, syscall.SIGTERM, syscall.SIGINT)
	defer stop()
	st := state.Open(c.StateDir, sys.protector)
	admin, err := sys.listenAdmin(st.File(state.AdminSocket))
	if err != nil {
		log.ErrorContext(ctx, "admin endpoint unavailable")
		return exitFailed
	}
	defer func() { _ = admin.Close() }()
	d := startup.Deps{Config: c, State: st, Doctor: sys.doctorEnv(c, st), Control: startup.NoControl{}, Log: log, Admin: admin,
		AdminUID: adminUID(), OSVersion: sys.osVersion(), Platform: buildinfo.Platform(), HostVersion: buildinfo.Version}
	if once {
		return startup.DoctorOnce(ctx, d)
	}
	return startup.Run(ctx, d)
}
