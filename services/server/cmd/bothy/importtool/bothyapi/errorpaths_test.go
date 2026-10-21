// SPDX-License-Identifier: Apache-2.0

package bothyapi

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// TestLimitsValidateNamesEachInvalidField checks that every field of Limits is checked on its own.
func TestLimitsValidateNamesEachInvalidField(t *testing.T) {
	cases := map[string]func(*Limits){
		"import.login_poll_interval":     func(l *Limits) { l.LoginPollInterval = 0 },
		"import.discord.max_429_retries": func(l *Limits) { l.Max429Retries = -1 },
		"import.discord.max_rate_wait":   func(l *Limits) { l.MaxRateWait = 0 },
		"import.http_timeout":            func(l *Limits) { l.HTTPTimeout = 0 },
		"egress.max_response_bytes":      func(l *Limits) { l.MaxResponseBytes = 0 },
	}
	for key, mutate := range cases {
		t.Run(key, func(t *testing.T) {
			l := DefaultLimits()
			mutate(&l)
			err := l.validate()
			if err == nil || !strings.Contains(err.Error(), key) {
				t.Fatalf("validate() = %v, want an error naming %s", err, key)
			}
		})
	}
	if err := DefaultLimits().validate(); err != nil {
		t.Fatalf("DefaultLimits().validate() = %v", err)
	}
}

// TestNewRejectsInvalidLimits checks that New refuses a Limits value before it reads the session.
func TestNewRejectsInvalidLimits(t *testing.T) {
	l := DefaultLimits()
	l.HTTPTimeout = 0
	_, err := New(context.Background(), Options{SessionFile: filepath.Join(t.TempDir(), "s.json"), Limits: l})
	if err == nil {
		t.Fatal("New with an invalid limit returned no error")
	}
}

// TestSleepContextEndsWithTheContext checks the production sleeper both ways.
func TestSleepContextEndsWithTheContext(t *testing.T) {
	if err := sleepContext(context.Background(), time.Nanosecond); err != nil {
		t.Fatalf("sleepContext elapsed = %v, want nil", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := sleepContext(ctx, time.Hour); !errors.Is(err, context.Canceled) {
		t.Fatalf("sleepContext cancelled = %v, want context.Canceled", err)
	}
}

// TestGoStringShowsNoSecret checks %#v of a client and a session.
func TestGoStringShowsNoSecret(t *testing.T) {
	if got := fmt.Sprintf("%#v", &Client{}); got != "bothyapi.Client" {
		t.Fatalf("%%#v of a client = %q", got)
	}
	s := Session{ServerURL: "https://bothy.example.com", DeviceID: "dev", AccessToken: "secret-access", RefreshToken: "secret-refresh"}
	got := s.GoString()
	if strings.Contains(got, "secret") || !strings.Contains(got, "dev") {
		t.Fatalf("GoString of a session = %q", got)
	}
}

// TestDefaultSessionPathIsUnderTheUserConfigDir checks the default path and its error.
func TestDefaultSessionPathIsUnderTheUserConfigDir(t *testing.T) {
	if runtime.GOOS == "windows" || runtime.GOOS == "darwin" || runtime.GOOS == "ios" || runtime.GOOS == "plan9" {
		t.Skip("the configuration directory comes from XDG_CONFIG_HOME or HOME only on Unix")
	}
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	got, err := DefaultSessionPath()
	if err != nil || got != filepath.Join(dir, "bothy", "import-session.json") {
		t.Fatalf("DefaultSessionPath() = %q, %v", got, err)
	}
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("HOME", "")
	if _, err := DefaultSessionPath(); err == nil {
		t.Fatal("DefaultSessionPath without HOME returned no error")
	}
}

// TestSessionFileRefusesAnOpenDirectory checks Store and Lock in a directory that others can read.
func TestSessionFileRefusesAnOpenDirectory(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("no mode check on Windows (D-M8-10-2)")
	}
	dir := filepath.Join(t.TempDir(), "open")
	if err := os.Mkdir(dir, 0o755); err != nil { // #nosec G301 -- the test needs a directory others can read
		t.Fatal(err)
	}
	if err := os.Chmod(dir, 0o755); err != nil { // #nosec G302 -- see above, against the umask
		t.Fatal(err)
	}
	f := SessionFile{Path: filepath.Join(dir, "s.json")}
	if err := f.Store(&Session{}); !errors.Is(err, ErrInsecureSessionFile) {
		t.Fatalf("Store = %v, want ErrInsecureSessionFile", err)
	}
	if _, err := f.Lock(); !errors.Is(err, ErrInsecureSessionFile) {
		t.Fatalf("Lock = %v, want ErrInsecureSessionFile", err)
	}
}

// TestSessionFileFailsBelowAFile checks Store, Lock and Remove when the directory is a file.
func TestSessionFileFailsBelowAFile(t *testing.T) {
	parent := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(parent, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	f := SessionFile{Path: filepath.Join(parent, "s.json")}
	if err := f.Store(&Session{}); err == nil {
		t.Fatal("Store below a file returned no error")
	}
	if _, err := f.Lock(); err == nil {
		t.Fatal("Lock below a file returned no error")
	}
	if err := f.Remove(); err == nil {
		t.Fatal("Remove below a file returned no error")
	}
}

// TestSessionFileStoreKeepsTheOldFileWhenTheRenameFails checks the cleanup of the temporary file.
func TestSessionFileStoreKeepsTheOldFileWhenTheRenameFails(t *testing.T) {
	dir := t.TempDir()
	if err := os.Chmod(dir, 0o700); err != nil { // #nosec G302 -- a directory needs the x bit; 0700 is the mode Store wants
		t.Fatal(err)
	}
	failed := errors.New("rename failed")
	f := SessionFile{Path: filepath.Join(dir, "s.json"), rename: func(string, string) error { return failed }}
	if err := f.Store(&Session{}); !errors.Is(err, failed) {
		t.Fatalf("Store = %v, want the rename error", err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("Store left %d files behind", len(entries))
	}
}
