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
	// answers caches the last answer of each helper, so RunLightUser asks only one helper and
	// keeps the latest periodic answers of the others (M2-45 Design note 11).
	answers map[string]cachedAnswer
}

// cachedAnswer is the last check.run answer of one helper.
type cachedAnswer struct {
	res HelperResult
	err error
}

// NewRunner returns a Runner over env; onChange (may be nil) is called with the new result when
// a finished entry scan changes it, so the service can send a new report.
func NewRunner(env Env, onChange func(*Result)) *Runner {
	return &Runner{env: env, scans: map[string]*scanState{}, onChange: onChange, answers: map[string]cachedAnswer{}}
}

// RunFull runs HD1 to HD11 and starts the entry scan of every reachable session user in its
// helper. Until its first scan finishes, a session user fails HD11 with entry_scan_running; after
// that the previous finding holds until the new scan finishes.
func (r *Runner) RunFull(ctx context.Context) *Result {
	// The entry scan runs over the snapshot itself; the checks record the helper answers for
	// RunLightUser (M2-45 Design note 11).
	snap := r.snapshotEnv()
	env := snap
	env.Helpers = r.recording(snap.Helpers, "")
	res := RunService(ctx, env)
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
		// The entry scan covers only the working directories HD7 keeps: a folder of two session
		// users holds entries of both and is no place of either (M2-45, TM-T29 (c)).
		req.Workdirs = nil
		for _, w := range res.Workdirs[u] {
			req.Workdirs = append(req.Workdirs, w.Path)
		}
		r.wg.Go(func() { r.scan(ctx, snap, u, req) })
	}
	out := r.merged()
	r.mu.Unlock()
	return out
}

// scan runs the entry scan of user over env, the snapshot RunFull took under the lock, so a
// SetDeclaration during the scan does not race with it.
func (r *Runner) scan(ctx context.Context, env Env, user string, req Request) {
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
	return r.light(ctx, "")
}

// RunLightUser is the Light run before a PTY of sessionUser (M2-45 Design note 11): HD1 to HD11 as
// the service user, the places for all included, but only the helper of sessionUser is asked;
// every other session user keeps the answer of its last run (its age is bounded by
// host_service.credential_check_interval). A session user without an earlier answer is asked too.
func (r *Runner) RunLightUser(ctx context.Context, sessionUser string) *Result {
	return r.light(ctx, sessionUser)
}

func (r *Runner) light(ctx context.Context, only string) *Result {
	env := r.snapshotEnv()
	env.Mode = ModeLight
	env.Helpers = r.recording(env.Helpers, only)
	res := RunService(ctx, env)
	r.mu.Lock()
	defer r.mu.Unlock()
	r.base = res
	return r.merged()
}

// recording wraps h: every answer is cached; with only set, a helper other than only answers from
// the cache when it has an entry.
func (r *Runner) recording(h Helpers, only string) Helpers {
	return cachingHelpers{r: r, h: h, only: only}
}

type cachingHelpers struct {
	r    *Runner
	h    Helpers
	only string
}

func (c cachingHelpers) Check(ctx context.Context, user string, req Request) (HelperResult, error) {
	if c.only != "" && user != c.only {
		c.r.mu.Lock()
		a, ok := c.r.answers[user]
		c.r.mu.Unlock()
		if ok {
			return a.res, a.err
		}
	}
	res, err := c.h.Check(ctx, user, req)
	c.r.mu.Lock()
	c.r.answers[user] = cachedAnswer{res: res, err: err}
	c.r.mu.Unlock()
	return res, err
}

func (c cachingHelpers) Scan(ctx context.Context, user string, req Request) (string, error) {
	return c.h.Scan(ctx, user, req)
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
