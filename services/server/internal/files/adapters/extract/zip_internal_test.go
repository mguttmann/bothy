// SPDX-License-Identifier: Apache-2.0

package extract

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/files/adapters/extract/extracttest"
	"github.com/OWNER/bothy/services/server/internal/files/domain"
)

// archiveRun extracts data as a DOCX and returns the bytes inflated and the error.
func archiveRun(t *testing.T, cfg Config, data []byte) (int64, error) {
	t.Helper()
	r, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	c := &inflateCounter{}
	err = r.extractArchive(context.Background(), bytes.NewReader(data), TypeDOCX, domain.NewTextBuilder(cfg.MaxTextBytes), c)
	return c.n, err
}

func codeOf(err error) domain.ExtractCode {
	var xe *domain.ExtractError
	if errors.As(err, &xe) {
		return xe.Code
	}
	return ""
}

func TestZipLimits(t *testing.T) {
	t.Parallel()
	cfg := Config{MaxTextBytes: 1 << 20, MaxUncompressedBytes: 4096, MaxArchiveEntries: 10}
	n, err := archiveRun(t, cfg, extracttest.TooManyEntries(20))
	if codeOf(err) != domain.CodeArchiveEntriesExceeded || n != 0 {
		t.Fatalf("entries: %v inflated %d", err, n)
	}
	n, err = archiveRun(t, cfg, extracttest.BombDeclared(1<<20))
	if codeOf(err) != domain.CodeArchiveSizeExceeded || n != 0 {
		t.Fatalf("declared: %v inflated %d", err, n)
	}
	const declared = 1000
	n, err = archiveRun(t, cfg, extracttest.BombLyingHeader(declared, 3000))
	if codeOf(err) != domain.CodeArchiveInvalid || n > declared+1 {
		t.Fatalf("lying header: %v inflated %d", err, n)
	}
	// The shared counter trips across two parts: each slide fits the limit alone, both do not.
	// The check of the declared sums would refuse them first, so the parts are read directly.
	big := strings.Repeat("y", 2500)
	two := extracttest.PPTXWith(map[int]string{1: extracttest.Slide(big), 2: extracttest.Slide(big)}, []int{1, 2})
	r, _ := New(Config{MaxTextBytes: 1 << 20, MaxUncompressedBytes: 4096, MaxArchiveEntries: 10})
	a, err := openArchive(bytes.NewReader(two), r.cfg, &inflateCounter{})
	if err != nil {
		t.Fatal(err)
	}
	b := domain.NewTextBuilder(1 << 20)
	if err := a.readPart(context.Background(), "ppt/slides/slide1.xml", b, pptx.handler()); err != nil {
		t.Fatalf("first part: %v", err)
	}
	err = a.readPart(context.Background(), "ppt/slides/slide2.xml", b, pptx.handler())
	if codeOf(err) != domain.CodeArchiveSizeExceeded || a.counter.n > 4096+4096 {
		t.Fatalf("shared counter: %v inflated %d", err, a.counter.n)
	}
	for name, data := range map[string][]byte{
		"encrypted": extracttest.Encrypted(), "method": extracttest.UnknownMethod(), "duplicate": extracttest.DuplicatePart(),
		"garbage": []byte("not a zip at all"),
	} {
		n, err := archiveRun(t, cfg, data)
		if codeOf(err) != domain.CodeArchiveInvalid || n != 0 {
			t.Errorf("%s: %v inflated %d", name, err, n)
		}
	}
}

// countingReader counts what the run guard took from the part.
type countingReader struct {
	r interface{ Read([]byte) (int, error) }
	n int64
}

func (c *countingReader) Read(p []byte) (int, error) {
	n, err := c.r.Read(p)
	c.n += int64(n)
	return n, err
}

func TestXMLRunGuard(t *testing.T) {
	t.Parallel()
	const limit = 1000
	doc := extracttest.DOCXDocument(extracttest.Paragraphs("vorher") + `<w:p><w:r><w:t>` + strings.Repeat("z", 50*limit) + `</w:t></w:r></w:p>`)
	cr := &countingReader{r: strings.NewReader(doc)}
	b := domain.NewTextBuilder(limit)
	if err := xmlText(context.Background(), cr, b, limit, docx.handler()); err != nil {
		t.Fatal(err)
	}
	if b.String() != "vorher" || !b.Truncated() {
		t.Fatalf("%q truncated=%v", b.String(), b.Truncated())
	}
	// The decoder reads through a bufio.Reader of 4096 bytes: at most the prefix, the limit and
	// one buffer.
	if prefix := int64(strings.Index(doc, strings.Repeat("z", 10))); cr.n > prefix+limit+4096 {
		t.Fatalf("decoder consumed %d bytes", cr.n)
	}
}
