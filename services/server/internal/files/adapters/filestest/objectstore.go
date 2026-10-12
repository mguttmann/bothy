// SPDX-License-Identifier: Apache-2.0

package filestest

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"io"
	"math/rand"
	"sync"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/files/app"
	"github.com/OWNER/bothy/services/server/internal/files/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// Options tunes RunObjectStoreConformance to a driver.
type Options struct {
	// PartSize is the size of every part but the last in the multipart cases. It must be at
	// least the smallest part the driver accepts: the file system driver accepts any size, the
	// S3 driver needs the S3 minimum that docs/spec/12-dateien.md "3.1" cites.
	PartSize int64
}

// RunObjectStoreConformance runs the conformance suite of the ObjectStore port against one
// driver (docs/spec/12-dateien.md "15. Tests" row "Treiber-Konformität", ADR 0010
// "Confirmation"). Every driver must pass it. Each case is a subtest with a fresh, empty store
// from newStore; file IDs come from ids.New, keys only from the domain builders, and content from
// a generator whose seed the subtest logs.
//
// The suite asserts nothing a driver may do differently: the form of the upload ID, whether
// parts are readable as objects, and what Put on an existing key does.
//
// Example (in a _test.go file of a driver):
//
//	func TestFSConformance(t *testing.T) {
//		filestest.RunObjectStoreConformance(t, func(t *testing.T) app.ObjectStore {
//			return openStore(t)
//		}, filestest.Options{PartSize: 1024})
//	}
func RunObjectStoreConformance(t *testing.T, newStore func(t *testing.T) app.ObjectStore, opts Options) {
	t.Helper()
	if opts.PartSize < 1 {
		t.Fatalf("filestest: Options.PartSize must be at least 1, got %d", opts.PartSize)
	}
	cases := []struct {
		name string
		run  func(t *testing.T, s app.ObjectStore, c *caseEnv)
	}{
		{"PutGetHead", casePutGetHead},
		{"GetRange", caseGetRange},
		{"InvalidRange", caseInvalidRange},
		{"MissingObject", caseMissingObject},
		{"DeleteMissing", caseDeleteMissing},
		{"PutSizeMismatch", casePutSizeMismatch},
		{"PutCancelled", casePutCancelled},
		{"InvalidKeys", caseInvalidKeys},
		{"MultipartComplete", caseMultipartComplete},
		{"MultipartAbort", caseMultipartAbort},
		{"MultipartInvalidParts", caseMultipartInvalidParts},
		{"MultipartUnknownUpload", caseMultipartUnknownUpload},
		{"KeysIndependent", caseKeysIndependent},
		{"ConcurrentPuts", caseConcurrentPuts},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			seed := time.Now().UnixNano()
			t.Logf("content seed %d", seed)
			rnd := rand.New(rand.NewSource(seed)) //nolint:gosec // G404: reproducible test content from a logged seed
			env := &caseEnv{rnd: rnd, partSize: opts.PartSize}
			tc.run(t, newStore(t), env)
		})
	}
}

// caseEnv carries the seeded content generator and the options of one case.
type caseEnv struct {
	mu       sync.Mutex
	rnd      *rand.Rand
	partSize int64
}

// content returns n bytes from the seeded generator.
func (c *caseEnv) content(n int64) []byte {
	c.mu.Lock()
	defer c.mu.Unlock()
	b := make([]byte, n)
	_, _ = c.rnd.Read(b)
	return b
}

func originalKey(t *testing.T, id ids.ID) string {
	t.Helper()
	k, err := domain.OriginalKey(id, 1)
	if err != nil {
		t.Fatal(err)
	}
	return k
}

func put(t *testing.T, s app.ObjectStore, key string, data []byte) {
	t.Helper()
	if err := s.Put(t.Context(), key, bytes.NewReader(data), int64(len(data))); err != nil {
		t.Fatalf("Put(%s, %d bytes): %v", key, len(data), err)
	}
}

