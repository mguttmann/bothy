// SPDX-License-Identifier: Apache-2.0

//go:build unix

package filesink_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/audit/adapters/filesink"
	"github.com/OWNER/bothy/services/server/internal/audit/app"
	"github.com/OWNER/bothy/services/server/internal/audit/app/sinktest"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// Compile-time check: the sink is the outgoing port of app.
var _ app.CheckpointSink = (*filesink.Sink)(nil)

// newSink returns a sink over a fresh directory from mktemp (t.TempDir) and that directory.
func newSink(t *testing.T) (*filesink.Sink, string, ids.ID) {
	t.Helper()
	root := t.TempDir()
	inst := ids.New()
	s, err := filesink.New(root, inst)
	if err != nil {
		t.Fatal(err)
	}
	return s, root, inst
}

// TestSinkWriteOnce: Put stores <root>/<instance_id>/<seq with 20 digits>.jws with mode 0600 in
// a directory with mode 0700; the same content again is accepted, other content for a stored
// seq fails and the stored file stays; List returns what was put.
func TestSinkWriteOnce(t *testing.T) {
	t.Parallel()
	s, root, inst := newSink(t)
	ctx := context.Background()
	if !s.Configured() {
		t.Fatal("a file sink is configured")
	}
	first := app.SignedCheckpoint{Seq: 42, JWS: []byte("aaa.bbb.ccc")}
	if err := s.Put(ctx, first); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(root, inst.String(), fmt.Sprintf("%020d.jws", 42))
	got, err := os.ReadFile(file) //nolint:gosec // G304: a file below t.TempDir
	if err != nil || string(got) != "aaa.bbb.ccc" {
		t.Fatalf("file %q, %v", got, err)
	}
	if fi, _ := os.Stat(file); fi.Mode().Perm() != 0o600 {
		t.Errorf("file mode %v, want 0600", fi.Mode().Perm())
	}
	if di, _ := os.Stat(filepath.Dir(file)); di.Mode().Perm() != 0o700 {
		t.Errorf("directory mode %v, want 0700", di.Mode().Perm())
	}
	if err := s.Put(ctx, first); err != nil {
		t.Errorf("the same content again: %v", err)
	}
	if err := s.Put(ctx, app.SignedCheckpoint{Seq: 42, JWS: []byte("xxx.yyy.zzz")}); err == nil {
		t.Error("other content for a stored seq was accepted")
	}
	if got, _ := os.ReadFile(file); string(got) != "aaa.bbb.ccc" { //nolint:gosec // G304: a file below t.TempDir
		t.Errorf("the stored file changed to %q", got)
	}
	if err := s.Put(ctx, app.SignedCheckpoint{Seq: 7, JWS: []byte("d.e.f")}); err != nil {
		t.Fatal(err)
	}
	// Leftovers of a crash and foreign files are not checkpoints.
	if err := os.WriteFile(filepath.Join(root, inst.String(), ".tmp-1"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	list, err := s.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	slices.SortFunc(list, func(a, b app.SignedCheckpoint) int { return int(a.Seq - b.Seq) })
	if len(list) != 2 || list[0].Seq != 7 || string(list[0].JWS) != "d.e.f" || list[1].Seq != 42 {
		t.Fatalf("List = %+v", list)
	}
	entries, _ := os.ReadDir(filepath.Dir(file))
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".tmp-") && e.Name() != ".tmp-1" {
			t.Errorf("temporary file %s left behind", e.Name())
		}
	}
}

// TestSinkConcurrentPut: concurrent writers of one seq never replace each other's file; the
// first one wins and every writer of other content fails.
func TestSinkConcurrentPut(t *testing.T) {
	t.Parallel()
	s, root, inst := newSink(t)
	ctx := context.Background()
	var wg sync.WaitGroup
	errs := make([]error, 8)
	for i := range errs {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs[i] = s.Put(ctx, app.SignedCheckpoint{Seq: 1, JWS: []byte{'a' + byte(i)}}) //nolint:gosec // G115: i is below 8
		}()
	}
	wg.Wait()
	got, err := os.ReadFile(filepath.Join(root, inst.String(), fmt.Sprintf("%020d.jws", 1))) //nolint:gosec // G304: a file below t.TempDir
	if err != nil {
		t.Fatal(err)
	}
	ok := 0
	for i, err := range errs {
		if err == nil {
			ok++
			if got[0] != 'a'+byte(i) {
				t.Errorf("writer %d succeeded, but the file holds %q", i, got)
			}
		}
	}
	if ok != 1 {
		t.Errorf("%d writers succeeded, want 1", ok)
	}
}

