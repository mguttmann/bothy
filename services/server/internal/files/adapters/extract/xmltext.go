// SPDX-License-Identifier: Apache-2.0

package extract

import (
	"bytes"
	"context"
	"encoding/xml"
	"errors"
	"io"

	"github.com/OWNER/bothy/services/server/internal/files/domain"
)

// xmlHandler receives the tokens of one XML part.
type xmlHandler interface {
	start(e xml.StartElement, b *domain.TextBuilder)
	end(e xml.EndElement, b *domain.TextBuilder)
	text(t xml.CharData, b *domain.TextBuilder)
}

// errRunTooLong is the run guard's signal: a run between two '<' passed the text limit.
var errRunTooLong = errors.New("files: an XML run exceeds the text limit")

// runGuard sits between a part and the decoder and fails when more than max bytes pass without a
// '<' (brief M3-15 design note 7). encoding/xml buffers a whole text, comment or attribute token
// before it returns it; the guard bounds every token by the text limit.
type runGuard struct {
	r   io.Reader
	max int64
	run int64
	// total counts the bytes delivered to the decoder; tripAt is total when the guard failed,
	// -1 before.
	total, tripAt int64
}

// Read implements io.Reader.
func (g *runGuard) Read(p []byte) (int, error) {
	n, err := g.r.Read(p)
	for i := range n {
		if p[i] == '<' {
			g.run = 0
			continue
		}
		g.run++
		if g.run > g.max {
			g.total += int64(i)
			g.tripAt = g.total
			return i, errRunTooLong
		}
	}
	g.total += int64(n)
	return n, err
}

// partialRun reports a token that ends where the guard failed: the cut-off start of a run.
func (g *runGuard) partialRun(d *xml.Decoder) bool {
	return g.tripAt >= 0 && d.InputOffset() >= g.tripAt
}

// xmlText decodes one part with encoding/xml: Strict, no entities beyond the predefined ones and
// no CharsetReader, so a declared encoding other than UTF-8 is xml_invalid. A document type
// declaration gives xml_doctype at once; every other syntax error, an undefined entity included,
// gives xml_invalid. When the run guard trips, the part ends with its text so far and the builder
// is marked truncated. The context is checked at every token. Errors of the archive (the
// inflation limit, format errors) come back unchanged.
func xmlText(ctx context.Context, r io.Reader, b *domain.TextBuilder, maxRun int64, h xmlHandler) error {
	g := &runGuard{r: r, max: maxRun, tripAt: -1}
	return decodeXML(ctx, g, b, h)
}

// decodeXML is the token loop of xmlText over the guarded reader g.
func decodeXML(ctx context.Context, g *runGuard, b *domain.TextBuilder, h xmlHandler) error {
	d := xml.NewDecoder(g)
	d.Strict = true
	d.Entity = nil
	d.CharsetReader = nil
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		if b.Truncated() {
			return nil
		}
		tok, err := d.Token()
		if err != nil {
			return xmlError(err, b)
		}
		switch t := tok.(type) {
		case xml.StartElement:
			h.start(t, b)
		case xml.EndElement:
			h.end(t, b)
		case xml.CharData:
			if g.partialRun(d) {
				// The decoder returns the part of a run it read before the guard failed; the run
				// alone exceeds the text limit, so it is dropped and the part ends.
				b.MarkTruncated()
				return nil
			}
			h.text(t, b)
		case xml.Directive:
			return directiveError(t)
		case xml.Comment, xml.ProcInst:
		}
	}
}

// directiveError fails a part at a directive: a document type declaration gives xml_doctype,
// any other directive xml_invalid.
func directiveError(t xml.Directive) error {
	if bytes.HasPrefix(bytes.TrimSpace(t), []byte("DOCTYPE")) {
		return domain.NewExtractError(domain.CodeXMLDoctype)
	}
	return domain.NewExtractError(domain.CodeXMLInvalid)
}

// xmlError maps an error of the decoder.
func xmlError(err error, b *domain.TextBuilder) error {
	var xe *domain.ExtractError
	switch {
	case errors.Is(err, io.EOF):
		return nil
	case errors.Is(err, errRunTooLong):
		b.MarkTruncated()
		return nil
	case errors.As(err, &xe):
		return xe
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		return err
	}
	return domain.NewExtractError(domain.CodeXMLInvalid)
}

// textIn is the handler shape shared by the formats: text of the elements named textElem
// (outside the subtrees of skipElems), separators at the named start and end elements.
type textIn struct {
	// textElem is the local name whose character data is text; "" takes all character data
	// inside within (or everywhere when within is "").
	textElem string
	// within limits the text to the subtree of this local name; "" for everywhere.
	within string
	// skip are subtrees whose text is dropped.
	skip map[string]bool
	// startSep and endSep map a local name to the separator its start or end tag writes.
	startSep, endSep map[string]rune

	inText, inWithin, skipping int
}

func (h *textIn) start(e xml.StartElement, b *domain.TextBuilder) {
	n := e.Name.Local
	if h.skipping > 0 || h.skip[n] {
		h.skipping++
		return
	}
	if h.within != "" && n == h.within {
		h.inWithin++
	}
	if h.textElem != "" && n == h.textElem {
		h.inText++
	}
	if r, ok := h.startSep[n]; ok && h.active() {
		b.Separator(r)
	}
}

func (h *textIn) end(e xml.EndElement, b *domain.TextBuilder) {
	if h.skipping > 0 {
		h.skipping--
		return
	}
	n := e.Name.Local
	if r, ok := h.endSep[n]; ok && h.active() {
		b.Separator(r)
	}
	if h.textElem != "" && n == h.textElem && h.inText > 0 {
		h.inText--
	}
	if h.within != "" && n == h.within && h.inWithin > 0 {
		h.inWithin--
	}
}

func (h *textIn) text(t xml.CharData, b *domain.TextBuilder) {
	if h.skipping > 0 || !h.active() {
		return
	}
	if h.textElem != "" && h.inText == 0 {
		return
	}
	b.WriteText(string(t))
}

// active reports being inside the within subtree (or no such limit).
func (h *textIn) active() bool { return h.within == "" || h.inWithin > 0 }
