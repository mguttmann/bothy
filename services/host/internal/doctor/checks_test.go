// SPDX-License-Identifier: Apache-2.0

package doctor

import (
	"context"
	"errors"
	"net"
	"strings"
	"testing"

	"github.com/OWNER/bothy/services/host/internal/hostos"
	"github.com/OWNER/bothy/services/host/internal/state"
)

func TestHD1(t *testing.T) {
	for _, tc := range []struct {
		name  string
		setup func(x *tw)
		want  string
	}{
		{"empty list", func(x *tw) { x.cfg.SessionUsers = nil }, CodeSessionUsersEmpty},
		{"root", func(x *tw) { x.cfg.SessionUsers = []string{"root"} }, CodePrivilegedUser},
		{"Administrator", func(x *tw) { x.cfg.SessionUsers = []string{"Administrator"} }, CodePrivilegedUser},
		{"SYSTEM", func(x *tw) { x.cfg.SessionUsers = []string{"SYSTEM"} }, CodePrivilegedUser},
		{"uid 0 alias", func(x *tw) { x.w.AddUser("toor", "0", "/root", false); x.cfg.SessionUsers = []string{"toor"} }, CodePrivilegedUser},
		{"service user root", func(x *tw) { x.env.ServiceUser = "root" }, CodePrivilegedUser},
		{"duplicate without case", func(x *tw) {
			x.w.AddUser("BOTHY-S-ALEX", "1009", "/home/x", true)
			x.cfg.SessionUsers = []string{alex, "BOTHY-S-ALEX"}
		}, CodeSessionUserDuplicate},
		{"service user as session user", func(x *tw) { x.cfg.SessionUsers = []string{alex, svc} }, CodeSessionUserIsService},
		{"unknown user", func(x *tw) { x.cfg.SessionUsers = []string{alex, "bothy-s-ghost"} }, CodeUserMissing},
		{"unknown service user", func(x *tw) { x.env.ServiceUser = "bothy-ghost" }, CodeUserMissing},
		{"invalid name", func(x *tw) {
			x.w.AddUser("-bad name", "1010", "/home/b", true)
			x.cfg.SessionUsers = []string{"-bad name"}
		}, CodeSessionUserNameInvalid},
	} {
		x := newTW(t)
		tc.setup(x)
		r := x.run()
		if !has(codes(r, HD1), tc.want) || !r.StartRefused() {
			t.Errorf("%s: %v", tc.name, codes(r, HD1))
		}
		for _, e := range r.Check(HD1).Entries {
			if e.SessionUser != "" {
				t.Errorf("%s: HD1 names %s", tc.name, e.SessionUser)
			}
		}
	}
}

