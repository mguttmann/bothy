// SPDX-License-Identifier: Apache-2.0

package s3_test

import (
	"bytes"
	"context"
	"crypto/x509"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/files/adapters/s3"
	"github.com/OWNER/bothy/services/server/internal/files/app"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/platform/egress"
	"github.com/OWNER/bothy/services/server/internal/platform/secrets"
)

// fakeEnv is a store on a fake S3 behind the real egress client and the real secret provider.
type fakeEnv struct {
	fake           *fakeS3
	sec            *testSecrets
	access, secret string
	cfg            s3.Config
	deps           s3.Deps
	rec            *s3.Recorder
	store          *s3.Store
}

// newFakeEnv starts a fake with answer and builds a store on it with an allow_private entry for
// the fake; mutate may change configuration and dependencies before New.
func newFakeEnv(t *testing.T, answer func(http.ResponseWriter, *http.Request) bool, mutate func(*s3.Config, *s3.Deps)) *fakeEnv {
	t.Helper()
	e := &fakeEnv{fake: newFake(t, answer), rec: &s3.Recorder{}}
	e.access, e.secret = testKeys(t)
	e.sec = newSecrets(t, e.access, e.secret)
	e.cfg = e.sec.config(e.fake.URL(), "bothy-test")
	e.deps = e.sec.deps(loopbackRule(t, e.fake.URL()))
	if mutate != nil {
		mutate(&e.cfg, &e.deps)
	}
	store, err := s3.NewRecorded(t.Context(), e.cfg, e.deps, e.rec)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	e.store = store
	return e
}

// isSentinel reports whether err is one of the port sentinels.
func isSentinel(err error) bool {
	for _, s := range []error{app.ErrInvalidKey, app.ErrNotFound, app.ErrInvalidRange, app.ErrSizeMismatch,
		app.ErrUploadNotFound, app.ErrInvalidPart, app.ErrNotSupported} {
		if errors.Is(err, s) {
			return true
		}
	}
	return false
}

// TestChecksBeforeAnyRequest holds the argument checks of the port (M3-01 note 3): each refusal
// comes before any request.
func TestChecksBeforeAnyRequest(t *testing.T) {
	t.Parallel()
	e := newFakeEnv(t, nil, nil)
	s, ctx, key := e.store, t.Context(), newKey(t)
	variant := strings.TrimSuffix(key, "orig-1") + "small"
	body := func() io.Reader { return strings.NewReader("x") }
	get := func(rng *app.ByteRange) error {
		rc, err := s.Get(ctx, key, rng)
		if rc != nil {
			_ = rc.Close()
		}
		return err
	}
	cases := []struct {
		name string
		err  error
		want error
	}{
		{"Put invalid key", s.Put(ctx, "A/b", body(), 1), app.ErrInvalidKey},
		{"Delete invalid key", s.Delete(ctx, "a//b"), app.ErrInvalidKey},
		{"Get negative offset", get(&app.ByteRange{Offset: -1, Length: 5}), app.ErrInvalidRange},
		{"Get length 0", get(&app.ByteRange{Offset: 0, Length: 0}), app.ErrInvalidRange},
		{"UploadPart n 0", func() error { _, err := s.UploadPart(ctx, key, "upload-1", 0, body(), 1); return err }(), app.ErrInvalidPart},
		{"PresignUploadPart n 0", func() error { _, err := s.PresignUploadPart(ctx, key, "upload-1", 0, 1, time.Minute); return err }(), app.ErrInvalidPart},
		{"PresignUploadPart size 0", func() error { _, err := s.PresignUploadPart(ctx, key, "upload-1", 1, 0, time.Minute); return err }(), app.ErrSizeMismatch},
		{"CompleteMultipart empty", s.CompleteMultipart(ctx, key, "upload-1", nil), app.ErrInvalidPart},
		{"CompleteMultipart descending", s.CompleteMultipart(ctx, key, "upload-1", []app.Part{{Number: 2, ETag: `"b"`}, {Number: 1, ETag: `"a"`}}), app.ErrInvalidPart},
		{"CreateMultipart variant key", func() error { _, err := s.CreateMultipart(ctx, variant); return err }(), app.ErrInvalidKey},
		{"Put negative size", s.Put(ctx, key, body(), -1), app.ErrSizeMismatch},
		{"Put empty with a body", s.Put(ctx, key, body(), 0), app.ErrSizeMismatch},
		{"UploadPart negative size", func() error { _, err := s.UploadPart(ctx, key, "upload-1", 1, body(), -1); return err }(), app.ErrSizeMismatch},
		{"UploadPart empty upload ID", func() error { _, err := s.UploadPart(ctx, key, "", 1, body(), 1); return err }(), app.ErrUploadNotFound},
	}
	for _, c := range cases {
		if !errors.Is(c.err, c.want) {
			t.Errorf("%s: err = %v, want %v", c.name, c.err, c.want)
		}
	}
	if err := s.AbortMultipart(ctx, key, ""); err != nil {
		t.Errorf("AbortMultipart without upload ID = %v, want nil", err)
	}
	if got := e.fake.Requests(); len(got) != 0 {
		t.Errorf("the fake saw %d requests, want 0: %+v", len(got), got)
	}
	if got := e.rec.Requests(); len(got) != 0 {
		t.Errorf("the recorder saw %d requests, want 0", len(got))
	}
}

