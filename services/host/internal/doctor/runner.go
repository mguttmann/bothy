// SPDX-License-Identifier: Apache-2.0

package doctor

import (
	"context"
	"slices"
	"sync"
)

// scanState is the entry scan state of one session user.
type scanState struct {
	code    string // finding of the last finished Full scan ("" when it found nothing)
	done    bool   // a scan finished at least once
	running bool
}

// Runner runs the checks for the host service over its lifetime: it keeps the findings of the
// entry scan of HD11 between runs (only a later Full run that finishes and finds nothing lifts
// one; Light never runs it), lets each session user wait only for its own scan, and re-evaluates
// HD6 when a ready brings a declaration (D-M2-42-15). M2-45 schedules RunLight and consults
// Readiness.
type Runner struct {
	mu       sync.Mutex
	env      Env
	scans    map[string]*scanState
	base     *Result
	wg       sync.WaitGroup
	onChange func(*Result)
}

// NewRunner returns a Runner over env; onChange (may be nil) is called with the new result when
// a finished entry scan changes it, so the service can send a new report.
func NewRunner(env Env, onChange func(*Result)) *Runner {
	return &Runner{env: env, scans: map[string]*scanState{}, onChange: onChange}
}

// RunFull runs HD1 to HD11 and starts the entry scan of every reachable session user in its
// helper. Until its first scan finishes, a session user fails HD11 with entry_scan_running; after
// that the previous finding holds until the new scan finishes.
func (r *Runner) RunFull(ctx context.Context) *Result {
	res := RunService(ctx, r.snapshotEnv())
	r.mu.Lock()
	r.base = res
	for _, u := range res.reached {
		st := r.scans[u]
		if st == nil {
			st = &scanState{}
			r.scans[u] = st
		}
		if st.running {
			continue
		}
		st.running = true
		req := res.requests[u]
		r.wg.Go(func() { r.scan(ctx, u, req) })
	}
	out := r.merged()
	r.mu.Unlock()
	return out
}

func (r *Runner) scan(ctx context.Context, user string, req Request) {
	env := r.snapshotEnv() // SetDeclaration may write r.env while the scan runs
	sctx, cancel := context.WithTimeout(ctx, env.scanBound())
	defer cancel()
	code, err := env.Helpers.Scan(sctx, user, req)
	if err != nil {
		code = CodeEntryScanTimeout // a scan that did not finish has not seen every entry
	}
	r.mu.Lock()
	st := r.scans[user]
	st.running, st.done, st.code = false, true, code
	out := r.merged()
	cb := r.onChange
	r.mu.Unlock()
	if cb != nil {
		cb(out)
	}
}

// WaitScans blocks until every started entry scan finished.
func (r *Runner) WaitScans() { r.wg.Wait() }

// RunLight runs HD1 to HD11 without the entry scan; the findings of earlier scans stay.
func (r *Runner) RunLight(ctx context.Context) *Result {
	env := r.snapshotEnv()
	res := RunService(ctx, env)
	r.mu.Lock()
	defer r.mu.Unlock()
	r.base = res
	return r.merged()
}

// SetDeclaration stores the declaration of a ready and returns the last result evaluated with it.
func (r *Runner) SetDeclaration(decl string) *Result {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.env.Declaration = decl
	if r.base == nil {
		return nil
	}
	r.base = r.base.WithDeclaration(decl)
	return r.merged()
}

// Last returns the last result with the scan state merged in, or nil before the first run.
func (r *Runner) Last() *Result {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.base == nil {
		return nil
	}
	return r.merged()
}

// Readiness tells M2-45 whether the host may give sessions at all and whether sessionUser may get
// them (HD11 for it, entry_scan_running included).
func (r *Runner) Readiness(sessionUser string) (host, user bool) {
	res := r.Last()
	if res == nil {
		return false, false
	}
	return res.HostReady(), res.SessionUserReady(sessionUser)
}

func (r *Runner) snapshotEnv() Env {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.env
}

// merged returns the base result with the scan findings in HD11; r.mu is held.
func (r *Runner) merged() *Result {
	out := *r.base
	out.Checks = slices.Clone(r.base.Checks)
	hd11 := out.Check(HD11)
	hd11.Entries = slices.Clone(hd11.Entries)
	users := make([]string, 0, len(r.scans))
	for u := range r.scans {
		users = append(users, u)
	}
	slices.Sort(users)
	for _, u := range users {
		st := r.scans[u]
		code := st.code
		if st.running && !st.done {
			code = CodeEntryScanRunning
		}
		if code != "" {
			hd11.Entries = append(hd11.Entries, Entry{Status: Fail, Code: code, SessionUser: u})
		}
	}
	out.set(hd11)
	return &out
}
