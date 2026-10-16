// SPDX-License-Identifier: Apache-2.0

package s3sink_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"

	"github.com/OWNER/bothy/services/server/internal/audit/adapters/s3sink"
	"github.com/OWNER/bothy/services/server/internal/audit/app"
	"github.com/OWNER/bothy/services/server/internal/audit/app/sinktest"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/egress"
	"github.com/OWNER/bothy/services/server/internal/platform/secrets"
)

// Compile-time check: the sink is the outgoing port of app.
var _ app.CheckpointSink = (*s3sink.Sink)(nil)

// The names of the key files, as in the secret catalog (docs/operations/configuration.md 5).
const (
	accessKeyFile = "audit_checkpoint_s3_access_key"
	secretKeyFile = "audit_checkpoint_s3_secret_key" // #nosec G101 -- a file name of the secret catalog, not a credential
)

// testPageSize is the page size of List in these tests, below sinktest.ManyCheckpoints.
const testPageSize = 10

// testSink is a sink over one bucket of the shared SeaweedFS and the instance it files under.
type testSink struct {
	*s3sink.Sink
	srv      *seaweedServer
	bucket   string
	prefix   string
	instance ids.ID
}

// sinkConfig returns the configuration of a sink on bucket of srv with the references into a
// fresh secrets directory holding the keys of srv, and the provider that reads it.
func sinkConfig(t *testing.T, srv *seaweedServer, bucket, prefix string) (s3sink.Config, *secrets.Provider) {
	t.Helper()
	dir := t.TempDir()
	for name, value := range map[string]string{accessKeyFile: srv.access, secretKeyFile: srv.secret} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(value+"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	p, err := secrets.NewProvider(secrets.Options{Dir: dir, CacheTTL: time.Minute})
	if err != nil {
		t.Fatal(err)
	}
	return s3sink.Config{
		Bucket: bucket, Prefix: prefix, Endpoint: srv.endpoint, Region: testRegion, UsePathStyle: true,
		AccessKeyRef: "file:<secrets.dir>/" + accessKeyFile, SecretKeyRef: "file:<secrets.dir>/" + secretKeyFile,
	}, p
}

// egressFor allows plain http to the loopback endpoint of srv, as egress.allowed_hosts does
// with "127.0.0.1:<port>;allow_private".
func egressFor(t *testing.T, endpoint string) egress.Config {
	t.Helper()
	u, err := url.Parse(endpoint)
	if err != nil {
		t.Fatal(err)
	}
	r, err := egress.ParseRule(u.Host + ";allow_private")
	if err != nil {
		t.Fatal(err)
	}
	return egress.Config{AllowedHosts: []egress.Rule{r}, Timeout: 30 * time.Second, MaxResponseBytes: 1 << 20}
}

// newTestSink builds a sink on a fresh bucket with the lock setup.
func newTestSink(t *testing.T, lock bucketLock, prefix string) *testSink {
	t.Helper()
	srv := sharedSeaweed(t)
	bucket := srv.newBucket(t, lock)
	cfg, p := sinkConfig(t, srv, bucket, prefix)
	inst := ids.New()
	s, err := s3sink.New(t.Context(), cfg, inst, s3sink.Deps{Egress: egressFor(t, srv.endpoint), Secrets: p})
	if err != nil {
		t.Fatal(err)
	}
	s3sink.SetPageSize(s, testPageSize)
	return &testSink{Sink: s, srv: srv, bucket: bucket, prefix: prefix, instance: inst}
}

// TestConformance runs the conformance suite of app.CheckpointSink against the sink on SeaweedFS
// with a Compliance lock, with and without a prefix (M7-22, Design note 3).
func TestConformance(t *testing.T) {
	t.Parallel()
	t.Run("no prefix", func(t *testing.T) {
		t.Parallel()
		sinktest.Run(t, func(t *testing.T) app.CheckpointSink { return newTestSink(t, compliance, "").Sink })
	})
	t.Run("prefix", func(t *testing.T) {
		t.Parallel()
		sinktest.Run(t, func(t *testing.T) app.CheckpointSink { return newTestSink(t, compliance, "env/prod").Sink })
	})
}

// seqKey returns the object key of the checkpoint seq in s.
func (s *testSink) seqKey(seq int64) string {
	return s.key(fmt.Sprintf("%020d", seq))
}

// key returns the object key of the name stem in s.
func (s *testSink) key(seq string) string {
	k := s.instance.String() + "/" + seq + ".jws"
	if s.prefix != "" {
		k = s.prefix + "/" + k
	}
	return k
}

// TestObjectKeys: a checkpoint is the object [<prefix>/]<instance_id>/<seq with 20 digits>.jws
// holding the compact JWS (docs/DECISIONS.md M7-22/3).
func TestObjectKeys(t *testing.T) {
	t.Parallel()
	s := newTestSink(t, compliance, "prod")
	if err := s.Put(t.Context(), app.SignedCheckpoint{Seq: 42, JWS: []byte("aaa.bbb.ccc")}); err != nil {
		t.Fatal(err)
	}
	out, err := s.srv.admin.GetObject(t.Context(), &awss3.GetObjectInput{Bucket: aws.String(s.bucket), Key: aws.String(s.seqKey(42))})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = out.Body.Close() }()
	body, _ := io.ReadAll(out.Body)
	if string(body) != "aaa.bbb.ccc" {
		t.Fatalf("object body %q", body)
	}
	if out.ObjectLockMode != types.ObjectLockModeCompliance {
		t.Errorf("object lock mode %q, want COMPLIANCE from the default retention", out.ObjectLockMode)
	}
}

