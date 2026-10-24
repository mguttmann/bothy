// SPDX-License-Identifier: Apache-2.0

package extract

import (
	"context"
	"errors"
	"io"

	"golang.org/x/net/html"

	"github.com/OWNER/bothy/services/server/internal/files/domain"
)

// blockElements write a line feed at their start and end tags (brief M3-15 design note 5); every
// other element writes nothing, so Rech<b>nung</b> stays one word.
var blockElements = map[string]bool{
	"address": true, "article": true, "aside": true, "blockquote": true, "br": true, "dd": true, "div": true,
	"dl": true, "dt": true, "fieldset": true, "figcaption": true, "figure": true, "footer": true, "form": true,
	"h1": true, "h2": true, "h3": true, "h4": true, "h5": true, "h6": true, "header": true, "hr": true, "li": true,
	"main": true, "nav": true, "ol": true, "p": true, "pre": true, "section": true, "table": true, "tbody": true,
	"td": true, "tfoot": true, "th": true, "thead": true, "title": true, "tr": true, "ul": true,
}

// extractHTML is builtin-html (14 5.2 row 2): the tokenizer of golang.org/x/net/html gives the
// text tokens with entities decoded; the content of script and style, comments and the doctype
// are dropped. No charset conversion: the bytes are read as UTF-8 through the builder. Broken
// markup still gives its text, because the tokenizer never fails on syntax.
func extractHTML(ctx context.Context, r io.Reader, b *domain.TextBuilder) error {
	z := html.NewTokenizer(r)
	st := htmlState{b: b}
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		if b.Truncated() {
			return nil
		}
		tt := z.Next()
		switch tt {
		case html.ErrorToken:
			if err := z.Err(); !errors.Is(err, io.EOF) {
				return readError(err)
			}
			return nil
		case html.TextToken:
			if st.skip == "" {
				b.WriteText(string(z.Text()))
			}
		case html.StartTagToken, html.SelfClosingTagToken, html.EndTagToken:
			name, _ := z.TagName()
			st.tag(tt, string(name))
		case html.CommentToken, html.DoctypeToken:
		}
	}
}

// htmlState is the state of the token loop: skip names the script or style element whose
// content is being dropped.
type htmlState struct {
	b    *domain.TextBuilder
	skip string
}

// tag handles a start, self-closing or end tag.
func (s *htmlState) tag(tt html.TokenType, tag string) {
	switch {
	case s.skip != "":
		if tt == html.EndTagToken && tag == s.skip {
			s.skip = ""
		}
	case (tag == "script" || tag == "style") && tt == html.StartTagToken:
		s.skip = tag
	case blockElements[tag]:
		s.b.Separator('\n')
	}
}
