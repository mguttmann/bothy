// SPDX-License-Identifier: Apache-2.0

package imaging

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"

	"github.com/OWNER/bothy/services/server/internal/files/app"
	"github.com/OWNER/bothy/services/server/internal/files/domain"
)

// The JPEG markers the filter treats by name (ITU-T T.81 table B.1); every marker is the byte
// after markerPrefix.
const (
	markerPrefix byte = 0xFF
	// markerStuff after 0xFF inside entropy-coded data is a stuffed data byte 0xFF (T.81 B.1.1.5).
	markerStuff byte = 0x00
	markerTEM   byte = 0x01
	markerDHT   byte = 0xC4
	markerJPG   byte = 0xC8
	markerDAC   byte = 0xCC
	markerRST0  byte = 0xD0
	markerRST7  byte = 0xD7
	markerSOI   byte = 0xD8
	markerEOI   byte = 0xD9
	markerSOS   byte = 0xDA
	markerDHP   byte = 0xDE
	markerAPP0  byte = 0xE0
	markerAPP1  byte = 0xE1
	markerAPP2  byte = 0xE2
	markerAPP14 byte = 0xEE
	markerSOF0  byte = 0xC0
	markerSOF15 byte = 0xCF
)

// The identifiers of the application segments the filter keeps or reads (JFIF 1.02, EXIF 2.32
// section 4.7.2, ICC.1:2022 Annex B.4, Adobe technical note 5116).
const (
	jfifIdentifier  = "JFIF\x00"
	exifIdentifier  = "Exif\x00\x00"
	iccIdentifier   = "ICC_PROFILE\x00"
	adobeIdentifier = "Adobe"
)

// The sizes of JPEG structures the filter checks.
const (
	// segmentLengthSize is the length field of a marker segment, which counts itself (T.81
	// B.1.1.4); a segment is at most 65535 bytes after the marker.
	segmentLengthSize = 2
	// maxSegmentPayload is the largest payload after the length field.
	maxSegmentPayload = 65535 - segmentLengthSize
	// jfifPayloadLen is a JFIF APP0 payload without thumbnail: identifier, version (2), units,
	// densities (2 x 2) and thumbnail width and height (JFIF 1.02).
	jfifPayloadLen = 14
	// iccHeaderLen is identifier, sequence number and part count of an APP2 ICC_PROFILE part.
	iccHeaderLen = 14
	// frameHeaderMin is the start of a frame header: precision, height (2), width (2) and the
	// number of components (T.81 B.2.2).
	frameHeaderMin = 6
)

// The reasons of the structure errors that occur at several places.
const (
	reasonNoSOI     = "missing SOI"
	reasonNoEOI     = "end of input before EOI"
	reasonBeyondEnd = "segment length beyond the input"
)

// jfifHeader is the content of the first APP0 JFIF segment that the filter rewrites.
type jfifHeader struct {
	Major, Minor, Units byte
	XDensity, YDensity  uint16
}

// jpegInfo are the facts of one JPEG.
type jpegInfo struct {
	Width, Height int                // from the (first) frame header SOFn
	Components    int                // number of components of the frame header
	Orientation   domain.Orientation // from the first APP1 "Exif\0\0"; 1 when absent, unreadable or out of range
	ICC           []byte             // the profile reassembled from all APP2 ICC_PROFILE parts in sequence order; nil when none or the parts are inconsistent
	JFIF          *jfifHeader        // nil without a JFIF APP0
	Adobe         bool               // an APP14 Adobe segment exists
}

// scanJPEG reads the whole structure of r without writing and validates it exactly like stripJPEG,
// because both run the same walker. Structure errors wrap app.ErrImageMalformed; a read error of r
// other than a premature end is returned unwrapped, since it is a store failure and transient.
func scanJPEG(r io.Reader) (jpegInfo, error) {
	j := newJPEGWalker(r, nil)
	if err := j.run(nil); err != nil {
		return jpegInfo{}, err
	}
	j.info.ICC = j.icc.profile()
	return j.info, nil
}

// stripJPEG streams the cleaned JPEG of r into w (brief M3-06 design note 4). info is the result of
// scanJPEG over the same bytes; it is needed because the output starts with the rewritten APP0 and
// the new APP1, which a single pass could only write after reading every segment. Structure
// errors wrap app.ErrImageMalformed; a write error of w and a read error of r are returned
// unwrapped. On an error the output is incomplete and must be discarded.
func stripJPEG(r io.Reader, w io.Writer, info jpegInfo) error {
	bw := bufio.NewWriter(w)
	var prefix [][]byte
	if info.JFIF != nil {
		prefix = append(prefix, jfifSegment(*info.JFIF))
	}
	if app1 := orientationAPP1(info.Orientation); app1 != nil {
		prefix = append(prefix, app1)
	}
	if err := newJPEGWalker(r, bw).run(prefix); err != nil {
		return err
	}
	return bw.Flush()
}

