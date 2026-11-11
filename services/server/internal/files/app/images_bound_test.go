// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"math"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/files/app"
	"github.com/OWNER/bothy/services/server/internal/files/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// meteredStore is the fake store with a counter: per key the most bytes one reader of Get handed
// out.
type meteredStore struct {
	*store
	mu     sync.Mutex
	pulled map[string]int64
}

// Get implements app.ObjectStore.
func (m *meteredStore) Get(ctx context.Context, key string, rng *app.ByteRange) (io.ReadCloser, error) {
	rc, err := m.store.Get(ctx, key, rng)
	if err != nil {
		return nil, err
	}
	return &meteredBody{ReadCloser: rc, m: m, key: key}, nil
}

// most returns the most bytes one reader of key handed out.
func (m *meteredStore) most(key string) int64 {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.pulled[key]
}

// meteredBody counts what one reader of Get hands out.
type meteredBody struct {
	io.ReadCloser
	m   *meteredStore
	key string
	n   int64
}

// Read implements io.Reader.
func (b *meteredBody) Read(p []byte) (int, error) {
	n, err := b.ReadCloser.Read(p)
	b.n += int64(n)
	b.m.mu.Lock()
	b.m.pulled[b.key] = max(b.m.pulled[b.key], b.n)
	b.m.mu.Unlock()
	return n, err
}

// grow makes the stored object of key longer than its row says, as a storage that answers more
// than was stored or an object that was replaced behind the server.
func (e *imageEnv) grow(key string) {
	e.store.mu.Lock()
	defer e.store.mu.Unlock()
	e.store.objects[key] = append(bytes.Clone(e.store.objects[key]), bytes.Repeat([]byte("x"), 1<<20)...)
}

// TestProcessImageReadsAtMostTheStoredSize (docs/decisions.d/FIX-S3-READ-BOUND-1111.md): the image
// job hands every original to the codec through a reader that ends at the stored size of the
// row, so a codec that keeps what it reads in memory never holds more than the row says. An
// object that yields more is a read error: the run returns it for River to retry, and it
// writes no row, no event and no object.
func TestProcessImageReadsAtMostTheStoredSize(t *testing.T) {
	t.Parallel()
	content := []byte("an image of Robin")
	orig2 := func(f app.FileRecord) string {
		k, _ := domain.OriginalKey(f.ID, 2)
		return k
	}
	for _, c := range []struct {
		name, mime string
		// prepare makes an object longer and returns its key and the size its row holds.
		prepare func(e *imageEnv, f app.FileRecord) (string, int64)
		// wantState, wantWrites and wantKey describe what the run leaves behind.
		wantState  string
		wantWrites []string
		wantKey    func(f app.FileRecord) string
	}{
		{"the original", "image/jpeg", func(e *imageEnv, f app.FileRecord) (string, int64) {
			e.grow(f.ObjectKey)
			return f.ObjectKey, f.SizeBytes
		}, "pending", nil, func(f app.FileRecord) string { return f.ObjectKey }},
		{"the original that stays unchanged", "image/webp", func(e *imageEnv, f app.FileRecord) (string, int64) {
			e.codec.cleanErr = app.ErrImageAnimated
			e.grow(f.ObjectKey)
			return f.ObjectKey, f.SizeBytes
		}, "pending", nil, func(f app.FileRecord) string { return f.ObjectKey }},
		{"the cleaned original of the thumbnails", "image/png", func(e *imageEnv, f app.FileRecord) (string, int64) {
			var once sync.Once
			e.hooks.AfterPut = func(context.Context, ids.ID) { once.Do(func() { e.grow(orig2(f)) }) }
			return orig2(f), int64(len(cleanedPrefix) + len(content))
		}, "complete", []string{"T1"}, orig2},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			var metered *meteredStore
			e := newImageEnvOn(t, nil, func(s *store) app.ObjectStore {
				metered = &meteredStore{store: s, pulled: map[string]int64{}}
				return metered
			})
			e.codec.drain = true
			f := e.seed(t, c.mime, "pending", content)
			key, size := c.prepare(e, f)
			err := e.proc.ProcessImage(context.Background(), f.ID)
			if !errors.Is(err, app.ErrObjectLonger) {
				t.Fatalf("ProcessImage = %v, want the error of an object longer than its row", err)
			}
			if n := metered.most(key); n > size+1 {
				t.Errorf("the job read %d bytes of an object whose row holds %d, want at most the size plus one byte", n, size)
			}
			got, _ := e.repo.imageFile(f.ID)
			if got.UploadState != c.wantState || got.ObjectKey != c.wantKey(f) || len(got.Thumbnails) != 0 || !slices.Equal(e.repo.writes, c.wantWrites) ||
				len(e.events.list) != len(c.wantWrites) {
				t.Errorf("file %+v, writes %v, %d events; want state %s and writes %v", got, e.repo.writes, len(e.events.list), c.wantState, c.wantWrites)
			}
			if k := e.keys(); len(k) != 1 || k[0] != c.wantKey(f) {
				t.Errorf("objects %v, want only %s", k, c.wantKey(f))
			}
		})
	}
}

