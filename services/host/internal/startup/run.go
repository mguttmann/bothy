// SPDX-License-Identifier: Apache-2.0

package startup

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"log/slog"
	"net"
	"sync"

	"github.com/OWNER/bothy/services/host/internal/config"
	"github.com/OWNER/bothy/services/host/internal/doctor"
	"github.com/OWNER/bothy/services/host/internal/localchan"
	"github.com/OWNER/bothy/services/host/internal/pairing"
	"github.com/OWNER/bothy/services/host/internal/state"
)

// Log attribute keys of the host service: codes and names only, never paths or values (24 2.2).
const (
	AttrCheck       = "check"
	AttrCode        = "code"
	AttrOK          = "ok"
	AttrSessionUser = "session_user"
)

// Deps are the parts of run; the configuration is read once by the caller, at start only.
type Deps struct {
	Config *config.Config
	State  *state.Dir
	// Doctor is the environment of the checks; Declaration, Bindings and ServerURL come from the
	// state directory.
	Doctor  doctor.Env
	Control Control
	Log     *slog.Logger
	// Admin is the open admin endpoint; AdminUID the uid of the allowed clients ("0").
	Admin    net.Listener
	AdminUID string
	// OSVersion is reported as os_version; Platform and HostVersion go into pairHost.
	OSVersion, Platform, HostVersion string
	// Pair is pairing.Pair; tests replace it.
	Pair func(ctx context.Context, o pairing.Options) (pairing.Result, error)
}

// PairRequest is the op pair.run of the admin endpoint: an administrator pairs the running
// service, which writes the keys in the context of the service user (D-M2-42-1, -16).
type PairRequest struct {
	ServerURL string `json:"server_url"`
	Code      string `json:"code"`
	CAPEM     []byte `json:"ca_pem,omitempty"`
}

// PairAnswer is the answer of pair.run: the result, or the error line of pair (never the code or
// a key).
type PairAnswer struct {
	Fingerprint string `json:"fingerprint,omitempty"`
	HostID      string `json:"host_id,omitempty"`
	Error       string `json:"error,omitempty"`
}

// Service is a running host service.
type Service struct {
	d        Deps
	runner   *doctor.Runner
	mu       sync.Mutex
	loggedIn bool
	// started is set once the start is not refused; only then does a finished entry scan send a
	// report, so a refused start sends exactly one.
	started bool
	end     chan int
	once    sync.Once
}

// DoctorAnswer is the answer of the op doctor.run of the admin endpoint.
type DoctorAnswer struct {
	// Output is the local output of doctor (Format), Exit its exit code.
	Output string `json:"output"`
	Exit   int    `json:"exit"`
	// Doctor are the report entries, which the platform suite reads directly.
	Doctor []doctor.ReportEntry `json:"doctor"`
}

func (s *Service) report(ctx context.Context, res *doctor.Result) {
	s.mu.Lock()
	in := s.loggedIn
	s.mu.Unlock()
	if !in {
		return
	}
	s.send(ctx, res)
}

func (s *Service) changed(ctx context.Context, res *doctor.Result) {
	s.mu.Lock()
	ok := s.loggedIn && s.started
	s.mu.Unlock()
	if ok {
		s.send(ctx, res)
	}
}

func (s *Service) send(ctx context.Context, res *doctor.Result) {
	if err := s.d.Control.Report(ctx, s.toReport(res)); err != nil {
		s.d.Log.WarnContext(ctx, "report failed")
	}
}

func (s *Service) toReport(res *doctor.Result) Report {
	return Report{Profiles: res.Profiles, SessionUsers: res.SessionUsers, Doctor: res.Report(), IsServerHost: res.IsServerHost,
		RequireLocalConfirm: s.d.Config.RequireLocalConfirm, OSVersion: s.d.OSVersion}
}

func (s *Service) logCodes(ctx context.Context, res *doctor.Result) {
	for _, e := range res.Report() {
		if e.Code == "" {
			continue
		}
		attrs := []slog.Attr{slog.String(AttrCheck, e.Check), slog.String(AttrCode, e.Code), slog.Bool(AttrOK, e.OK)}
		if e.SessionUser != "" {
			attrs = append(attrs, slog.String(AttrSessionUser, e.SessionUser))
		}
		s.d.Log.LogAttrs(ctx, slog.LevelWarn, "doctor finding", attrs...)
	}
}

