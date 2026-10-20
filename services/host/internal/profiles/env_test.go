// SPDX-License-Identifier: Apache-2.0

package profiles

import (
	"context"
	"io/fs"
	"maps"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/host/internal/hostos"
	"github.com/OWNER/bothy/services/host/internal/hostos/hostostest"
)

const (
	alexHome  = "/home/bothy-s-alex"
	alexRepo  = "/home/bothy-s-alex/repo"
	claudeBin = "/opt/claude/bin/claude"
	orgDir    = "/home/bothy-s-alex/.config/bothy-host/claude"
	apiKey    = "sk-example-organization-key"
)

var searchList = []string{"/opt/claude/bin", "/usr/bin", "/bin"}

// helperEnviron is the environment of the helper process: the allowed variables plus credentials
// and unrelated values that no profile may inherit.
var helperEnviron = []string{
	"HOME=/var/empty",
	"USER=someone",
	"PATH=/usr/local/bin:/opt/tools/bin:/usr/bin::.:relative:/missing:/bin:/usr/bin:/tmp:/usr/local/link",
	"LANG=en_US.UTF-8",
	"LC_ALL=C",
	"LC_CTYPE=C.UTF-8",
	"LCX=not-a-locale",
	"TMPDIR=/tmp/",
	"TERM=dumb",
	"SHELL=/bin/zsh",
	"CLAUDE_CODE_OAUTH_TOKEN=oauth-example",
	"ANTHROPIC_AUTH_TOKEN=auth-example",
	"ANTHROPIC_API_KEY=helper-key",
	"CLAUDE_CONFIG_DIR=/tmp/shared",
	"AWS_SECRET_ACCESS_KEY=aws-example",
	"UNRELATED=1",
}

// unixWorld is the session user bothy-s-alex on a Linux host with a few PATH folders.
func unixWorld(t *testing.T) (*hostostest.World, Env) {
	t.Helper()
	w := hostostest.New("linux")
	w.AddUser("bothy-s-alex", "1001", alexHome, true)
	w.AddUser("bothy-s-sam", "1002", "/home/bothy-s-sam", true)
	w.Dir(alexHome, "1001", 0o700)
	w.Dir(alexRepo, "1001", 0o700)
	w.Dir(orgDir, "1001", 0o700)
	w.Dir("/home/bothy-s-sam", "1002", 0o700)
	w.Dir("/usr/bin", "0", 0o755)
	w.Dir("/bin", "0", 0o755)
	w.Dir("/usr/local/bin", "0", 0o755)
	w.Dir("/opt/tools/bin", "0", 0o775)
	w.Dir("/opt/claude/bin", "0", 0o755)
	w.Dir("/tmp", "0", 0o1777)
	w.Dir("/srv/real", "0", 0o755)
	w.Link("/usr/local/link", "0", "/srv/real")
	w.File("/bin/bash", "0", 0o755, "")
	w.File("/bin/sh", "0", 0o755, "")
	w.File(claudeBin, "0", 0o755, "")
	w.File("/opt/tool/bin/tool", "0", 0o755, "")
	w.Link("/usr/local/bin/tool", "0", "/opt/tool/bin/tool")
	a, err := w.Users().Lookup("bothy-s-alex")
	if err != nil {
		t.Fatal(err)
	}
	a.Shell = "/bin/bash"
	return w, Env{GOOS: "linux", Self: a, Environ: slices.Clone(helperEnviron), Resolver: w.As("bothy-s-alex"), Security: w.Security(),
		Shells: []string{"# /etc/shells: valid login shells", "/bin/sh", " /bin/bash ", ""}}
}

// unixAllowed is the allow list built from helperEnviron for bothy-s-alex.
func unixAllowed() map[string]string {
	return map[string]string{
		"HOME":     alexHome,
		"USER":     "bothy-s-alex",
		"LOGNAME":  "bothy-s-alex",
		"SHELL":    "/bin/bash",
		"PATH":     "/usr/local/bin:/usr/bin:/bin:/usr/local/link",
		"LANG":     "en_US.UTF-8",
		"LC_ALL":   "C",
		"LC_CTYPE": "C.UTF-8",
		"TMPDIR":   "/tmp/",
		"TERM":     "xterm-256color",
	}
}

