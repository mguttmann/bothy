// SPDX-License-Identifier: Apache-2.0

package isobmff

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"math/big"

	"github.com/OWNER/bothy/services/server/internal/files/app"
)

// Box layout of ISO/IEC 14496-12 (4.2 box header, 8.2.2 mvhd, 8.3.2 tkhd, 8.4.3 hdlr, 8.8.2
// mehd). These are format constants of the specification, not tuning numbers.
const (
	// headerSize is the compact box header: 32-bit size and four-letter type.
	headerSize = 8
	// largeHeaderSize is the header with size 1 followed by the 64-bit largesize.
	largeHeaderSize = 16
	// sizeLarge in the 32-bit size field announces a 64-bit largesize.
	sizeLarge = 1
	// sizeToEnd in the 32-bit size field means that the box extends to the end of its parent.
	sizeToEnd = 0
	// fullBoxHeader is version (1 byte) and flags (3 bytes) at the start of a full box payload.
	fullBoxHeader = 4

	// mvhdLenV0 and mvhdLenV1 are the payload lengths of mvhd including version and flags.
	mvhdLenV0 = 100
	mvhdLenV1 = 112
	// tkhdLenV0 and tkhdLenV1 are the payload lengths of tkhd including version and flags.
	tkhdLenV0 = 84
	tkhdLenV1 = 96
	// tkhdMatrixV0 and tkhdMatrixV1 are the payload offsets of the 3x3 matrix; width and
	// height follow the 36 matrix bytes as the last 8 bytes of the box.
	tkhdMatrixV0 = 40
	tkhdMatrixV1 = 52
	// hdlrLen is the payload prefix of hdlr that holds the handler type at offset 8.
	hdlrLen = 12
	// mehdLenV0 and mehdLenV1 are the payload lengths of mehd including version and flags.
	mehdLenV0 = 8
	mehdLenV1 = 12

	// fixedOne is 1.0 in the 16.16 fixed point of the tkhd matrix entries a, b, c and d.
	fixedOne = 0x00010000
	// fixedMinusOne is -1.0 in 16.16 fixed point, as the unsigned bits of the field.
	fixedMinusOne = 0xFFFF0000

	// handlerVideo is the hdlr handler type of a video track.
	handlerVideo = "vide"
	// millisPerSecond converts a duration in timescale units per second into milliseconds.
	millisPerSecond = 1000
)

// errMalformed marks a box structure that breaks ISO/IEC 14496-12; every error of the reader
// wraps it or a read error of the io.ReaderAt.
var errMalformed = errors.New("isobmff: malformed box structure")

// MovieInfo is app.MovieInfo: display size, duration in milliseconds and flags for which of
// them were found.
type MovieInfo = app.MovieInfo

// Reader implements app.MovieReader with ReadMovieInfo. The zero value is ready to use.
type Reader struct{}

var _ app.MovieReader = Reader{}

// ReadMovieInfo reads the boxes of an MP4 or QuickTime file through r, whose size is size
// bytes; see the package function ReadMovieInfo for result and errors.
func (Reader) ReadMovieInfo(r io.ReaderAt, size int64) (app.MovieInfo, error) {
	return ReadMovieInfo(r, size)
}

// ReadMovieInfo reads the boxes of an MP4 or QuickTime file through r, whose size is size
// bytes. The result holds the display size of the first video track (swapped for a rotation by
// 90 or 270 degrees) and the movie duration in milliseconds, each with a flag whether it was
// found. Errors: a broken box structure (a child larger than its parent or smaller than its
// header, a truncated small box or header) or a read error of r; any error comes with an
// empty MovieInfo.
func ReadMovieInfo(r io.ReaderAt, size int64) (MovieInfo, error) {
	if size < 0 {
		return MovieInfo{}, fmt.Errorf("%w: negative file size", errMalformed)
	}
	p := &parser{r: r}
	if err := p.walk(0, size, p.topLevel); err != nil {
		return MovieInfo{}, err
	}
	return p.result(), nil
}

// boxHeader is one box: its type and the offsets of its payload and of its end in the file.
type boxHeader struct {
	typ     string
	payload int64
	end     int64
}

// track collects what one trak says: the tkhd size and matrix and whether hdlr marks video.
type track struct {
	width, height int
	rotated       bool
	video         bool
}

// parser holds the reader and what the walk has found so far.
type parser struct {
	r io.ReaderAt

	timescale    uint32
	haveMvhd     bool
	duration     uint64
	haveDuration bool
	fragment     uint64
	haveFragment bool

	videoSeen     bool
	width, height int
}

// walk reads the box headers between start and end one after the other and passes each to
// visit; it skips every box by its offset. Errors: a malformed header, or the error of visit.
func (p *parser) walk(start, end int64, visit func(boxHeader) error) error {
	for off := start; off < end; {
		h, err := p.header(off, end)
		if err != nil {
			return err
		}
		if err := visit(h); err != nil {
			return err
		}
		off = h.end
	}
	return nil
}

