// SPDX-License-Identifier: Apache-2.0

package doctor

import (
	"slices"
	"testing"

	"github.com/OWNER/bothy/services/host/internal/hostos"
	"github.com/OWNER/bothy/services/host/internal/state"
)

// The owner account mode (ADR 0073, D-OA-9): the host service and the one session user are the
// existing administrator account "owner" of the instance owner, installed under its home without
// root. "guest" is another person account of the host.
const (
	owner      = "owner"
	ownerHome  = "/home/owner"
	ownerState = "/home/owner/.local/state/bothy-host"
	ownerBin   = "/home/owner/.local/lib/bothy-host/bothy-host"
	ownerConf  = "/home/owner/.config/bothy-host/config.yaml"
	ownerUnit  = "/home/owner/.config/systemd/user/bothy-host.service"
	guest      = "guest"
)

// newOwnerTW returns the world of a host in the owner account mode: owner (uid 1000) is in the
// group sudo with passwordless sudo, his home is readable by his group, guest's home is listable,
// and every program lies in a folder of owner.
func newOwnerTW(t *testing.T) *tw {
	t.Helper()
	x := newTW(t)
	w := x.w
	w.AddUser(owner, "1000", ownerHome, true, hostos.Group{Name: "sudo", ID: "27"})
	w.AddUser(guest, "1005", "/home/"+guest, true)
	w.SudoOK[owner] = true
	w.Dir("/home", "0", 0o755)
	w.Dir("/home/"+guest, "1005", 0o755)
	w.Dir(ownerHome, "1000", 0o750)
	for _, d := range []string{"/home/owner/.local", "/home/owner/.local/state", "/home/owner/.local/lib", "/home/owner/.local/lib/bothy-host",
		"/home/owner/.config", "/home/owner/.config/bothy-host", "/home/owner/.config/systemd", "/home/owner/.config/systemd/user",
		"/home/owner/.config/bothy-host/claude", "/home/owner/.local/bin"} {
		w.Dir(d, "1000", 0o755)
	}
	w.Dir("/home/owner/.config/bothy-host/claude", "1000", 0o700)
	w.Dir(ownerState, "1000", 0o700)
	w.Dir(ownerState+"/run", "1000", 0o700)
	w.File(ownerState+"/keys", "1000", 0o600, "EXAMPLE-KEYS")
	w.File(ownerBin, "1000", 0o755, "\x7fELF")
	w.File(ownerConf, "1000", 0o600, "host_service: {}")
	w.File(ownerUnit, "1000", 0o644, "")
	x.cfg.Path = ownerConf
	x.cfg.StateDir = ownerState
	x.cfg.SessionUsers = []string{owner}
	x.cfg.OwnerAccount = owner
	x.cfg.ClaudeWorkdirs = nil
	x.claudeBin("/home/owner/.local/bin/claude", "\x7fELF")
	w.File("/home/owner/.local/bin/claude", "1000", 0o755, "\x7fELF")
	x.env.ServiceUser = owner
	x.env.OwnerAccount = owner
	x.env.CallerID = "1000"
	x.env.StateDir, x.env.KeyFile = ownerState, ownerState+"/keys"
	x.env.Resolver, x.env.Files, x.env.Probe = w.As(owner), w.As(owner), w.As(owner).Probe()
	x.env.HelperBinary = ownerBin
	x.env.ServiceDefinitions = []string{ownerUnit}
	x.env.EndpointDirs = []string{ownerState + "/run"}
	x.env.PeerCheckProven = false
	x.w.Ptrace = 0
	return x
}

func failed(r *Result) []string {
	var out []string
	for _, c := range r.Checks {
		for _, e := range c.Entries {
			if e.Status == Fail {
				out = append(out, c.Check+":"+e.Code)
			}
		}
	}
	return out
}

func TestOwnerAccountModePasses(t *testing.T) {
	x := newOwnerTW(t)
	x.auth[owner] = map[string]string{"": "claude.ai"} // the owner's own subscription login
	for _, decl := range []string{state.DeclNone, state.DeclOrganization, state.DeclPersonal} {
		x.env.Declaration = decl
		r := x.run()
		if f := failed(r); len(f) > 0 || r.StartRefused() || !r.SessionUserReady(owner) {
			t.Fatalf("%s: owner account mode refused: %v", decl, f)
		}
		if !has(codes(r, HD1), CodeOwnerAccountMode) {
			t.Errorf("%s: HD1 does not warn about the mode: %v", decl, codes(r, HD1))
		}
		for _, want := range []struct{ check, code string }{
			{HD2, CodeForbiddenGroup}, {HD3, CodePasswordlessSudo}, {HD9, CodeForeignHomeListable},
			{HD10, CodePeerCheckUnavailable}, {HD10, CodePtraceScopeLow},
		} {
			if !has(codes(r, want.check), want.code) {
				t.Errorf("%s: %s lacks the warning %s: %v", decl, want.check, want.code, codes(r, want.check))
			}
		}
		if len(r.SessionUsers) != 1 || r.SessionUsers[0].OSUser != owner || !r.SessionUsers[0].OwnerAccount {
			t.Errorf("%s: the report does not mark the owner account: %+v", decl, r.SessionUsers)
		}
		if decl != state.DeclPersonal && !has(codes(r, HD6), CodeSubscriptionLogin+"@"+owner) {
			t.Errorf("%s: HD6 does not warn about the owner's own login: %v", decl, codes(r, HD6))
		}
	}
}

