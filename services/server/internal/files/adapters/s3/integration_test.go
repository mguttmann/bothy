// SPDX-License-Identifier: Apache-2.0

package s3_test

import (
	"bytes"
	"crypto/rand"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"

	"github.com/OWNER/bothy/services/server/internal/files/adapters/filestest"
	"github.com/OWNER/bothy/services/server/internal/files/adapters/s3"
	"github.com/OWNER/bothy/services/server/internal/files/app"
)

// minPartSize is the smallest part S3 accepts for every part but the last, 5 MiB (Amazon S3,
// "Multipart upload limits", docs/spec/12-files.md "3.1" [Q1]).
const minPartSize = 5 << 20

// seaweedStore returns a store of the driver on bucket of the shared SeaweedFS, with the keys of
// the server read through the real secret provider and an allow_private entry for the endpoint,
// so every request takes the production path through the egress client.
func seaweedStore(t *testing.T, srv *seaweedServer, bucket string, mutate func(*s3.Config)) (*s3.Store, *s3.Recorder) {
	t.Helper()
	sec := newSecrets(t, srv.access, srv.secret)
	cfg := sec.config(srv.endpoint, bucket)
	if mutate != nil {
		mutate(&cfg)
	}
	rec := &s3.Recorder{}
	store, err := s3.NewRecorded(t.Context(), cfg, sec.deps(loopbackRule(t, srv.endpoint)), rec)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return store, rec
}

// TestS3Conformance runs the conformance suite of the port unchanged against SeaweedFS (12 "15"
// row "Driver conformance", ADR 0010 "Confirmation").
func TestS3Conformance(t *testing.T) {
	t.Parallel()
	srv := sharedSeaweed(t)
	filestest.RunObjectStoreConformance(t, func(t *testing.T) app.ObjectStore {
		store, _ := seaweedStore(t, srv, srv.newBucket(t), nil)
		return store
	}, filestest.Options{PartSize: minPartSize})
	filestest.RunMultipartConformance(t, func(t *testing.T) filestest.MultipartDriver {
		store, _ := seaweedStore(t, srv, srv.newBucket(t), nil)
		return store
	}, filestest.Options{PartSize: minPartSize})
}

// TestS3ReadsObjectAboveBodyLimit (docs/decisions.d/AUD-srv-files-01.md): an object larger than
// the body limit of the store is read completely from SeaweedFS, whole and by range. An export
// package (20 5.2) has no size limit and a converted original may be larger than its upload, so
// no limit of an upload or of an answer may cut the read of a stored object.
func TestS3ReadsObjectAboveBodyLimit(t *testing.T) {
	t.Parallel()
	srv := sharedSeaweed(t)
	store, _ := seaweedStore(t, srv, srv.newBucket(t), nil)
	key := newKey(t)
	data := make([]byte, 3*testAnswerBytes+1)
	if _, err := rand.Read(data); err != nil {
		t.Fatal(err)
	}
	if err := store.Put(t.Context(), key, bytes.NewReader(data), int64(len(data))); err != nil {
		t.Fatalf("Put: %v", err)
	}
	read := func(rng *app.ByteRange) []byte {
		t.Helper()
		rc, err := store.Get(t.Context(), key, rng)
		if err != nil {
			t.Fatalf("Get %+v: %v", rng, err)
		}
		defer func() { _ = rc.Close() }()
		got, err := io.ReadAll(rc)
		if err != nil {
			t.Fatalf("read %+v after %d bytes: %v", rng, len(got), err)
		}
		return got
	}
	if got := read(nil); !bytes.Equal(got, data) {
		t.Errorf("the whole object: %d bytes, want the %d bytes stored", len(got), len(data))
	}
	rng := &app.ByteRange{Offset: testAnswerBytes - 1, Length: 2*testAnswerBytes + 2}
	if got := read(rng); !bytes.Equal(got, data[rng.Offset:rng.Offset+rng.Length]) {
		t.Errorf("the range %+v: %d bytes, want %d", rng, len(got), rng.Length)
	}
}

// TestS3RequestsAreCoreS3 holds "nur Kern-S3" (12 "4.3", ADR 0010): during the suite every
// request is one of the core S3 requests on an object, never DeleteObjects, a bucket request, a
// listing, a copy, an encryption header or, with when_required, a checksum header.
func TestS3RequestsAreCoreS3(t *testing.T) {
	t.Parallel()
	srv := sharedSeaweed(t)
	var recs []*s3.Recorder
	var buckets []string
	newStore := func(t *testing.T) *s3.Store {
		bucket := srv.newBucket(t)
		store, rec := seaweedStore(t, srv, bucket, nil)
		recs, buckets = append(recs, rec), append(buckets, bucket)
		return store
	}
	filestest.RunObjectStoreConformance(t, func(t *testing.T) app.ObjectStore { return newStore(t) }, filestest.Options{PartSize: minPartSize})
	filestest.RunMultipartConformance(t, func(t *testing.T) filestest.MultipartDriver { return newStore(t) }, filestest.Options{PartSize: minPartSize})
	total := 0
	for i, rec := range recs {
		for _, r := range rec.Requests() {
			total++
			if why := notCoreS3(r, buckets[i]); why != "" {
				t.Errorf("%s %s?%s: %s", r.Method, r.Path, r.RawQuery, why)
			}
		}
	}
	if total == 0 {
		t.Fatal("no request recorded; the check would prove nothing")
	}
}