// TestNoDefaultCredentialChain holds docs/DECISIONS.md M3-02/2: the SDK never reads the
// environment or shared files for credentials; only the secret provider supplies them.
func TestNoDefaultCredentialChain(t *testing.T) {
	t.Setenv("AWS_ACCESS_KEY_ID", "AKENVEXAMPLE")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "ENVSECRETEXAMPLE")
	t.Setenv("AWS_PROFILE", "example")
	e := newFakeEnv(t, nil, nil)
	if err := e.store.Put(t.Context(), newKey(t), strings.NewReader("hello"), 5); err != nil {
		t.Fatal(err)
	}
	reqs := e.fake.Requests()
	if len(reqs) != 1 {
		t.Fatalf("requests %d, want 1", len(reqs))
	}
	if got := accessKeyOf(reqs[0].Authorization); got != e.access {
		t.Errorf("signed with access key %q, want the one of the secret provider", got)
	}
}

// TestCredentialsFollowRotation holds docs/DECISIONS.md M3-02/6: a rotated key takes effect
// without restart once the secret provider reads it again after secrets.cache_ttl.
func TestCredentialsFollowRotation(t *testing.T) {
	t.Parallel()
	e := newFakeEnv(t, nil, nil)
	put := func() string {
		t.Helper()
		before := len(e.fake.Requests())
		if err := e.store.Put(t.Context(), newKey(t), strings.NewReader("x"), 1); err != nil {
			t.Fatal(err)
		}
		reqs := e.fake.Requests()
		if len(reqs) != before+1 {
			t.Fatalf("requests %d, want %d", len(reqs), before+1)
		}
		return accessKeyOf(reqs[len(reqs)-1].Authorization)
	}
	if got := put(); got != e.access {
		t.Fatalf("first request signed with %q, want %q", got, e.access)
	}
	access2, secret2 := testKeys(t)
	writeSecret(t, e.sec.dir, accessKeyFile, access2, 0o600)
	writeSecret(t, e.sec.dir, secretKeyFile, secret2, 0o600)
	if got := put(); got != e.access {
		t.Errorf("inside secrets.cache_ttl signed with %q, want the old key", got)
	}
	e.sec.clock.Advance(testCacheTTL + time.Second)
	if got := put(); got != access2 {
		t.Errorf("after secrets.cache_ttl signed with %q, want the rotated key", got)
	}
}

