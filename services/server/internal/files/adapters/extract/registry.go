// SPDX-License-Identifier: Apache-2.0

package extract

import (
	"context"
	"errors"
	"io"
	"strings"

	"github.com/OWNER/bothy/services/server/internal/files/domain"
)

// The adapter identifiers stored in file_text_extracts.extractor. Raise the number of an adapter
// whenever its output changes (TestExtractorGolden enforces it), so the refresh job re-extracts
// the rows of the older version.
const (
	VersionText  = "builtin-text/1"
	VersionHTML  = "builtin-html/1"
	VersionOOXML = "builtin-ooxml/1"
	VersionODF   = "builtin-odf/1"
)

// The detected media types of the Office and OpenDocument formats (the names of the detector
// github.com/gabriel-vasile/mimetype, adapters/mimetype).
const (
	TypeDOCX = "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
	TypeXLSX = "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
	TypePPTX = "application/vnd.openxmlformats-officedocument.presentationml.presentation"
	TypeODT  = "application/vnd.oasis.opendocument.text"
	TypeODS  = "application/vnd.oasis.opendocument.spreadsheet"
	TypeODP  = "application/vnd.oasis.opendocument.presentation"
)

// Config holds the limits of an extraction, bound in internal/wire from search.extract.*.
type Config struct {
	// MaxTextBytes is search.extract.max_text_bytes: the most text kept, and the longest run an
	// XML part may have between two '<'.
	MaxTextBytes int64 `config:"search.extract.max_text_bytes"`
	// MaxUncompressedBytes is search.extract.max_uncompressed_bytes: the most bytes the parts of
	// one archive may inflate to.
	MaxUncompressedBytes int64 `config:"search.extract.max_uncompressed_bytes"`
	// MaxArchiveEntries is search.extract.max_archive_entries: the most entries the central
	// directory of an archive may list.
	MaxArchiveEntries int `config:"search.extract.max_archive_entries"`
	// MaxXMLDepth is search.extract.max_xml_depth: the deepest element nesting an XML part may
	// have; encoding/xml keeps one stack entry per open element, so the nesting bounds memory.
	MaxXMLDepth int `config:"search.extract.max_xml_depth"`
}

// Registry implements app.TextExtractor (checked in the tests, so the package imports no part of
// app and with it no network package) over the four built-in adapters. It holds no state
// between calls and is safe for concurrent use.
type Registry struct {
	cfg Config
}

// New returns the registry. Errors: a limit below 1.
func New(cfg Config) (*Registry, error) {
	if cfg.MaxTextBytes < 1 || cfg.MaxUncompressedBytes < 1 || cfg.MaxArchiveEntries < 1 || cfg.MaxXMLDepth < 1 {
		return nil, errors.New("files: search.extract.max_text_bytes, search.extract.max_uncompressed_bytes, search.extract.max_archive_entries and search.extract.max_xml_depth must be positive")
	}
	return &Registry{cfg: cfg}, nil
}

// adapter names the extraction function of a media type.
type adapter uint8

const (
	adapterNone adapter = iota
	adapterText
	adapterHTML
	adapterOOXML
	adapterODF
)

// adapterOf maps a media type (without parameters, lower case) to its adapter.
func adapterOf(mt string) adapter {
	switch mt {
	case "text/html":
		return adapterHTML
	case "text/rtf":
		return adapterNone
	case "application/json":
		return adapterText
	case TypeDOCX, TypeXLSX, TypePPTX:
		return adapterOOXML
	case TypeODT, TypeODS, TypeODP:
		return adapterODF
	}
	if strings.HasPrefix(mt, "text/") {
		return adapterText
	}
	return adapterNone
}

// version is the identifier of a.
func (a adapter) version() string {
	switch a {
	case adapterText:
		return VersionText
	case adapterHTML:
		return VersionHTML
	case adapterOOXML:
		return VersionOOXML
	case adapterODF:
		return VersionODF
	case adapterNone:
	}
	return ""
}

// Version implements app.TextExtractor.
//
// Example:
//
//	v, ok := reg.Version("text/csv") // "builtin-text/1", true
func (r *Registry) Version(mimeType string) (string, bool) {
	a := adapterOf(domain.MediaType(mimeType))
	return a.version(), a != adapterNone
}

// errUnsupported reports a type no adapter supports; callers ask Version first.
var errUnsupported = errors.New("files: no text extractor supports the type")

// Extract implements app.TextExtractor (see the package documentation for the rules of each
// adapter). Errors: *domain.ExtractError for a content problem; the error of r wrapped; ctx.Err()
// when ctx ends; errUnsupported for a type Version does not support.
func (r *Registry) Extract(ctx context.Context, rd io.Reader, mimeType string) (domain.ExtractResult, error) {
	mt := domain.MediaType(mimeType)
	a := adapterOf(mt)
	if a == adapterNone {
		return domain.ExtractResult{}, errUnsupported
	}
	if err := ctx.Err(); err != nil {
		return domain.ExtractResult{}, err
	}
	b := domain.NewTextBuilder(r.cfg.MaxTextBytes)
	in := newCtxReader(ctx, rd)
	var err error
	switch a {
	case adapterText:
		err = extractText(ctx, in, b)
	case adapterHTML:
		err = extractHTML(ctx, in, b)
	case adapterOOXML, adapterODF:
		err = r.extractArchive(ctx, in, mt, b, &inflateCounter{})
	case adapterNone:
	}
	if ctx.Err() != nil {
		return domain.ExtractResult{}, ctx.Err()
	}
	if err != nil {
		return domain.ExtractResult{}, err
	}
	return domain.ExtractResult{Text: b.String(), Truncated: b.Truncated(), Version: a.version()}, nil
}

// ctxReader ends a read when ctx ends, also when the underlying reader stalls: each read runs in
// its own goroutine on a private buffer, so a read that never returns is abandoned and the caller
// gets ctx.Err(). After that the reader only returns ctx.Err().
type ctxReader struct {
	ctx context.Context //nolint:containedctx // the reader lives only inside one extraction
	r   io.Reader
	buf []byte
	err error
}

func newCtxReader(ctx context.Context, r io.Reader) *ctxReader { return &ctxReader{ctx: ctx, r: r} }

type readResult struct {
	n   int
	err error
}

// Read implements io.Reader.
func (c *ctxReader) Read(p []byte) (int, error) {
	if c.err != nil {
		return 0, c.err
	}
	if err := c.ctx.Err(); err != nil {
		c.err = err
		return 0, err
	}
	if len(p) == 0 {
		return 0, nil
	}
	if cap(c.buf) < len(p) {
		c.buf = make([]byte, len(p))
	}
	buf := c.buf[:len(p)]
	done := make(chan readResult, 1)
	go func() {
		n, err := c.r.Read(buf)
		done <- readResult{n, err}
	}()
	select {
	case res := <-done:
		copy(p, buf[:res.n])
		return res.n, res.err
	case <-c.ctx.Done():
		// The goroutine may still use buf: never touch it again.
		c.buf, c.err = nil, c.ctx.Err()
		return 0, c.err
	}
}

// readError wraps an error of the object reader unchanged for errors.Is.
func readError(err error) error {
	return &wrappedReadError{err: err}
}

type wrappedReadError struct{ err error }

func (e *wrappedReadError) Error() string { return "files: reading the object: " + e.err.Error() }
func (e *wrappedReadError) Unwrap() error { return e.err }
