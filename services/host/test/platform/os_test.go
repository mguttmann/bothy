// SPDX-License-Identifier: Apache-2.0

//go:build platform

package platform

import (
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

// serviceExit returns the exit code of the stopped host service.
func serviceExit() (int, bool) {
	switch runtime.GOOS {
	case "linux":
		out, _ := try("/usr/bin/systemctl", "show", "--property=ActiveState,ExecMainStatus", "--value", "bothy-host.service")
		f := strings.Fields(out)
		if len(f) == 2 && (f[0] == "failed" || f[0] == "inactive") {
			n, _ := strconv.Atoi(f[1])
			return n, true
		}
	case "darwin":
		out, _ := try("/bin/launchctl", "print", "system/bothy-host")
		if strings.Contains(out, "state = not running") {
			for _, l := range strings.Split(out, "\n") {
				if v, ok := strings.CutPrefix(strings.TrimSpace(l), "last exit code = "); ok {
					n, _ := strconv.Atoi(strings.Fields(v)[0])
					return n, true
				}
			}
		}
	case "windows":
		out, _ := try("sc.exe", "query", "bothy-host")
		if strings.Contains(out, "STOPPED") {
			if i := strings.Index(out, "SERVICE_EXIT_CODE"); i >= 0 {
				f := strings.Fields(out[i:])
				if len(f) >= 3 {
					n, _ := strconv.Atoi(f[2])
					return n, true
				}
			}
		}
	}
	return 0, false
}

// starts returns how often the host service started (Linux: NRestarts plus one).
func starts() int {
	switch runtime.GOOS {
	case "linux":
		out, _ := try("/usr/bin/systemctl", "show", "--property=NRestarts", "--value", "bothy-host.service")
		n, _ := strconv.Atoi(strings.TrimSpace(out))
		return n + 1
	case "darwin":
		out, _ := try("/bin/launchctl", "print", "system/bothy-host")
		for _, l := range strings.Split(out, "\n") {
			if v, ok := strings.CutPrefix(strings.TrimSpace(l), "runs = "); ok {
				n, _ := strconv.Atoi(v)
				return n
			}
		}
	}
	return 1
}

// asUser runs a command as a session user and returns output and exit code.
func asUser(user string, args ...string) (string, int) {
	switch runtime.GOOS {
	case "windows":
		return asWindowsUser(user, args)
	}
	return try("/usr/bin/sudo", append([]string{"-n", "-u", user, "--"}, args...)...)
}

// windowsPassword is the known password the suite gives a session user on its disposable runner;
// the helper service gets it too, so it keeps running.
const windowsPassword = "EXAMPLE-platform-pw-1!"

var windowsReady = map[string]bool{}

// asWindowsUser runs args as user through Start-Process with a credential and returns the output
// written to a file and the exit code.
func asWindowsUser(user string, args []string) (string, int) {
	if !windowsReady[user] {
		_, _ = try("net", "user", user, windowsPassword)
		_, _ = try("sc.exe", "config", "bothy-host-helper-"+user, "password=", windowsPassword)
		windowsReady[user] = true
	}
	out := filepath.Join(os.TempDir(), "bothy-platform-"+user+".out")
	quoted := make([]string, len(args)-1)
	for i, a := range args[1:] {
		quoted[i] = "'" + strings.ReplaceAll(a, "'", "''") + "'"
	}
	script := "$p = ConvertTo-SecureString '" + windowsPassword + "' -AsPlainText -Force; " +
		"$c = New-Object System.Management.Automation.PSCredential('" + user + "', $p); " +
		"$r = Start-Process -Wait -PassThru -NoNewWindow -Credential $c -FilePath '" + args[0] + "'"
	if len(quoted) > 0 {
		script += " -ArgumentList " + strings.Join(quoted, ",")
	}
	script += " -RedirectStandardOutput '" + out + "'; exit $r.ExitCode"
	_, code := try("powershell", "-NoProfile", "-Command", script)
	b, _ := os.ReadFile(out)
	_ = os.Remove(out)
	return string(b), code
}

// home returns the home of a session user.
func home(user string) string {
	switch runtime.GOOS {
	case "darwin":
		return "/Users/" + user
	case "windows":
		return `C:\Users\` + user
	}
	return "/home/" + user
}

// addToGroup puts user into group (creating the group).
func addToGroup(t *testing.T, user, group string) {
	t.Helper()
	switch runtime.GOOS {
	case "linux":
		_, _ = try("/usr/sbin/groupadd", "-f", group)
		sh(t, "/usr/sbin/usermod", "-aG", group, user)
		t.Cleanup(func() { _, _ = try("/usr/bin/gpasswd", "-d", user, group) })
	case "darwin":
		sh(t, "/usr/sbin/dseditgroup", "-o", "edit", "-a", user, "-t", "user", group)
		t.Cleanup(func() { _, _ = try("/usr/sbin/dseditgroup", "-o", "edit", "-d", user, "-t", "user", group) })
	case "windows":
		sh(t, "net", "localgroup", group, user, "/add")
		t.Cleanup(func() { _, _ = try("net", "localgroup", group, user, "/delete") })
	}
}

// sudoRule gives user passwordless sudo (Linux and macOS).
func sudoRule(t *testing.T, user string) {
	t.Helper()
	p := filepath.Join("/etc/sudoers.d", "bothy-platform-"+user)
	if err := os.WriteFile(p, []byte(user+" ALL=(ALL) NOPASSWD: ALL\n"), 0o440); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Remove(p) })
}

// uninstall removes the services, the users and the state of the suite.
func uninstall(t *testing.T, users []string) {
	switch runtime.GOOS {
	case "linux":
		_, _ = try("/usr/bin/systemctl", "disable", "--now", "bothy-host.service")
		for _, u := range users {
			_, _ = try("/usr/bin/systemctl", "disable", "--now", "bothy-host-helper-"+u+".socket", "bothy-host-helper-"+u+".service")
			_ = os.Remove("/etc/systemd/system/bothy-host-helper-" + u + ".socket")
			_ = os.Remove("/etc/systemd/system/bothy-host-helper-" + u + ".service")
			_, _ = try("/usr/sbin/userdel", "-r", u)
		}
		_ = os.Remove("/etc/systemd/system/bothy-host.service")
		_, _ = try("/usr/bin/systemctl", "daemon-reload")
		_, _ = try("/usr/sbin/userdel", "bothy-host")
	case "darwin":
		_, _ = try("/bin/launchctl", "bootout", "system/bothy-host")
		_ = os.Remove("/Library/LaunchDaemons/bothy-host.plist")
		for _, u := range users {
			_, _ = try("/bin/launchctl", "bootout", "system/bothy-host.helper."+u)
			_ = os.Remove("/Library/LaunchDaemons/bothy-host.helper." + u + ".plist")
			_, _ = try("/usr/sbin/sysadminctl", "-deleteUser", u)
			_, _ = try("/usr/bin/dscl", ".", "-delete", "/Groups/"+u)
		}
		_, _ = try("/usr/bin/dscl", ".", "-delete", "/Users/"+"bothy-host")
		_, _ = try("/usr/bin/dscl", ".", "-delete", "/Groups/bothy-host")
	case "windows":
		_, _ = try("sc.exe", "stop", "bothy-host")
		_, _ = try("sc.exe", "delete", "bothy-host")
		for _, u := range users {
			_, _ = try("sc.exe", "stop", "bothy-host-helper-"+u)
			_, _ = try("sc.exe", "delete", "bothy-host-helper-"+u)
			_, _ = try("net", "user", u, "/delete")
		}
	}
	_ = os.RemoveAll(stateDir())
	_ = os.Remove(configPath())
	_ = t
}
