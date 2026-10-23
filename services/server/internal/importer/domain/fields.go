// SPDX-License-Identifier: Apache-2.0

package domain

import (
	"bytes"
	"encoding/json"
	"slices"
	"strconv"
	"strings"

	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
)

// Codes of the field errors of the importer (errors[].code of 400 validation_failed).
const (
	// FieldRequired: a required member is missing or empty.
	FieldRequired = "required"
	// FieldUnknownMember: a member the schema does not name (additionalProperties false).
	FieldUnknownMember = "unknown_member"
	// FieldInvalid: a member of the wrong type or value.
	FieldInvalid = "invalid"
	// FieldNotDigits: a Discord ID that is not a string of digits.
	FieldNotDigits = "not_digits"
	// FieldDuplicate: a Discord ID named twice in one list.
	FieldDuplicate = "duplicate"
	// FieldTooLong: a name longer than its configured limit.
	FieldTooLong = "too_long"
	// FieldUnknownHandle: a handle no person or agent of that kind has.
	FieldUnknownHandle = "unknown_handle"
	// FieldNotActive: a person or agent that is not active.
	FieldNotActive = "not_active"
	// FieldUnknownSpace: an existing UUID that names no live space.
	FieldUnknownSpace = "unknown_space"
	// FieldUnknownChannel: an existing UUID that names no live channel.
	FieldUnknownChannel = "unknown_channel"
	// FieldUnknownCategory: uncategorized_space names no category of spaces.
	FieldUnknownCategory = "unknown_category"
	// FieldNotEnabled: an item kind, member or source kind that a later ticket enables.
	FieldNotEnabled = "not_enabled"
	// FieldKindMismatch: an origin key whose prefix does not match item_kind.
	FieldKindMismatch = "kind_mismatch"
)

// Problems collects field errors in order, so every violation is reported at once.
type Problems struct {
	list []errs.FieldError
}

// Add records one violation at pointer.
func (p *Problems) Add(pointer, code, detail string) {
	p.list = append(p.list, errs.FieldError{Pointer: pointer, Code: code, Detail: detail})
}

// Empty reports that nothing was recorded.
func (p *Problems) Empty() bool { return len(p.list) == 0 }

// Fields returns the recorded violations.
func (p *Problems) Fields() []errs.FieldError { return slices.Clone(p.list) }

// Err returns 400 validation_failed with every recorded violation, nil when there is none.
func (p *Problems) Err() error {
	if p.Empty() {
		return nil
	}
	return errs.New(errs.CodeValidationFailed, "the request has invalid members").WithFields(p.Fields()...)
}

// Pointer joins a JSON pointer and one more token, escaped per RFC 6901.
func Pointer(base string, token any) string {
	var s string
	switch v := token.(type) {
	case int:
		s = strconv.Itoa(v)
	case string:
		s = strings.ReplaceAll(strings.ReplaceAll(v, "~", "~0"), "/", "~1")
	}
	return base + "/" + s
}

// object decodes raw as a JSON object and records every member that allowed does not name. A
// value that is no object is one invalid at pointer; then the result is nil.
func object(raw json.RawMessage, pointer string, allowed []string, p *Problems) map[string]json.RawMessage {
	var m map[string]json.RawMessage
	if t := bytes.TrimSpace(raw); len(t) == 0 || t[0] != '{' {
		p.Add(pointer, FieldInvalid, "an object is required")
		return nil
	}
	if err := json.Unmarshal(raw, &m); err != nil || m == nil {
		p.Add(pointer, FieldInvalid, "an object is required")
		return nil
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	for _, k := range keys {
		if !slices.Contains(allowed, k) {
			p.Add(Pointer(pointer, k), FieldUnknownMember, "the member is not part of the schema")
		}
	}
	return m
}

// isNull reports a JSON null, which counts as a missing member.
func isNull(raw json.RawMessage) bool { return string(raw) == "null" }

// str reads the string member key of m. A missing member is required at pointer when required;
// a member that is no string is invalid. ok reports a present string.
func str(m map[string]json.RawMessage, key, pointer string, required bool, p *Problems) (string, bool) {
	raw, present := m[key]
	if !present || isNull(raw) {
		if required {
			p.Add(Pointer(pointer, key), FieldRequired, "the member is required")
		}
		return "", false
	}
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		p.Add(Pointer(pointer, key), FieldInvalid, "a string is required")
		return "", false
	}
	return s, true
}

// Digits reports a non-empty string of ASCII digits (a Discord snowflake, 22 7.1).
func Digits(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

// discordID reads the string member key as a Discord ID: required, digits only.
func discordID(m map[string]json.RawMessage, key, pointer string, p *Problems) (string, bool) {
	s, ok := str(m, key, pointer, true, p)
	if !ok {
		return "", false
	}
	if !Digits(s) {
		p.Add(Pointer(pointer, key), FieldNotDigits, "a Discord ID is a string of digits")
		return "", false
	}
	return s, true
}
