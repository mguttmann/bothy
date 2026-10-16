// SPDX-License-Identifier: Apache-2.0

package bothyapi

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"

	"github.com/OWNER/bothy/services/server/cmd/bothy/importtool/bothyapi/bothytest"
)

// TestSessionFileModes (requirement 14): a session file or directory that group or others can
// read or write is refused on load and on store. Skipped on Windows, where the file lives in the
// user profile and the mode check is skipped (docs/DECISIONS.md D-M8-10-2).
func TestSessionFileModes(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows has no POSIX modes; the file lives in the user profile (D-M8-10-2)")
	}
	e := newEnv(t)
	e.login(t)
	f := SessionFile{Path: e.path}
	s, err := f.Load()
	if err != nil {
		t.Fatal(err)
	}
	for _, mode := range []os.FileMode{0o640, 0o604, 0o620, 0o602} {
		if err := os.Chmod(e.path, mode); err != nil {
			t.Fatal(err)
		}
		if _, err := f.Load(); !errors.Is(err, ErrInsecureSessionFile) {
			t.Errorf("Load with file mode %v = %v, want ErrInsecureSessionFile", mode, err)
		}
	}
	if err := os.Chmod(e.path, 0o600); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Dir(e.path)
	for _, mode := range []os.FileMode{0o750, 0o705, 0o770} {
		if err := os.Chmod(dir, mode); err != nil {
			t.Fatal(err)
		}
		if _, err := f.Load(); !errors.Is(err, ErrInsecureSessionFile) {
			t.Errorf("Load with directory mode %v = %v, want ErrInsecureSessionFile", mode, err)
		}
		if err := f.Store(s); !errors.Is(err, ErrInsecureSessionFile) {
			t.Errorf("Store with directory mode %v = %v, want ErrInsecureSessionFile", mode, err)
		}
	}
	if err := os.Chmod(dir, 0o700); err != nil { // #nosec G302 -- a directory needs the x bit; 0700 is the required mode
		t.Fatal(err)
	}
	if _, err := f.Load(); err != nil {
		t.Errorf("Load with 0600 in 0700 = %v", err)
	}
}

// TestAtomicStore: a crash between writing the temporary file and the rename leaves the old file
// intact, and no temporary file stays behind after a failed store.
func TestAtomicStore(t *testing.T) {
	e := newEnv(t)
	e.login(t)
	f := SessionFile{Path: e.path}
	old, err := os.ReadFile(e.path)
	if err != nil {
		t.Fatal(err)
	}
	s, err := f.Load()
	if err != nil {
		t.Fatal(err)
	}
	s.RefreshToken = "rt-new"
	f.rename = func(string, string) error { return errors.New("simulated crash before rename") }
	if err := f.Store(s); err == nil {
		t.Fatal("Store with a failing rename = nil")
	}
	got, err := os.ReadFile(e.path)
	if err != nil || string(got) != string(old) {
		t.Fatalf("the old file changed: %v", err)
	}
	entries, err := os.ReadDir(filepath.Dir(e.path))
	if err != nil {
		t.Fatal(err)
	}
	for _, en := range entries {
		if strings.Contains(en.Name(), ".tmp") {
			t.Errorf("temporary file %s left behind", en.Name())
		}
	}
	f.rename = nil
	if err := f.Store(s); err != nil {
		t.Fatal(err)
	}
	if s2, err := f.Load(); err != nil || s2.RefreshToken != "rt-new" {
		t.Errorf("Load after Store = %v, %v", s2, err)
	}
}

// TestConcurrentRefreshRotatesOnce (08 7.4, note 2): two clients on one session file refresh an
// expired access token in parallel; the lock and the re-read make exactly one refreshToken
// reach the fake, and the session is not revoked.
func TestConcurrentRefreshRotatesOnce(t *testing.T) {
	e := newEnv(t)
	e.login(t)
	e.clk.Advance(bothytest.AccessTTL + 1)
	var wg sync.WaitGroup
	errs := make([]error, 2)
	for i := range errs {
		c, err := New(t.Context(), e.options())
		if err != nil {
			t.Fatal(err)
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, errs[i] = c.Me(t.Context())
		}()
	}
	wg.Wait()
	for i, err := range errs {
		if err != nil {
			t.Errorf("client %d: %v", i, err)
		}
	}
	if n := e.fake.Rotations(); n != 1 {
		t.Errorf("rotations = %d, want 1", n)
	}
	if e.fake.Revoked() {
		t.Error("the session was revoked")
	}
}

// TestSessionPrintsNoSecret: every print form of a session shows only server_url and device_id.
func TestSessionPrintsNoSecret(t *testing.T) {
	s := &Session{Version: 1, ServerURL: "https://chat.example.com", DeviceID: "0192f4a0-7b1c-7848-8501-1716705c7dbe",
		AccessToken: "at-secret", RefreshToken: "rt-secret", DPoPPrivateJWK: []byte(`{"d":"key-secret"}`)}
	for _, form := range []string{"%v", "%+v", "%#v", "%s"} {
		for _, v := range []any{s, *s} {
			out := fmt.Sprintf(form, v)
			if strings.Contains(out, "secret") || !strings.Contains(out, s.DeviceID) {
				t.Errorf("%s = %q", form, out)
			}
		}
	}
}
