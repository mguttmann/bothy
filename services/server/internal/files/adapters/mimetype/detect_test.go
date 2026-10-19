// SPDX-License-Identifier: Apache-2.0

package mimetype_test

import (
	"archive/zip"
	"bytes"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/files/adapters/mimetype"
	"github.com/OWNER/bothy/services/server/internal/files/domain"
)

// docx builds a minimal OOXML word document whose markers lie at the start of the archive.
func docx(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	for _, name := range []string{"[Content_Types].xml", "_rels/.rels", "word/document.xml"} {
		f, err := w.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := f.Write([]byte("<x/>")); err != nil {
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
	d := mimetype.Detector{}
	if got := domain.MediaType(d.Detect(png)); got != "image/png" {
		t.Errorf("PNG declared as text/plain detected as %q", got)
	}
	if got := domain.MediaType(d.Detect(docx(t))); got != "application/vnd.openxmlformats-officedocument.wordprocessingml.document" {
		t.Errorf("OOXML detected as %q", got)
	}
	if got := d.Detect([]byte("plain text of Sam\n")); got != "text/plain; charset=utf-8" || domain.MediaType(got) != "text/plain" {
		t.Errorf("text detected as %q", got)
	}
	if got := d.Detect(nil); got != "application/octet-stream" {
		t.Errorf("empty head detected as %q", got)
	}
}
