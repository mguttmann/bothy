// SPDX-License-Identifier: Apache-2.0

package domain

import (
	"math"
	"slices"
	"strings"
)

// CodeMask reports for every byte of content whether it lies in code: in a fenced or indented
// code block or in a code span of CommonMark 0.31.2 (03 messages "Inhaltsformat" item 2). A code
// block covers its whole lines, fences and container markers included; a code span covers its
// backticks. The line breaks between code block lines are not code.
//
// It is the block and inline structure CommonMark defines, reduced to what decides code: block
// quotes and list items as containers (with lazy continuation), paragraphs, ATX and setext
// headings, thematic breaks, the two code blocks, backslash escapes, autolinks and code spans.
// Raw HTML is not part of content format 1 (SEC-OUT-1), so HTML tags are text; this is the one
// deliberate difference to CommonMark (docs/DECISIONS.md M1-56). The tests hold the code-span
// and code-block examples of the specification.
//
// Example:
//
//	mask := domain.CodeMask("see `x`") // mask[4] to mask[6] are true
func CodeMask(content string) []bool {
	p := &blockParser{src: content, mask: make([]bool, len(content))}
	for start := 0; start <= len(content); {
		end := strings.IndexByte(content[start:], '\n')
		next := 0
		if end < 0 {
			end, next = len(content), len(content)+1
		} else {
			end += start
			next = end + 1
		}
		if start == len(content) && end == start {
			break
		}
		lineEnd := end
		if lineEnd > start && content[lineEnd-1] == '\r' {
			lineEnd--
		}
		p.line(start, lineEnd)
		start = next
	}
	p.closeParagraph()
	return p.mask
}

// leafKind is the open leaf block of the innermost container.
type leafKind int

const (
	leafNone leafKind = iota
	leafParagraph
	leafFenced
	leafIndented
)

// container is an open block quote or list item; indent is the content indentation of a list
// item in columns, counted from where the item's own matching starts.
type container struct {
	quote      bool
	indent     int
	hasContent bool
}

// segment is a source range of inline content.
type segment struct{ start, end int }

// blockParser holds the state of CodeMask between lines.
type blockParser struct {
	src   string
	mask  []bool
	stack []container
	leaf  leafKind
	// fenceChar, fenceLen and fenceIndent describe the open fenced code block.
	fenceChar   byte
	fenceLen    int
	fenceIndent int
	// para holds the lines of the open paragraph.
	para []segment
	// stops are the stack indexes, ascending, of the containers a blank line does not continue:
	// the block quotes and the list items without content. A blank line, also one that is blank
	// after some block quote markers, continues the containers up to the first stop after them;
	// a binary search finds it, so a blank line costs logarithmic time also below thousands of
	// nested items.
	stops []int
	// lastText is the last byte of the current line that is not a space or tab.
	lastText int
	// lastBad holds, for each thematic break character of breakChars, the last byte of the
	// current line that is neither that character nor a space or tab.
	lastBad [3]int
	// items are the stack indexes of the open list items, ascending, so the innermost one is found
	// in constant time.
	items []int
}

// breakChars are the characters of a thematic break.
const breakChars = "-*_"

// scanBreaks fills lastBad for the line [start, end), so the container loop decides a thematic
// break in constant time per marker instead of rescanning the rest of the line (a line of
// thousands of bullets would otherwise cost quadratic time).
func (p *blockParser) scanBreaks(start, end int) {
	for k := range len(breakChars) {
		last := start - 1
		for i := start; i < end; i++ {
			if b := p.src[i]; b != breakChars[k] && b != ' ' && b != '\t' {
				last = i
			}
		}
		p.lastBad[k] = last
	}
}

// breakAt reports whether the line is a thematic break from the byte index next on.
func (p *blockParser) breakAt(c *cursor, next int) bool {
	k := strings.IndexByte(breakChars, c.s[next])
	if k < 0 || p.lastBad[k] >= next {
		return false
	}
	return thematicBreak(c.s[next:c.end])
}

// push opens the container ct.
func (p *blockParser) push(ct container) {
	if ct.quote || !ct.hasContent {
		p.stops = append(p.stops, len(p.stack))
	}
	if !ct.quote {
		p.items = append(p.items, len(p.stack))
	}
	p.stack = append(p.stack, ct)
}