// jfifSegment returns the APP0 JFIF segment of h with thumbnail width and height 0 and no
// thumbnail bytes: an embedded thumbnail can show what a crop removed (D-M3-06, note 4).
func jfifSegment(h jfifHeader) []byte {
	seg := []byte{markerPrefix, markerAPP0}
	seg = binary.BigEndian.AppendUint16(seg, segmentLengthSize+jfifPayloadLen)
	seg = append(seg, jfifIdentifier...)
	seg = append(seg, h.Major, h.Minor, h.Units)
	seg = binary.BigEndian.AppendUint16(seg, h.XDensity)
	seg = binary.BigEndian.AppendUint16(seg, h.YDensity)
	return append(seg, 0, 0)
}

// jpegWalker reads a JPEG segment by segment from a buffered reader and either collects jpegInfo
// (out nil, scanJPEG) or writes the kept segments to out (stripJPEG). One walker serves both, so
// the scan rejects exactly what the filter rejects. Entropy-coded data is streamed; at most one
// segment payload is held at a time.
type jpegWalker struct {
	br  *bufio.Reader
	out io.Writer // nil while scanning
	// length holds the length field of the segment just read, written back unchanged.
	length  [segmentLengthSize]byte
	payload []byte
	info    jpegInfo
	icc     iccParts
	// frames counts frame headers; hierarchical is set by DHP (T.81 B.3), which allows several.
	frames       int
	hierarchical bool
	sawSOS       bool
	sawExif      bool
}

// newJPEGWalker returns a walker over r that writes to out, or only scans when out is nil.
func newJPEGWalker(r io.Reader, out io.Writer) *jpegWalker {
	return &jpegWalker{
		br:      bufio.NewReader(r),
		out:     out,
		payload: make([]byte, maxSegmentPayload),
		info:    jpegInfo{Orientation: domain.OrientationNormal},
	}
}

// run checks SOI, writes SOI and the prefix segments, and walks the segments up to the first EOI.
func (j *jpegWalker) run(prefix [][]byte) error {
	if err := j.expectSOI(); err != nil {
		return err
	}
	if err := j.write([]byte{markerPrefix, markerSOI}); err != nil {
		return err
	}
	for _, seg := range prefix {
		if err := j.write(seg); err != nil {
			return err
		}
	}
	m, err := j.readMarker()
	for err == nil {
		switch m {
		case markerEOI:
			return j.endOfImage()
		case markerSOS:
			m, err = j.scan()
		default:
			if err = j.segment(m); err == nil {
				m, err = j.readMarker()
			}
		}
	}
	return err
}

// expectSOI fails unless the input starts with SOI.
func (j *jpegWalker) expectSOI() error {
	var soi [2]byte
	if _, err := io.ReadFull(j.br, soi[:]); err != nil {
		return jpegReadErr(err, reasonNoSOI)
	}
	if soi != [2]byte{markerPrefix, markerSOI} {
		return malformedJPEG(reasonNoSOI)
	}
	return nil
}

// readMarker reads the next marker between segments. Bytes other than 0xFF before it are
// extraneous data that some encoders leave behind; they are dropped like fill bytes, so such
// files still pass while nothing unknown reaches the output.
func (j *jpegWalker) readMarker() (byte, error) {
	for {
		b, err := j.br.ReadByte()
		if err != nil {
			return 0, jpegReadErr(err, reasonNoEOI)
		}
		if b == markerPrefix {
			return j.markerAfterPrefix(reasonNoEOI)
		}
	}
}

// markerAfterPrefix reads the marker code after a 0xFF, dropping fill bytes 0xFF (T.81 B.1.1.2);
// what names the failure when the input ends.
func (j *jpegWalker) markerAfterPrefix(what string) (byte, error) {
	for {
		b, err := j.br.ReadByte()
		if err != nil {
			return 0, jpegReadErr(err, what)
		}
		if b != markerPrefix {
			return b, nil
		}
	}
}

