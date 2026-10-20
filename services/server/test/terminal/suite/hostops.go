// SPDX-License-Identifier: Apache-2.0

package suite

import (
	"fmt"
	"strings"

	"github.com/OWNER/bothy/services/server/test/terminal/stack"
)

// hostStep runs a step of a host container: the operator's commands as root, a person's
// commands as their user, and the faults of the machine.
func (w *instance) hostStep(s Step) []string {
	h, err := w.container(s.Host)
	if err != nil {
		return []string{err.Error()}
	}
	var res stack.ExecResult
	switch s.Do {
	case "exec", "exec_as":
		args := s.Run
		if s.Script != "" {
			args = []string{"bash", "-c", s.Script}
		}
		for i := range args {
			args[i] = w.expand(args[i])
		}
		if len(args) == 0 {
			return []string{"exec needs run or script"}
		}
		if s.Do == "exec_as" {
			res, err = h.ExecAs(w.ctx, s.User, nil, args...)
		} else {
			res, err = h.Exec(w.ctx, nil, args...)
		}
	case "restart_service":
		res, err = h.Bothy(w.ctx, "restart")
	case "doctor":
		res, err = h.Doctor(w.ctx)
	case "reboot":
		err = h.Reboot(w.ctx)
	case "pause":
		err = h.Pause(w.ctx)
	case "unpause":
		err = h.Unpause(w.ctx)
	case "net_down":
		err = h.NetDown(w.ctx)
	case "net_up":
		err = h.NetUp(w.ctx)
	case "clone_state":
		to, cerr := w.container(s.To)
		if cerr != nil {
			return []string{cerr.Error()}
		}
		err = h.CloneState(w.ctx, to)
	case "pair":
		res, err = h.PairResult(w.ctx, w.expand(s.Text))
		if fp := stack.Fingerprint(res); err == nil && fp != "" {
			w.mu.Lock()
			w.hosts[s.Host].fingerprint = fp
			w.mu.Unlock()
		}
	default:
		return []string{"unknown host op " + s.Do}
	}
	if err != nil {
		return []string{fmt.Sprintf("%s on %s: %v", s.Do, s.Host, err)}
	}
	if s.Expect != nil {
		return CompareExec(*s.Expect, res.Code, res.Stdout, res.Stderr)
	}
	if (s.Do == "exec" || s.Do == "exec_as") && res.Code != 0 {
		return []string{fmt.Sprintf("%s on %s exited %d", s.Do, s.Host, res.Code)}
	}
	return nil
}

// serverStep runs a step of the server: a restart of a Compose service, a routing mode of the TLS
// front or SQL (read only; a write needs bypass and exists only in tm-t28).
func (w *instance) serverStep(s Step) []string {
	switch s.Server {
	case "restart":
		svc := orDefault(s.Service, "api")
		if err := w.env.Stack.RestartService(w.ctx, svc); err != nil {
			return []string{fmt.Sprintf("restart %s: %v", svc, err)}
		}
		return nil
	case "front_mode":
		if err := w.env.Stack.FrontMode(w.ctx, stack.Mode(s.Mode)); err != nil {
			return []string{fmt.Sprintf("front mode %s: %v", s.Mode, err)}
		}
		return nil
	case "sql":
		q := w.expand(s.SQL)
		if sqlWrite.MatchString(q) && !s.Bypass {
			return []string{"a writing sql step needs bypass"}
		}
		rows, err := w.env.Stack.SQL(w.ctx, q, s.Bypass)
		if err != nil {
			return []string{err.Error()}
		}
		if len(s.Keep) > 0 {
			if len(rows) == 0 {
				return []string{"the query returned no row to keep"}
			}
			w.mu.Lock()
			for name, col := range s.Keep {
				w.keep[name] = fmt.Sprint(rows[0][strings.TrimPrefix(col, "/")])
			}
			w.mu.Unlock()
		}
		return nil
	}
	return []string{"unknown server op " + s.Server}
}