// TestOverwriteKeepsFirstVersion: another writer that overwrites a checkpoint object (a service
// that ignores If-None-Match, or a client outside the server) only adds a version on a locked
// bucket; List reads the oldest version of every key, and Put of other bytes under that seq
// fails while the same bytes pass (Design note 2).
func TestOverwriteKeepsFirstVersion(t *testing.T) {
	t.Parallel()
	s := newTestSink(t, compliance, "")
	first := app.SignedCheckpoint{Seq: 3, JWS: []byte("first.jws.a")}
	if err := s.Put(t.Context(), first); err != nil {
		t.Fatal(err)
	}
	if _, err := s.srv.admin.PutObject(t.Context(), &awss3.PutObjectInput{
		Bucket: aws.String(s.bucket), Key: aws.String(s.seqKey(3)), Body: strings.NewReader("forged.jws.b"),
	}); err != nil {
		t.Fatalf("the overwrite of the test: %v", err)
	}
	list, err := s.List(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].Seq != 3 || !bytes.Equal(list[0].JWS, first.JWS) {
		t.Fatalf("List = %+v, want the first version", list)
	}
	if err := s.Put(t.Context(), first); err != nil {
		t.Errorf("the same bytes again: %v", err)
	}
	if err := s.Put(t.Context(), app.SignedCheckpoint{Seq: 3, JWS: []byte("forged.jws.b")}); !errors.Is(err, app.ErrCheckpointConflict) {
		t.Errorf("the overwritten bytes: %v, want ErrCheckpointConflict", err)
	}
	// A delete marker hides the key from a plain GET, never from the sink.
	if _, err := s.srv.admin.DeleteObject(t.Context(), &awss3.DeleteObjectInput{
		Bucket: aws.String(s.bucket), Key: aws.String(s.seqKey(3)),
	}); err != nil {
		t.Fatalf("delete marker: %v", err)
	}
	if list, err := s.List(t.Context()); err != nil || len(list) != 1 || !bytes.Equal(list[0].JWS, first.JWS) {
		t.Fatalf("List after a delete marker = %+v, %v", list, err)
	}
}