// TestStartFailsWithoutSecret: a missing key file stops the start with an error that names the key
// and no value (02 "14.4" row Secret-Provider).
func TestStartFailsWithoutSecret(t *testing.T) {
	t.Parallel()
	access, secret := testKeys(t)
	sec := newSecrets(t, access, secret)
	if err := os.Remove(filepath.Join(sec.dir, accessKeyFile)); err != nil {
		t.Fatal(err)
	}
	_, err := s3.New(t.Context(), sec.config("https://s3.example.com", "bothy-test"), sec.deps())
	if err == nil || !strings.Contains(err.Error(), "file.s3.access_key_ref") {
		t.Fatalf("New = %v, want an error naming file.s3.access_key_ref", err)
	}
	if !errors.Is(err, secrets.ErrNotFound) {
		t.Errorf("error %v does not wrap secrets.ErrNotFound", err)
	}
	if strings.Contains(err.Error(), secret) {
		t.Errorf("error %q carries the secret", err)
	}
	if err := os.Remove(filepath.Join(sec.dir, secretKeyFile)); err != nil {
		t.Fatal(err)
	}
	writeSecret(t, sec.dir, accessKeyFile, access, 0o600)
	if _, err := s3.New(t.Context(), sec.config("https://s3.example.com", "bothy-test"), sec.deps()); err == nil ||
		!strings.Contains(err.Error(), "file.s3.secret_key_ref") || strings.Contains(err.Error(), access) {
		t.Errorf("New without the secret key = %v", err)
	}
}

// TestStartFailsWithInsecureSecretFile is the S3 path of SEC-T11: a key file at mode 0644 stops
// the start with the insecure-file error of the provider.
func TestStartFailsWithInsecureSecretFile(t *testing.T) {
	t.Parallel()
	access, secret := testKeys(t)
	sec := newSecrets(t, access, secret)
	writeSecret(t, sec.dir, secretKeyFile, secret, 0o644)
	_, err := s3.New(t.Context(), sec.config("https://s3.example.com", "bothy-test"), sec.deps())
	if !errors.Is(err, secrets.ErrInsecureFile) || !strings.Contains(err.Error(), "file.s3.secret_key_ref") {
		t.Fatalf("New = %v, want the insecure-file error for file.s3.secret_key_ref", err)
	}
	if strings.Contains(err.Error(), secret) {
		t.Errorf("error %q carries the secret", err)
	}
}

// TestNewValidatesConfig: New runs Config.Validate, so a direct caller cannot skip the rules.
func TestNewValidatesConfig(t *testing.T) {
	t.Parallel()
	access, secret := testKeys(t)
	sec := newSecrets(t, access, secret)
	cfg := sec.config("http://s3.example.com:8333", "bothy-test")
	if _, err := s3.New(t.Context(), cfg, sec.deps()); err == nil || !strings.Contains(err.Error(), "egress.allowed_hosts") {
		t.Errorf("New with an http endpoint without entry = %v", err)
	}
	cfg = sec.config("https://s3.example.com", "")
	if _, err := s3.New(t.Context(), cfg, sec.deps()); err == nil || !strings.Contains(err.Error(), "file.s3.bucket") {
		t.Errorf("New without bucket = %v", err)
	}
}

// TestNoRetries holds docs/DECISIONS.md M3-02/3: a 503 SlowDown reaches the fake exactly once and
// is a storage failure (02 "14.3").
func TestNoRetries(t *testing.T) {
	t.Parallel()
	e := newFakeEnv(t, func(w http.ResponseWriter, _ *http.Request) bool {
		s3Error(w, http.StatusServiceUnavailable, "SlowDown")
		return true
	}, nil)
	err := e.store.Put(t.Context(), newKey(t), strings.NewReader("hello"), 5)
	if err == nil || isSentinel(err) {
		t.Fatalf("Put = %v, want a storage failure", err)
	}
	if n := len(e.fake.Requests()); n != 1 {
		t.Errorf("the fake saw %d requests, want 1", n)
	}
	_, err = e.store.Head(t.Context(), newKey(t))
	if err == nil || isSentinel(err) {
		t.Errorf("Head = %v, want a storage failure", err)
	}
	if n := len(e.fake.Requests()); n != 2 {
		t.Errorf("the fake saw %d requests, want 2", n)
	}
}

