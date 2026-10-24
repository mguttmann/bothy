// SPDX-License-Identifier: Apache-2.0

package extract

import (
	"sort"
	"strconv"
	"strings"

	"github.com/OWNER/bothy/services/server/internal/files/domain"
)

// The fixed parts of OOXML (brief M3-15 design note 7). Fixed names instead of the package
// relationships keep the parser surface small: a producer that uses other names gives
// archive_invalid or empty text, never a wrong text.
const (
	partDOCX    = "word/document.xml"
	partXLSX    = "xl/sharedStrings.xml"
	slidePrefix = "ppt/slides/slide"
	slideSuffix = ".xml"
	partODF     = "content.xml"
)

// docx reads word/document.xml (missing: archive_invalid): the text of t elements; tab writes a
// tab, br and cr a line feed, the end of p a line feed. delText and instrText are other elements,
// so deleted revisions and field codes stay out.
var docx = format{
	parts: func(a *archive) ([]string, error) {
		if _, ok := a.parts[partDOCX]; !ok {
			return nil, domain.NewExtractError(domain.CodeArchiveInvalid)
		}
		return []string{partDOCX}, nil
	},
	handler: func() xmlHandler {
		return &textIn{textElem: "t", startSep: map[string]rune{"tab": '\t', "br": '\n', "cr": '\n'},
			endSep: map[string]rune{"p": '\n'}}
	},
}

// xlsx reads xl/sharedStrings.xml (missing: empty text, a workbook may hold only numbers): the
// text of t elements outside rPh (phonetic runs), a line feed after each si.
var xlsx = format{
	parts: func(a *archive) ([]string, error) {
		if _, ok := a.parts[partXLSX]; !ok {
			return nil, nil
		}
		return []string{partXLSX}, nil
	},
	handler: func() xmlHandler {
		return &textIn{textElem: "t", skip: map[string]bool{"rPh": true}, endSep: map[string]rune{"si": '\n'}}
	},
}

// pptx reads every ppt/slides/slide<N>.xml with a decimal N in ascending numeric order (slide2
// before slide10): the text of t elements, a line feed after each p and between slides.
var pptx = format{
	parts: func(a *archive) ([]string, error) {
		type slide struct {
			n    int
			name string
		}
		var slides []slide
		for _, name := range a.names() {
			num, ok := strings.CutPrefix(name, slidePrefix)
			if !ok {
				continue
			}
			num, ok = strings.CutSuffix(num, slideSuffix)
			if !ok || num == "" || strings.Trim(num, "0123456789") != "" {
				continue
			}
			n, err := strconv.Atoi(num)
			if err != nil {
				continue
			}
			slides = append(slides, slide{n, name})
		}
		sort.Slice(slides, func(i, j int) bool {
			if slides[i].n != slides[j].n {
				return slides[i].n < slides[j].n
			}
			return slides[i].name < slides[j].name
		})
		out := make([]string, 0, len(slides))
		for _, s := range slides {
			out = append(out, s.name)
		}
		return out, nil
	},
	handler: func() xmlHandler {
		return &textIn{textElem: "t", endSep: map[string]rune{"p": '\n'}}
	},
}