func TestHD2(t *testing.T) {
	for _, g := range ForbiddenGroups {
		x := newTW(t)
		x.w.SetGroups(sam, hostos.Group{Name: sam, ID: "1002"}, hostos.Group{Name: g, ID: "4242"})
		if r := x.run(); !has(codes(r, HD2), CodeForbiddenGroup) || !r.StartRefused() {
			t.Errorf("%s: %v", g, codes(r, HD2))
		}
	}
	x := newTW(t)
	x.w.SetGroups(svc, hostos.Group{Name: "lab-admins", ID: "4300"})
	x.cfg.ExtraForbiddenGroups = []string{"lab-admins"}
	if r := x.run(); !has(codes(r, HD2), CodeForbiddenGroup) {
		t.Errorf("extra group: %v", codes(r, HD2))
	}
	// The fixed list cannot be shortened: an extra list without docker still forbids docker.
	x = newTW(t)
	x.cfg.ExtraForbiddenGroups = []string{}
	x.w.SetGroups(alex, hostos.Group{Name: "docker", ID: "998"})
	if r := x.run(); !has(codes(r, HD2), CodeForbiddenGroup) {
		t.Error("fixed list shortened")
	}
	// Windows compares without case.
	x = newTW(t)
	x.env.GOOS = "windows"
	x.env.PeerCheckProven = true
	x.w.SetGroups(alex, hostos.Group{Name: "ADMINISTRATORS", ID: hostos.SIDAdministrators})
	if r := x.run(); !has(codes(r, HD2), CodeForbiddenGroup) {
		t.Error("windows case")
	}
	x = newTW(t)
	x.w.SetGroups(alex) // groups cannot be read
	x.w.Remove("/x")
	delete(x.w.GroupMap, alex)
	if r := x.run(); !has(codes(r, HD2), CodeForbiddenGroup) {
		t.Error("unreadable groups pass")
	}
	for name, sec := range map[string]func(x *tw){
		"by mode": func(x *tw) { x.w.File("/var/run/docker.sock", "0", 0o666, "") },
		"by ACL": func(x *tw) {
			x.w.File("/var/run/docker.sock", "0", 0o660, "", hostos.ACE{Allow: true, Kind: hostos.PrincipalUser, ID: "1002", Rights: hostos.RightWriteData})
		},
		"by group": func(x *tw) { x.w.File("/var/run/docker.sock", "0", 0o660, "").Sec.GroupID = "1001" },
		"by link": func(x *tw) {
			x.w.File("/run/docker.sock", "0", 0o666, "")
			x.w.Link("/var/run/docker.sock", "0", "/run/docker.sock")
		},
	} {
		x := newTW(t)
		sec(x)
		if r := x.run(); !has(codes(r, HD2), CodeDockerSocketWritable) {
			t.Errorf("docker socket %s: %v", name, codes(r, HD2))
		}
	}
	x = newTW(t)
	x.w.File("/var/run/docker.sock", "0", 0o660, "")
	if r := x.run(); len(codes(r, HD2)) != 0 {
		t.Errorf("closed socket: %v", codes(r, HD2))
	}
}

func TestHD3(t *testing.T) {
	x := newTW(t)
	x.w.SudoOK[svc] = true
	if r := x.run(); !has(codes(r, HD3), CodePasswordlessSudo) || !r.StartRefused() {
		t.Errorf("service user: %v", codes(r, HD3))
	}
	x = newTW(t)
	x.w.SudoOK[sam] = true
	if r := x.run(); !has(codes(r, HD3), CodePasswordlessSudo) || codes(r, HD3)[0] != CodePasswordlessSudo {
		t.Errorf("session user (unnamed): %v", codes(r, HD3))
	}
	x = newTW(t)
	x.env.GOOS = "windows"
	x.w.SudoOK[svc], x.w.SudoOK[sam] = true, true
	if r := x.run(); len(codes(r, HD3)) != 0 {
		t.Errorf("windows has no probe: %v", codes(r, HD3))
	}
	x = newTW(t)
	x.env.Probe = errProbe{}
	if r := x.run(); !has(codes(r, HD3), CodePasswordlessSudo) {
		t.Error("probe failure passes")
	}
}

type errProbe struct{}

func (errProbe) SudoWithoutPassword(context.Context) (bool, error) { return false, errors.New("x") }
func (errProbe) PtraceScope() (int, error)                         { return 0, errors.New("x") }

