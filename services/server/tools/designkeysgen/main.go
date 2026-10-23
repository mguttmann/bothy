// SPDX-License-Identifier: Apache-2.0

// Command designkeysgen writes the keys of the design system that the server checks (M1-46,
// ADR 0030, ADR 0044): the category colours and the space icons, read from
// design/build/tokens.resolved.json, into internal/kernel/designkeys/designkeys_gen.go.
//
// A category colour is the second path segment of every token key color.category.<key>.<role>
// of contexts.theme.light and contexts.theme.dark; both themes must give the same set, otherwise
// the run fails (a colour that exists in one theme only would pass the server but break a
// client). The space icons are the list icons.space-icons. The semantic icon names are the keys of
// icons.glyphs, the names clients draw through (docs/design/icons.md "Rules"); runs checks the icon
// of a button against them (M2-15). The lists are written sorted and without duplicates.
//
// It runs through go generate (directive in internal/kernel/designkeys/doc.go), which
// make codegen-check runs and diffs:
//
//	go generate ./internal/kernel/designkeys
//
// Exit codes: 0 written, 1 invalid tokens or I/O error, 2 usage error.
package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"go/format"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"text/template"
)

func main() { os.Exit(run(os.Args[1:], os.Stderr)) }

// run parses args (-tokens, the resolved token file; -out, the Go file to write), generates and
// returns the exit code; problems go to stderr.
func run(args []string, stderr io.Writer) int {
	fs := flag.NewFlagSet("designkeysgen", flag.ContinueOnError)
	fs.SetOutput(stderr)
	tokens := fs.String("tokens", "", "design/build/tokens.resolved.json to read")
	out := fs.String("out", "", "Go file to write")
	if err := fs.Parse(args); err != nil || *tokens == "" || *out == "" || fs.NArg() > 0 {
		_, _ = fmt.Fprintln(stderr, "usage: designkeysgen -tokens tokens.resolved.json -out designkeys_gen.go")
		return 2
	}
	k, err := ReadKeys(*tokens)
	var src []byte
	if err == nil {
		src, err = Render(k)
	}
	if err == nil {
		err = os.WriteFile(*out, src, 0o600)
	}
	if err != nil {
		_, _ = fmt.Fprintln(stderr, "designkeysgen:", err)
		return 1
	}
	return 0
}

// Keys are the generated lists, sorted and without duplicates.
type Keys struct {
	// CategoryColors are the keys of the category colours.
	CategoryColors []string
	// SpaceIcons are the names of the space icons.
	SpaceIcons []string
	// Glyphs are the semantic icon names, the keys of icons.glyphs (M2-15).
	Glyphs []string
}

// keyPattern is the form of a colour key and an icon name: lower case words joined by "-". A key
// outside it could not be a stable value of the API.
var keyPattern = regexp.MustCompile(`^[a-z][a-z0-9]*(-[a-z0-9]+)*$`)

// glyphPattern is the form of a semantic icon name: keys of keyPattern, with "_" inside a word,
// joined by "." (trigger.received, step.tool_call, action.quick-switch).
var glyphPattern = regexp.MustCompile(`^[a-z][a-z0-9_]*(-[a-z0-9_]+)*(\.[a-z][a-z0-9_]*(-[a-z0-9_]+)*)*$`)

// categoryPrefix starts every token key of a category colour.
const categoryPrefix = "color.category."

// tokenFile is the part of tokens.resolved.json the generator reads.
type tokenFile struct {
	Contexts struct {
		Theme map[string]map[string]json.RawMessage `json:"theme"`
	} `json:"contexts"`
	Icons struct {
		SpaceIcons *[]string                  `json:"space-icons"`
		Glyphs     map[string]json.RawMessage `json:"glyphs"`
	} `json:"icons"`
}

