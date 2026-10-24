// SPDX-License-Identifier: Apache-2.0

package extract

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"io"

	"github.com/OWNER/bothy/services/server/internal/files/domain"
)

// archive is one OOXML or ODF container opened in memory (brief M3-15 design note 6): the whole
// object, at most search.extract.max_input_bytes (the use case bounds the reader), read into a
// byte slice and opened with zip.NewReader, because archive/zip needs random access and ranged
// reads through the object store would cost one request per small read.
type archive struct {
	cfg   Config
	parts map[string]*zip.File
	// counter counts the bytes inflated over all parts of the file.
	counter *inflateCounter
}

// inflateCounter counts the bytes actually inflated from the parts of one archive. A fresh one
// belongs to each extraction; the tests read it to prove what was inflated.
type inflateCounter struct{ n int64 }

// openArchive reads r into memory and opens it. Any error of zip.NewReader (also the insecure
// path error) and a name that occurs twice give archive_invalid; more central directory entries
// than search.extract.max_archive_entries give archive_entries_exceeded before any entry is
// opened. Errors of r come back wrapped.
func openArchive(r io.Reader, cfg Config, counter *inflateCounter) (*archive, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, readError(err)
	}
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, domain.NewExtractError(domain.CodeArchiveInvalid)
	}
	if len(zr.File) > cfg.MaxArchiveEntries {
		return nil, domain.NewExtractError(domain.CodeArchiveEntriesExceeded)
	}
	parts := make(map[string]*zip.File, len(zr.File))
	for _, f := range zr.File {
		if _, dup := parts[f.Name]; dup {
			// An ambiguous archive may be read differently by other tools.
			return nil, domain.NewExtractError(domain.CodeArchiveInvalid)
		}
		parts[f.Name] = f
	}
	return &archive{cfg: cfg, parts: parts, counter: counter}, nil
}

// names returns the part names of the archive.
func (a *archive) names() []string {
	out := make([]string, 0, len(a.parts))
	for n := range a.parts {
		out = append(out, n)
	}
	return out
}

// check refuses the parts the format reads before any byte is inflated: an encrypted entry or a
// method other than store and deflate gives archive_invalid, and a sum of declared uncompressed
// sizes above search.extract.max_uncompressed_bytes gives archive_size_exceeded.
func (a *archive) check(names []string) error {
	var sum uint64
	for _, n := range names {
		f := a.parts[n]
		if f.Flags&0x1 != 0 || (f.Method != zip.Store && f.Method != zip.Deflate) {
			return domain.NewExtractError(domain.CodeArchiveInvalid)
		}
		sum += f.UncompressedSize64
		if sum > uint64(a.cfg.MaxUncompressedBytes) { //nolint:gosec // G115: the limit is at least 1
			return domain.NewExtractError(domain.CodeArchiveSizeExceeded)
		}
	}
	return nil
}

// open opens the part name for reading through the shared inflation counter. A part that
// inflates beyond its declared size fails inside archive/zip (archive_invalid), so a lying header
// never inflates more than it declared.
func (a *archive) open(name string) (io.ReadCloser, error) {
	rc, err := a.parts[name].Open()
	if err != nil {
		return nil, domain.NewExtractError(domain.CodeArchiveInvalid)
	}
	return &countingPart{rc: rc, counter: a.counter, limit: a.cfg.MaxUncompressedBytes}, nil
}

// errInflateLimit is the error countingPart returns when the shared counter passes the limit.
var errInflateLimit = domain.NewExtractError(domain.CodeArchiveSizeExceeded)

// countingPart adds the bytes it delivers to the shared counter and fails when the counter
// passes the limit; every other error of the part is a format error of the archive.
type countingPart struct {
	rc      io.ReadCloser
	counter *inflateCounter
	limit   int64
}

// Read implements io.Reader.
func (c *countingPart) Read(p []byte) (int, error) {
	n, err := c.rc.Read(p)
	c.counter.n += int64(n)
	if c.counter.n > c.limit {
		return n, errInflateLimit
	}
	if err != nil && !errors.Is(err, io.EOF) {
		return n, domain.NewExtractError(domain.CodeArchiveInvalid)
	}
	return n, err
}

// Close implements io.Closer.
func (c *countingPart) Close() error { return c.rc.Close() }

// readPart runs the XML reader h over the part name (check must have passed for it).
func (a *archive) readPart(ctx context.Context, name string, b *domain.TextBuilder, h xmlHandler) error {
	rc, err := a.open(name)
	if err != nil {
		return err
	}
	defer func() { _ = rc.Close() }()
	return xmlText(ctx, rc, b, a.cfg.MaxTextBytes, h)
}

// extractArchive is builtin-ooxml and builtin-odf: open the archive, pick the fixed parts of the
// format, check them, then read them in order.
func (r *Registry) extractArchive(ctx context.Context, in io.Reader, mt string, b *domain.TextBuilder, counter *inflateCounter) error {
	a, err := openArchive(in, r.cfg, counter)
	if err != nil {
		return err
	}
	var f format
	switch mt {
	case TypeDOCX:
		f = docx
	case TypeXLSX:
		f = xlsx
	case TypePPTX:
		f = pptx
	default:
		f = odf
	}
	names, err := f.parts(a)
	if err != nil {
		return err
	}
	if err := a.check(names); err != nil {
		return err
	}
	for i, n := range names {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if i > 0 {
			b.Separator('\n')
		}
		if err := a.readPart(ctx, n, b, f.handler()); err != nil {
			return err
		}
		if b.Truncated() {
			return nil
		}
	}
	return nil
}

// format names the parts of a container format and builds the XML handler of one part.
type format struct {
	parts   func(a *archive) ([]string, error)
	handler func() xmlHandler
}
