// SPDX-License-Identifier: Apache-2.0

//go:build platform

package platform

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"
)

func lsCmd(p string) []string {
	if runtime.GOOS == "windows" {
		return []string{"cmd", "/c", "dir", p}
	}
	return []string{"/bin/ls", p}
}

func catCmd(p string) []string {
	if runtime.GOOS == "windows" {
		return []string{"cmd", "/c", "type", p}
	}
	return []string{"/bin/cat", p}
}

func exeName(n string) string {
	if runtime.GOOS == "windows" {
		return n + ".exe"
	}
	return n
}

func installedBinary() string {
	switch runtime.GOOS {
	case "darwin":
		return "/Library/Application Support/bothy-host/bin/bothy-host"
	case "windows":
		return os.Getenv("ProgramFiles") + `\bothy-host\bothy-host.exe`
	}
	return "/opt/bothy-host/bin/bothy-host"
}

// programDir returns a folder of a trusted owner for the programs and places of a test.
func programDir(name string) string {
	if runtime.GOOS == "windows" {
		return filepath.Join(os.Getenv("ProgramFiles"), "bothy-platform", name)
	}
	return filepath.Join("/opt/bothy-platform", name)
}

func copyFile(t *testing.T, src, dst string, mode os.FileMode) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		t.Fatal(err)
	}
	in, err := os.Open(src)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = in.Close() }()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, mode)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = out.Close() }()
	if _, err := io.Copy(out, in); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Remove(dst) })
}

func uid(t *testing.T, user string) (int, int) {
	t.Helper()
	u, _ := try("/usr/bin/id", "-u", user)
	g, _ := try("/usr/bin/id", "-g", user)
	ui, err1 := strconv.Atoi(strings.TrimSpace(u))
	gi, err2 := strconv.Atoi(strings.TrimSpace(g))
	if err1 != nil || err2 != nil {
		t.Fatalf("id of %s", user)
	}
	return ui, gi
}

func chownTo(t *testing.T, p, user string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		sh(t, "icacls", p, "/setowner", user)
		return
	}
	u, g := uid(t, user)
	if err := os.Lchown(p, u, g); err != nil {
		t.Fatal(err)
	}
}

// pendingOwners are ownership changes for session users that install has not created yet;
// newWorld applies them right after install, before pair restarts the host service.
var pendingOwners []func(t *testing.T)

// userExists reports whether the operating system knows user.
func userExists(user string) bool {
	if runtime.GOOS == "windows" {
		_, code := try("net", "user", user)
		return code == 0
	}
	_, code := try("/usr/bin/id", "-u", user)
	return code == 0
}

// applyPendingOwners runs the ownership changes recorded before the users existed.
func applyPendingOwners(t *testing.T) {
	t.Helper()
	pend := pendingOwners
	pendingOwners = nil
	for _, f := range pend {
		f(t)
	}
}

func mkdirOwned(t *testing.T, p, user string) {
	t.Helper()
	if err := os.MkdirAll(p, 0o700); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(p) })
	if !userExists(user) {
		pendingOwners = append(pendingOwners, func(t *testing.T) { t.Helper(); chownTo(t, p, user) })
		return
	}
	chownTo(t, p, user)
}

func writeOwned(t *testing.T, p, user string) {
	t.Helper()
	if err := os.WriteFile(p, []byte("EXAMPLE\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	chownTo(t, p, user)
}

func makeWritableBy(t *testing.T, p, user string) {
	t.Helper()
	switch runtime.GOOS {
	case "linux":
		sh(t, "/usr/bin/setfacl", "-m", "u:"+user+":rwx", p)
	case "darwin":
		sh(t, "/bin/chmod", "+a", "user:"+user+" allow write,append,add_file,add_subdirectory,delete_child", p)
	case "windows":
		sh(t, "icacls", p, "/grant", user+":(M)")
	}
}

func grantRead(t *testing.T, p, user string) {
	t.Helper()
	switch runtime.GOOS {
	case "linux":
		sh(t, "/usr/bin/setfacl", "-m", "u:"+user+":rx", p)
	case "darwin":
		sh(t, "/bin/chmod", "+a", "user:"+user+" allow list,search,read", p)
	case "windows":
		sh(t, "icacls", p, "/grant", user+":(RX)")
	}
}

func openHome(t *testing.T, p string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		sh(t, "icacls", p, "/grant", "Users:(RX)")
		return
	}
	if err := os.Chmod(p, 0o755); err != nil {
		t.Fatal(err)
	}
}

func closeHome(t *testing.T, p string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		sh(t, "icacls", p, "/remove:g", "Users")
		return
	}
	_ = os.Chmod(p, 0o700)
}

func createPerson(t *testing.T, name string) {
	t.Helper()
	switch runtime.GOOS {
	case "linux":
		sh(t, "/usr/sbin/useradd", "--create-home", name)
	case "darwin":
		sh(t, "/usr/sbin/sysadminctl", "-addUser", name, "-password", "EXAMPLE-pw-1!")
		_, _ = try("/usr/sbin/createhomedir", "-c", "-u", name)
	case "windows":
		sh(t, "net", "user", name, "EXAMPLE-pw-1!", "/add")
	}
	t.Cleanup(func() { deleteUser(t, name) })
}