// TestRedirectNotFollowed: a 307 ends with one request and a storage failure, even when it points
// to the endpoint itself, which the host policy would allow.
func TestRedirectNotFollowed(t *testing.T) {
	t.Parallel()
	e := newFakeEnv(t, func(w http.ResponseWriter, r *http.Request) bool {
		if r.URL.Path == "/elsewhere" {
			w.WriteHeader(http.StatusOK)
			return true
		}
		w.Header().Set("Location", "/elsewhere")
		w.WriteHeader(http.StatusTemporaryRedirect)
		return true
	}, nil)
	rc, err := e.store.Get(t.Context(), newKey(t), nil)
	if rc != nil {
		_ = rc.Close()
	}
	if err == nil || isSentinel(err) {
		t.Fatalf("Get = %v, want a storage failure", err)
	}
	if n := len(e.fake.Requests()); n != 1 {
		t.Errorf("the fake saw %d requests, want 1", n)
	}
}

// countingMetrics counts refused attempts.
type countingMetrics struct {
	mu      sync.Mutex
	reasons []egress.Reason
}

func (m *countingMetrics) Blocked(_ context.Context, r egress.Reason) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.reasons = append(m.reasons, r)
}

// TestEgressBlocksUnlistedEndpoint is SEC-T09 for the driver: an https endpoint on loopback without
// an allow_private entry is refused by the egress client, counted once, and never reached.
func TestEgressBlocksUnlistedEndpoint(t *testing.T) {
	t.Parallel()
	metrics := &countingMetrics{}
	e := newFakeEnv(t, nil, func(c *s3.Config, d *s3.Deps) {
		c.Endpoint = strings.Replace(c.Endpoint, "http://", "https://", 1)
		d.Egress.AllowedHosts = nil
		d.EgressDeps.Metrics = metrics
	})
	err := e.store.Put(t.Context(), newKey(t), strings.NewReader("hello"), 5)
	if !errors.Is(err, egress.ErrBlocked) || isSentinel(err) {
		t.Fatalf("Put = %v, want egress.ErrBlocked as a storage failure", err)
	}
	if n := e.fake.conns.Load(); n != 0 {
		t.Errorf("the fake saw %d connections, want 0", n)
	}
	metrics.mu.Lock()
	defer metrics.mu.Unlock()
	if len(metrics.reasons) != 1 {
		t.Errorf("bothy.egress.blocked counted %v, want once", metrics.reasons)
	}
}

// TestOnlyGivenHTTPClient is SEC-NET-1 for the driver: after every port operation the fake saw
// exactly the requests the recorder around the egress client saw, so the SDK opens no
// connection of its own.
func TestOnlyGivenHTTPClient(t *testing.T) {
	t.Parallel()
	e := newFakeEnv(t, nil, nil)
	s, ctx, key := e.store, t.Context(), newKey(t)
	steps := []func() error{
		func() error { return s.Put(ctx, key, strings.NewReader("hello"), 5) },
		func() error {
			rc, err := s.Get(ctx, key, &app.ByteRange{Offset: 1, Length: 2})
			if err == nil {
				_, _ = io.Copy(io.Discard, rc)
				_ = rc.Close()
			}
			return err
		},
		func() error { _, err := s.Head(ctx, key); return err },
		func() error { return s.Delete(ctx, key) },
		func() error { _, err := s.CreateMultipart(ctx, key); return err },
		func() error { _, err := s.UploadPart(ctx, key, "upload-1", 1, strings.NewReader("abc"), 3); return err },
		func() error {
			return s.CompleteMultipart(ctx, key, "upload-1", []app.Part{{Number: 1, ETag: `"part-1"`}})
		},
		func() error { return s.AbortMultipart(ctx, key, "upload-1") },
		func() error { _, err := s.PresignUploadPart(ctx, key, "upload-1", 1, 3, time.Minute); return err },
	}
	for i, step := range steps {
		if err := step(); err != nil {
			t.Fatalf("step %d: %v", i, err)
		}
		rec, got := e.rec.Requests(), e.fake.Requests()
		if len(rec) != len(got) {
			t.Fatalf("after step %d the recorder saw %d requests and the fake %d", i, len(rec), len(got))
		}
		for j := range rec {
			if rec[j].Method != got[j].Method || rec[j].Path != got[j].Path || rec[j].RawQuery != got[j].RawQuery {
				t.Errorf("request %d: recorded %s %s?%s, fake saw %s %s?%s", j, rec[j].Method, rec[j].Path, rec[j].RawQuery, got[j].Method, got[j].Path, got[j].RawQuery)
			}
		}
	}
	if n := len(e.fake.Requests()); n != 8 {
		t.Errorf("the fake saw %d requests, want 8 (presigning sends nothing)", n)
	}
}

