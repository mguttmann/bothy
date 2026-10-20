// SPDX-License-Identifier: Apache-2.0

// Package searchlang holds the validated language configuration of full-text search
// (docs/spec/14-search.md 3.1 "Language per object"): the configuration keys search.languages, the
// text search configurations every query is built with, and search.locale_map, which maps the
// first language subtag of a BCP 47 locale to the configuration a new message or thread is
// indexed with. It is part of the kernel itself (standard library only): identity uses it to
// answer the language of a principal, wire to check the configuration at start, and the search
// coordinator to build its queries (docs/DECISIONS.md D-M3-13-3).
//
// Example:
//
//	m, err := searchlang.New([]string{"german", "english"}, map[string]string{"de": "german"})
//	if err != nil {
//		return err // names search.languages or search.locale_map and a position, never a value
//	}
//	m.For("de-AT") // "german"
//	m.For("fr")    // "simple"
package searchlang
