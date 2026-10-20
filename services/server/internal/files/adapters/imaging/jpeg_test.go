// SPDX-License-Identifier: Apache-2.0

package imaging

import (
	"bytes"
	"encoding/binary"
	"errors"
	"image/color"
	"image/jpeg"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/files/app"
	"github.com/OWNER/bothy/services/server/internal/files/domain"
)

// fullJPEGExtras is every kind of metadata of TestStripJPEGRemovesMetadata.
func fullJPEGExtras() jpegExtras {
	return jpegExtras{
		Orientation: 6, EXIF: true, JFIFThumbnail: true, JFXX: true, XMP: true, IPTC: true, COM: true, MPF: true,
		ICC: syntheticICC(testColourRGB, 3000), ICCParts: 2, Adobe: true, Appended: true,
	}
}

// stripTestJPEG runs scanJPEG and stripJPEG over in and fails the test on an error.
func stripTestJPEG(tb testing.TB, in []byte) (jpegInfo, []byte) {
	tb.Helper()
	info, err := scanJPEG(bytes.NewReader(in))
	if err != nil {
		tb.Fatalf("scanJPEG: %v", err)
	}
	var out bytes.Buffer
	if err := stripJPEG(bytes.NewReader(in), &out, info); err != nil {
		tb.Fatalf("stripJPEG: %v", err)
	}
	return info, out.Bytes()
}

// isCoreMarker reports the markers whose segments the filter must copy byte for byte.
func isCoreMarker(m byte) bool {
	return m == 0xDB || m == 0xC4 || m == 0xDA || m == 0xDD || (m >= 0xC0 && m <= 0xCF)
}

// coreSegments returns the bytes of the core segments of segs in order.
func coreSegments(segs []jpegSegment) [][]byte {
	var out [][]byte
	for _, s := range segs {
		if isCoreMarker(s.Marker) {
			out = append(out, s.Bytes)
		}
	}
	return out
}

// segmentsWith returns the segments of marker m.
func segmentsWith(segs []jpegSegment, m byte) []jpegSegment {
	var out []jpegSegment
	for _, s := range segs {
		if s.Marker == m {
			out = append(out, s)
		}
	}
	return out
}

func TestStripJPEGRemovesMetadata(t *testing.T) {
	x := fullJPEGExtras()
	in := buildJPEG(t, twoColour(64, 32, color.RGBA{R: 200, A: 255}, color.RGBA{B: 200, A: 255}), x)
	info, out := stripTestJPEG(t, in)

	if info.Width != 64 || info.Height != 32 || info.Components != 3 || info.Orientation != 6 || !info.Adobe || info.JFIF == nil {
		t.Fatalf("info = %+v", info)
	}
	if !bytes.Equal(info.ICC, x.ICC) {
		t.Fatalf("scanned ICC differs from the input")
	}
	segs := jpegSegments(t, out)
	checkOnlyAllowedAPP(t, segs)
	checkOrientationAPP1(t, segs, 6)
	checkRewrittenAPP0(t, segs)
	checkICCOutput(t, out, segs, x.ICC)
	if len(segmentsWith(segs, 0xFE)) != 0 {
		t.Fatalf("COM kept")
	}
	if len(jpegTrailing(t, out)) != 0 || !bytes.HasSuffix(out, []byte{0xFF, 0xD9}) {
		t.Fatalf("output does not end with the first EOI")
	}
	want, got := coreSegments(jpegSegments(t, in)), coreSegments(segs)
	if len(want) != len(got) {
		t.Fatalf("core segments: %d, want %d", len(got), len(want))
	}
	for i := range want {
		if !bytes.Equal(want[i], got[i]) {
			t.Fatalf("core segment %d differs", i)
		}
	}
	checkSamePixels(t, in, out)
}

// checkOnlyAllowedAPP fails when any APPn other than APP0, APP1, APP2 and APP14 is present.
func checkOnlyAllowedAPP(t *testing.T, segs []jpegSegment) {
	t.Helper()
	for _, s := range segs {
		if s.Marker >= 0xE0 && s.Marker <= 0xEF && s.Marker != 0xE0 && s.Marker != 0xE1 && s.Marker != 0xE2 && s.Marker != 0xEE {
			t.Fatalf("APP%d kept", s.Marker-0xE0)
		}
	}
	if len(segmentsWith(segs, 0xEE)) != 1 {
		t.Fatalf("APP14 Adobe missing")
	}
}

