// SPDX-License-Identifier: Apache-2.0

package config

import (
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"time"
)

// Key describes one configuration key: its name, kind, default and documentation. The values
// come from the chapter table "Konfigurationswerte dieses Kapitels" that owns the key; M1-08
// compares schema and chapter tables.
type Key struct {
	// Name is the key, for example "server.listen_addr": at least two segments joined by ".",
	// each matching ^[a-z][a-z0-9]*(_[a-z0-9]+)*$.
	Name string
	// Kind selects the parser and the Go type of the value.
	Kind Kind
	// Default is exactly the chapter table cell without backticks; "" for "leer". In a
	// secret_ref default, <other.key> stands for the effective value of that string key. A
	// configured secret_ref value may use only <secrets.dir> (see Load).
	Default string
	// Required marks the chapter cell "keiner, Pflicht": a source must set a non-empty value,
	// and Default must be "".
	Required bool
	// Enum lists the allowed values of a KindEnum key and is empty for every other kind.
	Enum []string
	// Min and Max are optional inclusive bounds in the text form of the kind; only int, float
	// and duration keys have bounds.
	Min, Max string
	// Doc is one English sentence that says what the key does.
	Doc string
	// Chapter is the file name of the chapter whose table holds the key, for example
	// "16-betrieb.md".
	Chapter string
}

// Schema is the set of keys the server reads and the cross-checks between them. Build it with
// NewSchema and AddCheck once per process (internal/wire does it); after that it is only read,
// and Load may use it from several goroutines. AddCheck is not safe for concurrent use.
type Schema struct {
	entries  map[string]*keyEntry
	names    []string
	sections map[string]bool
	checks   []check
}

// keyEntry is a key with its parsed bounds and the placeholders of its default.
type keyEntry struct {
	Key
	min, max         any
	minText, maxText string
	placeholders     []string
}

// check is a cross-check registered with AddCheck.
type check struct {
	name string
	keys []string
	fn   func(*Values) error
}

var (
	chapterPattern = regexp.MustCompile(`^[0-9]{2}-[a-z0-9-]+\.md$`)
	// placeholderPattern finds <other.key> in a secret_ref default (16 "Secret-Provider": the
	// default reference is file:<secrets.dir>/<name>).
	placeholderPattern = regexp.MustCompile(`<([^<>]*)>`)
)

// NewSchema validates keys and returns the schema. The error lists every invalid key, one line
// each ("config schema: key "<name>": <reason>"); the rules are:
//
//   - names are valid (see Key.Name), unique, and no name is a section of another one
//     ("a.b" and "a.b.c" would need "b" to be both a value and a mapping in the file);
//   - Kind is set; a name ends in "_ref" exactly when the kind is secret_ref;
//   - Enum is non-empty exactly for enum keys, without empty or repeated values;
//   - a required key has no default; an empty default is allowed only for string and list
//     keys; every other default parses with the parser of its kind, after expanding the
//     placeholders of a secret_ref default with the defaults of the keys they name, which must
//     be registered string keys;
//   - bounds only on int, float and duration keys; they parse, Min <= Max, and the default lies
//     within them;
//   - Doc is not blank, and Chapter matches ^[0-9]{2}-[a-z0-9-]+\.md$.
//
// The schema keeps its own copies of keys.
func NewSchema(keys []Key) (*Schema, error) {
	s := &Schema{entries: map[string]*keyEntry{}, sections: map[string]bool{}}
	var problems []string
	add := func(name, format string, args ...any) {
		problems = append(problems, fmt.Sprintf("config schema: key %q: ", name)+fmt.Sprintf(format, args...))
	}

	var valid []*keyEntry
	for _, k := range keys {
		k.Enum = slices.Clone(k.Enum)
		switch {
		case !validName(k.Name):
			add(k.Name, "invalid key name; want segments joined by \".\", at least two, each matching %s", segmentPattern)
			continue
		case s.entries[k.Name] != nil:
			add(k.Name, "registered twice")
			continue
		}
		e := &keyEntry{Key: k}
		s.entries[k.Name] = e
		valid = append(valid, e)
	}
	for _, e := range valid {
		for _, msg := range s.validateKey(e) {
			add(e.Name, "%s", msg)
		}
	}
	for name := range s.entries {
		s.names = append(s.names, name)
		for i := strings.Index(name, "."); i >= 0; i = nextDot(name, i) {
			s.sections[name[:i]] = true
		}
	}
	slices.Sort(s.names)
	for _, name := range s.names {
		if s.sections[name] {
			add(name, "is also a section of another key, so the file could not hold both")
		}
	}
	if len(problems) > 0 {
		return nil, errors.New(strings.Join(problems, "\n"))
	}
	return s, nil
}

func nextDot(name string, i int) int {
	j := strings.Index(name[i+1:], ".")
	if j < 0 {
		return -1
	}
	return i + 1 + j
}

// validateKey returns the problems of one key whose name is valid and unique.
func (s *Schema) validateKey(e *keyEntry) []string {
	var problems []string
	if !e.Kind.valid() {
		return []string{fmt.Sprintf("kind %s is no kind of this package", e.Kind)}
	}
	if strings.HasSuffix(e.Name, "_ref") != (e.Kind == KindSecretRef) {
		problems = append(problems, "a name ends in _ref exactly when the kind is secret_ref (02 \"Konfigurationsladen\" item 3)")
	}
	problems = append(problems, validateEnum(e.Key)...)
	if strings.TrimSpace(e.Doc) == "" {
		problems = append(problems, "Doc is empty")
	}
	if !chapterPattern.MatchString(e.Chapter) {
		problems = append(problems, fmt.Sprintf("Chapter %q does not match %s", e.Chapter, chapterPattern))
	}
	problems = append(problems, s.validateBounds(e)...)
	if msg := s.validateDefault(e); msg != "" {
		problems = append(problems, msg)
	}
	return problems
}

