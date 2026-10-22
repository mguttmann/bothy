// SPDX-License-Identifier: Apache-2.0

package imaging

import (
	"bytes"
	"compress/zlib"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"io"

	"golang.org/x/image/draw"

	"github.com/OWNER/bothy/services/server/internal/files/app"
	"github.com/OWNER/bothy/services/server/internal/files/domain"
)

// The ICC colour spaces a thumbnail can carry (header field "data colour space").
const (
	colourSpaceRGB  = "RGB "
	colourSpaceGray = "GRAY"
)

// The media types of the encoded thumbnails and converted originals.
const (
	outJPEG = mimeJPEG
	outPNG  = mimePNG
)

// errNoImage reports an image without pixels.
var errNoImage = fmt.Errorf("%w: image without pixels", app.ErrImageUndecodable)

// canvasKind is the pixel model of a rendered image: gray, RGB or RGB with alpha.
type canvasKind uint8

const (
	canvasRGB canvasKind = iota
	canvasGray
	canvasAlpha
)

// opaque reports whether every pixel of img is opaque (12 7.3 alpha rule, D-M3-06-6). Every image
// type the decoders return has the method Opaque; a type without it counts as not opaque, so its
// alpha is never lost.
func opaque(img image.Image) bool {
	if o, ok := img.(interface{ Opaque() bool }); ok {
		return o.Opaque()
	}
	return false
}

// kindOf returns the canvas of a decoded image: alpha when it is not opaque, gray for a gray
// image, RGB otherwise.
func kindOf(img image.Image) canvasKind {
	if !opaque(img) {
		return canvasAlpha
	}
	switch img.ColorModel() {
	case color.GrayModel, color.Gray16Model:
		return canvasGray
	default:
		return canvasRGB
	}
}

// colourSpaceOf is the ICC colour space a profile needs to fit the canvas k.
func colourSpaceOf(k canvasKind) string {
	if k == canvasGray {
		return colourSpaceGray
	}
	return colourSpaceRGB
}

// newCanvas returns an empty image of kind k and size w x h.
func newCanvas(k canvasKind, w, h int) draw.Image {
	r := image.Rect(0, 0, w, h)
	switch k {
	case canvasGray:
		return image.NewGray(r)
	case canvasAlpha:
		return image.NewNRGBA(r)
	case canvasRGB:
		return image.NewRGBA(r)
	default:
		return image.NewRGBA(r)
	}
}

// scaleTo draws src into a new canvas of kind k and size w x h, with the resampler CatmullRom
// when the size changes (D-M3-06-6) and a plain copy otherwise.
func scaleTo(src image.Image, k canvasKind, w, h int) draw.Image {
	dst := newCanvas(k, w, h)
	b := src.Bounds()
	if b.Dx() == w && b.Dy() == h {
		draw.Draw(dst, dst.Bounds(), src, b.Min, draw.Src)
		return dst
	}
	draw.CatmullRom.Scale(dst, dst.Bounds(), src, b, draw.Src, nil)
	return dst
}

// pixelBuffer returns the pixel bytes, stride and bytes per pixel of a canvas of newCanvas.
func pixelBuffer(img draw.Image) (pix []byte, stride, bpp int) {
	switch m := img.(type) {
	case *image.Gray:
		return m.Pix, m.Stride, 1
	case *image.NRGBA:
		return m.Pix, m.Stride, 4
	case *image.RGBA:
		return m.Pix, m.Stride, 4
	default:
		return nil, 0, 0
	}
}

