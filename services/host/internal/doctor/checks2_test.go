// SPDX-License-Identifier: Apache-2.0

package doctor

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/host/internal/config"
	"github.com/OWNER/bothy/services/host/internal/hostos"
	"github.com/OWNER/bothy/services/host/internal/hostos/hostostest"
	"github.com/OWNER/bothy/services/host/internal/state"
)

func workdir(x *tw, user, name, p string) {
	if x.w.Node(p) == nil {
		x.w.Dir(p, x.w.UID(user), 0o700)
	}
	for i := range x.cfg.ClaudeWorkdirs {
		if x.cfg.ClaudeWorkdirs[i].SessionUser == user {
			x.cfg.ClaudeWorkdirs[i].Dirs = append(x.cfg.ClaudeWorkdirs[i].Dirs, config.Workdir{DisplayName: name, Path: p})
			return
		}
	}
	x.cfg.ClaudeWorkdirs = append(x.cfg.ClaudeWorkdirs, config.WorkdirList{SessionUser: user, Dirs: []config.Workdir{{DisplayName: name, Path: p}}})
}

func reported(r *Result, user string) []string {
	for _, u := range r.SessionUsers {
		if u.OSUser == user {
			return u.Workdirs
		}
	}
	return nil
}

func TestHD7(t *testing.T) {
	x := newTW(t)
	x.cfg.Profiles = []config.Profile{{Name: "shell", DisplayName: "Shell", Kind: "shell"}, {Name: "claude", DisplayName: "Claude Code", Kind: "claude"},
		{Name: "top", DisplayName: "Top", Kind: "command", Program: "/usr/bin/top"}}
	x.w.File("/usr/bin/top", "0", 0o755, "")
	workdir(x, alex, "Project A", "/home/"+alex+"/a")
	workdir(x, sam, "Project S", "/home/"+sam+"/s")
	r := x.run()
	if len(codes(r, HD7)) != 0 || len(r.Profiles) != 3 || !reflect.DeepEqual(reported(r, alex), []string{"Project A"}) || len(reported(r, robin)) != 0 {
		t.Fatalf("baseline %v %+v %+v", codes(r, HD7), r.Profiles, r.SessionUsers)
	}

	// Each drop rule.
	for name, tc := range map[string]struct {
		setup func(x *tw)
		code  string
	}{
		"key not a session user":                      {func(x *tw) { workdir(x, "bothy-s-ghost", "Ghost", "/srv/ghost") }, CodeWorkdirDropped},
		"unresolvable folder":                         {func(x *tw) { x.cfg.ClaudeWorkdirs[0].Dirs[0].Path = "/home/" + alex + "/missing" }, CodeWorkdirDropped},
		"folder of another user closed to the helper": {func(x *tw) { workdir(x, alex, "Peek", "/home/"+sam+"/s") }, CodeWorkdirDropped},
		"empty display name":                          {func(x *tw) { x.cfg.ClaudeWorkdirs[0].Dirs[0].DisplayName = "" }, CodeWorkdirDropped},
		"repeated display name":                       {func(x *tw) { workdir(x, alex, "Project A", "/home/"+alex+"/b") }, CodeWorkdirDropped},
		"claude without claude_bin":                   {func(x *tw) { x.cfg.ClaudeBin = "" }, CodeClaudeProfileUnavailable},
		"command not absolute":                        {func(x *tw) { x.cfg.Profiles[2].Program = "bin/top" }, CodeProfileDropped},
		"command missing":                             {func(x *tw) { x.cfg.Profiles[2].Program = "/usr/bin/nothing" }, CodeProfileDropped},
		"precedence": {func(x *tw) {
			x.cfg.ClaudeBin = ""
			x.cfg.Profiles[2].Program = "x"
			x.cfg.ClaudeWorkdirs[0].Dirs[0].DisplayName = ""
		}, CodeWorkdirDropped},
	} {
		x := newTW(t)
		x.cfg.Profiles = []config.Profile{{Name: "shell", DisplayName: "Shell", Kind: "shell"}, {Name: "claude", DisplayName: "Claude Code", Kind: "claude"},
			{Name: "top", DisplayName: "Top", Kind: "command", Program: "/usr/bin/top"}}
		x.w.File("/usr/bin/top", "0", 0o755, "")
		workdir(x, alex, "Project A", "/home/"+alex+"/a")
		tc.setup(x)
		r := x.run()
		if got := codes(r, HD7); len(got) != 1 || got[0] != tc.code || r.StartRefused() {
			t.Errorf("%s: %v", name, got)
		}
	}

	// A folder under A and B (through a link and in another case) drops from both lists.
	x = newTW(t)
	x.w.Dir("/srv/shared", "0", 0o777).ID = "dev:42"
	x.w.Link("/home/"+alex+"/shared", "1001", "/srv/shared")
	workdir(x, alex, "Mine", "/home/"+alex+"/mine")
	x.cfg.ClaudeWorkdirs[0].Dirs = append(x.cfg.ClaudeWorkdirs[0].Dirs, config.Workdir{DisplayName: "Shared", Path: "/home/" + alex + "/shared"})
	x.w.Dir("/srv/SHARED", "0", 0o777).ID = "dev:42"
	x.cfg.ClaudeWorkdirs = append(x.cfg.ClaudeWorkdirs, config.WorkdirList{SessionUser: sam, Dirs: []config.Workdir{{DisplayName: "Shared", Path: "/srv/SHARED"}}})
	r = x.run()
	if !reflect.DeepEqual(reported(r, alex), []string{"Mine"}) || len(reported(r, sam)) != 0 || !has(codes(r, HD7), CodeWorkdirDropped) {
		t.Errorf("shared folder: %+v %v", r.SessionUsers, codes(r, HD7))
	}
	if len(r.Workdirs[alex]) != 1 || r.Workdirs[alex][0].Path != "/home/"+alex+"/mine" {
		t.Errorf("lists after HD7 %+v", r.Workdirs)
	}
	if !strings.Contains(r.Format(), "item=Shared") {
		t.Errorf("local output lacks the item:\n%s", r.Format())
	}
	// Declarations: none drops claude silently, personal keeps it without claude_bin.
	x = newTW(t)
	x.cfg.Profiles = []config.Profile{{Name: "claude", DisplayName: "Claude Code", Kind: "claude"}}
	x.cfg.ClaudeBin = ""
	r = x.run()
	if len(r.WithDeclaration(state.DeclNone).Profiles) != 0 || len(codes(r.WithDeclaration(state.DeclNone), HD7)) != 0 {
		t.Error("none keeps claude")
	}
	if p := r.WithDeclaration(state.DeclPersonal); len(p.Profiles) != 1 || len(codes(p, HD7)) != 0 {
		t.Error("personal drops claude")
	}
	if u := r.WithDeclaration(state.DeclUnknown); len(u.Profiles) != 0 || !has(codes(u, HD7), CodeClaudeProfileUnavailable) {
		t.Error("unknown keeps claude without claude_bin")
	}
}

