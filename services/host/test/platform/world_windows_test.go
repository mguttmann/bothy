// SPDX-License-Identifier: Apache-2.0

//go:build platform && windows

package platform

import (
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// The commands the suite types into a PTY on Windows (PowerShell, the login shell of Design note
// 9). Each regular expression needs digits or a word where the typed command line has a quote, so
// the echo of the input never matches.
const (
	enter       = "\r"
	identityCmd = `'WHO=' + (whoami)`
	parentCmd   = `'PP=' + (Get-CimInstance Win32_Process -Filter "ProcessId=$PID").ParentProcessId`
	shellPIDCmd = `'SH=' + $PID`
	sizeCmd     = `'SZ=' + [Console]::WindowHeight + ' ' + [Console]::WindowWidth`
)

var (
	identityRe = regexp.MustCompile(`WHO=([^\s'"]+)`)
	parentRe   = regexp.MustCompile(`PP=(\d+)`)
	shellPIDRe = regexp.MustCompile(`SH=(\d+)`)
	sizeRe     = regexp.MustCompile(`SZ=(\d+) (\d+)`)
	authRe     = regexp.MustCompile(`"authMethod": "(\w[\w.]*)"`)
	listRe     = regexp.MustCompile(`LS=(\d+)`)
	loginRe    = regexp.MustCompile(`Login (successful)`)
	// backgroundCmds start a hidden child and a child of cmd.exe; each prints its process ID.
	backgroundCmds = []string{
		`$p = Start-Process -PassThru -WindowStyle Hidden -FilePath ping.exe -ArgumentList '-n','3000','127.0.0.1'; 'BG=' + $p.Id`,
		`$q = Start-Process -PassThru -WindowStyle Hidden -FilePath cmd.exe -ArgumentList '/c','ping -n 3001 127.0.0.1'; 'SID=' + $q.Id`,
	}
	backgroundRes = []*regexp.Regexp{regexp.MustCompile(`BG=(\d+)`), regexp.MustCompile(`SID=(\d+)`)}
)

// wantIdentity returns what identityCmd prints for user, in lower case without the machine.
func wantIdentity(_ *testing.T, user string) string { return strings.ToLower(user) }

// normalIdentity reduces whoami output (machine\user) to the user.
func normalIdentity(s string) string {
	if _, u, ok := strings.Cut(s, `\`); ok {
		s = u
	}
	return strings.ToLower(strings.TrimSpace(s))
}

// listCmd tries to list dir and prints LS=0 when it could.
func listCmd(dir string) string {
	return `try { Get-ChildItem -LiteralPath '` + dir + `' -ErrorAction Stop | Out-Null; 'LS=' + 0 } catch { 'LS=' + 1 }`
}

// claudeCmd runs claude with args from dir first in PATH.
func claudeCmd(dir string, args ...string) string {
	return `$env:PATH = '` + dir + `;' + $env:PATH; claude ` + strings.Join(args, " ")
}

// pidAlive reports whether pid exists.
func pidAlive(pid int) bool {
	out, _ := try("tasklist", "/FI", "PID eq "+strconv.Itoa(pid), "/NH", "/FO", "CSV")
	return strings.Contains(out, `"`+strconv.Itoa(pid)+`"`)
}

// loginEnv is what the registry hive of a session user sets (the probe of HD6 on Windows reads
// it, Design note 9).
type loginEnv struct {
	PathDir   string
	ConfigDir string
	TokenFile string
}

// sidOf returns the SID of user.
func sidOf(t *testing.T, user string) string {
	t.Helper()
	out := sh(t, "powershell", "-NoProfile", "-Command",
		"(New-Object System.Security.Principal.NTAccount('"+user+"')).Translate([System.Security.Principal.SecurityIdentifier]).Value")
	return strings.TrimSpace(out)
}

// setLoginEnv writes the variables into the Environment key of the loaded hive of user (the
// helper service of the user keeps it loaded).
func setLoginEnv(t *testing.T, user string, e loginEnv) {
	t.Helper()
	key := `HKU\` + sidOf(t, user) + `\Environment`
	set := func(name, value string) {
		sh(t, "reg", "add", key, "/v", name, "/t", "REG_SZ", "/d", value, "/f")
		t.Cleanup(func() { _, _ = try("reg", "delete", key, "/v", name, "/f") })
	}
	if e.PathDir != "" {
		set("Path", e.PathDir)
	}
	if e.ConfigDir != "" {
		set("CLAUDE_CONFIG_DIR", e.ConfigDir)
	}
	if e.TokenFile != "" {
		b, err := os.ReadFile(e.TokenFile)
		if err != nil {
			t.Fatal(err)
		}
		set("CLAUDE_CODE_OAUTH_TOKEN", string(b))
	}
}

// writeSecretOf writes a file of user with content.
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

// symlinkOf creates the directory link p to target (a junction needs no privilege of the user).
func symlinkOf(t *testing.T, user, target, p string) {
	t.Helper()
	_ = os.Remove(p)
	sh(t, "cmd", "/c", "mklink", "/J", p, target)
	chownTo(t, p, user)
	t.Cleanup(func() { _ = os.Remove(p) })
}

// revokeWrite removes what makeWritableBy granted.
func revokeWrite(t *testing.T, p, user string) {
	t.Helper()
	sh(t, "icacls", p, "/remove:g", user)
}

// lockUser disables the account on the machine only.
func lockUser(t *testing.T, user string) {
	t.Helper()
	sh(t, "net", "user", user, "/active:no")
}

func stopService(t *testing.T) {
	t.Helper()
	_, _ = try("sc.exe", "stop", "bothy-host")
}

func startService(t *testing.T) {
	t.Helper()
	_, _ = try("sc.exe", "start", "bothy-host")
}

// helperPID returns the process of the helper service of user.
func helperPID(t *testing.T, user string) int {
	t.Helper()
	out, _ := try("sc.exe", "queryex", "bothy-host-helper-"+user)
	if i := strings.Index(out, "PID"); i >= 0 {
		if f := strings.Fields(out[i:]); len(f) >= 3 {
			return atoiOr(f[2])
		}
	}
	return 0
}

// startAs starts args as user in the background with the known password of the suite and returns
// its stop.
func startAs(t *testing.T, user string, args ...string) func() {
	t.Helper()
	if !windowsReady[user] {
		_, _ = try("net", "user", user, windowsPassword)
		windowsReady[user] = true
	}
	quoted := make([]string, len(args)-1)
	for i, a := range args[1:] {
		quoted[i] = "'" + strings.ReplaceAll(a, "'", "''") + "'"
	}
	script := "$p = ConvertTo-SecureString '" + windowsPassword + "' -AsPlainText -Force; " +
		"$c = New-Object System.Management.Automation.PSCredential('" + user + "', $p); " +
		"$r = Start-Process -PassThru -WindowStyle Hidden -Credential $c -FilePath '" + args[0] + "'"
	if len(quoted) > 0 {
		script += " -ArgumentList " + strings.Join(quoted, ",")
	}
	script += "; $r.Id"
	pid := atoiOr(strings.TrimSpace(sh(t, "powershell", "-NoProfile", "-Command", script)))
	stop := func() {
		if pid > 0 {
			_, _ = try("taskkill", "/F", "/PID", strconv.Itoa(pid))
		}
	}
	t.Cleanup(stop)
	return stop
}

// consoleOwner is empty on Windows: the agent of the console user would need the interactive
// desktop of the runner, which the suite cannot drive.
func consoleOwner() string { return "" }

// serviceLog is not kept in a form the suite reads on Windows.
func serviceLog() (string, bool) { return "", false }

// openForAll creates p, if missing, and lets the group Users modify it: the start log, which
// every session user's fake claude appends to.
func openForAll(p string) error {
	f, err := os.OpenFile(p, os.O_CREATE|os.O_WRONLY, 0o666) //nolint:gosec // G302, G304: the start log every session user appends to
	if err != nil {
		return err
	}
	_ = f.Close()
	if out, code := try("icacls", p, "/grant", "*S-1-5-32-545:(M)"); code != 0 {
		return errors.New(firstLine(out))
	}
	return nil
}

// sharedBin is a folder in the system PATH of every account and service.
func sharedBin() string { return os.Getenv("SystemRoot") } //nolint:forbidigo // the folder of the system the suite runs on
