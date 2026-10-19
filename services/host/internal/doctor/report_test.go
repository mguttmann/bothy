// SPDX-License-Identifier: Apache-2.0

package doctor

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/host/internal/state"
)

func hd11For(r *Result, user string) string {
	for _, e := range r.Report() {
		if e.Check == HD11 && e.SessionUser == user {
			return e.Code
		}
	}
	return ""
}

func TestEntryScanSemantics(t *testing.T) {
	x := newTW(t)
	x.w.File("/home/"+sam+"/notes/from-alex.txt", "1001", 0o600, "EXAMPLE")
	x.w.Node("/home/" + sam + "/notes").Sec.OwnerID = "1002"
	runner := NewRunner(x.env, nil)
	runner.RunFull(context.Background())
	runner.WaitScans()
	r := runner.Last()
	if hd11For(r, sam) != CodeForeignEntry || r.SessionUserReady(sam) || !r.SessionUserReady(alex) {
		t.Fatalf("finding: %v", codes(r, HD11))
	}
	// Removed: a Light run keeps the finding.
	x.w.Remove("/home/" + sam + "/notes/from-alex.txt")
	r = runner.RunLight(context.Background())
	if hd11For(r, sam) != CodeForeignEntry {
		t.Errorf("light lifted the finding: %v", codes(r, HD11))
	}
	// A Full run that finishes without a finding lifts it.
	runner.RunFull(context.Background())
	runner.WaitScans()
	if r := runner.Last(); hd11For(r, sam) != "" {
		t.Errorf("full kept the finding: %v", codes(r, HD11))
	}
	if host, user := runner.Readiness(sam); !host || !user {
		t.Error("readiness after the lift")
	}

	// Timeout: B's home is large; the scan stops at host_service.entry_scan_timeout with B's
	// name while A is ready first.
	x = newTW(t)
	for i := range 5 {
		x.w.Dir(fmt.Sprintf("/home/%s/d%d", sam, i), "1002", 0o700)
	}
	release := make(chan struct{})
	var once sync.Once
	x.w.ListHook = func(p string) {
		if strings.HasPrefix(p, "/home/"+sam+"/d") {
			once.Do(func() { <-release })
		}
	}
	changes := make(chan *Result, 10)
	runner = NewRunner(x.env, func(r *Result) { changes <- r })
	r = runner.RunFull(context.Background())
	for _, u := range []string{alex, sam, robin} {
		if hd11For(r, u) != CodeEntryScanRunning {
			t.Errorf("%s while running: %q", u, hd11For(r, u))
		}
	}
	// Wait until A's and Robin's scans finished: A is ready while B still runs.
	deadline := time.Now().Add(5 * time.Second)
	for {
		_, a := runner.Readiness(alex)
		_, r := runner.Readiness(robin)
		if a && r {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("A and Robin never became ready")
		}
		<-changes
	}
	if _, ok := runner.Readiness(sam); ok {
		t.Error("B ready while its scan runs")
	}
	// Only B's scan holds a timer now.
	for x.clk.Pending() != 1 {
		time.Sleep(time.Millisecond)
	}
	x.clk.Advance(x.cfg.EntryScanTimeout)
	close(release)
	runner.WaitScans()
	if r := runner.Last(); hd11For(r, sam) != CodeEntryScanTimeout || !strings.Contains(r.Format(), "entry_scan_timeout session_user="+sam) {
		t.Errorf("timeout: %v", codes(r, HD11))
	}
	// A helper that cannot scan counts as not finished.
	x = newTW(t)
	runner = NewRunner(x.env, nil)
	runner.RunFull(context.Background())
	x.setPeer(robin, fmt.Errorf("gone"))
	runner.WaitScans()
	if runner.SetDeclaration(state.DeclNone) == nil {
		t.Error("SetDeclaration after a run")
	}
	if NewRunner(x.env, nil).SetDeclaration(state.DeclNone) != nil || NewRunner(x.env, nil).Last() != nil {
		t.Error("before the first run")
	}
	if h, u := NewRunner(x.env, nil).Readiness(alex); h || u {
		t.Error("ready before the first run")
	}
}

func TestReportNamesRule(t *testing.T) {
	x := newTW(t)
	x.auth[alex] = map[string]string{"": "claude.ai"}
	x.auth[sam] = map[string]string{"org": "oauth_token"}
	x.peer[robin] = fmt.Errorf("x: %w", ErrPeerMismatch)
	x.w.AddUser("bothy-s-old", "1600", "/home/bothy-s-old", true)
	x.w.Dir("/home/bothy-s-old", "1600", 0o755)
	x.w.AddUser("casey", "1500", "/home/casey", true)
	x.w.Dir("/home/casey", "1500", 0o755)
	x.env.Bindings = map[string]string{"bothy-s-old": "p"}
	x.w.Node("/home/" + sam).Sec.Mode = 0o750
	x.w.Node("/home/" + sam).Sec.GroupID = "1001"
	x.w.SudoOK[svc] = true
	r := x.run()
	rep := r.Report()
	seen := map[string]bool{}
	for _, e := range rep {
		key := e.Check + "/" + e.SessionUser
		if seen[key] {
			t.Errorf("duplicate %s", key)
		}
		seen[key] = true
		switch e.Check {
		case HD1, HD2, HD3, HD4, HD5, HD7, HD8:
			if e.SessionUser != "" {
				t.Errorf("%s names %s", e.Check, e.SessionUser)
			}
		}
		if e.SessionUser == "casey" {
			t.Error("a name of another user left the host")
		}
	}
	for _, want := range []string{HD6 + "/" + alex, HD6 + "/" + sam, HD10 + "/" + robin, HD9 + "/bothy-s-old", HD9 + "/", HD11 + "/" + sam, HD3 + "/"} {
		if !seen[want] {
			t.Errorf("missing %s in %+v", want, rep)
		}
	}
}

