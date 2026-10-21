// SPDX-License-Identifier: Apache-2.0

package title

import (
	"bytes"
	"embed"
	"encoding/json"
	"fmt"
	"path"
	"slices"
	"strings"

	"github.com/OWNER/bothy/services/server/internal/kernel/textcheck"
)

// files holds the word lists, one file per language (brief M2-18 note 2): reading it touches only
// memory, so the domain stays free of input and output.
//
//go:embed wordlists/*.txt
var files embed.FS

// lists are the parsed word lists by language, and languages their stems in ascending order;
// both are set once at initialisation.
var (
	lists     = map[string][]string{}
	languages []string
)

func init() {
	entries, err := files.ReadDir("wordlists")
	if err != nil {
		panic(err)
	}
	for _, e := range entries {
		lang, ok := strings.CutSuffix(e.Name(), ".txt")
		if !ok {
			continue
		}
		raw, err := files.ReadFile(path.Join("wordlists", e.Name()))
		if err != nil {
			panic(err)
		}
		words, err := parseWordList(raw)
		if err != nil {
			panic(fmt.Sprintf("title: word list %s: %v", e.Name(), err))
		}
		lists[lang] = words
		languages = append(languages, lang)
	}
	slices.Sort(languages)
}

// spdxLine is the required first line of a word list.
const spdxLine = "# SPDX-License-Identifier: Apache-2.0"

// parseWordList parses one file: the SPDX line first, "#" lines are comments, blank lines are
// ignored; every entry is normalised, a display text and once in the file under EqualFold.
// Malformed embedded data panics in init like regexp.MustCompile; the unit tests keep the files
// valid.
func parseWordList(raw []byte) ([]string, error) {
	lines := strings.Split(string(bytes.ReplaceAll(raw, []byte("\r\n"), []byte("\n"))), "\n")
	if len(lines) == 0 || lines[0] != spdxLine {
		return nil, fmt.Errorf("the first line is not %q", spdxLine)
	}
	var out []string
	for i, l := range lines[1:] {
		if strings.HasPrefix(l, "#") || strings.TrimSpace(l) == "" {
			continue
		}
		switch {
		case Normalize(l) != l:
			return nil, fmt.Errorf("line %d is not normalised", i+2)
		case textcheck.DisplayText(l) != nil:
			return nil, fmt.Errorf("line %d is no display text", i+2)
		case slices.ContainsFunc(out, func(o string) bool { return strings.EqualFold(o, l) }):
			return nil, fmt.Errorf("line %d repeats an entry", i+2)
		}
		out = append(out, l)
	}
	return out, nil
}

// Languages returns the languages that have a word list, in ascending order ("de", "en").
func Languages() []string { return slices.Clone(languages) }

// WordList returns the entries of the word list of lang in file order; ok is false for a
// language without one.
func WordList(lang string) (entries []string, ok bool) {
	l, ok := lists[lang]
	return slices.Clone(l), ok
}

// DefaultEntries returns the default of thread.placeholder_titles: the entries of every word list
// in language order and file order, without an entry already present under strings.EqualFold.
//
// Example:
//
//	title.DefaultEntries() // [Neuer Chat Chat Ohne Titel Test New chat Untitled]
func DefaultEntries() []string {
	var out []string
	for _, lang := range languages {
		for _, e := range lists[lang] {
			if !slices.ContainsFunc(out, func(o string) bool { return strings.EqualFold(o, e) }) {
				out = append(out, e)
			}
		}
	}
	return out
}

// DefaultConfigText renders DefaultEntries as the default text of thread.placeholder_titles: a
// JSON array with ", " between the elements and no HTML escaping, the form of the cell in
// docs/spec/09-agenten.md section 21.
func DefaultConfigText() string {
	entries := DefaultEntries()
	parts := make([]string, 0, len(entries))
	for _, e := range entries {
		var b bytes.Buffer
		enc := json.NewEncoder(&b)
		enc.SetEscapeHTML(false)
		if err := enc.Encode(e); err != nil {
			panic(err)
		}
		parts = append(parts, strings.TrimSuffix(b.String(), "\n"))
	}
	return "[" + strings.Join(parts, ", ") + "]"
}