// read returns the bytes Get yields for key and rng.
func read(t *testing.T, s app.ObjectStore, key string, rng *app.ByteRange) []byte {
	t.Helper()
	rc, err := s.Get(t.Context(), key, rng)
	if err != nil {
		t.Fatalf("Get(%s, %+v): %v", key, rng, err)
	}
	defer func() { _ = rc.Close() }()
	data, err := io.ReadAll(rc)
	if err != nil {
		t.Fatalf("read %s: %v", key, err)
	}
	return data
}

func sameBytes(t *testing.T, what string, got, want []byte) {
	t.Helper()
	if sha256.Sum256(got) != sha256.Sum256(want) || len(got) != len(want) {
		t.Errorf("%s: got %d bytes (sha256 %x), want %d bytes (sha256 %x)", what, len(got), sha256.Sum256(got), len(want), sha256.Sum256(want))
	}
}

func wantHead(t *testing.T, s app.ObjectStore, key string, size int64) {
	t.Helper()
	info, err := s.Head(t.Context(), key)
	if err != nil || info.Size != size {
		t.Errorf("Head(%s) = %+v, %v; want size %d", key, info, err, size)
	}
}

func wantMissing(t *testing.T, s app.ObjectStore, key string) {
	t.Helper()
	if _, err := s.Head(t.Context(), key); !errors.Is(err, app.ErrNotFound) {
		t.Errorf("Head(%s) err = %v, want ErrNotFound", key, err)
	}
}

func casePutGetHead(t *testing.T, s app.ObjectStore, c *caseEnv) {
	key := originalKey(t, ids.New())
	data := c.content(3*c.partSize + 17)
	put(t, s, key, data)
	sameBytes(t, "Get", read(t, s, key, nil), data)
	wantHead(t, s, key, int64(len(data)))
	empty := originalKey(t, ids.New())
	put(t, s, empty, nil)
	wantHead(t, s, empty, 0)
	sameBytes(t, "Get of the empty object", read(t, s, empty, nil), nil)
}

func caseGetRange(t *testing.T, s app.ObjectStore, c *caseEnv) {
	key := originalKey(t, ids.New())
	data := c.content(1000)
	put(t, s, key, data)
	cases := []struct {
		name string
		rng  app.ByteRange
		want []byte
	}{
		{"middle", app.ByteRange{Offset: 100, Length: 250}, data[100:350]},
		{"ends at last byte", app.ByteRange{Offset: 900, Length: 100}, data[900:]},
		{"beyond the end", app.ByteRange{Offset: 990, Length: 500}, data[990:]},
		{"first byte", app.ByteRange{Offset: 0, Length: 1}, data[:1]},
		{"last byte", app.ByteRange{Offset: 999, Length: 1}, data[999:]},
	}
	for _, rc := range cases {
		rng := rc.rng
		sameBytes(t, "range "+rc.name, read(t, s, key, &rng), rc.want)
	}
	sameBytes(t, "nil range", read(t, s, key, nil), data)
}

func caseInvalidRange(t *testing.T, s app.ObjectStore, c *caseEnv) {
	key := originalKey(t, ids.New())
	put(t, s, key, c.content(100))
	for _, rng := range []app.ByteRange{{Offset: -1, Length: 10}, {Offset: 0, Length: 0}, {Offset: 0, Length: -5}, {Offset: 100, Length: 1}, {Offset: 500, Length: 1}} {
		rc, err := s.Get(t.Context(), key, &rng)
		if rc != nil {
			_ = rc.Close()
		}
		if !errors.Is(err, app.ErrInvalidRange) {
			t.Errorf("Get(%+v) err = %v, want ErrInvalidRange", rng, err)
		}
	}
}

func caseMissingObject(t *testing.T, s app.ObjectStore, _ *caseEnv) {
	key := originalKey(t, ids.New())
	rc, err := s.Get(t.Context(), key, nil)
	if rc != nil {
		_ = rc.Close()
	}
	if !errors.Is(err, app.ErrNotFound) {
		t.Errorf("Get of a missing object err = %v, want ErrNotFound", err)
	}
	wantMissing(t, s, key)
}