// TestListSkipsForeignObjects: objects outside the directory of the instance and names that are
// no checkpoint are not checkpoints.
func TestListSkipsForeignObjects(t *testing.T) {
	t.Parallel()
	s := newTestSink(t, compliance, "")
	if err := s.Put(t.Context(), app.SignedCheckpoint{Seq: 1, JWS: []byte("a.b.c")}); err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{ids.New().String() + "/" + fmt.Sprintf("%020d", 2) + ".jws", s.key("notes"), s.seqKey(2) + ".bak", s.key("2")} {
		if _, err := s.srv.admin.PutObject(t.Context(), &awss3.PutObjectInput{Bucket: aws.String(s.bucket), Key: aws.String(k), Body: strings.NewReader("x")}); err != nil {
			t.Fatal(err)
		}
	}
	list, err := s.List(t.Context())
	if err != nil || len(list) != 1 || list[0].Seq != 1 {
		t.Fatalf("List = %+v, %v", list, err)
	}
}

// TestLockCheck: the target is immutable only with Object Lock enabled and a default retention
// in mode Compliance of at least retention.audit; Governance, a shorter period, a bucket
// without a default retention and a bucket without Object Lock fail with an error naming
// audit.checkpoint_sink (runbook backup-restore 8).
func TestLockCheck(t *testing.T) {
	t.Parallel()
	const day = 24 * time.Hour
	for _, c := range []struct {
		name string
		lock bucketLock
		min  time.Duration
		ok   bool
		want string
	}{
		{"compliance with enough days", compliance, day, true, ""},
		{"compliance in years", bucketLock{enabled: true, mode: types.ObjectLockRetentionModeCompliance, years: 1}, 365 * day, true, ""},
		{"compliance too short", compliance, 2 * day, false, "retention.audit"},
		{"governance", bucketLock{enabled: true, mode: types.ObjectLockRetentionModeGovernance, days: 400}, day, false, "COMPLIANCE"},
		{"lock without default retention", bucketLock{enabled: true}, day, false, "default retention"},
		{"no lock", bucketLock{}, day, false, "Object Lock"},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			s := newTestSink(t, c.lock, "")
			err := s.CheckLock(t.Context(), c.min)
			if c.ok {
				if err != nil {
					t.Fatalf("CheckLock: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), "audit.checkpoint_sink") || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("CheckLock: %v, want an error naming audit.checkpoint_sink and %q", err, c.want)
			}
			if strings.Contains(err.Error(), s.srv.endpoint) || strings.Contains(err.Error(), "127.0.0.1") {
				t.Errorf("the error names the endpoint: %v", err)
			}
		})
	}
}

// TestWrongCredentialsFail: a wrong secret key fails Put, List and CheckLock without the key or
// the endpoint in the message.
func TestWrongCredentialsFail(t *testing.T) {
	t.Parallel()
	srv := sharedSeaweed(t)
	bucket := srv.newBucket(t, compliance)
	cfg, _ := sinkConfig(t, srv, bucket, "")
	dir := t.TempDir()
	for name, value := range map[string]string{accessKeyFile: srv.access, secretKeyFile: "EXAMPLEWRONG" + randomHex(t, 8)} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(value), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	p, err := secrets.NewProvider(secrets.Options{Dir: dir, CacheTTL: time.Minute})
	if err != nil {
		t.Fatal(err)
	}
	s, err := s3sink.New(t.Context(), cfg, ids.New(), s3sink.Deps{Egress: egressFor(t, srv.endpoint), Secrets: p})
	if err != nil {
		t.Fatal(err)
	}
	errs := []error{
		s.Put(t.Context(), app.SignedCheckpoint{Seq: 1, JWS: []byte("a")}),
		s.CheckLock(t.Context(), time.Hour),
	}
	_, lerr := s.List(t.Context())
	errs = append(errs, lerr)
	for i, err := range errs {
		if err == nil {
			t.Errorf("call %d passed with a wrong key", i)
			continue
		}
		if strings.Contains(err.Error(), srv.endpoint) || strings.Contains(err.Error(), "EXAMPLEWRONG") {
			t.Errorf("call %d: the error leaks: %v", i, err)
		}
	}
}