// TestOwnerAccountModeHD6OnlyForTheOwner: the owner's own subscription does not block, the same
// login in a separate session user of another person still refuses the start (D-OA-9).
func TestOwnerAccountModeHD6OnlyForTheOwner(t *testing.T) {
	x := newOwnerTW(t)
	x.env.Declaration = state.DeclOrganization
	x.auth[owner] = map[string]string{"": "oauth_token"}
	if r := x.run(); r.Check(HD6).Failed() {
		t.Fatalf("owner account: %v", codes(r, HD6))
	}
	y := newTW(t) // separate session users, the default
	y.env.Declaration = state.DeclOrganization
	y.auth[sam] = map[string]string{"": "claude.ai"}
	r := y.run()
	for _, u := range r.SessionUsers {
		if u.OwnerAccount {
			t.Errorf("a separate session user is reported as the owner account: %+v", u)
		}
	}
	if !r.Check(HD6).Failed() || !has(codes(r, HD6), CodeSubscriptionLogin+"@"+sam) || !r.StartRefused() {
		t.Fatalf("another person's login passes: %v", codes(r, HD6))
	}
}

func TestOwnerAccountModeStillRefuses(t *testing.T) {
	for _, tc := range []struct {
		name  string
		setup func(x *tw)
		check string
		code  string
	}{
		{"service runs as another account", func(x *tw) { x.env.CallerID = "1005" }, HD1, CodeOwnerAccountMismatch},
		{"second session user", func(x *tw) { x.cfg.SessionUsers = []string{owner, alex} }, HD1, CodeOwnerAccountMismatch},
		{"root as owner account", func(x *tw) {
			x.cfg.OwnerAccount, x.env.OwnerAccount, x.env.ServiceUser, x.cfg.SessionUsers, x.env.CallerID = "root", "root", "root", []string{"root"}, "0"
		}, HD1, CodePrivilegedUser},
		{"state directory readable by guest", func(x *tw) { x.w.Dir(ownerState, "1000", 0o755) }, HD4, CodeStateDirMode},
		{"key file of guest", func(x *tw) { x.w.File(ownerState+"/keys", "1005", 0o600, "K") }, HD4, CodeKeyFileUnprotected},
		{"binary writable by everyone", func(x *tw) { x.w.File(ownerBin, "1000", 0o757, "\x7fELF") }, HD10, CodeHelperUntrusted},
		{"binary of guest", func(x *tw) { x.w.File(ownerBin, "1005", 0o755, "\x7fELF") }, HD10, CodeHelperUntrusted},
		{"folder of the binary writable by guest", func(x *tw) {
			x.w.Dir("/home/owner/.local/lib/bothy-host", "1000", 0o755, hostos.ACE{Allow: true, Kind: hostos.PrincipalUser, ID: "1005", Rights: hostos.RightWriteData})
		}, HD10, CodeHelperUntrusted},
		{"claude of guest", func(x *tw) { x.w.File("/home/owner/.local/bin/claude", "1005", 0o755, "\x7fELF") }, HD10, CodeClaudeBinUntrusted},
		{"peer is not the helper", func(x *tw) { x.setPeer(owner, ErrPeerMismatch) }, HD10, CodePeerMismatch},
	} {
		x := newOwnerTW(t)
		tc.setup(x)
		r := x.run()
		if !slices.Contains(failed(r), tc.check+":"+tc.code) {
			t.Errorf("%s: want %s %s failed, got %v", tc.name, tc.check, tc.code, failed(r))
		}
	}
}

// TestSeparateModeUnchanged: without host_service.owner_account an administrator account as session
// user and the service user as session user still refuse the start (defaults stay, D-OA-1).
func TestSeparateModeUnchanged(t *testing.T) {
	x := newTW(t)
	x.w.SetGroups(alex, hostos.Group{Name: alex, ID: "1001"}, hostos.Group{Name: "sudo", ID: "27"})
	x.w.SudoOK[alex] = true
	r := x.run()
	if !r.Check(HD2).Failed() || !r.Check(HD3).Failed() || !r.StartRefused() || has(codes(r, HD1), CodeOwnerAccountMode) {
		t.Fatalf("separate mode relaxed: HD1 %v HD2 %v HD3 %v", codes(r, HD1), codes(r, HD2), codes(r, HD3))
	}
	y := newTW(t)
	y.cfg.SessionUsers = []string{alex, svc}
	if r := y.run(); !has(codes(r, HD1), CodeSessionUserIsService) || !r.StartRefused() {
		t.Fatalf("service user as session user passes: %v", codes(r, HD1))
	}
}
