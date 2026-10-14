// SPDX-License-Identifier: Apache-2.0

package wire_test

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/files/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/config"
	"github.com/OWNER/bothy/services/server/internal/wire"
)

// TestFileFSRootAbsolute holds the cross-check file_fs_root_absolute (docs/DECISIONS.md
// M3-01/12): with the driver fs the root is absolute and clean; with s3 it is not read.
func TestFileFSRootAbsolute(t *testing.T) {
	t.Parallel()
	const want = "config: file.storage.driver, file.fs.root: file_fs_root_absolute: " +
		"file.fs.root must be an absolute path without . or .. elements and without a trailing slash when file.storage.driver is fs " +
		"(source: file.storage.driver from default, file.fs.root from env BOTHY_FILE__FS__ROOT)"
	for _, root := range []string{"relative/x", "./x", "/a/../b", "/a/", "/a//b"} {
		got := load(t, "BOTHY_FILE__FS__ROOT="+root)
		if len(got) != 1 || got[0].String() != want {
			t.Errorf("root %q: problems %v, want\n%s", root, got, want)
		}
		noValues(t, got, root)
	}
	if got := load(t, "BOTHY_FILE__FS__ROOT=/srv/bothy/files"); got != nil {
		t.Errorf("absolute clean root: problems %v, want none", got)
	}
	if got := load(t, "BOTHY_FILE__STORAGE__DRIVER=s3", "BOTHY_FILE__FS__ROOT=relative/x"); got != nil {
		t.Errorf("driver s3 with a relative root: problems %v, want none", got)
	}
}

// filesValues loads the server schema with env added to the smallest working environment.
func filesValues(t *testing.T, env ...string) *config.Values {
	t.Helper()
	v, err := config.Load(schema(t), config.Sources{Env: append(testEnv(), env...)})
	if err != nil {
		t.Fatal(err)
	}
	return v
}

func TestFilesObjectStoreFS(t *testing.T) {
	t.Parallel()
	root := filepath.Join(t.TempDir(), "files")
	if err := os.Mkdir(root, 0o700); err != nil {
		t.Fatal(err)
	}
	store, err := wire.FilesObjectStore(filesValues(t, "BOTHY_FILE__FS__ROOT="+root))
	if err != nil {
		t.Fatalf("FilesObjectStore: %v", err)
	}
	if c, ok := store.(io.Closer); ok {
		t.Cleanup(func() { _ = c.Close() })
	}
	key, err := domain.OriginalKey(ids.New(), 1)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Put(t.Context(), key, strings.NewReader("round trip"), 10); err != nil {
		t.Fatal(err)
	}
	rc, err := store.Get(t.Context(), key, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = rc.Close() }()
	if got, _ := io.ReadAll(rc); !bytes.Equal(got, []byte("round trip")) {
		t.Errorf("Get = %q", got)
	}
}

func TestFilesObjectStoreFSRefusesBadRoot(t *testing.T) {
	t.Parallel()
	missing := filepath.Join(t.TempDir(), "missing")
	store, err := wire.FilesObjectStore(filesValues(t, "BOTHY_FILE__FS__ROOT="+missing))
	if err == nil || !strings.Contains(err.Error(), "file.fs.root") || strings.Contains(err.Error(), missing) || store != nil {
		t.Errorf("FilesObjectStore with a missing root: %v, %v", store, err)
	}
}

func TestFilesObjectStoreS3NotSupported(t *testing.T) {
	t.Parallel()
	_, err := wire.FilesObjectStore(filesValues(t, "BOTHY_FILE__STORAGE__DRIVER=s3"))
	if err == nil {
		t.Fatal("driver s3 opened")
	}
	if want := "config: file.storage.driver: driver s3 is not supported by this server version"; err.Error() != want {
		t.Errorf("error %q, want %q", err, want)
	}
}
