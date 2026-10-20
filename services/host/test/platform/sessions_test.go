// SPDX-License-Identifier: Apache-2.0

//go:build platform

package platform

import (
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	hd "github.com/OWNER/bothy/services/host/internal/doctor"
	"github.com/OWNER/bothy/services/host/internal/frame"
	"github.com/OWNER/bothy/services/host/internal/session/sessiontest"
)

// TestPlatform_PTYRunsAsSessionUser (SR12, T12, T-TM-12): every PTY runs as the user of osu in
// the helper of osu, never as the service user; start, resize, exit code and the kill of the tree
// hold under the real services.
func TestPlatform_PTYRunsAsSessionUser(t *testing.T) {
	track(t)
	w := newWorld(t, "organization", "", userA, userB)
	for _, u := range []string{userA, userB} {
		s := w.open(shellOf(u))
		got := normalIdentity(s.identity())
		if got != wantIdentity(t, u) {
			t.Errorf("the PTY of %s runs as another user", u)
		}
		if runtime.GOOS != "windows" && got == wantIdentity(t, "bothy-host") {
			t.Errorf("the PTY of %s runs as the service user", u)
		}
		s.typeLine(parentCmd)
		if pp := atoiOr(s.expect(parentRe)[1]); pp != helperPID(t, u) || pp <= 0 {
			t.Errorf("the PTY of %s is no child of the helper of %s", u, u)
		}
		if u == userB && helperPID(t, userA) == helperPID(t, userB) {
			t.Error("Alex and Sam share one helper")
		}
		s.closeByClient()
	}
	t.Run("resize and exit code", func(t *testing.T) {
		s := w.open(shellOf(userA))
		s.send(frame.Resize(100, 30))
		s.typeLine(sizeCmd)
		if m := s.expect(sizeRe); m[1] != "30" || m[2] != "100" {
			t.Errorf("size %sx%s after resize, want 30 rows and 100 columns", m[1], m[2])
		}
		s.typeLine("exit 7")
		if code := s.waitExit(); code != 7 {
			t.Errorf("exit frame %d, want 7", code)
		}
		e := s.ended()
		if e.CloseReason != reasonExited || e.ExitCode == nil || *e.ExitCode != 7 {
			t.Errorf("end %s with exit code %v, want %s 7", e.CloseReason, e.ExitCode, reasonExited)
		}
		s.serverEnd()
	})
	t.Run("tree killed at the end", func(t *testing.T) {
		s := w.open(shellOf(userB))
		pids := s.background()
		s.closeByClient()
		waitGone(t, "close", pids)
	})
}

// TestPlatform_TMT28c (TM-T28 (c), Ruling T12): an osu outside host_service.session_users and an
// osu bound to another person start no PTY; the binding lies in bindings.json. A token without own
// on this server host is refused too (step 13).
func TestPlatform_TMT28c(t *testing.T) {
	track(t)
	w := newWorld(t, "organization", "", userA, userB)
	w.refused(shellOf(userC), codeHandshakeFail, "session_user_unknown")
	s := w.open(shellOf(userA))
	s.closeByClient()
	if got := bindingOf(t, userA); got != subAlex {
		t.Errorf("bindings.json binds %s to another person", userA)
	}
	// Sam with Alex's session user: the assignment changed in the database bypassing the server.
	samOnAlex := shellOf(userA)
	samOnAlex.mut = func(c map[string]any) { c["sub"] = subSam }
	w.refused(samOnAlex, codeHandshakeFail, "session_user_mismatch")
	if got := bindingOf(t, userA); got != subAlex {
		t.Error("a refused opening changed the binding")
	}
	notOwner := shellOf(userB)
	notOwner.mut = func(c map[string]any) { c["own"] = false }
	w.refused(notOwner, codeHandshakeFail, "server_host_owner_only")
}