// forbiddenInErrors returns the strings no error text may contain for env.
func forbiddenInErrors(e *fakeEnv) []string {
	u, _ := url.Parse(e.cfg.Endpoint)
	return []string{"http://", "https://", u.Host, "X-Amz-", "x-amz-", e.access, e.secret}
}

// checkClean fails when the text of err or its log rendering through kernel/errs carries a
// forbidden string, or does not name the operation and the key.
func checkClean(t *testing.T, what string, err error, op, key string, forbidden []string) {
	t.Helper()
	if err == nil {
		t.Errorf("%s: no error", what)
		return
	}
	text := err.Error()
	if !strings.Contains(text, op) || (key != "" && !strings.Contains(text, key)) {
		t.Errorf("%s: error %q does not name %q and %q", what, text, op, key)
	}
	var buf bytes.Buffer
	slog.New(slog.NewJSONHandler(&buf, nil)).Error("storage failed", slog.Any("error", errs.Wrap(errs.CodeUnavailable, err, "object storage unavailable")))
	for _, f := range forbidden {
		if f == "" {
			continue
		}
		if strings.Contains(text, f) {
			t.Errorf("%s: error %q contains %q", what, text, f)
		}
		if strings.Contains(buf.String(), f) {
			t.Errorf("%s: log record %s contains %q", what, buf.String(), f)
		}
	}
}

