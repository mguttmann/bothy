// SPDX-License-Identifier: Apache-2.0

// Package images_test holds the integration tests of brief M3-06 (images: removing metadata and
// thumbnails): the real server of internal/wire in role all through test/harness, so River runs
// files.scan and files.thumbnail from the queue files; PostgreSQL 18 from pgtest; the file system
// driver in the temporary root of the harness; file.scan.mode = off (no clamd); uploads through
// the real HTTP chain by Alex in space Support, channel Allgemein, with Sam as a reader of the
// channel. The test images are built in code with synthetic metadata (build_test.go).
package images_test

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"
	"net/http"
	"slices"
	"sync"
	"testing"

	filesapp "github.com/OWNER/bothy/services/server/internal/files/app"
	"github.com/OWNER/bothy/services/server/internal/files/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/testkit"
)

// TestImages runs every case on one shared server: the hooks act per file ID, so cases that hold
// or fail a run do not touch the others (subtests run one after another, never in parallel).
func TestImages(t *testing.T) {
	e := startEnv(t)
	t.Run("ImageJPEGEndToEnd", func(t *testing.T) { testJPEGEndToEnd(t, e) })
	t.Run("ImageNotCompleteBeforeCleaned", func(t *testing.T) { testNotCompleteBeforeCleaned(t, e) })
	t.Run("TruncatedJPEGFails", func(t *testing.T) { testTruncatedJPEGFails(t, e) })
	t.Run("PNGEndToEnd", func(t *testing.T) { testPNGEndToEnd(t, e) })
	t.Run("GIFReencoded", func(t *testing.T) { testGIFReencoded(t, e) })
	t.Run("StillWebPConverted", func(t *testing.T) { testStillWebPConverted(t, e) })
	t.Run("AnimatedWebPUnchanged", func(t *testing.T) { testAnimatedWebPUnchanged(t, e) })
	t.Run("ImageOverPixelLimit", func(t *testing.T) { testImageOverPixelLimit(t, e) })
	t.Run("SECT08Images", func(t *testing.T) { testSECT08Images(t, e) })
	t.Run("UnsupportedImagesUnchanged", func(t *testing.T) { testUnsupportedImagesUnchanged(t, e) })
	t.Run("VideoContainerValues", func(t *testing.T) { testVideoContainerValues(t, e) })
	t.Run("ImageEvents", func(t *testing.T) { testImageEvents(t, e) })
	t.Run("ImageJobDeletedMidway", func(t *testing.T) { testJobDeletedMidway(t, e) })
	t.Run("ImageJobCrashBeforeCommit", func(t *testing.T) { testJobCrashBeforeCommit(t, e) })
	t.Run("ImageJobRunTwice", func(t *testing.T) { testJobRunTwice(t, e) })
	t.Run("ImageJobConcurrencyLimit", func(t *testing.T) { testJobConcurrencyLimit(t, e) })
	// Last: it reads the logs of the whole flow of every case above.
	t.Run("ImageJobLogsNoMetadata", func(t *testing.T) { testJobLogsNoMetadata(t, e) })
}

// processedObjects are the objects of a cleaned image with thumbnails, as os.ReadDir sorts them.
var processedObjects = []string{"large", "medium", "orig-2", "small"}

// checkStored checks that the row describes the stored original: revision rev, sha256 and
// size_bytes of the object, and the declared hash of the session is the upload's.
func checkStored(t *testing.T, e *env, r fileRow, rev int, uploaded []byte) []byte {
	t.Helper()
	if r.revision() != rev {
		t.Fatalf("object_key %s, want revision %d", r.ObjectKey, rev)
	}
	stored := e.object(t, r.ObjectKey)
	if fmt.Sprintf("%x", r.SHA256) != sha(stored) || r.SizeBytes != int64(len(stored)) {
		t.Errorf("sha256 %x size %d, stored object %s size %d", r.SHA256, r.SizeBytes, sha(stored), len(stored))
	}
	if fmt.Sprintf("%x", r.DeclaredSHA256) != sha(uploaded) {
		t.Errorf("declared_sha256 %x changed", r.DeclaredSHA256)
	}
	return stored
}