func TestHD4(t *testing.T) {
	for name, setup := range map[string]func(x *tw){
		"owner":   func(x *tw) { x.w.Node("/var/lib/bothy-host").Sec.OwnerID = "0" },
		"missing": func(x *tw) { x.w.Remove("/var/lib/bothy-host") },
		"mode":    func(x *tw) { x.w.Node("/var/lib/bothy-host").Sec.Mode = 0o750 },
		"dir ACL": func(x *tw) {
			x.w.Node("/var/lib/bothy-host").Sec.ACL = []hostos.ACE{{Allow: true, Kind: hostos.PrincipalUser, ID: "5"}}
		},
		"key mode":  func(x *tw) { x.w.Node("/var/lib/bothy-host/keys").Sec.Mode = 0o640 },
		"key owner": func(x *tw) { x.w.Node("/var/lib/bothy-host/keys").Sec.OwnerID = "0" },
	} {
		x := newTW(t)
		setup(x)
		if r := x.run(); len(codes(r, HD4)) == 0 || !r.StartRefused() {
			t.Errorf("%s: %v", name, codes(r, HD4))
		}
	}
	x := newTW(t)
	x.w.Remove("/var/lib/bothy-host/keys")
	if r := x.run(); len(codes(r, HD4)) != 0 {
		t.Errorf("unpaired: %v", codes(r, HD4))
	}
	// Windows: the DPAPI header and the three principals.
	x = newTW(t)
	x.env.GOOS = "windows"
	x.w.Node("/var/lib/bothy-host/keys").Content = append([]byte(nil), dpapiHeader...)
	x.w.Node("/var/lib/bothy-host").Sec.ACL = []hostos.ACE{{Allow: true, Kind: hostos.PrincipalSID, ID: hostos.SIDSystem}, {Allow: true, Kind: hostos.PrincipalSID, ID: "999"}}
	if r := x.run(); len(codes(r, HD4)) != 0 {
		t.Errorf("windows ok: %v", codes(r, HD4))
	}
	x.w.Node("/var/lib/bothy-host/keys").Content = []byte("plain")
	if r := x.run(); !has(codes(r, HD4), CodeKeyFileUnprotected) {
		t.Errorf("windows plain key: %v", codes(r, HD4))
	}
	x.w.Node("/var/lib/bothy-host").Sec.ACL = append(x.w.Node("/var/lib/bothy-host").Sec.ACL, hostos.ACE{Allow: true, Kind: hostos.PrincipalSID, ID: hostos.SIDUsers})
	if r := x.run(); !has(codes(r, HD4), CodeStateDirMode) {
		t.Errorf("windows DACL: %v", codes(r, HD4))
	}
}

func TestHD5(t *testing.T) {
	for _, tc := range []struct {
		name, url string
		lookup    string
		want      bool
	}{
		{"loopback", "https://127.0.0.1", "", true},
		{"own interface", "https://chat.example.com", "192.0.2.10", true},
		{"foreign address", "https://chat.example.com", "192.0.2.1", false},
		{"not paired", "", "", false},
		{"bad URL", "https://%zz", "", false},
	} {
		x := newTW(t)
		x.env.ServerURL = tc.url
		if tc.lookup != "" {
			ip := tc.lookup
			x.env.LookupIP = func(context.Context, string) ([]net.IP, error) { return []net.IP{net.ParseIP(ip)}, nil }
		}
		r := x.run()
		if r.IsServerHost != tc.want || has(codes(r, HD5), CodeServerHostDetected) != tc.want {
			t.Errorf("%s: %v %v", tc.name, r.IsServerHost, codes(r, HD5))
		}
		if r.StartRefused() {
			t.Errorf("%s: HD5 refused the start", tc.name)
		}
		for _, e := range r.Report() {
			if e.Check == HD5 && !e.OK {
				t.Errorf("%s: HD5 not ok", tc.name)
			}
		}
	}
}