// cursor walks one line in columns; rem is the number of columns of a partly consumed tab at i.
type cursor struct {
	s        string
	i, end   int
	col, rem int
}

// indent returns the white space in columns from the cursor and the byte index after it.
func (c *cursor) indent() (cols, next int) { return c.indentUpTo(math.MaxInt) }

// indentUpTo is indent, but stops once at least limit columns are counted; next is then the byte
// after the last white space counted.
func (c *cursor) indentUpTo(limit int) (cols, next int) {
	i, col := c.i, c.col
	if c.rem > 0 {
		cols, col, i = c.rem, col+c.rem, i+1
	}
	for i < c.end && cols < limit {
		switch c.s[i] {
		case ' ':
			cols, col, i = cols+1, col+1, i+1
		case '\t':
			w := 4 - col%4
			cols, col, i = cols+w, col+w, i+1
		default:
			return cols, i
		}
	}
	return cols, i
}

// skip consumes n columns of white space (fewer when the white space ends).
func (c *cursor) skip(n int) {
	for n > 0 && c.i < c.end {
		if c.rem > 0 {
			if n < c.rem {
				c.rem -= n
				c.col += n
				return
			}
			n -= c.rem
			c.col += c.rem
			c.rem = 0
			c.i++
			continue
		}
		switch c.s[c.i] {
		case ' ':
			c.i++
			c.col++
			n--
		case '\t':
			c.rem = 4 - c.col%4
		default:
			return
		}
	}
}

// skipTo consumes the white space up to the byte index next.
func (c *cursor) skipTo(next int) {
	cols, _ := c.indent()
	c.skip(cols)
	c.i, c.rem = next, 0
}

// blank reports a line with only white space from the cursor.
func (c *cursor) blank() bool {
	_, next := c.indent()
	return next >= c.end
}

// line processes one line [start, end) of the source.
func (p *blockParser) line(start, end int) {
	p.scanBreaks(start, end)
	p.lastText = start - 1
	for i := end - 1; i >= start; i-- {
		if b := p.src[i]; b != ' ' && b != '\t' {
			p.lastText = i
			break
		}
	}
	c := &cursor{s: p.src, i: start, end: end}
	matched := p.matchContainers(c)
	blank := c.blank()
	if matched < len(p.stack) {
		if p.leaf == leafParagraph && !blank && !p.startsBlock(c) {
			p.para = append(p.para, p.rest(c))
			return
		}
		p.closeFrom(matched)
	}
	if p.continueLeaf(c, start, end, blank) {
		return
	}
	p.openContainers(c)
	p.openLeaf(c, start, end)
}

// matchContainers consumes the markers of the open containers the line continues and returns
// how many it continues.
func (p *blockParser) matchContainers(c *cursor) int {
	if p.restBlank(c) {
		return p.firstStop(0)
	}
	for n := range p.stack {
		ct := &p.stack[n]
		// A container needs at most its indent (a quote at most 3 columns), so the white space
		// is scanned only that far: a long indent below many items stays linear.
		limit := ct.indent
		if ct.quote {
			limit = 4
		}
		cols, next := c.indentUpTo(limit)
		switch {
		case ct.quote:
			if cols > 3 || next >= c.end || c.s[next] != '>' {
				return n
			}
			c.skipTo(next)
			c.i++
			c.col++
			c.optionalSpace()
			if p.restBlank(c) {
				// Blank after its quote markers: the line continues the inner items up to the
				// next stop like a blank line (CommonMark 0.31.2 example 262).
				return p.firstStop(n + 1)
			}
		case cols >= ct.indent:
			c.skip(ct.indent)
		default:
			return n
		}
	}
	return len(p.stack)
}

// optionalSpace consumes the one optional space or tab column after a block quote marker.
func (c *cursor) optionalSpace() {
	if c.rem > 0 {
		c.skip(1)
		return
	}
	if c.i < c.end && (c.s[c.i] == ' ' || c.s[c.i] == '\t') {
		c.skip(1)
	}
}

// closeFrom closes the containers from index n on, and with them the open leaf.
func (p *blockParser) closeFrom(n int) {
	p.closeParagraph()
	p.leaf = leafNone
	p.stack = p.stack[:n]
	for len(p.stops) > 0 && p.stops[len(p.stops)-1] >= n {
		p.stops = p.stops[:len(p.stops)-1]
	}
	for len(p.items) > 0 && p.items[len(p.items)-1] >= n {
		p.items = p.items[:len(p.items)-1]
	}
}