// segment handles the marker m between scans: standalone markers have no length; every other
// marker has a segment that is checked, noted and copied when the allow list keeps it.
func (j *jpegWalker) segment(m byte) error {
	switch {
	case m >= markerRST0 && m <= markerRST7:
		return j.write([]byte{markerPrefix, m})
	case m == markerTEM:
		return nil
	case m == markerSOI || m == markerStuff:
		return malformedJPEG("unexpected marker between segments")
	}
	payload, err := j.readSegment()
	if err != nil {
		return err
	}
	if err := j.frameHeader(m, payload); err != nil {
		return err
	}
	if j.out == nil {
		j.note(m, payload)
		return nil
	}
	if !keepSegment(m, payload) {
		return nil
	}
	return j.writeSegment(m, payload)
}

// readSegment reads the length field and the payload of a marker segment. A length below 2 or
// beyond the input is malformed.
func (j *jpegWalker) readSegment() ([]byte, error) {
	if _, err := io.ReadFull(j.br, j.length[:]); err != nil {
		return nil, jpegReadErr(err, reasonBeyondEnd)
	}
	n := int(binary.BigEndian.Uint16(j.length[:]))
	if n < segmentLengthSize {
		return nil, malformedJPEG("segment length below 2")
	}
	payload := j.payload[:n-segmentLengthSize]
	if _, err := io.ReadFull(j.br, payload); err != nil {
		return nil, jpegReadErr(err, reasonBeyondEnd)
	}
	return payload, nil
}

// frameHeader checks a frame header SOFn: only one outside the hierarchical mode, long enough for
// size and components; the first one gives width, height and components. DHP switches to the
// hierarchical mode, which has several frames.
func (j *jpegWalker) frameHeader(m byte, payload []byte) error {
	if m == markerDHP {
		j.hierarchical = true
	}
	if !isFrameHeader(m) {
		return nil
	}
	if j.frames > 0 && !j.hierarchical {
		return malformedJPEG("second frame header outside the hierarchical mode")
	}
	if len(payload) < frameHeaderMin {
		return malformedJPEG("short frame header")
	}
	if j.frames == 0 {
		j.info.Height = int(binary.BigEndian.Uint16(payload[1:]))
		j.info.Width = int(binary.BigEndian.Uint16(payload[3:]))
		j.info.Components = int(payload[5])
	}
	j.frames++
	return nil
}

// isFrameHeader reports the frame header markers SOF0 to SOF15; DHT, JPG and DAC share that range
// but are no frame headers (T.81 table B.1).
func isFrameHeader(m byte) bool {
	return m >= markerSOF0 && m <= markerSOF15 && m != markerDHT && m != markerJPG && m != markerDAC
}

// note collects the facts of the application segments while scanning: the first JFIF APP0, the
// orientation of the first Exif APP1, the ICC parts and an Adobe APP14.
func (j *jpegWalker) note(m byte, payload []byte) {
	switch m {
	case markerAPP0:
		if j.info.JFIF == nil && len(payload) >= jfifPayloadLen && bytes.HasPrefix(payload, []byte(jfifIdentifier)) {
			j.info.JFIF = &jfifHeader{
				Major: payload[5], Minor: payload[6], Units: payload[7],
				XDensity: binary.BigEndian.Uint16(payload[8:]), YDensity: binary.BigEndian.Uint16(payload[10:]),
			}
		}
	case markerAPP1:
		if !j.sawExif && bytes.HasPrefix(payload, []byte(exifIdentifier)) {
			j.sawExif = true
			j.info.Orientation = orientationFromTIFF(payload[len(exifIdentifier):])
		}
	case markerAPP2:
		if isICCSegment(payload) {
			j.icc.add(payload)
		}
	case markerAPP14:
		j.info.Adobe = j.info.Adobe || bytes.HasPrefix(payload, []byte(adobeIdentifier))
	}
}

// keepSegment applies the allow list to a segment between scans: APP0 and APP1 never pass
// (stripJPEG writes their replacements first), APP2 only as an ICC_PROFILE part, APP14 only with
// the identifier Adobe, every other marker as domain.JPEGMarkerKept decides.
func keepSegment(m byte, payload []byte) bool {
	switch m {
	case markerAPP0, markerAPP1:
		return false
	case markerAPP2:
		return isICCSegment(payload)
	case markerAPP14:
		return bytes.HasPrefix(payload, []byte(adobeIdentifier))
	default:
		return domain.JPEGMarkerKept(m)
	}
}

// isICCSegment reports an APP2 payload with the identifier ICC_PROFILE and a complete part header.
func isICCSegment(payload []byte) bool {
	return len(payload) >= iccHeaderLen && bytes.HasPrefix(payload, []byte(iccIdentifier))
}