func deleteUser(t *testing.T, name string) {
	switch runtime.GOOS {
	case "linux":
		_, _ = try("/usr/sbin/userdel", "-r", name)
	case "darwin":
		_, _ = try("/usr/sbin/sysadminctl", "-deleteUser", name)
	case "windows":
		_, _ = try("net", "user", name, "/delete")
	}
	_ = t
}

// writeAsService writes a state file through bothy-host's own layout: a JSON file owned by the
// service user with mode 0600, as internal/state writes it.
func writeAsService(t *testing.T, name string, v any) {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(stateDir(), name)
	if err := os.WriteFile(p, b, 0o600); err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" {
		chownTo(t, p, "bothy-host")
	}
}

func writeDeclaration(t *testing.T, decl string) {
	writeAsService(t, "declaration.json", map[string]string{"llm_credentials": decl})
}

func writeBindings(t *testing.T, m map[string]string) { writeAsService(t, "bindings.json", m) }

// writeKeysAsService pairs nothing; it runs doctor so the state exists and checks the files the
// service wrote. The key file appears with pair against a server (TestHostPairAgainstServer).
func writeKeysAsService(t *testing.T) {
	t.Helper()
	if _, err := os.Stat(filepath.Join(stateDir(), "keys")); err != nil {
		t.Skip("no key file without a server; TestHostPairAgainstServer covers the file")
	}
}

// fakeLogin places a subscription login of the fake claude (and a login file) in user's account.
func fakeLogin(t *testing.T, user, method string) {
	t.Helper()
	dir := filepath.Join(home(user), ".claude")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	writeOwned(t, filepath.Join(dir, "fake-auth"), user)
	_ = os.WriteFile(filepath.Join(dir, "fake-auth"), []byte(method), 0o600)
	writeOwned(t, filepath.Join(dir, ".credentials.json"), user)
	chownTo(t, dir, user)
	t.Cleanup(func() {
		_ = os.Remove(filepath.Join(dir, "fake-auth"))
		_ = os.Remove(filepath.Join(dir, ".credentials.json"))
	})
}

func mainPID(t *testing.T) int {
	t.Helper()
	switch runtime.GOOS {
	case "linux":
		out, _ := try("/usr/bin/systemctl", "show", "--property=MainPID", "--value", "bothy-host.service")
		n, _ := strconv.Atoi(strings.TrimSpace(out))
		return n
	case "darwin":
		out, _ := try("/bin/launchctl", "print", "system/bothy-host")
		for _, l := range strings.Split(out, "\n") {
			if v, ok := strings.CutPrefix(strings.TrimSpace(l), "pid = "); ok {
				n, _ := strconv.Atoi(v)
				return n
			}
		}
	case "windows":
		out, _ := try("sc.exe", "queryex", "bothy-host")
		if i := strings.Index(out, "PID"); i >= 0 {
			f := strings.Fields(out[i:])
			if len(f) >= 3 {
				n, _ := strconv.Atoi(f[2])
				return n
			}
		}
	}
	return 0
}

// startImpostor stops the helper of user and lets a process of that user take its endpoint.
func startImpostor(t *testing.T, user string) func() {
	t.Helper()
	switch runtime.GOOS {
	case "linux":
		pid := strings.TrimSpace(sh(t, "/usr/bin/systemctl", "show", "--property=MainPID", "--value", "bothy-host-helper-"+user+".service"))
		_, _ = asUser(user, "/bin/kill", pid)
	case "darwin":
		_, _ = try("/bin/launchctl", "kill", "SIGKILL", "system/bothy-host.helper."+user)
	case "windows":
		_, _ = try("sc.exe", "stop", "bothy-host-helper-"+user)
	}
	cmd := impostorCmd(user)
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	time.Sleep(2 * time.Second)
	return func() { _ = cmd.Process.Kill(); _ = cmd.Wait() }
}

// attachSucceeds tries to attach to the helper of user from another process of that user.
func attachSucceeds(t *testing.T, user string) bool {
	t.Helper()
	switch runtime.GOOS {
	case "linux":
		pid := strings.TrimSpace(sh(t, "/usr/bin/systemctl", "show", "--property=MainPID", "--value", "bothy-host-helper-"+user+".service"))
		_, c := asUser(user, "/usr/bin/gdb", "-batch", "-p", pid)
		out, _ := asUser(user, "/bin/cat", "/proc/"+pid+"/mem")
		return c == 0 && !strings.Contains(out, "Permission denied")
	case "darwin":
		out, _ := try("/bin/launchctl", "print", "system/bothy-host.helper."+user)
		_, c := asUser(user, "/usr/bin/lldb", "--batch", "-o", "process attach --pid "+pidFrom(out))
		return c == 0
	}
	return false
}

func pidFrom(out string) string {
	for _, l := range strings.Split(out, "\n") {
		if v, ok := strings.CutPrefix(strings.TrimSpace(l), "pid = "); ok {
			return v
		}
	}
	return "0"
}