// TestErrorsCarryNoURLOrSecret holds docs/DECISIONS.md M3-02/5 (SEC-LOG-3, SEC-SEC-3).
func TestErrorsCarryNoURLOrSecret(t *testing.T) {
	t.Parallel()
	t.Run("unknown key", func(t *testing.T) {
		t.Parallel()
		e := newFakeEnv(t, nil, nil)
		key := newKey(t)
		rc, err := e.store.Get(t.Context(), key, nil)
		if rc != nil {
			_ = rc.Close()
		}
		if !errors.Is(err, app.ErrNotFound) {
			t.Errorf("Get = %v, want ErrNotFound", err)
		}
		checkClean(t, "unknown key", err, "get", key, forbiddenInErrors(e))
	})
	t.Run("wrong credentials", func(t *testing.T) {
		t.Parallel()
		e := newFakeEnv(t, func(w http.ResponseWriter, _ *http.Request) bool {
			s3Error(w, http.StatusForbidden, "SignatureDoesNotMatch")
			return true
		}, nil)
		key := newKey(t)
		err := e.store.Put(t.Context(), key, strings.NewReader("hello"), 5)
		if err == nil || isSentinel(err) || !strings.Contains(err.Error(), "SignatureDoesNotMatch") {
			t.Errorf("Put = %v, want a storage failure with the S3 code", err)
		}
		checkClean(t, "wrong credentials", err, "put", key, forbiddenInErrors(e))
	})
	t.Run("closed port", func(t *testing.T) {
		t.Parallel()
		ln, err := (&net.ListenConfig{}).Listen(t.Context(), "tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		endpoint := "http://" + ln.Addr().String()
		_ = ln.Close()
		e := newFakeEnv(t, nil, func(c *s3.Config, d *s3.Deps) {
			c.Endpoint = endpoint
			d.Egress.AllowedHosts = []egress.Rule{loopbackRule(t, endpoint)}
		})
		key := newKey(t)
		_, err = e.store.Head(t.Context(), key)
		if err == nil || isSentinel(err) {
			t.Errorf("Head = %v, want a storage failure", err)
		}
		checkClean(t, "closed port", err, "head", key, forbiddenInErrors(e))
	})
	t.Run("oversized body", func(t *testing.T) {
		t.Parallel()
		e := newFakeEnv(t, nil, func(_ *s3.Config, d *s3.Deps) { d.MaxObjectBytes = 8 })
		key := newKey(t)
		if err := e.store.Put(t.Context(), key, strings.NewReader("0123456789"), 10); err != nil {
			t.Fatal(err)
		}
		rc, err := e.store.Get(t.Context(), key, nil)
		if err != nil {
			t.Fatal(err)
		}
		_, err = io.ReadAll(rc)
		_ = rc.Close()
		if !errors.Is(err, egress.ErrResponseTooLarge) {
			t.Errorf("read = %v, want ErrResponseTooLarge", err)
		}
		checkClean(t, "oversized body", err, "get", key, forbiddenInErrors(e))
	})
	t.Run("failed presign", func(t *testing.T) {
		t.Parallel()
		e := newFakeEnv(t, nil, nil)
		key := newKey(t)
		_, err := e.store.PresignUploadPart(t.Context(), key, "upload-1", 1, 5, 0)
		checkClean(t, "presign with ttl 0", err, "presign upload part", key, forbiddenInErrors(e))
	})
}

// TestResponseLimitIsMaxObjectBytes holds docs/DECISIONS.md M3-02/1: a Get body of exactly
// MaxObjectBytes reads completely; one byte more fails with ErrResponseTooLarge, never a short
// read without error.
func TestResponseLimitIsMaxObjectBytes(t *testing.T) {
	t.Parallel()
	const limit = 64
	e := newFakeEnv(t, nil, func(_ *s3.Config, d *s3.Deps) { d.MaxObjectBytes = limit })
	exact, over := newKey(t), newKey(t)
	data := bytes.Repeat([]byte("a"), limit+1)
	if err := e.store.Put(t.Context(), exact, bytes.NewReader(data[:limit]), limit); err != nil {
		t.Fatal(err)
	}
	if err := e.store.Put(t.Context(), over, bytes.NewReader(data), limit+1); err != nil {
		t.Fatal(err)
	}
	read := func(key string) ([]byte, error) {
		rc, err := e.store.Get(t.Context(), key, nil)
		if err != nil {
			return nil, err
		}
		defer func() { _ = rc.Close() }()
		return io.ReadAll(rc)
	}
	if got, err := read(exact); err != nil || len(got) != limit {
		t.Errorf("Get of exactly the limit = %d bytes, %v", len(got), err)
	}
	if got, err := read(over); !errors.Is(err, egress.ErrResponseTooLarge) {
		t.Errorf("Get of one byte more = %d bytes, %v; want ErrResponseTooLarge", len(got), err)
	}
}

// TestPresignUploadPartSignsLength holds docs/DECISIONS.md M3-02/9: the signature binds the part
// length and the validity, and the URL carries no secret.
func TestPresignUploadPartSignsLength(t *testing.T) {
	t.Parallel()
	e := newFakeEnv(t, nil, nil)
	key := newKey(t)
	before := time.Now()
	p, err := e.store.PresignUploadPart(t.Context(), key, "upload-1", 2, 5<<20, 15*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	u, err := url.Parse(p.URL)
	if err != nil {
		t.Fatal(err)
	}
	q := u.Query()
	signed := strings.Split(q.Get("X-Amz-SignedHeaders"), ";")
	if !containsAll(signed, "content-length", "host") {
		t.Errorf("X-Amz-SignedHeaders = %v, want content-length and host", signed)
	}
	if got := q.Get("X-Amz-Expires"); got != strconv.Itoa(15*60) {
		t.Errorf("X-Amz-Expires = %q, want %d", got, 15*60)
	}
	if q.Get("partNumber") != "2" || q.Get("uploadId") != "upload-1" {
		t.Errorf("query %v does not address part 2 of upload-1", q)
	}
	if strings.Contains(p.URL, e.secret) {
		t.Error("the URL carries the secret key")
	}
	if p.ExpiresAt.Before(before.Add(15*time.Minute)) || p.ExpiresAt.After(time.Now().Add(15*time.Minute)) {
		t.Errorf("ExpiresAt %v is not the signing time plus ttl", p.ExpiresAt)
	}
	if _, err := e.store.PresignUploadPart(t.Context(), key, "upload-1", 1, 5, 0); err == nil {
		t.Error("ttl 0 was signed")
	}
	if _, err := e.store.PresignUploadPart(t.Context(), key, "upload-1", 1, 5, -time.Second); err == nil {
		t.Error("a negative ttl was signed")
	}
	if n := len(e.fake.Requests()); n != 0 {
		t.Errorf("presigning sent %d requests", n)
	}
}

func containsAll(have []string, want ...string) bool {
	for _, w := range want {
		found := false
		for _, h := range have {
			found = found || h == w
		}
		if !found {
			return false
		}
	}
	return true
}

// TestS3SDKOnlyInAdapter holds 12 "15" and ADR 0048 "Confirmation": no package of the module but
// internal/files/adapters/s3, the checkpoint sink internal/audit/adapters/s3sink (M7-22) and the
// object mirror of the backup service internal/status/adapters/s3mirror (M7-02) imports a path
// below github.com/aws/, in production code, tests or external tests. The image test of
// bothy-postgres builds only with the tag imagetest and is held by depguard alone.
func TestS3SDKOnlyInAdapter(t *testing.T) {
	t.Parallel()
	out, err := exec.CommandContext(t.Context(), "go", "list", "-test=false",
		"-f", `{{.ImportPath}} {{join .Imports " "}} {{join .TestImports " "}} {{join .XTestImports " "}}`,
		"github.com/OWNER/bothy/services/server/...").Output()
	if err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			t.Fatalf("go list: %v\n%s", err, ee.Stderr)
		}
		t.Fatalf("go list: %v", err)
	}
	const adapter = "github.com/OWNER/bothy/services/server/internal/files/adapters/s3"
	const sink = "github.com/OWNER/bothy/services/server/internal/audit/adapters/s3sink"
	const mirror = "github.com/OWNER/bothy/services/server/internal/status/adapters/s3mirror"
	seen := false
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		pkg := fields[0]
		seen = seen || pkg == adapter
		for _, imp := range fields[1:] {
			if strings.HasPrefix(imp, "github.com/aws/") && pkg != adapter && pkg != sink && pkg != mirror {
				t.Errorf("%s imports %s", pkg, imp)
			}
		}
	}
	if !seen {
		t.Fatal("go list does not list the adapter; the check would prove nothing")
	}
}