// header reads the box header at off inside a parent that ends at end. Errors: fewer bytes
// than a header left in the parent, a size smaller than the header or larger than the rest of
// the parent, or a read error.
func (p *parser) header(off, end int64) (boxHeader, error) {
	rest := end - off
	if rest < headerSize {
		return boxHeader{}, fmt.Errorf("%w: truncated box header at %d", errMalformed, off)
	}
	b, err := readFull(p.r, off, headerSize)
	if err != nil {
		return boxHeader{}, err
	}
	size, hlen := int64(binary.BigEndian.Uint32(b[:4])), int64(headerSize)
	switch size {
	case sizeLarge:
		if size, err = p.largeSize(off, rest); err != nil {
			return boxHeader{}, err
		}
		hlen = largeHeaderSize
	case sizeToEnd:
		size = rest
	}
	if size < hlen {
		return boxHeader{}, fmt.Errorf("%w: box at %d smaller than its header", errMalformed, off)
	}
	if size > rest {
		return boxHeader{}, fmt.Errorf("%w: box at %d larger than its parent", errMalformed, off)
	}
	return boxHeader{typ: string(b[4:8]), payload: off + hlen, end: off + size}, nil
}

// largeSize reads the 64-bit largesize of the box at off, whose parent has rest bytes left.
// Errors: no room for the largesize, a value above the int64 range (necessarily larger than
// its parent), or a read error.
func (p *parser) largeSize(off, rest int64) (int64, error) {
	if rest < largeHeaderSize {
		return 0, fmt.Errorf("%w: truncated largesize at %d", errMalformed, off)
	}
	ext, err := readFull(p.r, off+headerSize, largeHeaderSize-headerSize)
	if err != nil {
		return 0, err
	}
	if ext[0]&0x80 != 0 {
		return 0, fmt.Errorf("%w: box at %d larger than its parent", errMalformed, off)
	}
	// Assembled byte by byte so that no unsigned value is converted to int64.
	var size int64
	for _, c := range ext {
		size = size<<8 | int64(c)
	}
	return size, nil
}

// topLevel descends into moov and skips every other top-level box (ftyp, mdat, free, wide).
func (p *parser) topLevel(h boxHeader) error {
	if h.typ == "moov" {
		return p.walk(h.payload, h.end, p.moov)
	}
	return nil
}

// moov reads mvhd and descends into trak and mvex; other children are skipped. Errors: those
// of the small box reads and of the walks below.
func (p *parser) moov(h boxHeader) error {
	switch h.typ {
	case "mvhd":
		return p.readMvhd(h)
	case "trak":
		var t track
		if err := p.walk(h.payload, h.end, func(c boxHeader) error { return p.trak(c, &t) }); err != nil {
			return err
		}
		if t.video && !p.videoSeen {
			p.videoSeen, p.width, p.height = true, t.width, t.height
			if t.rotated {
				p.width, p.height = t.height, t.width
			}
		}
	case "mvex":
		return p.walk(h.payload, h.end, p.mvex)
	}
	return nil
}

// trak reads tkhd into t and descends into mdia to find its hdlr. Errors: those of the reads.
func (p *parser) trak(h boxHeader, t *track) error {
	switch h.typ {
	case "tkhd":
		return p.readTkhd(h, t)
	case "mdia":
		return p.walk(h.payload, h.end, func(c boxHeader) error { return p.mdia(c, t) })
	}
	return nil
}

// mdia reads hdlr and marks t as video when its handler type is "vide". Errors: a hdlr shorter
// than its handler type field, or a read error.
func (p *parser) mdia(h boxHeader, t *track) error {
	if h.typ != "hdlr" {
		return nil
	}
	_, b, err := p.fullBox(h, hdlrLen)
	if err != nil {
		return err
	}
	if len(b) < hdlrLen {
		return fmt.Errorf("%w: truncated hdlr", errMalformed)
	}
	t.video = string(b[8:12]) == handlerVideo
	return nil
}

// mvex reads mehd, the duration of a fragmented movie; other children are skipped. Errors: a
// truncated mehd, or a read error.
func (p *parser) mvex(h boxHeader) error {
	if h.typ != "mehd" {
		return nil
	}
	version, b, err := p.fullBox(h, mehdLenV1)
	if err != nil {
		return err
	}
	switch {
	case version == 0 && len(b) >= mehdLenV0:
		p.fragment, p.haveFragment = known(uint64(binary.BigEndian.Uint32(b[4:8])), math.MaxUint32)
	case version == 1 && len(b) >= mehdLenV1:
		p.fragment, p.haveFragment = known(binary.BigEndian.Uint64(b[4:12]), math.MaxUint64)
	case version <= 1:
		return fmt.Errorf("%w: truncated mehd", errMalformed)
	}
	return nil
}