// checkSize fails the test unless the row has width x height.
func checkSize(t *testing.T, r fileRow, width, height int) {
	t.Helper()
	if r.Width == nil || r.Height == nil || int(*r.Width) != width || int(*r.Height) != height {
		t.Errorf("width %v height %v, want %dx%d", r.Width, r.Height, width, height)
	}
}

// checkThumbnails checks the three variant rows of file id: order, keys, sizes, type, and an
// object of the recorded size per row; it returns the rows.
func checkThumbnails(t *testing.T, e *env, id, mime string, sizes [3][2]int) []thumbRow {
	t.Helper()
	rows := e.thumbRows(t, id)
	if len(rows) != 3 {
		t.Fatalf("%d thumbnails, want 3", len(rows))
	}
	for i, v := range domain.RenderedVariants() {
		r := rows[i]
		key, err := domain.VariantKey(ids.MustParse(id), v)
		if err != nil {
			t.Fatal(err)
		}
		if r.Variant != string(v) || r.ObjectKey != key || r.MIMEType != mime || r.Width != sizes[i][0] || r.Height != sizes[i][1] {
			t.Errorf("thumbnail %d: %+v, want %s %s %v", i, r, v, mime, sizes[i])
		}
		if b := e.object(t, r.ObjectKey); int64(len(b)) != r.SizeBytes {
			t.Errorf("variant %s: object %d bytes, row %d", r.Variant, len(b), r.SizeBytes)
		}
	}
	return rows
}

// near reports whether c is within a JPEG tolerance of want.
func near(c color.Color, want color.RGBA) bool {
	r, g, b, _ := c.RGBA()
	d := func(x uint32, y uint8) bool { return max(int(x>>8), int(y))-min(int(x>>8), int(y)) <= 48 }
	return d(r, want.R) && d(g, want.G) && d(b, want.B)
}

// checkOrientedJPEG checks a JPEG variant of the metadata JPEG: portrait, the first colour at the
// top and the second at the bottom (orientation 6 applied), no APP0, APP1 or COM, and the
// profile of the original in APP2.
func checkOrientedJPEG(t *testing.T, name string, b, profile []byte) {
	t.Helper()
	img, err := jpeg.Decode(bytes.NewReader(b))
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	r := img.Bounds()
	if r.Dy() <= r.Dx() || !near(img.At(r.Dx()/2, r.Dy()/4), colourFirst) || !near(img.At(r.Dx()/2, 3*r.Dy()/4), colourSecond) {
		t.Errorf("%s: %v, top %v, bottom %v", name, r, img.At(r.Dx()/2, r.Dy()/4), img.At(r.Dx()/2, 3*r.Dy()/4))
	}
	checkVariantJPEG(t, name, b, profile)
}

// checkVariantJPEG checks that a JPEG variant has no APP0, APP1, APP13 or COM and carries
// profile in APP2.
func checkVariantJPEG(t *testing.T, name string, b, profile []byte) {
	t.Helper()
	parts, _ := parseJPEG(t, b)
	for _, p := range parts {
		if p.Marker == markerAPP0 || p.Marker == markerAPP1 || p.Marker == 0xED || p.Marker == markerCOM {
			t.Errorf("%s: marker 0x%02x", name, p.Marker)
		}
	}
	if !bytes.Equal(jpegICC(parts), profile) {
		t.Errorf("%s: colour profile differs from the original's", name)
	}
	noMetadata(t, name, b)
}