// envMap turns an environment into a map and fails on a repeated key (case-insensitive on Windows).
func envMap(t *testing.T, goos string, env []string) map[string]string {
	t.Helper()
	out := map[string]string{}
	seen := map[string]bool{}
	for _, kv := range env {
		k, v, ok := strings.Cut(kv, "=")
		if !ok || k == "" {
			t.Fatalf("malformed entry %q", kv)
		}
		norm := k
		if goos == hostos.Windows {
			norm = strings.ToUpper(k)
		}
		if seen[norm] {
			t.Fatalf("key %s twice in %v", k, env)
		}
		seen[norm] = true
		out[k] = v
	}
	return out
}

func sameEnv(t *testing.T, goos string, got []string, want map[string]string) {
	t.Helper()
	g := envMap(t, goos, got)
	if !maps.Equal(g, want) {
		gk := slices.Sorted(maps.Keys(g))
		wk := slices.Sorted(maps.Keys(want))
		t.Errorf("environment\n got keys %v\nwant keys %v\n got %v\nwant %v", gk, wk, g, want)
	}
}

// winFS is a fake Windows file system: resolved paths and their security.
type winFS struct {
	final map[string]string
	sec   map[string]hostos.FileSec
}

func (f winFS) Resolve(p string) (hostos.Resolved, error) {
	if to, ok := f.final[p]; ok {
		return hostos.Resolved{Final: to}, nil
	}
	return hostos.Resolved{}, hostos.ErrUnresolvable
}

func (f winFS) Identity(p string) (string, error) {
	r, err := f.Resolve(p)
	return r.Final, err
}

func (f winFS) Lstat(p string) (hostos.FileSec, error) {
	if s, ok := f.sec[p]; ok {
		return s, nil
	}
	return hostos.FileSec{}, fs.ErrNotExist
}

const (
	winHome   = `C:\Users\bothy-s-alex`
	winRepo   = `C:\Users\bothy-s-alex\repo`
	winOrgDir = `C:\Users\bothy-s-alex\.config\bothy-host\claude`
	winClaude = `C:\Program Files\Claude\claude.exe`
	winShell  = `C:\Windows\System32\WindowsPowerShell\v1.0\powershell.exe`
)

var winSearch = []string{`C:\Program Files\Claude`, `C:\Windows\System32`}

// windowsEnv is the session user bothy-s-alex on Windows with the environment block of its token.
func windowsEnv() Env {
	dirs := []string{winHome, winRepo, winOrgDir, `C:\Windows\System32`, `C:\Windows`, `C:\Users\bothy-s-alex\bin`, `C:\Program Files\Claude`, winHome + `\.claude`}
	f := winFS{final: map[string]string{}, sec: map[string]hostos.FileSec{}}
	for _, d := range dirs {
		f.final[d] = d
		// Mode bits mean nothing on Windows; a value that would fail on Unix proves they are skipped.
		f.sec[d] = hostos.FileSec{IsDir: true, Mode: 0o777}
	}
	f.final[winClaude] = winClaude
	f.sec[winClaude] = hostos.FileSec{Mode: 0o777}
	return Env{
		GOOS:     hostos.Windows,
		Self:     hostos.Account{Name: "bothy-s-alex", ID: "S-1-5-21-1-2-3-1001", Home: winHome, Person: true},
		Environ:  slices.Clone(helperEnviron),
		Resolver: f,
		Security: f,
		EnvBlock: []string{
			`USERPROFILE=` + winHome,
			`USERNAME=bothy-s-alex`,
			`Path=C:\Windows\System32;relative;C:\Users\bothy-s-alex\bin;C:\Windows\System32;C:\missing`,
			`SystemRoot=C:\Windows`,
			`TERM=dumb`,
			`ANTHROPIC_AUTH_TOKEN=auth-example`,
			`CLAUDE_CODE_OAUTH_TOKEN=oauth-example`,
			`anthropic_profile=work`,
			`CLAUDE_CODE_USE_BEDROCK=1`,
			`CLAUDE_CODE_USE_VERTEX=1`,
			`CLAUDE_CODE_USE_FOUNDRY=1`,
			`Claude_Config_Dir=C:\shared`,
			`UNRELATED=1`,
		},
		SystemRoot: `C:\Windows`,
	}
}