// readMvhd reads timescale and duration of the first mvhd; an unknown version is skipped.
// Errors: a truncated mvhd, or a read error.
func (p *parser) readMvhd(h boxHeader) error {
	if p.haveMvhd {
		return nil
	}
	version, b, err := p.fullBox(h, mvhdLenV1)
	if err != nil {
		return err
	}
	switch {
	case version == 0 && len(b) >= mvhdLenV0:
		p.timescale = binary.BigEndian.Uint32(b[12:16])
		p.duration, p.haveDuration = known(uint64(binary.BigEndian.Uint32(b[16:20])), math.MaxUint32)
	case version == 1 && len(b) >= mvhdLenV1:
		p.timescale = binary.BigEndian.Uint32(b[20:24])
		p.duration, p.haveDuration = known(binary.BigEndian.Uint64(b[24:32]), math.MaxUint64)
	case version <= 1:
		return fmt.Errorf("%w: truncated mvhd", errMalformed)
	default:
		return nil
	}
	p.haveMvhd = true
	return nil
}

// readTkhd reads width, height and matrix of a tkhd into t; an unknown version is skipped.
// Errors: a truncated tkhd, or a read error.
func (p *parser) readTkhd(h boxHeader, t *track) error {
	version, b, err := p.fullBox(h, tkhdLenV1)
	if err != nil {
		return err
	}
	var m int
	switch {
	case version == 0 && len(b) >= tkhdLenV0:
		m = tkhdMatrixV0
	case version == 1 && len(b) >= tkhdLenV1:
		m = tkhdMatrixV1
	case version <= 1:
		return fmt.Errorf("%w: truncated tkhd", errMalformed)
	default:
		return nil
	}
	be := binary.BigEndian
	// Matrix {a, b, u, c, d, v, x, y, w}: a turn by 90 or 270 degrees has a = d = 0 and
	// b = -c = +-1.0; mirroring or scaling keeps the size.
	a, bb, c, d := be.Uint32(b[m:]), be.Uint32(b[m+4:]), be.Uint32(b[m+12:]), be.Uint32(b[m+16:])
	t.rotated = a == 0 && d == 0 && ((bb == fixedOne && c == fixedMinusOne) || (bb == fixedMinusOne && c == fixedOne))
	// The integer part of a 16.16 value is its first two bytes.
	t.width = int(b[m+36])<<8 | int(b[m+37])
	t.height = int(b[m+40])<<8 | int(b[m+41])
	return nil
}

// fullBox reads the version and at most maxLen payload bytes of the full box h; the bound is
// the largest known version of that box, so a huge box never causes a huge allocation. The
// returned bytes start with version and flags. Errors: a payload shorter than version and
// flags, or a read error.
func (p *parser) fullBox(h boxHeader, maxLen int) (byte, []byte, error) {
	if h.end-h.payload < fullBoxHeader {
		return 0, nil, fmt.Errorf("%w: truncated %s", errMalformed, h.typ)
	}
	buf := make([]byte, maxLen)
	n, err := io.NewSectionReader(p.r, h.payload, h.end-h.payload).ReadAt(buf, 0)
	// A box shorter than maxLen ends the section early; only a shortfall inside the box is a
	// read error.
	if int64(n) < min(h.end-h.payload, int64(maxLen)) {
		if err == nil || errors.Is(err, io.EOF) {
			err = io.ErrUnexpectedEOF
		}
		return 0, nil, fmt.Errorf("isobmff: read %s at %d: %w", h.typ, h.payload, err)
	}
	return buf[0], buf[:n], nil
}

// result turns what the walk found into MovieInfo: the size of the first video track when it
// is not zero, and the mvhd duration or else the mehd duration in milliseconds.
func (p *parser) result() MovieInfo {
	var info MovieInfo
	if p.videoSeen && p.width != 0 && p.height != 0 {
		info.Width, info.Height, info.HasSize = p.width, p.height, true
	}
	switch {
	case p.haveDuration:
		info.DurationMS, info.HasDuration = millis(p.duration, p.timescale)
	case p.haveFragment:
		info.DurationMS, info.HasDuration = millis(p.fragment, p.timescale)
	}
	return info
}

// known reports v and true unless v is 0 or all bits set (allOnes), which ISO/IEC 14496-12
// uses for an unknown duration.
func known(v, allOnes uint64) (uint64, bool) {
	if v == 0 || v == allOnes {
		return 0, false
	}
	return v, true
}

// millis converts duration in timescale units into whole milliseconds, rounded down; false
// when timescale is 0 or the result does not fit into an int64. math/big keeps the product
// exact for every 64-bit duration.
func millis(duration uint64, timescale uint32) (int64, bool) {
	if timescale == 0 {
		return 0, false
	}
	q := new(big.Int).SetUint64(duration)
	q.Mul(q, big.NewInt(millisPerSecond))
	q.Quo(q, new(big.Int).SetUint64(uint64(timescale)))
	if !q.IsInt64() {
		return 0, false
	}
	return q.Int64(), true
}

// readFull reads exactly n bytes at off. Errors: a read error of r, or io.ErrUnexpectedEOF when
// r ends early.
func readFull(r io.ReaderAt, off int64, n int) ([]byte, error) {
	buf := make([]byte, n)
	got, err := r.ReadAt(buf, off)
	if got == n {
		return buf, nil
	}
	if err == nil || errors.Is(err, io.EOF) {
		err = io.ErrUnexpectedEOF
	}
	return nil, fmt.Errorf("isobmff: read %d bytes at %d: %w", n, off, err)
}
