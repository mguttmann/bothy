// SPDX-License-Identifier: Apache-2.0

package imaging

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"image"
	"image/gif"
	"image/jpeg"
	"image/png"
	"io"

	"golang.org/x/image/webp"

	"github.com/OWNER/bothy/services/server/internal/files/app"
	"github.com/OWNER/bothy/services/server/internal/files/domain"
)

// decodeFunc decodes one image (the first frame of an animation) of mimeType from r.
type decodeFunc func(mimeType string, r io.Reader) (image.Image, error)

// Codec implements app.ImageCodec in pure Go (ADR 0047, brief M3-06 design notes 4 to 9): the
// lossless JPEG and PNG filters, the GIF re-encoding, the WebP conversion and the thumbnails. The
// zero value is ready; it is safe for concurrent use. The fields exist for the tests of this
// package, which count the decodes behind the pixel guard (SEC-T08).
//
// Example:
//
//	var c imaging.Codec
//	cleaned, err := c.CleanOriginal(ctx, domain.PlanStripJPEG, open, size, app.CleanLimits{...})
type Codec struct {
	// decode replaces decodeImage; nil means decodeImage.
	decode decodeFunc
	// reencode replaces reencodeGIF; nil means reencodeGIF.
	reencode func(data []byte) ([]byte, error)
}

// Compile-time check: Codec is the outgoing port of app.
var _ app.ImageCodec = Codec{}

// decodeImage decodes the first frame or default image with the standard decoders and
// golang.org/x/image/webp.
func decodeImage(mimeType string, r io.Reader) (image.Image, error) {
	switch mimeType {
	case mimeJPEG:
		return jpeg.Decode(r)
	case mimePNG, mimeAPNG:
		return png.Decode(r)
	case mimeGIF:
		return gif.Decode(r)
	case mimeWebP:
		return webp.Decode(r)
	default:
		return nil, fmt.Errorf("%w: no decoder for the type", app.ErrImageUndecodable)
	}
}

// decoder returns the decode function in use.
func (c Codec) decoder() decodeFunc {
	if c.decode != nil {
		return c.decode
	}
	return decodeImage
}

// CleanOriginal implements app.ImageCodec.
func (c Codec) CleanOriginal(ctx context.Context, plan domain.OriginalPlan, open app.Opener, size int64, lim app.CleanLimits) (app.CleanedOriginal, error) {
	switch plan {
	case domain.PlanStripJPEG:
		return c.cleanJPEG(ctx, open)
	case domain.PlanStripPNG:
		return c.cleanPNG(ctx, open)
	case domain.PlanReencodeGIF:
		return c.cleanGIF(ctx, open, size, lim)
	case domain.PlanConvertWebP:
		return c.convertWebP(ctx, open, size, lim)
	case domain.PlanNone:
		return app.CleanedOriginal{}, fmt.Errorf("%w: no plan", app.ErrImageUndecodable)
	default:
		return app.CleanedOriginal{}, fmt.Errorf("%w: no plan", app.ErrImageUndecodable)
	}
}

// digestWriter counts and hashes what the filter writes in the first pass (no output kept).
type digestWriter struct {
	n int64
	h interface {
		io.Writer
		Sum([]byte) []byte
	}
}

// Write implements io.Writer.
func (d *digestWriter) Write(b []byte) (int, error) {
	d.n += int64(len(b))
	return d.h.Write(b)
}

// sum returns the SHA-256 of what was written.
func (d *digestWriter) sum() [32]byte {
	var out [32]byte
	copy(out[:], d.h.Sum(nil))
	return out
}

// filterBody returns an Opener that streams the filter output of a new read of open through a
// pipe (the second pass of design note 3c).
func filterBody(open app.Opener, filter func(r io.Reader, w io.Writer) error) app.Opener {
	return func(ctx context.Context) (io.ReadCloser, error) {
		src, err := open(ctx)
		if err != nil {
			return nil, err
		}
		pr, pw := io.Pipe()
		go func() {
			err := filter(src, pw)
			_ = src.Close()
			_ = pw.CloseWithError(err)
		}()
		return pr, nil
	}
}

