// SPDX-License-Identifier: Apache-2.0

//go:build platform

package platform

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	hd "github.com/OWNER/bothy/services/host/internal/doctor"
	"github.com/OWNER/bothy/services/host/internal/session/sessiontest"
)

// tokenCanary is the subscription token of Robin's login shell in TM-T25; it must never leave
// her session user.
const tokenCanary = "EXAMPLE-OAUTH-TOKEN-CANARY-91c4" //nolint:gosec // G101: a canary of the test, no credential

// forbiddenEnv are the variables the claude profile with organization never sets (09 18.3).
var forbiddenEnv = []string{"ANTHROPIC_AUTH_TOKEN", "CLAUDE_CODE_OAUTH_TOKEN", "ANTHROPIC_PROFILE", "CLAUDE_CODE_USE_BEDROCK",
	"CLAUDE_CODE_USE_VERTEX", "CLAUDE_CODE_USE_FOUNDRY"}

// envNames returns the variable names of an interactive start log entry.
func envNames(e map[string]string) []string { return strings.Split(e["env"], ",") }

func hasName(names []string, n string) bool {
	for _, x := range names {
		if strings.EqualFold(x, n) {
			return true
		}
	}
	return false
}

// claudeOffered waits for a report that offers claude with one working directory of sessionUser.
func claudeOffered(w *world, sessionUser string) {
	w.t.Helper()
	w.report(0, "the claude profile", func(r sessiontest.Report) bool {
		return r.HasProfile("claude") && len(r.Workdirs(sessionUser)) == 1
	})
}

// exitClaude ends the fake claude of s and expects process_exited.
func exitClaude(s *tsess) {
	s.w.t.Helper()
	s.typeLine("exit")
	e := s.ended()
	if e.CloseReason != reasonExited {
		s.w.t.Errorf("claude ended %s %s, want %s", e.CloseReason, e.CloseDetail, reasonExited)
	}
	s.serverEnd()
}

// TestPlatform_TMT29g_Token (TM-T29 (g), Ruling T13): a token with wdi outside the reported list
// of osu (the fake server bypasses the server's check) starts no PTY; the index inside starts the
// resolved host_service.claude_bin in exactly that directory with the organization environment.
func TestPlatform_TMT29g_Token(t *testing.T) {
	track(t)
	resetStartLog(t)
	wa := programDir("work-alex")
	mkdirOwned(t, wa, userA)
	w := newWorld(t, "organization", claudeYAML(t)+workdirsYAML(map[string][][2]string{userA: {{"Alex", wa}}}), userA)
	writeAPIKey(t)
	claudeOffered(w, userA)
	for _, wdi := range []int{1, 9} {
		w.refused(claudeOf(userA, wdi), "profile_unknown")
	}
	if len(interactiveStarts(t)) != 0 {
		t.Fatal("a refused token started claude")
	}
	s := w.open(claudeOf(userA, 0))
	e := firstInteractive(t)
	if !samePath(realPath(e["cwd"]), realPath(wa)) {
		t.Error("claude did not start in the working directory of wdi 0")
	}
	if e["api_key_sha256"] != apiKeyHash() {
		t.Error("claude did not get the key of claude_api_key")
	}
	if !samePath(e["config_dir"], filepath.Join(home(userA), ".config", "bothy-host", "claude")) {
		t.Error("CLAUDE_CONFIG_DIR is not the folder of the host service in the home")
	}
	for _, n := range forbiddenEnv {
		if hasName(envNames(e), n) {
			t.Errorf("the organization environment sets %s", n)
		}
	}
	exitClaude(s)
}

// TestPlatform_TMT29c_Open (TM-T29 (c)): a directory listed for Alex and Sam is dropped from both
// reported lists, and Sam's reported index of his further directory starts claude in exactly that
// directory.
func TestPlatform_TMT29c_Open(t *testing.T) {
	track(t)
	resetStartLog(t)
	shared, wa, further := programDir("work-shared"), programDir("work-alex"), programDir("work-sam-further")
	if err := os.MkdirAll(shared, 0o755); err != nil { //nolint:gosec // G301: both session users must reach it
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(shared) })
	mkdirOwned(t, wa, userA)
	mkdirOwned(t, further, userB)
	w := newWorld(t, "organization", claudeYAML(t)+workdirsYAML(map[string][][2]string{
		userA: {{"Shared", shared}, {"Alex", wa}},
		userB: {{"Shared", shared}, {"Further", further}},
	}), userA, userB)
	writeAPIKey(t)
	r := w.report(0, "the lists after HD7", func(r sessiontest.Report) bool {
		return len(r.Workdirs(userA)) == 1 && len(r.Workdirs(userB)) == 1
	})
	if r.Workdirs(userA)[0] != "Alex" || r.Workdirs(userB)[0] != "Further" {
		t.Errorf("reported lists %v and %v, want [Alex] and [Further]", r.Workdirs(userA), r.Workdirs(userB))
	}
	s := w.open(claudeOf(userB, 0))
	if e := firstInteractive(t); !samePath(realPath(e["cwd"]), realPath(further)) {
		t.Error("index 0 of Sam did not start claude in his further directory")
	}
	exitClaude(s)
}