func TestHD8(t *testing.T) {
	for name, setup := range map[string]func(x *tw){
		"group of the dir": func(x *tw) {
			x.w.Node("/var/lib/bothy-host").Sec.GroupID = "1002"
			x.w.Node("/var/lib/bothy-host").Sec.Mode = 0o750
		},
		"ACL on the key": func(x *tw) {
			x.w.Node("/var/lib/bothy-host/keys").Sec.ACL = []hostos.ACE{{Allow: true, Kind: hostos.PrincipalUser, ID: "1003", Rights: hostos.RightRead}}
		},
		"owner session user": func(x *tw) { x.w.Node("/var/lib/bothy-host/keys").Sec.OwnerID = "1001" },
	} {
		x := newTW(t)
		setup(x)
		if r := x.run(); !has(codes(r, HD8), CodeStateAccessForSessionUser) || !r.StartRefused() {
			t.Errorf("%s: %v", name, codes(r, HD8))
		}
	}
	// A helper that can open the folder: the helper part.
	x := newTW(t)
	x.w.Node("/var/lib/bothy-host").Sec.Mode = 0o705
	r := x.run()
	if !has(codes(r, HD8), CodeHelperReadsState) {
		t.Errorf("helper part: %v", codes(r, HD8))
	}
}

func TestHD9(t *testing.T) {
	// daemon: UID 1, home /usr/sbin listable, skipped.
	x := newTW(t)
	r := x.run()
	if len(codes(r, HD9)) != 0 {
		t.Fatalf("system account checked: %v", codes(r, HD9))
	}
	req := x.requestFor(alex)
	for _, h := range req.ForeignHomes {
		if h.Name == "daemon" || h.Name == alex || h.Name == sam {
			t.Errorf("home of %s listed by HD9", h.Name)
		}
	}
	names := map[string]bool{}
	for _, h := range req.ForeignHomes {
		names[h.Name] = true
	}
	if !names["root"] || !names[svc] {
		t.Errorf("root and the service user must be checked: %v", names)
	}
	// A person account with a listable home fails, unnamed in the report, named locally.
	x = newTW(t)
	x.w.AddUser("casey", "1500", "/home/casey", true)
	x.w.Dir("/home/casey", "1500", 0o755)
	r = x.run()
	if !has(codes(r, HD9), CodeForeignHomeListable) || !r.StartRefused() || !strings.Contains(r.Format(), "user=casey") {
		t.Errorf("person account: %v\n%s", codes(r, HD9), r.Format())
	}
	for _, e := range r.Report() {
		if e.Check == HD9 && e.SessionUser != "" {
			t.Errorf("person account named in the report: %+v", e)
		}
	}
	// A former session user: named in the report and locally.
	x = newTW(t)
	x.w.AddUser("bothy-s-old", "1600", "/home/bothy-s-old", true)
	x.w.Dir("/home/bothy-s-old", "1600", 0o755)
	x.env.Bindings = map[string]string{"bothy-s-old": "p-old", alex: "p-alex"}
	r = x.run()
	if !has(codes(r, HD9), CodeForeignHomeListable+"@bothy-s-old") || !strings.Contains(r.Format(), "user=bothy-s-old") {
		t.Errorf("former: %v", codes(r, HD9))
	}
	// The service user with a system UID and root are checked.
	x = newTW(t)
	x.w.Node("/var/lib/bothy-host").Sec.Mode = 0o705
	if r := x.run(); !has(codes(r, HD9), CodeForeignHomeListable) || !strings.Contains(r.Format(), "user="+svc) {
		t.Errorf("service user: %v", codes(r, HD9))
	}
	x = newTW(t)
	x.w.Node("/root").Sec.Mode = 0o755
	if r := x.run(); !strings.Contains(r.Format(), "user=root") {
		t.Errorf("root: %v", codes(r, HD9))
	}
	// A macOS system account (UID below 501) with a 0755 home is skipped.
	x = newTW(t)
	x.w.AddUser("_www", "70", "/Library/WebServer", false)
	x.w.Dir("/Library/WebServer", "70", 0o755)
	if r := x.run(); len(codes(r, HD9)) != 0 {
		t.Errorf("_www: %v", codes(r, HD9))
	}
	// Without the right it passes.
	x = newTW(t)
	x.w.AddUser("casey", "1500", "/home/casey", true)
	x.w.Dir("/home/casey", "1500", 0o700)
	if r := x.run(); len(codes(r, HD9)) != 0 {
		t.Errorf("closed: %v", codes(r, HD9))
	}
}

