// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"errors"
	"io"

	"github.com/OWNER/bothy/services/server/internal/files/domain"
)

// The errors of an ImageCodec (brief M3-06 design note 2). Every other error of the codec is a
// read error of the store and transient.
var (
	// ErrImageMalformed: the structure of a JPEG or PNG is broken; the file fails with
	// image_malformed (12 7.1, ADR 0047 point 3).
	ErrImageMalformed = errors.New("files: malformed image")
	// ErrTooManyPixels: width times height (times frames) is above file.image.max_pixels; nothing
	// was decoded (12 7.2, SEC-UP-4).
	ErrTooManyPixels = errors.New("files: image above the pixel limit")
	// ErrImageUndecodable: the image could not be decoded, or its colour profile does not fit
	// into a thumbnail; there are no thumbnails and no retry.
	ErrImageUndecodable = errors.New("files: image cannot be decoded")
	// ErrImageAnimated: an animated WebP, stored unchanged (12 7.1).
	ErrImageAnimated = errors.New("files: animated image is stored unchanged")
)

// Opener opens the stored original anew for every pass; the caller closes the reader.
type Opener func(ctx context.Context) (io.ReadCloser, error)

// CleanLimits are the configuration values the cleaning of an original needs.
type CleanLimits struct {
	// MaxPixels is file.image.max_pixels; a GIF or WebP above it is not decoded.
	MaxPixels int64
	// JPEGQuality is file.image.jpeg_quality, the quality of a still WebP stored as JPEG.
	JPEGQuality int
}

// CleanedOriginal is the cleaned original of a plan: its size, SHA-256 and media type, its
// display size, and Body, which yields the cleaned bytes again on every call (the filter of a
// JPEG or PNG runs again over a new Opener reader, design note 3c).
type CleanedOriginal struct {
	// Size and SHA256 describe the bytes Body yields.
	Size   int64
	SHA256 [32]byte
	// MIMEType is the media type of the cleaned bytes (image/png or image/jpeg for a WebP).
	MIMEType string
	// Width and Height are the display size after the orientation.
	Width, Height int
	// Body yields the cleaned bytes; the caller closes the reader.
	Body Opener
}

// ImageInfo is what Inspect reads from the header of an image without decoding pixels.
type ImageInfo struct {
	// Width and Height are the stored size (before the orientation).
	Width, Height int
	// Frames is the number of frames (1 for a still image).
	Frames int
	// Orientation is the EXIF orientation that applies to the thumbnails (JPEG only, else 1).
	Orientation domain.Orientation
	// HasProfile reports an embedded ICC profile.
	HasProfile bool
}

// VariantSpec is one variant to render with its longest edge.
type VariantSpec struct {
	Variant     domain.Variant
	LongestEdge int
}

// RenderSpec is the input of RenderVariants besides the original.
type RenderSpec struct {
	// MIMEType is the media type of the stored original (image/jpeg, image/png, image/apng or
	// image/gif).
	MIMEType string
	// Variants are rendered in this order.
	Variants []VariantSpec
	// JPEGQuality is file.thumbnail.jpeg_quality.
	JPEGQuality int
	// MaxPixels is file.image.max_pixels.
	MaxPixels int64
}

// RenderedVariant is one encoded thumbnail.
type RenderedVariant struct {
	Variant  domain.Variant
	MIMEType string
	Width    int
	Height   int
	Data     []byte
}

// ImageCodec is the outgoing port of the image processing (12 7, ADR 0047, brief M3-06 design
// note 2); adapters/imaging implements it in pure Go. The use case sees only io, plain values and
// the errors above; image types stay in the adapter. Implementations are safe for concurrent use.
type ImageCodec interface {
	// CleanOriginal cleans the original of plan (not PlanNone) read through open, whose object
	// has size bytes. JPEG and PNG are filtered without decoding pixels (ErrImageMalformed on a
	// broken structure); GIF and still WebP are decoded and encoded in memory (ErrTooManyPixels
	// above lim.MaxPixels, ErrImageUndecodable, ErrImageAnimated for an animated WebP).
	CleanOriginal(ctx context.Context, plan domain.OriginalPlan, open Opener, size int64, lim CleanLimits) (CleanedOriginal, error)
	// Inspect reads header, size, frames, orientation and profile of an image of mimeType
	// without decoding pixels; ErrImageMalformed when the header cannot be read.
	Inspect(ctx context.Context, mimeType string, open Opener) (ImageInfo, error)
	// RenderVariants decodes the original (the first frame of an animation), applies the
	// orientation and renders spec.Variants. Errors: ErrTooManyPixels before any decode,
	// ErrImageUndecodable, or a read error of open.
	RenderVariants(ctx context.Context, open Opener, spec RenderSpec) ([]RenderedVariant, error)
}

// MovieInfo is what MovieReader reads from an MP4 or QuickTime file: display size and duration,
// each with a flag whether it was found.
type MovieInfo struct {
	Width, Height int
	HasSize       bool
	DurationMS    int64
	HasDuration   bool
}

// MovieReader is the outgoing port of the box reader (12 7.3); adapters/isobmff implements it.
type MovieReader interface {
	// ReadMovieInfo reads the boxes mvhd and tkhd of the file r of size bytes. An error means
	// that nothing is known; it never blocks the file.
	ReadMovieInfo(r io.ReaderAt, size int64) (MovieInfo, error)
}
