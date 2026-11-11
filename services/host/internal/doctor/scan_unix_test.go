// SPDX-License-Identifier: Apache-2.0

//go:build linux || darwin

package doctor

import (
	"context"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/host/internal/clock"
	"github.com/OWNER/bothy/services/host/internal/hostos"
	"github.com/OWNER/bothy/services/host/internal/hostos/hostostest"
)

// countingRunner starts the real programs and counts them by path.
type countingRunner struct {
	mu     sync.Mutex
	starts map[string]int
}

func (r *countingRunner) Run(ctx context.Context, c hostos.Cmd) (hostos.Output, error) {
	r.mu.Lock()
	if r.starts == nil {
		r.starts = map[string]int{}
	}
	r.starts[c.Path]++
	r.mu.Unlock()
	return hostos.ExecRunner{}.Run(ctx, c)
}

func (r *countingRunner) total() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	n := 0
	for _, k := range r.starts {
		n += k
	}
	return n
}

// scanTree builds a real tree of folders and files below a temporary home and returns the home
// and the number of entries.
func scanTree(t *testing.T) (string, int) {
	t.Helper()
	home := t.TempDir()
	entries := 0
	for d := range 4 {
		dir := filepath.Join(home, fmt.Sprintf("d%d", d), "sub")
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
		entries += 2
		for f := range 6 {
			if err := os.WriteFile(filepath.Join(dir, fmt.Sprintf("f%d", f)), nil, 0o600); err != nil {
				t.Fatal(err)
			}
			entries++
		}
	}
	if err := os.Symlink(filepath.Join(home, "d0"), filepath.Join(home, "link")); err != nil {
		t.Fatal(err)
	}
	return home, entries + 1
}

// scanEnv is the helper of the session user self over the real file system, with the security
// reader of macOS (owner, group and mode from lstat, the extended ACL from "/bin/ls -led") and a
// runner that counts the programs it starts.
func scanEnv(w *hostostest.World, self hostos.Account, r hostos.Runner) HelperEnv {
	return HelperEnv{
		GOOS: hostos.Darwin, Self: self, Users: w.Users(),
		Security: hostos.DarwinSecurity{Runner: r, Users: w.Users()},
		Resolver: hostos.OSResolver{}, Files: hostos.OSLister{}, Clock: clock.Real(),
	}
}

// TestEntryScanStartsNoProgramPerEntry: the entry scan of HD11 judges the owner and the type of
// an entry, never its ACL, so it starts no program per entry. The security reader of macOS starts
// "/bin/ls -led" for the ACL of a path; with one start per entry the scan of a home of one
// million entries did not end within the hour of the platform run on macOS
// (TestPlatform_EntryScanMeasurement, docs/decisions.d/AUD-misc-misc-01.md). The test walks a
// real tree with the reader of macOS; on Linux the same reader runs with the ls of Linux.
func TestEntryScanStartsNoProgramPerEntry(t *testing.T) {
	me, err := user.Current()
	if err != nil {
		t.Fatal(err)
	}
	home, entries := scanTree(t)
	w := hostostest.New(hostos.Darwin)
	w.AddUser(alex, me.Uid, home, true)
	w.AddUser(sam, me.Uid+"0", "/Users/"+sam, true)
	self, err := w.Users().Lookup(alex)
	if err != nil {
		t.Fatal(err)
	}
	r := &countingRunner{}
	code := ScanEntries(context.Background(), scanEnv(w, self, r), Request{Others: []string{sam}, EntryScanTimeout: time.Hour})
	if code != "" {
		t.Errorf("code %q for a home whose entries all belong to its session user", code)
	}
	if n := r.total(); n != 0 {
		t.Errorf("the scan of %d entries started %d programs (%v), want none", entries, n, r.starts)
	}
}

// TestEntryScanStillFindsAForeignEntry: read without the ACL, the owner of an entry is still the
// real one. Every entry of the tree belongs to the account that runs the test; seen from another
// session user they are foreign entries.
func TestEntryScanStillFindsAForeignEntry(t *testing.T) {
	me, err := user.Current()
	if err != nil {
		t.Fatal(err)
	}
	home, _ := scanTree(t)
	w := hostostest.New(hostos.Darwin)
	w.AddUser(alex, me.Uid+"0", home, true)
	w.AddUser(sam, me.Uid, "/Users/"+sam, true)
	self, err := w.Users().Lookup(alex)
	if err != nil {
		t.Fatal(err)
	}
	r := &countingRunner{}
	code := ScanEntries(context.Background(), scanEnv(w, self, r), Request{Others: []string{sam}, EntryScanTimeout: time.Hour})
	if code != CodeForeignEntry {
		t.Errorf("code %q, want %s", code, CodeForeignEntry)
	}
	if n := r.total(); n != 0 {
		t.Errorf("the scan started %d programs (%v), want none", n, r.starts)
	}
}
