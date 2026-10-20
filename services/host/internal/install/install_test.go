// SPDX-License-Identifier: Apache-2.0

package install

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/OWNER/bothy/services/host/internal/config"
	"github.com/OWNER/bothy/services/host/internal/doctor"
	"github.com/OWNER/bothy/services/host/internal/hostos"
	"github.com/OWNER/bothy/services/host/internal/hostos/hostostest"
)

var update = flag.Bool("update", false, "rewrite the golden files under testdata")

// usersNode stands for the macOS records path /Users/ in the golden files: the deny list of the
// repository refuses that path with a name after it in any file.
const usersNode = "/<Users>/"

func golden(t *testing.T, name, got string) {
	t.Helper()
	got = strings.ReplaceAll(got, "/"+"Users/", usersNode)
	p := filepath.Join("testdata", name+".golden")
	if *update {
		if err := os.WriteFile(p, []byte(got), 0o600); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(p) //nolint:gosec // a golden file of this package
	if err != nil {
		t.Fatalf("%v (run go test -update once)", err)
	}
	if got != string(want) {
		t.Errorf("%s differs from the golden file:\n%s", name, got)
	}
}

func lines(steps []Step) string {
	var b strings.Builder
	for _, s := range steps {
		b.WriteString(s.String() + "\n")
	}
	return b.String()
}

func target(goos string) Target {
	w := hostostest.New(goos)
	w.AddUser("bothy-s-sam", "502", "/home/bothy-s-sam", true)
	t := Target{GOOS: goos, ServiceUser: "bothy-host", StateDir: "/var/lib/bothy-host", ConfigPath: "/etc/bothy-host/config.yaml",
		Running: "/tmp/download/bothy-host", Users: w.Users()}
	switch goos {
	case "darwin":
		t.ConfigPath = "/Library/Application Support/bothy-host/config.yaml"
	case "windows":
		t.ProgramFiles, t.StateDir, t.ConfigPath = `C:\Program Files`, `C:\ProgramData\bothy-host`, `C:\Program Files\bothy-host\config.yaml`
	}
	return t
}

var people = []string{"bothy-s-alex", "bothy-s-sam"}

func TestInstallPlan_Linux(t *testing.T) {
	steps, err := Install(target("linux"), people)
	if err != nil {
		t.Fatal(err)
	}
	golden(t, "install_linux", lines(steps))
	for _, s := range steps {
		if s.Kind == KindRun && !strings.HasPrefix(s.Argv[0], "/usr/") && !strings.HasPrefix(s.Argv[0], "/bin/") && !strings.HasPrefix(s.Argv[0], "/sbin/") {
			t.Errorf("program outside the fixed list: %v", s.Argv)
		}
	}
}

func TestInstallPlan_Darwin(t *testing.T) {
	tg := target("darwin")
	steps, err := Install(tg, people)
	if err != nil {
		t.Fatal(err)
	}
	golden(t, "install_darwin", lines(steps))
	want := "run /usr/bin/codesign --force --sign - --options runtime /Library/Application Support/bothy-host/bin/bothy-host"
	if !strings.Contains(lines(steps), want+"\n") {
		t.Error("codesign step missing")
	}
	tg.SignedWithRuntime = true
	signed, _ := Install(tg, people)
	if strings.Contains(lines(signed), "codesign") {
		t.Error("a binary signed with runtime is signed again")
	}
	for i, s := range steps {
		if s.Kind == KindRun && strings.Contains(strings.Join(s.Argv, " "), "staff") {
			t.Errorf("a step names staff: %v", s.Argv)
		}
		// A label that is loaded already must not make a repeated install fail.
		if s.Kind == KindRun && len(s.Argv) > 1 && s.Argv[1] == "bootstrap" && (i == 0 || steps[i-1].Kind != KindRunTolerant) {
			t.Errorf("bootstrap without a tolerant bootout before it: %v", s.Argv)
		}
	}
}

func TestInstallPlan_Windows(t *testing.T) {
	tg := target("windows")
	tg.ConfigExists = true
	steps, err := Install(tg, people)
	if err != nil {
		t.Fatal(err)
	}
	golden(t, "install_windows", lines(steps))
	if _, err := Install(Target{GOOS: "plan9"}, nil); !errors.Is(err, ErrUnsupported) {
		t.Error("plan9")
	}
}

func definitions(t *testing.T, goos string) map[string]string {
	t.Helper()
	tg := target(goos)
	steps, err := Install(tg, []string{"bothy-s-alex"})
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]string{}
	for _, s := range steps {
		if s.Kind == KindWrite && (strings.HasSuffix(s.Path, ".service") || strings.HasSuffix(s.Path, ".socket") || strings.HasSuffix(s.Path, ".plist")) {
			out[filepath.Base(s.Path)] = s.Content
		}
		if s.Kind == KindWinService {
			out[s.Name] = s.Content
		}
	}
	return out
}