// notCoreS3 returns why r is not a core S3 request on an object of bucket, or "".
func notCoreS3(r s3.Recorded, bucket string) string {
	if !strings.HasPrefix(r.Path, "/"+bucket+"/") || len(r.Path) == len(bucket)+2 {
		return "not a request on an object of the bucket"
	}
	q, err := url.ParseQuery(r.RawQuery)
	if err != nil {
		return "query does not parse"
	}
	names := map[string]bool{}
	for k := range q {
		if k != "x-id" {
			names[k] = true
		}
	}
	has := func(want ...string) bool {
		if len(names) != len(want) {
			return false
		}
		for _, w := range want {
			if !names[w] {
				return false
			}
		}
		return true
	}
	ok := false
	switch r.Method {
	case http.MethodPut:
		ok = has() || has("partNumber", "uploadId")
	case http.MethodGet, http.MethodHead:
		ok = has()
	case http.MethodDelete:
		ok = has() || has("uploadId")
	case http.MethodPost:
		ok = has("uploads") || has("uploadId")
	}
	if !ok {
		return "method and query are not a core S3 operation"
	}
	for name := range r.Header {
		n := strings.ToLower(name)
		switch {
		case n == "x-amz-copy-source", strings.HasPrefix(n, "x-amz-server-side-encryption"):
			return "header " + n + " is not core S3"
		case strings.HasPrefix(n, "x-amz-checksum-"), n == "x-amz-sdk-checksum-algorithm", n == "x-amz-trailer":
			return "checksum header " + n + " with checksum mode when_required"
		case n == "x-amz-acl", n == "x-amz-tagging", n == "x-amz-storage-class", strings.HasPrefix(n, "x-amz-object-lock"),
			n == "if-match", n == "if-none-match":
			return "header " + n + " is not set by the driver"
		}
	}
	return ""
}

// unseekable hides every method but Read, so the SDK cannot seek or hash the body before sending.
type unseekable struct{ r io.Reader }

func (u unseekable) Read(p []byte) (int, error) { return u.r.Read(p) }

// TestS3StreamsUnseekableBodies holds docs/DECISIONS.md M3-02/4: Put and UploadPart stream readers
// without Seek over plain http as UNSIGNED-PAYLOAD.
func TestS3StreamsUnseekableBodies(t *testing.T) {
	t.Parallel()
	srv := sharedSeaweed(t)
	store, rec := seaweedStore(t, srv, srv.newBucket(t), nil)
	ctx, key := t.Context(), newKey(t)
	data := bytes.Repeat([]byte("bothy"), 1000)
	if err := store.Put(ctx, key, unseekable{bytes.NewReader(data)}, int64(len(data))); err != nil {
		t.Fatalf("Put: %v", err)
	}
	mpKey := newKey(t)
	upload, err := store.CreateMultipart(ctx, mpKey)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.UploadPart(ctx, mpKey, upload, 1, unseekable{bytes.NewReader(data)}, int64(len(data))); err != nil {
		t.Fatalf("UploadPart: %v", err)
	}
	n := 0
	for _, r := range rec.Requests() {
		if r.Method != http.MethodPut {
			continue
		}
		n++
		if got := r.Header.Get("X-Amz-Content-Sha256"); got != "UNSIGNED-PAYLOAD" {
			t.Errorf("PUT %s?%s: X-Amz-Content-Sha256 = %q, want UNSIGNED-PAYLOAD", r.Path, r.RawQuery, got)
		}
	}
	if n < 2 {
		t.Errorf("recorded %d PUT requests, want at least 2", n)
	}
	rc, err := store.Get(ctx, key, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = rc.Close() }()
	if got, _ := io.ReadAll(rc); !bytes.Equal(got, data) {
		t.Errorf("Get returned %d bytes, want %d", len(got), len(data))
	}
}

