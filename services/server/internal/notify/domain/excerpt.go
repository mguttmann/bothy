// SPDX-License-Identifier: Apache-2.0

package domain

import "strings"

// ellipsis ends a cut excerpt (13 section 6.4, brief M3-22 design note 10 d).
const ellipsis = '…'

// excerptInputFactor bounds the part of content_md the excerpt parses to this many times
// notify.preview_max_chars code points (excerptSource). The result has at most maxChars code
// points, and the factor leaves room for markup that disappears (link destinations, markers), so
// the cost of a call depends on notify.preview_max_chars only, never on the length of the message:
// the scans of links, code spans and delimiter runs look ahead within a line and would otherwise
// grow with the square of a long line (docs/DECISIONS.md D-M3-22-10).
const excerptInputFactor = 8

// Excerpt returns the text of a preview (13 section 6.4: "text without Markdown, mentions as
// names, at most notify.preview_max_chars"; brief M3-22 design note 10, docs/DECISIONS.md
// D-M3-22-10). contentMD is messages.content_md of content_format 1, mentionNames maps the lower
// case handle of every principal in the message's mentions rows to its display name, maxChars is
// notify.preview_max_chars. The steps:
//
//  1. Mentions: an @handle token (the token rule of 03 "content format": start of text or a
//     character that is no letter, digit, _, ., -, @ or /; trailing ., - and _ not part of the
//     handle) whose handle is in mentionNames becomes the display name; any other @ text stays.
//     Tokens in code stay as they are.
//  2. Markdown: fence lines are dropped and the code text kept; the backticks of inline code,
//     paired emphasis, strong and strike markers are removed; an inline link becomes its text, an
//     image its alt text, an autolink in angle brackets its address; leading heading, quote and
//     list markers, thematic breaks, table pipes and table separator rows are removed; backslash
//     escapes are resolved; HTML stays as literal text.
//  3. Every control and bidi control character is removed, every run of white space (line breaks
//     included) becomes one space, and the result is trimmed.
//  4. A result longer than maxChars code points is cut to maxChars - 1 code points followed by
//     U+2026; no code point is split. maxChars below 1 gives "".
//
// Only the first excerptInputFactor * maxChars code points of contentMD are parsed, so the cost of
// a call is bounded by maxChars; a construct cut at that bound stays as literal text.
//
// The result is valid UTF-8 (an invalid byte becomes U+FFFD). Excerpt never fails.
//
// Example:
//
//	domain.Excerpt("**@sam** see [the doc](https://example.com)", map[string]string{"sam": "Sam"}, 240) // "Sam see the doc"
func Excerpt(contentMD string, mentionNames map[string]string, maxChars int) string {
	if maxChars < 1 {
		return ""
	}
	var b strings.Builder
	fence := ""
	for _, line := range strings.Split(strings.ReplaceAll(excerptSource(contentMD, maxChars), "\r\n", "\n"), "\n") {
		if marker, ok := fenceMarker(line); ok {
			switch {
			case fence == "":
				fence = marker
			case strings.HasPrefix(strings.TrimSpace(line), fence) && strings.Trim(strings.TrimSpace(line), fence[:1]) == "":
				fence = ""
			default:
				b.WriteString(line)
				b.WriteByte(' ')
			}
			continue
		}
		if fence != "" {
			b.WriteString(line)
			b.WriteByte(' ')
			continue
		}
		b.WriteString(inline([]rune(blockText(line)), mentionNames))
		b.WriteByte(' ')
	}
	return cut(clean(b.String()), maxChars)
}

// excerptSource returns the prefix of content of at most excerptInputFactor * maxChars code points
// (an invalid byte counts as one), the only part Excerpt parses; a shorter content is returned
// whole.
func excerptSource(content string, maxChars int) string {
	limit, n := maxChars*excerptInputFactor, 0
	for i := range content {
		if n == limit {
			return content[:i]
		}
		n++
	}
	return content
}

// fenceMarker reports a fence line (at most three spaces, then at least three backticks or
// tildes) and returns its marker run.
func fenceMarker(line string) (string, bool) {
	t := strings.TrimLeft(line, " ")
	if len(line)-len(t) > 3 || len(t) < 3 || (t[0] != '`' && t[0] != '~') {
		return "", false
	}
	n := 0
	for n < len(t) && t[n] == t[0] {
		n++
	}
	if n < 3 || (t[0] == '`' && strings.ContainsRune(t[n:], '`')) {
		return "", false
	}
	return t[:n], true
}

