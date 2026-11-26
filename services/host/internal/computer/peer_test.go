// SPDX-License-Identifier: Apache-2.0

package computer

import (
	"context"
	"errors"
	"testing"

	"github.com/OWNER/bothy/services/host/internal/confirm"
	"github.com/OWNER/bothy/services/host/internal/hostos"
)

const (
	uidService = "400"
	uidAlex    = "501" // the console user
	uidSam     = "502" // a session user
	uidRobin   = "503" // another user
	helperPath = "/Applications/Bothy Computer Helper.app/Contents/MacOS/bothy-computer-helper"
	copyPath   = "/tmp/bothy-computer-helper"
)

// procTable is a fake process table: pid to executable path.
type procTable map[int]string

func (p procTable) exe(pid int) (string, error) {
	if s, ok := p[pid]; ok {
		return s, nil
	}
	return "", errors.New("no such process")
}

// rule returns the peer rule of macOS with the fake process table and a signature check that
// accepts exactly the signed paths.
func rule(procs procTable, requirement string, signed map[string]bool, trusted map[string]bool) PeerRule {
	return PeerRule{
		GOOS: hostos.Darwin,
		Console: confirm.Rule{ServiceUID: uidService, Excluded: func() []string { return []string{uidSam} },
			ConsoleOwner: func(context.Context) (string, error) { return uidAlex, nil }},
		HelperPath:  helperPath,
		ExePath:     procs.exe,
		Trusted:     func(p string) bool { return trusted[p] },
		Requirement: requirement,
		VerifySignature: func(_ context.Context, p, req string) error {
			if signed[p] && req != "" {
				return nil
			}
			return errors.New("unsigned")
		},
	}
}

// TestPeerRule runs the peer rule of 27 9.4 against a fake process table (CU-T23).
func TestPeerRule(t *testing.T) {
	procs := procTable{10: helperPath, 11: copyPath, 12: helperPath, 13: helperPath, 14: helperPath}
	trusted := map[string]bool{helperPath: true, copyPath: true}
	req := `identifier "com.example.bothy.computer-helper" and anchor apple generic`
	cases := []struct {
		name    string
		peer    confirm.Peer
		r       PeerRule
		allowed bool
	}{
		{"console user, installed signed helper", confirm.Peer{UID: uidAlex, PID: 10}, rule(procs, req, map[string]bool{helperPath: true}, trusted), true},
		{"development build without requirement", confirm.Peer{UID: uidAlex, PID: 10}, rule(procs, "", nil, trusted), true},
		{"service user", confirm.Peer{UID: uidService, PID: 12}, rule(procs, "", nil, trusted), false},
		{"session user", confirm.Peer{UID: uidSam, PID: 13}, rule(procs, "", nil, trusted), false},
		{"another user", confirm.Peer{UID: uidRobin, PID: 14}, rule(procs, "", nil, trusted), false},
		{"copy outside the installed path", confirm.Peer{UID: uidAlex, PID: 11}, rule(procs, "", nil, trusted), false},
		{"unsigned helper with a requirement", confirm.Peer{UID: uidAlex, PID: 10}, rule(procs, req, nil, trusted), false},
		{"untrusted folder chain", confirm.Peer{UID: uidAlex, PID: 10}, rule(procs, "", nil, map[string]bool{}), false},
		{"unknown process", confirm.Peer{UID: uidAlex, PID: 99}, rule(procs, "", nil, trusted), false},
		{"no pid", confirm.Peer{UID: uidAlex}, rule(procs, "", nil, trusted), false},
	}
	for _, c := range cases {
		err := c.r.Check(context.Background(), c.peer)
		if (err == nil) != c.allowed {
			t.Errorf("%s: %v, want allowed %v", c.name, err, c.allowed)
		}
		if err != nil && !errors.Is(err, ErrPeerRefused) {
			t.Errorf("%s: %v", c.name, err)
		}
	}
}

// TestPeerRuleLinux: Linux admits exactly the user of host_service.computer_test_peer; empty
// admits nobody (27 9.4).
func TestPeerRuleLinux(t *testing.T) {
	r := PeerRule{GOOS: hostos.Linux, Console: confirm.Rule{ServiceUID: uidService}}
	if r.Check(context.Background(), confirm.Peer{UID: uidAlex, PID: 5}) == nil {
		t.Error("empty test peer admits a user")
	}
	r.TestPeerUID = uidAlex
	if err := r.Check(context.Background(), confirm.Peer{UID: uidAlex, PID: 5}); err != nil {
		t.Errorf("test peer refused: %v", err)
	}
	if r.Check(context.Background(), confirm.Peer{UID: uidRobin, PID: 5}) == nil {
		t.Error("another user admitted")
	}
	r.TestPeerUID = uidService
	if r.Check(context.Background(), confirm.Peer{UID: uidService, PID: 5}) == nil {
		t.Error("the service user admitted as test peer")
	}
}

// fakeRes and fakeSec are a resolver and security reader of a fixed file tree.
type fakeRes struct{ r hostos.Resolved }

func (f fakeRes) Resolve(string) (hostos.Resolved, error) { return f.r, nil }
func (f fakeRes) Identity(string) (string, error)         { return "", nil }

type fakeSec map[string]hostos.FileSec

func (f fakeSec) Lstat(p string) (hostos.FileSec, error) {
	if s, ok := f[p]; ok {
		return s, nil
	}
	return hostos.FileSec{}, errors.New("missing")
}

// TestTrustedPath applies the rule of HD10 to the helper path.
func TestTrustedPath(t *testing.T) {
	res := fakeRes{hostos.Resolved{Final: "/A/h", Chain: []string{"/", "/A"}, Components: []string{"/A", "/A/h"}}}
	root := hostos.FileSec{OwnerID: "0", Mode: 0o755}
	ok := fakeSec{"/": root, "/A": root, "/A/h": root}
	if !TrustedPath(hostos.Darwin, res, ok, nil)("/A/h") {
		t.Error("root chain not trusted")
	}
	userOwned := fakeSec{"/": root, "/A": {OwnerID: uidAlex, Mode: 0o755}, "/A/h": root}
	if TrustedPath(hostos.Darwin, res, userOwned, nil)("/A/h") {
		t.Error("a folder of a user is trusted")
	}
	writable := fakeSec{"/": root, "/A": {OwnerID: "0", Mode: 0o777}, "/A/h": root}
	everyone := func() []hostos.Subject { return []hostos.Subject{{Name: "everyone", ID: "everyone"}} }
	if TrustedPath(hostos.Darwin, res, writable, everyone)("/A/h") {
		t.Error("a folder writable by everyone is trusted")
	}
	if TrustedPath(hostos.Darwin, nil, ok, nil)("/A/h") {
		t.Error("no resolver trusts")
	}
}