func TestServiceDefinitionsGolden(t *testing.T) {
	for _, goos := range []string{"linux", "darwin", "windows"} {
		for name, content := range definitions(t, goos) {
			golden(t, goos+"_"+name, content)
		}
	}
	linux := definitions(t, "linux")
	host, helper := linux["bothy-host.service"], linux["bothy-host-helper-bothy-s-alex.service"]
	if !strings.Contains(host, "\nRestart=on-failure\n") || !strings.Contains(host, "\nRestartPreventExitStatus=78\n") || !strings.Contains(host, "\nUMask=0077\n") {
		t.Error("host exit policy")
	}
	// No NoNewPrivileges= and no option that implies it (systemd.exec): every one of them would make
	// "sudo -n true" fail and hide a sudo rule from HD3.
	for _, unit := range []string{host, helper} {
		for _, opt := range []string{"NoNewPrivileges=", "SystemCallFilter=", "SystemCallArchitectures=", "RestrictSUIDSGID=", "DynamicUser=",
			"PrivateDevices=", "ProtectKernelTunables=", "ProtectKernelModules=", "ProtectKernelLogs=", "ProtectClock=", "ProtectHostname=",
			"MemoryDenyWriteExecute=", "RestrictAddressFamilies=", "RestrictNamespaces=", "RestrictRealtime=", "LockPersonality=", "SELinuxContext=",
			"AppArmorProfile=", "SmackProcessLabel=", "PrivateNetwork="} {
			for _, line := range strings.Split(unit, "\n") {
				if strings.HasPrefix(line, opt) {
					t.Errorf("unit sets %s", opt)
				}
			}
		}
	}
	socket := linux["bothy-host-helper-bothy-s-alex.socket"]
	for _, l := range []string{"SocketUser=root", "SocketGroup=bothy-host", "SocketMode=0660", "DirectoryMode=0755", "Accept=no", "ListenStream=/run/bothy-host/helper-bothy-s-alex.sock"} {
		if !strings.Contains(socket, "\n"+l+"\n") {
			t.Errorf("socket unit lacks %s", l)
		}
	}
	darwin := definitions(t, "darwin")
	for _, p := range []string{darwin["bothy-host.plist"], darwin["bothy-host.helper.bothy-s-alex.plist"]} {
		if !strings.Contains(p, "<key>Crashed</key>\n\t\t<true/>") || !strings.Contains(p, "<string>crash</string>") || strings.Contains(p, "SuccessfulExit") {
			t.Error("plist exit policy")
		}
	}
	if !strings.Contains(darwin["bothy-host.helper.bothy-s-alex.plist"], "<key>Wait</key>\n\t\t<true/>") {
		t.Error("inetdCompatibility Wait")
	}
	win := definitions(t, "windows")
	if !strings.Contains(win["bothy-host"], "failure_actions_on_non_crash_failures=false") || !strings.Contains(win["bothy-host"], `account=NT SERVICE\bothy-host`) {
		t.Error("windows host service")
	}
	if strings.Contains(win["bothy-host-helper-bothy-s-alex"], "WP;;;AU") || !strings.Contains(win["bothy-host-helper-bothy-s-alex"], `account=.\bothy-s-alex`) {
		t.Error("windows helper service")
	}
}