// checkOrientationAPP1 fails unless segs hold exactly one APP1 with the single IFD0 entry
// Orientation = o and no next IFD.
func checkOrientationAPP1(t *testing.T, segs []jpegSegment, o domain.Orientation) {
	t.Helper()
	app1 := segmentsWith(segs, 0xE1)
	if len(app1) != 1 {
		t.Fatalf("%d APP1 segments, want 1", len(app1))
	}
	tiff := app1[0].Bytes[4+len(exifIdentifier):]
	if got := orientationFromTIFF(tiff); got != o {
		t.Fatalf("orientation %d, want %d", got, o)
	}
	order := binary.BigEndian
	ifd := order.Uint32(tiff[4:])
	if n := order.Uint16(tiff[ifd:]); n != 1 {
		t.Fatalf("IFD0 has %d entries", n)
	}
	if next := order.Uint32(tiff[ifd+2+12:]); next != 0 || len(tiff) != int(ifd)+2+12+4 {
		t.Fatalf("APP1 has more than IFD0")
	}
	for i, s := range segs {
		if s.Marker == 0xE1 && segs[i-1].Marker != 0xD8 && segs[i-1].Marker != 0xE0 {
			t.Fatalf("APP1 is not right after SOI and APP0")
		}
	}
}

// checkRewrittenAPP0 fails unless the second segment is the only APP0, a JFIF header without a
// thumbnail.
func checkRewrittenAPP0(t *testing.T, segs []jpegSegment) {
	t.Helper()
	app0 := segmentsWith(segs, 0xE0)
	if len(app0) != 1 || segs[1].Marker != 0xE0 {
		t.Fatalf("APP0 segments %d, or not first after SOI", len(app0))
	}
	want := testSegment(0xE0, []byte("JFIF\x00\x01\x02\x01\x00\x48\x00\x48\x00\x00"))
	if !bytes.Equal(app0[0].Bytes, want) {
		t.Fatalf("APP0 = %x, want %x", app0[0].Bytes, want)
	}
}

// checkICCOutput fails unless every APP2 of segs is an ICC_PROFILE part and the profile of out
// equals icc.
func checkICCOutput(t *testing.T, out []byte, segs []jpegSegment, icc []byte) {
	t.Helper()
	app2 := segmentsWith(segs, 0xE2)
	if len(app2) != 2 {
		t.Fatalf("%d APP2 segments, want the 2 ICC parts", len(app2))
	}
	for _, s := range app2 {
		if !bytes.HasPrefix(s.Bytes[4:], []byte(iccIdentifier)) {
			t.Fatalf("APP2 without ICC_PROFILE kept")
		}
	}
	info, err := scanJPEG(bytes.NewReader(out))
	if err != nil || !bytes.Equal(info.ICC, icc) {
		t.Fatalf("output profile differs (err %v)", err)
	}
}

// checkSamePixels fails unless image/jpeg decodes a and b to equal pixels.
func checkSamePixels(t *testing.T, a, b []byte) {
	t.Helper()
	ia, err := jpeg.Decode(bytes.NewReader(a))
	if err != nil {
		t.Fatalf("decode input: %v", err)
	}
	ib, err := jpeg.Decode(bytes.NewReader(b))
	if err != nil {
		t.Fatalf("decode output: %v", err)
	}
	samePixels(t, ia, ib)
}

// progressiveTestJPEG builds a synthetic progressive JPEG: SOF2, DRI and three scans whose
// entropy-coded data holds byte stuffing and RST0 to RST7, with a COM and an APP1 in between.
func progressiveTestJPEG() []byte {
	b := []byte{0xFF, 0xD8}
	b = append(b, testSegment(0xDB, append([]byte{0x00}, bytes.Repeat([]byte{1}, 64)...))...)
	b = append(b, testSegment(0xC2, []byte{8, 0, 16, 0, 16, 3, 1, 0x22, 0, 2, 0x11, 1, 3, 0x11, 1})...)
	b = append(b, testSegment(0xDD, []byte{0, 1})...)
	b = append(b, testSegment(0xFE, []byte("between the tables"))...)
	for scan := range byte(3) {
		b = append(b, testSegment(0xC4, []byte{0x00, 1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, scan})...)
		b = append(b, testSegment(0xDA, []byte{1, scan + 1, 0x00, 0, 63, 0})...)
		b = append(b, progressiveEntropy(scan)...)
		b = append(b, testSegment(0xE1, []byte("Exif\x00\x00garbage"))...)
	}
	return append(b, 0xFF, 0xD9)
}