func (s *Service) finish(code int) { s.once.Do(func() { s.end <- code }) }

// Start runs the start sequence and returns the running service, or the exit code of a refused
// start (ExitStartRefused) with a nil service.
func Start(ctx context.Context, d Deps) (*Service, int) {
	decl, err := d.State.ReadDeclaration()
	if err != nil {
		d.Log.WarnContext(ctx, "stored declaration unreadable")
	}
	bindings, err := d.State.ReadBindings()
	if err != nil {
		d.Log.WarnContext(ctx, "bindings unreadable")
		bindings = map[string]string{}
	}
	pairing, perr := d.State.ReadPairing()
	env := d.Doctor
	env.Declaration, env.Bindings, env.ServerURL = decl, bindings, pairing.ServerURL
	s := &Service{d: d, end: make(chan int, 1)}
	s.runner = doctor.NewRunner(env, func(r *doctor.Result) { s.changed(context.WithoutCancel(ctx), r) })
	res := s.runner.RunFull(ctx)
	if perr == nil {
		ready, err := d.Control.Login(ctx)
		if err == nil {
			s.mu.Lock()
			s.loggedIn = true
			s.mu.Unlock()
			res = s.applyDeclaration(ctx, ready.LLMCredentials)
		} else {
			d.Log.WarnContext(ctx, "login failed, the stored declaration decides")
		}
	}
	if res.StartRefused() {
		s.logCodes(ctx, res)
		if s.loggedIn {
			s.report(ctx, res)
			_ = d.Control.Close()
		}
		d.Log.ErrorContext(ctx, "start refused")
		return nil, ExitStartRefused
	}
	s.logCodes(ctx, res)
	s.mu.Lock()
	s.started = true
	s.mu.Unlock()
	if ev, ok := d.Control.(Events); ok {
		// Every later ready is the current declaration (M2-42 Design note 15, M2-43).
		ev.SetReadyHandler(func(ctx context.Context, r Ready) { s.OnReady(ctx, r) })
	}
	s.report(ctx, s.runner.Last())
	return s, 0
}

func (s *Service) applyDeclaration(ctx context.Context, decl string) *doctor.Result {
	if state.ValidDeclaration(decl) {
		if err := s.d.State.WriteDeclaration(decl); err != nil {
			s.d.Log.WarnContext(ctx, "declaration not stored")
		}
		if r := s.runner.SetDeclaration(decl); r != nil {
			return r
		}
	}
	return s.runner.Last()
}

// OnReady applies the declaration of a later ready (the server sends ready again on every change
// of the declaration, M2-43): store it, evaluate HD6 again and send a new report.
func (s *Service) OnReady(ctx context.Context, r Ready) *doctor.Result {
	// A ready is a login: a host that started while the server was unreachable reports from its
	// first later ready on (M2-43).
	s.mu.Lock()
	s.loggedIn = true
	s.mu.Unlock()
	res := s.applyDeclaration(ctx, r.LLMCredentials)
	s.report(ctx, res)
	return res
}

// Readiness tells M2-45 whether the host may give sessions and whether sessionUser may get them.
func (s *Service) Readiness(sessionUser string) (host, user bool) {
	return s.runner.Readiness(sessionUser)
}

// Runner returns the doctor runner; M2-45 schedules its Light runs.
func (s *Service) Runner() *doctor.Runner { return s.runner }

// RunDoctor is the op doctor.run: a full run with the entry scan, reported; on a "start refused"
// finding other than HD6, HD10 and HD11 the service ends like a refused start.
func (s *Service) RunDoctor(ctx context.Context) DoctorAnswer {
	s.runner.RunFull(ctx)
	s.runner.WaitScans()
	res := s.runner.Last()
	s.report(ctx, res)
	if res.EndsRunningService() {
		s.logCodes(ctx, res)
		s.finish(ExitStartRefused)
	}
	return DoctorAnswer{Output: res.Format(), Exit: res.ExitCode(), Doctor: res.Report()}
}

