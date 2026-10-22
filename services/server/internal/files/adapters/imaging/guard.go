// SPDX-License-Identifier: Apache-2.0

package imaging

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/OWNER/bothy/services/server/internal/files/app"
	"github.com/OWNER/bothy/services/server/internal/files/domain"
)

// The media types the codec reads, as the type detection names them.
const (
	mimeJPEG = "image/jpeg"
	mimePNG  = "image/png"
	mimeAPNG = "image/apng"
	mimeGIF  = "image/gif"
	mimeWebP = "image/webp"
)

// imageHeader is what the guard reads before any decode (12 7.2, SEC-UP-4): the stored size, the
// frames, the orientation and the colour profile, without decoding a pixel.
type imageHeader struct {
	// Width and Height are the stored size, before the orientation.
	Width, Height int
	// Frames is the number of frames (1 for a still image).
	Frames int
	// Orientation is the EXIF orientation of a JPEG; 1 otherwise (a PNG eXIf is never applied,
	// D-M3-06-4).
	Orientation domain.Orientation
	// Profile is the ICC profile of a JPEG, nil when none.
	Profile []byte
	// ICCP is the data of the iCCP chunk of a PNG, nil when none.
	ICCP []byte
}

// trackingReader remembers the first read error of the store that is not io.EOF, so a parse
// error caused by the store is told apart from a broken image (the first is transient, the
// second permanent).
type trackingReader struct {
	r   io.Reader
	err error
}

// Read implements io.Reader.
func (t *trackingReader) Read(b []byte) (int, error) {
	n, err := t.r.Read(b)
	if err != nil && !errors.Is(err, io.EOF) && t.err == nil {
		t.err = err
	}
	return n, err
}

// withSource opens the original, runs fn over a tracking reader and closes it. A failure of fn
// after a read error of the store returns the read error.
func withSource(ctx context.Context, open app.Opener, fn func(r io.Reader) error) error {
	rc, err := open(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = rc.Close() }()
	tr := &trackingReader{r: rc}
	if err := fn(tr); err != nil {
		if tr.err != nil {
			return tr.err
		}
		return err
	}
	return nil
}

// readHeader reads the header of an original of mimeType through open without decoding pixels:
// the segment scan of a JPEG, the chunk walk of a PNG, the block scan of a GIF. Errors wrap
// app.ErrImageMalformed, or are read errors of the store.
func readHeader(ctx context.Context, mimeType string, open app.Opener) (imageHeader, error) {
	var h imageHeader
	err := withSource(ctx, open, func(r io.Reader) error {
		switch mimeType {
		case mimeJPEG:
			info, err := scanJPEG(r)
			h = imageHeader{Width: info.Width, Height: info.Height, Frames: 1, Orientation: info.Orientation, Profile: info.ICC}
			return err
		case mimePNG, mimeAPNG:
			info, err := stripPNG(r, io.Discard)
			h = imageHeader{Width: info.Width, Height: info.Height, Frames: 1, Orientation: domain.OrientationNormal, ICCP: info.ICCP}
			return err
		case mimeGIF:
			info, err := scanGIF(r)
			h = imageHeader{Width: info.Width, Height: info.Height, Frames: max(info.Frames, 1), Orientation: domain.OrientationNormal}
			return err
		default:
			return fmt.Errorf("%w: no header reader for the type", app.ErrImageUndecodable)
		}
	})
	if !h.Orientation.Valid() {
		h.Orientation = domain.OrientationNormal
	}
	return h, err
}

// checkPixels is the pixel guard: ErrTooManyPixels when width times height times frames is above
// maxPixels (file.image.max_pixels).
func checkPixels(width, height, frames int, maxPixels int64) error {
	if domain.ExceedsPixelLimit(int64(width), int64(height), int64(frames), maxPixels) {
		return app.ErrTooManyPixels
	}
	return nil
}

// readAll reads the whole original of size bytes into memory (GIF and WebP are decoded and
// encoded in memory, design note 3c); a longer object is a read error.
func readAll(ctx context.Context, open app.Opener, size int64) ([]byte, error) {
	var data []byte
	err := withSource(ctx, open, func(r io.Reader) error {
		var buf bytes.Buffer
		n, err := io.Copy(&buf, io.LimitReader(r, size+1))
		if err != nil {
			return err
		}
		if n != size {
			return errors.New("files/imaging: the original has another size than its row")
		}
		data = buf.Bytes()
		return nil
	})
	return data, err
}
