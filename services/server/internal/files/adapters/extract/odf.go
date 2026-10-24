// SPDX-License-Identifier: Apache-2.0

package extract

import "github.com/OWNER/bothy/services/server/internal/files/domain"

// odf reads content.xml of ODT, ODS and ODP (missing: archive_invalid): inside body all
// character data; s writes a space, tab a tab, line-break and the end of p and h a line feed; the
// subtrees annotation and tracked-changes (comments and deleted revisions) are skipped.
var odf = format{
	parts: func(a *archive) ([]string, error) {
		if _, ok := a.parts[partODF]; !ok {
			return nil, domain.NewExtractError(domain.CodeArchiveInvalid)
		}
		return []string{partODF}, nil
	},
	handler: func() xmlHandler {
		return &textIn{within: "body", skip: map[string]bool{"annotation": true, "tracked-changes": true},
			startSep: map[string]rune{"s": ' ', "tab": '\t', "line-break": '\n'},
			endSep:   map[string]rune{"p": '\n', "h": '\n'}}
	},
}
