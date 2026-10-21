// SPDX-License-Identifier: Apache-2.0

package domain

import (
	"bytes"
	"encoding/json"
	"io"
	"math"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/textcheck"
)

// CheckValues checks the values of createInteraction (Design note 9) against the normalized
// definition of a component of type typ; values nil, empty or "null" means {}. The shapes:
//
//   - button: {} only.
//   - select: {"selected": [<option value>, ...]} with known, unique entries and a count between
//     min_values and max_values; with min_values zero an absent selected is an empty list.
//   - every type: no escape of NUL or of an unpaired surrogate anywhere (/values).
//   - form: an object keyed by field name without unknown keys; required fields present (text
//     and textarea with a character that is not white space); text and textarea strings, text
//     without a line break, at most max_length code points when set; number a finite JSON
//     number; boolean a JSON boolean; choice one of the choices.
//
// Values are content, not display texts, so textcheck does not apply to them.
//
// Error: one *errs.Error validation_failed with one FieldError per finding, code
// FieldValuesInvalid, pointer /values/<name> (form), /values/selected/<index> or
// /values/selected (select), /values (button, unknown members and a values document that is not
// an object); the Detail names the rule, never a value (SEC-T10). A definition that does not
// decode, or a type outside the three, is an internal error: CheckDefinition or CheckCard
// accepted every stored definition.
//
// Example:
//
//	if err := domain.CheckValues(comp.Type, comp.Definition, cmd.Values); err != nil {
//		return err // 400 interaction_values_invalid
//	}
func CheckValues(typ string, definition, values json.RawMessage) error {
	parsed := parseDefinition(&findings{}, typ, definition, "/definition", 0)
	if parsed == nil {
		return errs.New(errs.CodeInternalError, "runs: a stored component definition does not decode")
	}
	v := &valueFindings{}
	doc, ok := valuesObject(values)
	if !ok {
		v.add("/values", "the values are not one JSON object")
		return v.err()
	}
	if !textcheck.StorableJSON(values) {
		// PostgreSQL refuses NUL and an unpaired surrogate in jsonb (D-M2-15-27).
		v.add("/values", "the values hold NUL or an unpaired surrogate")
		return v.err()
	}
	switch d := normalizeDefinition(parsed).(type) {
	case *buttonDefinition:
		if len(doc) > 0 {
			v.add("/values", "a button takes no values")
		}
	case *selectDefinition:
		checkSelection(v, d, doc)
	case *formDefinition:
		checkFormValues(v, d, doc)
	}
	if len(v.list) > 0 {
		return v.err()
	}
	return nil
}

// valueFindings collects the findings of CheckValues, all with code FieldValuesInvalid.
type valueFindings struct {
	list []errs.FieldError
}

// add records one finding at pointer; detail names the rule and never the value.
func (v *valueFindings) add(pointer, detail string) {
	v.list = append(v.list, errs.FieldError{Pointer: pointer, Code: FieldValuesInvalid, Detail: detail})
}

// err returns the validation_failed error with every finding.
func (v *valueFindings) err() error {
	return errs.New(errs.CodeValidationFailed, "the values do not fit the component definition").WithFields(v.list...)
}

// valuesObject decodes values as one JSON object; nil, white space and null are the empty
// object. ok is false for any other document, for repeated keys and for invalid UTF-8.
func valuesObject(values json.RawMessage) (object, bool) {
	if isAbsent(values) {
		return object{}, true
	}
	return objectMembers(values)
}

// The pointer of a selection.
const pointerSelected = "/values/selected"

// checkSelection checks the values of a select against its normalized definition d.
func checkSelection(v *valueFindings, d *selectDefinition, doc object) {
	for key := range doc {
		if key != "selected" {
			v.add("/values", "the values of a select have a member other than selected")
			break
		}
	}
	var selected []string
	raw, given := doc["selected"]
	switch {
	case !given && *d.MinValues > 0:
		v.add(pointerSelected, "selected is missing")
		return
	case given:
		var ok bool
		if selected, ok = stringList(raw); !ok {
			v.add(pointerSelected, "selected is not a list of strings")
			return
		}
	}
	options := make(map[string]bool, len(d.Options))
	for _, o := range d.Options {
		options[o.Value] = true
	}
	seen := make(map[string]bool, len(selected))
	for i, s := range selected {
		switch {
		case !options[s]:
			v.add(pointerSelected+"/"+strconv.Itoa(i), "the entry is not a value of the options")
		case seen[s]:
			v.add(pointerSelected+"/"+strconv.Itoa(i), "the entry repeats an earlier entry")
		}
		seen[s] = true
	}
	if len(selected) < *d.MinValues || len(selected) > *d.MaxValues {
		v.add(pointerSelected, "the number of entries lies outside min_values and max_values")
	}
}

// stringList decodes raw as a JSON array of strings; null entries and other types are refused.
func stringList(raw json.RawMessage) ([]string, bool) {
	raw = bytes.TrimSpace(raw)
	var items []json.RawMessage
	if len(raw) == 0 || raw[0] != '[' || json.Unmarshal(raw, &items) != nil {
		return nil, false
	}
	out := make([]string, 0, len(items))
	for _, item := range items {
		s, ok := decodeString(item)
		if !ok {
			return nil, false
		}
		out = append(out, s)
	}
	return out, true
}