// requestFor builds the check.run request of a session user like RunService.
func (x *tw) requestFor(user string) Request {
	r := &run{env: x.env, res: &Result{}}
	r.users = x.cfg.SessionUsers
	return r.request(user)
}

func TestHD9PersonAccounts(t *testing.T) {
	min, max := hostos.ParseLoginDefs([]byte("UID_MIN 2000\n"))
	if min != 2000 || max != hostos.DefaultUIDMax || hostos.PersonUID("linux", "alex", 1000, min, max) {
		t.Error("login.defs fixture")
	}
	if !hostos.PersonUID("darwin", "alex", 501, 0, 0) || !hostos.PersonUID("linux", "alex", 1000, hostos.DefaultUIDMin, hostos.DefaultUIDMax) {
		t.Error("person accounts")
	}
}

func TestHD10(t *testing.T) {
	for name, tc := range map[string]struct {
		setup func(x *tw)
		code  string
	}{
		"helper binary owned by a session user": {func(x *tw) { x.w.Node("/opt/bothy-host/bin/bothy-host").Sec.OwnerID = "1001" }, CodeHelperUntrusted},
		"helper folder writable":                {func(x *tw) { x.w.Node("/opt/bothy-host/bin").Sec.Mode = 0o777 }, CodeHelperUntrusted},
		"helper setuid":                         {func(x *tw) { x.w.Node("/opt/bothy-host/bin/bothy-host").Sec.Mode |= 1 << 23 }, CodeHelperUntrusted},
		"helper missing":                        {func(x *tw) { x.w.Remove("/opt/bothy-host/bin/bothy-host") }, CodeHelperUntrusted},
		"service definition writable": {func(x *tw) {
			x.w.Node("/etc/systemd/system/bothy-host.service").Sec.ACL = []hostos.ACE{{Allow: true, Kind: hostos.PrincipalGroup, ID: "1002", Rights: hostos.RightAppend}}
		}, CodeHelperUntrusted},
		"endpoint folder owned by another":    {func(x *tw) { x.w.Node("/run/bothy-host").Sec.OwnerID = "999" }, CodeHelperUntrusted},
		"configuration file writable":         {func(x *tw) { x.w.Node("/etc/bothy-host/config.yaml").Sec.Mode = 0o646 }, CodeHelperUntrusted},
		"unmappable ACL entry":                {func(x *tw) { x.w.Node("/etc/bothy-host").Sec.ACL = []hostos.ACE{{Unmappable: true}} }, CodeHelperUntrusted},
		"claude_bin writable":                 {func(x *tw) { x.w.Node("/opt/claude/bin/claude").Sec.Mode = 0o757 }, CodeClaudeBinUntrusted},
		"claude_bin folder of a session user": {func(x *tw) { x.w.Node("/opt/claude").Sec.OwnerID = "1002" }, CodeClaudeBinUntrusted},
		"claude_bin through a link into A": {func(x *tw) {
			x.w.File("/home/"+alex+"/claude", "1001", 0o755, "\x7fELF")
			x.w.Node("/home/" + alex).Sec.Mode = 0o711
			x.w.Link("/opt/claude/bin/claude", "0", "/home/"+alex+"/claude")
		}, CodeClaudeBinUntrusted},
		"search list folder writable": {func(x *tw) {
			x.w.Node("/usr/bin").Sec.ACL = []hostos.ACE{{Allow: true, Kind: hostos.PrincipalEveryone, Rights: hostos.RightWriteData}}
		}, CodeClaudeBinUntrusted},
		"interpreter writable": {func(x *tw) {
			x.claudeBin("/opt/claude/bin/claude", "#!/usr/bin/env node\n")
			x.w.File("/usr/bin/env", "0", 0o755, "")
			x.w.File("/usr/bin/node", "1002", 0o755, "")
		}, CodeClaudeBinUntrusted},
		"env writable": {func(x *tw) {
			x.claudeBin("/opt/claude/bin/claude", "#!/usr/bin/env -S node --x\n")
			x.w.File("/usr/bin/env", "0", 0o777, "")
			x.w.File("/usr/bin/node", "0", 0o755, "")
		}, CodeClaudeBinUntrusted},
		"direct interpreter writable": {func(x *tw) {
			x.claudeBin("/opt/claude/bin/claude", "#!/opt/node/bin/node\n")
			x.w.File("/opt/node/bin/node", "1001", 0o755, "")
		}, CodeClaudeBinUntrusted},
		"ptrace scope":           {func(x *tw) { x.w.Ptrace = 0 }, CodePtraceScopeLow},
		"peer check unavailable": {func(x *tw) { x.env.PeerCheckProven = false }, CodePeerCheckUnavailable},
		"peer mismatch":          {func(x *tw) { x.peer[sam] = fmt.Errorf("dial: %w", ErrPeerMismatch) }, CodePeerMismatch + "@" + sam},
		"helper unreachable":     {func(x *tw) { x.peer[sam] = errors.New("connection refused") }, CodeHelperUnreachable + "@" + sam},
	} {
		x := newTW(t)
		tc.setup(x)
		r := x.run()
		if !has(codes(r, HD10), tc.code) || !r.StartRefused() {
			t.Errorf("%s: %v", name, codes(r, HD10))
		}
	}
	// A script with its interpreter in the search list, all trusted, passes; the checked node is
	// the one of the search list.
	x := newTW(t)
	x.claudeBin("/opt/claude/bin/claude", "#!/usr/bin/env node\n")
	x.w.File("/usr/bin/env", "0", 0o755, "")
	x.w.File("/opt/claude/bin/node", "0", 0o755, "")
	if r := x.run(); len(codes(r, HD10)) != 0 || len(codes(r, HD6)) != 0 {
		t.Errorf("script: %v %v", codes(r, HD10), codes(r, HD6))
	}
	// Linux: /bin writable with the checked node in the folder of claude_bin fails.
	x.w.Node("/bin").Sec.Mode = 0o777
	if r := x.run(); !has(codes(r, HD10), CodeClaudeBinUntrusted) {
		t.Errorf("/bin writable: %v", codes(r, HD10))
	}
	// Windows: a .cmd fails, an .exe passes.
	x = newTW(t)
	x.env.GOOS = "windows"
	x.claudeBin("/opt/claude/bin/claude.cmd", "@echo off")
	x.w.Node("/opt/claude/bin/claude.cmd").Sec.OwnerID = hostos.SIDSystem
	if r := x.run(); !has(codes(r, HD10), CodeClaudeBinUntrusted) {
		t.Errorf(".cmd: %v", codes(r, HD10))
	}
	// macOS: the hardened runtime flag and get-task-allow.
	for name, info := range map[string]CodesignInfo{"adhoc,linker-signed": {Runtime: false}, "get-task-allow": {Runtime: true, GetTaskAllow: true}} {
		x := newTW(t)
		x.env.GOOS = "darwin"
		x.env.Codesign = func(context.Context, string) (CodesignInfo, error) { return info, nil }
		if r := x.run(); !has(codes(r, HD10), CodeHelperUnprotected) {
			t.Errorf("%s: %v", name, codes(r, HD10))
		}
	}
	x = newTW(t)
	x.env.GOOS = "darwin"
	x.env.Codesign = func(context.Context, string) (CodesignInfo, error) { return CodesignInfo{Runtime: true}, nil }
	if r := x.run(); len(codes(r, HD10)) != 0 {
		t.Errorf("signed with runtime: %v", codes(r, HD10))
	}
}

