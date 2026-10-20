// SPDX-License-Identifier: Apache-2.0

//go:build platform && (linux || darwin)

package platform

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

// The commands the suite types into a PTY on Linux and macOS (bash and zsh, the login shells
// install gives the session users). Each regular expression needs digits or a word where the
// typed command line has a quote or a dollar sign, so the echo of the input never matches.
const (
	enter       = "\n"
	identityCmd = `echo "UID=$(id -u)"`
	parentCmd   = `echo "PP=$PPID"`
	shellPIDCmd = `echo "SH=$$"`
	sizeCmd     = `echo "SZ=$(stty size)"`
)

var (
	identityRe = regexp.MustCompile(`UID=(\d+)`)
	parentRe   = regexp.MustCompile(`PP=(\d+)`)
	shellPIDRe = regexp.MustCompile(`SH=(\d+)`)
	sizeRe     = regexp.MustCompile(`SZ=(\d+) (\d+)`)
	authRe     = regexp.MustCompile(`"authMethod": "(\w[\w.]*)"`)
	listRe     = regexp.MustCompile(`LS=(\d+)`)
	loginRe    = regexp.MustCompile(`Login (successful)`)
	// backgroundCmds start a nohup child and a child that left the session with setsid (perl is on
	// every hosted runner); each prints its process ID.
	backgroundCmds = []string{
		`nohup sleep 3000 >/dev/null 2>&1 & echo "BG=$!"`,
		`perl -e 'use POSIX; if (fork) { exit 0 } POSIX::setsid(); print "SID=$$\n"; exec "sleep", "3001"'; sleep 1`,
	}
	backgroundRes = []*regexp.Regexp{regexp.MustCompile(`BG=(\d+)`), regexp.MustCompile(`SID=(\d+)`)}
)

// wantIdentity returns what identityCmd prints for user.
func wantIdentity(t *testing.T, user string) string {
	u, _ := uid(t, user)
	return strconv.Itoa(u)
}

// listCmd tries to list dir and prints LS=0 when it could.
func listCmd(dir string) string { return `ls '` + dir + `' >/dev/null 2>&1; echo "LS=$?"` }

// claudeCmd runs claude with args from dir first in PATH, as a person runs her own claude.
func claudeCmd(dir string, args ...string) string {
	return `PATH='` + dir + `':"$PATH" claude ` + strings.Join(args, " ")
}

// pidAlive reports whether pid exists and is no zombie.
func pidAlive(pid int) bool {
	out, err := exec.Command("/bin/ps", "-o", "stat=", "-p", strconv.Itoa(pid)).Output() //nolint:gosec // a number
	if err != nil {
		return false
	}
	st := strings.TrimSpace(string(out))
	return st != "" && !strings.HasPrefix(st, "Z")
}

// loginEnv is what the login shell of a session user exports (24 7.4: the probe of HD6 and the
// start of claude with personal read it).
type loginEnv struct {
	// PathDir goes first into PATH.
	PathDir string
	// ConfigDir is CLAUDE_CONFIG_DIR; empty leaves it unset.
	ConfigDir string
	// TokenFile is a file whose content the shell exports as CLAUDE_CODE_OAUTH_TOKEN.
	TokenFile string
}

// loginFile is the startup file of the login shell install gives a session user.
func loginFile(user string) string {
	if runtime.GOOS == "darwin" {
		return filepath.Join(home(user), ".zprofile")
	}
	return filepath.Join(home(user), ".bash_profile")
}