// ReadKeys reads the token file at path and returns its keys. Errors: an unreadable or invalid
// file, a missing theme, icon list or glyph map, no category colour, a key or icon outside
// keyPattern, a glyph name outside glyphPattern, and light and dark themes with different category colours (the error names the keys of each side).
func ReadKeys(path string) (Keys, error) {
	data, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return Keys{}, err
	}
	var f tokenFile
	if err := json.Unmarshal(data, &f); err != nil {
		return Keys{}, fmt.Errorf("%s: %w", path, err)
	}
	light, err := categories(f, "light")
	if err != nil {
		return Keys{}, fmt.Errorf("%s: %w", path, err)
	}
	dark, err := categories(f, "dark")
	if err != nil {
		return Keys{}, fmt.Errorf("%s: %w", path, err)
	}
	if !slices.Equal(light, dark) {
		return Keys{}, fmt.Errorf("%s: the themes have different category colours: light %s, dark %s",
			path, strings.Join(light, ", "), strings.Join(dark, ", "))
	}
	if f.Icons.SpaceIcons == nil || len(*f.Icons.SpaceIcons) == 0 {
		return Keys{}, fmt.Errorf("%s: icons.space-icons is missing or empty", path)
	}
	icons := slices.Clone(*f.Icons.SpaceIcons)
	for _, icon := range icons {
		if !keyPattern.MatchString(icon) {
			return Keys{}, fmt.Errorf("%s: space icon %q is not a key", path, icon)
		}
	}
	slices.Sort(icons)
	if len(f.Icons.Glyphs) == 0 {
		return Keys{}, fmt.Errorf("%s: icons.glyphs is missing or empty", path)
	}
	glyphs := make([]string, 0, len(f.Icons.Glyphs))
	for name := range f.Icons.Glyphs {
		if !glyphPattern.MatchString(name) {
			return Keys{}, fmt.Errorf("%s: glyph %q is not a semantic icon name", path, name)
		}
		glyphs = append(glyphs, name)
	}
	slices.Sort(glyphs)
	return Keys{CategoryColors: light, SpaceIcons: slices.Compact(icons), Glyphs: glyphs}, nil
}

// categories returns the sorted category colour keys of the theme name.
func categories(f tokenFile, name string) ([]string, error) {
	theme, ok := f.Contexts.Theme[name]
	if !ok {
		return nil, fmt.Errorf("contexts.theme.%s is missing", name)
	}
	var keys []string
	for token := range theme {
		rest, ok := strings.CutPrefix(token, categoryPrefix)
		if !ok {
			continue
		}
		key, _, ok := strings.Cut(rest, ".")
		if !ok || !keyPattern.MatchString(key) {
			return nil, fmt.Errorf("contexts.theme.%s: token %q is not color.category.<key>.<role>", name, token)
		}
		keys = append(keys, key)
	}
	if len(keys) == 0 {
		return nil, fmt.Errorf("contexts.theme.%s has no category colour", name)
	}
	slices.Sort(keys)
	return slices.Compact(keys), nil
}

var fileTemplate = template.Must(template.New("designkeys").Funcs(template.FuncMap{"quote": strconv.Quote}).Parse(
	`// Code generated by tools/designkeysgen from design/build/tokens.resolved.json; DO NOT EDIT.

package designkeys

// categoryColors are the keys of the category colours of the design system, sorted.
var categoryColors = []string{
{{- range .CategoryColors}}
	{{quote .}},
{{- end}}
}

// spaceIcons are the names of the space icons of the design system, sorted.
var spaceIcons = []string{
{{- range .SpaceIcons}}
	{{quote .}},
{{- end}}
}

// iconNames are the semantic icon names of the design system (the keys of icons.glyphs), sorted.
var iconNames = []string{
{{- range .Glyphs}}
	{{quote .}},
{{- end}}
}
`))

// Render returns the formatted Go source of designkeys_gen.go for k.
func Render(k Keys) ([]byte, error) {
	if len(k.CategoryColors) == 0 || len(k.SpaceIcons) == 0 || len(k.Glyphs) == 0 {
		return nil, errors.New("nothing to render")
	}
	var buf bytes.Buffer
	if err := fileTemplate.Execute(&buf, k); err != nil {
		return nil, err
	}
	return format.Source(buf.Bytes())
}