// TestChecksumWhenSupportedOverHTTPS: with file.s3.checksum_mode = when_supported the SDK sends a
// trailing checksum, which it allows over TLS only (an http endpoint is refused at load); the
// streamed body still passes the length check (docs/DECISIONS.md M3-02/10).
func TestChecksumWhenSupportedOverHTTPS(t *testing.T) {
	t.Parallel()
	fake := newTLSFake(t)
	access, secret := testKeys(t)
	sec := newSecrets(t, access, secret)
	cfg := sec.config(fake.URL(), "bothy-test")
	cfg.ChecksumMode = "when_supported"
	deps := sec.deps(loopbackRule(t, fake.URL()))
	pool := x509.NewCertPool()
	pool.AddCert(fake.srv.Certificate())
	deps.EgressDeps.RootCAs = pool
	store, err := s3.New(t.Context(), cfg, deps)
	if err != nil {
		t.Fatal(err)
	}
	data := bytes.Repeat([]byte("c"), 3000)
	if err := store.Put(t.Context(), newKey(t), bytes.NewReader(data), int64(len(data))); err != nil {
		t.Fatalf("Put: %v", err)
	}
	if _, err := store.UploadPart(t.Context(), newKey(t), "upload-1", 1, bytes.NewReader(data), int64(len(data))); err != nil {
		t.Fatalf("UploadPart: %v", err)
	}
	if err := store.Put(t.Context(), newKey(t), bytes.NewReader(data[:10]), 11); !errors.Is(err, app.ErrSizeMismatch) {
		t.Errorf("short Put = %v, want ErrSizeMismatch", err)
	}
	fake.mu.Lock()
	defer fake.mu.Unlock()
	if len(fake.headers) < 2 || fake.headers[0].Get("X-Amz-Trailer") == "" {
		t.Errorf("no trailing checksum with when_supported: %v", fake.headers)
	}
}