// progressiveEntropy returns synthetic entropy-coded data with stuffing and every RST marker.
func progressiveEntropy(scan byte) []byte {
	var e []byte
	for i := range byte(8) {
		e = append(e, 0x10+scan, 0xFF, 0x00, i*3)
		e = append(e, 0xFF, 0xD0+i)
	}
	return append(e, 0xFF, 0x00, 0x42)
}

func TestStripJPEGProgressiveWithRestarts(t *testing.T) {
	in := progressiveTestJPEG()
	_, out := stripTestJPEG(t, in)
	inSegs, outSegs := jpegSegments(t, in), jpegSegments(t, out)
	inScans, outScans := segmentsWith(inSegs, 0xDA), segmentsWith(outSegs, 0xDA)
	if len(inScans) != 3 || len(outScans) != 3 {
		t.Fatalf("scans: in %d, out %d", len(inScans), len(outScans))
	}
	for i := range inScans {
		if !bytes.Equal(inScans[i].Bytes, outScans[i].Bytes) {
			t.Fatalf("scan %d differs", i)
		}
	}
	if len(segmentsWith(outSegs, 0xDD)) != 1 || len(segmentsWith(outSegs, 0xC2)) != 1 {
		t.Fatalf("DRI or SOF2 missing")
	}
	if len(segmentsWith(outSegs, 0xFE)) != 0 || len(segmentsWith(outSegs, 0xE1)) != 0 {
		t.Fatalf("COM or APP1 kept")
	}
}

func TestStripJPEGWithoutOrientationHasNoAPP1(t *testing.T) {
	img := twoColour(16, 8, color.Black, color.White)
	for name, x := range map[string]jpegExtras{
		"no exif":            {},
		"exif without tag":   {EXIF: true},
		"orientation 1":      {Orientation: 1, EXIF: true},
		"orientation 1 only": {Orientation: 1},
	} {
		t.Run(name, func(t *testing.T) {
			info, out := stripTestJPEG(t, buildJPEG(t, img, x))
			if info.Orientation != domain.OrientationNormal {
				t.Fatalf("orientation %d", info.Orientation)
			}
			if n := len(segmentsWith(jpegSegments(t, out), 0xE1)); n != 0 {
				t.Fatalf("%d APP1 segments", n)
			}
		})
	}
}

func TestStripJPEGOrientationFromFirstExif(t *testing.T) {
	img := twoColour(16, 8, color.Black, color.White)
	plain := encodeTestJPEG(t, img)
	first := testSegment(0xE1, append([]byte(exifIdentifier), testTIFF(t, jpegExtras{Orientation: 3})...))
	second := testSegment(0xE1, append([]byte(exifIdentifier), testTIFF(t, jpegExtras{Orientation: 8})...))
	xmp := testSegment(0xE1, []byte("http://ns.adobe.com/xap/1.0/\x00x"))
	in := append(append(append(append([]byte{0xFF, 0xD8}, xmp...), first...), second...), plain[2:]...)
	info, out := stripTestJPEG(t, in)
	if info.Orientation != 3 {
		t.Fatalf("orientation %d, want 3 from the first Exif APP1", info.Orientation)
	}
	checkOrientationAPP1(t, jpegSegments(t, out), 3)
}

