// SPDX-License-Identifier: Apache-2.0

package fs_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	iofs "io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/files/adapters/filestest"
	"github.com/OWNER/bothy/services/server/internal/files/adapters/fs"
	"github.com/OWNER/bothy/services/server/internal/files/app"
	"github.com/OWNER/bothy/services/server/internal/files/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// newRoot creates an empty root with mode 0700 inside the test's own temporary directory.
func newRoot(t *testing.T) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), "files")
	if err := os.Mkdir(root, 0o700); err != nil {
		t.Fatal(err)
	}
	return root
}

// openStore opens a store on root and closes it when the test ends.
func openStore(t *testing.T, root string) *fs.Store {
	t.Helper()
	s, err := fs.Open(fs.Config{Root: root})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func origKey(t *testing.T, id ids.ID) string {
	t.Helper()
	k, err := domain.OriginalKey(id, 1)
	if err != nil {
		t.Fatal(err)
	}
	return k
}

func partKey(t *testing.T, id ids.ID, n int) string {
	t.Helper()
	k, err := domain.UploadPartKey(id, n)
	if err != nil {
		t.Fatal(err)
	}
	return k
}

func putBytes(t *testing.T, s app.ObjectStore, key string, data []byte) {
	t.Helper()
	if err := s.Put(t.Context(), key, bytes.NewReader(data), int64(len(data))); err != nil {
		t.Fatalf("Put(%s): %v", key, err)
	}
}

func getBytes(t *testing.T, s app.ObjectStore, key string) []byte {
	t.Helper()
	rc, err := s.Get(t.Context(), key, nil)
	if err != nil {
		t.Fatalf("Get(%s): %v", key, err)
	}
	defer func() { _ = rc.Close() }()
	data, err := io.ReadAll(rc)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// entries lists every path below root relative to it, directories with a trailing slash.
func entries(t *testing.T, root string) []string {
	t.Helper()
	var out []string
	err := filepath.WalkDir(root, func(p string, d iofs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if p == root {
			return nil
		}
		rel, _ := filepath.Rel(root, p)
		if d.IsDir() {
			rel += "/"
		}
		out = append(out, filepath.ToSlash(rel))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func tmpEntries(t *testing.T, root string) []string {
	t.Helper()
	es, err := os.ReadDir(filepath.Join(root, ".tmp"))
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, e := range es {
		names = append(names, e.Name())
	}
	return names
}

func TestFSConformance(t *testing.T) {
	t.Parallel()
	filestest.RunObjectStoreConformance(t, func(t *testing.T) app.ObjectStore {
		return openStore(t, newRoot(t))
	}, filestest.Options{PartSize: 4096})
}

func TestOpenChecksRoot(t *testing.T) {
	// Not parallel: one case changes the expected owner of the package.
	base := t.TempDir()
	mk := func(name string, mode os.FileMode) string {
		p := filepath.Join(base, name)
		if err := os.Mkdir(p, mode); err != nil {
			t.Fatal(err)
		}
		if err := os.Chmod(p, mode); err != nil {
			t.Fatal(err)
		}
		return p
	}
	good := mk("good", 0o700)
	regular := filepath.Join(base, "regular")
	if err := os.WriteFile(regular, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(base, "link")
	if err := os.Symlink(good, link); err != nil {
		t.Fatal(err)
	}
	refused := map[string]string{
		"relative":      "files/root",
		"not clean":     good + "/",
		"dot element":   good + "/../good",
		"empty":         "",
		"missing":       filepath.Join(base, "missing"),
		"regular file":  regular,
		"symbolic link": link,
		"group bits":    mk("group", 0o750),
		"other bits":    mk("other", 0o701),
		"sticky":        mk("sticky", 0o700|os.ModeSticky),
	}
	if err := os.Chmod(refused["sticky"], 0o700|os.ModeSticky); err != nil {
		t.Fatal(err)
	}
	check := func(name, root string) {
		t.Helper()
		s, err := fs.Open(fs.Config{Root: root})
		if err == nil {
			_ = s.Close()
			t.Errorf("%s: Open succeeded", name)
			return
		}
		msg := err.Error()
		if !strings.Contains(msg, "file.fs.root") {
			t.Errorf("%s: error %q does not name file.fs.root", name, msg)
		}
		if root != "" && (strings.Contains(msg, root) || strings.Contains(msg, base)) {
			t.Errorf("%s: error %q contains the path", name, msg)
		}
	}
	for name, root := range refused {
		check(name, root)
	}
	restore := fs.SetExpectedOwner(os.Geteuid() + 1)
	check("other owner", good)
	restore()

	s, err := fs.Open(fs.Config{Root: good})
	if err != nil {
		t.Fatalf("Open of a correct root: %v", err)
	}
	defer func() { _ = s.Close() }()
	info, err := os.Lstat(filepath.Join(good, ".tmp"))
	if err != nil || !info.IsDir() || info.Mode().Perm() != 0o700 {
		t.Fatalf(".tmp = %v, %v; want a directory with mode 0700", info, err)
	}
	// A second store on the same root opens too and leaves .tmp alone.
	marker := filepath.Join(good, ".tmp", "other-process")
	if err := os.WriteFile(marker, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	s2, err := fs.Open(fs.Config{Root: good})
	if err != nil {
		t.Fatalf("second Open: %v", err)
	}
	_ = s2.Close()
	if _, err := os.Stat(marker); err != nil {
		t.Errorf("Open removed an entry of .tmp: %v", err)
	}
	// A .tmp with group or other bits is refused.
	loose := mk("loose", 0o700)
	if err := os.Mkdir(filepath.Join(loose, ".tmp"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(filepath.Join(loose, ".tmp"), 0o777); err != nil { //nolint:gosec // G302: the case under test is a too open .tmp
		t.Fatal(err)
	}
	check(".tmp mode 0777", loose)
	// A .tmp that is a symbolic link is refused.
	linked := mk("linked", 0o700)
	if err := os.Symlink(base, filepath.Join(linked, ".tmp")); err != nil {
		t.Fatal(err)
	}
	check(".tmp is a link", linked)
}

func TestKeyMapsToPathBelowRoot(t *testing.T) {
	t.Parallel()
	root := newRoot(t)
	s := openStore(t, root)
	id := ids.MustParse("0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f5a6b")
	key := origKey(t, id)
	putBytes(t, s, key, []byte("hello"))
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(key))) //nolint:gosec // G304: a path below the test's own TempDir
	if err != nil || string(data) != "hello" {
		t.Fatalf("file at <root>/<key> = %q, %v", data, err)
	}
	want := []string{".tmp/", "f/", "f/5a/", "f/5a/6b/", "f/5a/6b/" + id.String() + "/", key}
	if got := entries(t, root); !slices.Equal(got, want) {
		t.Errorf("entries = %v, want %v", got, want)
	}
	for _, e := range want {
		info, err := os.Lstat(filepath.Join(root, filepath.FromSlash(strings.TrimSuffix(e, "/"))))
		if err != nil {
			t.Fatal(err)
		}
		wantMode := os.FileMode(0o600)
		if info.IsDir() {
			wantMode = 0o700
		}
		if info.Mode().Perm() != wantMode {
			t.Errorf("%s has mode %v, want %v", e, info.Mode().Perm(), wantMode)
		}
	}
}

// allOps runs every operation of the port with key and returns the errors by operation name.
func allOps(ctx context.Context, s app.ObjectStore, key string) map[string]error {
	rc, getErr := s.Get(ctx, key, nil)
	if rc != nil {
		_ = rc.Close()
	}
	_, headErr := s.Head(ctx, key)
	_, createErr := s.CreateMultipart(ctx, key)
	_, partErr := s.UploadPart(ctx, key, "", 1, strings.NewReader("x"), 1)
	_, presignErr := s.PresignUploadPart(ctx, key, "", 1, 1, time.Minute)
	return map[string]error{
		"Put":               s.Put(ctx, key, strings.NewReader("x"), 1),
		"Get":               getErr,
		"Head":              headErr,
		"Delete":            s.Delete(ctx, key),
		"CreateMultipart":   createErr,
		"UploadPart":        partErr,
		"CompleteMultipart": s.CompleteMultipart(ctx, key, "", []app.Part{{Number: 1, ETag: `"x"`}}),
		"AbortMultipart":    s.AbortMultipart(ctx, key, ""),
		"PresignUploadPart": presignErr,
	}
}

func TestTraversalKeysTouchNothing(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	root := filepath.Join(dir, "files")
	if err := os.Mkdir(root, 0o700); err != nil {
		t.Fatal(err)
	}
	canary := filepath.Join(dir, "canary")
	if err := os.WriteFile(canary, []byte("canary"), 0o600); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-time.Hour).Truncate(time.Second)
	if err := os.Chtimes(canary, old, old); err != nil {
		t.Fatal(err)
	}
	s := openStore(t, root)
	keys := []string{"../canary", "f/../../canary", "/" + strings.TrimPrefix(filepath.ToSlash(canary), "/"), "..", canary}
	run := func(phase string) {
		for _, k := range keys {
			for op, err := range allOps(t.Context(), s, k) {
				if !errors.Is(err, app.ErrInvalidKey) {
					t.Errorf("%s: %s(%q) err = %v, want ErrInvalidKey", phase, op, k, err)
				}
			}
		}
		data, err := os.ReadFile(canary) //nolint:gosec // G304: the canary in the test's own TempDir
		info, err2 := os.Stat(canary)
		if err != nil || err2 != nil || string(data) != "canary" || !info.ModTime().Equal(old) {
			t.Errorf("%s: canary changed: %q %v %v", phase, data, err, err2)
		}
		es, _ := os.ReadDir(dir)
		var names []string
		for _, e := range es {
			names = append(names, e.Name())
		}
		if want := []string{"canary", "files"}; phase == "root present" && !slices.Equal(names, want) {
			t.Errorf("%s: entries next to the root = %v, want %v", phase, names, want)
		}
	}
	run("root present")
	if got := entries(t, root); !slices.Equal(got, []string{".tmp/"}) {
		t.Errorf("entries below root = %v", got)
	}
	// Without the root every file system call would fail; ErrInvalidKey proves the check comes
	// first.
	if err := os.RemoveAll(root); err != nil {
		t.Fatal(err)
	}
	run("root removed")
}

func TestNoSymbolicLinkFollowed(t *testing.T) {
	t.Parallel()
	outside := t.TempDir()
	canary := filepath.Join(outside, "canary")
	writeCanary := func() {
		if err := os.WriteFile(canary, []byte("canary"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	writeCanary()
	ops := []string{"Put", "Get", "Head", "Delete", "CreateMultipart", "UploadPart"}

	type setup func(t *testing.T, root string, s *fs.Store) (key string)
	setups := map[string]setup{
		"fan-out level links outside": func(t *testing.T, root string, _ *fs.Store) string {
			id := ids.New()
			key := origKey(t, id)
			aa := filepath.Join(root, "f", key[2:4])
			if err := os.MkdirAll(filepath.Dir(aa), 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(outside, aa); err != nil {
				t.Fatal(err)
			}
			return key
		},
		"final component links to canary": func(t *testing.T, root string, _ *fs.Store) string {
			key := origKey(t, ids.New())
			p := filepath.Join(root, filepath.FromSlash(key))
			if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(canary, p); err != nil {
				t.Fatal(err)
			}
			return key
		},
		"link inside the root to another object": func(t *testing.T, root string, s *fs.Store) string {
			other := origKey(t, ids.New())
			putBytes(t, s, other, []byte("other object"))
			key := origKey(t, ids.New())
			p := filepath.Join(root, filepath.FromSlash(key))
			if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(filepath.Join(root, filepath.FromSlash(other)), p); err != nil {
				t.Fatal(err)
			}
			return key
		},
		"uploads directory links outside": func(t *testing.T, root string, _ *fs.Store) string {
			id := ids.New()
			if err := os.MkdirAll(filepath.Join(root, "uploads"), 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(outside, filepath.Join(root, "uploads", id.String())); err != nil {
				t.Fatal(err)
			}
			return origKey(t, id)
		},
	}
	for name, mk := range setups {
		root := newRoot(t)
		s := openStore(t, root)
		key := mk(t, root, s)
		errs := allOps(t.Context(), s, key)
		for _, op := range ops {
			if name == "uploads directory links outside" && (op == "Get" || op == "Head" || op == "Delete" || op == "Put") {
				continue // these do not touch uploads/
			}
			err := errs[op]
			if err == nil || errors.Is(err, app.ErrNotFound) || errors.Is(err, app.ErrInvalidKey) {
				t.Errorf("%s: %s err = %v, want a storage failure", name, op, err)
			}
		}
		data, err := os.ReadFile(canary) //nolint:gosec // G304: the canary in the test's own TempDir
		if err != nil || string(data) != "canary" {
			t.Errorf("%s: canary = %q, %v", name, data, err)
			writeCanary()
		}
		es, _ := os.ReadDir(outside)
		if len(es) != 1 {
			t.Errorf("%s: %d entries outside the root, want only the canary", name, len(es))
		}
	}
}

// gate is a body that yields first, then blocks until released, then yields rest.
type gate struct {
	first, rest []byte
	reached     chan struct{}
	release     chan struct{}
	stage       int
}

func newGate(first, rest []byte) *gate {
	return &gate{first: first, rest: rest, reached: make(chan struct{}), release: make(chan struct{})}
}

func (g *gate) Read(p []byte) (int, error) {
	switch g.stage {
	case 0:
		n := copy(p, g.first)
		g.first = g.first[n:]
		if len(g.first) == 0 {
			g.stage = 1
		}
		return n, nil
	case 1:
		close(g.reached)
		<-g.release
		g.stage = 2
		fallthrough
	default:
		if len(g.rest) == 0 {
			return 0, io.EOF
		}
		n := copy(p, g.rest)
		g.rest = g.rest[n:]
		return n, nil
	}
}

func TestPutIsAtomic(t *testing.T) {
	t.Parallel()
	root := newRoot(t)
	s := openStore(t, root)
	key := origKey(t, ids.New())
	first, rest := bytes.Repeat([]byte("a"), 70000), bytes.Repeat([]byte("b"), 70000)
	g := newGate(first, rest)
	done := make(chan error, 1)
	go func() { done <- s.Put(context.Background(), key, g, int64(len(first)+len(rest))) }()
	awaitGate(t, g, done)
	if _, err := s.Head(t.Context(), key); !errors.Is(err, app.ErrNotFound) {
		t.Errorf("Head during the write err = %v, want ErrNotFound", err)
	}
	if rc, err := s.Get(t.Context(), key, nil); !errors.Is(err, app.ErrNotFound) {
		if rc != nil {
			_ = rc.Close()
		}
		t.Errorf("Get during the write err = %v, want ErrNotFound", err)
	}
	if _, err := os.Lstat(filepath.Join(root, filepath.FromSlash(key))); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("target exists during the write: %v", err)
	}
	if n := len(tmpEntries(t, root)); n != 1 {
		t.Errorf(".tmp holds %d entries during the write, want 1", n)
	}
	close(g.release)
	if err := <-done; err != nil {
		t.Fatalf("Put: %v", err)
	}
	if got := getBytes(t, s, key); !bytes.Equal(got, append(first, rest...)) {
		t.Errorf("content after the write has %d bytes", len(got))
	}
	if n := len(tmpEntries(t, root)); n != 0 {
		t.Errorf(".tmp holds %d entries after the write", n)
	}
}

func TestPutSyncOrder(t *testing.T) {
	t.Parallel()
	root := newRoot(t)
	s := openStore(t, root)
	var mu sync.Mutex
	var events []string
	s.SetHook(func(event, _ string) {
		if event == "before-rename" || event == "before-open" || event == "sync-new-dir" {
			return
		}
		mu.Lock()
		events = append(events, event)
		mu.Unlock()
	})
	want := []string{"write", "sync-file", "rename", "sync-dir"}
	take := func(op string) {
		t.Helper()
		mu.Lock()
		defer mu.Unlock()
		if !slices.Equal(events, want) {
			t.Errorf("%s steps = %v, want %v", op, events, want)
		}
		events = nil
	}
	id := ids.New()
	putBytes(t, s, origKey(t, id), []byte("x"))
	take("Put")
	k2 := origKey(t, ids.New())
	up, err := s.CreateMultipart(t.Context(), k2)
	if err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	events = nil
	mu.Unlock()
	p, err := s.UploadPart(t.Context(), k2, up, 1, strings.NewReader("part"), 4)
	if err != nil {
		t.Fatal(err)
	}
	take("UploadPart")
	if err := s.CompleteMultipart(t.Context(), k2, up, []app.Part{p}); err != nil {
		t.Fatal(err)
	}
	take("CompleteMultipart")
}

type errBody struct{ n int }

func (b *errBody) Read(p []byte) (int, error) {
	if b.n > 0 {
		k := min(len(p), b.n)
		b.n -= k
		return k, nil
	}
	return 0, errors.New("body broke")
}

func TestFailedWritesLeaveNothing(t *testing.T) {
	t.Parallel()
	root := newRoot(t)
	s := openStore(t, root)
	cancelled, cancel := context.WithCancel(t.Context())
	cancel()
	type attempt struct {
		name string
		ctx  context.Context
		body io.Reader
		size int64
	}
	attempts := []attempt{
		{"short body", t.Context(), strings.NewReader("abc"), 4},
		{"long body", t.Context(), strings.NewReader("abcde"), 4},
		{"cancelled context", cancelled, strings.NewReader("abcd"), 4},
		{"body error", t.Context(), &errBody{n: 100000}, 200000},
	}
	for _, a := range attempts {
		key := origKey(t, ids.New())
		if err := s.Put(a.ctx, key, a.body, a.size); err == nil {
			t.Errorf("Put with %s succeeded", a.name)
		}
		if _, err := s.Head(t.Context(), key); !errors.Is(err, app.ErrNotFound) {
			t.Errorf("Put with %s left an object: %v", a.name, err)
		}
		if n := len(tmpEntries(t, root)); n != 0 {
			t.Errorf("Put with %s left %d temporary files", a.name, n)
		}
	}
	for _, a := range []attempt{
		{"short body", t.Context(), strings.NewReader("abc"), 4},
		{"long body", t.Context(), strings.NewReader("abcde"), 4},
		{"cancelled context", cancelled, strings.NewReader("abcd"), 4},
		{"body error", t.Context(), &errBody{n: 100000}, 200000},
	} {
		id := ids.New()
		key := origKey(t, id)
		up, err := s.CreateMultipart(t.Context(), key)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := s.UploadPart(a.ctx, key, up, 1, a.body, a.size); err == nil {
			t.Errorf("UploadPart with %s succeeded", a.name)
		}
		if _, err := s.Head(t.Context(), partKey(t, id, 1)); !errors.Is(err, app.ErrNotFound) {
			t.Errorf("UploadPart with %s left a part: %v", a.name, err)
		}
		if n := len(tmpEntries(t, root)); n != 0 {
			t.Errorf("UploadPart with %s left %d temporary files", a.name, n)
		}
	}
}

func TestPutReplacesAtomically(t *testing.T) {
	t.Parallel()
	root := newRoot(t)
	s := openStore(t, root)
	key := origKey(t, ids.New())
	oldData := bytes.Repeat([]byte("o"), 100000)
	putBytes(t, s, key, oldData)
	first, rest := bytes.Repeat([]byte("n"), 60000), bytes.Repeat([]byte("N"), 60000)
	g := newGate(first, rest)
	done := make(chan error, 1)
	go func() { done <- s.Put(context.Background(), key, g, int64(len(first)+len(rest))) }()
	awaitGate(t, g, done)
	if got := getBytes(t, s, key); !bytes.Equal(got, oldData) {
		t.Errorf("Get during the replace returned %d bytes, not the old content", len(got))
	}
	// A reader opened before the rename reads the old object to its end after the rename.
	early, err := s.Get(t.Context(), key, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = early.Close() }()
	head := make([]byte, 10)
	if _, err := io.ReadFull(early, head); err != nil {
		t.Fatal(err)
	}
	close(g.release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	tail, err := io.ReadAll(early)
	if err != nil || !bytes.Equal(append(head, tail...), oldData) {
		t.Errorf("reader opened before the rename read %d bytes, not the old content: %v", len(head)+len(tail), err)
	}
	if got := getBytes(t, s, key); !bytes.Equal(got, append(first, rest...)) {
		t.Errorf("Get after the replace returned %d bytes, not the new content", len(got))
	}
}

func TestDeleteRemovesEmptyFileDirectory(t *testing.T) {
	t.Parallel()
	root := newRoot(t)
	s := openStore(t, root)
	id := ids.New()
	key := origKey(t, id)
	small, _ := domain.VariantKey(id, domain.VariantSmall)
	fileDir := filepath.Join(root, filepath.FromSlash(filepath.Dir(key)))
	putBytes(t, s, key, []byte("o"))
	putBytes(t, s, small, []byte("s"))
	if err := s.Delete(t.Context(), small); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(fileDir); err != nil {
		t.Errorf("file directory removed while an object is left: %v", err)
	}
	if err := s.Delete(t.Context(), key); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(fileDir); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("file directory after deleting the last object: %v", err)
	}
	if info, err := os.Stat(filepath.Dir(fileDir)); err != nil || !info.IsDir() {
		t.Errorf("fan-out level f/<aa>/<bb> removed: %v", err)
	}
	// A new object of the same file recreates the directory.
	putBytes(t, s, key, []byte("again"))
	if got := getBytes(t, s, key); string(got) != "again" {
		t.Errorf("content after recreate = %q", got)
	}
}

func TestPutRecreatesVanishedDirectory(t *testing.T) {
	t.Parallel()
	root := newRoot(t)
	s := openStore(t, root)
	key := origKey(t, ids.New())
	dir := filepath.Join(root, filepath.FromSlash(filepath.Dir(key)))
	var once sync.Once
	removed := false
	s.SetHook(func(event, _ string) {
		if event == "before-rename" {
			once.Do(func() { removed = os.Remove(dir) == nil })
		}
	})
	putBytes(t, s, key, []byte("survives"))
	if !removed {
		t.Fatal("the seam did not remove the directory")
	}
	if got := getBytes(t, s, key); string(got) != "survives" {
		t.Errorf("content = %q", got)
	}
}

func quotedSHA256(b []byte) string {
	sum := sha256.Sum256(b)
	return `"` + hex.EncodeToString(sum[:]) + `"`
}

func TestMultipartLayout(t *testing.T) {
	t.Parallel()
	root := newRoot(t)
	s := openStore(t, root)
	for _, finish := range []string{"complete", "abort"} {
		id := ids.New()
		key := origKey(t, id)
		up, err := s.CreateMultipart(t.Context(), key)
		if err != nil || up != "" {
			t.Fatalf("CreateMultipart = %q, %v; want \"\"", up, err)
		}
		if again, err := s.CreateMultipart(t.Context(), key); err != nil || again != "" {
			t.Fatalf("second CreateMultipart = %q, %v", again, err)
		}
		data := [][]byte{[]byte("first part"), []byte("second")}
		var parts []app.Part
		for i, d := range data {
			p, err := s.UploadPart(t.Context(), key, up, i+1, bytes.NewReader(d), int64(len(d)))
			if err != nil {
				t.Fatal(err)
			}
			if p.ETag != quotedSHA256(d) {
				t.Errorf("ETag = %s, want %s", p.ETag, quotedSHA256(d))
			}
			parts = append(parts, p)
			pk := partKey(t, id, i+1)
			raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(pk))) //nolint:gosec // G304: a path below the test's own TempDir
			if err != nil || !bytes.Equal(raw, d) {
				t.Errorf("part file %s = %q, %v", pk, raw, err)
			}
			if got := getBytes(t, s, pk); !bytes.Equal(got, d) {
				t.Errorf("Get(%s) = %q", pk, got)
			}
			if info, err := s.Head(t.Context(), pk); err != nil || info.Size != int64(len(d)) {
				t.Errorf("Head(%s) = %+v, %v", pk, info, err)
			}
		}
		if finish == "complete" {
			if err := s.CompleteMultipart(t.Context(), key, up, parts); err != nil {
				t.Fatal(err)
			}
		} else if err := s.AbortMultipart(t.Context(), key, up); err != nil {
			t.Fatal(err)
		}
		if _, err := os.Lstat(filepath.Join(root, "uploads", id.String())); !errors.Is(err, os.ErrNotExist) {
			t.Errorf("after %s uploads/<file_id> still exists: %v", finish, err)
		}
	}
}

func TestMultipartOnlyForOriginalKeys(t *testing.T) {
	t.Parallel()
	s := openStore(t, newRoot(t))
	id := ids.New()
	small, _ := domain.VariantKey(id, domain.VariantSmall)
	for _, k := range []string{small, partKey(t, id, 1), "a/b"} {
		if _, err := s.CreateMultipart(t.Context(), k); !errors.Is(err, app.ErrInvalidKey) {
			t.Errorf("CreateMultipart(%s) err = %v, want ErrInvalidKey", k, err)
		}
	}
}

func TestUploadPartWithForeignUploadID(t *testing.T) {
	t.Parallel()
	root := newRoot(t)
	s := openStore(t, root)
	id := ids.New()
	key := origKey(t, id)
	up, err := s.CreateMultipart(t.Context(), key)
	if err != nil {
		t.Fatal(err)
	}
	p, err := s.UploadPart(t.Context(), key, up, 1, strings.NewReader("abc"), 3)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.UploadPart(t.Context(), key, "foreign", 2, strings.NewReader("x"), 1); !errors.Is(err, app.ErrUploadNotFound) {
		t.Errorf("UploadPart err = %v, want ErrUploadNotFound", err)
	}
	if err := s.CompleteMultipart(t.Context(), key, "foreign", []app.Part{p}); !errors.Is(err, app.ErrUploadNotFound) {
		t.Errorf("CompleteMultipart err = %v, want ErrUploadNotFound", err)
	}
	if err := s.AbortMultipart(t.Context(), key, "foreign"); err != nil {
		t.Errorf("AbortMultipart = %v, want nil", err)
	}
	if got := getBytes(t, s, partKey(t, id, 1)); string(got) != "abc" {
		t.Errorf("open upload touched: part = %q", got)
	}
}

func TestUploadPartReplacesSameNumber(t *testing.T) {
	t.Parallel()
	s := openStore(t, newRoot(t))
	key := origKey(t, ids.New())
	up, err := s.CreateMultipart(t.Context(), key)
	if err != nil {
		t.Fatal(err)
	}
	up1 := func(body string) app.Part {
		p, err := s.UploadPart(t.Context(), key, up, 1, strings.NewReader(body), int64(len(body)))
		if err != nil {
			t.Fatal(err)
		}
		return p
	}
	a := up1("same")
	if b := up1("same"); b != a {
		t.Errorf("same bytes gave %+v, then %+v", a, b)
	}
	c := up1("other")
	if c.ETag == a.ETag {
		t.Error("other bytes kept the ETag")
	}
	if err := s.CompleteMultipart(t.Context(), key, up, []app.Part{a}); !errors.Is(err, app.ErrInvalidPart) {
		t.Errorf("CompleteMultipart with the old ETag err = %v, want ErrInvalidPart", err)
	}
	if err := s.CompleteMultipart(t.Context(), key, up, []app.Part{c}); err != nil {
		t.Fatal(err)
	}
	if got := getBytes(t, s, key); string(got) != "other" {
		t.Errorf("object = %q", got)
	}
}

func TestPresignNotSupported(t *testing.T) {
	t.Parallel()
	root := newRoot(t)
	s := openStore(t, root)
	_, err := s.PresignUploadPart(t.Context(), origKey(t, ids.New()), "", 1, 1, time.Minute)
	if !errors.Is(err, app.ErrNotSupported) {
		t.Errorf("PresignUploadPart err = %v, want ErrNotSupported", err)
	}
	// The port contract refuses a part number below 1 first, on every driver.
	if _, err := s.PresignUploadPart(t.Context(), origKey(t, ids.New()), "", 0, 1, time.Minute); !errors.Is(err, app.ErrInvalidPart) {
		t.Errorf("PresignUploadPart(0) err = %v, want ErrInvalidPart", err)
	}
	// So is a size below 1 (docs/DECISIONS.md M3-02/9).
	if _, err := s.PresignUploadPart(t.Context(), origKey(t, ids.New()), "", 1, 0, time.Minute); !errors.Is(err, app.ErrSizeMismatch) {
		t.Errorf("PresignUploadPart(size 0) err = %v, want ErrSizeMismatch", err)
	}
	if got := entries(t, root); !slices.Equal(got, []string{".tmp/"}) {
		t.Errorf("entries = %v", got)
	}
}

func TestTwoStoresShareRoot(t *testing.T) {
	t.Parallel()
	root := newRoot(t)
	api, worker := openStore(t, root), openStore(t, root)
	ids3 := []ids.ID{ids.New(), ids.New(), ids.New()}
	var wg sync.WaitGroup
	errc := make(chan error, 1000)
	for i, st := range []*fs.Store{api, worker} {
		for _, id := range ids3 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for round := range 20 {
					var key string
					if i == 0 {
						key, _ = domain.OriginalKey(id, round+1)
					} else {
						key, _ = domain.VariantKey(id, []domain.Variant{domain.VariantSmall, domain.VariantMedium, domain.VariantLarge, domain.VariantPoster}[round%4])
					}
					body := bytes.Repeat([]byte{byte(round)}, 1000+round)
					if err := st.Put(context.Background(), key, bytes.NewReader(body), int64(len(body))); err != nil {
						errc <- err
						return
					}
					rc, err := st.Get(context.Background(), key, nil)
					if err != nil {
						errc <- err
						return
					}
					got, err := io.ReadAll(rc)
					_ = rc.Close()
					if err != nil || !bytes.Equal(got, body) {
						errc <- errors.New("wrong content " + key)
						return
					}
					if err := st.Delete(context.Background(), key); err != nil {
						errc <- err
						return
					}
				}
			}()
		}
	}
	wg.Wait()
	close(errc)
	for err := range errc {
		t.Error(err)
	}
	if n := len(tmpEntries(t, root)); n != 0 {
		t.Errorf(".tmp holds %d entries", n)
	}
}

// awaitGate waits until the body of a write blocks halfway; a write that ends before fails the
// test instead of hanging it.
func awaitGate(t *testing.T, g *gate, done <-chan error) {
	t.Helper()
	select {
	case <-g.reached:
	case err := <-done:
		t.Fatalf("Put ended before reading half the body: %v", err)
	}
}

// TestGetRefusesLinkSwappedBeforeOpen: os.Root follows a link inside the root even with
// O_NOFOLLOW, so a link that replaces the object between the Lstat checks and the open must be
// caught after the open. Get fails with a storage failure and never returns the other object.
func TestGetRefusesLinkSwappedBeforeOpen(t *testing.T) {
	t.Parallel()
	root := newRoot(t)
	s := openStore(t, root)
	other := origKey(t, ids.New())
	putBytes(t, s, other, []byte("other object"))
	key := origKey(t, ids.New())
	putBytes(t, s, key, []byte("own object"))
	p := filepath.Join(root, filepath.FromSlash(key))
	var once sync.Once
	s.SetHook(func(event, name string) {
		if event != "before-open" || name != key {
			return
		}
		once.Do(func() {
			if err := os.Remove(p); err != nil {
				t.Error(err)
			}
			// A relative target stays inside the root, so os.Root follows it; an absolute one
			// would already fail in os.Root and prove nothing about the driver.
			target, err := filepath.Rel(filepath.Dir(p), filepath.Join(root, filepath.FromSlash(other)))
			if err != nil {
				t.Error(err)
			}
			if err := os.Symlink(target, p); err != nil {
				t.Error(err)
			}
		})
	})
	rc, err := s.Get(t.Context(), key, nil)
	if err == nil {
		data, _ := io.ReadAll(rc)
		_ = rc.Close()
		t.Fatalf("Get through a swapped link succeeded with %q", data)
	}
	if !errors.Is(err, fs.ErrSymlink) {
		t.Errorf("Get err = %v, want the symbolic link failure", err)
	}
}

// TestGetAcceptsConcurrentReplace: a Put that replaces the object between the checks and the
// open is a valid read of the new object, not a failure.
func TestGetAcceptsConcurrentReplace(t *testing.T) {
	t.Parallel()
	root := newRoot(t)
	s := openStore(t, root)
	key := origKey(t, ids.New())
	putBytes(t, s, key, []byte("old object"))
	var once sync.Once
	s.SetHook(func(event, name string) {
		if event == "before-open" && name == key {
			once.Do(func() {
				p := filepath.Join(root, filepath.FromSlash(key))
				if err := os.WriteFile(p+"-new", []byte("new object"), 0o600); err != nil {
					t.Error(err)
				}
				if err := os.Rename(p+"-new", p); err != nil {
					t.Error(err)
				}
			})
		}
	})
	if got := getBytes(t, s, key); string(got) != "new object" {
		t.Errorf("Get = %q, want the new object", got)
	}
}

// TestCompleteSurvivesCleanupFailure: when the parts cannot be removed after the rename, the
// completion still succeeds (the object is done), and Delete of the original removes the parts
// left behind, so deleting a file removes all its bytes (12 "12").
func TestCompleteSurvivesCleanupFailure(t *testing.T) {
	t.Parallel()
	if os.Geteuid() == 0 {
		t.Skip("root ignores the read-only directory that makes the cleanup fail")
	}
	root := newRoot(t)
	s := openStore(t, root)
	id := ids.New()
	key := origKey(t, id)
	up, err := s.CreateMultipart(t.Context(), key)
	if err != nil {
		t.Fatal(err)
	}
	p, err := s.UploadPart(t.Context(), key, up, 1, strings.NewReader("part bytes"), 10)
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(root, "uploads", id.String())
	if err := os.Chmod(dir, 0o500); err != nil { //nolint:gosec // G302: a read-only upload directory is the case under test
		t.Fatal(err)
	}
	if err := s.CompleteMultipart(t.Context(), key, up, []app.Part{p}); err != nil {
		t.Fatalf("CompleteMultipart with a failing cleanup: %v", err)
	}
	if got := getBytes(t, s, key); string(got) != "part bytes" {
		t.Errorf("object = %q", got)
	}
	if err := os.Chmod(dir, 0o700); err != nil { //nolint:gosec // G302: a directory needs the x bit
		t.Fatal(err)
	}
	if _, err := os.Lstat(dir); err != nil {
		t.Fatalf("the parts were removed although the directory was read-only: %v", err)
	}
	if err := s.Delete(t.Context(), key); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(dir); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("Delete of the original left the parts: %v", err)
	}
	// Also when the object is already gone, as for a retried delete.
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := s.Delete(t.Context(), key); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(dir); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("Delete of a missing original left the parts: %v", err)
	}
}

// TestUploadPartAgainstConcurrentAbort: an abort that removes uploads/<file_id> while a part is
// written gives ErrUploadNotFound, recreates nothing and leaves no temporary file.
func TestUploadPartAgainstConcurrentAbort(t *testing.T) {
	t.Parallel()
	root := newRoot(t)
	s := openStore(t, root)
	id := ids.New()
	key := origKey(t, id)
	up, err := s.CreateMultipart(t.Context(), key)
	if err != nil {
		t.Fatal(err)
	}
	s.SetHook(func(event, _ string) {
		if event == "before-rename" {
			if err := os.RemoveAll(filepath.Join(root, "uploads", id.String())); err != nil {
				t.Error(err)
			}
		}
	})
	if _, err := s.UploadPart(t.Context(), key, up, 1, strings.NewReader("part"), 4); !errors.Is(err, app.ErrUploadNotFound) {
		t.Errorf("UploadPart err = %v, want ErrUploadNotFound", err)
	}
	if _, err := os.Lstat(filepath.Join(root, "uploads", id.String())); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("the aborted upload directory came back: %v", err)
	}
	if n := len(tmpEntries(t, root)); n != 0 {
		t.Errorf(".tmp holds %d entries", n)
	}
}

// TestNewDirectoriesAreSynced: every directory a write creates is made durable in its parent,
// so an object whose Put returned survives a crash with its whole path.
func TestNewDirectoriesAreSynced(t *testing.T) {
	t.Parallel()
	s := openStore(t, newRoot(t))
	var mu sync.Mutex
	var synced []string
	s.SetHook(func(event, name string) {
		if event == "sync-new-dir" {
			mu.Lock()
			synced = append(synced, name)
			mu.Unlock()
		}
	})
	id := ids.MustParse("0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f5a6b")
	putBytes(t, s, origKey(t, id), []byte("x"))
	want := []string{"f", "f/5a", "f/5a/6b", "f/5a/6b/" + id.String()}
	if !slices.Equal(synced, want) {
		t.Errorf("synced new directories = %v, want %v", synced, want)
	}
	synced = nil
	putBytes(t, s, origKey(t, id)[:len(origKey(t, id))-1]+"2", []byte("y"))
	if len(synced) != 0 {
		t.Errorf("existing directories synced again: %v", synced)
	}
}