// TestPlatform_TMT28d (TM-T28 (d), macOS): with organization and an empty host_service.claude_bin
// a login of Robin in the keychain leaves no file HD6 checks; the host warns auth_status_unchecked
// and claude_profile_unavailable, offers no claude profile, and Alex's session, which runs as
// Alex's session user, sees no login and cannot read Robin's home.
func TestPlatform_TMT28d(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("TM-T28 (d) is the keychain case of macOS; Linux and Windows keep the login in a file that HD6 checks (TestPlatform_TMT21_Sessions)")
	}
	track(t)
	resetStartLog(t)
	fake := fakeClaude(t)
	w := newWorld(t, "organization", "  profiles: [shell, claude]\n", userC, userA)
	robin := w.open(shellOf(userC))
	robin.typeLine(claudeCmd(filepath.Dir(fake), "login"))
	robin.expect(loginRe)
	robin.closeByClient()
	r := w.report(0, "the warnings of an empty claude_bin", func(r sessiontest.Report) bool {
		return r.Has(hd.HD6, hd.CodeAuthStatusUnchecked) && r.Has(hd.HD7, hd.CodeClaudeProfileUnavailable)
	})
	if !r.Has(hd.HD6, hd.CodeAuthStatusUnchecked) || !r.Has(hd.HD7, hd.CodeClaudeProfileUnavailable) || r.HasProfile("claude") {
		t.Errorf("report: auth_status_unchecked %v, claude_profile_unavailable %v, claude offered %v",
			r.Has(hd.HD6, hd.CodeAuthStatusUnchecked), r.Has(hd.HD7, hd.CodeClaudeProfileUnavailable), r.HasProfile("claude"))
	}
	alex := w.open(shellOf(userA))
	if normalIdentity(alex.identity()) != wantIdentity(t, userA) {
		t.Error("Alex's session runs as another user")
	}
	alex.typeLine(claudeCmd(filepath.Dir(fake), "auth", "status"))
	if m := alex.expect(authRe); m[1] != "none" {
		t.Errorf("auth status in Alex's session reports %s, want none", m[1])
	}
	alex.typeLine(listCmd(home(userC)))
	if alex.expect(listRe)[1] == "0" {
		t.Error("Alex's shell lists Robin's home")
	}
	alex.closeByClient()
}

// TestPlatform_HelperChannelBreak (D-W-90, R-TM-27): bothy-host restart during a session with a
// background process: the helper ends the PTY with its tree when its channel breaks.
func TestPlatform_HelperChannelBreak(t *testing.T) {
	track(t)
	w := newWorld(t, "organization", "", userA)
	s := w.open(shellOf(userA))
	pids := s.background()
	s.typeLine(shellPIDCmd)
	pids = append(pids, atoiOr(s.expect(shellPIDRe)[1]))
	w.restart()
	waitGone(t, "restart of the host service", pids)
	ids := w.srv.Identifies()
	if last := ids[len(ids)-1]; len(last.LiveSessions) != 0 {
		t.Errorf("identify after the restart lists %d sessions", len(last.LiveSessions))
	}
}

// TestPlatform_LocalConfirm (24 6.4, Ruling T3, Design note 15): with
// host_service.require_local_confirm no agent, and an agent run as a session user, give
// local_confirm_denied. The allowed path needs a person who clicks the dialog of the console user,
// which a hosted runner cannot provide (the agent has no answer without a dialog); it is not
// provable here and stays with TestConfirmOpenAndAttach. The console owner's agent of a macOS
// runner answers deny, because nobody answers its dialog.
func TestPlatform_LocalConfirm(t *testing.T) {
	track(t)
	w := newWorld(t, "organization", "  require_local_confirm: true\n  local_confirm_timeout: 3s\n", userA, userB)
	if !w.lastReport().RequireLocalConfirm {
		t.Error("report does not say require_local_confirm")
	}
	t.Run("no agent", func(t *testing.T) {
		if e := w.openFails(shellOf(userA)); e.CloseReason != reasonOpenFailed || e.CloseDetail != "local_confirm_denied" {
			t.Errorf("end %s %s, want open_failed local_confirm_denied", e.CloseReason, e.CloseDetail)
		}
	})
	t.Run("agent of a session user", func(t *testing.T) {
		stop := startAs(t, userB, installedBinary(), "confirm-agent")
		defer stop()
		time.Sleep(3 * time.Second) // the agent dials once per second
		if e := w.openFails(shellOf(userA)); e.CloseReason != reasonOpenFailed || e.CloseDetail != "local_confirm_denied" {
			t.Errorf("end %s %s, want open_failed local_confirm_denied", e.CloseReason, e.CloseDetail)
		}
		if log, ok := serviceLog(); ok && !strings.Contains(log, "confirm_peer_refused") {
			t.Error("the service did not log confirm_peer_refused for the agent of a session user")
		}
	})
	if owner := consoleOwner(); owner != "" {
		t.Run("agent of the console owner without an answer", func(t *testing.T) {
			stop := startAs(t, owner, installedBinary(), "confirm-agent")
			defer stop()
			time.Sleep(3 * time.Second)
			if e := w.openFails(shellOf(userA)); e.CloseDetail != "local_confirm_denied" {
				t.Errorf("an unanswered request ended %s %s, want local_confirm_denied", e.CloseReason, e.CloseDetail)
			}
		})
	}
}
