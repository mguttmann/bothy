// SPDX-License-Identifier: Apache-2.0

package profiles

import (
	"path/filepath"
	"strings"

	"github.com/OWNER/bothy/services/host/internal/hostos"
)

// forbiddenWindows are variables that select a login or a provider of the coding agent (09 18.3).
// On Linux and macOS they are not on the allow list; on Windows the environment block is the base,
// so the profile claude with organization removes them from its copy of the block.
var forbiddenWindows = []string{
	"ANTHROPIC_AUTH_TOKEN", "CLAUDE_CODE_OAUTH_TOKEN", "ANTHROPIC_PROFILE",
	"CLAUDE_CODE_USE_BEDROCK", "CLAUDE_CODE_USE_VERTEX", "CLAUDE_CODE_USE_FOUNDRY",
}

// baseEnv returns the environment of every profile: on Linux and macOS the allow list (HOME,
// USER, LOGNAME, SHELL of the session user, the cleaned PATH, LANG, LC_* and TMPDIR of the helper)
// plus TERM; on Windows the environment block of the token plus TERM.
func baseEnv(e Env) []string {
	if e.GOOS == hostos.Windows {
		return setVar(e.GOOS, e.EnvBlock, "TERM", TERM)
	}
	env := []string{
		"HOME=" + e.Self.Home,
		"USER=" + e.Self.Name,
		"LOGNAME=" + e.Self.Name,
		"SHELL=" + e.Self.Shell,
		"PATH=" + CleanPath(e, lookup(e.GOOS, e.Environ, "PATH")),
	}
	seen := map[string]bool{}
	for _, kv := range e.Environ {
		k, _, ok := strings.Cut(kv, "=")
		if !ok || seen[k] || k != "LANG" && k != "TMPDIR" && !strings.HasPrefix(k, "LC_") {
			continue
		}
		// The first entry of a repeated key wins, as in the C library's getenv.
		seen[k] = true
		env = append(env, kv)
	}
	return append(env, "TERM="+TERM)
}

// CleanPath returns path with only the entries a profile may search: absolute folders that
// resolve as the session user (e.Resolver) and that neither group nor others may write (the mode
// bits of e.Security.Lstat of the resolved folder; on Windows the mode is not checked, the ACL of
// a system folder is HD10's matter). Empty entries, ".", relative entries and a second entry of
// the same resolved folder are removed; the kept entries stay as written, in order. Without
// e.Security nothing is kept.
func CleanPath(e Env, path string) string {
	sep := listSep(e.GOOS)
	var out []string
	seen := map[string]bool{}
	for _, p := range strings.Split(path, sep) {
		final, ok := searchable(e, p)
		if !ok {
			continue
		}
		key := final
		if e.GOOS == hostos.Windows {
			key = strings.ToLower(final)
		}
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, p)
	}
	return strings.Join(out, sep)
}

// searchable reports whether the PATH entry p may be searched and returns its resolved folder.
func searchable(e Env, p string) (string, bool) {
	if e.Security == nil || !isAbs(e.GOOS, p) {
		return "", false
	}
	r, err := e.Resolver.Resolve(p)
	if err != nil {
		return "", false
	}
	sec, err := e.Security.Lstat(r.Final)
	if err != nil || !sec.IsDir {
		return "", false
	}
	if e.GOOS != hostos.Windows && sec.Mode.Perm()&0o022 != 0 {
		return "", false
	}
	return r.Final, true
}

// listSep is the separator of PATH.
func listSep(goos string) string {
	if goos == hostos.Windows {
		return ";"
	}
	return ":"
}

// pathSep is the separator of the folders of a path.
func pathSep(goos string) string {
	if goos == hostos.Windows {
		return `\`
	}
	return "/"
}

// joinPath joins folders with the separator of goos, so the Windows rules are tested on every
// build host.
func joinPath(goos string, elem ...string) string {
	if goos != hostos.Windows {
		return filepath.Join(elem...)
	}
	parts := make([]string, 0, len(elem))
	for i, el := range elem {
		el = strings.ReplaceAll(el, "/", `\`)
		if i > 0 {
			el = strings.Trim(el, `\`)
		} else {
			el = strings.TrimRight(el, `\`)
		}
		if el != "" {
			parts = append(parts, el)
		}
	}
	return strings.Join(parts, `\`)
}

// isAbs reports whether p is absolute for goos: on Windows a drive with a separator or a UNC path.
func isAbs(goos, p string) bool {
	if goos != hostos.Windows {
		return strings.HasPrefix(p, "/")
	}
	if strings.HasPrefix(p, `\\`) {
		return len(p) > 2
	}
	return len(p) >= 3 && p[1] == ':' && (p[2] == '\\' || p[2] == '/') &&
		(p[0] >= 'A' && p[0] <= 'Z' || p[0] >= 'a' && p[0] <= 'z')
}

// sameKey compares names of variables; Windows ignores their case.
func sameKey(goos, a, b string) bool {
	if goos == hostos.Windows {
		return strings.EqualFold(a, b)
	}
	return a == b
}

// lookup returns the value of the first entry key in env, "" when it is absent.
func lookup(goos string, env []string, key string) string {
	for _, kv := range env {
		if k, v, ok := strings.Cut(kv, "="); ok && sameKey(goos, k, key) {
			return v
		}
	}
	return ""
}

// without returns a copy of env without every entry of the keys.
func without(goos string, env []string, keys ...string) []string {
	out := make([]string, 0, len(env))
	for _, kv := range env {
		k, _, _ := strings.Cut(kv, "=")
		drop := false
		for _, key := range keys {
			drop = drop || sameKey(goos, k, key)
		}
		if !drop {
			out = append(out, kv)
		}
	}
	return out
}

// setVar returns a copy of env with every entry of key replaced by one key=value at the end.
func setVar(goos string, env []string, key, value string) []string {
	return append(without(goos, env, key), key+"="+value)
}
