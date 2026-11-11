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

// gifFrameOverhead is what the pixel guard counts for every frame of a GIF on top of its pixels
// (12 7.2): what image/gif keeps per decoded frame besides the pixels, in bytes, which is the
// size of as many paletted pixels. That is a palette of up to 256 colours, each an interface
// value of 16 bytes and a colour of 4 bytes (a frame with a transparency index gets its own
// copy of the palette, enlarged to the index when the index lies outside it), and 256 bytes
// for the frame record. Without it a file of many frames of one pixel passes
// file.image.max_pixels and keeps thousands of times its pixels in memory.
const gifFrameOverhead = 256*(16+4) + 256

// checkPixels is the pixel guard (12 7.2): ErrTooManyPixels when width times height times frames
// is above maxPixels (file.image.max_pixels) or an edge is longer than domain.MaxImageEdge.
func checkPixels(width, height, frames int, maxPixels int64) error {
	if domain.ExceedsEdgeLimit(int64(width), int64(height)) ||
		domain.ExceedsPixelLimit(int64(width), int64(height), int64(frames), maxPixels) {
		return app.ErrTooManyPixels
	}
	return nil
}

// checkGIFFrames is the pixel guard of a GIF that is decoded with all its frames (12 7.2):
// ErrTooManyPixels when the frames, each counted with the pixels of the logical screen plus
// gifFrameOverhead, are above maxPixels. The screen of a GIF is at most 65535 pixels wide and
// high, so the sum of one frame cannot overflow.
func checkGIFFrames(info gifInfo, maxPixels int64) error {
	perFrame := int64(info.Width)*int64(info.Height) + gifFrameOverhead
	if maxPixels < 0 || int64(max(info.Frames, 1)) > maxPixels/perFrame {
		return app.ErrTooManyPixels
	}
	return nil
}

// errOriginalSize reports an original that holds more or fewer bytes than its row says. It is a
// read error of the store, never an image error: the job retries and stores nothing.
var errOriginalSize = errors.New("files/imaging: the original has another size than its row")

// readAll reads the whole original of size bytes into memory (GIF and WebP are decoded, encoded
// and inspected in memory, design note 3c). The row size is the bound of that memory: at most
// size bytes and one more are read, whatever the store answers, and an object of another length
// is errOriginalSize.
func readAll(ctx context.Context, open app.Opener, size int64) ([]byte, error) {
	var data []byte
	err := withSource(ctx, open, func(r io.Reader) error {
		var buf bytes.Buffer
		n, err := io.Copy(&buf, io.LimitReader(r, size+1))
		if err != nil {
			return err
		}
		if n != size {
			return errOriginalSize
		}
		data = buf.Bytes()
		return nil
	})
	return data, err
}