// setLoginEnv writes the startup file of the login shell of user.
func setLoginEnv(t *testing.T, user string, e loginEnv) {
	t.Helper()
	var b strings.Builder
	if e.PathDir != "" {
		b.WriteString("export PATH='" + e.PathDir + "':\"$PATH\"\n")
	}
	if e.ConfigDir != "" {
		b.WriteString("export CLAUDE_CONFIG_DIR='" + e.ConfigDir + "'\n")
	}
	if e.TokenFile != "" {
		b.WriteString("export CLAUDE_CODE_OAUTH_TOKEN=\"$(cat '" + e.TokenFile + "')\"\n")
	}
	p := loginFile(user)
	if err := os.WriteFile(p, []byte(b.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	chownTo(t, p, user)
	t.Cleanup(func() { _ = os.Remove(p) })
}

// writeSecretOf writes a file only user can read with content.
func writeSecretOf(t *testing.T, user, name, content string) string {
	t.Helper()
	p := filepath.Join(home(user), name)
	if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	chownTo(t, p, user)
	t.Cleanup(func() { _ = os.Remove(p) })
	return p
}

// symlinkOf creates the link p to target owned by user.
func symlinkOf(t *testing.T, user, target, p string) {
	t.Helper()
	_ = os.Remove(p)
	if err := os.Symlink(target, p); err != nil {
		t.Fatal(err)
	}
	chownTo(t, p, user)
	t.Cleanup(func() { _ = os.Remove(p) })
}

// revokeWrite removes what makeWritableBy granted.
func revokeWrite(t *testing.T, p, user string) {
	t.Helper()
	switch runtime.GOOS {
	case "linux":
		sh(t, "/usr/bin/setfacl", "-x", "u:"+user, p)
	case "darwin":
		sh(t, "/bin/chmod", "-a", "user:"+user+" allow write,append,add_file,add_subdirectory,delete_child", p)
	}
}

// lockUser locks the account on the machine only (the server side of the assignment is
// untouched).
func lockUser(t *testing.T, user string) {
	t.Helper()
	if runtime.GOOS == "darwin" {
		sh(t, "/usr/bin/pwpolicy", "-u", user, "-disableuser")
		return
	}
	sh(t, "/usr/sbin/usermod", "-L", user)
}

// stopService stops the host service through its service manager.
func stopService(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "darwin" {
		_, _ = try("/bin/launchctl", "bootout", "system/bothy-host")
		return
	}
	sh(t, "/usr/bin/systemctl", "stop", "bothy-host.service")
}

// startService starts the host service through its service manager.
func startService(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "darwin" {
		_, _ = try("/bin/launchctl", "bootstrap", "system", "/Library/LaunchDaemons/bothy-host.plist")
		return
	}
	_, _ = try("/usr/bin/systemctl", "start", "bothy-host.service")
}

// helperPID returns the main process of the PTY helper of user.
func helperPID(t *testing.T, user string) int {
	t.Helper()
	if runtime.GOOS == "darwin" {
		out, _ := try("/bin/launchctl", "print", "system/bothy-host.helper."+user)
		return atoiOr(pidFrom(out))
	}
	return atoiOr(sh(t, "/usr/bin/systemctl", "show", "--property=MainPID", "--value", "bothy-host-helper-"+user+".service"))
}

// startAs starts args as user in the background and returns its stop.
func startAs(t *testing.T, user string, args ...string) func() {
	t.Helper()
	cmd := exec.Command("/usr/bin/sudo", append([]string{"-n", "-u", user, "--"}, args...)...) //nolint:gosec // commands of the suite
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	stop := func() {
		// sudo hands SIGTERM to its child; SIGKILL would leave the child running.
		_ = cmd.Process.Signal(syscall.SIGTERM)
		done := make(chan struct{})
		go func() { _ = cmd.Wait(); close(done) }()
		select {
		case <-done:
		case <-time.After(10 * time.Second):
			_ = cmd.Process.Kill()
			<-done
		}
	}
	t.Cleanup(stop)
	return stop
}

// consoleOwner returns the user who owns the console session, or "" without one: macOS the owner
// of /dev/console, Linux the user of the active session on seat0 (24 6.4, Design note 15 a).
func consoleOwner() string {
	if runtime.GOOS == "darwin" {
		out, code := try("/usr/bin/stat", "-f", "%Su", "/dev/console")
		if u := strings.TrimSpace(out); code == 0 && u != "root" {
			return u
		}
		return ""
	}
	s, code := try("/usr/bin/loginctl", "show-seat", "seat0", "--property=ActiveSession", "--value")
	if code != 0 || strings.TrimSpace(s) == "" {
		return ""
	}
	u, code := try("/usr/bin/loginctl", "show-session", strings.TrimSpace(s), "--property=Name", "--value")
	if code != 0 {
		return ""
	}
	return strings.TrimSpace(u)
}

// serviceLog returns the log of the host service and whether the platform keeps one the suite
// can read (Linux: the journal; launchd keeps no log of the unit).
func serviceLog() (string, bool) {
	if runtime.GOOS != "linux" {
		return "", false
	}
	out, code := try("/usr/bin/journalctl", "--unit=bothy-host.service", "--no-pager", "--output=cat")
	return out, code == 0
}

// normalIdentity trims the printed user ID.
func normalIdentity(s string) string { return strings.TrimSpace(s) }

// openForAll creates p, if missing, writable by every user: the start log, which every session
// user's fake claude appends to.
func openForAll(p string) error {
	f, err := os.OpenFile(p, os.O_CREATE|os.O_WRONLY, 0o666)
	if err != nil {
		return err
	}
	_ = f.Close()
	return os.Chmod(p, 0o666)
}
