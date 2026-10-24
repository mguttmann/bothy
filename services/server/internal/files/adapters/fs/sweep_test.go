// SPDX-License-Identifier: Apache-2.0

package fs_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// TestSweepTemp holds the sweep of <root>/.tmp (12 4.2, M3-08): an entry older than the bound is
// removed and counted; a fresh entry and every object under f/ and uploads/ stay; a symbolic link
// in .tmp to a canary file outside the root is removed as a link and the canary keeps its content.
func TestSweepTemp(t *testing.T) {
	t.Parallel()
	root := newRoot(t)
	s := openStore(t, root)
	id := ids.New()
	putBytes(t, s, origKey(t, id), []byte("original"))
	putBytes(t, s, partKey(t, id, 1), []byte("part"))
	// The bound lies after the creation of every entry, so the link (whose own time a portable
	// test cannot set) is older than it; the fresh entry is moved after it.
	bound := time.Now().Add(time.Hour)
	old, fresh := bound.Add(-time.Hour), bound.Add(time.Hour)
	tmp := filepath.Join(root, ".tmp")
	stale := filepath.Join(tmp, "stale")
	young := filepath.Join(tmp, "young")
	for _, p := range []string{stale, young} {
		if err := os.WriteFile(p, []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	canary := filepath.Join(t.TempDir(), "canary")
	if err := os.WriteFile(canary, []byte("canary"), 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(tmp, "link")
	if err := os.Symlink(canary, link); err != nil {
		t.Fatal(err)
	}
	for p, at := range map[string]time.Time{stale: old, young: fresh} {
		if err := os.Chtimes(p, at, at); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Chtimes(filepath.Join(root, filepath.FromSlash(origKey(t, id))), old, old); err != nil {
		t.Fatal(err)
	}
	n, err := s.SweepTemp(t.Context(), bound)
	if err != nil || n != 2 {
		t.Fatalf("SweepTemp = %d, %v; want 2", n, err)
	}
	for _, p := range []string{stale, link} {
		if _, err := os.Lstat(p); !os.IsNotExist(err) {
			t.Errorf("%s stays: %v", filepath.Base(p), err)
		}
	}
	if _, err := os.Lstat(young); err != nil {
		t.Errorf("the fresh entry went: %v", err)
	}
	b, err := os.ReadFile(canary) // #nosec G304 -- a file in the test's own temporary directory
	if err != nil || string(b) != "canary" {
		t.Errorf("canary %q %v", b, err)
	}
	if got := getBytes(t, s, origKey(t, id)); string(got) != "original" {
		t.Errorf("original %q", got)
	}
	if got := getBytes(t, s, partKey(t, id, 1)); string(got) != "part" {
		t.Errorf("part %q", got)
	}
}
