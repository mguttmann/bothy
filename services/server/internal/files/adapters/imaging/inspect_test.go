// SPDX-License-Identifier: Apache-2.0

package imaging

import (
	"bytes"
	"context"
	"errors"
	"io"
	"sync/atomic"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/files/app"
)

// countingOpener returns an Opener over b and the number of bytes its readers handed out so far.
func countingOpener(b []byte) (app.Opener, *atomic.Int64) {
	n := &atomic.Int64{}
	return func(context.Context) (io.ReadCloser, error) {
		return io.NopCloser(&countingReader{r: bytes.NewReader(b), n: n}), nil
	}, n
}

// countingReader counts the bytes read from r.
type countingReader struct {
	r io.Reader
	n *atomic.Int64
}

// Read implements io.Reader.
func (c *countingReader) Read(p []byte) (int, error) {
	n, err := c.r.Read(p)
	c.n.Add(int64(n))
	return n, err
}

// TestInspectWebPReadsAtMostTheStoredSize (docs/decisions.d/FIX-S3-READ-BOUND-1111.md): Inspect
// keeps a WebP in memory to scan its chunks, so it reads at most the stored size of the row plus
// one byte. A storage that answers more than the row says cannot make the job hold more, and an
// object of another length is a read error of the store, never an image error: the job retries
// and stores nothing.
func TestInspectWebPReadsAtMostTheStoredSize(t *testing.T) {
	data := buildWebP(t, twoColour(8, 4, testRed, testBlue), webpOptions{Animated: true})
	size := int64(len(data))

	t.Run("the stored object", func(t *testing.T) {
		open, read := countingOpener(data)
		info, err := Codec{}.Inspect(t.Context(), mimeWebP, open, size)
		if err != nil || info.Width != 8 || info.Height != 4 || info.Frames != 1 {
			t.Fatalf("Inspect = %+v, %v; want 8x4 with one frame", info, err)
		}
		if read.Load() != size {
			t.Errorf("read %d bytes of an object of %d", read.Load(), size)
		}
	})

	for name, object := range map[string][]byte{
		"a longer object":  append(bytes.Clone(data), make([]byte, 1<<20)...),
		"a shorter object": data[:len(data)-1],
	} {
		t.Run(name, func(t *testing.T) {
			open, read := countingOpener(object)
			_, err := Codec{}.Inspect(t.Context(), mimeWebP, open, size)
			if !errors.Is(err, errOriginalSize) {
				t.Fatalf("Inspect = %v, want the size error of the original", err)
			}
			for _, image := range []error{app.ErrImageMalformed, app.ErrImageUndecodable, app.ErrTooManyPixels, app.ErrImageAnimated} {
				if errors.Is(err, image) {
					t.Errorf("Inspect = %v, an image error; want a read error the job retries", err)
				}
			}
			if read.Load() > size+1 {
				t.Errorf("read %d bytes for a row of %d, want at most the size plus one byte", read.Load(), size)
			}
		})
	}
}
