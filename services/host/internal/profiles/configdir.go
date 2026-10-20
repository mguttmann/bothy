// SPDX-License-Identifier: Apache-2.0

package profiles

import (
	"strings"

	"github.com/OWNER/bothy/services/host/internal/doctor"
	"github.com/OWNER/bothy/services/host/internal/hostos"
)

// orgConfigDir returns <home>/.config/bothy-host/claude (Windows %USERPROFILE%\.config\bothy-host\claude),
// the CLAUDE_CONFIG_DIR of the profile claude with organization, when it resolves as the session
// user to exactly that folder inside the resolved home; otherwise a *StartError with
// DetailSharedLocation. A link anywhere on the way that leads elsewhere, even inside the home,
// would let two places share the login, so only the exact folder passes.
func orgConfigDir(e Env) (string, error) {
	dir := joinPath(e.GOOS, e.Self.Home, doctor.OrgConfigDir)
	home, err := e.Resolver.Resolve(e.Self.Home)
	if err != nil {
		return "", refuse("claude", DetailSharedLocation)
	}
	r, err := e.Resolver.Resolve(dir)
	if err != nil || !samePath(e.GOOS, r.Final, joinPath(e.GOOS, home.Final, doctor.OrgConfigDir)) {
		return "", refuse("claude", DetailSharedLocation)
	}
	return dir, nil
}

// personalConfigDir checks the CLAUDE_CONFIG_DIR of the login shell for the profile claude with
// personal: value, or <home>/.claude when it is unset (""), must resolve as the session user to
// the home or a folder below it. A value outside, a link leading outside, a dangling link, a
// missing folder or a relative value gives a *StartError with DetailSharedLocation (TM-T29 (k)).
func personalConfigDir(e Env, value string) error {
	dir := value
	if dir == "" {
		dir = joinPath(e.GOOS, e.Self.Home, ".claude")
	}
	if !isAbs(e.GOOS, dir) {
		return refuse("claude", DetailSharedLocation)
	}
	home, err := e.Resolver.Resolve(e.Self.Home)
	if err != nil {
		return refuse("claude", DetailSharedLocation)
	}
	r, err := e.Resolver.Resolve(dir)
	if err != nil || !within(e.GOOS, home.Final, r.Final) {
		return refuse("claude", DetailSharedLocation)
	}
	return nil
}

// within reports whether p is root or lies below it; both are resolved paths.
func within(goos, root, p string) bool {
	if samePath(goos, root, p) {
		return true
	}
	prefix := strings.TrimSuffix(root, pathSep(goos)) + pathSep(goos)
	if goos == hostos.Windows {
		return strings.HasPrefix(strings.ToLower(p), strings.ToLower(prefix))
	}
	return strings.HasPrefix(p, prefix)
}

// samePath compares resolved paths; Windows ignores case.
func samePath(goos, a, b string) bool {
	if goos == hostos.Windows {
		return strings.EqualFold(a, b)
	}
	return a == b
}
