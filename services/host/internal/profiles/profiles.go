// SPDX-License-Identifier: Apache-2.0

package profiles

import (
	"context"
	"slices"
	"time"

	"github.com/OWNER/bothy/services/host/internal/config"
	"github.com/OWNER/bothy/services/host/internal/doctor"
	"github.com/OWNER/bothy/services/host/internal/hostos"
	"github.com/OWNER/bothy/services/host/internal/state"
)

// Close details of 24 10.3 that a start can produce.
const (
	// DetailSharedLocation: a CLAUDE_CONFIG_DIR does not resolve to its place inside the home.
	DetailSharedLocation = "shared_location"
	// DetailPTYFailed: the program, the login shell or the probe of the login shell failed.
	DetailPTYFailed = "pty_failed"
)

// TERM is the value of TERM for every profile: the renderer of the clients is xterm (24 12). It
// is a protocol constant, not a configuration value.
const TERM = "xterm-256color"

// PersonalLoginDocumented is the result of OQ-03 (Ruling T10, RK-30), checked on 2026-10-20: the
// provider documents the holder's own login in the account of the session user (path A, a token
// of "claude setup-token" in CLAUDE_CODE_OAUTH_TOKEN, and path B, "/login" with the credentials in
// the home or the keychain), so the profile claude is offered with the declaration personal. Set
// to false if a later check finds neither path documented; Offered then drops claude for personal.
const PersonalLoginDocumented = true

// StartError is a refused start. Detail is DetailSharedLocation or DetailPTYFailed; Op names the
// operation. The text names only these two, never a path, a value or a key.
type StartError struct{ Op, Detail string }

// Error returns "profiles: <op>: <detail>".
func (e *StartError) Error() string { return "profiles: " + e.Op + ": " + e.Detail }

func refuse(op, detail string) error { return &StartError{Op: op, Detail: detail} }

// Offered returns the profiles the host offers (HostProfile items), for the report and for step 9
// of the token check. res is the latest result of doctor: shell and every command that HD7 kept
// (res.Profiles) are offered; claude only with the declaration organization and a usable
// host_service.claude_bin (res.ClaudeBin not empty), or with personal while
// PersonalLoginDocumented, and in both cases only when at least one session user has a working
// directory after HD7 (res.Workdirs). With none, an unknown or any other declaration there is no
// claude. A nil res gives the empty list; the result is never nil and never aliases res.
func Offered(res *doctor.Result) []doctor.Profile { return offered(res, PersonalLoginDocumented) }

// offered is Offered with the OQ-03 result as a parameter, so both branches are tested.
func offered(res *doctor.Result, personalDocumented bool) []doctor.Profile {
	out := []doctor.Profile{}
	if res == nil {
		return out
	}
	claude := hasWorkdir(res.Workdirs) && (res.Declaration == state.DeclOrganization && res.ClaudeBin != "" ||
		res.Declaration == state.DeclPersonal && personalDocumented)
	for _, p := range res.Profiles {
		switch p.Kind {
		case config.KindShell, config.KindCommand:
		case config.KindClaude:
			if !claude {
				continue
			}
		default:
			continue // a kind this host cannot start is never offered
		}
		out = append(out, p)
	}
	return out
}

func hasWorkdir(w map[string][]doctor.Workdir) bool {
	for _, dirs := range w {
		if len(dirs) > 0 {
			return true
		}
	}
	return false
}

// Env is what the PTY helper knows about its session user and its own process for building a
// start.
type Env struct {
	// GOOS is the operating system (hostos.Linux, hostos.Darwin or hostos.Windows).
	GOOS string
	// Self is the session user of the helper: Name, Home and the login Shell.
	Self hostos.Account
	// Environ is the helper's own environment (os.Environ()); only PATH, LANG, LC_* and TMPDIR are
	// taken from it, and only on Linux and macOS.
	Environ []string
	// Resolver resolves paths as the session user.
	Resolver hostos.Resolver
	// Security reads the mode of the folders of PATH; without it no folder of PATH is kept.
	Security hostos.Security
	// Shells are the lines of /etc/shells (Linux, macOS); the caller reads the file.
	Shells []string
	// LoginEnv is the probe of the login shell of HD6 (doctor.HelperEnv.LoginEnv): whether
	// CLAUDE_CODE_OAUTH_TOKEN is set and the value of CLAUDE_CONFIG_DIR, "" when unset.
	LoginEnv func(ctx context.Context, timeout time.Duration) (bool, string, error)
	// EnvBlock is, on Windows only, the environment block of the helper's token without
	// inheritance (TokenEnvironment); nil elsewhere.
	EnvBlock []string
	// SystemRoot is %SystemRoot% on Windows.
	SystemRoot string
}

// Spec is one start: the program, argv[0], the arguments after argv[0], the complete environment
// and the working directory.
type Spec struct {
	Path, Argv0 string
	Args, Env   []string
	Dir         string
}

// Shell returns the start of the profile shell: the login shell of the session user in its home.
// On Linux and macOS that is Self.Shell with argv[0] "-" plus its base name (login(1)); a shell
// that is not absolute, cannot be resolved or is not listed in Shells gives a *StartError with
// DetailPTYFailed. On Windows it is PowerShell of %SystemRoot% with -NoLogo and the environment
// block plus TERM.
func Shell(e Env) (Spec, error) {
	s, err := loginShell(e, "shell")
	if err != nil {
		return Spec{}, err
	}
	if e.GOOS == hostos.Windows {
		s.Args = []string{powershellNoLogo}
	}
	s.Dir = e.Self.Home
	s.Env = baseEnv(e)
	return s, nil
}

// Command returns the start of a command profile: program resolved again as the session user
// (the path HD11 checked) with a copy of args, argv[0] the configured program, the home as working
// directory and the allow list. A program that is not absolute or cannot be resolved gives a
// *StartError with DetailPTYFailed.
func Command(e Env, program string, args []string) (Spec, error) {
	if !isAbs(e.GOOS, program) {
		return Spec{}, refuse("command", DetailPTYFailed)
	}
	r, err := e.Resolver.Resolve(program)
	if err != nil {
		return Spec{}, refuse("command", DetailPTYFailed)
	}
	return Spec{Path: r.Final, Argv0: program, Args: slices.Clone(args), Env: baseEnv(e), Dir: e.Self.Home}, nil
}