// continueLeaf adds the line to an open code block and reports whether it did.
func (p *blockParser) continueLeaf(c *cursor, start, end int, blank bool) bool {
	switch p.leaf {
	case leafFenced:
		p.markLine(start, end)
		if p.closingFence(c) {
			p.leaf = leafNone
		}
		return true
	case leafIndented:
		if cols, _ := c.indent(); blank || cols >= 4 {
			p.markLine(start, end)
			return true
		}
		p.leaf = leafNone
	case leafNone, leafParagraph:
	}
	return false
}

// openContainers starts the block quotes and list items at the cursor.
func (p *blockParser) openContainers(c *cursor) {
	for {
		cols, next := c.indent()
		if cols > 3 || next >= c.end {
			return
		}
		if c.s[next] == '>' {
			p.closeParagraph()
			p.leaf = leafNone
			p.markContent()
			c.skipTo(next)
			c.i++
			c.col++
			c.optionalSpace()
			p.push(container{quote: true})
			continue
		}
		if p.breakAt(c, next) {
			return
		}
		width, ok := listMarker(c.s[next:c.end], p.leaf == leafParagraph)
		if !ok {
			return
		}
		p.closeParagraph()
		p.leaf = leafNone
		p.markContent()
		c.skipTo(next)
		c.i += width
		c.col += width
		spaces, after := c.indent()
		item := container{indent: cols + width + 1}
		switch {
		case after >= c.end:
		case spaces >= 5:
			c.skip(1)
		default:
			item.indent = cols + width + spaces
			c.skip(spaces)
		}
		p.push(item)
	}
}

// openLeaf starts or continues the leaf block of the rest of the line.
func (p *blockParser) openLeaf(c *cursor, start, end int) {
	cols, next := c.indent()
	if next >= c.end {
		p.closeParagraph()
		p.leaf = leafNone
		return
	}
	p.markContent()
	rest := c.s[next:c.end]
	switch {
	case cols >= 4 && p.leaf != leafParagraph:
		p.leaf = leafIndented
		p.markLine(start, end)
	case cols >= 4:
		p.para = append(p.para, segment{next, c.end})
	case p.openFence(rest, cols):
		p.closeParagraph()
		p.leaf = leafFenced
		p.markLine(start, end)
	case atxHeading(rest) || thematicBreak(rest) || p.leaf == leafParagraph && setextUnderline(rest):
		p.closeParagraph()
		p.leaf = leafNone
		if atxHeading(rest) {
			p.inline([]segment{{next, c.end}})
		}
	default:
		p.leaf = leafParagraph
		p.para = append(p.para, segment{next, c.end})
	}
}

// startsBlock reports whether the rest of the line starts a block that may interrupt a
// paragraph; such a line is no lazy continuation (CommonMark "Block quotes", laziness).
func (p *blockParser) startsBlock(c *cursor) bool {
	cols, next := c.indent()
	if cols > 3 {
		return false
	}
	rest := c.s[next:c.end]
	if rest[0] == '>' || thematicBreak(rest) || atxHeading(rest) {
		return true
	}
	if _, _, ok := fenceOpener(rest); ok {
		return true
	}
	_, ok := listMarker(rest, true)
	return ok
}

// rest returns the segment from the first non-space byte of the cursor to the line end.
func (p *blockParser) rest(c *cursor) segment {
	_, next := c.indent()
	return segment{next, c.end}
}

// markContent records that the innermost list item has content.
func (p *blockParser) markContent() {
	if len(p.items) == 0 {
		return
	}
	i := p.items[len(p.items)-1]
	if p.stack[i].hasContent {
		return
	}
	p.stack[i].hasContent = true
	// Only block quotes follow the innermost item, and each of them is a stop, so the item is
	// the stop that many places from the end. Each quote is shifted at most once: only the
	// innermost item before it can gain content while it is open.
	k := len(p.stops) - (len(p.stack) - i)
	p.stops = slices.Delete(p.stops, k, k+1)
}

// firstStop returns the first stop at or after the stack index n, len(stack) when there is none.
func (p *blockParser) firstStop(n int) int {
	k, _ := slices.BinarySearch(p.stops, n)
	if k < len(p.stops) {
		return p.stops[k]
	}
	return len(p.stack)
}