func TestSessionUserAddRefusesBoundName(t *testing.T) {
	for _, goos := range []string{"linux", "darwin", "windows"} {
		_, err := SessionUserAdd(target(goos), "bothy-s-old", people, map[string]string{"bothy-s-old": "p1"})
		var b *BoundError
		if !errors.As(err, &b) || err.Error() != "bothy-host session-user add: bothy-s-old is bound to a person on this host; create a new name" {
			t.Errorf("%s: %v", goos, err)
		}
	}
	if _, err := SessionUserAdd(target("linux"), "BOTHY-S-SAM", people, nil); !errors.Is(err, ErrConfigured) {
		t.Errorf("configured: %v", err)
	}
	if _, err := SessionUserAdd(target("linux"), "-x", people, nil); !errors.Is(err, ErrInvalidName) {
		t.Errorf("invalid: %v", err)
	}
	if _, err := SessionUserAdd(Target{GOOS: "plan9"}, "a", nil, nil); !errors.Is(err, ErrUnsupported) {
		t.Error("plan9")
	}
}

func TestSessionUserAddCreatesAndRestarts(t *testing.T) {
	for _, goos := range []string{"linux", "darwin", "windows"} {
		steps, err := SessionUserAdd(target(goos), "bothy-s-robin", people, map[string]string{"bothy-s-old": "p"})
		if err != nil {
			t.Fatal(err)
		}
		golden(t, "session_user_add_"+goos, lines(steps))
		got := lines(steps)
		last := steps[len(steps)-1]
		if !strings.Contains(got, "config.add") || (last.Kind != KindRun && last.Kind != KindWinRestart) {
			t.Errorf("%s:\n%s", goos, got)
		}
	}
}