// TestS3PrefixSeparatesEnvironments holds 12 "4.4" (file.s3.prefix): two stores on one bucket
// with prefixes env-a and env-b never see each other's objects.
func TestS3PrefixSeparatesEnvironments(t *testing.T) {
	t.Parallel()
	srv := sharedSeaweed(t)
	bucket := srv.newBucket(t)
	a, _ := seaweedStore(t, srv, bucket, func(c *s3.Config) { c.Prefix = "env-a" })
	b, _ := seaweedStore(t, srv, bucket, func(c *s3.Config) { c.Prefix = "env-b" })
	ctx, key := t.Context(), newKey(t)
	if err := a.Put(ctx, key, strings.NewReader("alex"), 4); err != nil {
		t.Fatal(err)
	}
	if _, err := srv.admin.HeadObject(ctx, &awss3.HeadObjectInput{Bucket: aws.String(bucket), Key: aws.String("env-a/" + key)}); err != nil {
		t.Errorf("full client: env-a/%s: %v", key, err)
	}
	if _, err := srv.admin.HeadObject(ctx, &awss3.HeadObjectInput{Bucket: aws.String(bucket), Key: aws.String(key)}); err == nil {
		t.Errorf("full client: %s exists without prefix", key)
	}
	if _, err := b.Head(ctx, key); !errors.Is(err, app.ErrNotFound) {
		t.Errorf("store env-b Head = %v, want ErrNotFound", err)
	}
	if err := b.Put(ctx, key, strings.NewReader("sam"), 3); err != nil {
		t.Fatal(err)
	}
	for store, want := range map[*s3.Store]string{a: "alex", b: "sam"} {
		rc, err := store.Get(ctx, key, nil)
		if err != nil {
			t.Fatal(err)
		}
		got, _ := io.ReadAll(rc)
		_ = rc.Close()
		if string(got) != want {
			t.Errorf("Get = %q, want %q", got, want)
		}
	}
	if err := b.Delete(ctx, key); err != nil {
		t.Fatal(err)
	}
	if info, err := a.Head(ctx, key); err != nil || info.Size != 4 {
		t.Errorf("env-a after deleting in env-b: %+v, %v", info, err)
	}
}

// TestS3WrongCredentialsRefused: SeaweedFS with the start command of the Compose service refuses a
// wrong secret key; the store gets a storage failure and nothing is written.
func TestS3WrongCredentialsRefused(t *testing.T) {
	t.Parallel()
	srv := sharedSeaweed(t)
	bucket := srv.newBucket(t)
	_, wrongSecret := testKeys(t)
	sec := newSecrets(t, srv.access, wrongSecret)
	store, err := s3.New(t.Context(), sec.config(srv.endpoint, bucket), sec.deps(loopbackRule(t, srv.endpoint)))
	if err != nil {
		t.Fatal(err)
	}
	key := newKey(t)
	err = store.Put(t.Context(), key, strings.NewReader("robin"), 5)
	if err == nil || isSentinel(err) {
		t.Fatalf("Put with a wrong secret = %v, want a storage failure", err)
	}
	if _, err := srv.admin.HeadObject(t.Context(), &awss3.HeadObjectInput{Bucket: aws.String(bucket), Key: aws.String(key)}); err == nil {
		t.Error("the object was written with a wrong secret")
	}
	if strings.Contains(err.Error(), wrongSecret) || strings.Contains(err.Error(), srv.endpoint) {
		t.Errorf("error %q carries the secret or the endpoint", err)
	}
}

// listParts returns the parts the full client sees for an upload.
func listParts(t *testing.T, srv *seaweedServer, bucket, key, upload string) []int32 {
	t.Helper()
	out, err := srv.admin.ListParts(t.Context(), &awss3.ListPartsInput{Bucket: aws.String(bucket), Key: aws.String(key), UploadId: aws.String(upload)})
	if err != nil {
		t.Fatalf("ListParts: %v", err)
	}
	var nums []int32
	for _, p := range out.Parts {
		nums = append(nums, aws.ToInt32(p.PartNumber))
	}
	return nums
}

// TestS3UploadPartSizeMismatch holds docs/DECISIONS.md M3-02/8: a part body one byte shorter or
// longer than size gives ErrSizeMismatch and leaves no part.
func TestS3UploadPartSizeMismatch(t *testing.T) {
	t.Parallel()
	srv := sharedSeaweed(t)
	bucket := srv.newBucket(t)
	store, _ := seaweedStore(t, srv, bucket, nil)
	ctx, key := t.Context(), newKey(t)
	upload, err := store.CreateMultipart(ctx, key)
	if err != nil {
		t.Fatal(err)
	}
	data := bytes.Repeat([]byte("z"), 4097)
	if _, err := store.UploadPart(ctx, key, upload, 1, bytes.NewReader(data[:4095]), 4096); !errors.Is(err, app.ErrSizeMismatch) {
		t.Errorf("short part err = %v, want ErrSizeMismatch", err)
	}
	if _, err := store.UploadPart(ctx, key, upload, 2, bytes.NewReader(data), 4096); !errors.Is(err, app.ErrSizeMismatch) {
		t.Errorf("long part err = %v, want ErrSizeMismatch", err)
	}
	if parts := listParts(t, srv, bucket, key, upload); len(parts) != 0 {
		t.Errorf("parts %v stored, want none", parts)
	}
	if _, err := store.UploadPart(ctx, key, upload, 3, bytes.NewReader(data[:4096]), 4096); err != nil {
		t.Errorf("exact part: %v", err)
	}
	if parts := listParts(t, srv, bucket, key, upload); len(parts) != 1 || parts[0] != 3 {
		t.Errorf("parts %v, want [3]", parts)
	}
	if err := store.AbortMultipart(ctx, key, upload); err != nil {
		t.Fatal(err)
	}
}
