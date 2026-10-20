// SPDX-License-Identifier: Apache-2.0

package helper

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"slices"
	"sync"
	"time"

	"github.com/OWNER/bothy/services/host/internal/config"
	"github.com/OWNER/bothy/services/host/internal/doctor"
	"github.com/OWNER/bothy/services/host/internal/frame"
	"github.com/OWNER/bothy/services/host/internal/localchan"
	"github.com/OWNER/bothy/services/host/internal/noise"
	"github.com/OWNER/bothy/services/host/internal/profiles"
	"github.com/OWNER/bothy/services/host/internal/pty"
	"github.com/OWNER/bothy/services/host/internal/state"
)

// exitDrain bounds how long the helper keeps reading the PTY after the session leader exited before
// it ends the tree: a background process that holds the PTY open must not keep the session alive,
// and ConPTY delivers the end of its output only when the pseudo console closes. A protocol constant
// of the helper, not a product window.
const exitDrain = 250 * time.Millisecond

// inputQueue bounds the input frames waiting for one PTY; a program that stops reading its input
// loses further input instead of blocking the channel of every session of the helper. A protocol
// constant of the channel.
const inputQueue = 256

// Starter starts PTYs (pty.Supervisor).
type Starter interface {
	Start(spec pty.StartSpec) (pty.Process, error)
}

// SessionEnv is what the helper needs to serve session channels: the checks of M2-42, the
// environment for the profiles, the PTY supervisor and a log for codes.
type SessionEnv struct {
	Check    doctor.HelperEnv
	Profiles profiles.Env
	PTY      Starter
	Log      *slog.Logger
}

// State is what the helper remembers between channels: the last check.run request (claude_bin,
// search list, working directories) and the identities it reported for the working directories.
// It is safe for concurrent use.
type State struct {
	mu   sync.Mutex
	req  doctor.Request
	ids  map[string]string
	seen bool
}

func (st *State) remember(req doctor.Request, res doctor.HelperResult) {
	st.mu.Lock()
	defer st.mu.Unlock()
	st.req, st.seen = req, true
	st.ids = map[string]string{}
	for i, w := range req.Workdirs {
		if i < len(res.WorkdirIDs) && res.WorkdirIDs[i] != "" {
			st.ids[w] = res.WorkdirIDs[i]
		}
	}
}

func (st *State) last() (doctor.Request, map[string]string, bool) {
	st.mu.Lock()
	defer st.mu.Unlock()
	return st.req, st.ids, st.seen
}

// proc is one PTY started on a channel.
type proc struct {
	p     pty.Process
	in    chan []byte
	ended chan struct{}
	once  sync.Once
}

func (pr *proc) stop() {
	pr.once.Do(func() {
		_ = pr.p.Kill()
		close(pr.ended)
	})
}

// ServeSessions serves one verified channel of the service: the ops hello, check.run and scan.run
// of M2-42 and the session ops of M2-45. Every PTY started on the channel ends with its tree when
// the channel breaks (D-W-90); ServeSessions returns then and closes c.
func ServeSessions(ctx context.Context, c *localchan.Conn, env SessionEnv, st *State) {
	defer func() { _ = c.Close() }()
	var mu sync.Mutex
	procs := map[[localchan.SessionIDLen]byte]*proc{}
	defer func() {
		mu.Lock()
		all := procs
		procs = map[[localchan.SessionIDLen]byte]*proc{}
		mu.Unlock()
		for _, pr := range all {
			pr.stop()
		}
	}()
	check := Handler(env.Check)
	for {
		p, err := c.ReadPacket()
		if err != nil {
			return
		}
		if p.IsFrame {
			mu.Lock()
			pr := procs[p.SessionID]
			mu.Unlock()
			f, ferr := frame.Decode(p.Frame, frame.FromDevice)
			if pr == nil || ferr != nil || f.Type != frame.TypeData {
				continue // a frame of an ended session
			}
			select {
			case pr.in <- f.Data:
			default:
			}
			continue
		}
		m := p.Msg
		switch m.Op {
		case localchan.OpHello, localchan.OpScanRun:
			go answer(ctx, c, check, m)
		case localchan.OpCheckRun:
			go func() {
				var req doctor.Request
				if json.Unmarshal(m.D, &req) != nil || req.SessionUser != env.Check.Self.Name {
					_ = c.Close()
					return
				}
				res := doctor.RunHelper(ctx, env.Check, req)
				st.remember(req, res)
				_ = c.Send(localchan.OpCheckResult, m.ID, res)
			}()
		case localchan.OpPTYStart:
			var req localchan.PTYStart
			if json.Unmarshal(m.D, &req) != nil {
				return
			}
			go start(ctx, c, env, st, req, &mu, procs)
		case localchan.OpPTYResize:
			var req localchan.PTYResize
			if json.Unmarshal(m.D, &req) != nil {
				return
			}
			if pr := lookup(&mu, procs, req.SessionID); pr != nil {
				_ = pr.p.Resize(req.Cols, req.Rows)
			}
		case localchan.OpPTYStop:
			var req localchan.PTYStop
			if json.Unmarshal(m.D, &req) != nil {
				return
			}
			if pr := lookup(&mu, procs, req.SessionID); pr != nil {
				pr.stop()
			}
		default:
			return
		}
	}
}