// malformedJPEGs are broken inputs of TestStripJPEGMalformed and the fuzz seeds.
func malformedJPEGs(tb testing.TB) map[string][]byte {
	tb.Helper()
	b := encodeTestJPEG(tb, twoColour(32, 16, color.Black, color.White))
	segs := jpegSegments(tb, b)
	sos := segmentsWith(segs, 0xDA)[0]
	sosAt := bytes.Index(b, sos.Bytes)
	var tables, frame []byte
	for _, s := range segs {
		if s.Marker == 0xDB || s.Marker == 0xC4 {
			tables = append(tables, s.Bytes...)
		}
		if s.Marker == 0xC0 {
			frame = s.Bytes
		}
	}
	soi := []byte{0xFF, 0xD8}
	return map[string][]byte{
		"truncated scan":       b[:sosAt+len(sos.Bytes)/2],
		"segment beyond end":   append(append([]byte{}, soi...), 0xFF, 0xDB, 0x10, 0x00, 1, 2, 3),
		"length below 2":       append(append([]byte{}, soi...), 0xFF, 0xDB, 0x00, 0x01),
		"no SOI":               b[2:],
		"SOS before SOF":       append(append(append(append([]byte{}, soi...), tables...), sos.Bytes...), 0xFF, 0xD9),
		"EOI before SOS":       append(append(append(append([]byte{}, soi...), tables...), frame...), 0xFF, 0xD9),
		"no EOI":               b[:len(b)-2],
		"second frame header":  append(append(append(append([]byte{}, soi...), frame...), frame...), b[2:]...),
		"empty input":          {},
		"end between segments": append(append([]byte{}, soi...), tables...),
		"second SOI":           append(append([]byte{}, soi...), b...),
	}
}

func TestStripJPEGMalformed(t *testing.T) {
	for name, in := range malformedJPEGs(t) {
		t.Run(name, func(t *testing.T) {
			if _, err := scanJPEG(bytes.NewReader(in)); !errors.Is(err, app.ErrImageMalformed) {
				t.Fatalf("scanJPEG error %v, want ErrImageMalformed", err)
			}
			var out bytes.Buffer
			err := stripJPEG(bytes.NewReader(in), &out, jpegInfo{Orientation: 1})
			if !errors.Is(err, app.ErrImageMalformed) {
				t.Fatalf("stripJPEG error %v, want ErrImageMalformed", err)
			}
			if bytes.HasSuffix(out.Bytes(), []byte{0xFF, 0xD9}) {
				t.Fatalf("a complete JPEG was written")
			}
		})
	}
}

func TestStripJPEGHierarchical(t *testing.T) {
	b := encodeTestJPEG(t, twoColour(16, 16, color.Black, color.White))
	frame := segmentsWith(jpegSegments(t, b), 0xC0)[0].Bytes
	dhp := testSegment(0xDE, frame[4:])
	in := append(append(append([]byte{0xFF, 0xD8}, dhp...), frame...), b[2:]...)
	info, out := stripTestJPEG(t, in)
	if info.Width != 16 || info.Height != 16 {
		t.Fatalf("size %dx%d", info.Width, info.Height)
	}
	if n := len(segmentsWith(jpegSegments(t, out), 0xC0)); n != 2 {
		t.Fatalf("%d frame headers, want 2", n)
	}
}

// failingWriter fails every write with errTestWrite.
type failingWriter struct{}

var errTestWrite = errors.New("test write failure")

func (failingWriter) Write([]byte) (int, error) { return 0, errTestWrite }

func TestStripJPEGWriteErrorUnwrapped(t *testing.T) {
	in := buildJPEG(t, twoColour(16, 8, color.Black, color.White), jpegExtras{Orientation: 6})
	info, err := scanJPEG(bytes.NewReader(in))
	if err != nil {
		t.Fatal(err)
	}
	err = stripJPEG(bytes.NewReader(in), failingWriter{}, info)
	if !errors.Is(err, errTestWrite) || errors.Is(err, app.ErrImageMalformed) {
		t.Fatalf("error %v, want the write error alone", err)
	}
}

// failingReader yields data and then fails with errTestRead.
type failingReader struct{ data []byte }

var errTestRead = errors.New("test read failure")

func (r *failingReader) Read(p []byte) (int, error) {
	if len(r.data) == 0 {
		return 0, errTestRead
	}
	n := copy(p, r.data)
	r.data = r.data[n:]
	return n, nil
}

func TestScanJPEGReadErrorUnwrapped(t *testing.T) {
	in := encodeTestJPEG(t, twoColour(16, 8, color.Black, color.White))
	_, err := scanJPEG(&failingReader{data: in[:len(in)/2]})
	if !errors.Is(err, errTestRead) || errors.Is(err, app.ErrImageMalformed) {
		t.Fatalf("error %v, want the read error alone", err)
	}
}