const (
	canaryPath    = "/srv/CANARY-PATH-7f3e"
	canaryToken   = "CANARY-TOKEN-91ab" //nolint:gosec // a canary value, not a credential
	canaryContent = "CANARY-CONTENT-55cd"
)

func canaryWorld(t *testing.T) *Result {
	t.Helper()
	x := newTW(t)
	x.w.Dir(canaryPath, "1001", 0o777)
	x.w.File(canaryPath+"/CLAUDE.md", "1002", 0o666, canaryContent)
	workdir(x, alex, "Canary", canaryPath+"/w")
	x.w.File("/home/"+alex+"/.claude/.credentials.json", "1001", 0o600, canaryToken)
	x.auth[sam] = map[string]string{"": "garbage"}
	x.cfg.ClaudeBin = canaryPath + "/claude"
	x.w.File(canaryPath+"/claude", "1001", 0o777, "#!"+canaryPath+"/node\n"+canaryContent)
	x.w.AddUser("casey", "1500", "/home/casey", true)
	x.w.Dir("/home/casey", "1500", 0o755)
	x.w.File("/home/casey/"+canaryToken, "1500", 0o644, canaryContent)
	return x.run()
}

func TestReportHasNoPathsOrValues(t *testing.T) {
	r := canaryWorld(t)
	b, err := json.Marshal(map[string]any{"doctor": r.Report(), "profiles": r.Profiles, "session_users": r.SessionUsers, "is_server_host": r.IsServerHost})
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []string{"CANARY", "/", `\`} {
		if strings.Contains(string(b), c) {
			t.Errorf("report contains %q: %s", c, b)
		}
	}
	if r.ExitCode() != 1 {
		t.Error("canary world passes")
	}
}

func TestDoctorOutputHasNoPathsOrValues(t *testing.T) {
	out := canaryWorld(t).Format()
	for _, c := range []string{"CANARY", "/", `\`} {
		if strings.Contains(out, c) {
			t.Errorf("output contains %q:\n%s", c, out)
		}
	}
}

func TestDoctorOutputFormatAndExitCode(t *testing.T) {
	x := newTW(t)
	x.cfg.ClaudeBin = ""
	r := x.run()
	want := "HD1 ok\nHD2 ok\nHD3 ok\nHD4 ok\nHD5 ok\nHD6 WARN auth_status_unchecked\nHD7 ok\nHD8 ok\nHD9 ok\nHD10 ok\nHD11 ok\n" +
		"doctor: 0 failed, 1 warnings\ndeclaration: organization\n"
	if got := r.Format(); got != want || r.ExitCode() != 0 {
		t.Errorf("warnings only (exit %d):\n%s", r.ExitCode(), got)
	}
	x = newTW(t)
	x.env.Declaration = state.DeclUnknown
	x.w.AddUser("casey", "1500", "/home/casey", true)
	x.w.Dir("/home/casey", "1500", 0o755)
	x.w.AddUser("dana", "1501", "/home/dana", true)
	x.w.Dir("/home/dana", "1501", 0o755)
	x.auth[sam] = map[string]string{"": "claude.ai"}
	r = x.run()
	want = "HD1 ok\nHD2 ok\nHD3 ok\nHD4 ok\nHD5 ok\nHD6 WARN declaration_unknown session_user=bothy-s-sam\nHD7 ok\nHD8 ok\n" +
		"HD9 FAIL foreign_home_listable user=casey user=dana\nHD10 ok\nHD11 ok\ndoctor: 1 failed, 1 warnings\ndeclaration: unknown\n"
	if got := r.Format(); got != want || r.ExitCode() != 1 {
		t.Errorf("one failure (exit %d):\n%s", r.ExitCode(), got)
	}
}

// A declaration that arrives while entry scans run must not race with the scans' read of the
// environment (INT-W44/1); go test -race reports the unsynchronised access.
func TestSetDeclarationDuringEntryScans(t *testing.T) {
	x := newTW(t)
	runner := NewRunner(x.env, nil)
	runner.RunFull(context.Background())
	runner.SetDeclaration("api_key")
	runner.WaitScans()
	if runner.Last() == nil {
		t.Fatal("no result after the scans")
	}
}