// checkOrientationAPP1 checks the only APP1 of a cleaned JPEG: a big-endian TIFF with IFD0 of one
// entry, the orientation want, and no next IFD.
func checkOrientationAPP1(t *testing.T, payload []byte, want uint16) {
	t.Helper()
	be := binary.BigEndian
	tiff, ok := bytes.CutPrefix(payload, []byte(exifIdentifier))
	if !ok || len(tiff) < 8 || string(tiff[:4]) != "MM\x00\x2a" {
		t.Fatalf("APP1 %x", payload)
	}
	off := int(be.Uint32(tiff[4:]))
	if off+18 > len(tiff) {
		t.Fatalf("APP1 IFD0 outside the block: %x", payload)
	}
	ifd := tiff[off:]
	if be.Uint16(ifd) != 1 || be.Uint16(ifd[2:]) != tagOrientation || be.Uint16(ifd[4:]) != 3 || be.Uint32(ifd[6:]) != 1 ||
		be.Uint16(ifd[10:]) != want || be.Uint32(ifd[14:]) != 0 {
		t.Errorf("APP1 IFD0 %x, want only the orientation %d", ifd[:18], want)
	}
}

// coreSegments returns the segments of a JPEG that are no APPn and no COM.
func coreSegments(parts []jpegPart) [][]byte {
	var out [][]byte
	for _, p := range parts {
		if (p.Marker < markerAPP0 || p.Marker > 0xEF) && p.Marker != markerCOM {
			out = append(out, p.Bytes)
		}
	}
	return out
}

// checkAPPSegment checks one APPn or COM segment of a cleaned JPEG and returns 1 for an APP1.
func checkAPPSegment(t *testing.T, p jpegPart) int {
	t.Helper()
	switch {
	case p.Marker == markerAPP0:
		pl := p.payload()
		if !bytes.HasPrefix(pl, []byte("JFIF\x00")) || len(pl) != 14 || pl[12] != 0 || pl[13] != 0 {
			t.Errorf("APP0 %x keeps a thumbnail or is no JFIF", pl)
		}
	case p.Marker == markerAPP1:
		checkOrientationAPP1(t, p.payload(), 6)
		return 1
	case p.Marker == markerAPP2:
		if !bytes.HasPrefix(p.payload(), []byte(iccIdentifier)) {
			t.Errorf("APP2 other than ICC_PROFILE: %q", p.payload()[:min(8, len(p.payload()))])
		}
	case p.Marker == markerAPP14:
		if !bytes.HasPrefix(p.payload(), []byte("Adobe")) {
			t.Error("APP14 other than Adobe")
		}
	case p.Marker == markerCOM || (p.Marker > markerAPP0 && p.Marker <= 0xEF):
		t.Errorf("marker 0x%02x kept", p.Marker)
	}
	return 0
}

// checkStrippedJPEG checks the cleaned original of the metadata JPEG like the unit test
// TestStripJPEGRemovesMetadata: one APP1 with only the orientation, APP2 only ICC_PROFILE with
// the profile byte for byte, no other APPn except APP0 without thumbnail and APP14, no COM,
// nothing after the first EOI, every other segment byte for byte equal, equal pixels.
func checkStrippedJPEG(t *testing.T, in, out, profile []byte) {
	t.Helper()
	inParts, _ := parseJPEG(t, in)
	outParts, trailing := parseJPEG(t, out)
	if len(trailing) != 0 {
		t.Errorf("%d bytes after EOI", len(trailing))
	}
	app1 := 0
	for _, p := range outParts {
		app1 += checkAPPSegment(t, p)
	}
	if app1 != 1 {
		t.Errorf("%d APP1 segments, want 1", app1)
	}
	if !slices.EqualFunc(coreSegments(inParts), coreSegments(outParts), bytes.Equal) {
		t.Error("DQT, DHT, SOF or SOS segments differ from the input")
	}
	if !bytes.Equal(jpegICC(outParts), profile) {
		t.Error("ICC profile differs from the input")
	}
	a, errA := jpeg.Decode(bytes.NewReader(in))
	b, errB := jpeg.Decode(bytes.NewReader(out))
	if errA != nil || errB != nil {
		t.Fatalf("decode: %v %v", errA, errB)
	}
	samePixels(t, a, b)
	noMetadata(t, "the stored original", out)
}

