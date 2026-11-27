// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"log/slog"
	"os"
	"time"

	"github.com/OWNER/bothy/services/host/internal/buildinfo"
	"github.com/OWNER/bothy/services/host/internal/clock"
	"github.com/OWNER/bothy/services/host/internal/computer"
	"github.com/OWNER/bothy/services/host/internal/config"
	"github.com/OWNER/bothy/services/host/internal/confirm"
	"github.com/OWNER/bothy/services/host/internal/control"
	"github.com/OWNER/bothy/services/host/internal/doctor"
	"github.com/OWNER/bothy/services/host/internal/hostos"
	"github.com/OWNER/bothy/services/host/internal/noise"
	"github.com/OWNER/bothy/services/host/internal/session"
	"github.com/OWNER/bothy/services/host/internal/state"
)

// computerWiring holds the computer sessions of run (docs/spec/27-computer-use.md, C3): the helper
// channel and the manager. It exists only with host_service.computer_use.
type computerWiring struct {
	helper *computer.Helper
	mgr    *computer.Manager
	cl     *control.Client
}

// wireComputers returns the computer manager of a paired host with host_service.computer_use
// (docs/spec/27-computer-use.md); without the key, without pairing or without the endpoint it is
// RefuseComputers: nothing listens and the report says helper absent.
func wireComputers(ctx context.Context, c *config.Config, st *state.Dir, log *slog.Logger, env doctor.Env, paired bool) (control.Computers, *computerWiring) {
	refuse := control.RefuseComputers{Signature: buildSignature()}
	if !c.ComputerUse || !paired {
		return refuse, nil
	}
	cw, err := newComputer(c, st, log, env)
	if err != nil {
		log.WarnContext(ctx, "computer sessions unavailable")
		return refuse, nil
	}
	return cw.mgr, cw
}

// buildSignature returns the signature state of this build: verified with a code signing
// requirement for the helper, development without one (D-CU-01-22).
func buildSignature() string {
	if buildinfo.ComputerHelperRequirement != "" {
		return control.SignatureVerified
	}
	return control.SignatureDevelopment
}

// newComputer opens the helper endpoint and builds the manager of a paired host; bind sets the
// control client it dials and reports through. Errors: the keys or the pairing record cannot be
// read, or the endpoint cannot be opened (its folder is missing or taken).
func newComputer(c *config.Config, st *state.Dir, log *slog.Logger, env doctor.Env) (*computerWiring, error) {
	keys, err := st.ReadKeys()
	if err != nil {
		return nil, err
	}
	nk, err := noise.KeyPairFromPrivate(keys.NoisePrivate)
	if err != nil {
		return nil, err
	}
	p, err := st.ReadPairing()
	if err != nil {
		return nil, err
	}
	l, err := confirm.Listen(computer.Endpoint(sys.goos))
	if err != nil {
		return nil, err
	}
	cw := &computerWiring{}
	cw.helper = computer.NewHelper(computer.HelperOptions{Listener: l, PeerOf: confirm.PeerOf, Bind: computer.DefaultBind, Rule: peerRule(c, st, env),
		MaxBytes: c.ComputerFrameMaxBytes, Log: log, OnEvent: func(e computer.Event) {
			if cw.mgr != nil {
				cw.mgr.HelperEvent(e)
			}
		}})
	cw.mgr = computer.NewManager(computer.Config{HostID: p.HostID, NoiseKey: nk, HostVersion: buildinfo.Version,
		Platform: buildinfo.Platform(), Signature: buildSignature(), Helper: cw.helper, Clock: clock.Real(), Log: log,
		Dial: func(ctx context.Context, path, sid string) (computer.Stream, error) {
			url, err := cw.cl.DataURL(path)
			if err != nil {
				return nil, err
			}
			return session.DialStream(ctx, cw.cl.HTTPClient(), url, session.StreamIdentify{HostID: p.HostID, HostVersion: buildinfo.Version, SessionID: sid},
				func(ch string) string { return control.Proof(keys.AuthPrivate, p.HostID, ch, time.Now()) })
		},
		OnReport: func() {
			if cw.cl != nil {
				_ = cw.cl.ComputerChanged(context.Background())
			}
		}})
	return cw, nil
}

