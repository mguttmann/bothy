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

// Log attributes of the helper: codes and IDs only.
const (
	attrSession = "session_id"
	attrDetail  = "close_detail"
)

// sessionChannel is one verified channel of the service with the PTYs started on it.
type sessionChannel struct {
	c     *localchan.Conn
	env   SessionEnv
	st    *State
	check localchan.Handler
	mu    sync.Mutex
	procs map[[localchan.SessionIDLen]byte]*proc
}

// ServeSessions serves one verified channel of the service: the ops hello, check.run and scan.run
// of M2-42 and the session ops of M2-45. Every PTY started on the channel ends with its tree when
// the channel breaks (D-W-90); ServeSessions returns then and closes c.
func ServeSessions(ctx context.Context, c *localchan.Conn, env SessionEnv, st *State) {
	sc := &sessionChannel{c: c, env: env, st: st, check: Handler(env.Check), procs: map[[localchan.SessionIDLen]byte]*proc{}}
	defer sc.close()
	for {
		p, err := c.ReadPacket()
		if err != nil {
			return
		}
		if p.IsFrame {
			sc.input(p)
			continue
		}
		if !sc.control(ctx, p.Msg) {
			return
		}
	}
}

// close ends every PTY of the channel and closes it.
func (sc *sessionChannel) close() {
	_ = sc.c.Close()
	sc.mu.Lock()
	all := sc.procs
	sc.procs = map[[localchan.SessionIDLen]byte]*proc{}
	sc.mu.Unlock()
	for _, pr := range all {
		pr.stop()
	}
}

// input hands the data frame of a session to its PTY; a frame of an ended session is dropped, and
// so is input a program does not read in time.
func (sc *sessionChannel) input(p localchan.Packet) {
	sc.mu.Lock()
	pr := sc.procs[p.SessionID]
	sc.mu.Unlock()
	f, err := frame.Decode(p.Frame, frame.FromDevice)
	if pr == nil || err != nil || f.Type != frame.TypeData {
		return
	}
	select {
	case pr.in <- f.Data:
	default:
	}
}

// control handles one control message; false closes the channel.
func (sc *sessionChannel) control(ctx context.Context, m localchan.Message) bool {
	switch m.Op {
	case localchan.OpHello, localchan.OpScanRun:
		go sc.answer(ctx, m)
	case localchan.OpCheckRun:
		var req doctor.Request
		if json.Unmarshal(m.D, &req) != nil || req.SessionUser != sc.env.Check.Self.Name {
			return false
		}
		go func() {
			res := doctor.RunHelper(ctx, sc.env.Check, req)
			sc.st.remember(req, res)
			_ = sc.c.Send(localchan.OpCheckResult, m.ID, res)
		}()
	case localchan.OpPTYStart:
		var req localchan.PTYStart
		if json.Unmarshal(m.D, &req) != nil {
			return false
		}
		go sc.start(ctx, req)
	case localchan.OpPTYResize:
		var req localchan.PTYResize
		if json.Unmarshal(m.D, &req) != nil {
			return false
		}
		if pr := sc.lookup(req.SessionID); pr != nil {
			_ = pr.p.Resize(req.Cols, req.Rows)
		}
	case localchan.OpPTYStop:
		var req localchan.PTYStop
		if json.Unmarshal(m.D, &req) != nil {
			return false
		}
		if pr := sc.lookup(req.SessionID); pr != nil {
			pr.stop()
		}
	default:
		return false
	}
	return true
}

func (sc *sessionChannel) answer(ctx context.Context, m localchan.Message) {
	op, d := sc.check(ctx, m)
	if op == "" {
		_ = sc.c.Close()
		return
	}
	_ = sc.c.Send(op, m.ID, d)
}

func (sc *sessionChannel) lookup(id string) *proc {
	sid, err := noise.ParseID(id)
	if err != nil {
		return nil
	}
	sc.mu.Lock()
	defer sc.mu.Unlock()
	return sc.procs[sid]
}