// samePixels fails the test when a and b differ in bounds or any pixel.
func samePixels(t *testing.T, a, b image.Image) {
	t.Helper()
	if a.Bounds() != b.Bounds() {
		t.Fatalf("bounds %v != %v", a.Bounds(), b.Bounds())
	}
	r := a.Bounds()
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			if a.At(x, y) != b.At(x, y) {
				t.Fatalf("pixel %d,%d: %v != %v", x, y, a.At(x, y), b.At(x, y))
			}
		}
	}
}

// checkReader checks getFile for a finished image with thumbnails. Before an attachment exists
// (M3-07) only the uploader reads a file, so Sam, a reader of the channel through the group
// Leser, gets 404; the thumbnails are then checked through Alex.
func checkReader(t *testing.T, e *env, id string, rows []thumbRow) {
	t.Helper()
	status, f := getFile(t, e.sam, id)
	t.Logf("getFile as Sam: %d", status)
	if status != http.StatusOK {
		if status != http.StatusNotFound {
			t.Errorf("getFile as Sam: %d", status)
		}
		status, f = getFile(t, e.alex, id)
	}
	if status != http.StatusOK || f.UploadState != stateComplete || len(f.Thumbnails) != len(rows) {
		t.Fatalf("getFile: %d %+v", status, f)
	}
	for i, r := range rows {
		got := f.Thumbnails[i]
		if got.Variant != r.Variant || got.MIMEType != r.MIMEType || got.Width != r.Width || got.Height != r.Height {
			t.Errorf("getFile thumbnail %d: %+v, row %+v", i, got, r)
		}
	}
}

// testJPEGEndToEnd: the JPEG of TestStripJPEGRemovesMetadata goes through scan and
// files.thumbnail (plan row, 12 15 JPEG row).
func testJPEGEndToEnd(t *testing.T, e *env) {
	in, profile := metadataJPEG(t)
	id := e.send(t, "photo.jpg", "", in)
	r := e.waitJobDone(t, id)
	if r.UploadState != stateComplete || r.ScanStatus != scanSkipped || r.MIMEType != mimeJPEG {
		t.Fatalf("row %+v", r)
	}
	stored := checkStored(t, e, r, 2, in)
	checkStrippedJPEG(t, in, stored, profile)
	// 64x32 with orientation 6: the display size is portrait.
	checkSize(t, r, 32, 64)
	if got := e.objects(t, id); !slices.Equal(got, processedObjects) {
		t.Errorf("objects %v, want %v", got, processedObjects)
	}
	rows := checkThumbnails(t, e, id, mimeJPEG, [3][2]int{{8, 16}, {16, 32}, {32, 64}})
	for _, row := range rows {
		checkOrientedJPEG(t, row.Variant, e.object(t, row.ObjectKey), profile)
	}
	checkReader(t, e, id, rows)
	checkImageEvents(t, e, id)
}