func caseDeleteMissing(t *testing.T, s app.ObjectStore, c *caseEnv) {
	never := originalKey(t, ids.New())
	if err := s.Delete(t.Context(), never); err != nil {
		t.Errorf("Delete of a key never written = %v, want nil", err)
	}
	key := originalKey(t, ids.New())
	put(t, s, key, c.content(10))
	for i := range 2 {
		if err := s.Delete(t.Context(), key); err != nil {
			t.Errorf("Delete #%d = %v, want nil", i+1, err)
		}
	}
	wantMissing(t, s, key)
}

func casePutSizeMismatch(t *testing.T, s app.ObjectStore, c *caseEnv) {
	key := originalKey(t, ids.New())
	data := c.content(64)
	if err := s.Put(t.Context(), key, bytes.NewReader(data[:63]), 64); !errors.Is(err, app.ErrSizeMismatch) {
		t.Errorf("Put of a short body err = %v, want ErrSizeMismatch", err)
	}
	wantMissing(t, s, key)
	if err := s.Put(t.Context(), key, bytes.NewReader(data), 63); !errors.Is(err, app.ErrSizeMismatch) {
		t.Errorf("Put of a long body err = %v, want ErrSizeMismatch", err)
	}
	wantMissing(t, s, key)
	if err := s.Put(t.Context(), key, bytes.NewReader(nil), -1); !errors.Is(err, app.ErrSizeMismatch) {
		t.Errorf("Put with size -1 err = %v, want ErrSizeMismatch", err)
	}
	wantMissing(t, s, key)
}

func casePutCancelled(t *testing.T, s app.ObjectStore, c *caseEnv) {
	key := originalKey(t, ids.New())
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	data := c.content(128)
	if err := s.Put(ctx, key, bytes.NewReader(data), int64(len(data))); err == nil {
		t.Error("Put with a cancelled context succeeded")
	}
	wantMissing(t, s, key)
}

// invalidKeys are refused by every operation of every driver before any storage call.
var invalidKeys = []string{"..", "../x", "/abs", "A", "a.b", "a//b", ""}

func caseInvalidKeys(t *testing.T, s app.ObjectStore, _ *caseEnv) {
	ctx := t.Context()
	for _, k := range invalidKeys {
		rc, getErr := s.Get(ctx, k, nil)
		if rc != nil {
			_ = rc.Close()
		}
		_, headErr := s.Head(ctx, k)
		_, createErr := s.CreateMultipart(ctx, k)
		_, partErr := s.UploadPart(ctx, k, "", 1, bytes.NewReader([]byte("x")), 1)
		_, presignErr := s.PresignUploadPart(ctx, k, "", 1, time.Minute)
		errs := map[string]error{
			"Put":               s.Put(ctx, k, bytes.NewReader([]byte("x")), 1),
			"Get":               getErr,
			"Head":              headErr,
			"Delete":            s.Delete(ctx, k),
			"CreateMultipart":   createErr,
			"UploadPart":        partErr,
			"CompleteMultipart": s.CompleteMultipart(ctx, k, "", []app.Part{{Number: 1, ETag: `"x"`}}),
			"AbortMultipart":    s.AbortMultipart(ctx, k, ""),
			"PresignUploadPart": presignErr,
		}
		for op, err := range errs {
			if !errors.Is(err, app.ErrInvalidKey) {
				t.Errorf("%s(%q) err = %v, want ErrInvalidKey", op, k, err)
			}
		}
	}
}

// startUpload opens a multipart upload on a fresh original key.
func startUpload(t *testing.T, s app.ObjectStore) (key, uploadID string) {
	t.Helper()
	key = originalKey(t, ids.New())
	uploadID, err := s.CreateMultipart(t.Context(), key)
	if err != nil {
		t.Fatalf("CreateMultipart: %v", err)
	}
	return key, uploadID
}