func answer(ctx context.Context, c *localchan.Conn, h localchan.Handler, m localchan.Message) {
	op, d := h(ctx, m)
	if op == "" {
		_ = c.Close()
		return
	}
	_ = c.Send(op, m.ID, d)
}

func lookup(mu *sync.Mutex, procs map[[localchan.SessionIDLen]byte]*proc, id string) *proc {
	sid, err := noise.ParseID(id)
	if err != nil {
		return nil
	}
	mu.Lock()
	defer mu.Unlock()
	return procs[sid]
}

// start runs one pty.start and answers pty.started or pty.failed; then it pumps the PTY until the
// process ends and reports pty.exited.
func start(ctx context.Context, c *localchan.Conn, env SessionEnv, st *State, req localchan.PTYStart, mu *sync.Mutex,
	procs map[[localchan.SessionIDLen]byte]*proc) {
	sid, err := noise.ParseID(req.SessionID)
	if err != nil {
		_ = c.Close()
		return
	}
	fail := func(detail string) {
		if env.Log != nil {
			env.Log.LogAttrs(ctx, slog.LevelWarn, "pty not started", slog.String("session_id", req.SessionID), slog.String("close_detail", detail))
		}
		_ = c.Send(localchan.OpPTYFailed, "", localchan.PTYFailed{SessionID: req.SessionID, CloseDetail: detail})
	}
	spec, detail := buildSpec(ctx, env, st, req)
	req.APIKey = ""
	if detail != "" {
		fail(detail)
		return
	}
	p, err := env.PTY.Start(pty.StartSpec{SessionID: req.SessionID, Path: spec.Path, Argv0: spec.Argv0, Args: spec.Args, Env: spec.Env,
		Dir: spec.Dir, Cols: req.Cols, Rows: req.Rows})
	clear(spec.Env)
	if err != nil {
		fail(profiles.DetailPTYFailed)
		return
	}
	pr := &proc{p: p, in: make(chan []byte, inputQueue), ended: make(chan struct{})}
	mu.Lock()
	procs[sid] = pr
	mu.Unlock()
	if err := c.Send(localchan.OpPTYStarted, "", localchan.PTYStarted{SessionID: req.SessionID}); err != nil {
		pr.stop()
		return
	}
	go func() {
		for {
			select {
			case b := <-pr.in:
				if _, err := p.Write(b); err != nil {
					return
				}
			case <-pr.ended:
				return
			}
		}
	}()
	read := make(chan struct{})
	go func() {
		defer close(read)
		buf := make([]byte, localchan.MaxData)
		for {
			n, err := p.Read(buf)
			if n > 0 {
				f, ferr := frame.Encode(frame.Data(buf[:n]))
				if ferr == nil && c.SendFrame(sid, f) != nil {
					return
				}
			}
			if err != nil {
				return
			}
		}
	}()
	code := p.Wait()
	t := time.NewTimer(exitDrain)
	select {
	case <-read:
	case <-t.C:
	}
	t.Stop()
	pr.stop()
	<-read
	mu.Lock()
	if procs[sid] == pr {
		delete(procs, sid)
	}
	mu.Unlock()
	_ = c.Send(localchan.OpPTYExited, "", localchan.PTYExited{SessionID: req.SessionID, ExitCode: code})
}

// buildSpec builds the start of req; the detail is "" or a close_detail.
func buildSpec(ctx context.Context, env SessionEnv, st *State, req localchan.PTYStart) (profiles.Spec, string) {
	var (
		spec profiles.Spec
		err  error
	)
	switch req.Kind {
	case config.KindShell:
		spec, err = profiles.Shell(env.Profiles)
	case config.KindCommand:
		spec, err = profiles.Command(env.Profiles, req.Program, req.Args)
	case config.KindClaude:
		last, ids, ok := st.last()
		wd := req.Workdir
		if !ok || !slices.Contains(last.Workdirs, wd) {
			return spec, profiles.DetailSharedLocation
		}
		if id, ierr := env.Check.Resolver.Identity(wd); ierr != nil || id == "" || id != ids[wd] {
			return spec, profiles.DetailSharedLocation
		}
		switch req.Declaration {
		case state.DeclOrganization:
			if last.ClaudeBin == "" {
				return spec, profiles.DetailPTYFailed
			}
			spec, err = profiles.ClaudeOrganization(env.Profiles, last.ClaudeBin, last.SearchList, wd, req.APIKey)
		case state.DeclPersonal:
			spec, err = profiles.ClaudePersonal(ctx, env.Profiles, wd, last.AuthStatusTimeout)
		default:
			return spec, profiles.DetailPTYFailed
		}
	default:
		return spec, profiles.DetailPTYFailed
	}
	if err != nil {
		var se *profiles.StartError
		if errors.As(err, &se) && se.Detail == profiles.DetailSharedLocation {
			return spec, profiles.DetailSharedLocation
		}
		return spec, profiles.DetailPTYFailed
	}
	return spec, ""
}
