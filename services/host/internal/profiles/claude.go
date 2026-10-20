// SPDX-License-Identifier: Apache-2.0

package profiles

import (
	"context"
	"path/filepath"
	"strings"
	"time"

	"github.com/OWNER/bothy/services/host/internal/hostos"
)

// ClaudeOrganization returns the start of the profile claude with the declaration organization
// (24 7.4, 11.1): program claudeBin, the resolved host_service.claude_bin of HD10 (never resolved
// again), in workdir, the working directory of wdi the helper resolved and checked; PATH is
// searchList, the fixed search list of HD6, followed by the cleaned PATH without the folders the
// list already holds; ANTHROPIC_API_KEY is apiKey, the content of the key file claude_api_key; and
// CLAUDE_CONFIG_DIR is the folder of orgConfigDir. On Windows the base is the environment block
// without the variables that select another login or provider.
//
// An empty claudeBin or apiKey gives a *StartError with DetailPTYFailed, because without them the
// agent would ask for a login; a CLAUDE_CONFIG_DIR that does not resolve to exactly its folder in
// the home gives DetailSharedLocation.
func ClaudeOrganization(e Env, claudeBin string, searchList []string, workdir, apiKey string) (Spec, error) {
	if claudeBin == "" || apiKey == "" {
		return Spec{}, refuse("claude", DetailPTYFailed)
	}
	cfg, err := orgConfigDir(e)
	if err != nil {
		return Spec{}, err
	}
	sep := listSep(e.GOOS)
	inherited := lookup(e.GOOS, e.Environ, "PATH")
	env := baseEnv(e)
	if e.GOOS == hostos.Windows {
		inherited = lookup(e.GOOS, e.EnvBlock, "PATH")
		env = without(e.GOOS, env, forbiddenWindows...)
	}
	path := append([]string{}, searchList...)
	for _, p := range strings.Split(CleanPath(e, inherited), sep) {
		if p != "" && !inSearchList(e, searchList, p) {
			path = append(path, p)
		}
	}
	env = setVar(e.GOOS, env, "PATH", strings.Join(path, sep))
	env = setVar(e.GOOS, env, "ANTHROPIC_API_KEY", apiKey)
	env = setVar(e.GOOS, env, "CLAUDE_CONFIG_DIR", cfg)
	// TERM stays last, where baseEnv put it, so the order is the same for every profile.
	env = setVar(e.GOOS, env, "TERM", TERM)
	return Spec{Path: claudeBin, Argv0: claudeBin, Env: env, Dir: workdir}, nil
}

// inSearchList reports whether the PATH entry p resolves to a folder of the search list.
func inSearchList(e Env, searchList []string, p string) bool {
	r, err := e.Resolver.Resolve(p)
	if err != nil {
		return false
	}
	for _, s := range searchList {
		if samePath(e.GOOS, s, p) {
			return true
		}
		if rs, err := e.Resolver.Resolve(s); err == nil && samePath(e.GOOS, rs.Final, r.Final) {
			return true
		}
	}
	return false
}

// ClaudePersonal returns the start of the profile claude with the declaration personal (24 7.4,
// 11.1, Design note 9): it runs the probe of the login shell e.LoginEnv bounded by probeTimeout
// (host_service.auth_status_timeout) and checks its CLAUDE_CONFIG_DIR with personalConfigDir; then
// the login shell starts with PersonalCommand, which sets that value (or removes the variable when
// it was unset) and runs the holder's own claude from the shell's PATH, in workdir. No key, token
// or login crosses into the environment.
//
// A missing probe, a probe error or a probe that outlasts probeTimeout gives a *StartError with
// DetailPTYFailed, as does a login shell that Shell would refuse; a folder outside the home gives
// DetailSharedLocation.
func ClaudePersonal(ctx context.Context, e Env, workdir string, probeTimeout time.Duration) (Spec, error) {
	sh, err := loginShell(e, "claude")
	if err != nil {
		return Spec{}, err
	}
	dir, err := probeConfigDir(ctx, e, probeTimeout)
	if err != nil {
		return Spec{}, err
	}
	if err := personalConfigDir(e, dir); err != nil {
		return Spec{}, err
	}
	kind := "powershell"
	if e.GOOS != hostos.Windows {
		kind = filepath.Base(sh.Path)
	}
	sh.Args = PersonalCommand(kind, dir, dir != "")
	sh.Env = baseEnv(e)
	sh.Dir = workdir
	return sh, nil
}

// probeConfigDir runs e.LoginEnv and returns its CLAUDE_CONFIG_DIR. The probe gets the timeout
// and a context that ends with it; a probe that ignores both is abandoned at the deadline, so a
// hanging login shell cannot hold the start.
func probeConfigDir(ctx context.Context, e Env, timeout time.Duration) (string, error) {
	if e.LoginEnv == nil {
		return "", refuse("claude", DetailPTYFailed)
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	type answer struct {
		dir string
		err error
	}
	done := make(chan answer, 1) // buffered: an abandoned probe still finishes its send
	go func() {
		_, dir, err := e.LoginEnv(ctx, timeout)
		done <- answer{dir, err}
	}()
	select {
	case a := <-done:
		if a.err != nil || ctx.Err() != nil {
			return "", refuse("claude", DetailPTYFailed)
		}
		return a.dir, nil
	case <-ctx.Done():
		return "", refuse("claude", DetailPTYFailed)
	}
}
