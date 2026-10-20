// SPDX-License-Identifier: Apache-2.0

package searchlang

import (
	"fmt"
	"maps"
	"slices"
	"strings"
)

// Simple is the configuration that keeps words unchanged; every vector and every query has it
// (docs/spec/14-search.md 3.2 and 4.2).
const Simple = "simple"

// The configuration keys the errors of New name.
const (
	keyLanguages = "search.languages"
	keyLocaleMap = "search.locale_map"
)

// maxIdentifierBytes is the longest identifier PostgreSQL keeps unchanged (NAMEDATALEN - 1); a
// longer name would be cut and could then name another configuration.
const maxIdentifierBytes = 63

// Map is the validated language configuration of 14 3.1 "Language per object". The zero value maps
// every locale to Simple and has no languages. A Map never changes after New, so it is safe for
// concurrent use.
type Map struct {
	languages []string
	locales   map[string]string
}

// New validates search.languages (languages) and search.locale_map (localeMap) and returns the
// Map. It copies both arguments.
//
// Rules (docs/DECISIONS.md D-M3-13-5): every language is an unqualified PostgreSQL identifier in
// lower case (letters, digits and "_", not starting with a digit, at most 63 bytes), unique, and
// not Simple, which every query adds itself. Every key of localeMap is a first language subtag of
// BCP 47 in lower-case ASCII letters (two to eight); every value is a valid configuration name
// that is listed in languages or equals Simple, so a query always finds the stems a vector holds.
//
// The error names the key and the position of the first entry that breaks a rule (1-based, the
// map in key order), never a value: a configured value can be a secret in the wrong key
// (docs/spec/02-architecture.md "Configuration loading" item 4).
func New(languages []string, localeMap map[string]string) (Map, error) {
	seen := make(map[string]bool, len(languages))
	for i, l := range languages {
		switch {
		case !validName(l):
			return Map{}, fmt.Errorf("%s: entry %d is not an unqualified lower-case PostgreSQL identifier of at most %d bytes",
				keyLanguages, i+1, maxIdentifierBytes)
		case l == Simple:
			return Map{}, fmt.Errorf("%s: entry %d is %s, which every query adds itself", keyLanguages, i+1, Simple)
		case seen[l]:
			return Map{}, fmt.Errorf("%s: entry %d repeats an earlier entry", keyLanguages, i+1)
		}
		seen[l] = true
	}
	for i, k := range slices.Sorted(maps.Keys(localeMap)) {
		v := localeMap[k]
		switch {
		case !validSubtag(k):
			return Map{}, fmt.Errorf("%s: the key of entry %d (in key order) is not a first language subtag in lower-case ASCII letters",
				keyLocaleMap, i+1)
		case !validName(v):
			return Map{}, fmt.Errorf("%s: the value of entry %d (in key order) is not an unqualified lower-case PostgreSQL identifier",
				keyLocaleMap, i+1)
		case v != Simple && !seen[v]:
			return Map{}, fmt.Errorf("%s: the value of entry %d (in key order) is missing from %s; queries would never find its stems",
				keyLocaleMap, i+1, keyLanguages)
		}
	}
	return Map{languages: slices.Clone(languages), locales: maps.Clone(localeMap)}, nil
}

// For returns the configuration for a BCP 47 locale: the entry of the map for its first language
// subtag (the part before the first "-", ASCII lower case), or Simple when there is none. For
// example "de-AT" and "DE-at" give the entry of "de"; "", "-" and "fr" without an entry give
// Simple.
func (m Map) For(locale string) string {
	subtag, _, _ := strings.Cut(locale, "-")
	if v, ok := m.locales[asciiLower(subtag)]; ok {
		return v
	}
	return Simple
}

// Languages returns search.languages in configuration order, without Simple: the languages a
// query is built with besides Simple (14 4.2). The result is a copy and never nil.
func (m Map) Languages() []string {
	if m.languages == nil {
		return []string{}
	}
	return slices.Clone(m.languages)
}

// Configs returns every configuration name the server uses: Languages, the values of the locale
// map and Simple, sorted and without duplicates. The start check of the database
// (pg.CheckTextSearch) asks for each of them.
func (m Map) Configs() []string {
	out := append(slices.Clone(m.languages), Simple)
	for _, v := range m.locales {
		out = append(out, v)
	}
	slices.Sort(out)
	return slices.Compact(out)
}

// LocaleIndex returns the position (1-based, keys in sorted order) of the first entry of the
// locale map whose value is config, or 0 when no entry has it. Start errors use it to name the
// entry of search.locale_map that holds a configuration without printing the configuration.
func (m Map) LocaleIndex(config string) int {
	for i, k := range slices.Sorted(maps.Keys(m.locales)) {
		if m.locales[k] == config {
			return i + 1
		}
	}
	return 0
}

// validName reports whether s is an unqualified PostgreSQL identifier in lower case that needs no
// quoting and keeps its length.
func validName(s string) bool {
	if s == "" || len(s) > maxIdentifierBytes || (s[0] >= '0' && s[0] <= '9') {
		return false
	}
	for i := range len(s) {
		c := s[i]
		if (c < 'a' || c > 'z') && (c < '0' || c > '9') && c != '_' {
			return false
		}
	}
	return true
}

// validSubtag reports whether s is a primary language subtag of BCP 47 (RFC 5646 section 2.1,
// two to eight letters) in lower-case ASCII.
func validSubtag(s string) bool {
	if len(s) < 2 || len(s) > 8 {
		return false
	}
	for i := range len(s) {
		if s[i] < 'a' || s[i] > 'z' {
			return false
		}
	}
	return true
}

// asciiLower lowers the ASCII letters of s only, so a non-ASCII letter never folds into a key.
func asciiLower(s string) string {
	b := []byte(s)
	for i, c := range b {
		if c >= 'A' && c <= 'Z' {
			b[i] = c + 'a' - 'A'
		}
	}
	return string(b)
}