// restBlank reports that the line holds only white space from the cursor on.
func (p *blockParser) restBlank(c *cursor) bool {
	return c.i > p.lastText
}

// openFence starts a fenced code block when rest is an opening fence.
func (p *blockParser) openFence(rest string, cols int) bool {
	ch, n, ok := fenceOpener(rest)
	if !ok {
		return false
	}
	p.fenceChar, p.fenceLen, p.fenceIndent = ch, n, cols
	return true
}

// fenceOpener reads an opening code fence: at least three backticks or tildes; a backtick fence
// has no backtick in its info string.
func fenceOpener(rest string) (ch byte, n int, ok bool) {
	if rest == "" || rest[0] != '`' && rest[0] != '~' {
		return 0, 0, false
	}
	ch = rest[0]
	for n < len(rest) && rest[n] == ch {
		n++
	}
	if n < 3 || ch == '`' && strings.IndexByte(rest[n:], '`') >= 0 {
		return 0, 0, false
	}
	return ch, n, true
}

// closingFence reports whether the line at the cursor closes the open fenced code block: at most
// three columns of indentation, at least as many fence characters, then only white space.
func (p *blockParser) closingFence(c *cursor) bool {
	cols, next := c.indent()
	if cols > 3 {
		return false
	}
	rest := c.s[next:c.end]
	n := 0
	for n < len(rest) && rest[n] == p.fenceChar {
		n++
	}
	return n >= p.fenceLen && strings.Trim(rest[n:], " \t") == ""
}

// thematicBreak reports three or more of one of -, * and _ with only spaces or tabs between.
func thematicBreak(rest string) bool {
	if rest == "" || rest[0] != '-' && rest[0] != '*' && rest[0] != '_' {
		return false
	}
	n := 0
	for i := 0; i < len(rest); i++ {
		switch rest[i] {
		case rest[0]:
			n++
		case ' ', '\t':
		default:
			return false
		}
	}
	return n >= 3
}

// atxHeading reports one to six # followed by a space, a tab or the line end.
func atxHeading(rest string) bool {
	n := 0
	for n < len(rest) && rest[n] == '#' {
		n++
	}
	return n >= 1 && n <= 6 && (n == len(rest) || rest[n] == ' ' || rest[n] == '\t')
}

// setextUnderline reports a line of = or - characters with trailing white space only.
func setextUnderline(rest string) bool {
	if rest == "" || rest[0] != '=' && rest[0] != '-' {
		return false
	}
	t := strings.TrimRight(rest, " \t")
	return strings.Trim(t, rest[:1]) == ""
}

// listMarker reads a bullet (-, + or *) or an ordered marker (one to nine digits and . or ))
// followed by white space or the line end and returns its width. In a paragraph an item may
// interrupt only when it is not empty and, if ordered, starts with 1.
func listMarker(rest string, inParagraph bool) (int, bool) {
	width, ok := markerWidth(rest, inParagraph)
	if !ok || width < len(rest) && rest[width] != ' ' && rest[width] != '\t' {
		return 0, false
	}
	if inParagraph && strings.Trim(rest[width:], " \t") == "" {
		return 0, false
	}
	return width, true
}

// markerWidth returns the width of the bullet or ordered marker at the start of rest; in a
// paragraph an ordered marker must be 1.
func markerWidth(rest string, inParagraph bool) (int, bool) {
	if rest[0] == '-' || rest[0] == '+' || rest[0] == '*' {
		return 1, true
	}
	width := 0
	for width < len(rest) && width < 9 && isASCIIDigit(rest[width]) {
		width++
	}
	if width == 0 || width == len(rest) || rest[width] != '.' && rest[width] != ')' || inParagraph && rest[:width] != "1" {
		return 0, false
	}
	return width + 1, true
}

// markLine marks the line [start, end) as code.
func (p *blockParser) markLine(start, end int) {
	for i := start; i < end; i++ {
		p.mask[i] = true
	}
}

// closeParagraph scans the open paragraph for code spans and ends it.
func (p *blockParser) closeParagraph() {
	if len(p.para) > 0 {
		p.inline(p.para)
	}
	p.para = nil
	if p.leaf == leafParagraph {
		p.leaf = leafNone
	}
}