// scan handles SOS: it requires a frame header before, copies the scan header and streams the
// entropy-coded data. It returns the marker that ends the data.
func (j *jpegWalker) scan() (byte, error) {
	if j.frames == 0 {
		return 0, malformedJPEG("SOS before a frame header")
	}
	payload, err := j.readSegment()
	if err != nil {
		return 0, err
	}
	if err := j.writeSegment(markerSOS, payload); err != nil {
		return 0, err
	}
	j.sawSOS = true
	return j.entropy()
}

// entropy copies entropy-coded data byte for byte, including stuffed bytes (0xFF 0x00) and the
// restart markers RST0 to RST7, and returns the first other marker; fill bytes before a marker
// are dropped. The data is streamed through the reader's buffer and never held as a whole.
func (j *jpegWalker) entropy() (byte, error) {
	const truncated = "truncated entropy-coded segment"
	for {
		chunk, err := j.br.ReadSlice(markerPrefix)
		if errors.Is(err, bufio.ErrBufferFull) {
			if err := j.write(chunk); err != nil {
				return 0, err
			}
			continue
		}
		if err != nil {
			return 0, jpegReadErr(err, truncated)
		}
		if err := j.write(chunk[:len(chunk)-1]); err != nil {
			return 0, err
		}
		m, err := j.markerAfterPrefix(truncated)
		if err != nil || !inEntropyData(m) {
			return m, err
		}
		if err := j.write([]byte{markerPrefix, m}); err != nil {
			return 0, err
		}
	}
}

// inEntropyData reports the codes after 0xFF that belong to the entropy-coded data: a stuffed
// byte and the restart markers.
func inEntropyData(m byte) bool {
	return m == markerStuff || (m >= markerRST0 && m <= markerRST7)
}

// endOfImage handles the first EOI: it requires a scan before and writes EOI; whatever follows
// is never read.
func (j *jpegWalker) endOfImage() error {
	if !j.sawSOS {
		return malformedJPEG("EOI before any SOS")
	}
	return j.write([]byte{markerPrefix, markerEOI})
}

// writeSegment writes marker m with the length field just read and payload.
func (j *jpegWalker) writeSegment(m byte, payload []byte) error {
	if err := j.write([]byte{markerPrefix, m, j.length[0], j.length[1]}); err != nil {
		return err
	}
	return j.write(payload)
}

// write writes b to the output; it does nothing while scanning.
func (j *jpegWalker) write(b []byte) error {
	if j.out == nil {
		return nil
	}
	_, err := j.out.Write(b)
	return err
}

// iccParts reassembles the APP2 ICC_PROFILE parts of a JPEG (ICC.1:2022 Annex B.4). Parts carry
// a sequence number from 1 and the part count; a profile is only trusted when every part agrees
// on the count and each sequence number occurs exactly once. Otherwise there is no profile, but
// the image is not rejected: the parts are still copied, the thumbnails just carry no profile.
type iccParts struct {
	parts  [][]byte // by sequence number minus one
	seen   []bool
	broken bool
}

// add records one ICC part payload (identifier and part header included).
func (p *iccParts) add(payload []byte) {
	if p.broken {
		return
	}
	seq, count := int(payload[len(iccIdentifier)]), int(payload[len(iccIdentifier)+1])
	if p.parts == nil && count > 0 {
		p.parts = make([][]byte, count)
		p.seen = make([]bool, count)
	}
	if count == 0 || count != len(p.parts) || seq < 1 || seq > count || p.seen[seq-1] {
		p.broken, p.parts, p.seen = true, nil, nil
		return
	}
	p.parts[seq-1] = bytes.Clone(payload[iccHeaderLen:])
	p.seen[seq-1] = true
}

// profile returns the reassembled profile, or nil when there were no parts, the parts were
// inconsistent, one is missing or the profile is empty.
func (p *iccParts) profile() []byte {
	if p.broken || p.parts == nil {
		return nil
	}
	for _, ok := range p.seen {
		if !ok {
			return nil
		}
	}
	profile := bytes.Join(p.parts, nil)
	if len(profile) == 0 {
		return nil
	}
	return profile
}

// malformedJPEG returns an error wrapping app.ErrImageMalformed with the reason.
func malformedJPEG(reason string) error {
	return fmt.Errorf("%w: jpeg %s", app.ErrImageMalformed, reason)
}

// jpegReadErr turns a premature end of the input into a malformed JPEG with the reason; every other
// read error is a failure of the store and returned unwrapped, so the job retries.
func jpegReadErr(err error, reason string) error {
	if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
		return malformedJPEG(reason)
	}
	return err
}
