// SPDX-License-Identifier: Apache-2.0

//go:build platform

package platform

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	hd "github.com/OWNER/bothy/services/host/internal/doctor"
	"github.com/OWNER/bothy/services/host/internal/session/sessiontest"
)

const loginDetected = "subscription_login_detected"

// hd6Failed reports a failed HD6 entry with a subscription login for sessionUser.
func hd6Failed(r sessiontest.Report, sessionUser string) bool {
	return r.Failed(hd.HD6, loginDetected, sessionUser)
}

// removeLogin removes the subscription login fakeLogin placed for user.
func removeLogin(user string) {
	_ = os.Remove(filepath.Join(home(user), ".claude", "fake-auth"))
	_ = os.Remove(filepath.Join(home(user), ".claude", ".credentials.json"))
}

// TestPlatform_TMT21_Sessions is the session part of the seven steps of TM-T21 (24 14, D-W-89,
// T-TM-08, T-TM-14). The server-side expectations (audit, host.updated, 403 terminal_denied with
// no_session_user, the console) are M2-40's, M2-43's, M2-44's and M2-48's.
func TestPlatform_TMT21_Sessions(t *testing.T) {
	track(t)
	fake := fakeClaude(t)
	w := newWorld(t, "organization", "  claude_bin: '"+fake+"'\n  credential_check_interval: "+checkPeriod+"\n", userA, userB)

	t.Run("(a) Alex logs in by subscription in her session", func(t *testing.T) {
		alex, sam := w.open(shellOf(userA)), w.open(shellOf(userB))
		mark := w.srv.ReportCount()
		alex.typeLine(claudeCmd(filepath.Dir(fake), "login"))
		alex.expect(loginRe)
		alex.expectPolicyEnd(loginDetected)
		sam.expectPolicyEnd(loginDetected)
		w.report(mark, "an HD6 entry of Alex", func(r sessiontest.Report) bool { return hd6Failed(r, userA) })
		if e := w.openFails(shellOf(userB)); e.CloseReason != reasonOpenFailed || e.CloseDetail != loginDetected {
			t.Errorf("Sam's opening ended %s %s, want open_failed %s", e.CloseReason, e.CloseDetail, loginDetected)
		}
	})

	t.Run("(b) Sam logs in too", func(t *testing.T) {
		mark := w.srv.ReportCount()
		fakeLogin(t, userB, "claude.ai")
		w.report(mark, "two HD6 entries", func(r sessiontest.Report) bool { return hd6Failed(r, userA) && hd6Failed(r, userB) })
	})

	t.Run("(c) Sam's login removed, Alex's session user locked", func(t *testing.T) {
		mark := w.srv.ReportCount()
		removeLogin(userB)
		lockUser(t, userA)
		w.report(mark, "only Alex's HD6 entry", func(r sessiontest.Report) bool { return hd6Failed(r, userA) && !hd6Failed(r, userB) })
		if e := w.openFails(shellOf(userB)); e.CloseDetail != loginDetected {
			t.Errorf("Sam's opening ended %s %s, want %s while Alex's login stays", e.CloseReason, e.CloseDetail, loginDetected)
		}
	})

	t.Run("(d) session-user remove of Alex", func(t *testing.T) {
		// Alex's home is readable for all before the removal; remove closes it and restarts the
		// host service itself.
		openHome(t, home(userA))
		n, mark := w.srv.Logins(), w.srv.ReportCount()
		if out, code := host("session-user", "remove", userA); code != 0 {
			t.Fatalf("remove: exit %d: %s", code, firstLine(out))
		}
		w.waitLogin(n + 1)
		if homeOpen(t, home(userA)) {
			t.Error("remove left Alex's home readable for all")
		}
		r := w.report(mark, "the report after the restart", func(sessiontest.Report) bool { return true })
		if r.Names(userA) || r.Lists(userA) {
			t.Error("the report still names Alex's session user")
		}
		if anyFailed(r, hd.HD6) || anyFailed(r, hd.HD9) {
			t.Error("HD6 or HD9 fail after the removal")
		}
		if out, _ := doctor(t); strings.Contains(out, "HD9 FAIL") || strings.Contains(out, "HD6 FAIL") {
			t.Error("doctor reports HD6 or HD9 after the removal")
		}
		w.open(shellOf(userB)).closeByClient()
	})

	t.Run("(e) session-user add of the removed name", func(t *testing.T) {
		if out, code := host("session-user", "add", userA); code == 0 || !strings.Contains(out, "is bound to a person on this host") {
			t.Errorf("add of a bound name: exit %d: %s", code, firstLine(out))
		}
	})

	// What session-user add creates for Robin's name in (f) goes after (g), never the host service.
	t.Cleanup(func() { removeAddedUser(t, userC) })
	t.Run("(f) session-user add while Sam has a session", func(t *testing.T) {
		sam := w.open(shellOf(userB))
		pids := sam.background()
		sam.typeLine(shellPIDCmd)
		pids = append(pids, atoiOr(sam.expect(shellPIDRe)[1]))
		n, mark := w.srv.Logins(), w.srv.ReportCount()
		if out, code := host("session-user", "add", userC); code != 0 {
			t.Fatalf("add: exit %d: %s", code, firstLine(out))
		}
		w.waitLogin(n + 1)
		waitGone(t, "the channel break of the helper", pids)
		ids := w.srv.Identifies()
		if last := ids[len(ids)-1]; len(last.LiveSessions) != 0 {
			t.Errorf("identify after the restart lists %d sessions", len(last.LiveSessions))
		}
		w.report(mark, "the new session user", func(r sessiontest.Report) bool { return r.Lists(userC) })
	})

	t.Run("(g) a login in Sam's account while the service is stopped", func(t *testing.T) {
		stopService(t)
		fakeLogin(t, userB, "claude.ai")
		mark := w.srv.ReportCount()
		startService(t)
		w.report(mark, "Sam's HD6 entry at the start", func(r sessiontest.Report) bool { return hd6Failed(r, userB) })
		if !waitRefused(t) {
			t.Fatal("the start was not refused with 78")
		}
		if n := len(w.srv.Reports()) - mark; n != 1 {
			t.Errorf("%d reports of the refused start, want one", n)
		}
		// No control connection is left that could accept a session.open.
		deadline := time.Now().Add(waitShort)
		for w.srv.Connected() {
			if time.Now().After(deadline) {
				t.Fatal("the refused service kept its control connection")
			}
			time.Sleep(pollEvery)
		}
		if err := w.srv.Open(w.ctx, sessiontest.NewID(), "shell", openWindow); !errors.Is(err, sessiontest.ErrNoControl) {
			t.Errorf("session.open after the refused start: %v", err)
		}
		time.Sleep(10 * time.Second) // longer than host_service.credential_check_interval of this test
		if _, stopped := serviceExit(); !stopped {
			t.Error("the refused service did not stay stopped")
		}
		if out, code := doctor(t); code == 0 || !strings.Contains(out, userB) {
			t.Errorf("doctor exit %d without Sam's session user", code)
		}
		removeLogin(userB)
		n := w.srv.Logins()
		if out, code := host("restart"); code != 0 {
			t.Fatalf("restart: exit %d: %s", code, firstLine(out))
		}
		w.waitLogin(n + 1)
		w.open(shellOf(userB)).closeByClient()
	})
}