// checkFormValues checks the values of a form against its normalized definition d, one finding
// per field at most, in the order of the fields.
func checkFormValues(v *valueFindings, d *formDefinition, doc object) {
	names := make(map[string]bool, len(d.Fields))
	for _, f := range d.Fields {
		names[f.Name] = true
	}
	for key := range doc {
		if !names[key] {
			// The pointer names the values, never the key: it is human text (SEC-T10).
			v.add("/values", "the values have a key that is not a field name")
			break
		}
	}
	for _, f := range d.Fields {
		raw, given := doc[f.Name]
		switch {
		case !given && *f.Required:
			v.add("/values/"+f.Name, "a required field is missing")
		case given:
			if rule := fieldValueRule(f, raw); rule != "" {
				v.add("/values/"+f.Name, rule)
			}
		}
	}
}

// fieldValueRule returns the rule that the value raw of field f breaks, or "".
func fieldValueRule(f formField, raw json.RawMessage) string {
	switch f.Type {
	case fieldText, fieldTextarea:
		return textValueRule(f, raw)
	case fieldNumber:
		t := bytes.TrimSpace(raw)
		if len(t) == 0 || (t[0] != '-' && (t[0] < '0' || t[0] > '9')) {
			return "a JSON number is expected"
		}
		// strconv.ParseFloat behind json.Number reports a number outside the range of a 64-bit
		// float, for example 1e400, as an error.
		n, err := json.Number(t).Float64()
		if err != nil || math.IsInf(n, 0) || math.IsNaN(n) {
			return "the number is not a finite 64-bit float"
		}
	case fieldBoolean:
		if _, ok := decodeBool(raw); !ok {
			return "a JSON boolean is expected"
		}
	case fieldChoice:
		s, ok := decodeString(raw)
		if !ok || !oneOf(s, f.Choices) {
			return "the value is not one of the choices"
		}
	}
	return ""
}

// textValueRule returns the rule that the value raw of a text or textarea field f breaks, or "".
func textValueRule(f formField, raw json.RawMessage) string {
	s, ok := decodeString(raw)
	switch {
	case !ok:
		return "a JSON string is expected"
	case *f.Required && strings.TrimSpace(s) == "":
		return "a required text has no character other than white space"
	case f.Type == fieldText && strings.ContainsAny(s, "\n\r"):
		return "a text field takes no line break"
	case f.MaxLength != nil && utf8.RuneCountInString(s) > *f.MaxLength:
		return "the text is longer than max_length"
	}
	return ""
}

// SameValues reports whether two values documents are equal as JSON, key order ignored ({} for
// nil, empty or "null"): the replay comparison of createInteraction (Design note 10 d). Numbers
// compare by value, because the stored jsonb gives back 1e2 as 100. A document that does not
// decode is never the same.
func SameValues(a, b json.RawMessage) bool {
	va, okA := decodeJSON(emptyAsObject(a))
	vb, okB := decodeJSON(emptyAsObject(b))
	return okA && okB && equalJSON(va, vb)
}

// emptyAsObject returns {} for nil, white space and null, else raw.
func emptyAsObject(raw json.RawMessage) json.RawMessage {
	if isAbsent(raw) {
		return json.RawMessage(`{}`)
	}
	return raw
}

// decodeJSON decodes raw as exactly one JSON value with numbers kept as json.Number.
func decodeJSON(raw json.RawMessage) (any, bool) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return nil, false
	}
	if _, err := dec.Token(); err != io.EOF {
		return nil, false
	}
	return v, true
}

// equalJSON compares two decoded JSON values; numbers are equal when their text is equal or
// both parse to the same 64-bit float.
func equalJSON(a, b any) bool {
	switch x := a.(type) {
	case map[string]any:
		y, ok := b.(map[string]any)
		return ok && equalObjects(x, y)
	case []any:
		y, ok := b.([]any)
		return ok && equalArrays(x, y)
	case json.Number:
		y, ok := b.(json.Number)
		return ok && equalNumber(x, y)
	}
	return a == b
}

// equalObjects compares two decoded JSON objects member by member.
func equalObjects(x, y map[string]any) bool {
	if len(x) != len(y) {
		return false
	}
	for k, xv := range x {
		yv, found := y[k]
		if !found || !equalJSON(xv, yv) {
			return false
		}
	}
	return true
}

// equalArrays compares two decoded JSON arrays entry by entry.
func equalArrays(x, y []any) bool {
	if len(x) != len(y) {
		return false
	}
	for i := range x {
		if !equalJSON(x[i], y[i]) {
			return false
		}
	}
	return true
}

// equalNumber reports whether two JSON numbers have the same text or the same finite value.
func equalNumber(a, b json.Number) bool {
	if a == b {
		return true
	}
	fa, errA := a.Float64()
	fb, errB := b.Float64()
	return errA == nil && errB == nil && fa == fb
}