// windowsBlock is the environment block of windowsEnv as a map, with TERM replaced.
func windowsBlock() map[string]string {
	m := map[string]string{}
	for _, kv := range windowsEnv().EnvBlock {
		k, v, _ := strings.Cut(kv, "=")
		m[k] = v
	}
	m["TERM"] = TERM
	return m
}

func TestEnvShell(t *testing.T) {
	_, e := unixWorld(t)
	s, err := Shell(e)
	if err != nil {
		t.Fatal(err)
	}
	if s.Path != "/bin/bash" || s.Argv0 != "-bash" || len(s.Args) != 0 || s.Dir != alexHome {
		t.Errorf("spec %+v", s)
	}
	sameEnv(t, "linux", s.Env, unixAllowed())

	// A login shell that is not absolute, missing or not listed refuses the start.
	for _, shell := range []string{"bash", "", "/bin/zsh", "/usr/local/bin/tool"} {
		_, e := unixWorld(t)
		e.Self.Shell = shell
		if _, err := Shell(e); detailOf(err) != DetailPTYFailed {
			t.Errorf("shell %q: %v", shell, err)
		}
	}
	w, e := unixWorld(t)
	w.Remove("/bin/bash")
	if _, err := Shell(e); detailOf(err) != DetailPTYFailed {
		t.Errorf("missing listed shell: %v", err)
	}
	_, e = unixWorld(t)
	e.Shells = []string{"#/bin/bash"}
	if _, err := Shell(e); detailOf(err) != DetailPTYFailed {
		t.Errorf("commented shell: %v", err)
	}

	// Without PATH or locale in the helper's environment only the fixed keys remain.
	_, e = unixWorld(t)
	e.Environ = []string{"UNRELATED=1"}
	s, err = Shell(e)
	if err != nil {
		t.Fatal(err)
	}
	sameEnv(t, "linux", s.Env, map[string]string{"HOME": alexHome, "USER": "bothy-s-alex", "LOGNAME": "bothy-s-alex", "SHELL": "/bin/bash", "PATH": "", "TERM": TERM})

	// Windows: PowerShell of the system root with the environment block of the token plus TERM.
	we := windowsEnv()
	s, err = Shell(we)
	if err != nil {
		t.Fatal(err)
	}
	if s.Path != winShell || s.Argv0 != "powershell.exe" || !slices.Equal(s.Args, []string{"-NoLogo"}) || s.Dir != winHome {
		t.Errorf("windows spec %+v", s)
	}
	sameEnv(t, hostos.Windows, s.Env, windowsBlock())
	we.SystemRoot = ""
	if _, err := Shell(we); detailOf(err) != DetailPTYFailed {
		t.Errorf("windows without SystemRoot: %v", err)
	}
	we = windowsEnv()
	we.SystemRoot = `Windows`
	if _, err := Shell(we); detailOf(err) != DetailPTYFailed {
		t.Errorf("windows with a relative SystemRoot: %v", err)
	}
}

func TestEnvCommand(t *testing.T) {
	_, e := unixWorld(t)
	args := []string{"--flag", "a b"}
	s, err := Command(e, "/usr/local/bin/tool", args)
	if err != nil {
		t.Fatal(err)
	}
	if s.Path != "/opt/tool/bin/tool" || s.Argv0 != "/usr/local/bin/tool" || !slices.Equal(s.Args, args) || s.Dir != alexHome {
		t.Errorf("spec %+v", s)
	}
	args[0] = "changed"
	if s.Args[0] != "--flag" {
		t.Error("the arguments alias the caller's slice")
	}
	sameEnv(t, "linux", s.Env, unixAllowed())
	for _, p := range []string{"/usr/local/bin/gone", "tool", "", "../bin/tool"} {
		if _, err := Command(e, p, nil); detailOf(err) != DetailPTYFailed {
			t.Errorf("program %q: %v", p, err)
		}
	}

	we := windowsEnv()
	s, err = Command(we, winClaude, []string{"--version"})
	if err != nil {
		t.Fatal(err)
	}
	if s.Path != winClaude || s.Dir != winHome || !slices.Equal(s.Args, []string{"--version"}) {
		t.Errorf("windows spec %+v", s)
	}
	sameEnv(t, hostos.Windows, s.Env, windowsBlock())
	if _, err := Command(we, `relative\tool.exe`, nil); detailOf(err) != DetailPTYFailed {
		t.Errorf("windows relative program: %v", err)
	}
}