// testNotCompleteBeforeCleaned (design note 1): while a hook holds the job after the scan, the
// row is pending and skipped and getFile shows pending.
func testNotCompleteBeforeCleaned(t *testing.T, e *env) {
	u := e.create(t, "held.jpg", "", plainJPEG(t))
	entered, release := make(chan struct{}), make(chan struct{})
	var enteredOnce, releaseOnce sync.Once
	e.hooks.beforeRun.Store(u.FileID, func(ctx context.Context) error {
		enteredOnce.Do(func() { close(entered) })
		select {
		case <-release:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	})
	defer e.hooks.beforeRun.Delete(u.FileID)
	defer releaseOnce.Do(func() { close(release) })
	e.complete(t, u)
	waitClosed(t, entered, "the held run")
	r := e.readFile(t, u.FileID)
	if r.UploadState != statePending || r.ScanStatus != scanSkipped || r.revision() != 1 || e.jobs(t, u.FileID) != 1 {
		t.Fatalf("row while held %+v, %d jobs", r, e.jobs(t, u.FileID))
	}
	status, f := getFile(t, e.alex, u.FileID)
	if status != http.StatusOK || f.UploadState != statePending || f.ScanStatus != scanSkipped {
		t.Errorf("getFile while held: %d %+v", status, f)
	}
	// downloadFile (M3-05) is not merged on this branch, so its 409 file_not_ready is not checked.
	releaseOnce.Do(func() { close(release) })
	if r := e.waitJobDone(t, u.FileID); r.UploadState != stateComplete || r.revision() != 2 {
		t.Fatalf("after the release %+v", r)
	}
}

// testTruncatedJPEGFails (plan row, 12 15 "truncated JPEG"): failed with image_malformed, no
// object of the file left, one file.updated with failed after the scan's, no audit entry.
func testTruncatedJPEGFails(t *testing.T, e *env) {
	id := e.send(t, "broken.jpg", "", truncatedJPEG(t))
	r := e.waitJobDone(t, id)
	if r.UploadState != stateFailed || strOf(r.FailureReason) != filesapp.ReasonImageMalformed || r.MIMEType != mimeJPEG {
		t.Fatalf("row %+v", r)
	}
	poll(t, "objects of the failed file gone", func() bool { return len(e.pathsOf(t, id)) == 0 })
	evs := e.events(t, id)
	failed := 0
	for _, ev := range evs {
		if ev.Payload.File.UploadState == stateFailed {
			failed++
		}
	}
	last := evs[len(evs)-1].Payload.File
	if failed != 1 || last.UploadState != stateFailed || len(evs) < 2 || evs[len(evs)-2].Payload.File.ScanStatus != scanSkipped {
		t.Errorf("%d events, %d failed, last %+v", len(evs), failed, last)
	}
	if n := e.audits(t, id); n != 0 {
		t.Errorf("%d audit entries", n)
	}
	if n := e.thumbnails(t, id); n != 0 {
		t.Errorf("%d thumbnails", n)
	}
}

// testPNGEndToEnd: the PNG of TestStripPNGRemovesMetadata is cleaned to revision 2 with only
// allow-list chunks, IDAT and iCCP unchanged, and gets three JPEG variants (opaque) with the
// profile.
func testPNGEndToEnd(t *testing.T, e *env) {
	in, profile := metadataPNG(t)
	id := e.send(t, "drawing.png", "", in)
	r := e.waitJobDone(t, id)
	if r.UploadState != stateComplete || r.MIMEType != mimePNG {
		t.Fatalf("row %+v", r)
	}
	stored := checkStored(t, e, r, 2, in)
	checkSize(t, r, 48, 24)
	checkStrippedPNG(t, in, stored)
	if got := e.objects(t, id); !slices.Equal(got, processedObjects) {
		t.Errorf("objects %v", got)
	}
	rows := checkThumbnails(t, e, id, mimeJPEG, [3][2]int{{16, 8}, {32, 16}, {48, 24}})
	for _, row := range rows {
		checkVariantJPEG(t, row.Variant, e.object(t, row.ObjectKey), profile)
	}
	checkImageEvents(t, e, id)
}

// pngIDAT returns the concatenated IDAT data and the iCCP data of chunks.
func pngIDAT(chunks []pngPart) ([]byte, []byte) {
	var idat, iccp []byte
	for _, c := range chunks {
		switch c.Type {
		case chunkIDAT:
			idat = append(idat, c.Data...)
		case chunkICCP:
			iccp = c.Data
		}
	}
	return idat, iccp
}

// checkStrippedPNG checks a cleaned PNG: only allow-list chunks, nothing after IEND, IDAT and
// iCCP byte for byte equal, equal pixels, no metadata canary.
func checkStrippedPNG(t *testing.T, in, out []byte) {
	t.Helper()
	inChunks, _ := parsePNG(t, in)
	outChunks, trailing := parsePNG(t, out)
	if len(trailing) != 0 {
		t.Errorf("%d bytes after IEND", len(trailing))
	}
	for _, c := range outChunks {
		if !domain.PNGChunkKept(c.Type) {
			t.Errorf("chunk %s kept", c.Type)
		}
	}
	inIDAT, inICCP := pngIDAT(inChunks)
	outIDAT, outICCP := pngIDAT(outChunks)
	if !bytes.Equal(inIDAT, outIDAT) || !bytes.Equal(inICCP, outICCP) || outICCP == nil {
		t.Error("IDAT or iCCP differ from the input")
	}
	a, errA := png.Decode(bytes.NewReader(in))
	b, errB := png.Decode(bytes.NewReader(out))
	if errA != nil || errB != nil {
		t.Fatalf("decode: %v %v", errA, errB)
	}
	samePixels(t, a, b)
	noMetadata(t, "the stored PNG", out)
}

// testGIFReencoded: an animated GIF with a comment and an XMP application extension is
// re-encoded to revision 2 with the same frames, delays, disposal and loop count and without the
// comment and the XMP block.
func testGIFReencoded(t *testing.T, e *env) {
	in := metadataGIF(t)
	id := e.send(t, "anim.gif", "", in)
	r := e.waitJobDone(t, id)
	if r.UploadState != stateComplete || r.MIMEType != mimeGIF {
		t.Fatalf("row %+v", r)
	}
	stored := checkStored(t, e, r, 2, in)
	checkSize(t, r, 16, 16)
	for _, x := range gifExtensions(t, stored) {
		if x.Label == markerCOM || (x.Label == 0xFF && x.App != "NETSCAPE2.0") {
			t.Errorf("extension %+v kept", x)
		}
	}
	a, errA := gif.DecodeAll(bytes.NewReader(in))
	b, errB := gif.DecodeAll(bytes.NewReader(stored))
	if errA != nil || errB != nil {
		t.Fatalf("decode: %v %v", errA, errB)
	}
	if len(a.Image) != len(b.Image) || !slices.Equal(a.Delay, b.Delay) || !bytes.Equal(a.Disposal, b.Disposal) || a.LoopCount != b.LoopCount {
		t.Fatalf("frames %d/%d delays %v/%v disposal %v/%v loop %d/%d", len(a.Image), len(b.Image), a.Delay, b.Delay, a.Disposal, b.Disposal,
			a.LoopCount, b.LoopCount)
	}
	for i := range a.Image {
		if !bytes.Equal(a.Image[i].Pix, b.Image[i].Pix) || a.Image[i].Bounds() != b.Image[i].Bounds() {
			t.Errorf("frame %d differs", i)
		}
	}
	noMetadata(t, "the stored GIF", stored)
	if n := e.thumbnails(t, id); n != 3 {
		t.Errorf("%d thumbnails", n)
	}
	checkImageEvents(t, e, id)
}

// testStillWebPConverted: a still opaque WebP with EXIF orientation 6 becomes a JPEG of revision
// 2 with the orientation applied; original_name keeps its extension.
func testStillWebPConverted(t *testing.T, e *env) {
	in := stillWebP(t)
	id := e.send(t, "still.webp", "", in)
	r := e.waitJobDone(t, id)
	if r.UploadState != stateComplete || r.MIMEType != mimeJPEG || r.OriginalName != canaryNamePrefix+"still.webp" {
		t.Fatalf("row %+v", r)
	}
	stored := checkStored(t, e, r, 2, in)
	checkSize(t, r, 8, 16)
	img, err := jpeg.Decode(bytes.NewReader(stored))
	if err != nil || img.Bounds().Dx() != 8 || img.Bounds().Dy() != 16 {
		t.Fatalf("stored JPEG: %v %v", err, img)
	}
	parts, _ := parseJPEG(t, stored)
	for _, p := range parts {
		if p.Marker == markerAPP1 {
			t.Error("the converted WebP carries APP1")
		}
	}
	noMetadata(t, "the converted WebP", stored)
	if n := e.thumbnails(t, id); n != 3 {
		t.Errorf("%d thumbnails", n)
	}
	checkImageEvents(t, e, id)
}

// testAnimatedWebPUnchanged: an animated WebP stays revision 1, complete, without thumbnails.
func testAnimatedWebPUnchanged(t *testing.T, e *env) {
	in := animatedWebP(t)
	id := e.send(t, "anim.webp", "", in)
	r := e.waitJobDone(t, id)
	if r.UploadState != stateComplete || r.MIMEType != mimeWebP {
		t.Fatalf("row %+v", r)
	}
	if stored := checkStored(t, e, r, 1, in); !bytes.Equal(stored, in) {
		t.Error("the stored object changed")
	}
	if n := e.thumbnails(t, id); n != 0 {
		t.Errorf("%d thumbnails", n)
	}
	if got := e.objects(t, id); !slices.Equal(got, []string{"orig-1"}) {
		t.Errorf("objects %v", got)
	}
}

// checkGuarded checks a file above file.image.max_pixels: complete in revision rev, no
// thumbnails, and the job logged the class too_many_pixels, so the pixel guard refused before
// any decode.
func checkGuarded(t *testing.T, e *env, id, mime string, rev int) {
	t.Helper()
	r := e.waitJobDone(t, id)
	if r.UploadState != stateComplete || r.MIMEType != mime || r.revision() != rev {
		t.Fatalf("row %+v, revision %d, want %d", r, r.revision(), rev)
	}
	if n := e.thumbnails(t, id); n != 0 {
		t.Errorf("%d thumbnails", n)
	}
	if !e.logLine(id, classTooManyPixels) {
		t.Errorf("no log line %s for %s", classTooManyPixels, id)
	}
}

// testImageOverPixelLimit (12 15 "huge dimensions"): JPEG and PNG declaring huge dimensions are
// cleaned to revision 2 without thumbnails; a GIF above the limit stays revision 1.
func testImageOverPixelLimit(t *testing.T, e *env) {
	checkGuarded(t, e, e.send(t, "huge.jpg", "", hugeJPEG(t)), mimeJPEG, 2)
	checkGuarded(t, e, e.send(t, "huge.png", "", hugePNG(t)), mimePNG, 2)
	checkGuarded(t, e, e.send(t, "huge.gif", "", hugeGIF(t)), mimeGIF, 1)
}

// testSECT08Images (SEC-T08, image part): the detected type decides, an SVG gets no job, a
// decompression bomb is not decoded.
func testSECT08Images(t *testing.T, e *env) {
	in := encodePNG(t, twoColour(32, 16, colourFirst, colourSecond))
	id := e.send(t, "disguised.jpg", mimeJPEG, in)
	r := e.waitJobDone(t, id)
	if r.UploadState != stateComplete || r.MIMEType != mimePNG || r.revision() != 2 || e.thumbnails(t, id) != 3 {
		t.Fatalf("PNG declared as JPEG: %+v", r)
	}
	svg := e.send(t, "vector.svg", "image/svg+xml", svgWithScript())
	r = e.waitScanned(t, svg)
	if r.UploadState != stateComplete || r.MIMEType != "image/svg+xml" || r.revision() != 1 || e.jobs(t, svg) != 0 || e.thumbnails(t, svg) != 0 {
		t.Fatalf("SVG: %+v, %d jobs", r, e.jobs(t, svg))
	}
	checkGuarded(t, e, e.send(t, "bomb.png", "", bombPNG(t)), mimePNG, 2)
}

// testUnsupportedImagesUnchanged: TIFF and HEIC are complete after the scan, without job and
// thumbnails.
func testUnsupportedImagesUnchanged(t *testing.T, e *env) {
	for _, c := range []struct {
		name, mime string
		content    []byte
	}{{"scan.tiff", "image/tiff", tiffImage()}, {"photo.heic", "image/heic", heicImage()}} {
		id := e.send(t, c.name, "", c.content)
		r := e.waitScanned(t, id)
		if r.UploadState != stateComplete || r.MIMEType != c.mime || r.revision() != 1 || e.jobs(t, id) != 0 || e.thumbnails(t, id) != 0 {
			t.Errorf("%s: %+v, %d jobs", c.name, r, e.jobs(t, id))
		}
	}
}

// eventsAfterScan returns the file.updated events after the scan's (the last one with
// scan_status skipped and the type the scan detected before any job event).
func eventsAfterScan(t *testing.T, evs []fileEvent) []fileEvent {
	t.Helper()
	for i, ev := range evs {
		if ev.Payload.File.ScanStatus == scanSkipped {
			return evs[i+1:]
		}
	}
	t.Fatalf("no event of the scan in %d events", len(evs))
	return nil
}

// testVideoContainerValues: a rotated MP4 gets swapped width and height and duration_ms with one
// more file.updated; a broken box structure leaves the values absent and the file complete.
func testVideoContainerValues(t *testing.T, e *env) {
	id := e.send(t, "clip.mp4", "", rotatedMP4())
	r := e.waitJobDone(t, id)
	if r.UploadState != stateComplete || r.MIMEType != "video/mp4" || r.DurationMS == nil || *r.DurationMS != 4500 {
		t.Fatalf("row %+v", r)
	}
	checkSize(t, r, 1080, 1920)
	after := eventsAfterScan(t, e.events(t, id))
	if len(after) != 1 || after[0].Payload.File.Width != 1080 || after[0].Payload.File.Height != 1920 || after[0].Payload.File.DurationMS != 4500 {
		t.Errorf("%d events after the scan: %+v", len(after), after)
	}
	broken := e.send(t, "broken.mp4", "", brokenMP4())
	r = e.waitJobDone(t, broken)
	if r.UploadState != stateComplete || r.Width != nil || r.Height != nil || r.DurationMS != nil {
		t.Fatalf("broken MP4: %+v", r)
	}
	if after := eventsAfterScan(t, e.events(t, broken)); len(after) != 0 {
		t.Errorf("%d events after the scan of the broken MP4", len(after))
	}
}

// checkImageEvents checks the file.updated events of a processed image: after the events of the
// upload and the scan exactly one event of T1 (complete, no thumbnails) and one of T2 (three
// thumbnails in order), rising seq, no JSON null and no metadata value in any payload.
func checkImageEvents(t *testing.T, e *env, id string) {
	t.Helper()
	evs := e.events(t, id)
	for i, ev := range evs {
		if testkit.HasJSONNull(ev.Raw) || ev.Payload.File.ID != id {
			t.Errorf("event %d: %s", i, ev.Raw)
		}
		noMetadata(t, "a file.updated payload", ev.Raw)
		if i > 0 && ev.Seq <= evs[i-1].Seq {
			t.Errorf("seq %d after %d", ev.Seq, evs[i-1].Seq)
		}
	}
	after := eventsAfterScan(t, evs)
	if len(after) != 2 {
		t.Fatalf("%d events after the scan, want T1 and T2", len(after))
	}
	t1, t2 := after[0].Payload.File, after[1].Payload.File
	if t1.UploadState != stateComplete || len(t1.Thumbnails) != 0 || t1.SHA256 == "" {
		t.Errorf("T1 event %s", after[0].Raw)
	}
	var variants []string
	for _, th := range t2.Thumbnails {
		variants = append(variants, th.Variant)
	}
	if t2.UploadState != stateComplete || !slices.Equal(variants, []string{"small", "medium", "large"}) || t2.SHA256 != t1.SHA256 {
		t.Errorf("T2 event %s", after[1].Raw)
	}
}

// testImageEvents: the events of one image per processed type (JPEG, PNG, GIF, still WebP).
func testImageEvents(t *testing.T, e *env) {
	for _, c := range []struct {
		name    string
		content []byte
	}{
		{"events.jpg", plainJPEG(t)},
		{"events.png", encodePNG(t, twoColour(32, 16, colourFirst, colourSecond))},
		{"events.gif", metadataGIF(t)},
		{"events.webp", stillWebP(t)},
	} {
		id := e.send(t, c.name, "", c.content)
		e.waitJobDone(t, id)
		t.Run(c.name, func(t *testing.T) { checkImageEvents(t, e, id) })
	}
}

// strOf returns *p or "".
func strOf(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}