// orient applies the EXIF orientation o to a canvas of kind k (TIFF 6.0 tag 274): the values 2
// to 8 mirror and turn the stored pixels so the result shows the image upright.
func orient(img draw.Image, k canvasKind, o domain.Orientation) draw.Image {
	if !o.Valid() || o == domain.OrientationNormal {
		return img
	}
	w, h := img.Bounds().Dx(), img.Bounds().Dy()
	ow, oh := o.DisplaySize(w, h)
	out := newCanvas(k, ow, oh)
	src, sstride, bpp := pixelBuffer(img)
	dst, dstride, _ := pixelBuffer(out)
	for y := range h {
		for x := range w {
			dx, dy := orientedPoint(o, x, y, w, h)
			copy(dst[dy*dstride+dx*bpp:dy*dstride+dx*bpp+bpp], src[y*sstride+x*bpp:y*sstride+x*bpp+bpp])
		}
	}
	return out
}

// orientedPoint maps the stored pixel (x, y) of a w x h image to its place in the upright image.
func orientedPoint(o domain.Orientation, x, y, w, h int) (int, int) {
	switch o {
	case 2: // mirrored horizontally
		return w - 1 - x, y
	case 3: // turned by 180 degrees
		return w - 1 - x, h - 1 - y
	case 4: // mirrored vertically
		return x, h - 1 - y
	case 5: // transposed
		return y, x
	case 6: // turned by 90 degrees clockwise for display
		return h - 1 - y, x
	case 7: // transversed
		return h - 1 - y, w - 1 - x
	case 8: // turned by 90 degrees counter-clockwise for display
		return y, w - 1 - x
	case domain.OrientationNormal:
		return x, y
	default:
		return x, y
	}
}

// renderVariant scales the decoded image to the variant with the longest edge edge after the
// orientation and returns the upright canvas. The scaling runs before the turn, so only the small
// image is turned.
func renderVariant(src image.Image, k canvasKind, o domain.Orientation, edge int) (draw.Image, error) {
	b := src.Bounds()
	if b.Dx() < 1 || b.Dy() < 1 {
		return nil, errNoImage
	}
	ow, oh := o.DisplaySize(b.Dx(), b.Dy())
	vw, vh := domain.VariantSize(ow, oh, edge)
	sw, sh := vw, vh
	if o >= 5 && o <= 8 {
		sw, sh = vh, vw
	}
	return orient(scaleTo(src, k, sw, sh), k, o), nil
}

// sourceProfile is the colour profile of an original: raw is the ICC profile (of a JPEG, or of a
// PNG once decompressed), iccp the data of the iCCP chunk of a PNG, space the data colour space.
type sourceProfile struct {
	raw   []byte
	iccp  []byte
	space string
}

// maxJPEGProfile is the capacity of APP2 ICC_PROFILE segments (ICC.1 Annex B), the most a PNG
// profile is decompressed for a JPEG thumbnail (design note 9).
const maxJPEGProfile = maxICCParts * iccPartPayload

// iccProfileHeaderLen is the length of the ICC profile header (ICC.1 7.2).
const iccProfileHeaderLen = 128

// pngProfile returns the profile of the iCCP data of a PNG: the chunk data and the colour space
// read from the decompressed header; an unreadable chunk gives no profile.
func pngProfile(iccp []byte) sourceProfile {
	if len(iccp) == 0 {
		return sourceProfile{}
	}
	i := bytes.IndexByte(iccp, 0)
	if i < 0 || i+2 > len(iccp) {
		return sourceProfile{}
	}
	zr, err := zlib.NewReader(bytes.NewReader(iccp[i+2:]))
	if err != nil {
		return sourceProfile{}
	}
	defer func() { _ = zr.Close() }()
	hdr := make([]byte, iccProfileHeaderLen)
	if _, err := io.ReadFull(zr, hdr); err != nil {
		return sourceProfile{}
	}
	return sourceProfile{iccp: iccp, space: profileColourSpace(hdr)}
}

// jpegProfile returns the profile of a JPEG source.
func jpegProfile(raw []byte) sourceProfile {
	if len(raw) == 0 {
		return sourceProfile{}
	}
	return sourceProfile{raw: raw, space: profileColourSpace(raw)}
}

