// SPDX-License-Identifier: Apache-2.0

// Package mimetype is the adapter of the files module for the extensions of a media type, used by
// the early check of file.denied_mime_types (docs/spec/12-files.md 6.1, docs/DECISIONS.md
// D-M3-03-5). It reads the tables of github.com/gabriel-vasile/mimetype, the library 12 6.1
// names, instead of mime.ExtensionsByType, which reads tables of the operating system and differs
// between hosts. It implements app.TypeExtensions.
//
// Example:
//
//	exts := mimetype.Extensions{}.Extensions("image/png") // [".png"]
package mimetype

import (
	"strings"

	"github.com/gabriel-vasile/mimetype"

	"github.com/OWNER/bothy/services/server/internal/files/app"
)

// Extensions returns the extensions the library knows for a media type. The zero value is ready
// to use.
type Extensions struct{}

// Compile-time check: Extensions is the outgoing port of app.
var _ app.TypeExtensions = Extensions{}

// Extensions implements app.TypeExtensions: the extension of the type found by mimetype.Lookup
// (the main type or one of its aliases), with its leading dot and in lower case; nil for an
// unknown type or a type without extension. The library keeps one extension per type.
func (Extensions) Extensions(mediaType string) []string {
	t := mimetype.Lookup(strings.ToLower(mediaType))
	if t == nil || t.Extension() == "" {
		return nil
	}
	return []string{strings.ToLower(t.Extension())}
}