func TestHD6(t *testing.T) {
	type tcase struct {
		decl, method string
		want         string // "" ok, "W:code" warning, "F:code" failure
	}
	var cases []tcase
	for _, m := range []string{"none", "claude.ai", "oauth_token", "api_key", "api_key_helper", "third_party", "unknown_method", "garbage", "hang", "exit2"} {
		finding := m == "claude.ai" || m == "oauth_token"
		unknown := m == "unknown_method" || m == "garbage" || m == "hang" || m == "exit2"
		for _, d := range []string{state.DeclNone, state.DeclOrganization, state.DeclPersonal, state.DeclUnknown} {
			want := ""
			switch {
			case !finding && !unknown, d == state.DeclPersonal:
			case d == state.DeclUnknown:
				want = "W:" + CodeDeclarationUnknown
			case finding:
				want = "F:" + CodeSubscriptionLogin
			default:
				want = "F:" + CodeAuthStatusUnknown
			}
			cases = append(cases, tcase{d, m, want})
		}
	}
	for _, tc := range cases {
		for _, where := range []string{"", "org"} {
			x := newTW(t)
			x.env.Declaration = tc.decl
			x.auth[sam] = map[string]string{where: tc.method}
			r := x.run()
			got := ""
			for _, e := range r.Check(HD6).Entries {
				if e.SessionUser != sam {
					t.Errorf("%+v: entry for %q", tc, e.SessionUser)
				}
				got = map[Status]string{Warn: "W:", Fail: "F:"}[e.Status] + e.Code
			}
			if got != tc.want {
				t.Errorf("%s %s in %q: got %q want %q", tc.decl, tc.method, where, got, tc.want)
			}
			if r.StartRefused() != (tc.want != "" && tc.want[0] == 'F') {
				t.Errorf("%+v: refused %v", tc, r.StartRefused())
			}
		}
	}
	// Login files and the variable, one entry per affected session user.
	x := newTW(t)
	x.w.File("/home/"+alex+"/.claude/.credentials.json", "1001", 0o600, "EXAMPLE-TOKEN")
	x.w.File("/home/"+sam+"/.config/bothy-host/claude/.credentials.json", "1002", 0o600, "EXAMPLE-TOKEN")
	x.token[robin] = true
	r := x.run()
	for _, u := range []string{alex, sam, robin} {
		if !has(codes(r, HD6), CodeSubscriptionLogin+"@"+u) {
			t.Errorf("%s not named: %v", u, codes(r, HD6))
		}
	}
	// The coding agent only through host_service.claude_bin with the fixed environment.
	if len(x.cmds) != 6 {
		t.Errorf("starts %d, want two per session user", len(x.cmds))
	}
	for _, c := range x.cmds {
		env := strings.Join(c.Env, " ")
		if c.Path != "/opt/claude/bin/claude" || strings.Join(c.Args, " ") != "auth status" || c.Timeout != x.cfg.AuthStatusTimeout ||
			!strings.Contains(env, "PATH=/opt/claude/bin:/usr/bin:/bin") || !strings.HasPrefix(env, "HOME=/home/") || len(c.Env) > 3 {
			t.Errorf("start %+v", c)
		}
	}
	// Empty, unresolvable and script without interpreter: warning auth_status_unchecked.
	for name, setup := range map[string]func(x *tw){
		"empty":        func(x *tw) { x.cfg.ClaudeBin = "" },
		"unresolvable": func(x *tw) { x.cfg.ClaudeBin = "/opt/missing/claude" },
		"no interpreter": func(x *tw) {
			x.claudeBin("/opt/claude/bin/claude", "#!/usr/bin/env node\n")
			x.w.File("/usr/bin/env", "0", 0o755, "")
		},
	} {
		x := newTW(t)
		setup(x)
		x.auth[sam] = map[string]string{"": "claude.ai"}
		r := x.run()
		if !has(codes(r, HD6), CodeAuthStatusUnchecked) || r.StartRefused() {
			t.Errorf("%s: %v", name, codes(r, HD6))
		}
		for _, s := range x.starts {
			t.Errorf("%s: claude started: %s", name, s)
		}
	}
}

func TestHD2WindowsAdministratorsBySID(t *testing.T) {
	x := newTW(t)
	x.env.GOOS = "windows"
	x.w.SetGroups(alex, hostos.Group{Name: "Administratoren", ID: hostos.SIDAdministrators})
	if r := x.run(); !has(codes(r, HD2), CodeForbiddenGroup) {
		t.Errorf("localized Administrators: %v", codes(r, HD2))
	}
}

func TestHD4WindowsOwner(t *testing.T) {
	x := newTW(t)
	x.env.GOOS = "windows"
	x.w.Node("/var/lib/bothy-host").Sec.OwnerID = hostos.SIDAdministrators
	x.w.Node("/var/lib/bothy-host/keys").Content = append([]byte(nil), dpapiHeader...)
	if r := x.run(); has(codes(r, HD4), CodeStateDirOwner) {
		t.Errorf("Administrators as owner: %v", codes(r, HD4))
	}
	x.w.Node("/var/lib/bothy-host").Sec.OwnerID = "S-1-5-21-1-2-3-1001"
	if r := x.run(); !has(codes(r, HD4), CodeStateDirOwner) {
		t.Errorf("a person as owner: %v", codes(r, HD4))
	}
}
