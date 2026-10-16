// SPDX-License-Identifier: Apache-2.0

package bothyapi

import (
	"context"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/OWNER/bothy/services/server/cmd/bothy/importtool/bothyapi/bothytest"
)

// TestLoginPairing (22 10.1, 08 5.3 and 6.4): the code alone on standard output, the
// instruction on standard error, a cli registration without step-up key, and after the approval
// a session file with mode 0600 in a directory with mode 0700.
func TestLoginPairing(t *testing.T) {
	e := newEnv(t)
	e.login(t)
	if got := e.out.String(); got != e.fake.Code()+"\n" {
		t.Errorf("stdout = %q, want the code alone", got)
	}
	if !strings.Contains(e.errw.String(), "Add device") || !strings.Contains(e.errw.String(), "logged in") {
		t.Errorf("stderr = %q", e.errw.String())
	}
	reg, ok := e.fake.Registration()
	if !ok {
		t.Fatal("no registration")
	}
	if reg.Platform != "cli" || len(reg.StepUpKey) != 0 || reg.Alg != "EdDSA" || reg.AppVersion != "1.0.0" {
		t.Errorf("registration = %+v", reg)
	}
	if reg.Name != "bothy import on workstation" {
		t.Errorf("name = %q", reg.Name)
	}
	s, err := SessionFile{Path: e.path}.Load()
	if err != nil {
		t.Fatal(err)
	}
	if s.ServerURL != e.fake.URL || s.UserID != bothytest.UserID || s.DeviceID != reg.DeviceID || s.RefreshToken != e.fake.CurrentRefresh() {
		t.Errorf("session = %s", s)
	}
	if runtime.GOOS != "windows" {
		for p, want := range map[string]os.FileMode{e.path: 0o600, filepath.Dir(e.path): 0o700} {
			info, err := os.Stat(p)
			if err != nil {
				t.Fatal(err)
			}
			if info.Mode().Perm() != want {
				t.Errorf("%s mode = %v, want %v", p, info.Mode().Perm(), want)
			}
		}
	}
	if got := e.fake.Count(http.MethodPost, bothytest.PathCompletePairing); got < 1 {
		t.Errorf("completePairing calls = %d", got)
	}
	for _, w := range e.sleep.all() {
		if w != DefaultLimits().LoginPollInterval {
			t.Errorf("poll wait %v, want import.login_poll_interval", w)
		}
	}
	// A second login for the same server is refused while the session is valid.
	if err := Login(t.Context(), e.loginOptions()); !errors.Is(err, ErrAlreadyLoggedIn) {
		t.Errorf("second Login = %v, want ErrAlreadyLoggedIn", err)
	}
}

// TestLoginExpired: expires_at passes without an approval; nothing is written.
func TestLoginExpired(t *testing.T) {
	e := newEnv(t)
	e.sleep.before = func(int) { e.clk.Advance(bothytest.PairingTTL / 4) }
	err := Login(t.Context(), e.loginOptions())
	if !errors.Is(err, ErrPairingExpired) {
		t.Fatalf("Login = %v, want ErrPairingExpired", err)
	}
	assertNoFile(t, e.path)
}

// TestLoginRejected: completePairing answers 410; nothing is written.
func TestLoginRejected(t *testing.T) {
	e := newEnv(t)
	e.sleep.before = func(n int) {
		if n == 2 {
			e.fake.Reject()
		}
	}
	if err := Login(t.Context(), e.loginOptions()); !errors.Is(err, ErrPairingGone) {
		t.Fatalf("Login = %v, want ErrPairingGone", err)
	}
	assertNoFile(t, e.path)
}

// TestLoginInterrupted: Ctrl-C (a cancelled context) ends the poll; nothing is written, even
// when the owner approves at the same moment.
func TestLoginInterrupted(t *testing.T) {
	e := newEnv(t)
	ctx, cancel := context.WithCancel(t.Context())
	e.sleep.before = func(n int) {
		if n == 2 {
			e.fake.Approve()
			cancel()
		}
	}
	if err := Login(ctx, e.loginOptions()); !errors.Is(err, ErrInterrupted) {
		t.Fatalf("Login = %v, want ErrInterrupted", err)
	}
	assertNoFile(t, e.path)
}

// TestLoginRefusesHTTPForNonLoopback (requirement 15): http only for a loopback server, and the
// server URL is an origin; nothing is sent and nothing is written.
func TestLoginRefusesHTTPForNonLoopback(t *testing.T) {
	e := newEnv(t)
	for _, u := range []string{"http://example.com", "http://192.0.2.1:8080", "ftp://example.com", "https://example.com/api",
		"https://alex@example.com", "https://example.com?x=1", "example.com", ""} {
		o := e.loginOptions()
		o.ServerURL = u
		if err := Login(t.Context(), o); !errors.Is(err, ErrInvalidServerURL) {
			t.Errorf("Login(%q) = %v, want ErrInvalidServerURL", u, err)
		}
	}
	for _, u := range []string{"http://127.0.0.1:1", "http://localhost:1", "http://[::1]:1", "https://example.com", "https://example.com:8443/"} {
		if _, err := checkServerURL(u); err != nil {
			t.Errorf("checkServerURL(%q) = %v", u, err)
		}
	}
	if n := len(e.fake.Requests()); n != 0 {
		t.Errorf("requests = %d, want 0", n)
	}
	assertNoFile(t, e.path)
}

// TestLogoutDeletesFile (22 10.1): logout calls logout, deletes the session file and keeps the
// lock file; with a revoked session the file is still deleted; without a file it is
// ErrNotLoggedIn.
func TestLogoutDeletesFile(t *testing.T) {
	e := newEnv(t)
	e.login(t)
	if err := Logout(t.Context(), e.options()); err != nil {
		t.Fatalf("Logout: %v", err)
	}
	if n := e.fake.Count(http.MethodPost, bothytest.PathLogout); n < 1 {
		t.Errorf("logout calls = %d", n)
	}
	if !e.fake.Revoked() {
		t.Error("the session is not revoked at the server")
	}
	assertNoFile(t, e.path)
	if _, err := os.Stat(e.path + ".lock"); err != nil {
		t.Errorf("lock file: %v", err)
	}
	if err := Logout(t.Context(), e.options()); !errors.Is(err, ErrNotLoggedIn) {
		t.Errorf("Logout without a session = %v, want ErrNotLoggedIn", err)
	}

	// A revoked session (here: revoked by the first logout of a copy) still deletes the file.
	e2 := newEnv(t)
	e2.login(t)
	saved, err := os.ReadFile(e2.path)
	if err != nil {
		t.Fatal(err)
	}
	if err := Logout(t.Context(), e2.options()); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(e2.path, saved, 0o600); err != nil { // #nosec G703 -- a path inside t.TempDir()
		t.Fatal(err)
	}
	if err := Logout(t.Context(), e2.options()); err != nil {
		t.Fatalf("Logout of a revoked session = %v", err)
	}
	assertNoFile(t, e2.path)
}

// assertNoFile fails unless path does not exist.
func assertNoFile(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("%s exists or cannot be checked: %v", path, err)
	}
}