// TestKeptOriginalInspectsWithTheStoredSize: Inspect gets the stored size of the row, the bound
// of a codec that scans the image in memory.
func TestKeptOriginalInspectsWithTheStoredSize(t *testing.T) {
	t.Parallel()
	e := newImageEnv(t, nil)
	e.codec.cleanErr = app.ErrImageAnimated
	e.codec.info = app.ImageInfo{Width: 7, Height: 5, Frames: 1}
	f := e.seed(t, "image/webp", "pending", []byte("RIFF of Sam"))
	if err := e.proc.ProcessImage(context.Background(), f.ID); err != nil {
		t.Fatal(err)
	}
	if got, _ := e.repo.imageFile(f.ID); got.UploadState != "complete" || got.Width != 7 || e.codec.inspectSize != f.SizeBytes {
		t.Errorf("file %+v, Inspect got the size %d, want %d", got, e.codec.inspectSize, f.SizeBytes)
	}
}

// errOver is the error of the limit in TestLimitedObject.
var errOver = errors.New("over the limit")

// TestLimitedObject: the reader between an object and a consumer that keeps it in memory yields
// at most the limit, takes at most one byte more from the object, and fails with its error, for
// good, when the object has more.
func TestLimitedObject(t *testing.T) {
	t.Parallel()
	const limit = 10
	for _, c := range []struct {
		name    string
		object  string
		want    string
		wantErr error
	}{
		{"shorter than the limit", "0123", "0123", nil},
		{"exactly the limit", "0123456789", "0123456789", nil},
		{"one byte more", "0123456789a", "0123456789", errOver},
		{"far more", strings.Repeat("0123456789", 1000), "0123456789", errOver},
		{"empty", "", "", nil},
	} {
		for _, chunk := range []int{1, 3, limit, 64} {
			src := &chunkReader{data: []byte(c.object)}
			r := app.LimitObject(src, limit, errOver)
			var got []byte
			var err error
			for buf := make([]byte, chunk); err == nil; {
				var n int
				n, err = r.Read(buf)
				got = append(got, buf[:n]...)
			}
			if errors.Is(err, io.EOF) {
				err = nil
			}
			if string(got) != c.want || !errors.Is(err, c.wantErr) || (c.wantErr == nil && err != nil) {
				t.Errorf("%s in reads of %d: %q, %v; want %q, %v", c.name, chunk, got, err, c.want, c.wantErr)
			}
			if src.taken > limit+1 {
				t.Errorf("%s in reads of %d: %d bytes taken from the object, want at most %d", c.name, chunk, src.taken, limit+1)
			}
			if c.wantErr != nil {
				taken := src.taken
				if n, again := r.Read(make([]byte, chunk)); n != 0 || !errors.Is(again, errOver) || src.taken != taken {
					t.Errorf("%s in reads of %d: a read after the error gave %d bytes, %v and took %d more bytes", c.name, chunk, n, again, src.taken-taken)
				}
			}
		}
	}
}

// TestLimitedObjectEdgeLimits: a limit of zero hands out nothing and still tells an empty
// object from one with bytes, a negative limit counts as zero, and the largest limit reads a
// short object to its end.
func TestLimitedObjectEdgeLimits(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name    string
		limit   int64
		object  string
		want    string
		wantErr error
	}{
		{"zero, empty object", 0, "", "", nil},
		{"zero, object with bytes", 0, "abc", "", errOver},
		{"negative, empty object", -5, "", "", nil},
		{"negative, object with bytes", -5, "abc", "", errOver},
		{"largest limit", math.MaxInt64, "abc", "abc", nil},
	} {
		src := &chunkReader{data: []byte(c.object)}
		got, err := io.ReadAll(app.LimitObject(src, c.limit, errOver))
		if string(got) != c.want || !errors.Is(err, c.wantErr) || (c.wantErr == nil && err != nil) {
			t.Errorf("%s: %q, %v; want %q, %v", c.name, got, err, c.want, c.wantErr)
		}
		if src.taken > 1 && c.wantErr != nil {
			t.Errorf("%s: %d bytes taken from the object, want at most one", c.name, src.taken)
		}
	}
}

// chunkReader yields data in the sizes asked for and counts the bytes taken.
type chunkReader struct {
	data  []byte
	taken int
}

// Read implements io.Reader.
func (c *chunkReader) Read(p []byte) (int, error) {
	if len(c.data) == 0 {
		return 0, io.EOF
	}
	n := copy(p, c.data)
	c.data = c.data[n:]
	c.taken += n
	return n, nil
}
