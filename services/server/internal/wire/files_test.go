// SPDX-License-Identifier: Apache-2.0

package wire_test

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	filesapp "github.com/OWNER/bothy/services/server/internal/files/app"
	"github.com/OWNER/bothy/services/server/internal/files/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/config"
	"github.com/OWNER/bothy/services/server/internal/platform/egress"
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
	if got := load(t, append(s3Env("https://s3.example.com"), "BOTHY_FILE__FS__ROOT=relative/x")...); got != nil {
		t.Errorf("driver s3 with a relative root: problems %v, want none", got)
	}
}

// TestFilesConfigChecks holds the cross-checks of M3-03 (docs/DECISIONS.md D-M3-03-19):
// upload_part_plan, upload_part_size_s3 and presigned_mode_unavailable. Their messages name keys,
// never values.
func TestFilesConfigChecks(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name  string
		env   []string
		check string
	}{
		{"too many parts", []string{"BOTHY_FILE__MAX_UPLOAD_BYTES=10001", "BOTHY_FILE__UPLOAD_PART_BYTES=1"}, "upload_part_plan"},
		{"s3 part below the minimum", append(s3Env("https://s3.example.com"), "BOTHY_FILE__UPLOAD_PART_BYTES=5242879"), "upload_part_size_s3"},
		{"presigned mode", []string{"BOTHY_FILE__S3__PRESIGNED_UPLOAD=true"}, "presigned_mode_unavailable"},
	} {
		t.Run(c.name, func(t *testing.T) {
			got := load(t, c.env...)
			if len(got) != 1 || !strings.Contains(got[0].String(), ": "+c.check+": ") {
				t.Fatalf("problems %v, want exactly %s", got, c.check)
			}
			noValues(t, got, "5242879")
		})
	}
	for name, env := range map[string][]string{
		"exactly the part limit":   {"BOTHY_FILE__MAX_UPLOAD_BYTES=10000", "BOTHY_FILE__UPLOAD_PART_BYTES=1"},
		"small parts with fs":      {"BOTHY_FILE__MAX_UPLOAD_BYTES=1024000", "BOTHY_FILE__UPLOAD_PART_BYTES=1024"},
		"s3 with the minimum part": append(s3Env("https://s3.example.com"), "BOTHY_FILE__UPLOAD_PART_BYTES=5242880"),
	} {
		if got := load(t, env...); got != nil {
			t.Errorf("%s: problems %v, want none", name, got)
		}
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
	store, err := wire.FilesObjectStore(t.Context(), filesValues(t, "BOTHY_FILE__FS__ROOT="+root), wire.FilesDeps{})
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
	store, err := wire.FilesObjectStore(t.Context(), filesValues(t, "BOTHY_FILE__FS__ROOT="+missing), wire.FilesDeps{})
	if err == nil || !strings.Contains(err.Error(), "file.fs.root") || strings.Contains(err.Error(), missing) || store != nil {
		t.Errorf("FilesObjectStore with a missing root: %v, %v", store, err)
	}
}

// s3Env is the environment of a valid driver s3 configuration on endpoint; further entries may
// add egress.allowed_hosts.
func s3Env(endpoint string, extra ...string) []string {
	return append([]string{
		"BOTHY_FILE__STORAGE__DRIVER=s3",
		"BOTHY_FILE__S3__ENDPOINT=" + endpoint,
		"BOTHY_FILE__S3__BUCKET=bothy-files",
	}, extra...)
}

// TestFileS3Check holds the cross-check file_s3 (docs/DECISIONS.md M3-02/2 and M3-02/11): with
// the driver fs every file.s3.* value is ignored; with s3 each violation names its keys and never
// the value; valid https and http configurations load.
func TestFileS3Check(t *testing.T) {
	t.Parallel()
	const canary = "canary4z"
	if got := load(t, "BOTHY_FILE__S3__ENDPOINT=ftp://"+canary, "BOTHY_FILE__S3__BUCKET=No", "BOTHY_FILE__S3__PREFIX=/x"); got != nil {
		t.Errorf("driver fs with invalid file.s3.* values: problems %v, want none", got)
	}
	const keys = "config: file.storage.driver, file.s3.endpoint, file.s3.region, file.s3.bucket, file.s3.prefix, " +
		"file.s3.use_path_style, file.s3.checksum_mode, egress.allowed_hosts: file_s3: "
	cases := []struct {
		name string
		env  []string
		want string
	}{
		{"endpoint missing", []string{"BOTHY_FILE__STORAGE__DRIVER=s3", "BOTHY_FILE__S3__BUCKET=bothy-files"}, "file.s3.endpoint must be set"},
		{"endpoint with path", s3Env("https://s3.example.com/" + canary), "file.s3.endpoint must be an absolute http or https URL"},
		{"bucket missing", []string{"BOTHY_FILE__STORAGE__DRIVER=s3", "BOTHY_FILE__S3__ENDPOINT=https://s3.example.com"}, "file.s3.bucket must be set"},
		{"bucket invalid", []string{"BOTHY_FILE__STORAGE__DRIVER=s3", "BOTHY_FILE__S3__ENDPOINT=https://s3.example.com", "BOTHY_FILE__S3__BUCKET=Bad_" + canary}, "file.s3.bucket must be set"},
		{"region empty", append(s3Env("https://s3.example.com"), "BOTHY_FILE__S3__REGION="), "file.s3.region must be set"},
		{"prefix invalid", append(s3Env("https://s3.example.com"), "BOTHY_FILE__S3__PREFIX="+canary+"/"), "file.s3.prefix must be empty"},
		{"virtual hosted on ip", append(s3Env("https://192.0.2.1"), "BOTHY_FILE__S3__USE_PATH_STYLE=false"), "file.s3.use_path_style must be true"},
		{"http without entry", s3Env("http://" + canary + ".example.com:8333"), "an http file.s3.endpoint needs an egress.allowed_hosts entry"},
		{"when_supported over http", s3Env("http://seaweedfs:8333", "BOTHY_EGRESS__ALLOWED_HOSTS=seaweedfs:8333;allow_private", "BOTHY_FILE__S3__CHECKSUM_MODE=when_supported"), "file.s3.checksum_mode when_supported needs an https file.s3.endpoint"},
		{"http entry without allow_private", s3Env("http://"+canary+".example.com:8333", "BOTHY_EGRESS__ALLOWED_HOSTS="+canary+".example.com:8333"), "an http file.s3.endpoint needs an egress.allowed_hosts entry"},
	}
	for _, c := range cases {
		got := load(t, c.env...)
		if len(got) != 1 || !strings.HasPrefix(got[0].String(), keys+c.want) {
			t.Errorf("%s: problems %v, want one starting with\n%s%s", c.name, got, keys, c.want)
		}
		noValues(t, got, canary)
	}
	for name, env := range map[string][]string{
		"https":                     s3Env("https://s3.example.com"),
		"http with allow_private":   s3Env("http://seaweedfs:8333", "BOTHY_EGRESS__ALLOWED_HOSTS=seaweedfs:8333;allow_private"),
		"virtual hosted with entry": s3Env("http://s3.example.com:8333", "BOTHY_FILE__S3__USE_PATH_STYLE=false", "BOTHY_EGRESS__ALLOWED_HOSTS=bothy-files.s3.example.com:8333;allow_private"),
	} {
		if got := load(t, env...); got != nil {
			t.Errorf("%s: problems %v, want none", name, got)
		}
	}
}