// TestPlatform_TMT29a_Session (TM-T29 (a), T-TM-13): Sam's working directory becomes writable for
// Alex and holds a settings file with a hook and an apiKeyHelper (canary programs): Sam's opening
// ends with shared_location, Sam's running session ends with host_policy_violation and
// shared_location, report names Sam, neither canary runs, and Alex still opens.
func TestPlatform_TMT29a_Session(t *testing.T) {
	track(t)
	resetStartLog(t)
	wa, wb := programDir("work-alex"), programDir("work-sam")
	mkdirOwned(t, wa, userA)
	mkdirOwned(t, wb, userB)
	w := newWorld(t, "organization", claudeYAML(t)+workdirsYAML(map[string][][2]string{userA: {{"Alex", wa}}, userB: {{"Sam", wb}}}), userA, userB)
	writeAPIKey(t)
	claudeOffered(w, userB)
	running := w.open(shellOf(userB))
	hook := canary(t, programDir("canary"), "hook-canary")
	keyHelper := canary(t, programDir("canary"), "key-helper-canary")
	makeWritableBy(t, wb, userA)
	settings := filepath.Join(wb, ".claude")
	if err := os.MkdirAll(settings, 0o755); err != nil { //nolint:gosec // G301: Sam's claude must read it
		t.Fatal(err)
	}
	chownTo(t, settings, userA)
	body := `{"hooks":{"SessionStart":[{"hooks":[{"type":"command","command":"` + jsonPath(hook) + `"}]}]},"apiKeyHelper":"` + jsonPath(keyHelper) + `"}`
	if err := os.WriteFile(filepath.Join(settings, "settings.json"), []byte(body), 0o644); err != nil { //nolint:gosec // G306: Sam's claude must read it
		t.Fatal(err)
	}
	chownTo(t, filepath.Join(settings, "settings.json"), userA)
	mark := w.srv.ReportCount()
	if e := w.openFails(claudeOf(userB, 0)); e.CloseReason != reasonOpenFailed || e.CloseDetail != "shared_location" {
		t.Errorf("Sam's opening ended %s %s, want open_failed shared_location", e.CloseReason, e.CloseDetail)
	}
	running.expectPolicyEnd("shared_location")
	r := w.report(mark, "HD11 of Sam", func(r sessiontest.Report) bool { return hd11Failed(r, userB) })
	if hd11Failed(r, userA) {
		t.Error("report blames Alex's session user")
	}
	if ran(t, hook) || ran(t, keyHelper) {
		t.Error("a program of the foreign settings file ran")
	}
	alex := w.open(claudeOf(userA, 0))
	exitClaude(alex)
}