// start runs one pty.start and answers pty.started or pty.failed; then it pumps the PTY until the
// process ends and reports pty.exited.
func (sc *sessionChannel) start(ctx context.Context, req localchan.PTYStart) {
	sid, err := noise.ParseID(req.SessionID)
	if err != nil {
		_ = sc.c.Close()
		return
	}
	spec, detail := buildSpec(ctx, sc.env, sc.st, req)
	req.APIKey = ""
	var p pty.Process
	if detail == "" {
		p, err = sc.env.PTY.Start(pty.StartSpec{SessionID: req.SessionID, Path: spec.Path, Argv0: spec.Argv0, Args: spec.Args, Env: spec.Env,
			Dir: spec.Dir, Cols: req.Cols, Rows: req.Rows})
		clear(spec.Env)
		if err != nil {
			detail = profiles.DetailPTYFailed
		}
	}
	if detail != "" {
		if sc.env.Log != nil {
			sc.env.Log.LogAttrs(ctx, slog.LevelWarn, "pty not started", slog.String(attrSession, req.SessionID), slog.String(attrDetail, detail))
		}
		_ = sc.c.Send(localchan.OpPTYFailed, "", localchan.PTYFailed{SessionID: req.SessionID, CloseDetail: detail})
		return
	}
	pr := &proc{p: p, in: make(chan []byte, inputQueue), ended: make(chan struct{})}
	sc.mu.Lock()
	sc.procs[sid] = pr
	sc.mu.Unlock()
	if err := sc.c.Send(localchan.OpPTYStarted, "", localchan.PTYStarted{SessionID: req.SessionID}); err != nil {
		pr.stop()
		return
	}
	go pr.write()
	read := make(chan struct{})
	go sc.output(sid, p, read)
	code := p.Wait()
	t := time.NewTimer(exitDrain)
	select {
	case <-read:
	case <-t.C:
	}
	t.Stop()
	pr.stop()
	<-read
	sc.mu.Lock()
	if sc.procs[sid] == pr {
		delete(sc.procs, sid)
	}
	sc.mu.Unlock()
	_ = sc.c.Send(localchan.OpPTYExited, "", localchan.PTYExited{SessionID: req.SessionID, ExitCode: code})
}

// write feeds the input of the device to the PTY until the session ends.
func (pr *proc) write() {
	for {
		select {
		case b := <-pr.in:
			if _, err := pr.p.Write(b); err != nil {
				return
			}
		case <-pr.ended:
			return
		}
	}
}

// output sends the PTY output in data frames small enough for a kind 0x01 body until the PTY ends.
func (sc *sessionChannel) output(sid [localchan.SessionIDLen]byte, p pty.Process, done chan struct{}) {
	defer close(done)
	buf := make([]byte, localchan.MaxData)
	for {
		n, err := p.Read(buf)
		if n > 0 {
			f, ferr := frame.Encode(frame.Data(buf[:n]))
			if ferr == nil && sc.c.SendFrame(sid, f) != nil {
				return
			}
		}
		if err != nil {
			return
		}
	}
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
		var detail string
		spec, detail, err = claudeSpec(ctx, env, st, req)
		if detail != "" {
			return spec, detail
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

// claudeSpec builds the start of the profile claude: the working directory must be one of the
// last check.run with the identity the helper reported for it (D-W-84), then organization or
// personal.
func claudeSpec(ctx context.Context, env SessionEnv, st *State, req localchan.PTYStart) (profiles.Spec, string, error) {
	last, ids, ok := st.last()
	wd := req.Workdir
	if !ok || !slices.Contains(last.Workdirs, wd) {
		return profiles.Spec{}, profiles.DetailSharedLocation, nil
	}
	if id, err := env.Check.Resolver.Identity(wd); err != nil || id == "" || id != ids[wd] {
		return profiles.Spec{}, profiles.DetailSharedLocation, nil
	}
	switch req.Declaration {
	case state.DeclOrganization:
		if last.ClaudeBin == "" {
			return profiles.Spec{}, profiles.DetailPTYFailed, nil
		}
		spec, err := profiles.ClaudeOrganization(env.Profiles, last.ClaudeBin, last.SearchList, wd, req.APIKey)
		return spec, "", err
	case state.DeclPersonal:
		spec, err := profiles.ClaudePersonal(ctx, env.Profiles, wd, last.AuthStatusTimeout)
		return spec, "", err
	}
	return profiles.Spec{}, profiles.DetailPTYFailed, nil
}