// inline marks the code spans of one block of inline content made of the source segments segs,
// joined by line breaks: backslash escapes and autolinks come first where they start, then a
// backtick string opens a span that the next backtick string of the same length closes
// (CommonMark "Code spans").
func (p *blockParser) inline(segs []segment) {
	t, at := p.join(segs)
	for i := 0; i < len(t); {
		i = p.inlineStep(t, at, i)
	}
}

// join returns the text of segs joined by line breaks and the source offset of each byte; a line
// break maps to the end of the line before it.
func (p *blockParser) join(segs []segment) (string, []int) {
	var text []byte
	var at []int
	prevEnd := -1
	for _, s := range segs {
		if prevEnd >= 0 {
			text = append(text, '\n')
			at = append(at, prevEnd)
		}
		for i := s.start; i < s.end; i++ {
			text = append(text, p.src[i])
			at = append(at, i)
		}
		prevEnd = s.end
	}
	return string(text), at
}

// inlineStep handles the inline construct at i of t and returns the index after it, marking a
// code span in the mask through at.
func (p *blockParser) inlineStep(t string, at []int, i int) int {
	switch {
	case t[i] == '\\' && i+1 < len(t) && asciiPunct(t[i+1]):
		return i + 2
	case t[i] == '<':
		return i + max(autolink(t[i:]), 1)
	case t[i] == '`':
		n := run(t, i)
		closeAt := closingRun(t, i+n, n)
		if closeAt < 0 {
			return i + n
		}
		for k := i; k < closeAt+n; k++ {
			p.mask[at[k]] = true
		}
		return closeAt + n
	}
	return i + 1
}

// run returns the length of the backtick string at i.
func run(t string, i int) int {
	n := 0
	for i+n < len(t) && t[i+n] == '`' {
		n++
	}
	return n
}

// closingRun returns the index of the next backtick string of exactly n backticks from i, or -1.
func closingRun(t string, i, n int) int {
	for i < len(t) {
		if t[i] != '`' {
			i++
			continue
		}
		m := run(t, i)
		if m == n {
			return i
		}
		i += m
	}
	return -1
}

// asciiPunct reports an ASCII punctuation character (CommonMark "Backslash escapes").
func asciiPunct(b byte) bool {
	return b >= '!' && b <= '/' || b >= ':' && b <= '@' || b >= '[' && b <= '`' || b >= '{' && b <= '~'
}

// autolink returns the length of the URI or e-mail autolink at the start of t, 0 for none.
func autolink(t string) int {
	if n := uriAutolink(t); n > 0 {
		return n
	}
	return emailAutolink(t)
}

// uriAutolink matches "<" scheme ":" then no white space, control character, < or > up to ">".
func uriAutolink(t string) int {
	i := 1
	for i < len(t) && i <= 33 && (isASCIILetter(t[i]) || i > 1 && (isASCIIDigit(t[i]) || t[i] == '+' || t[i] == '.' || t[i] == '-')) {
		i++
	}
	if i < 3 || i > 33 || i >= len(t) || t[i] != ':' {
		return 0
	}
	for i++; i < len(t); i++ {
		switch b := t[i]; {
		case b == '>':
			return i + 1
		case b == '<' || b <= ' ' || b == 0x7f:
			return 0
		}
	}
	return 0
}

// emailAutolink matches "<" local part "@" domain labels ">".
func emailAutolink(t string) int {
	i := 1
	for i < len(t) && (isASCIILetter(t[i]) || isASCIIDigit(t[i]) || strings.IndexByte(".!#$%&'*+/=?^_`{|}~-", t[i]) >= 0) {
		i++
	}
	if i == 1 || i >= len(t) || t[i] != '@' {
		return 0
	}
	for {
		end := domainLabel(t, i+1)
		if end < 0 || end >= len(t) {
			return 0
		}
		switch t[end] {
		case '>':
			return end + 1
		case '.':
			i = end
		default:
			return 0
		}
	}
}

// domainLabel returns the end of the domain label of at most 63 letters, digits and inner hyphens
// that starts at start, or -1 when there is none.
func domainLabel(t string, start int) int {
	i := start
	for i < len(t) && i-start < 63 && (isASCIILetter(t[i]) || isASCIIDigit(t[i]) || t[i] == '-' && i > start) {
		i++
	}
	if i == start || t[i-1] == '-' {
		return -1
	}
	return i
}

func isASCIILetter(b byte) bool { return b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z' }

func isASCIIDigit(b byte) bool { return b >= '0' && b <= '9' }