// TestMissingSecretStopsNew: a key file that cannot be read fails New with the key name.
func TestMissingSecretStopsNew(t *testing.T) {
	t.Parallel()
	p, err := secrets.NewProvider(secrets.Options{Dir: t.TempDir(), CacheTTL: time.Minute})
	if err != nil {
		t.Fatal(err)
	}
	cfg := s3sink.Config{
		Bucket: "bothy-audit", Endpoint: "https://s3.example.com", Region: testRegion, UsePathStyle: true,
		AccessKeyRef: "file:<secrets.dir>/" + accessKeyFile, SecretKeyRef: "file:<secrets.dir>/" + secretKeyFile,
	}
	_, err = s3sink.New(context.Background(), cfg, ids.New(), s3sink.Deps{Egress: egress.Config{Timeout: time.Second, MaxResponseBytes: 1}, Secrets: p})
	if err == nil || !strings.Contains(err.Error(), "audit.checkpoint_s3_access_key_ref") {
		t.Fatalf("New: %v, want an error naming audit.checkpoint_s3_access_key_ref", err)
	}
}

// TestAPIMethodSet holds the narrow interface of the sink (docs/spec/12-files.md 4.3, ADR
// 0048): exactly the four operations the sink needs.
func TestAPIMethodSet(t *testing.T) {
	t.Parallel()
	got := s3sink.APIMethods()
	slices.Sort(got)
	want := []string{"GetObject", "GetObjectLockConfiguration", "ListObjectVersions", "PutObject"}
	if !slices.Equal(got, want) {
		t.Fatalf("methods %v, want %v", got, want)
	}
}

// TestListScalesWithManyCheckpoints: the job audit.chain_verify lists the sink on every run, so
// List reads every stored version once per process (a version never changes) and reads new ones
// in parallel; with a remote store answering each GET after a delay, a cold List of many
// checkpoints runs several GETs at once within the bound, and a second List reads nothing again.
func TestListScalesWithManyCheckpoints(t *testing.T) {
	t.Parallel()
	const n, delay = 120, 25 * time.Millisecond
	s := newTestSink(t, compliance, "")
	for seq := int64(1); seq <= n; seq++ {
		if err := s.Put(t.Context(), app.SignedCheckpoint{Seq: seq, JWS: fmt.Appendf(nil, "a.%d.c", seq)}); err != nil {
			t.Fatal(err)
		}
	}
	gets := s3sink.CountingAPI(s.Sink, delay)
	list, err := s.List(t.Context())
	if err != nil || len(list) != n || list[n-1].Seq != n || string(list[0].JWS) != "a.1.c" {
		t.Fatalf("List: %d checkpoints, %v", len(list), err)
	}
	// The cold List reads in parallel within the bound; counted, not timed, so a loaded host
	// cannot fail it.
	if peak := gets.Peak.Load(); peak < 2 || peak > s3sink.ReadParallel {
		t.Errorf("cold List ran %d GETs at once, want between 2 and %d", peak, s3sink.ReadParallel)
	}
	before := gets.Gets.Load()
	if _, err := s.List(t.Context()); err != nil {
		t.Fatal(err)
	}
	if again := gets.Gets.Load() - before; again != 0 {
		t.Errorf("the second List read %d objects again, want 0", again)
	}
	if err := s.Put(t.Context(), app.SignedCheckpoint{Seq: n + 1, JWS: []byte("new.jws.x")}); err != nil {
		t.Fatal(err)
	}
	before = gets.Gets.Load()
	if list, err := s.List(t.Context()); err != nil || len(list) != n+1 || string(list[n].JWS) != "new.jws.x" {
		t.Fatalf("List after a new checkpoint: %d, %v", len(list), err)
	}
	if read := gets.Gets.Load() - before; read != 1 {
		t.Errorf("List after one new checkpoint read %d objects, want 1", read)
	}
}
