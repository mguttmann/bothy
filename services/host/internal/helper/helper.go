// SPDX-License-Identifier: Apache-2.0

// Package helper is the process "bothy-host pty-helper": one per session user, started by the
// service manager under that user (docs/spec/24-terminal.md 2.2, PTY helper points 1 to 4). It
// protects its memory, serves its local channel to the service user only, and runs the
// helper-side checks of doctor as its session user: HD3, HD6, the helper part of HD8, HD9, the
// places of HD11 with the entry scan, and the identities of the working directories for HD7.
// Results carry codes, names and identities, never paths or contents. M2-45 adds the PTYs: the
// session ops pty.start, pty.resize and pty.stop with the frames of kind 0x01 (ServeSessions).
package helper

import (
	"context"
	"encoding/json"
	"net"

	"github.com/OWNER/bothy/services/host/internal/doctor"
	"github.com/OWNER/bothy/services/host/internal/localchan"
)

// Options configure Serve.
type Options struct {
	// Listener is the endpoint: the socket of the service manager (Linux, macOS) or the pipe the
	// helper created (Windows).
	Listener net.Listener
	// ServiceUID is the uid or SID of the service user, the only client the helper accepts.
	ServiceUID string
	// Env is the operating system as the session user sees it.
	Env doctor.HelperEnv
	// Session serves the session ops and PTYs of M2-45; nil answers the checks only.
	Session *SessionEnv
}

// Serve answers the service on its channels until ctx ends. With o.Session every channel is a
// session channel (ServeSessions): the PTYs started on it end with their trees when it breaks.
func Serve(ctx context.Context, o Options) error {
	if o.Session == nil {
		return localchan.ServeHelper(ctx, o.Listener, o.ServiceUID, Handler(o.Env))
	}
	st := &State{}
	return localchan.ServeHelperConns(ctx, o.Listener, o.ServiceUID, func(ctx context.Context, c *localchan.Conn) {
		ServeSessions(ctx, c, *o.Session, st)
	})
}

// Handler returns the handler of the ops hello, check.run and scan.run. Any other op, or a body
// that does not decode, closes the channel.
func Handler(env doctor.HelperEnv) localchan.Handler {
	return func(ctx context.Context, m localchan.Message) (string, any) {
		switch m.Op {
		case localchan.OpHello:
			return localchan.OpHello, map[string]any{"version": localchan.ProtocolVersion, "session_user": env.Self.Name}
		case localchan.OpCheckRun, localchan.OpScanRun:
			var req doctor.Request
			if err := json.Unmarshal(m.D, &req); err != nil || req.SessionUser != env.Self.Name {
				return "", nil
			}
			if m.Op == localchan.OpCheckRun {
				return localchan.OpCheckResult, doctor.RunHelper(ctx, env, req)
			}
			return localchan.OpScanResult, map[string]string{"code": doctor.ScanEntries(ctx, env, req)}
		}
		return "", nil
	}
}
