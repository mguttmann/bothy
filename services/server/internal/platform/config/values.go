// SPDX-License-Identifier: Apache-2.0

package config

import (
	"fmt"
	"maps"
	"net/netip"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

// SourceKind says which source set the value of a key.
type SourceKind int

// The sources in rising precedence (02 "Configuration loading" item 2). The zero value is the
// default, so a zero Source reads "default".
const (
	SourceDefault SourceKind = iota // built-in default of the schema
	SourceFile                      // configuration file (--config or BOTHY_CONFIG_FILE)
	SourceEnv                       // environment variable BOTHY_<KEY>
	SourceFlag                      // command-line flag (only --role)
)

// Source is the origin of a value: its kind and, for a file, a variable or a flag, its name.
type Source struct {
	// Kind is the kind of source.
	Kind SourceKind
	// Name is the file path, the variable name or the flag ("--role"); empty for the default.
	Name string
}

// String renders the source as bothy config print and problems show it: "default",
// "file <path>", "env BOTHY_X" or "flag --role". A name that would not stay one unambiguous line
// (a newline in a file path) is Go-quoted.
func (s Source) String() string {
	switch s.Kind {
	case SourceDefault:
		return "default"
	case SourceFile:
		return "file " + oneLine(s.Name)
	case SourceEnv:
		return "env " + oneLine(s.Name)
	case SourceFlag:
		return "flag " + oneLine(s.Name)
	}
	return fmt.Sprintf("SourceKind(%d)", int(s.Kind))
}

// oneLine returns s as it is, or Go-quoted when it would not read back as one unambiguous line:
// with invalid UTF-8 or a character that is not printable (a newline would forge a problem line
// of its own), with white space at either end, or starting with a quote. Operator text such as file paths and
// variable names goes through it before it reaches a problem.
func oneLine(s string) string {
	if !utf8.ValidString(s) || strings.TrimSpace(s) != s || strings.HasPrefix(s, `"`) ||
		strings.IndexFunc(s, func(r rune) bool { return !unicode.IsPrint(r) }) >= 0 {
		return strconv.Quote(s)
	}
	return s
}

// value is the loaded value of one key: the parsed value, its canonical text and its source.
type value struct {
	v    any
	text string
	src  Source
}

// Values is the loaded configuration: one parsed value per key of the schema with its source.
// Load builds it; it never changes afterwards, so it is safe for concurrent use.
//
// The accessors panic when the key is not in the schema or has a kind the accessor does not
// serve. Both are programming errors that tests catch, not operator errors; the panic message
// names the key and its kind, never a value. Slices and maps are returned as copies.
type Values struct {
	schema *Schema
	vals   map[string]value
	file   string
}

// stringKinds are the kinds whose value is a string (Values.String, Bind into string).
var stringKinds = []Kind{KindString, KindEnum, KindListenAddr, KindCron, KindSemVer, KindSecretRef, KindURL}

func (v *Values) lookup(method, key string, kinds ...Kind) value {
	val, ok := v.vals[key]
	if !ok {
		panic(fmt.Sprintf("config: %s: unknown key %q", method, key))
	}
	if k := v.schema.entries[key].Kind; len(kinds) > 0 && !slices.Contains(kinds, k) {
		panic(fmt.Sprintf("config: %s: key %q has kind %s", method, key, k))
	}
	return val
}

// String returns the value of a string, enum, listen address, cron, semver, secret_ref or url
// key. For a secret_ref key it is the reference, never the secret.
func (v *Values) String(key string) string {
	return v.lookup("String", key, stringKinds...).v.(string)
}

// Int returns the value of an int key.
func (v *Values) Int(key string) int64 { return v.lookup("Int", key, KindInt).v.(int64) }

// Float returns the value of a float key.
func (v *Values) Float(key string) float64 { return v.lookup("Float", key, KindFloat).v.(float64) }

// Bool returns the value of a bool key.
func (v *Values) Bool(key string) bool { return v.lookup("Bool", key, KindBool).v.(bool) }

// Duration returns the value of a duration key.
func (v *Values) Duration(key string) time.Duration {
	return v.lookup("Duration", key, KindDuration).v.(time.Duration)
}

// Strings returns a copy of the value of a list(string) key; the empty list is an empty, non-nil
// slice.
func (v *Values) Strings(key string) []string {
	return slices.Clone(v.lookup("Strings", key, KindStringList).v.([]string))
}

// Prefixes returns a copy of the value of a list(cidr) key.
func (v *Values) Prefixes(key string) []netip.Prefix {
	return slices.Clone(v.lookup("Prefixes", key, KindCIDRList).v.([]netip.Prefix))
}

// StringMap returns a copy of the value of a map(string) key; the empty map is an empty, non-nil
// map.
func (v *Values) StringMap(key string) map[string]string {
	return maps.Clone(v.lookup("StringMap", key, KindStringMap).v.(map[string]string))
}

// Text returns the canonical text of the value of any key: the text bothy config print shows,
// which parses back to the same value.
func (v *Values) Text(key string) string { return v.lookup("Text", key).text }

// Source returns the source that set the value of any key.
func (v *Values) Source(key string) Source { return v.lookup("Source", key).src }

// File returns the path of the configuration file that was read, or "" when there was none.
func (v *Values) File() string { return v.file }