func TestHD11(t *testing.T) {
	for name, tc := range map[string]struct {
		setup func(x *tw)
		user  string
		code  string
	}{
		"place writable by another": {func(x *tw) {
			workdir(x, sam, "S", "/home/"+sam+"/s")
			x.w.Node("/home/" + sam + "/s").Sec.ACL = []hostos.ACE{{Allow: true, Kind: hostos.PrincipalUser, ID: "1001", Rights: hostos.RightWriteData}}
		}, sam, CodeSharedLocation},
		"place owned by another": {func(x *tw) {
			workdir(x, sam, "S", "/home/"+sam+"/s")
			x.w.Node("/home/" + sam + "/s").Sec.OwnerID = "1001"
		}, sam, CodeSharedLocation},
		"folder on the way writable by another": {func(x *tw) {
			x.w.Dir("/srv/work", "0", 0o777)
			workdir(x, sam, "S", "/srv/work/s")
		}, sam, CodeSharedLocation},
		"chain file of another above": {func(x *tw) {
			x.w.Dir("/srv/work", "0", 0o755)
			x.w.File("/srv/work/CLAUDE.md", "1001", 0o644, "EXAMPLE instructions")
			workdir(x, sam, "S", "/srv/work/s")
		}, sam, CodeSharedLocation},
		"chain file of root writable by another": {func(x *tw) {
			x.w.Dir("/srv/work", "0", 0o755)
			x.w.File("/srv/work/AGENTS.md", "0", 0o646, "")
			workdir(x, sam, "S", "/srv/work/s")
		}, sam, CodeSharedLocation},
		".claude folder of another": {func(x *tw) {
			x.w.Dir("/srv/work", "0", 0o755)
			x.w.Dir("/srv/work/.claude", "0", 0o755)
			x.w.File("/srv/work/.claude/settings.json", "1001", 0o644, "{}")
			workdir(x, sam, "S", "/srv/work/s")
		}, sam, CodeSharedLocation},
		"readable place": {func(x *tw) { x.w.Node("/home/" + alex).Sec.Mode = 0o755 }, alex, CodeSharedLocation},
		"config dir readable": {func(x *tw) {
			x.w.Node("/home/" + alex + "/.config/bothy-host/claude").Sec.Mode = 0o705
			x.w.Node("/home/" + alex).Sec.Mode = 0o711
			x.w.Node("/home/" + alex + "/.config").Sec.Mode = 0o711
			x.w.Node("/home/" + alex + "/.config/bothy-host").Sec.Mode = 0o711
		}, alex, CodeSharedLocation},
		"unresolvable home": {func(x *tw) { x.w.Remove("/home/" + robin) }, robin, CodeHomeUnresolvable},
	} {
		x := newTW(t)
		tc.setup(x)
		r := x.run()
		if !has(codes(r, HD11), tc.code+"@"+tc.user) {
			t.Errorf("%s: %v", name, codes(r, HD11))
			continue
		}
		if r.StartRefused() || r.SessionUserReady(tc.user) {
			t.Errorf("%s: a place of a session user must block only that user", name)
		}
		for _, other := range []string{alex, sam, robin} {
			if other != tc.user && !r.SessionUserReady(other) {
				t.Errorf("%s: %s blocked too", name, other)
			}
		}
	}
	// A place for all: a command program owned by another account that is no session user.
	x := newTW(t)
	x.w.AddUser("pkg", "1700", "/home/pkg", false)
	x.w.File("/opt/tools/tool", "1700", 0o755, "")
	x.cfg.Profiles = append(x.cfg.Profiles, config.Profile{Name: "tool", DisplayName: "Tool", Kind: "command", Program: "/opt/tools/tool"})
	r := x.run()
	if !has(codes(r, HD11), CodeProgramLocationUntrusted) || !r.StartRefused() {
		t.Errorf("place for all: %v", codes(r, HD11))
	}
	for _, e := range r.Report() {
		if e.Check == HD11 && e.SessionUser != "" {
			t.Errorf("place for all names %s", e.SessionUser)
		}
	}
	// Windows drive root: writable for Authenticated Users with (AD).
	x = newTW(t)
	x.env.GOOS = "windows"
	x.env.SystemDrive = "C:"
	x.w.SetGroups(alex, hostos.Group{Name: "Authenticated Users", ID: hostos.SIDAuthenticated})
	x.w.Nodes[`C:\`] = &hostostest.Node{}
	x.w.Nodes[`C:\`].Sec = hostos.FileSec{IsDir: true, OwnerID: hostos.SIDSystem, ACL: []hostos.ACE{{Allow: true, Kind: hostos.PrincipalSID, ID: hostos.SIDAuthenticated, Rights: hostos.WindowsRights(0x4)}}}
	r = x.run()
	if !has(codes(r, HD11), CodeDriveRootWritable) || !strings.Contains(r.Format(), "drive=C:") || !r.StartRefused() {
		t.Errorf("drive root: %v\n%s", codes(r, HD11), r.Format())
	}
	var hd11 []ReportEntry
	for _, e := range r.Report() {
		if e.Check == HD11 {
			hd11 = append(hd11, e)
		}
	}
	if len(hd11) != 1 || hd11[0].Code != CodeDriveRootWritable {
		t.Errorf("precedence drive_root_writable: %+v", hd11)
	}
	// The (OI)(CI)(IO)(M) entry alone gives the root nothing.
	x.w.Nodes[`C:\`].Sec.ACL = []hostos.ACE{{Allow: true, Kind: hostos.PrincipalSID, ID: hostos.SIDAuthenticated, Rights: hostos.WindowsRights(0x1301bf), InheritOnly: true}}
	if r := x.run(); has(codes(r, HD11), CodeDriveRootWritable) {
		t.Error("inherit-only entry counted")
	}
}

func TestPathResolution(t *testing.T) {
	// A chain of links resolves; the configured components and every link are checked.
	x := newTW(t)
	x.w.Dir("/srv/a", "1002", 0o700)
	x.w.Link("/srv/l1", "0", "/srv/l2")
	x.w.Link("/srv/l2", "0", "/srv/a")
	workdir(x, sam, "Linked", "/srv/l1")
	x.w.Node("/srv/a").Sec.OwnerID = "1002"
	r := x.run()
	if len(codes(r, HD11)) != 0 || !reflect.DeepEqual(reported(r, sam), []string{"Linked"}) {
		t.Errorf("chain: %v %+v", codes(r, HD11), r.SessionUsers)
	}
	// A dangling link drops the folder.
	x = newTW(t)
	x.w.Link("/srv/dangling", "1002", "/srv/nowhere")
	workdir(x, sam, "Gone", "/srv/dangling")
	x.w.Remove("/srv/dangling-target")
	x.cfg.ClaudeWorkdirs[0].Dirs[0].Path = "/srv/dangling"
	if r := x.run(); len(reported(r, sam)) != 0 {
		t.Errorf("dangling: %+v", r.SessionUsers)
	}
	// A link of a trusted owner on the way to a place of B pointing into a folder of A fails like
	// a folder of A.
	x = newTW(t)
	x.w.Dir("/home/"+alex+"/proj", "1002", 0o700)
	x.w.Node("/home/" + alex).Sec.Mode = 0o711
	x.w.Link("/srv/proj", "0", "/home/"+alex+"/proj")
	workdir(x, sam, "P", "/srv/proj")
	x.w.Node("/home/" + alex + "/proj").Sec.OwnerID = "1002"
	if r := x.run(); !has(codes(r, HD11), CodeSharedLocation+"@"+sam) {
		t.Errorf("link into A: %v", codes(r, HD11))
	}
	// A configured component is checked as written even when the resolution skips it.
	x = newTW(t)
	x.w.Dir("/srv/open", "1001", 0o755)
	x.w.Dir("/srv/s", "1002", 0o700)
	workdir(x, sam, "S", "/srv/open/../s")
	x.w.Node("/srv/s").Sec.OwnerID = "1002"
	if r := x.run(); !has(codes(r, HD11), CodeSharedLocation+"@"+sam) {
		t.Errorf("component: %v", codes(r, HD11))
	}
}

func TestIdentityCompare(t *testing.T) {
	x := newTW(t)
	x.w.Dir("/srv/proj", "0", 0o755).ID = "dev:7"
	x.w.Dir("/srv/PROJ", "0", 0o755).ID = "dev:7"
	x.w.Link("/home/"+sam+"/p", "1002", "/srv/proj")
	workdir(x, alex, "A", "/srv/PROJ")
	x.cfg.ClaudeWorkdirs = append(x.cfg.ClaudeWorkdirs, config.WorkdirList{SessionUser: sam, Dirs: []config.Workdir{{DisplayName: "B", Path: "/home/" + sam + "/p"}}})
	r := x.run()
	if len(reported(r, alex)) != 0 || len(reported(r, sam)) != 0 {
		t.Errorf("same folder kept: %+v", r.SessionUsers)
	}
}

func TestParseCodesign(t *testing.T) {
	disp := []byte("Executable=/x\nCodeDirectory v=20500 size=1 flags=0x10000(runtime) hashes=1+7 location=embedded\n")
	ent := []byte(`<?xml version="1.0"?><plist><dict><key>com.apple.security.get-task-allow</key><true/></dict></plist>`)
	if i := ParseCodesign(disp, ent); !i.Runtime || !i.GetTaskAllow {
		t.Errorf("%+v", i)
	}
	adhoc := []byte("CodeDirectory v=20400 size=1 flags=0x20002(adhoc,linker-signed) hashes=1+0 location=embedded\n")
	if i := ParseCodesign(adhoc, nil); i.Runtime || i.GetTaskAllow {
		t.Errorf("%+v", i)
	}
	if i := ParseCodesign(disp, []byte(`<key>com.apple.security.get-task-allow</key><false/>`)); i.GetTaskAllow {
		t.Error("false entitlement")
	}
}

// TestUntrustedClaudeBinNeverRuns: a host_service.claude_bin that fails HD10 counts as empty, so
// no helper starts it in a session user (review finding of M2-42).
func TestUntrustedClaudeBinNeverRuns(t *testing.T) {
	for name, setup := range map[string]func(x *tw){
		"writable file":          func(x *tw) { x.w.Node("/opt/claude/bin/claude").Sec.Mode = 0o757 },
		"writable search folder": func(x *tw) { x.w.Node("/usr/bin").Sec.Mode = 0o777 },
	} {
		x := newTW(t)
		setup(x)
		r := x.run()
		if !has(codes(r, HD10), CodeClaudeBinUntrusted) || len(x.starts) != 0 || !has(codes(r, HD6), CodeAuthStatusUnchecked) {
			t.Errorf("%s: HD10 %v, starts %v, HD6 %v", name, codes(r, HD10), x.starts, codes(r, HD6))
		}
	}
}

// TestScanUnreadableEntryIsFinding: an entry whose security cannot be read counts as a finding
// of the entry scan (fail closed, review finding of M2-42).
func TestScanUnreadableEntryIsFinding(t *testing.T) {
	x := newTW(t)
	env := x.helperEnv(sam)
	env.Security = failingSec{Security: env.Security, fail: "/home/" + sam + "/odd"}
	x.w.File("/home/"+sam+"/odd", "1002", 0o600, "")
	if code := ScanEntries(context.Background(), env, Request{EntryScanTimeout: time.Hour}); code != CodeForeignEntry {
		t.Errorf("code %q", code)
	}
}

type failingSec struct {
	hostos.Security
	fail string
}

func (f failingSec) Lstat(p string) (hostos.FileSec, error) {
	if p == f.fail {
		return hostos.FileSec{}, errors.New("access denied")
	}
	return f.Security.Lstat(p)
}

// TestHD6LoginShellConfigDir: a CLAUDE_CONFIG_DIR the login shell of the session user exports is
// a login location too (24 7.4, review finding of M2-42).
func TestHD6LoginShellConfigDir(t *testing.T) {
	x := newTW(t)
	x.confDir[sam] = "/home/" + sam + "/my-claude"
	x.w.Dir("/home/"+sam+"/my-claude", "1002", 0o700)
	x.w.File("/home/"+sam+"/my-claude/.credentials.json", "1002", 0o600, "EXAMPLE")
	if r := x.run(); !has(codes(r, HD6), CodeSubscriptionLogin+"@"+sam) {
		t.Errorf("login file in the exported folder: %v", codes(r, HD6))
	}
	x = newTW(t)
	x.confDir[alex] = "/home/" + alex + "/other"
	x.auth[alex] = map[string]string{"/home/" + alex + "/other": "oauth_token"}
	r := x.run()
	if !has(codes(r, HD6), CodeSubscriptionLogin+"@"+alex) {
		t.Errorf("auth status with the exported folder: %v", codes(r, HD6))
	}
}
