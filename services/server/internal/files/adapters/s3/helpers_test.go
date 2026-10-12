// SPDX-License-Identifier: Apache-2.0

package s3_test

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/xml"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/files/adapters/s3"
	"github.com/OWNER/bothy/services/server/internal/files/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/egress"
	"github.com/OWNER/bothy/services/server/internal/platform/secrets"
)

// The names of the key files in a secrets directory, as in the secret catalog of the Compose
// stack (docs/operations/configuration.md "5. Geheimnisse").
const (
	accessKeyFile = "s3_access_key"
	secretKeyFile = "s3_secret_key" // #nosec G101 -- a file name of the secret catalog, not a credential
)

// testRegion is the default of file.s3.region.
const testRegion = "us-east-1"

// testCacheTTL is secrets.cache_ttl of the test providers.
const testCacheTTL = time.Minute

// randomHex returns n random bytes as hex.
func randomHex(t testing.TB, n int) string {
	t.Helper()
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		t.Fatal(err)
	}
	return hex.EncodeToString(b)
}

// testKeys returns a generated access key ID and secret key, both containing EXAMPLE.
func testKeys(t testing.TB) (access, secret string) {
	t.Helper()
	return "AKEXAMPLE" + strings.ToUpper(randomHex(t, 6)), "EXAMPLE" + randomHex(t, 16)
}

// testSecrets is a secrets directory with the two key files and the provider that reads it.
type testSecrets struct {
	dir      string
	clock    *clock.Fake
	provider *secrets.Provider
}

// newSecrets writes access and secret into a fresh directory at mode 0600 and builds a provider
// on a fake clock with testCacheTTL.
func newSecrets(t testing.TB, access, secret string) *testSecrets {
	t.Helper()
	dir := t.TempDir()
	writeSecret(t, dir, accessKeyFile, access, 0o600)
	writeSecret(t, dir, secretKeyFile, secret, 0o600)
	clk := clock.NewFake(time.Date(2026, 10, 12, 9, 0, 0, 0, time.UTC))
	p, err := secrets.NewProvider(secrets.Options{Dir: dir, CacheTTL: testCacheTTL, Clock: clk})
	if err != nil {
		t.Fatal(err)
	}
	return &testSecrets{dir: dir, clock: clk, provider: p}
}

// writeSecret writes one secret file with mode.
func writeSecret(t testing.TB, dir, name, value string, mode os.FileMode) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(value+"\n"), mode); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, mode); err != nil {
		t.Fatal(err)
	}
}

// config returns a store configuration for endpoint with the references into the directory.
func (s *testSecrets) config(endpoint, bucket string) s3.Config {
	return s3.Config{
		Endpoint:     endpoint,
		Region:       testRegion,
		Bucket:       bucket,
		UsePathStyle: true,
		ChecksumMode: "when_required",
		AccessKeyRef: "file:<secrets.dir>/" + accessKeyFile,
		SecretKeyRef: "file:<secrets.dir>/" + secretKeyFile,
	}
}

// loopbackRule is the egress.allowed_hosts entry 127.0.0.1:<port>;allow_private of endpoint.
func loopbackRule(t testing.TB, endpoint string) egress.Rule {
	t.Helper()
	u, err := url.Parse(endpoint)
	if err != nil {
		t.Fatal(err)
	}
	r, err := egress.ParseRule(u.Host + ";allow_private")
	if err != nil {
		t.Fatal(err)
	}
	return r
}

// deps returns the dependencies of a store with the given egress entries.
func (s *testSecrets) deps(rules ...egress.Rule) s3.Deps {
	return s3.Deps{
		Egress:          egress.Config{AllowedHosts: rules, Timeout: 30 * time.Second, MaxResponseBytes: 1 << 20},
		Secrets:         s.provider,
		TransferTimeout: 30 * time.Second,
		MaxObjectBytes:  64 << 20,
	}
}

// newKey returns a fresh original key.
func newKey(t testing.TB) string {
	t.Helper()
	k, err := domain.OriginalKey(ids.New(), 1)
	if err != nil {
		t.Fatal(err)
	}
	return k
}

// fakeRequest is one request that reached the fake S3.
type fakeRequest struct {
	Method, Path, RawQuery, Authorization string
	ContentSHA256                         string
}

// fakeS3 is an httptest server on 127.0.0.1:0 that answers the core S3 requests of the driver
// from memory. answer, when set, may take over a request and returns true when it did.
type fakeS3 struct {
	srv     *httptest.Server
	mu      sync.Mutex
	objects map[string][]byte
	reqs    []fakeRequest
	headers []http.Header
	conns   atomic.Int64
	answer  func(w http.ResponseWriter, r *http.Request) bool
}