func TestEnvClaudeOrganization(t *testing.T) {
	_, e := unixWorld(t)
	s, err := ClaudeOrganization(e, claudeBin, searchList, alexRepo, apiKey)
	if err != nil {
		t.Fatal(err)
	}
	if s.Path != claudeBin || s.Dir != alexRepo || len(s.Args) != 0 {
		t.Errorf("spec %+v", s)
	}
	want := unixAllowed()
	want["PATH"] = "/opt/claude/bin:/usr/bin:/bin:/usr/local/bin:/usr/local/link"
	want["ANTHROPIC_API_KEY"] = apiKey
	want["CLAUDE_CONFIG_DIR"] = orgDir
	sameEnv(t, "linux", s.Env, want)
	if !strings.HasPrefix(envMap(t, "linux", s.Env)["PATH"], strings.Join(searchList, ":")+":") {
		t.Error("PATH does not start with the search list")
	}

	// The folder must resolve to exactly <home>/.config/bothy-host/claude.
	cases := map[string]func(w *hostostest.World){
		"missing": func(w *hostostest.World) { w.Remove(orgDir) },
		"link outside": func(w *hostostest.World) {
			w.Remove(orgDir)
			w.Dir("/srv/shared", "0", 0o777)
			w.Link(orgDir, "1001", "/srv/shared")
		},
		"link inside to another folder": func(w *hostostest.World) {
			w.Remove(orgDir)
			w.Link(orgDir, "1001", alexRepo)
		},
		"parent link outside": func(w *hostostest.World) {
			w.Remove(alexHome + "/.config")
			w.Dir("/srv/cfg/bothy-host/claude", "1001", 0o700)
			w.Link(alexHome+"/.config", "1001", "/srv/cfg")
		},
		"dangling link": func(w *hostostest.World) {
			w.Remove(orgDir)
			w.Link(orgDir, "1001", "/nowhere")
		},
	}
	for name, mutate := range cases {
		w, e := unixWorld(t)
		mutate(w)
		if _, err := ClaudeOrganization(e, claudeBin, searchList, alexRepo, apiKey); detailOf(err) != DetailSharedLocation {
			t.Errorf("%s: %v", name, err)
		}
	}
	// A home reached through a link still holds the folder at exactly its place.
	w, e := unixWorld(t)
	w.Link("/users/alex", "0", alexHome)
	e.Self.Home = "/users/alex"
	s, err = ClaudeOrganization(e, claudeBin, searchList, alexRepo, apiKey)
	if err != nil {
		t.Fatalf("home through a link: %v", err)
	}
	if envMap(t, "linux", s.Env)["CLAUDE_CONFIG_DIR"] != "/users/alex/.config/bothy-host/claude" {
		t.Errorf("config dir %v", s.Env)
	}
	_, e = unixWorld(t)
	if _, err := ClaudeOrganization(e, "", searchList, alexRepo, apiKey); detailOf(err) != DetailPTYFailed {
		t.Errorf("empty claude_bin: %v", err)
	}
	if _, err := ClaudeOrganization(e, claudeBin, searchList, alexRepo, ""); detailOf(err) != DetailPTYFailed {
		t.Errorf("empty key: %v", err)
	}

	// Windows: the block without the forbidden variables, PATH, key and folder replaced, TERM.
	we := windowsEnv()
	s, err = ClaudeOrganization(we, winClaude, winSearch, winRepo, apiKey)
	if err != nil {
		t.Fatal(err)
	}
	if s.Path != winClaude || s.Dir != winRepo {
		t.Errorf("windows spec %+v", s)
	}
	wb := windowsBlock()
	for _, k := range []string{"ANTHROPIC_AUTH_TOKEN", "CLAUDE_CODE_OAUTH_TOKEN", "anthropic_profile", "CLAUDE_CODE_USE_BEDROCK", "CLAUDE_CODE_USE_VERTEX", "CLAUDE_CODE_USE_FOUNDRY", "Claude_Config_Dir", "Path"} {
		delete(wb, k)
	}
	wb["PATH"] = `C:\Program Files\Claude;C:\Windows\System32;C:\Users\bothy-s-alex\bin`
	wb["ANTHROPIC_API_KEY"] = apiKey
	wb["CLAUDE_CONFIG_DIR"] = winOrgDir
	sameEnv(t, hostos.Windows, s.Env, wb)
	we.Resolver = winFS{final: map[string]string{winHome: winHome, winOrgDir: `D:\shared`}}
	if _, err := ClaudeOrganization(we, winClaude, winSearch, winRepo, apiKey); detailOf(err) != DetailSharedLocation {
		t.Errorf("windows folder elsewhere: %v", err)
	}
}