func uploadPart(t *testing.T, s app.ObjectStore, key, uploadID string, n int, data []byte) app.Part {
	t.Helper()
	p, err := s.UploadPart(t.Context(), key, uploadID, n, bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("UploadPart(%d): %v", n, err)
	}
	if p.Number != n || p.ETag == "" {
		t.Fatalf("UploadPart(%d) = %+v", n, p)
	}
	return p
}

func caseMultipartComplete(t *testing.T, s app.ObjectStore, c *caseEnv) {
	key, uploadID := startUpload(t, s)
	data := [][]byte{c.content(c.partSize), c.content(c.partSize), c.content(c.partSize/2 + 1)}
	parts := make([]app.Part, 3)
	var wg sync.WaitGroup
	var mu sync.Mutex
	var firstErr error
	for _, n := range []int{3, 1, 2} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			p, err := s.UploadPart(t.Context(), key, uploadID, n, bytes.NewReader(data[n-1]), int64(len(data[n-1])))
			mu.Lock()
			defer mu.Unlock()
			if err != nil && firstErr == nil {
				firstErr = err
			}
			parts[n-1] = p
		}()
	}
	wg.Wait()
	if firstErr != nil {
		t.Fatalf("UploadPart in parallel: %v", firstErr)
	}
	if err := s.CompleteMultipart(t.Context(), key, uploadID, parts); err != nil {
		t.Fatalf("CompleteMultipart: %v", err)
	}
	whole := bytes.Join(data, nil)
	sameBytes(t, "completed object", read(t, s, key, nil), whole)
	wantHead(t, s, key, int64(len(whole)))
}

func caseMultipartAbort(t *testing.T, s app.ObjectStore, c *caseEnv) {
	key, uploadID := startUpload(t, s)
	p1 := uploadPart(t, s, key, uploadID, 1, c.content(c.partSize))
	p2 := uploadPart(t, s, key, uploadID, 2, c.content(10))
	if err := s.AbortMultipart(t.Context(), key, uploadID); err != nil {
		t.Fatalf("AbortMultipart: %v", err)
	}
	wantMissing(t, s, key)
	if _, err := s.UploadPart(t.Context(), key, uploadID, 3, bytes.NewReader([]byte("x")), 1); !errors.Is(err, app.ErrUploadNotFound) {
		t.Errorf("UploadPart after abort err = %v, want ErrUploadNotFound", err)
	}
	if err := s.CompleteMultipart(t.Context(), key, uploadID, []app.Part{p1, p2}); !errors.Is(err, app.ErrUploadNotFound) {
		t.Errorf("CompleteMultipart after abort err = %v, want ErrUploadNotFound", err)
	}
	if err := s.AbortMultipart(t.Context(), key, uploadID); err != nil {
		t.Errorf("second AbortMultipart = %v, want nil", err)
	}
	wantMissing(t, s, key)
}

func caseMultipartInvalidParts(t *testing.T, s app.ObjectStore, c *caseEnv) {
	key, uploadID := startUpload(t, s)
	d1, d2 := c.content(c.partSize), c.content(5)
	p1 := uploadPart(t, s, key, uploadID, 1, d1)
	p2 := uploadPart(t, s, key, uploadID, 2, d2)
	wrong := p2
	wrong.ETag = `"0000000000000000000000000000000000000000000000000000000000000000"`
	if wrong.ETag == p2.ETag {
		wrong.ETag = `"1111111111111111111111111111111111111111111111111111111111111111"`
	}
	bad := map[string][]app.Part{
		"wrong ETag":          {p1, wrong},
		"number not uploaded": {p1, p2, {Number: 3, ETag: p2.ETag}},
		"descending numbers":  {p2, p1},
		"repeated number":     {p1, p1},
		"empty list":          {},
		"number zero":         {{Number: 0, ETag: p1.ETag}, p2},
	}
	for name, parts := range bad {
		if err := s.CompleteMultipart(t.Context(), key, uploadID, parts); !errors.Is(err, app.ErrInvalidPart) {
			t.Errorf("CompleteMultipart with %s err = %v, want ErrInvalidPart", name, err)
		}
		wantMissing(t, s, key)
	}
	if _, err := s.UploadPart(t.Context(), key, uploadID, 0, bytes.NewReader([]byte("x")), 1); !errors.Is(err, app.ErrInvalidPart) {
		t.Errorf("UploadPart(0) err = %v, want ErrInvalidPart", err)
	}
	if err := s.CompleteMultipart(t.Context(), key, uploadID, []app.Part{p1, p2}); err != nil {
		t.Fatalf("correct CompleteMultipart after refusals: %v", err)
	}
	sameBytes(t, "completed object", read(t, s, key, nil), append(append([]byte{}, d1...), d2...))
}