// jsonPath escapes the backslashes of a Windows path for a JSON string.
func jsonPath(p string) string { return strings.ReplaceAll(p, `\`, `\\`) }

// TestPlatform_TMT29f_Operation (TM-T29 (f), D-W-89): the program of a command profile becomes
// writable for a session user: the next periodic run ends both running sessions with
// host_policy_violation and shared_location, refuses new ones, and report names no session user;
// after the fix both open again after the next periodic run.
func TestPlatform_TMT29f_Operation(t *testing.T) {
	track(t)
	tool := canary(t, programDir("tool"), "tool")
	w := newWorld(t, "organization", "  credential_check_interval: "+checkPeriod+"\n  profiles:\n    - shell\n"+
		"    - {name: tool, display_name: Tool, kind: command, program: '"+tool+"', args: []}\n", userA, userB)
	w.report(0, "the command profile", func(r sessiontest.Report) bool { return r.HasProfile("tool") })
	a, b := w.open(shellOf(userA)), w.open(shellOf(userB))
	mark := w.srv.ReportCount()
	makeWritableBy(t, tool, userA)
	a.expectPolicyEnd("shared_location")
	b.expectPolicyEnd("shared_location")
	r := w.report(mark, "HD11 for all", func(r sessiontest.Report) bool { return hd11Failed(r, "") })
	if hd11Failed(r, userA) || hd11Failed(r, userB) {
		t.Error("report names a session user for a location for all")
	}
	if e := w.openFails(shellOf(userB)); e.CloseReason != reasonOpenFailed || e.CloseDetail != "shared_location" {
		t.Errorf("an opening during the violation ended %s %s", e.CloseReason, e.CloseDetail)
	}
	mark = w.srv.ReportCount()
	revokeWrite(t, tool, userA)
	w.report(mark, "HD11 passing again", func(r sessiontest.Report) bool { return !hd11Failed(r, "") })
	w.open(shellOf(userA)).closeByClient()
	w.open(shellOf(userB)).closeByClient()
}

// TestPlatform_TMT29k (TM-T29 (k), D-W-82, D-W-84): with personal, Robin's login shell sets
// CLAUDE_CONFIG_DIR outside her home, to a link leading outside, to a dangling link and to a folder
// in the home: the first three end with open_failed and shared_location while shell still opens;
// the fourth starts claude with exactly that folder.
func TestPlatform_TMT29k(t *testing.T) {
	track(t)
	resetStartLog(t)
	fake := fakeClaude(t)
	wr, outside := programDir("work-robin"), programDir("outside-config")
	mkdirOwned(t, wr, userC)
	if err := os.MkdirAll(outside, 0o755); err != nil { //nolint:gosec // G301: an existing folder Robin can reach
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(outside) })
	w := newWorld(t, "personal", "  profiles: [shell, claude]\n"+workdirsYAML(map[string][][2]string{userC: {{"Robin", wr}}}), userC)
	claudeOffered(w, userC)
	link, dangling, inside := filepath.Join(home(userC), "cfg-link"), filepath.Join(home(userC), "cfg-dangling"), filepath.Join(home(userC), "cfg-inside")
	symlinkOf(t, userC, outside, link)
	symlinkOf(t, userC, filepath.Join(home(userC), "cfg-missing"), dangling)
	mkdirOwned(t, inside, userC)
	for _, c := range []struct{ name, dir string }{{"outside the home", outside}, {"link leading outside", link}, {"dangling link", dangling}} {
		t.Run(c.name, func(t *testing.T) {
			setLoginEnv(t, userC, loginEnv{PathDir: filepath.Dir(fake), ConfigDir: c.dir})
			if e := w.openFails(claudeOf(userC, 0)); e.CloseReason != reasonOpenFailed || e.CloseDetail != "shared_location" {
				t.Errorf("claude ended %s %s, want open_failed shared_location", e.CloseReason, e.CloseDetail)
			}
			w.open(shellOf(userC)).closeByClient()
		})
	}
	if len(interactiveStarts(t)) != 0 {
		t.Fatal("claude started with a CLAUDE_CONFIG_DIR outside the home")
	}
	t.Run("folder in the home", func(t *testing.T) {
		setLoginEnv(t, userC, loginEnv{PathDir: filepath.Dir(fake), ConfigDir: inside})
		s := w.open(claudeOf(userC, 0))
		if e := firstInteractive(t); !samePath(realPath(e["config_dir"]), realPath(inside)) {
			t.Error("claude did not get exactly the folder in the home as CLAUDE_CONFIG_DIR")
		}
		exitClaude(s)
	})
}

// TestPlatform_TMT25 (TM-T25, I5, D-W-17): with personal, Robin's login shell exports a canary
// token from a file of her session user: auth status in her session reports oauth_token, the
// claude profile gets the variable from her shell and no key; the canary is nowhere in the state
// directory or in anything sent to the fake server, and on Linux the trace of the service shows
// no open of a file under Robin's home and no canary in its local channels. macOS and Windows:
// the canary parts without the trace.
func TestPlatform_TMT25(t *testing.T) {
	track(t)
	resetStartLog(t)
	fake := fakeClaude(t)
	wr := programDir("work-robin")
	mkdirOwned(t, wr, userC)
	w := newWorld(t, "personal", "  profiles: [shell, claude]\n"+workdirsYAML(map[string][][2]string{userC: {{"Robin", wr}}}), userC)
	claudeOffered(w, userC)
	tokenFile := writeSecretOf(t, userC, ".bothy-platform-token", tokenCanary)
	setLoginEnv(t, userC, loginEnv{PathDir: filepath.Dir(fake), TokenFile: tokenFile})
	stopTrace, traced := traceService(t)
	shell := w.open(shellOf(userC))
	shell.typeLine(claudeCmd(filepath.Dir(fake), "auth", "status"))
	if m := shell.expect(authRe); m[1] != "oauth_token" {
		t.Errorf("auth status in Robin's session reports %s, want oauth_token", m[1])
	}
	shell.closeByClient()
	s := w.open(claudeOf(userC, 0))
	e := firstInteractive(t)
	if !hasName(envNames(e), "CLAUDE_CODE_OAUTH_TOKEN") {
		t.Error("claude with personal did not get the token of Robin's login shell")
	}
	empty := sha256.Sum256(nil)
	if e["api_key_sha256"] != hex.EncodeToString(empty[:]) {
		t.Error("claude with personal got an API key")
	}
	exitClaude(s)
	noCanaryIn(t, w, stateDir(), tokenCanary)
	if !traced {
		return
	}
	trace := stopTrace()
	if strings.Contains(trace, tokenCanary) {
		t.Error("the canary passed a system call of the service user")
	}
	under := `"` + home(userC) + `/`
	for _, l := range strings.Split(trace, "\n") {
		if strings.Contains(l, "open") && strings.Contains(l, under) {
			t.Error("the service user opened a file under Robin's home")
			break
		}
	}
	if !strings.Contains(trace, "write") {
		t.Error("the trace recorded nothing; strace did not attach")
	}
}

// TestPlatform_TMT29l (TM-T29 (l), D-W-81, D-W-85): with organization another claude canary lies
// in the PATH of the session user and of the service; Sam's claude starts exactly the resolved
// host_service.claude_bin. Then host_service.claude_bin counts as empty (a script with
// #!/usr/bin/env node whose interpreter is not in the search list, with a node canary only in the
// rest of the PATH; Windows, where host_service.claude_bin must be an .exe: a path that does not
// resolve): no claude profile, claude_profile_unavailable and auth_status_unchecked, a claude
// token gives 4104 profile_unknown, and the node canary never runs.
func TestPlatform_TMT29l(t *testing.T) {
	track(t)
	resetStartLog(t)
	fake := fakeClaude(t)
	wb := programDir("work-sam")
	mkdirOwned(t, wb, userB)
	parkExisting(t, filepath.Join(sharedBin(), exeName("claude")))
	shared := canary(t, sharedBin(), "claude")
	own := canary(t, programDir("sam-bin"), "claude")
	workdirs := workdirsYAML(map[string][][2]string{userB: {{"Sam", wb}}})
	w := newWorld(t, "organization", claudeYAML(t)+workdirs, userB)
	setLoginEnv(t, userB, loginEnv{PathDir: filepath.Dir(own)})
	writeAPIKey(t)
	claudeOffered(w, userB)
	s := w.open(claudeOf(userB, 0))
	firstInteractive(t)
	exitClaude(s)
	if !ran(t, fake) || ran(t, shared) || ran(t, own) {
		t.Errorf("started: host_service.claude_bin %v, canary in the shared PATH %v, canary in Sam's PATH %v", ran(t, fake), ran(t, shared), ran(t, own))
	}

	parkExisting(t, filepath.Join(sharedBin(), exeName("node")))
	node := canary(t, sharedBin(), "node")
	claudeBin := filepath.Join(programDir("script"), exeName("claude"))
	if runtime.GOOS == "windows" {
		_ = os.Remove(claudeBin)
	} else {
		for _, d := range []string{"/usr/bin", "/bin"} {
			if _, err := os.Stat(filepath.Join(d, "node")); err == nil {
				t.Fatalf("this runner has node in %s, which is in the search list of HD6; the case needs a runner without it", d)
			}
		}
		if err := os.MkdirAll(filepath.Dir(claudeBin), 0o755); err != nil { //nolint:gosec // G301: the folder of a program
			t.Fatal(err)
		}
		if err := os.WriteFile(claudeBin, []byte("#!/usr/bin/env node\n"), 0o755); err != nil { //nolint:gosec // G306: a program
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.Remove(claudeBin) })
	}
	mark := w.srv.ReportCount()
	writeConfig(t, "  session_users: ["+userB+"]\n  profiles: [shell, claude]\n  claude_bin: '"+claudeBin+"'\n"+workdirs)
	w.restart()
	r := w.report(mark, "no claude profile", func(r sessiontest.Report) bool {
		return !r.HasProfile("claude") && r.Has(hd.HD7, hd.CodeClaudeProfileUnavailable)
	})
	if !r.Has(hd.HD6, hd.CodeAuthStatusUnchecked) {
		t.Error("report lacks auth_status_unchecked")
	}
	w.refused(claudeOf(userB, 0), "profile_unknown")
	if ran(t, node) {
		t.Error("the node canary ran")
	}
}
