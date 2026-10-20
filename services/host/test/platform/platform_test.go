// SPDX-License-Identifier: Apache-2.0

//go:build platform

package platform

import (
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// refusedWith restarts the host service and expects a refused start (exit 78) with code in the
// output of doctor, which exits 1.
func refusedWith(t *testing.T, code string) {
	t.Helper()
	if out, c := host("restart"); c != 0 {
		t.Fatalf("restart %d %s", c, out)
	}
	if !waitRefused(t) {
		t.Errorf("start not refused with 78 for %s", code)
	}
	out, c := doctor(t)
	if c != 1 || !strings.Contains(out, " "+code) {
		t.Errorf("doctor %d, want 1 with %s:\n%s", c, code, out)
	}
}

func TestPlatform_TMT13(t *testing.T) {
	track(t)
	installed(t, "", userA, userB)
	forbidden := map[string]string{"linux": "sudo", "darwin": "admin", "windows": "Administrators"}[runtime.GOOS]
	t.Run("forbidden group", func(t *testing.T) {
		addToGroup(t, userB, forbidden)
		refusedWith(t, "forbidden_group")
	})
	if runtime.GOOS == "linux" {
		t.Run("docker group", func(t *testing.T) {
			addToGroup(t, userA, "docker")
			refusedWith(t, "forbidden_group")
		})
		t.Run("writable docker socket", func(t *testing.T) {
			_ = os.MkdirAll("/var/run", 0o755)
			l, err := net.Listen("unix", "/var/run/docker.sock")
			if err != nil {
				t.Skip("a Docker daemon owns the socket on this runner")
			}
			defer func() { _ = l.Close() }()
			_ = os.Chmod("/var/run/docker.sock", 0o666)
			refusedWith(t, "docker_socket_writable")
		})
	}
	if runtime.GOOS != "windows" {
		for _, u := range []string{"bothy-host", userB} {
			t.Run("passwordless sudo "+u, func(t *testing.T) {
				sudoRule(t, u)
				refusedWith(t, "passwordless_sudo")
			})
		}
	}
	t.Run("subscription login at organization", func(t *testing.T) {
		writeDeclaration(t, "organization")
		fakeLogin(t, userB, "claude.ai")
		refusedWith(t, "subscription_login_detected")
	})
	t.Run("same service and session user", func(t *testing.T) {
		writeConfig(t, "  session_users: [bothy-host]\n")
		refusedWith(t, "session_user_is_service_user")
	})
	t.Run("empty session users", func(t *testing.T) {
		writeConfig(t, "  session_users: []\n")
		refusedWith(t, "session_users_empty")
	})
	t.Run("two equal session users", func(t *testing.T) {
		writeConfig(t, "  session_users: ["+userA+", "+strings.ToUpper(userA)+"]\n")
		refusedWith(t, "session_user_duplicate")
	})
}

func TestPlatform_TMT22(t *testing.T) {
	track(t)
	installed(t, "", userA, userB)
	if _, c := asUser(userA, lsCmd(stateDir())...); c == 0 {
		t.Error("a session user lists the state directory")
	}
	if _, c := asUser(userA, catCmd(filepath.Join(stateDir(), "keys"))...); c == 0 {
		t.Error("a session user reads the key file")
	}
	grantRead(t, stateDir(), userB)
	refusedWith(t, "state_access_for_session_user")
}

func TestPlatform_TMT24(t *testing.T) {
	track(t)
	installed(t, "", userA, userB)
	person := "bothy-platform-person"
	createPerson(t, person)
	openHome(t, home(person))
	refusedWith(t, "foreign_home_listable")
	out, _ := doctor(t)
	if !strings.Contains(out, "user="+person) {
		t.Errorf("the local output does not name %s:\n%s", person, out)
	}
	closeHome(t, home(person))
	openHome(t, home(userA))
	host("restart")
	time.Sleep(5 * time.Second)
	out, _ = doctor(t)
	if !strings.Contains(out, "HD11 FAIL shared_location session_user="+userA) || strings.Contains(out, "HD9 FAIL") {
		t.Errorf("open home of A:\n%s", out)
	}
	writeBindings(t, map[string]string{userA: "0192f4a0-0000-7000-8000-00000000000a"})
	if out, c := host("session-user", "remove", userA); c != 0 {
		t.Fatalf("remove %d %s", c, out)
	}
	openHome(t, home(userA)) // the long runner of A reopens it
	out, c := doctor(t)
	if c != 1 || !strings.Contains(out, "HD9 FAIL foreign_home_listable") || !strings.Contains(out, "user="+userA) {
		t.Errorf("reopened home:\n%s", out)
	}
	deleteUser(t, userA)
	host("restart")
	time.Sleep(5 * time.Second)
	if out, _ := doctor(t); strings.Contains(out, "HD9 FAIL") {
		t.Errorf("after deleting A:\n%s", out)
	}
}

func TestPlatform_TMT26(t *testing.T) {
	track(t)
	installed(t, "", userA, userB)
	if runtime.GOOS == "darwin" {
		out, _ := try("/usr/bin/codesign", "--display", "--verbose=4", installedBinary())
		if !strings.Contains(out, "runtime") {
			t.Errorf("the installed binary lacks the runtime flag:\n%s", out)
		}
	}
	// B replaces its helper: kill it and listen on the endpoint (or create the pipe first).
	impostor := startImpostor(t, userB)
	defer impostor()
	out, c := doctor(t)
	if c != 1 || !strings.Contains(out, "HD10 FAIL peer_mismatch session_user="+userB) && !strings.Contains(out, "HD10 FAIL helper_unreachable session_user="+userB) {
		t.Errorf("replaced helper:\n%s", out)
	}
	if attachSucceeds(t, userA) {
		t.Error("a process of the session user attached to its helper")
	}
}

func TestPlatform_TMT27(t *testing.T) {
	track(t)
	claudeDir := programDir("claude")
	claude := filepath.Join(claudeDir, exeName("claude"))
	copyFile(t, fakeBin, claude, 0o755)
	installed(t, "  claude_bin: "+claude+"\n", userA, userB)
	writeDeclaration(t, "organization")
	fakeLogin(t, userB, "claude.ai")
	refusedWith(t, "subscription_login_detected")
	makeWritableBy(t, claude, userB)
	refusedWith(t, "claude_bin_untrusted")
	if runtime.GOOS != "windows" && fakeNode != "" {
		script := filepath.Join(claudeDir, "claude-script")
		if err := os.WriteFile(script, []byte("#!/usr/bin/env node\n"), 0o755); err != nil {
			t.Fatal(err)
		}
		copyFile(t, fakeNode, filepath.Join(claudeDir, "node"), 0o755)
		writeConfig(t, "  session_users: ["+userA+", "+userB+"]\n  claude_bin: "+script+"\n")
		makeWritableBy(t, filepath.Join(claudeDir, "node"), userB)
		refusedWith(t, "claude_bin_untrusted")
	}
}

func TestPlatform_TMT29_Parts(t *testing.T) {
	track(t)
	work := programDir("work")
	wa, wb := filepath.Join(work, "a"), filepath.Join(work, "b")
	mkdirOwned(t, wa, userA)
	mkdirOwned(t, wb, userB)
	installed(t, "  claude_workdirs:\n    "+userA+": [{display_name: A, path: "+wa+"}]\n    "+userB+": [{display_name: B, path: "+wb+"}]\n", userA, userB)
	t.Run("(a) working directory of B writable by A", func(t *testing.T) {
		makeWritableBy(t, wb, userA)
		out, _ := doctor(t)
		if !strings.Contains(out, "HD11 FAIL shared_location session_user="+userB) || strings.Contains(out, "session_user="+userA) {
			t.Errorf("%s", out)
		}
	})
	t.Run("(h) CLAUDE.md of A above B", func(t *testing.T) {
		writeOwned(t, filepath.Join(work, "CLAUDE.md"), userA)
		out, _ := doctor(t)
		if !strings.Contains(out, "HD11 FAIL shared_location session_user="+userB) {
			t.Errorf("%s", out)
		}
		_ = os.Remove(filepath.Join(work, "CLAUDE.md"))
	})
	t.Run("(e) entry of A in the working directory of B", func(t *testing.T) {
		writeOwned(t, filepath.Join(wb, "from-a"), userA)
		out, _ := doctor(t)
		if !strings.Contains(out, "foreign_entry session_user="+userB) {
			t.Errorf("%s", out)
		}
		_ = os.Remove(filepath.Join(wb, "from-a"))
		if out, _ := doctor(t); strings.Contains(out, "foreign_entry") {
			t.Errorf("a full run did not lift the finding:\n%s", out)
		}
	})
	if runtime.GOOS == "windows" {
		t.Run("(i) drive root", func(t *testing.T) {
			before := sh(t, "icacls", `C:\`)
			if strings.Contains(before, `NT AUTHORITY\Authenticated Users:(AD)`) {
				t.Errorf("install with confirmation left (AD):\n%s", before)
			}
			if _, c := asUser(userA, "cmd", "/c", "mkdir", `C:\.git`); c == 0 {
				t.Error("a session user created C:\\.git")
			}
		})
	}
}

func TestPlatform_TMT21_SessionUserAddRemove(t *testing.T) {
	track(t)
	installed(t, "", userA, userB)
	writeBindings(t, map[string]string{"bothy-s-old": "0192f4a0-0000-7000-8000-00000000000b"})
	if out, c := host("session-user", "add", "bothy-s-old"); c == 0 || !strings.Contains(out, "is bound to a person on this host") {
		t.Errorf("bound name: %d %s", c, out)
	}
	if out, c := host("session-user", "add", userC); c != 0 {
		t.Fatalf("add: %d %s", c, out)
	}
	t.Cleanup(func() { deleteUser(t, userC) })
	if out, _ := doctor(t); strings.Contains(out, "user_missing") {
		t.Errorf("new session user:\n%s", out)
	}
	if out, c := host("session-user", "remove", userC); c != 0 || !strings.Contains(out, "delete the account") {
		t.Errorf("remove: %d %s", c, out)
	}
	cfg, _ := os.ReadFile(configPath())
	if strings.Contains(string(cfg), userC) {
		t.Error("the name stayed in the configuration")
	}
	writeDeclaration(t, "organization")
	fakeLogin(t, userB, "claude.ai")
	before := starts()
	refusedWith(t, "subscription_login_detected")
	time.Sleep(70 * time.Second) // longer than host_service.credential_check_interval
	if after := starts(); after-before > 1 {
		t.Errorf("the service manager restarted a refused start: %d starts", after-before)
	}
}

func TestPlatform_ExitPolicy(t *testing.T) {
	track(t)
	installed(t, "", userA)
	pid := mainPID(t)
	crash(t, pid)
	time.Sleep(10 * time.Second)
	if mainPID(t) == 0 {
		t.Error("a crash was not restarted")
	}
	writeConfig(t, "  session_users: []\n")
	refusedWith(t, "session_users_empty")
}

func TestPlatform_DPAPIAndModes(t *testing.T) {
	track(t)
	installed(t, "", userA)
	keys := filepath.Join(stateDir(), "keys")
	writeKeysAsService(t)
	if runtime.GOOS == "windows" {
		b, err := os.ReadFile(keys)
		if err != nil || len(b) < 4 || b[0] != 1 {
			t.Errorf("not a DPAPI blob: %v", err)
		}
		return
	}
	fi, err := os.Stat(keys)
	if err != nil || fi.Mode().Perm() != 0o600 {
		t.Errorf("key file mode %v %v", fi, err)
	}
}

func TestPlatform_NoNetworkListener(t *testing.T) {
	track(t)
	installed(t, "", userA)
	time.Sleep(3 * time.Second)
	var out string
	switch runtime.GOOS {
	case "linux":
		out, _ = try("/usr/bin/ss", "-ltnup")
	case "darwin":
		out, _ = try("/usr/sbin/lsof", "-nP", "-iTCP", "-sTCP:LISTEN", "-iUDP")
	case "windows":
		out, _ = try("netstat", "-abno")
	}
	if strings.Contains(out, "bothy-host") {
		t.Errorf("a listener of the host service:\n%s", out)
	}
}

func TestPlatform_AdminEndpointPeer(t *testing.T) {
	track(t)
	installed(t, "", userA)
	if runtime.GOOS == "windows" {
		return // covered by the pipe created first in TestPlatform_TMT26
	}
	if _, c := asUser(userA, "/usr/bin/nc", "-U", filepath.Join(stateDir(), "admin.sock")); c == 0 {
		t.Error("a session user connected to the admin endpoint")
	}
}

var _ = exec.Command