func caseMultipartUnknownUpload(t *testing.T, s app.ObjectStore, _ *caseEnv) {
	key := originalKey(t, ids.New())
	// The list shape is checked before any storage call, so an empty list is ErrInvalidPart even
	// without an open upload.
	if err := s.CompleteMultipart(t.Context(), key, "unknown", nil); !errors.Is(err, app.ErrInvalidPart) {
		t.Errorf("CompleteMultipart with an empty list err = %v, want ErrInvalidPart", err)
	}
	if _, err := s.UploadPart(t.Context(), key, "unknown", 1, bytes.NewReader([]byte("x")), 1); !errors.Is(err, app.ErrUploadNotFound) {
		t.Errorf("UploadPart err = %v, want ErrUploadNotFound", err)
	}
	if err := s.CompleteMultipart(t.Context(), key, "unknown", []app.Part{{Number: 1, ETag: `"x"`}}); !errors.Is(err, app.ErrUploadNotFound) {
		t.Errorf("CompleteMultipart err = %v, want ErrUploadNotFound", err)
	}
	if err := s.AbortMultipart(t.Context(), key, "unknown"); err != nil {
		t.Errorf("AbortMultipart = %v, want nil", err)
	}
	wantMissing(t, s, key)
}

func caseKeysIndependent(t *testing.T, s app.ObjectStore, c *caseEnv) {
	id := ids.New()
	keys := []string{originalKey(t, id)}
	for _, v := range []domain.Variant{domain.VariantSmall, domain.VariantMedium, domain.VariantLarge, domain.VariantPoster} {
		k, err := domain.VariantKey(id, v)
		if err != nil {
			t.Fatal(err)
		}
		keys = append(keys, k)
	}
	data := map[string][]byte{}
	for i, k := range keys {
		data[k] = c.content(int64(20 + i))
		put(t, s, k, data[k])
	}
	for _, victim := range keys {
		if err := s.Delete(t.Context(), victim); err != nil {
			t.Fatalf("Delete(%s): %v", victim, err)
		}
		wantMissing(t, s, victim)
		for _, k := range keys {
			if k != victim {
				sameBytes(t, k+" after deleting "+victim, read(t, s, k, nil), data[k])
			}
		}
		put(t, s, victim, data[victim])
	}
}

func caseConcurrentPuts(t *testing.T, s app.ObjectStore, c *caseEnv) {
	id := ids.New()
	keys := []string{originalKey(t, id)}
	for _, v := range []domain.Variant{domain.VariantSmall, domain.VariantMedium, domain.VariantLarge, domain.VariantPoster} {
		k, _ := domain.VariantKey(id, v)
		keys = append(keys, k)
	}
	for rev := 2; rev <= 4; rev++ {
		k, _ := domain.OriginalKey(id, rev)
		keys = append(keys, k)
	}
	data := make([][]byte, len(keys))
	for i := range keys {
		data[i] = c.content(c.partSize + int64(i))
	}
	errs := make([]error, len(keys))
	var wg sync.WaitGroup
	for i, k := range keys {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs[i] = s.Put(t.Context(), k, bytes.NewReader(data[i]), int64(len(data[i])))
		}()
	}
	wg.Wait()
	for i, k := range keys {
		if errs[i] != nil {
			t.Errorf("parallel Put(%s): %v", k, errs[i])
			continue
		}
		sameBytes(t, k, read(t, s, k, nil), data[i])
	}
}