// s3Fake is an httptest server on 127.0.0.1 that answers every request with 200 and records the
// Authorization headers.
type s3Fake struct {
	srv  *httptest.Server
	mu   sync.Mutex
	auth []string
}

func newS3Fake(t *testing.T) *s3Fake {
	t.Helper()
	f := &s3Fake{}
	f.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		f.mu.Lock()
		f.auth = append(f.auth, r.Header.Get("Authorization"))
		f.mu.Unlock()
		w.Header().Set("ETag", `"x"`)
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(f.srv.Close)
	return f
}

func (f *s3Fake) requests() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.auth...)
}

// s3Secrets writes the two key files of the default references into a fresh secrets.dir and
// returns the directory and the access key ID.
func s3Secrets(t *testing.T) (dir, access string) {
	t.Helper()
	dir = t.TempDir()
	access = "AKWIREEXAMPLE1"
	for name, value := range map[string]string{"s3_access_key": access, "s3_secret_key": "WIRESECRETEXAMPLE"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(value), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return dir, access
}

// TestWireBuildsS3Store holds the wiring of docs/DECISIONS.md M3-02/14: driver s3 builds the S3
// driver from the loaded values, the secret provider and the egress client; an http endpoint with
// an allow_private entry is reached with the access key ID of the secrets directory, and an https
// endpoint on loopback without an entry is refused before any request.
func TestWireBuildsS3Store(t *testing.T) {
	t.Parallel()
	fake := newS3Fake(t)
	dir, access := s3Secrets(t)
	host := strings.TrimPrefix(fake.srv.URL, "http://")
	open := func(endpoint string, extra ...string) filesapp.ObjectStore {
		t.Helper()
		v := filesValues(t, append(s3Env(endpoint, "BOTHY_SECRETS__DIR="+dir), extra...)...)
		sp, err := wire.NewSecretsProvider(v, nil, nil)
		if err != nil {
			t.Fatal(err)
		}
		store, err := wire.FilesObjectStore(t.Context(), v, wire.FilesDeps{Secrets: sp})
		if err != nil {
			t.Fatalf("FilesObjectStore: %v", err)
		}
		return store
	}
	key, err := domain.OriginalKey(ids.New(), 1)
	if err != nil {
		t.Fatal(err)
	}
	store := open(fake.srv.URL, "BOTHY_EGRESS__ALLOWED_HOSTS="+host+";allow_private")
	if err := store.Put(t.Context(), key, strings.NewReader("hello"), 5); err != nil {
		t.Fatalf("Put: %v", err)
	}
	reqs := fake.requests()
	if len(reqs) != 1 || !strings.Contains(reqs[0], "Credential="+access+"/") {
		t.Fatalf("requests %v, want one signed with %s", reqs, access)
	}
	blocked := open("https://" + host)
	err = blocked.Put(t.Context(), key, strings.NewReader("hello"), 5)
	if !errors.Is(err, egress.ErrBlocked) {
		t.Errorf("Put to https on loopback without entry = %v, want egress.ErrBlocked", err)
	}
	if n := len(fake.requests()); n != 1 {
		t.Errorf("the fake saw %d requests, want 1", n)
	}
}

// TestFilesObjectStoreS3Errors: a configuration or secret error of the driver s3 names the key,
// never a value.
func TestFilesObjectStoreS3Errors(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	v := filesValues(t, s3Env("https://s3.example.com", "BOTHY_SECRETS__DIR="+dir)...)
	sp, err := wire.NewSecretsProvider(v, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := wire.FilesObjectStore(t.Context(), v, wire.FilesDeps{Secrets: sp}); err == nil || !strings.Contains(err.Error(), "file.s3.access_key_ref") {
		t.Errorf("FilesObjectStore without key files = %v, want an error naming file.s3.access_key_ref", err)
	}
	if _, err := wire.FilesObjectStore(t.Context(), v, wire.FilesDeps{}); err == nil || !strings.Contains(err.Error(), "secret provider") {
		t.Errorf("FilesObjectStore without a secret provider = %v", err)
	}
}
