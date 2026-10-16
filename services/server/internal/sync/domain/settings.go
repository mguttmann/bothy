// SPDX-License-Identifier: Apache-2.0

package domain

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"regexp"
	"slices"
	"strconv"

	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
)

// SettingKind is the shape of the value of a personal setting.
type SettingKind uint8

// The kinds of the registry.
const (
	// SettingEnum accepts exactly one of the strings in Values.
	SettingEnum SettingKind = iota + 1
	// SettingBool accepts JSON true or false.
	SettingBool
	// SettingObject is layout.column_widths: an object of column names matching
	// ^[a-z][a-z0-9_]*$ with non-negative integers (logical pixels).
	SettingObject
)

// SettingSpec is one key of the registry: its key, its kind and, for SettingEnum, the allowed
// strings in the order of the specification.
type SettingSpec struct {
	Key    string
	Kind   SettingKind
	Values []string
}

// registry holds exactly the keys of docs/spec 01 section 11 ("Personal settings") and 13
// section 3.3 ("Personal switches"); TestSettingRegistryMatchesSpec compares it with both
// tables. Defaults are not stored: clients apply them (04 section 14, row Settings).
var registry = []SettingSpec{
	{Key: "appearance.theme", Kind: SettingEnum, Values: []string{"system", "light", "dark"}},
	{Key: "appearance.density", Kind: SettingEnum, Values: []string{"comfortable", "compact"}},
	{Key: "appearance.reduce_motion", Kind: SettingEnum, Values: []string{"system", "on", "off"}},
	{Key: "composer.enter_sends", Kind: SettingBool},
	{Key: "privacy.crash_reports", Kind: SettingBool},
	{Key: "privacy.client_metrics", Kind: SettingBool},
	{Key: "layout.column_widths", Kind: SettingObject},
	{Key: "notify.default_level", Kind: SettingEnum, Values: []string{"all", "mentions", "mute"}},
	{Key: "notify.preview_mode", Kind: SettingEnum, Values: []string{"full", "names", "none"}},
	{Key: "notify.input_needed_breaks_quiet_hours", Kind: SettingBool},
	{Key: "notify.admin_enabled", Kind: SettingBool},
}

// Settings returns a copy of the registry in the order of the specification.
func Settings() []SettingSpec {
	out := make([]SettingSpec, len(registry))
	for i, s := range registry {
		s.Values = slices.Clone(s.Values)
		out[i] = s
	}
	return out
}

// lookup returns the spec of key.
func lookup(key string) (SettingSpec, bool) {
	i := slices.IndexFunc(registry, func(s SettingSpec) bool { return s.Key == key })
	if i < 0 {
		return SettingSpec{}, false
	}
	return registry[i], true
}

// KnownSetting reports whether key is a key of the registry.
func KnownSetting(key string) bool {
	_, ok := lookup(key)
	return ok
}

// columnName is the pattern of a member name of layout.column_widths (01 section 11).
var columnName = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

// errBadValue marks a value outside the rules of its key.
var errBadValue = errors.New("value outside the rules of the key")

// ValidateSetting checks value for key (brief M1-61 note 14). Errors: 404 not_found for an
// unknown key (the path names no resource), 400 validation_failed on /value with invalid for a
// value outside the rules of the key; JSON null is never valid.
//
// Example:
//
//	err := domain.ValidateSetting("appearance.theme", json.RawMessage(`"dark"`)) // nil
func ValidateSetting(key string, value json.RawMessage) error {
	spec, ok := lookup(key)
	if !ok {
		return errs.New(errs.CodeNotFound, "")
	}
	if err := checkValue(spec, value); err != nil {
		return fieldError("/value", FieldInvalid, "the value is not allowed for this setting")
	}
	return nil
}

// checkValue applies the kind of spec to value.
func checkValue(spec SettingSpec, value json.RawMessage) error {
	dec := json.NewDecoder(bytes.NewReader(value))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return errBadValue
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return errBadValue
	}
	switch spec.Kind {
	case SettingEnum:
		s, ok := v.(string)
		if !ok || !slices.Contains(spec.Values, s) {
			return errBadValue
		}
		return nil
	case SettingBool:
		if _, ok := v.(bool); !ok {
			return errBadValue
		}
		return nil
	case SettingObject:
		return checkColumns(value, v)
	}
	return errBadValue
}

// checkColumns checks layout.column_widths: an object without repeated names, names of the
// column pattern, values non-negative integers that fit int64 (no fraction, no exponent).
func checkColumns(raw json.RawMessage, v any) error {
	m, ok := v.(map[string]any)
	if !ok {
		return errBadValue
	}
	if unique, err := uniqueMembers(raw); err != nil || !unique {
		return errBadValue
	}
	for name, w := range m {
		n, ok := w.(json.Number)
		if !ok || !columnName.MatchString(name) {
			return errBadValue
		}
		i, err := strconv.ParseInt(n.String(), 10, 64)
		if err != nil || i < 0 {
			return errBadValue
		}
	}
	return nil
}

// uniqueMembers reports whether the top-level object raw names every member once; decoding into
// a map would keep only the last of a repeated name.
func uniqueMembers(raw json.RawMessage) (bool, error) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	if _, err := dec.Token(); err != nil {
		return false, err
	}
	seen := map[string]bool{}
	for dec.More() {
		t, err := dec.Token()
		if err != nil {
			return false, err
		}
		name, _ := t.(string)
		if seen[name] {
			return false, nil
		}
		seen[name] = true
		var skip json.RawMessage
		if err := dec.Decode(&skip); err != nil {
			return false, err
		}
	}
	return true, nil
}