// newFake starts a fake S3 over http; answer may be nil.
func newFake(t testing.TB, answer func(w http.ResponseWriter, r *http.Request) bool) *fakeS3 {
	t.Helper()
	return startFake(t, answer, false)
}

// newTLSFake starts a fake S3 over https with the test certificate of httptest.
func newTLSFake(t testing.TB) *fakeS3 {
	t.Helper()
	return startFake(t, nil, true)
}

func startFake(t testing.TB, answer func(w http.ResponseWriter, r *http.Request) bool, tls bool) *fakeS3 {
	t.Helper()
	f := &fakeS3{objects: map[string][]byte{}, answer: answer}
	f.srv = httptest.NewUnstartedServer(http.HandlerFunc(f.serve))
	f.srv.Config.ConnState = func(_ net.Conn, st http.ConnState) {
		if st == http.StateNew {
			f.conns.Add(1)
		}
	}
	if tls {
		f.srv.StartTLS()
	} else {
		f.srv.Start()
	}
	t.Cleanup(f.srv.Close)
	return f
}

// URL is the endpoint of the fake.
func (f *fakeS3) URL() string { return f.srv.URL }

// Requests returns the requests that reached the fake.
func (f *fakeS3) Requests() []fakeRequest {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]fakeRequest(nil), f.reqs...)
}

func (f *fakeS3) serve(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	f.reqs = append(f.reqs, fakeRequest{
		Method: r.Method, Path: r.URL.EscapedPath(), RawQuery: r.URL.RawQuery,
		Authorization: r.Header.Get("Authorization"), ContentSHA256: r.Header.Get("X-Amz-Content-Sha256"),
	})
	f.headers = append(f.headers, r.Header.Clone())
	f.mu.Unlock()
	if f.answer != nil && f.answer(w, r) {
		return
	}
	q := r.URL.Query()
	switch {
	case r.Method == http.MethodPost && q.Has("uploads"):
		writeXML(w, http.StatusOK, struct {
			XMLName  xml.Name `xml:"InitiateMultipartUploadResult"`
			UploadID string   `xml:"UploadId"`
		}{UploadID: "upload-1"})
	case r.Method == http.MethodPut && q.Has("uploadId"):
		_, _ = io.Copy(io.Discard, r.Body)
		w.Header().Set("ETag", `"part-`+q.Get("partNumber")+`"`)
		w.WriteHeader(http.StatusOK)
	case r.Method == http.MethodPost && q.Has("uploadId"):
		writeXML(w, http.StatusOK, struct {
			XMLName xml.Name `xml:"CompleteMultipartUploadResult"`
			ETag    string   `xml:"ETag"`
		}{ETag: `"done"`})
	case r.Method == http.MethodDelete && q.Has("uploadId"):
		w.WriteHeader(http.StatusNoContent)
	case r.Method == http.MethodPut:
		data, err := io.ReadAll(r.Body)
		if err != nil {
			return
		}
		f.mu.Lock()
		f.objects[r.URL.Path] = data
		f.mu.Unlock()
		w.Header().Set("ETag", `"object"`)
		w.WriteHeader(http.StatusOK)
	case r.Method == http.MethodGet || r.Method == http.MethodHead:
		f.mu.Lock()
		data, ok := f.objects[r.URL.Path]
		f.mu.Unlock()
		if !ok {
			if r.Method == http.MethodHead {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			s3Error(w, http.StatusNotFound, "NoSuchKey")
			return
		}
		w.Header().Set("Content-Length", strconv.Itoa(len(data)))
		w.WriteHeader(http.StatusOK)
		if r.Method == http.MethodGet {
			_, _ = w.Write(data)
		}
	case r.Method == http.MethodDelete:
		f.mu.Lock()
		delete(f.objects, r.URL.Path)
		f.mu.Unlock()
		w.WriteHeader(http.StatusNoContent)
	default:
		s3Error(w, http.StatusNotImplemented, "NotImplemented")
	}
}

// s3Error writes an S3 error document.
func s3Error(w http.ResponseWriter, status int, code string) {
	writeXML(w, status, struct {
		XMLName xml.Name `xml:"Error"`
		Code    string   `xml:"Code"`
		Message string   `xml:"Message"`
	}{Code: code, Message: fmt.Sprintf("fake %s", code)})
}

func writeXML(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/xml")
	w.WriteHeader(status)
	_ = xml.NewEncoder(w).Encode(v)
}

// accessKeyOf returns the access key ID of a SigV4 Authorization header.
func accessKeyOf(auth string) string {
	_, rest, ok := strings.Cut(auth, "Credential=")
	if !ok {
		return ""
	}
	id, _, _ := strings.Cut(rest, "/")
	return id
}