// TestSinkMissingRoot: a sink whose directory does not exist fails Put and List instead of
// creating it, because a missing mount would otherwise put the checkpoints on another volume.
func TestSinkMissingRoot(t *testing.T) {
	t.Parallel()
	root := filepath.Join(t.TempDir(), "absent")
	s, err := filesink.New(root, ids.New())
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Put(context.Background(), app.SignedCheckpoint{Seq: 1, JWS: []byte("a")}); err == nil {
		t.Error("Put into a missing root passed")
	}
	if _, err := s.List(context.Background()); err == nil {
		t.Error("List of a missing root passed")
	}
	if _, err := os.Stat(root); !os.IsNotExist(err) {
		t.Errorf("the root was created: %v", err)
	}
	if _, err := filesink.New("relative", ids.New()); err == nil {
		t.Error("a relative root was accepted")
	}
}

// TestSinkListWithoutCheckpoints: an existing root without a directory of the instance has no
// checkpoints yet.
func TestSinkListWithoutCheckpoints(t *testing.T) {
	t.Parallel()
	s, _, _ := newSink(t)
	list, err := s.List(context.Background())
	if err != nil || len(list) != 0 {
		t.Fatalf("List = %v, %v", list, err)
	}
}

// TestSinkRefusesLinksAndWideModes: a directory of the instance with a wider mode is set to
// 0700; a symbolic link in place of the directory or of a checkpoint file is refused, never
// followed.
func TestSinkRefusesLinksAndWideModes(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s, root, inst := newSink(t)
	dir := filepath.Join(root, inst.String())
	if err := os.Mkdir(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(dir, 0o777); err != nil { //nolint:gosec // G302: the test widens the mode on purpose
		t.Fatal(err)
	}
	if err := s.Put(ctx, app.SignedCheckpoint{Seq: 1, JWS: []byte("a")}); err != nil {
		t.Fatal(err)
	}
	if di, _ := os.Stat(dir); di.Mode().Perm() != 0o700 {
		t.Errorf("directory mode %v, want 0700", di.Mode().Perm())
	}
	// A link in place of a checkpoint file, pointing at a file with the same content.
	elsewhere := filepath.Join(t.TempDir(), "x")
	if err := os.WriteFile(elsewhere, []byte("b"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(elsewhere, filepath.Join(dir, fmt.Sprintf("%020d.jws", 2))); err != nil {
		t.Fatal(err)
	}
	if err := s.Put(ctx, app.SignedCheckpoint{Seq: 2, JWS: []byte("b")}); err == nil {
		t.Error("a link in place of a checkpoint file was accepted")
	}
	// A link in place of the directory of another instance.
	other := ids.New()
	if err := os.Symlink(t.TempDir(), filepath.Join(root, other.String())); err != nil {
		t.Fatal(err)
	}
	s2, err := filesink.New(root, other)
	if err != nil {
		t.Fatal(err)
	}
	if err := s2.Put(ctx, app.SignedCheckpoint{Seq: 1, JWS: []byte("a")}); err == nil {
		t.Error("a link in place of the directory was followed")
	}
	if _, err := s2.List(ctx); err == nil {
		t.Error("List followed a link in place of the directory")
	}
}

// TestConformance runs the conformance suite of app.CheckpointSink against the file sink
// (M7-22, Design note 3).
func TestConformance(t *testing.T) {
	t.Parallel()
	sinktest.Run(t, func(t *testing.T) app.CheckpointSink {
		t.Helper()
		s, _, _ := newSink(t)
		return s
	})
}