// forJPEG returns the raw profile for a JPEG output, decompressing a PNG profile up to the APP2
// capacity; a larger or broken profile is ErrImageUndecodable (no thumbnails, design note 9).
func (sp *sourceProfile) forJPEG() ([]byte, error) {
	if sp.raw != nil || sp.iccp == nil {
		return sp.raw, nil
	}
	raw, err := iccpProfile(sp.iccp, maxJPEGProfile)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", app.ErrImageUndecodable, err)
	}
	sp.raw = raw
	return raw, nil
}

// forPNG returns the iCCP data for a PNG output: the chunk of a PNG source byte for byte, or the
// raw profile compressed.
func (sp *sourceProfile) forPNG() ([]byte, error) {
	if sp.iccp != nil || sp.raw == nil {
		return sp.iccp, nil
	}
	data, err := iccpChunkData(sp.raw)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", app.ErrImageUndecodable, err)
	}
	sp.iccp = data
	return data, nil
}

// encodeJPEG encodes img as JPEG with quality and inserts the APP2 ICC_PROFILE segments of
// profile directly after SOI. Nothing else is written: Go's encoder writes no APP0, APP1 or COM.
func encodeJPEG(img image.Image, quality int, profile []byte) ([]byte, error) {
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: quality}); err != nil {
		return nil, fmt.Errorf("%w: %w", app.ErrImageUndecodable, err)
	}
	if len(profile) == 0 {
		return buf.Bytes(), nil
	}
	segs, err := jpegICCSegments(profile)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", app.ErrImageUndecodable, err)
	}
	enc := buf.Bytes()
	out := make([]byte, 0, len(enc)+len(profile)+len(segs)*iccSegmentOverhead)
	out = append(out, enc[:2]...) // SOI
	for _, s := range segs {
		out = append(out, s...)
	}
	return append(out, enc[2:]...), nil
}

// iccSegmentOverhead is the size of an APP2 ICC_PROFILE segment without its payload: marker,
// length, identifier, sequence number and count (ICC.1 Annex B).
const iccSegmentOverhead = 2 + 2 + 12 + 2

// pngHeaderEnd is the end of the signature and the IHDR chunk (8 + 4 + 4 + 13 + 4 bytes).
const pngHeaderEnd = 33

// encodePNG encodes img as PNG and inserts iccp as the data of an iCCP chunk directly after
// IHDR. Go's encoder writes no text, time or EXIF chunk.
func encodePNG(img image.Image, iccp []byte) ([]byte, error) {
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, fmt.Errorf("%w: %w", app.ErrImageUndecodable, err)
	}
	if len(iccp) == 0 {
		return buf.Bytes(), nil
	}
	enc := buf.Bytes()
	chunk := pngChunk("iCCP", iccp)
	out := make([]byte, 0, len(enc)+len(chunk))
	out = append(out, enc[:pngHeaderEnd]...)
	out = append(out, chunk...)
	return append(out, enc[pngHeaderEnd:]...), nil
}

// encodeCanvas encodes a rendered canvas: PNG when it has alpha, JPEG with quality otherwise
// (12 7.3, D-M3-06-6), with the profile of the source when its colour space fits the canvas
// (CMYK and other spaces are left out, D-M3-06-6).
func encodeCanvas(img image.Image, k canvasKind, quality int, sp *sourceProfile) ([]byte, string, error) {
	fits := sp != nil && sp.space != "" && sp.space == colourSpaceOf(k)
	if k == canvasAlpha {
		var iccp []byte
		if fits {
			var err error
			if iccp, err = sp.forPNG(); err != nil {
				return nil, "", err
			}
		}
		b, err := encodePNG(img, iccp)
		return b, outPNG, err
	}
	var raw []byte
	if fits {
		var err error
		if raw, err = sp.forJPEG(); err != nil {
			return nil, "", err
		}
	}
	b, err := encodeJPEG(img, quality, raw)
	return b, outJPEG, err
}

// errBadQuality reports a JPEG quality outside Go's range.
var errBadQuality = errors.New("files/imaging: JPEG quality outside 1 to 100")