// blockText removes the block markers at the start of line: quote, heading, list and task
// markers, a thematic break and table pipes, and drops a table separator row.
func blockText(line string) string {
	t := strings.TrimLeft(line, " \t")
	for {
		switch {
		case strings.HasPrefix(t, ">"):
			t = strings.TrimLeft(t[1:], " \t")
			continue
		case listMarker(t) > 0:
			t = strings.TrimLeft(t[listMarker(t):], " \t")
			for _, box := range []string{"[ ] ", "[x] ", "[X] "} {
				t = strings.TrimPrefix(t, box)
			}
			continue
		}
		break
	}
	if thematicBreak(t) {
		return ""
	}
	if h := headingMarker(t); h > 0 {
		t = strings.TrimRight(strings.TrimLeft(t[h:], " \t"), " \t")
		if c := strings.TrimRight(t, "#"); c != t && (c == "" || strings.HasSuffix(c, " ")) {
			t = c
		}
	}
	if strings.HasPrefix(t, "|") {
		if strings.Trim(t, "|-: \t") == "" {
			return ""
		}
		t = tablePipes(t)
	}
	return t
}

// listMarker returns the length of a bullet (-, *, +) or ordered (digits and . or )) list
// marker followed by a space at the start of t, or 0.
func listMarker(t string) int {
	if len(t) >= 2 && strings.ContainsRune("-*+", rune(t[0])) && (t[1] == ' ' || t[1] == '\t') {
		return 1
	}
	n := 0
	for n < len(t) && n < 9 && t[n] >= '0' && t[n] <= '9' {
		n++
	}
	if n > 0 && n+1 < len(t) && (t[n] == '.' || t[n] == ')') && (t[n+1] == ' ' || t[n+1] == '\t') {
		return n + 1
	}
	return 0
}

// headingMarker returns the length of an ATX heading marker (1 to 6 #, then a space or the end)
// at the start of t, or 0.
func headingMarker(t string) int {
	n := 0
	for n < len(t) && t[n] == '#' {
		n++
	}
	if n == 0 || n > 6 || (n < len(t) && t[n] != ' ' && t[n] != '\t') {
		return 0
	}
	return n
}

// thematicBreak reports a line of at least three -, * or _ of one kind and spaces.
func thematicBreak(t string) bool {
	s := strings.ReplaceAll(strings.ReplaceAll(t, " ", ""), "\t", "")
	return len(s) >= 3 && strings.Trim(s, s[:1]) == "" && strings.ContainsRune("-*_", rune(s[0]))
}

// tablePipes replaces every pipe of a table row that is neither escaped nor in a code span by a
// space.
func tablePipes(t string) string {
	out := []byte(t)
	code := false
	for i := 0; i < len(out); i++ {
		switch {
		case out[i] == '\\' && i+1 < len(out):
			i++
		case out[i] == '`':
			code = !code
		case out[i] == '|' && !code:
			out[i] = ' '
		}
	}
	return string(out)
}

// token is one piece of an inline line: text that markup never changes (lit), or a run of an
// emphasis or strike delimiter (delim with its length n) that pairing may remove.
type token struct {
	text  []rune
	delim rune
	n     int
	lit   bool
	// open and closeOK are the flanking facts of a delimiter run.
	open, closeOK bool
	removed       int
}

// inline converts one line of inline Markdown into plain text (steps 1 and 2 of Excerpt).
func inline(line []rune, names map[string]string) string {
	toks := scan(line, names)
	pair(toks)
	var b strings.Builder
	for _, t := range toks {
		if t.delim != 0 {
			for i := t.removed; i < t.n; i++ {
				b.WriteRune(t.delim)
			}
			continue
		}
		b.WriteString(string(t.text))
	}
	return b.String()
}

// scan splits line into tokens: escapes, code spans, links, images, autolinks and mentions become
// literal text, runs of *, _ and ~ become delimiter runs, everything else literal text.
func scan(line []rune, names map[string]string) []token {
	var toks []token
	for i := 0; i < len(line); {
		if r := line[i]; r == '*' || r == '_' || r == '~' {
			j := i
			for j < len(line) && line[j] == r {
				j++
			}
			toks = append(toks, flank(line, i, j))
			i = j
			continue
		}
		text, next := inlineAt(line, i, names)
		toks = append(toks, token{text: text, lit: true})
		i = next
	}
	return toks
}

// inlineAt returns the literal text of the construct that starts at line[i] (an escape, a code
// span, an image, a link, an autolink or a mention; otherwise the character itself) and the index
// after it.
func inlineAt(line []rune, i int, names map[string]string) ([]rune, int) {
	r := line[i]
	switch {
	case r == '\\' && i+1 < len(line) && asciiPunct(line[i+1]):
		return line[i+1 : i+2], i + 2
	case r == '`':
		n, end, content := codeSpan(line, i)
		if end < 0 {
			return line[i : i+n], i + n
		}
		return content, end
	case r == '!' && i+1 < len(line) && line[i+1] == '[':
		return linkAt(line, i+1, i, names)
	case r == '[':
		return linkAt(line, i, i, names)
	case r == '<':
		if addr, end, ok := autolink(line, i); ok {
			return addr, end
		}
	case r == '@':
		if name, end, ok := mention(line, i, names); ok {
			return []rune(name), end
		}
	}
	return line[i : i+1], i + 1
}