// probe returns a LoginEnv that answers dir (set when dir is not empty).
func probe(dir string) func(context.Context, time.Duration) (bool, string, error) {
	return func(context.Context, time.Duration) (bool, string, error) { return true, dir, nil }
}

func TestEnvClaudePersonal(t *testing.T) {
	w, e := unixWorld(t)
	w.Dir(alexHome+"/cfg", "1001", 0o700)
	e.LoginEnv = probe(alexHome + "/cfg")
	s, err := ClaudePersonal(context.Background(), e, alexRepo, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if s.Path != "/bin/bash" || s.Argv0 != "-bash" || s.Dir != alexRepo || !slices.Equal(s.Args, PersonalCommand("bash", alexHome+"/cfg", true)) {
		t.Errorf("spec %+v", s)
	}
	// No key, no folder and no token in the environment: the login shell sets the folder itself.
	sameEnv(t, "linux", s.Env, unixAllowed())

	we := windowsEnv()
	we.LoginEnv = probe("")
	s, err = ClaudePersonal(context.Background(), we, winRepo, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if s.Path != winShell || s.Argv0 != "powershell.exe" || s.Dir != winRepo || !slices.Equal(s.Args, PersonalCommand("powershell", "", false)) {
		t.Errorf("windows spec %+v", s)
	}
	m := envMap(t, hostos.Windows, s.Env)
	if _, ok := m["ANTHROPIC_API_KEY"]; ok {
		t.Error("a key with personal")
	}
	sameEnv(t, hostos.Windows, s.Env, windowsBlock())
}

func TestCleanPath(t *testing.T) {
	_, e := unixWorld(t)
	tests := []struct{ in, want string }{
		{"", ""},
		{"/usr/bin", "/usr/bin"},
		{"/usr/bin:/usr/bin:/bin", "/usr/bin:/bin"},
		{".:/usr/bin::relative:./bin", "/usr/bin"},
		{"/missing:/bin", "/bin"},
		{"/opt/tools/bin:/bin", "/bin"},                  // group writable
		{"/tmp:/bin", "/bin"},                            // writable by others (sticky does not help)
		{"/bin/bash:/bin", "/bin"},                       // a file, not a folder
		{"/usr/local/link:/bin", "/usr/local/link:/bin"}, // a link to a good folder stays as written
		{"/usr/bin/:/usr/bin", "/usr/bin/"},              // the same folder written twice
	}
	for _, tc := range tests {
		if got := CleanPath(e, tc.in); got != tc.want {
			t.Errorf("CleanPath(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
	w, e := unixWorld(t)
	w.Link("/usr/local/bad", "0", "/opt/tools/bin")
	if got := CleanPath(e, "/usr/local/bad:/bin"); got != "/bin" {
		t.Errorf("link to a writable folder: %q", got)
	}
	e.Security = nil
	if got := CleanPath(e, "/usr/bin"); got != "" {
		t.Errorf("without Security: %q", got)
	}

	we := windowsEnv()
	if got := CleanPath(we, `C:\Windows\System32;;.;relative;C:\Users\bothy-s-alex\bin;c:\windows\system32;C:\missing`); got != `C:\Windows\System32;C:\Users\bothy-s-alex\bin` {
		t.Errorf("windows %q", got)
	}
}