// Serve answers the admin endpoint until ctx ends or a doctor run ends the service; it returns the
// exit code of run.
func (s *Service) Serve(ctx context.Context) int {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	if s.d.Admin != nil {
		go func() { _ = localchan.ServeAdmin(ctx, s.d.Admin, s.d.AdminUID, s.adminHandler()) }()
	}
	var stopped <-chan struct{}
	if ev, ok := s.d.Control.(Events); ok {
		stopped = ev.Stopped()
	}
	select {
	case <-ctx.Done():
		_ = s.d.Control.Close()
		return 0
	case code := <-s.end:
		_ = s.d.Control.Close()
		return code
	case <-stopped:
		// The server refused the keys, found a second host service or does not speak our
		// version: no service manager restarts after this code (D-M2-42-14, M2-43).
		s.d.Log.ErrorContext(ctx, "control connection stopped")
		_ = s.d.Control.Close()
		return ExitStartRefused
	}
}

func (s *Service) adminHandler() localchan.Handler {
	return func(ctx context.Context, m localchan.Message) (string, any) {
		switch m.Op {
		case localchan.OpDoctorRun:
			return localchan.OpDoctorRes, s.RunDoctor(ctx)
		case localchan.OpPairRun:
			var req PairRequest
			if err := json.Unmarshal(m.D, &req); err != nil {
				return localchan.ErrorAnswer("malformed")
			}
			return localchan.OpPairRes, s.PairFromAdmin(ctx, req)
		}
		return localchan.ErrorAnswer("unknown_op")
	}
}

// PairFromAdmin pairs with the lists of the last Full run (D-M2-42-16); the administrator restarts
// the service afterwards, so M2-43 logs in with the new keys.
func (s *Service) PairFromAdmin(ctx context.Context, req PairRequest) PairAnswer {
	res := s.runner.Last()
	pair := s.d.Pair
	if pair == nil {
		pair = pairing.Pair
	}
	out, err := pair(ctx, pairing.Options{ServerURL: req.ServerURL, Code: req.Code, CAPEM: req.CAPEM, State: s.d.State, Random: rand.Reader,
		Platform: s.d.Platform, OSVersion: s.d.OSVersion, HostVersion: s.d.HostVersion,
		Lists: pairing.Lists{Profiles: res.Profiles, SessionUsers: res.SessionUsers}})
	if err != nil {
		return PairAnswer{Error: err.Error()}
	}
	return PairAnswer{Fingerprint: out.Fingerprint, HostID: out.HostID}
}

// AskPair calls pair.run on an open admin connection.
func AskPair(ctx context.Context, c *localchan.Conn, req PairRequest) (PairAnswer, error) {
	var a PairAnswer
	if err := c.Call(ctx, localchan.OpPairRun, req, localchan.OpPairRes, &a); err != nil {
		return PairAnswer{}, err
	}
	return a, nil
}

// Run is "bothy-host run": Start, then Serve.
func Run(ctx context.Context, d Deps) int {
	s, code := Start(ctx, d)
	if s == nil {
		return code
	}
	return s.Serve(ctx)
}

// DoctorOnce is the start argument doctor-once of the Windows service (D-M2-42-1): one full run
// with the entry scan, its answer to the first doctor.run on the admin endpoint, then exit 0. It
// connects to no server and accepts no session.
func DoctorOnce(ctx context.Context, d Deps) int {
	decl, _ := d.State.ReadDeclaration()
	bindings, _ := d.State.ReadBindings()
	pairing, _ := d.State.ReadPairing()
	env := d.Doctor
	env.Declaration, env.Bindings, env.ServerURL = decl, bindings, pairing.ServerURL
	runner := doctor.NewRunner(env, nil)
	runner.RunFull(ctx)
	runner.WaitScans()
	res := runner.Last()
	answer := DoctorAnswer{Output: res.Format(), Exit: res.ExitCode(), Doctor: res.Report()}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	served := make(chan struct{})
	var once sync.Once
	go func() {
		_ = localchan.ServeAdmin(ctx, d.Admin, d.AdminUID, func(context.Context, localchan.Message) (string, any) {
			defer once.Do(func() { close(served) })
			return localchan.OpDoctorRes, answer
		})
	}()
	select {
	case <-served:
	case <-ctx.Done():
	}
	return 0
}

// AskDoctor calls doctor.run on an open admin connection.
func AskDoctor(ctx context.Context, c *localchan.Conn) (DoctorAnswer, error) {
	var a DoctorAnswer
	if err := c.Call(ctx, localchan.OpDoctorRun, struct{}{}, localchan.OpDoctorRes, &a); err != nil {
		return DoctorAnswer{}, err
	}
	return a, nil
}