// linkAt returns the plain link text of the link whose [ is at line[at] and the index after it;
// without a link there it returns the character at start alone.
func linkAt(line []rune, at, start int, names map[string]string) ([]rune, int) {
	if text, end, ok := link(line, at); ok {
		return []rune(inline(text, names)), end
	}
	return line[start : start+1], start + 1
}

// flank returns the delimiter run line[i:j] with its flanking facts: it may open when the next
// character is no white space, close when the previous one is none; an underscore run also must
// not touch a letter or digit on the outer side (intraword underscores stay).
func flank(line []rune, i, j int) token {
	t := token{delim: line[i], n: j - i}
	before, after := ' ', ' '
	if i > 0 {
		before = line[i-1]
	}
	if j < len(line) {
		after = line[j]
	}
	t.open, t.closeOK = !space(after), !space(before)
	if t.delim == '_' {
		t.open = t.open && !alnum(before)
		t.closeOK = t.closeOK && !alnum(after)
	}
	return t
}

// pair removes the markers of paired delimiter runs: a run that may close takes the nearest
// earlier open run of the same character (strike: of the same length); both lose the shorter
// length (strike: the whole run). Unpaired runs stay as literal text.
func pair(toks []token) {
	var stack []int
	for i := range toks {
		t := &toks[i]
		if t.delim == 0 {
			continue
		}
		if t.closeOK {
			if k := opener(toks, stack, t); k >= 0 {
				idx := stack[k]
				o := &toks[idx]
				use := min(o.n-o.removed, t.n)
				o.removed += use
				t.removed += use
				stack = stack[:k]
				if o.removed < o.n {
					stack = append(stack, idx)
				}
				continue
			}
		}
		if t.open {
			stack = append(stack, i)
		}
	}
}

// opener returns the position in stack of the nearest open run that closer pairs with, or -1.
func opener(toks []token, stack []int, closer *token) int {
	for k := len(stack) - 1; k >= 0; k-- {
		o := toks[stack[k]]
		if o.delim != closer.delim {
			continue
		}
		if o.delim == '~' && o.n-o.removed != closer.n {
			continue
		}
		return k
	}
	return -1
}

// codeSpan reads the backtick run at i and its closing run of the same length. It returns the
// length n of the opening run, the index after the closing run (or -1 without one) and the content
// with one leading and trailing space removed when both are there.
func codeSpan(line []rune, i int) (n, end int, content []rune) {
	for i+n < len(line) && line[i+n] == '`' {
		n++
	}
	for j := i + n; j < len(line); {
		if line[j] != '`' {
			j++
			continue
		}
		k := j
		for k < len(line) && line[k] == '`' {
			k++
		}
		if k-j == n {
			c := line[i+n : j]
			if len(c) >= 2 && c[0] == ' ' && c[len(c)-1] == ' ' {
				c = c[1 : len(c)-1]
			}
			return n, k, c
		}
		j = k
	}
	return n, -1, nil
}

// link reads an inline link at the [ at i: [text](destination). It returns the text, the index
// after the closing parenthesis and true; false when line has no such link there.
func link(line []rune, i int) (text []rune, end int, ok bool) {
	j := closingBracket(line, i)
	if j < 0 || j+1 >= len(line) || line[j+1] != '(' {
		return nil, 0, false
	}
	k := closingParen(line, j+1)
	if k < 0 {
		return nil, 0, false
	}
	return line[i+1 : j], k + 1, true
}

// closingBracket returns the index of the ] that closes the [ at i (nested brackets and escaped
// characters skipped), or -1.
func closingBracket(line []rune, i int) int {
	depth := 0
	for j := i; j < len(line); j++ {
		switch line[j] {
		case '\\':
			j++
		case '[':
			depth++
		case ']':
			depth--
			if depth == 0 {
				return j
			}
		}
	}
	return -1
}

// closingParen returns the index of the ) that closes the ( at i (nested parentheses counted), or
// -1.
func closingParen(line []rune, i int) int {
	paren := 0
	for k := i; k < len(line); k++ {
		switch line[k] {
		case '(':
			paren++
		case ')':
			paren--
			if paren == 0 {
				return k
			}
		}
	}
	return -1
}