func TestScanJPEGInconsistentICC(t *testing.T) {
	icc := syntheticICC(testColourRGB, 600)
	part := func(seq, count byte, data []byte) []byte {
		return testSegment(0xE2, append(append([]byte(iccIdentifier), seq, count), data...))
	}
	plain := encodeTestJPEG(t, twoColour(16, 8, color.Black, color.White))
	for name, parts := range map[string][][]byte{
		"count differs":  {part(1, 2, icc[:300]), part(2, 3, icc[300:])},
		"duplicate part": {part(1, 2, icc[:300]), part(1, 2, icc[300:])},
		"missing part":   {part(1, 2, icc[:300])},
		"sequence zero":  {part(0, 1, icc)},
		"count zero":     {part(1, 0, icc)},
		"short header":   {testSegment(0xE2, []byte(iccIdentifier))},
	} {
		t.Run(name, func(t *testing.T) {
			in := []byte{0xFF, 0xD8}
			for _, p := range parts {
				in = append(in, p...)
			}
			info, err := scanJPEG(bytes.NewReader(append(in, plain[2:]...)))
			if err != nil || info.ICC != nil {
				t.Fatalf("err %v, ICC %d bytes; want no error and no profile", err, len(info.ICC))
			}
		})
	}
}

// FuzzStripJPEG checks the ADR 0047 property: for any input the filter fails, or its output holds
// only allow-list segments, at most one APP1 with only the orientation, and nothing after EOI.
func FuzzStripJPEG(f *testing.F) {
	img := twoColour(16, 8, color.RGBA{G: 255, A: 255}, color.White)
	f.Add(buildJPEG(f, img, fullJPEGExtras()))
	f.Add(buildJPEG(f, img, jpegExtras{Orientation: 3}))
	f.Add(buildJPEG(f, img, jpegExtras{}))
	f.Add(progressiveTestJPEG())
	for _, b := range malformedJPEGs(f) {
		f.Add(b)
	}
	f.Fuzz(func(t *testing.T, in []byte) {
		info, err := scanJPEG(bytes.NewReader(in))
		if err != nil {
			return
		}
		var out bytes.Buffer
		if err := stripJPEG(bytes.NewReader(in), &out, info); err != nil {
			t.Fatalf("strip failed after a clean scan: %v", err)
		}
		segs, trailing := parseTestJPEG(t, out.Bytes())
		if len(trailing) != 0 {
			t.Fatalf("bytes after EOI")
		}
		checkFuzzSegments(t, segs)
	})
}

// checkFuzzSegments fails on any segment outside the allow list or a second APP1.
func checkFuzzSegments(t *testing.T, segs []jpegSegment) {
	t.Helper()
	app1 := 0
	for _, s := range segs {
		if !domain.JPEGMarkerKept(s.Marker) {
			t.Fatalf("marker %x kept", s.Marker)
		}
		if !allowedAPPSegment(s) {
			t.Fatalf("APP segment %x with %q kept", s.Marker, s.Bytes)
		}
		if s.Marker == 0xE1 {
			app1++
		}
	}
	if app1 > 1 {
		t.Fatalf("%d APP1 segments", app1)
	}
}

// allowedAPPSegment reports whether an application segment has an allowed identifier and form.
func allowedAPPSegment(s jpegSegment) bool {
	switch s.Marker {
	case 0xE0:
		return len(s.Bytes) == 4+jfifPayloadLen && bytes.HasPrefix(s.Bytes[4:], []byte(jfifIdentifier)) && s.Bytes[16] == 0 && s.Bytes[17] == 0
	case 0xE1:
		for o := domain.Orientation(2); o <= 8; o++ {
			if bytes.Equal(s.Bytes, orientationAPP1(o)) {
				return true
			}
		}
		return false
	case 0xE2:
		return bytes.HasPrefix(s.Bytes[4:], []byte(iccIdentifier))
	case 0xEE:
		return bytes.HasPrefix(s.Bytes[4:], []byte(adobeIdentifier))
	default:
		return true
	}
}
