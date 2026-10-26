// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"crypto/x509"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"os/signal"
	"syscall"

	"github.com/OWNER/bothy/services/host/internal/buildinfo"
	"github.com/OWNER/bothy/services/host/internal/config"
	"github.com/OWNER/bothy/services/host/internal/control"
	"github.com/OWNER/bothy/services/host/internal/profiles"
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

// controlFor returns the control connection of a paired host (M2-43): the client of
// internal/control over the pairing record and the login key, with the session manager of M2-45
// (sessions). An unpaired host, or one whose state cannot be read, gets
// startup.NoControl, so the stored declaration decides (24 2.2). It never logs a key.
func controlFor(c *config.Config, st *state.Dir, log *slog.Logger, sessions control.Sessions) startup.Control {
	p, err := st.ReadPairing()
	if err != nil {
		return startup.NoControl{}
	}
	keys, err := st.ReadKeys()
	if err != nil {
		log.Warn("login key unreadable")
		return startup.NoControl{}
	}
	roots, err := serverRoots(st)
	if err != nil {
		log.Warn("server CA unreadable")
		return startup.NoControl{}
	}
	cl, err := control.New(control.Config{ServerURL: p.ServerURL, HostID: p.HostID, AuthKey: keys.AuthPrivate, HostVersion: buildinfo.Version,
		UpdatePath: buildinfo.UpdatePath(buildinfo.PackageKind, c.Path), BackoffBase: c.ReconnectBackoffBase, BackoffCap: c.ReconnectBackoffCap, Log: log, Proxy: http.ProxyFromEnvironment, RootCAs: roots},
		sessions)
	if err != nil {
		log.Warn("control connection unavailable")
		return startup.NoControl{}
	}
	return cl
}

// serverRoots returns the system roots plus the CA that `pair --ca-file` stored (M2-43), or nil
// (the system roots) without one. Errors: an unreadable or malformed stored CA.
func serverRoots(st *state.Dir) (*x509.CertPool, error) {
	ca, err := st.ReadServerCA()
	if err != nil || ca == nil {
		return nil, err
	}
	roots, err := x509.SystemCertPool()
	if err != nil {
		roots = x509.NewCertPool()
	}
	if !roots.AppendCertsFromPEM(ca) {
		return nil, errors.New("run: the stored server CA is no PEM certificate")
	}
	return roots, nil
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
	denv := sys.doctorEnv(c, st)
	d := startup.Deps{Config: c, State: st, Doctor: denv, Log: log, Admin: admin, AdminUID: adminUID(), OSVersion: sys.osVersion(),
		Platform: buildinfo.Platform(), HostVersion: buildinfo.Version, Offered: profiles.Offered}
	if once {
		d.Control = startup.NoControl{}
		return startup.DoctorOnce(ctx, d)
	}
	// The session manager of M2-45 for a paired host; without keys or pairing every session.open
	// is refused (RefuseSessions).
	var sessions control.Sessions = control.RefuseSessions{}
	w, err := newSessions(c, st, log, denv)
	if err == nil {
		sessions = w.mgr
	} else if !errors.Is(err, state.ErrNotPaired) {
		log.WarnContext(ctx, "terminal sessions unavailable")
	}
	d.Control = controlFor(c, st, log, sessions)
	cl, ok := d.Control.(*control.Client)
	switch {
	case w != nil && !ok:
		w = nil // no control connection: no session.open can arrive
	case w != nil:
		w.bind(cl)
	}
	svc, code := startup.Start(ctx, d)
	if svc == nil {
		return code
	}
	if w != nil {
		w.start(ctx, svc)
		defer w.mgr.Shutdown()
	}
	return svc.Serve(ctx)
}
