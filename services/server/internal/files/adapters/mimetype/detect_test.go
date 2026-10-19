// SPDX-License-Identifier: Apache-2.0

package mimetype_test

import (
	"archive/zip"
	"bytes"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/files/adapters/mimetype"
	"github.com/OWNER/bothy/services/server/internal/files/domain"
)

// docx builds a minimal OOXML word document; filler bytes of incompressible data stand in an entry
// before word/document.xml, so with a large filler the marker lies beyond 4 KiB.
func docx(t *testing.T, filler int) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	pad := make([]byte, filler)
	for i := range pad {
		pad[i] = byte((i*7919 + i>>3) % 256)
	}
	for _, e := range []struct {
		name string
		body []byte
	}{{"[Content_Types].xml", []byte("<x/>")}, {"docProps/thumbnail.jpeg", pad}, {"word/document.xml", []byte("<x/>")}} {
		f, err := w.CreateHeader(&zip.FileHeader{Name: e.name, Method: zip.Store})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := f.Write(e.body); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestDetect(t *testing.T) {
	t.Parallel()
	png := append([]byte("\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR"), make([]byte, 32)...)
	d := mimetype.NewDetector(1 << 20)
	if got := domain.MediaType(d.Detect(png)); got != "image/png" {
		t.Errorf("PNG declared as text/plain detected as %q", got)
	}
	const word = "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
	if got := domain.MediaType(d.Detect(docx(t, 0))); got != word {
		t.Errorf("OOXML detected as %q", got)
	}
	if got := domain.MediaType(d.Detect(docx(t, 6000))); got != word {
		t.Errorf("OOXML with its marker beyond 4 KiB detected as %q", got)
	}
	if got := d.Detect([]byte("plain text of Sam\n")); got != "text/plain; charset=utf-8" || domain.MediaType(got) != "text/plain" {
		t.Errorf("text detected as %q", got)
	}
	if got := d.Detect(nil); got != "application/octet-stream" {
		t.Errorf("empty head detected as %q", got)
	}
}