// autolink reads <scheme:address> or <local@domain> at i and returns the address and the index
// after the >; HTML tags and anything else are no autolink.
func autolink(line []rune, i int) (addr []rune, end int, ok bool) {
	for j := i + 1; j < len(line); j++ {
		r := line[j]
		if r == '>' {
			a := line[i+1 : j]
			if uriAutolink(a) || emailAutolink(a) {
				return a, j + 1, true
			}
			return nil, 0, false
		}
		if r == '<' || space(r) {
			return nil, 0, false
		}
	}
	return nil, 0, false
}

// uriAutolink reports a scheme of 2 to 32 letters, digits, +, . or - starting with a letter,
// then a colon.
func uriAutolink(a []rune) bool {
	c := -1
	for k, r := range a {
		if r == ':' {
			c = k
			break
		}
	}
	if c < 2 || c > 32 || !letter(a[0]) {
		return false
	}
	for _, r := range a[1:c] {
		if !alnum(r) && r != '+' && r != '.' && r != '-' {
			return false
		}
	}
	return true
}

// emailAutolink reports local@domain with an ASCII local part and a dotted domain.
func emailAutolink(a []rune) bool {
	at := -1
	for k, r := range a {
		if r == '@' {
			at = k
			break
		}
	}
	if at < 1 || at == len(a)-1 {
		return false
	}
	for _, r := range a {
		if r > 0x7e || r <= ' ' {
			return false
		}
	}
	return strings.ContainsRune(string(a[at+1:]), '.')
}

// mention reads the mention token at the @ at i. When its handle is in names it returns the
// display name, the index after the handle and true.
func mention(line []rune, i int, names map[string]string) (string, int, bool) {
	if i > 0 && !mentionMayFollow(line[i-1]) || i+1 >= len(line) || !handleStart(line[i+1]) {
		return "", 0, false
	}
	k := i + 2
	for k < len(line) && (handleStart(line[k]) || line[k] == '.' || line[k] == '_' || line[k] == '-') {
		k++
	}
	for k > i+2 && strings.ContainsRune("._-", line[k-1]) {
		k--
	}
	name, ok := names[strings.ToLower(string(line[i+1:k]))]
	if !ok || name == "" {
		return "", 0, false
	}
	return name, k, true
}

// mentionMayFollow reports a character after which a mention may start: no letter, digit, _, .,
// -, @ or /. A non-ASCII character counts as a letter, which never names a person wrongly.
func mentionMayFollow(r rune) bool {
	return r < 0x80 && !alnum(r) && !strings.ContainsRune("_.-@/", r)
}

// handleStart reports an ASCII letter or digit.
func handleStart(r rune) bool { return alnum(r) }

// clean removes every control and bidi control character, turns every run of white space into
// one space and trims the result (step 3 of Excerpt). Ranging over the string turns an invalid
// byte into U+FFFD, so the result is valid UTF-8.
func clean(s string) string {
	var b strings.Builder
	pending := false
	for _, r := range s {
		switch {
		case space(r):
			pending = b.Len() > 0
			continue
		case control(r) || bidi(r):
			continue
		}
		if pending {
			b.WriteByte(' ')
			pending = false
		}
		b.WriteRune(r)
	}
	return b.String()
}

// cut returns s when it has at most maxChars code points, otherwise its first maxChars - 1 code
// points and U+2026 (step 4 of Excerpt).
func cut(s string, maxChars int) string {
	rs := []rune(s)
	if len(rs) <= maxChars {
		return s
	}
	return strings.TrimRight(string(rs[:maxChars-1]), " ") + string(ellipsis)
}

// space reports the white space of Unicode (the Zs category, the line and paragraph separators
// and the white space controls).
func space(r rune) bool {
	switch r {
	case ' ', '\t', '\n', '\v', '\f', '\r', 0x85, 0xa0, 0x1680, 0x2028, 0x2029, 0x202f, 0x205f, 0x3000:
		return true
	}
	return r >= 0x2000 && r <= 0x200a
}

// control reports a character of the Unicode category Cc.
func control(r rune) bool { return r < 0x20 || (r >= 0x7f && r <= 0x9f) }

// bidi reports a bidi control character (Unicode Bidi_Control).
func bidi(r rune) bool {
	return r == 0x061c || r == 0x200e || r == 0x200f || (r >= 0x202a && r <= 0x202e) || (r >= 0x2066 && r <= 0x2069)
}

// asciiPunct reports an ASCII punctuation character, the ones a backslash escapes.
func asciiPunct(r rune) bool {
	return r >= '!' && r <= '/' || r >= ':' && r <= '@' || r >= '[' && r <= '`' || r >= '{' && r <= '~'
}

// letter reports an ASCII letter.
func letter(r rune) bool { return r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' }

// alnum reports an ASCII letter or digit.
func alnum(r rune) bool { return letter(r) || r >= '0' && r <= '9' }