// peerRule returns the peer rule of 27 9.4 of this host: the console-user rule of the
// confirmation agent, the installed helper path with the rule of HD10, the signing requirement of
// the build, and on Linux the test peer of host_service.computer_test_peer.
func peerRule(c *config.Config, st *state.Dir, env doctor.Env) computer.PeerRule {
	svc, _ := env.Users.Lookup(env.ServiceUser)
	u := sessionAccounts{c: c, st: st, users: env.Users}
	r := computer.PeerRule{GOOS: sys.goos, Console: confirm.DefaultRule(svc.ID, u.ids),
		HelperPath:  computer.HelperPath(sys.goos, sys.getenv("ProgramFiles")),
		Trusted:     computer.TrustedPath(sys.goos, env.Resolver, env.Security, u.writers),
		Requirement: buildinfo.ComputerHelperRequirement, VerifySignature: computer.DefaultVerifySignature}
	if sys.goos == hostos.Linux && c.ComputerTestPeer != "" {
		if a, err := env.Users.Lookup(c.ComputerTestPeer); err == nil {
			r.TestPeerUID = a.ID
		}
	}
	return r
}

// sessionAccounts are the configured and former session users (bindings.json), read at every call
// so a change applies at once.
type sessionAccounts struct {
	c     *config.Config
	st    *state.Dir
	users hostos.Users
}

// accounts returns the accounts of the session users that exist.
func (u sessionAccounts) accounts() []hostos.Account {
	names := append([]string{}, u.c.SessionUsers...)
	if b, err := u.st.ReadBindings(); err == nil {
		for n := range b {
			names = append(names, n)
		}
	}
	var out []hostos.Account
	for _, n := range names {
		if a, err := u.users.Lookup(n); err == nil {
			out = append(out, a)
		}
	}
	return out
}

// ids returns the uids or SIDs of the session users (confirm.Rule.Excluded).
func (u sessionAccounts) ids() []string {
	var ids []string
	for _, a := range u.accounts() {
		ids = append(ids, a.ID)
	}
	return ids
}

// writers returns the session users and everyone: the subjects that must not be able to write
// the helper path (HD10).
func (u sessionAccounts) writers() []hostos.Subject {
	var out []hostos.Subject
	for _, a := range u.accounts() {
		out = append(out, hostos.Subject{Name: a.Name, ID: a.ID})
	}
	return append(out, hostos.Subject{Name: "everyone", ID: "everyone"})
}

// bind sets the control client the manager reports through; a nil wiring (no computer use) does
// nothing.
func (cw *computerWiring) bind(cl *control.Client) {
	if cw != nil {
		cw.cl = cl
	}
}

// start serves the helper endpoint until ctx ends and returns the function that ends every
// computer session when the service stops; a nil wiring returns a function that does nothing.
func (cw *computerWiring) start(ctx context.Context) func() {
	if cw == nil {
		return func() {
			// No computer use: nothing to end.
		}
	}
	go func() { _ = cw.helper.Serve(ctx) }()
	return cw.mgr.Shutdown
}

// computerDoctor returns the section computer of bothy-host doctor (27 9.3) for c, with the state
// of the running helper when cw is set.
func computerDoctor(c *config.Config, env doctor.Env, cw *computerWiring) func() string {
	return func() string {
		s := doctor.ComputerSection{On: c.ComputerUse, TestPeer: c.ComputerTestPeer != "", Signed: buildinfo.ComputerHelperRequirement != "",
			Helper: control.HelperAbsent, Capture: control.PermUnknown, Input: control.PermUnknown}
		if path := computer.HelperPath(sys.goos, sys.getenv("ProgramFiles")); path != "" {
			if _, err := os.Stat(path); err == nil {
				s.Installed = true
				s.TrustedPath = computer.TrustedPath(sys.goos, env.Resolver, env.Security, nil)(path)
			}
		}
		if cw != nil {
			r := cw.mgr.Report()
			s.Helper, s.Enabled, s.Generation, s.Capture, s.Input = r.Helper, r.Enabled, r.Generation, r.Permissions.Capture, r.Permissions.Input
		}
		return s.Format()
	}
}
