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

// errRunTooLong is the run guard's signal: one token passed the text limit.
var errRunTooLong = errors.New("files: an XML token exceeds the text limit")

// decoderBuffer is the size of the bufio.Reader that xml.NewDecoder puts in front of a reader
// without ReadByte: the decoder never holds more than this many bytes it has read ahead.
const decoderBuffer = 4096

// runGuard sits between a part and the decoder and fails when one token grows beyond max bytes
// (brief M3-15 design note 7). encoding/xml buffers a whole token (text, start tag with its
// attributes, comment, processing instruction, CDATA section) before it returns it, so the guard
// counts from the start of the current token, which decodeXML sets after every token. The decoder
// reads ahead by at most decoderBuffer bytes, so a token of up to max bytes never trips the guard
// and none grows beyond max plus decoderBuffer.
type runGuard struct {
	r   io.Reader
	max int64
	// tokStart is the input offset where the current token starts.
	tokStart int64
	// total counts the bytes delivered to the decoder; tripAt is total when the guard failed,
	// -1 before.
	total, tripAt int64
}

// Read implements io.Reader.
func (g *runGuard) Read(p []byte) (int, error) {
	// Never take more from the part than the limit allows, so nothing is read past it.
	rem := g.tokStart + g.max + decoderBuffer - g.total
	if rem <= 0 {
		g.tripAt = g.total
		return 0, errRunTooLong
	}
	if int64(len(p)) > rem {
		p = p[:rem]
	}
	n, err := g.r.Read(p)
	g.total += int64(n)
	return n, err
}

// partialRun reports a token that ends where the guard failed: the cut-off start of a run.
func (g *runGuard) partialRun(d *xml.Decoder) bool {
	return g.tripAt >= 0 && d.InputOffset() >= g.tripAt
}

// openTags bounds what the decoder keeps for the open elements: their names, their attributes and
// one stack entry per namespace declaration stay alive until the element ends. The start tags of
// all open elements together may hold at most max bytes, and at most maxDepth may be open, so the
// decoder's state is bounded by the limits and not by the shape or the size of a part.
type openTags struct {
	sizes    []int64
	sum, max int64
	maxDepth int
}

// push records a start tag of size bytes; false when a bound is exceeded.
func (o *openTags) push(size int64) bool {
	o.sizes = append(o.sizes, size)
	o.sum += size
	return len(o.sizes) <= o.maxDepth && o.sum <= o.max
}

// pop forgets the innermost open element.
func (o *openTags) pop() {
	if n := len(o.sizes); n > 0 {
		o.sum -= o.sizes[n-1]
		o.sizes = o.sizes[:n-1]
	}
}

// xmlLimits are the bounds of one XML part: maxRun is search.extract.max_text_bytes for the run
// guard and for the start tags of the open elements together, maxDepth
// search.extract.max_xml_depth for the element nesting.
type xmlLimits struct {
	maxRun   int64
	maxDepth int
}

// xmlText decodes one part with encoding/xml: Strict, no entities beyond the predefined ones and
// no CharsetReader, so a declared encoding other than UTF-8 is xml_invalid. A document type
// declaration gives xml_doctype at once; every other syntax error, an undefined entity included,
// gives xml_invalid, and so does a nesting deeper than search.extract.max_xml_depth or open start
// tags of more than search.extract.max_text_bytes together (the decoder keeps the names,
// attributes and namespace declarations of every open element, so a part of nothing but start
// tags would otherwise grow the heap with its size). When the run guard trips (one token longer
// than the text limit), the part ends with its text so far and the builder is marked truncated. The context is checked at every token. Errors of the archive
// (the inflation limit, format errors) come back unchanged.
func xmlText(ctx context.Context, r io.Reader, b *domain.TextBuilder, lim xmlLimits, h xmlHandler) error {
	g := &runGuard{r: r, max: lim.maxRun, tripAt: -1}
	return decodeXML(ctx, g, b, &openTags{max: lim.maxRun, maxDepth: lim.maxDepth}, h)
}

// decodeXML is the token loop of xmlText over the guarded reader g.
func decodeXML(ctx context.Context, g *runGuard, b *domain.TextBuilder, open *openTags, h xmlHandler) error {
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
		if err != nil && g.tripAt >= 0 {
			// The guard cut the part off; whatever the decoder makes of the cut (it turns some
			// read errors into syntax errors) is the end of a token longer than the text limit.
			b.MarkTruncated()
			return nil
		}
		if err != nil {
			return xmlError(err)
		}
		end := d.InputOffset()
		size := end - g.tokStart
		g.tokStart = end
		if done, err := handleToken(tok, size, g.partialRun(d), open, b, h); done {
			return err
		}
	}
}

// handleToken passes one token of size bytes to h; done ends the part with err. partial reports
// character data the guard cut off.
func handleToken(tok xml.Token, size int64, partial bool, open *openTags, b *domain.TextBuilder, h xmlHandler) (done bool, err error) {
	switch t := tok.(type) {
	case xml.StartElement:
		if !open.push(size) {
			return true, domain.NewExtractError(domain.CodeXMLInvalid)
		}
		h.start(t, b)
	case xml.EndElement:
		open.pop()
		h.end(t, b)
	case xml.CharData:
		if partial {
			// The decoder returns the part of a run it read before the guard failed; the run
			// alone exceeds the text limit, so it is dropped and the part ends.
			b.MarkTruncated()
			return true, nil
		}
		h.text(t, b)
	case xml.Directive:
		return true, directiveError(t)
	case xml.Comment, xml.ProcInst:
	}
	return false, nil
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
func xmlError(err error) error {
	var xe *domain.ExtractError
	switch {
	case errors.Is(err, io.EOF):
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