func validateEnum(k Key) []string {
	if (len(k.Enum) > 0) != (k.Kind == KindEnum) {
		return []string{"Enum must be set exactly for enum keys"}
	}
	for i, v := range k.Enum {
		if v == "" || slices.Contains(k.Enum[:i], v) {
			return []string{"Enum must not contain empty or repeated values"}
		}
	}
	return nil
}

func (s *Schema) validateBounds(e *keyEntry) []string {
	if e.Min == "" && e.Max == "" {
		return nil
	}
	if !e.Kind.hasBounds() {
		return []string{"bounds apply only to int, float and duration keys"}
	}
	var problems []string
	parse := func(field, text string) (any, string) {
		if text == "" {
			return nil, ""
		}
		v, canon, err := parseValue(e.Kind, nil, text)
		if err != nil {
			problems = append(problems, fmt.Sprintf("%s does not parse: %v", field, err))
			return nil, ""
		}
		return v, canon
	}
	e.min, e.minText = parse("Min", e.Min)
	e.max, e.maxText = parse("Max", e.Max)
	if e.min != nil && e.max != nil && less(e.max, e.min) {
		problems = append(problems, "Min is greater than Max")
	}
	return problems
}

// validateDefault returns the problem of the default, or "".
func (s *Schema) validateDefault(e *keyEntry) string {
	switch {
	case e.Required:
		if e.Default != "" {
			return "a required key has no default"
		}
		return ""
	case e.Default == "":
		if e.Kind != KindString && !e.Kind.isList() {
			return "empty default; only string and list keys and required keys may have one"
		}
		return ""
	}
	text := e.Default
	if e.Kind == KindSecretRef {
		for _, m := range placeholderPattern.FindAllStringSubmatch(e.Default, -1) {
			ref := s.entries[m[1]]
			if ref == nil || ref.Kind != KindString {
				return fmt.Sprintf("default names %s, which is not a registered string key", m[0])
			}
			e.placeholders = append(e.placeholders, m[1])
		}
		text = s.expandDefault(e, func(name string) string { return s.entries[name].Default })
	}
	v, _, err := parseValue(e.Kind, e.Enum, text)
	if err != nil {
		return fmt.Sprintf("default %v", err)
	}
	if err := e.checkBounds(v); err != nil {
		return fmt.Sprintf("default %v", err)
	}
	return ""
}

// expandDefault replaces every placeholder of the default of e by value(name). The inserted text
// is not scanned again, so a value that contains "<x>" stays as it is.
func (s *Schema) expandDefault(e *keyEntry, value func(name string) string) string {
	if len(e.placeholders) == 0 {
		return e.Default
	}
	return placeholderPattern.ReplaceAllStringFunc(e.Default, func(m string) string {
		return value(m[1 : len(m)-1])
	})
}

// checkBounds returns an error naming the violated bound, never the value.
func (e *keyEntry) checkBounds(v any) error {
	if e.min != nil && less(v, e.min) {
		return fmt.Errorf("must be at least %s", e.minText)
	}
	if e.max != nil && less(e.max, v) {
		return fmt.Errorf("must be at most %s", e.maxText)
	}
	return nil
}

// less compares two values of the same bounded kind.
func less(a, b any) bool {
	switch x := a.(type) {
	case int64:
		return x < b.(int64)
	case float64:
		return x < b.(float64)
	case time.Duration:
		return x < b.(time.Duration)
	}
	return false
}

// AddCheck registers a cross-check: fn receives the loaded values of exactly keys and returns an
// error when they do not fit together. Load runs the check after all keys parsed, and only when
// none of its keys has a problem or stands in a file or below a file section that Load rejected.
// The error text becomes the problem message verbatim, so fn must never put a value into it.
//
// AddCheck returns an error when name is not ^[a-z][a-z0-9]*(_[a-z0-9]+)*$ or already
// registered, when keys is empty, names an unknown key or one key twice, or when fn is nil.
func (s *Schema) AddCheck(name string, keys []string, fn func(*Values) error) error {
	switch {
	case !segmentPattern.MatchString(name):
		return fmt.Errorf("config schema: invalid check name %q", name)
	case slices.ContainsFunc(s.checks, func(c check) bool { return c.name == name }):
		return fmt.Errorf("config schema: check %q registered twice", name)
	case len(keys) == 0:
		return fmt.Errorf("config schema: check %q has no keys", name)
	case fn == nil:
		return fmt.Errorf("config schema: check %q has no function", name)
	}
	for i, k := range keys {
		if s.entries[k] == nil {
			return fmt.Errorf("config schema: check %q: unknown key %q", name, k)
		}
		if slices.Contains(keys[:i], k) {
			return fmt.Errorf("config schema: check %q names key %q twice", name, k)
		}
	}
	s.checks = append(s.checks, check{name: name, keys: slices.Clone(keys), fn: fn})
	return nil
}

// Keys returns copies of all keys sorted by name.
func (s *Schema) Keys() []Key {
	keys := make([]Key, 0, len(s.names))
	for _, name := range s.names {
		k := s.entries[name].Key
		k.Enum = slices.Clone(k.Enum)
		keys = append(keys, k)
	}
	return keys
}