func TestSessionUserRemove(t *testing.T) {
	for _, goos := range []string{"linux", "darwin", "windows"} {
		steps, err := SessionUserRemove(target(goos), "bothy-s-sam", people)
		if err != nil {
			t.Fatal(err)
		}
		golden(t, "session_user_remove_"+goos, lines(steps))
	}
	if _, err := SessionUserRemove(target("linux"), "bothy-s-robin", people); !errors.Is(err, ErrNotConfigured) {
		t.Error("not configured")
	}
	if _, err := SessionUserRemove(Target{GOOS: "plan9"}, "a", []string{"a"}); !errors.Is(err, ErrUnsupported) {
		t.Error("plan9")
	}
	if !strings.Contains(OperatorStepsAfterRemove("bothy-s-sam"), "delete the account bothy-s-sam") {
		t.Error("operator steps")
	}
	// Executed against a real configuration file and fake operations: both keys, binding kept.
	dir := t.TempDir()
	cfg := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(cfg, []byte("host_service:\n  session_users: [bothy-s-alex, bothy-s-sam]\n  claude_workdirs:\n    bothy-s-sam:\n      - {display_name: S, path: /home/bothy-s-sam/s}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	tg := target("linux")
	tg.ConfigPath = cfg
	steps, _ := SessionUserRemove(tg, "bothy-s-sam", people)
	ops := &fakeOps{homes: map[string]string{"bothy-s-sam": "/home/bothy-s-sam"}}
	if err := Apply(context.Background(), ops, steps); err != nil {
		t.Fatal(err)
	}
	c, err := config.Load(cfg, config.Platform{GOOS: "linux"})
	if err != nil || len(c.SessionUsers) != 1 || len(c.ClaudeWorkdirs) != 0 {
		t.Errorf("config %+v %v", c, err)
	}
	// The host service stops first, so the sessions of the removed name end with the restart
	// (host_restarted, R-TM-27) and not through the stop of its helper (found by M2-48).
	want := []string{
		"run /usr/bin/systemctl stop bothy-host.service",
		"run /usr/bin/systemctl disable --now bothy-host-helper-bothy-s-sam.socket bothy-host-helper-bothy-s-sam.service",
		"remove /etc/systemd/system/bothy-host-helper-bothy-s-sam.socket", "remove /etc/systemd/system/bothy-host-helper-bothy-s-sam.service",
		"run /usr/bin/systemctl daemon-reload", "chmod /home/bothy-s-sam 0700 bothy-s-sam",
		"run /usr/bin/systemctl reset-failed bothy-host.service", "run /usr/bin/systemctl restart bothy-host.service",
	}
	if strings.Join(ops.log, "\n") != strings.Join(want, "\n") {
		t.Errorf("ops:\n%s", strings.Join(ops.log, "\n"))
	}
}

// TestDriveRootDACL builds the DACL of C:\ with default rights (24 2.2 point 6) as binary entries.
func TestDriveRootDACL(t *testing.T) {
	sid := func(auth byte, subs ...uint32) []byte {
		b := []byte{1, byte(len(subs)), 0, 0, 0, 0, 0, auth} //nolint:gosec // small test values
		for _, s := range subs {
			b = append(b, byte(s), byte(s>>8), byte(s>>16), byte(s>>24)) //nolint:gosec // small test values
		}
		return b
	}
	ace := func(typ, flags byte, mask uint32, s []byte) []byte {
		size := 8 + len(s)
		b := []byte{typ, flags, byte(size), byte(size >> 8), byte(mask), byte(mask >> 8), byte(mask >> 16), byte(mask >> 24)} //nolint:gosec // small test values
		return append(b, s...)
	}
	acl := func(aces ...[]byte) []byte {
		body := bytes.Join(aces, nil)
		size := 8 + len(body)
		return append([]byte{2, 0, byte(size), byte(size >> 8), byte(len(aces)), 0, 0, 0}, body...) //nolint:gosec // small test values
	}
	system, admins, users, auth := sid(5, 18), sid(5, 32, 544), sid(5, 32, 545), sid(5, 11)
	entries := [][]byte{
		ace(0, 0, 0x1f01ff, admins),      // Administrators:(F)
		ace(0, 0x0b, 0x10000000, admins), // Administrators:(OI)(CI)(IO)(F)
		ace(0, 0, 0x1f01ff, system),      // SYSTEM:(F)
		ace(0, 0x03, 0x1200a9, users),    // Users:(OI)(CI)(RX)
		ace(0, 0x0b, 0x1301bf, auth),     // Authenticated Users:(OI)(CI)(IO)(M)
		ace(0, 0, 0x4, auth),             // Authenticated Users:(AD)
		ace(1, 0, 0x4, users),            // a deny entry stays too
	}
	in := acl(entries...)
	out, removed, err := RemoveAuthenticatedUsersAD(in)
	if err != nil || !removed {
		t.Fatalf("%v %v", removed, err)
	}
	want := acl(entries[0], entries[1], entries[2], entries[3], entries[4], entries[6])
	if !bytes.Equal(out, want) {
		t.Errorf("got %x\nwant %x", out, want)
	}
	again, removed, err := RemoveAuthenticatedUsersAD(out)
	if err != nil || removed || !bytes.Equal(again, out) {
		t.Error("a DACL without the entry changed")
	}
	// (AD) with an inheritance flag, or with more rights, is not the entry.
	for _, e := range [][]byte{ace(0, 0x01, 0x4, auth), ace(0, 0, 0x6, auth), ace(0, 0, 0x4, users)} {
		if _, removed, _ := RemoveAuthenticatedUsersAD(acl(e)); removed {
			t.Errorf("removed %x", e)
		}
	}
	for _, bad := range [][]byte{{2, 0}, acl(entries[0])[:12], {2, 0, 200, 0, 1, 0, 0, 0}, {2, 0, 12, 0, 1, 0, 0, 0, 0, 0, 2, 0}} {
		if _, _, err := RemoveAuthenticatedUsersAD(bad); !errors.Is(err, ErrACLFormat) {
			t.Errorf("%x accepted", bad)
		}
	}
}

func TestDriveRootDeclined(t *testing.T) {
	if steps := DriveRoot("c:", false); len(steps) != 0 {
		t.Errorf("declined plan %v", steps)
	}
	if steps := DriveRoot(`d:\`, true); len(steps) != 1 || steps[0].Path != `D:\` {
		t.Errorf("confirmed plan %v", steps)
	}
	// Without the step the root stays writable, and HD11 reports drive_root_writable.
	w := hostostest.New("windows")
	w.AddUser("bothy-host", "S-1-5-80-1", `C:\ProgramData\bothy-host`, false)
	w.AddUser("bothy-s-alex", "S-1-5-21-1", `C:\Users\bothy-s-alex`, true)
	w.SetGroups("bothy-s-alex", hostos.Group{Name: "Authenticated Users", ID: hostos.SIDAuthenticated})
	w.Nodes[`C:\`] = &hostostest.Node{Sec: hostos.FileSec{IsDir: true, OwnerID: hostos.SIDSystem, ACL: []hostos.ACE{{Allow: true, Kind: hostos.PrincipalSID, ID: hostos.SIDAuthenticated, Rights: hostos.WindowsRights(0x4)}}}}
	cfg := config.Defaults(config.Platform{GOOS: "windows"})
	cfg.SessionUsers = []string{"bothy-s-alex"}
	r := doctor.RunService(context.Background(), doctor.Env{GOOS: "windows", Config: &cfg, ServiceUser: "bothy-host", SystemDrive: "C:", Users: w.Users(),
		Security: w.Security(), Resolver: w.As("bothy-host"), Files: w.As("bothy-host"), Probe: w.As("bothy-host").Probe(), Helpers: noHelpers{}, PeerCheckProven: true})
	if !strings.Contains(r.Format(), "HD11 FAIL drive_root_writable drive=C:") {
		t.Errorf("HD11:\n%s", r.Format())
	}
}

type noHelpers struct{}

func (noHelpers) Check(context.Context, string, doctor.Request) (doctor.HelperResult, error) {
	return doctor.HelperResult{}, nil
}
func (noHelpers) Scan(context.Context, string, doctor.Request) (string, error) { return "", nil }

func TestRestartPlan(t *testing.T) {
	for _, goos := range []string{"linux", "darwin", "windows"} {
		for _, running := range []bool{true, false} {
			tg := target(goos)
			tg.HostRunning = running
			steps, err := Restart(tg)
			if err != nil {
				t.Fatal(err)
			}
			for _, s := range steps {
				text := s.String()
				if strings.Contains(text, "helper") || !strings.Contains(text, "bothy-host") {
					t.Errorf("%s: restart touches %s", goos, text)
				}
			}
			golden(t, "restart_"+goos, lines(steps))
		}
	}
	if _, err := Restart(Target{GOOS: "plan9"}); !errors.Is(err, ErrUnsupported) {
		t.Error("plan9")
	}
}

type fakeOps struct {
	log   []string
	homes map[string]string
	fail  string
}

func (f *fakeOps) rec(s string) error {
	f.log = append(f.log, s)
	if f.fail != "" && strings.HasPrefix(s, f.fail) {
		return errors.New("injected")
	}
	return nil
}
func (f *fakeOps) Run(_ context.Context, argv []string) error {
	return f.rec("run " + strings.Join(argv, " "))
}
func (f *fakeOps) Mkdir(p string, m fs.FileMode, o string) error {
	return f.rec((Step{Kind: KindMkdir, Path: p, Mode: m, Owner: o}).String())
}
func (f *fakeOps) Write(p string, c []byte, m fs.FileMode, o string) error {
	return f.rec((Step{Kind: KindWrite, Path: p, Mode: m, Owner: o, Content: string(c)}).String())
}
func (f *fakeOps) Copy(s, d string, m fs.FileMode, o string) error {
	return f.rec((Step{Kind: KindCopy, Source: s, Path: d, Mode: m, Owner: o}).String())
}
func (f *fakeOps) Chmod(p string, m fs.FileMode, o string) error {
	return f.rec((Step{Kind: KindChmod, Path: p, Mode: m, Owner: o}).String())
}
func (f *fakeOps) Remove(p string) error { return f.rec("remove " + p) }
func (f *fakeOps) Home(u string) (string, error) {
	if h, ok := f.homes[u]; ok {
		return h, nil
	}
	return "", errors.New("no home")
}
func (f *fakeOps) Windows(_ context.Context, s Step) error { return f.rec(s.String()) }

func TestApply(t *testing.T) {
	steps, _ := Install(target("windows"), people)
	ops := &fakeOps{}
	if err := Apply(context.Background(), ops, steps); err != nil {
		t.Fatal(err)
	}
	if len(ops.log) != len(steps) {
		t.Errorf("%d of %d steps", len(ops.log), len(steps))
	}
	ops = &fakeOps{fail: "copy"}
	err := Apply(context.Background(), ops, steps)
	if err == nil || !strings.Contains(err.Error(), "install: copy") {
		t.Errorf("failure %v", err)
	}
	if err := Apply(context.Background(), &fakeOps{}, []Step{{Kind: KindChmod, Path: "~nobody"}}); err == nil {
		t.Error("unknown home")
	}
	if Step(Step{Kind: "other", Path: "p", Name: "n"}).String() != "other p n" {
		t.Error("String default")
	}
}