// cleanJPEG scans the structure (orientation and APP0 decide the output order), then runs the
// segment filter once into the digest; Body runs it again (design notes 3c and 4).
func (c Codec) cleanJPEG(ctx context.Context, open app.Opener) (app.CleanedOriginal, error) {
	var info jpegInfo
	if err := withSource(ctx, open, func(r io.Reader) (err error) {
		info, err = scanJPEG(r)
		return err
	}); err != nil {
		return app.CleanedOriginal{}, err
	}
	filter := func(r io.Reader, w io.Writer) error { return stripJPEG(r, w, info) }
	d := &digestWriter{h: sha256.New()}
	if err := withSource(ctx, open, func(r io.Reader) error { return filter(r, d) }); err != nil {
		return app.CleanedOriginal{}, err
	}
	o := info.Orientation
	if !o.Valid() {
		o = domain.OrientationNormal
	}
	w, h := o.DisplaySize(info.Width, info.Height)
	return app.CleanedOriginal{Size: d.n, SHA256: d.sum(), MIMEType: outJPEG, Width: w, Height: h, Body: filterBody(open, filter)}, nil
}

// cleanPNG runs the chunk filter once into the digest; Body runs it again (design notes 3c and 6).
func (c Codec) cleanPNG(ctx context.Context, open app.Opener) (app.CleanedOriginal, error) {
	var info pngInfo
	d := &digestWriter{h: sha256.New()}
	if err := withSource(ctx, open, func(r io.Reader) (err error) {
		info, err = stripPNG(r, d)
		return err
	}); err != nil {
		return app.CleanedOriginal{}, err
	}
	filter := func(r io.Reader, w io.Writer) error {
		_, err := stripPNG(r, w)
		return err
	}
	return app.CleanedOriginal{Size: d.n, SHA256: d.sum(), MIMEType: outPNG, Width: info.Width, Height: info.Height,
		Body: filterBody(open, filter)}, nil
}

// inMemory returns a cleaned original held in memory.
func inMemory(b []byte, mimeType string, w, h int) app.CleanedOriginal {
	return app.CleanedOriginal{Size: int64(len(b)), SHA256: sha256.Sum256(b), MIMEType: mimeType, Width: w, Height: h,
		Body: func(context.Context) (io.ReadCloser, error) { return io.NopCloser(bytes.NewReader(b)), nil }}
}

// cleanGIF re-encodes a GIF within the pixel limit in memory (design note 7); the block scan
// counts the frames first, so a GIF above file.image.max_pixels is never decoded.
func (c Codec) cleanGIF(ctx context.Context, open app.Opener, size int64, lim app.CleanLimits) (app.CleanedOriginal, error) {
	data, err := readAll(ctx, open, size)
	if err != nil {
		return app.CleanedOriginal{}, err
	}
	info, err := scanGIF(bytes.NewReader(data))
	if err != nil {
		return app.CleanedOriginal{}, err
	}
	if err := checkPixels(info.Width, info.Height, max(info.Frames, 1), lim.MaxPixels); err != nil {
		return app.CleanedOriginal{}, err
	}
	reencode := c.reencode
	if reencode == nil {
		reencode = reencodeGIF
	}
	out, err := reencode(data)
	if err != nil {
		return app.CleanedOriginal{}, err
	}
	return inMemory(out, mimeGIF, info.Width, info.Height), nil
}

// convertWebP decodes a still WebP within the pixel limit, applies its EXIF orientation to the
// pixels and stores it as PNG (alpha) or JPEG with file.image.jpeg_quality, with its ICC profile
// and without any other metadata (design note 7, D-M3-06-5). An animated WebP is
// ErrImageAnimated.
func (c Codec) convertWebP(ctx context.Context, open app.Opener, size int64, lim app.CleanLimits) (app.CleanedOriginal, error) {
	if lim.JPEGQuality < minJPEGQuality || lim.JPEGQuality > maxJPEGQuality {
		return app.CleanedOriginal{}, errBadQuality
	}
	data, err := readAll(ctx, open, size)
	if err != nil {
		return app.CleanedOriginal{}, err
	}
	info, err := scanWebP(data)
	if err != nil {
		return app.CleanedOriginal{}, err
	}
	if info.Animated {
		return app.CleanedOriginal{}, app.ErrImageAnimated
	}
	if err := checkPixels(info.Width, info.Height, 1, lim.MaxPixels); err != nil {
		return app.CleanedOriginal{}, err
	}
	img, err := c.decoder()(mimeWebP, bytes.NewReader(data))
	if err != nil {
		return app.CleanedOriginal{}, fmt.Errorf("%w: %w", app.ErrImageUndecodable, err)
	}
	b := img.Bounds()
	if b.Dx() < 1 || b.Dy() < 1 {
		return app.CleanedOriginal{}, errNoImage
	}
	o := orientationFromTIFF(info.EXIF)
	k := kindOf(img)
	upright := orient(scaleTo(img, k, b.Dx(), b.Dy()), k, o)
	sp := jpegProfile(info.ICC)
	out, mimeType, err := encodeCanvas(upright, k, lim.JPEGQuality, &sp)
	if err != nil {
		return app.CleanedOriginal{}, err
	}
	ub := upright.Bounds()
	return inMemory(out, mimeType, ub.Dx(), ub.Dy()), nil
}

