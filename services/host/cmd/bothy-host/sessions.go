// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/OWNER/bothy/services/host/internal/buildinfo"
	"github.com/OWNER/bothy/services/host/internal/clock"
	"github.com/OWNER/bothy/services/host/internal/config"
	"github.com/OWNER/bothy/services/host/internal/confirm"
	"github.com/OWNER/bothy/services/host/internal/control"
	"github.com/OWNER/bothy/services/host/internal/doctor"
	"github.com/OWNER/bothy/services/host/internal/noise"
	"github.com/OWNER/bothy/services/host/internal/profiles"
	"github.com/OWNER/bothy/services/host/internal/session"
	"github.com/OWNER/bothy/services/host/internal/startup"
	"github.com/OWNER/bothy/services/host/internal/state"
)

// sessionWiring holds the session manager of run with the parts it needs after the start.
type sessionWiring struct {
	mgr     *session.Manager
	gate    *session.Gate
	confirm *confirm.Server
	svc     *startup.Service
	cl      *control.Client
	runner  *session.LazyRunner
}

// newSessions builds the session manager of a paired host (M2-45) over the control client cl; the
// gate and the reports bind to the service once startup.Start returned it (bind). It reads the
// keys and the pairing record; a host without them gets no manager.
func newSessions(c *config.Config, st *state.Dir, log *slog.Logger, doctorEnv doctor.Env) (*sessionWiring, error) {
	keys, err := st.ReadKeys()
	if err != nil {
		return nil, err
	}
	p, err := st.ReadPairing()
	if err != nil {
		return nil, err
	}
	nk, err := noise.KeyPairFromPrivate(keys.NoisePrivate)
	if err != nil {
		return nil, err
	}
	w := &sessionWiring{runner: &session.LazyRunner{}}
	w.gate = session.NewGate(w.runner, clock.Real(), c.CredentialCheckInterval,
		func(ctx context.Context, res *doctor.Result) {
			if w.svc != nil {
				w.svc.Publish(ctx, res)
			}
		}, nil)
	helpers, _ := doctorEnv.Helpers.(startup.ChannelHelpers)
	d := session.Deps{HostID: p.HostID, NoiseKey: nk, AuthKey: keys.AuthPrivate, HostVersion: buildinfo.Version,
		HTTP:    func() *http.Client { return w.cl.HTTPClient() },
		DataURL: func(path string) (string, error) { return w.cl.DataURL(path) },
		Config:  c, Bindings: st,
		APIKey:  func() ([]byte, error) { return os.ReadFile(st.File(state.ClaudeAPIKeyFile)) },
		Gate:    w.gate,
		Result:  w.runner.Last,
		Offered: profiles.Offered,
		ServerHost: func() bool {
			res := w.gate.Last()
			return (res != nil && res.IsServerHost) || w.cl.PeerLocal()
		},
		Dial: helpers.Open, Clock: clock.Real(), Log: log}
	if c.RequireLocalConfirm {
		srv, err := newConfirmServer(doctorEnv, st, log)
		if err != nil {
			return nil, err
		}
		w.confirm = srv
		d.Confirm = func(ctx context.Context, kind, profile, osu string, timeout time.Duration) bool {
			return srv.Ask(ctx, confirm.Request{Kind: kind, Profile: profile, SessionUser: osu}, timeout)
		}
	}
	w.mgr = session.New(d)
	return w, nil
}

// bind sets the control client; the manager exists first, because control.New takes it, and no
// session.open arrives before the client is bound.
func (w *sessionWiring) bind(cl *control.Client) { w.cl = cl }

// newConfirmServer opens the confirmation endpoint (Design note 15): the peer must be neither the
// service user nor a configured or former session user and must own the console session.
func newConfirmServer(env doctor.Env, st *state.Dir, log *slog.Logger) (*confirm.Server, error) {
	l, err := confirm.Listen(confirm.Endpoint(sys.goos))
	if err != nil {
		return nil, err
	}
	svc, _ := env.Users.Lookup(env.ServiceUser)
	excluded := func() []string {
		var ids []string
		names := append([]string{}, env.Config.SessionUsers...)
		if b, err := st.ReadBindings(); err == nil {
			for n := range b {
				names = append(names, n)
			}
		}
		for _, n := range names {
			if a, err := env.Users.Lookup(n); err == nil {
				ids = append(ids, a.ID)
			}
		}
		return ids
	}
	return confirm.NewServer(confirm.ServerOptions{Listener: l, Log: log, PeerOf: confirm.PeerOf,
		Rule: confirm.DefaultRule(svc.ID, excluded)}), nil
}

// start binds the started service and runs the periodic checks and the confirmation endpoint
// until ctx ends.
func (w *sessionWiring) start(ctx context.Context, svc *startup.Service) {
	w.svc = svc
	w.runner.Set(svc.Runner())
	go w.gate.Loop(ctx)
	if w.confirm != nil {
		go func() { _ = w.confirm.Serve(ctx) }()
	}
}