// minJPEGQuality and maxJPEGQuality are Go's JPEG quality range (image/jpeg Options).
const (
	minJPEGQuality = 1
	maxJPEGQuality = 100
)

// Inspect implements app.ImageCodec: header, size, frames, orientation and profile without
// decoding pixels.
func (c Codec) Inspect(ctx context.Context, mimeType string, open app.Opener) (app.ImageInfo, error) {
	if mimeType == mimeWebP {
		var info webpInfo
		err := withSource(ctx, open, func(r io.Reader) error {
			data, err := io.ReadAll(r)
			if err != nil {
				return err
			}
			info, err = scanWebP(data)
			return err
		})
		if err != nil {
			return app.ImageInfo{}, err
		}
		return app.ImageInfo{Width: info.Width, Height: info.Height, Frames: 1, Orientation: domain.OrientationNormal,
			HasProfile: len(info.ICC) > 0}, nil
	}
	h, err := readHeader(ctx, mimeType, open)
	if err != nil {
		return app.ImageInfo{}, err
	}
	return app.ImageInfo{Width: h.Width, Height: h.Height, Frames: h.Frames, Orientation: h.Orientation,
		HasProfile: len(h.Profile) > 0 || len(h.ICCP) > 0}, nil
}

// decodeOriginal decodes the first frame or default image of the original once; a decoder error
// is ErrImageUndecodable, a read error of the store stays itself.
func (c Codec) decodeOriginal(ctx context.Context, mimeType string, open app.Opener) (image.Image, error) {
	var img image.Image
	err := withSource(ctx, open, func(r io.Reader) error {
		var derr error
		img, derr = c.decoder()(mimeType, r)
		if derr != nil {
			return fmt.Errorf("%w: %w", app.ErrImageUndecodable, derr)
		}
		return nil
	})
	return img, err
}

// RenderVariants implements app.ImageCodec (design note 9): the header and the pixel guard first,
// then one decode of the first frame, the orientation of a JPEG, CatmullRom to each variant, PNG
// for alpha and JPEG otherwise, with the colour profile of the original when it fits.
func (c Codec) RenderVariants(ctx context.Context, open app.Opener, spec app.RenderSpec) ([]app.RenderedVariant, error) {
	if spec.JPEGQuality < minJPEGQuality || spec.JPEGQuality > maxJPEGQuality {
		return nil, errBadQuality
	}
	h, err := readHeader(ctx, spec.MIMEType, open)
	if err != nil {
		return nil, err
	}
	if err := checkPixels(h.Width, h.Height, h.Frames, spec.MaxPixels); err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	img, err := c.decodeOriginal(ctx, spec.MIMEType, open)
	if err != nil {
		return nil, err
	}
	sp := jpegProfile(h.Profile)
	if h.ICCP != nil {
		sp = pngProfile(h.ICCP)
	}
	k := kindOf(img)
	out := make([]app.RenderedVariant, 0, len(spec.Variants))
	for _, v := range spec.Variants {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		canvas, err := renderVariant(img, k, h.Orientation, v.LongestEdge)
		if err != nil {
			return nil, err
		}
		data, mimeType, err := encodeCanvas(canvas, k, spec.JPEGQuality, &sp)
		if err != nil {
			return nil, err
		}
		b := canvas.Bounds()
		out = append(out, app.RenderedVariant{Variant: v.Variant, MIMEType: mimeType, Width: b.Dx(), Height: b.Dy(), Data: data})
	}
	return out, nil
}
