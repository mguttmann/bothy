// SPDX-License-Identifier: Apache-2.0

package domain

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"io"
	"regexp"
	"strconv"
	"time"
	"unicode/utf8"

	"github.com/OWNER/bothy/services/server/internal/kernel/designkeys"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/textcheck"
)

// The component types of component.v1 (09 11.1, CHECK of 03 message_components.type).
const (
	// ComponentButton is a button with one value.
	ComponentButton = "button"
	// ComponentSelect is a list of options with a bounded selection.
	ComponentSelect = "select"
	// ComponentForm is a form with typed fields.
	ComponentForm = "form"
)

// The states of a component (CHECK of 03 message_components.state).
const (
	// ComponentActive: the component can be used.
	ComponentActive = "active"
	// ComponentUsed: a single-use component was used; final.
	ComponentUsed = "used"
	// ComponentExpired: expires_at passed; final.
	ComponentExpired = "expired"
	// ComponentDisabled: a sibling was used, the run ended or the agent disabled it.
	ComponentDisabled = "disabled"
)

// The values of restricted_to (CHECK of 03 message_components.restricted_to, 09 11.2 point 4).
const (
	// RestrictAny: every principal that passes the other checks.
	RestrictAny = "any"
	// RestrictRunStarter: the person who triggered the run, or an effective owner.
	RestrictRunStarter = "run_starter"
	// RestrictOwner: an effective owner only.
	RestrictOwner = "owner"
)

// CapabilityApproveActions is the capability of an approval card (09 11.5); it makes
// run_starter the default of restricted_to (SEC-AI-4) and the allowed use audited (Design note 17).
const CapabilityApproveActions = "approve_actions"

// The codes of errors[] that the checks of this file and of CheckValues give (Design notes 4
// and 9; invalid is the generic code next to the codes of 06 5 point 3).
const (
	// FieldRequired: a required member is missing or empty.
	FieldRequired = "required"
	// FieldTooLong: custom_id has more code points than component.custom_id_max_chars.
	FieldTooLong = "too_long"
	// FieldLimitReached: more components than component.max_per_message or more fields than
	// component.form_max_fields.
	FieldLimitReached = "limit_reached"
	// FieldInvalid: every other rule (unknown member, wrong JSON type, value outside a list,
	// duplicate, bad bounds, rejected characters).
	FieldInvalid = "invalid"
	// FieldValuesInvalid: the values of an interaction do not fit the definition.
	FieldValuesInvalid = "interaction_values_invalid"
)

// CardLimits are the configuration keys of 09 21 (component.max_per_message,
// component.custom_id_max_chars, component.form_max_fields, component.default_ttl).
type CardLimits struct {
	MaxPerMessage    int
	CustomIDMaxChars int
	FormMaxFields    int
	DefaultTTL       time.Duration
}

// validate refuses limits that no configuration can produce, so a wiring mistake fails closed
// instead of accepting every card or creating components that expire at once.
func (l CardLimits) validate() error {
	if l.MaxPerMessage < 1 || l.CustomIDMaxChars < 1 || l.FormMaxFields < 1 || l.DefaultTTL <= 0 {
		return errs.New(errs.CodeInternalError, "runs: the component limits are not configured")
	}
	return nil
}

// ComponentInput is one member of MessageCreate.components as received; pointers mean "given".
type ComponentInput struct {
	CustomID, Type string
	// Definition is the JSON object as received; nil or "null" counts as missing.
	Definition         json.RawMessage
	SingleUse          *bool
	RequiredCapability string // "" when absent
	RestrictedTo       string // "" when absent
	ExpiresAt          *time.Time
}

// CheckedComponent is a component after CheckCard with the server fields of Design note 5.
type CheckedComponent struct {
	CustomID, Type string
	// Definition is the normalized definition: the typed form re-encoded in a fixed member order
	// with the defaults applied (button style, select min_values and max_values, form field
	// required) and without empty optional texts. It is the stored form.
	Definition         json.RawMessage
	Position           int // index in the request plus one
	SingleUse          bool
	RequiredCapability string // "" for none
	RestrictedTo       string
	ExpiresAt          time.Time // never zero
}

// The lists of component.v1 and of the server fields (Design notes 4 and 5).
var (
	buttonStyles = []string{"primary", "secondary", "success", "danger", "ghost"}
	fieldTypes   = []string{fieldText, fieldTextarea, fieldNumber, fieldBoolean, fieldChoice}
	// capabilities are the capabilities a component may require; terminal exists only on the
	// target host (07 2.4) and is refused.
	capabilities = []string{CapabilityApproveActions, "upload", "export", "manage"}
	restrictions = []string{RestrictAny, RestrictRunStarter, RestrictOwner}
	fieldName    = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)
)

// The defaults of component.v1 (Design note 4).
const (
	defaultButtonStyle = "secondary"
	defaultMinValues   = 1
	defaultMaxValues   = 1
)

// The typed definitions. The member order of each struct is the fixed encoding order of both
// the canonical bytes (Design note 6 c) and the normalized form; a pointer or nil slice is a
// member the request did not give.
type (
	buttonDefinition struct {
		Label string  `json:"label"`
		Style *string `json:"style,omitempty"`
		Icon  *string `json:"icon,omitempty"`
		Value string  `json:"value"`
	}
	selectOption struct {
		Label       string  `json:"label"`
		Value       string  `json:"value"`
		Description *string `json:"description,omitempty"`
	}
	selectDefinition struct {
		Placeholder *string        `json:"placeholder,omitempty"`
		Options     []selectOption `json:"options"`
		MinValues   *int           `json:"min_values,omitempty"`
		MaxValues   *int           `json:"max_values,omitempty"`
	}
	formField struct {
		Name      string   `json:"name"`
		Label     string   `json:"label"`
		Type      string   `json:"type"`
		Required  *bool    `json:"required,omitempty"`
		MaxLength *int     `json:"max_length,omitempty"`
		Choices   []string `json:"choices,omitempty"`
	}
	formDefinition struct {
		Title       string      `json:"title"`
		SubmitLabel *string     `json:"submit_label,omitempty"`
		Fields      []formField `json:"fields"`
	}
)

// CheckCard checks the components of one card at the request time now (Design notes 4 and 5)
// and returns them in request order with the canonical bytes of Design note 6 c, the input of
// the request hash bothy-msg-v2.
//
// Every component is checked completely and every finding is reported, with one exception: an
// empty card and a card with more than component.max_per_message components give only that one
// finding on /components, because the request has to change as a whole and the work stays
// bounded by the limit.
//
// Error: one *errs.Error validation_failed with one FieldError per finding, pointers below
// /components/<index>/..., codes FieldRequired, FieldTooLong, FieldLimitReached or FieldInvalid;
// the Detail names the rule, never the text. An internal error when lim is not configured.
//
// Example:
//
//	comps, canonical, err := domain.CheckCard(in.Components, now, limits)
//	if err != nil {
//		return err // 400 validation_failed with errors[]
//	}
func CheckCard(in []ComponentInput, now time.Time, lim CardLimits) ([]CheckedComponent, []byte, error) {
	if err := lim.validate(); err != nil {
		return nil, nil, err
	}
	c := &findings{}
	switch {
	case len(in) == 0:
		c.add("/components", FieldRequired, "a card needs at least one component")
		return nil, nil, c.err()
	case len(in) > lim.MaxPerMessage:
		c.add("/components", FieldLimitReached, "the card has more components than component.max_per_message")
		return nil, nil, c.err()
	}
	out := make([]CheckedComponent, 0, len(in))
	var canonical []byte
	customIDs := make(map[string]bool, len(in))
	for i, comp := range in {
		p := "/components/" + strconv.Itoa(i)
		checked, canon := checkComponent(c, comp, p, now, lim, customIDs)
		checked.Position = i + 1
		out = append(out, checked)
		canonical = append(canonical, canon...)
	}
	if len(c.list) > 0 {
		return nil, nil, c.err()
	}
	return out, canonical, nil
}

// checkComponent checks one component at pointer p and returns its checked form and its part of
// the canonical bytes; both are meaningless when c gained a finding.
func checkComponent(c *findings, in ComponentInput, p string, now time.Time, lim CardLimits, customIDs map[string]bool) (CheckedComponent, []byte) {
	out := CheckedComponent{CustomID: in.CustomID, Type: in.Type, SingleUse: true, RequiredCapability: in.RequiredCapability}
	checkCustomID(c, in.CustomID, p+"/custom_id", lim.CustomIDMaxChars, customIDs)

	var asReceived []byte
	switch {
	case in.Type == "":
		c.add(p+"/type", FieldRequired, "type is missing")
	case !knownComponentType(in.Type):
		// The definition of an unknown type has no rules to check it against.
		c.add(p+"/type", FieldInvalid, "type is not one of button, select, form")
	default:
		if def := parseDefinition(c, in.Type, in.Definition, p+"/definition", lim.FormMaxFields); def != nil {
			asReceived = encodeDefinition(def)
			out.Definition = encodeDefinition(normalizeDefinition(def))
		}
	}

	singleUse := ""
	if in.SingleUse != nil {
		out.SingleUse = *in.SingleUse
		singleUse = strconv.FormatBool(*in.SingleUse)
	}
	if in.RequiredCapability != "" && !oneOf(in.RequiredCapability, capabilities) {
		c.add(p+"/required_capability", FieldInvalid, "required_capability is not one of approve_actions, upload, export, manage")
	}
	out.RestrictedTo = restriction(c, in, p+"/restricted_to")

	expiresAt := ""
	switch {
	case in.ExpiresAt == nil:
		out.ExpiresAt = now.Add(lim.DefaultTTL)
	case !in.ExpiresAt.After(now):
		c.add(p+"/expires_at", FieldInvalid, "expires_at does not lie after the request time")
	default:
		out.ExpiresAt = *in.ExpiresAt
		expiresAt = in.ExpiresAt.UTC().Format(canonicalTime)
	}

	var canon []byte
	for _, part := range [][]byte{
		[]byte(in.CustomID), []byte(in.Type), asReceived, []byte(singleUse),
		[]byte(in.RequiredCapability), []byte(in.RestrictedTo), []byte(expiresAt),
	} {
		canon = binary.AppendUvarint(canon, uint64(len(part)))
		canon = append(canon, part...)
	}
	return out, canon
}

// checkCustomID checks custom_id at pointer p: not empty, a display text, at most maxChars code
// points and unique within the card (a duplicate is reported on the later component).
func checkCustomID(c *findings, id, p string, maxChars int, seen map[string]bool) {
	switch {
	case id == "":
		c.add(p, FieldRequired, "custom_id is missing")
	case textcheck.DisplayText(id) != nil:
		c.add(p, FieldInvalid, "custom_id contains a rejected character")
	case utf8.RuneCountInString(id) > maxChars:
		c.add(p, FieldTooLong, "custom_id is longer than component.custom_id_max_chars")
	case seen[id]:
		c.add(p, FieldInvalid, "custom_id is not unique within the card")
	default:
		seen[id] = true
	}
}

// restriction returns restricted_to of in, or its default: run_starter for an approve_actions
// component (SEC-AI-4: the person who started the run decides an approval unless the agent says
// otherwise), any for every other one. A value outside the list is a finding at p.
func restriction(c *findings, in ComponentInput, p string) string {
	switch {
	case in.RestrictedTo != "" && !oneOf(in.RestrictedTo, restrictions):
		c.add(p, FieldInvalid, "restricted_to is not one of any, run_starter, owner")
		return ""
	case in.RestrictedTo != "":
		return in.RestrictedTo
	case in.RequiredCapability == CapabilityApproveActions:
		return RestrictRunStarter
	}
	return RestrictAny
}

// canonicalTime is the timestamp layout of the canonical bytes and of event payloads: RFC 3339
// in UTC with milliseconds.
const canonicalTime = "2006-01-02T15:04:05.000Z"

// CheckDefinition checks a definition of updateComponent for the component type typ (Design
// note 13 c: it must pass Design note 4) and returns its normalized form, the same bytes
// CheckCard stores; a normalized definition comes back unchanged. Only lim.FormMaxFields is read.
//
// Error: one *errs.Error validation_failed with one FieldError per finding, pointers below
// /definition. An internal error for a type outside the three (the stored type is always one of
// them) and for an unconfigured lim.FormMaxFields.
//
// Example:
//
//	norm, err := domain.CheckDefinition(row.Type, cmd.Definition, limits)
func CheckDefinition(typ string, def json.RawMessage, lim CardLimits) (json.RawMessage, error) {
	if !knownComponentType(typ) {
		return nil, errs.New(errs.CodeInternalError, "runs: unknown component type")
	}
	if lim.FormMaxFields < 1 {
		return nil, errs.New(errs.CodeInternalError, "runs: the component limits are not configured")
	}
	c := &findings{}
	parsed := parseDefinition(c, typ, def, "/definition", lim.FormMaxFields)
	if len(c.list) > 0 {
		return nil, c.err()
	}
	return encodeDefinition(normalizeDefinition(parsed)), nil
}

// SameDefinition reports whether two normalized definitions are equal as JSON (key order and
// white space ignored, numbers compared by value): the "no change" test of updateComponent
// (Design note 13 d). A document that does not decode is never the same.
func SameDefinition(a, b json.RawMessage) bool {
	va, okA := decodeJSONOK(a)
	vb, okB := decodeJSONOK(b)
	return okA && okB && equalJSON(va, vb)
}

// knownComponentType reports whether typ is one of the three component types.
func knownComponentType(typ string) bool {
	return typ == ComponentButton || typ == ComponentSelect || typ == ComponentForm
}

// findings collects the field errors of one check, so a request learns every finding at once.
type findings struct {
	list []errs.FieldError
}

// add records one finding; detail names the rule and never the offending text.
func (c *findings) add(pointer, code, detail string) {
	c.list = append(c.list, errs.FieldError{Pointer: pointer, Code: code, Detail: detail})
}

// err returns the validation_failed error with every finding.
func (c *findings) err() error {
	return errs.New(errs.CodeValidationFailed, "the request breaks a rule of component.v1").WithFields(c.list...)
}

// parseDefinition decodes raw at pointer p into the typed definition of typ (a *buttonDefinition,
// *selectDefinition or *formDefinition, members as received) and records every finding in c. It
// returns nil when it recorded one. maxFields is component.form_max_fields; zero skips that
// limit, for definitions that passed it when they were stored.
func parseDefinition(c *findings, typ string, raw json.RawMessage, p string, maxFields int) any {
	before := len(c.list)
	if isAbsent(raw) {
		c.add(p, FieldRequired, "the definition is missing")
		return nil
	}
	obj, ok := decodeObject(c, raw, p)
	if !ok {
		return nil
	}
	var def any
	switch typ {
	case ComponentButton:
		def = parseButton(c, obj, p)
	case ComponentSelect:
		def = parseSelect(c, obj, p)
	case ComponentForm:
		def = parseForm(c, obj, p, maxFields)
	default:
		return nil
	}
	if len(c.list) > before {
		return nil
	}
	return def
}

// isAbsent reports whether raw is empty, white space or JSON null: a document that was not given.
func isAbsent(raw json.RawMessage) bool {
	t := bytes.TrimSpace(raw)
	return len(t) == 0 || string(t) == "null"
}

// object is a decoded JSON object: its members by key.
type object map[string]json.RawMessage

// decodeObject decodes raw as one JSON object; anything else is FieldInvalid at p.
func decodeObject(c *findings, raw json.RawMessage, p string) (object, bool) {
	obj, ok := objectMembers(raw)
	if !ok {
		c.add(p, FieldInvalid, "a JSON object is expected")
	}
	return obj, ok
}

// objectMembers decodes raw as exactly one JSON object with unique keys in valid UTF-8. A
// repeated key makes the object ambiguous (decoders differ in the member they keep), so it is
// refused like a document that is no object, and so is trailing data.
func objectMembers(raw json.RawMessage) (object, bool) {
	if !utf8.Valid(raw) {
		return nil, false
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	if tok, err := dec.Token(); err != nil || tok != json.Delim('{') {
		return nil, false
	}
	obj := object{}
	for dec.More() {
		tok, err := dec.Token()
		if err != nil {
			return nil, false
		}
		key, _ := tok.(string) // inside an object the decoder yields only string keys
		if _, dup := obj[key]; dup {
			return nil, false
		}
		var v json.RawMessage
		if err := dec.Decode(&v); err != nil {
			return nil, false
		}
		obj[key] = v
	}
	if _, err := dec.Token(); err != nil { // the closing brace
		return nil, false
	}
	if _, err := dec.Token(); err != io.EOF {
		return nil, false
	}
	return obj, true
}

// The details of the findings that several rules share.
const (
	detailMissing    = "a required member is missing"
	detailNotString  = "a JSON string is expected"
	detailRejected   = "the text contains a control or bidi character"
	detailOutsideSet = "the object has a member outside component.v1"
)

// closed records FieldInvalid at p when obj has a member outside allowed. The pointer names the
// object, never the member, because the keys are agent text (SEC-T10).
func closed(c *findings, obj object, p string, allowed ...string) {
	for key := range obj {
		if !oneOf(key, allowed) {
			c.add(p, FieldInvalid, detailOutsideSet)
			return
		}
	}
}

// text decodes the string member key of obj at pointer p and checks it with
// textcheck.DisplayText. A required member that is absent or empty gives FieldRequired; a
// member of another JSON type (null included) or with a rejected character gives FieldInvalid.
// It returns nil for an absent member and for every finding.
func text(c *findings, obj object, key, p string, required bool) *string {
	raw, ok := obj[key]
	if !ok {
		if required {
			c.add(p, FieldRequired, detailMissing)
		}
		return nil
	}
	s, ok := decodeString(raw)
	switch {
	case !ok:
		c.add(p, FieldInvalid, detailNotString)
		return nil
	case required && s == "":
		c.add(p, FieldRequired, "a required member is empty")
		return nil
	case textcheck.DisplayText(s) != nil:
		c.add(p, FieldInvalid, detailRejected)
		return nil
	}
	return &s
}

// decodeString decodes raw as a JSON string; null and every other type are refused (the
// standard decoder would turn null into "").
func decodeString(raw json.RawMessage) (string, bool) {
	raw = bytes.TrimSpace(raw)
	var s string
	if len(raw) == 0 || raw[0] != '"' || json.Unmarshal(raw, &s) != nil {
		return "", false
	}
	return s, true
}

// integer decodes the member key of obj as a JSON integer (a number literal without fraction or
// exponent); any other value gives FieldInvalid at p. It returns nil for an absent member and
// for a finding.
func integer(c *findings, obj object, key, p string) *int {
	raw, ok := obj[key]
	if !ok {
		return nil
	}
	n, err := strconv.ParseInt(string(bytes.TrimSpace(raw)), 10, 0)
	if err != nil {
		c.add(p, FieldInvalid, "an integer is expected")
		return nil
	}
	v := int(n)
	return &v
}

// boolean decodes the member key of obj as a JSON boolean; any other value gives FieldInvalid.
func boolean(c *findings, obj object, key, p string) *bool {
	raw, ok := obj[key]
	if !ok {
		return nil
	}
	v, ok := decodeBool(raw)
	if !ok {
		c.add(p, FieldInvalid, "a JSON boolean is expected")
		return nil
	}
	return &v
}

// decodeBool decodes raw as the JSON literal true or false.
func decodeBool(raw json.RawMessage) (v, ok bool) {
	switch string(bytes.TrimSpace(raw)) {
	case "true":
		return true, true
	case "false":
		return false, true
	}
	return false, false
}

// list decodes the required member key of obj as a non-empty JSON array: absent or empty gives
// FieldRequired, another JSON type FieldInvalid at p.
func list(c *findings, obj object, key, p string) []json.RawMessage {
	raw, ok := obj[key]
	if !ok {
		c.add(p, FieldRequired, detailMissing)
		return nil
	}
	raw = bytes.TrimSpace(raw)
	var items []json.RawMessage
	if len(raw) == 0 || raw[0] != '[' || json.Unmarshal(raw, &items) != nil {
		c.add(p, FieldInvalid, "a JSON array is expected")
		return nil
	}
	if len(items) == 0 {
		c.add(p, FieldRequired, "the list is empty")
	}
	return items
}

// parseButton decodes a button definition (Design note 4).
func parseButton(c *findings, obj object, p string) *buttonDefinition {
	closed(c, obj, p, "label", "style", "icon", "value")
	d := &buttonDefinition{}
	if s := text(c, obj, "label", p+"/label", true); s != nil {
		d.Label = *s
	}
	if s := text(c, obj, "style", p+"/style", false); s != nil {
		if oneOf(*s, buttonStyles) {
			d.Style = s
		} else {
			c.add(p+"/style", FieldInvalid, "style is not a role of the design system")
		}
	}
	if s := text(c, obj, "icon", p+"/icon", false); s != nil {
		// A semantic icon name only: an emoji, a glyph file name or a free name is refused
		// (docs/design/icons.md "Rules").
		if designkeys.IsIconName(*s) {
			d.Icon = s
		} else {
			c.add(p+"/icon", FieldInvalid, "icon is not a semantic icon name")
		}
	}
	if s := text(c, obj, "value", p+"/value", true); s != nil {
		d.Value = *s
	}
	return d
}

// parseSelect decodes a select definition (Design note 4).
func parseSelect(c *findings, obj object, p string) *selectDefinition {
	closed(c, obj, p, "placeholder", "options", "min_values", "max_values")
	d := &selectDefinition{Placeholder: text(c, obj, "placeholder", p+"/placeholder", false)}
	items := list(c, obj, "options", p+"/options")
	values := make(map[string]bool, len(items))
	for k, item := range items {
		if opt, ok := parseOption(c, item, p+"/options/"+strconv.Itoa(k), values); ok {
			d.Options = append(d.Options, opt)
		}
	}
	d.MinValues = integer(c, obj, "min_values", p+"/min_values")
	d.MaxValues = integer(c, obj, "max_values", p+"/max_values")
	if len(items) > 0 {
		checkBounds(c, d, len(items), p)
	}
	return d
}

// parseOption decodes one option at pointer op; values holds the values of the options before it.
func parseOption(c *findings, item json.RawMessage, op string, values map[string]bool) (selectOption, bool) {
	o, ok := decodeObject(c, item, op)
	if !ok {
		return selectOption{}, false
	}
	closed(c, o, op, "label", "value", "description")
	opt := selectOption{Description: text(c, o, "description", op+"/description", false)}
	if s := text(c, o, "label", op+"/label", true); s != nil {
		opt.Label = *s
	}
	if s := text(c, o, "value", op+"/value", true); s != nil {
		if values[*s] {
			c.add(op+"/value", FieldInvalid, "the option value is not unique within the select")
		}
		values[*s] = true
		opt.Value = *s
	}
	return opt, true
}

// checkBounds checks min_values and max_values of d with their defaults against the number of
// options: 0 <= min_values <= max_values, max_values >= 1 and at most the options. The finding
// names the member that breaks the first rule that fails.
func checkBounds(c *findings, d *selectDefinition, options int, p string) {
	minV, maxV := defaultMinValues, defaultMaxValues
	if d.MinValues != nil {
		minV = *d.MinValues
	}
	if d.MaxValues != nil {
		maxV = *d.MaxValues
	}
	switch {
	case minV < 0:
		c.add(p+"/min_values", FieldInvalid, "min_values is below zero")
	case maxV < 1:
		c.add(p+"/max_values", FieldInvalid, "max_values is below one")
	case minV > maxV:
		c.add(p+"/min_values", FieldInvalid, "min_values is above max_values")
	case maxV > options:
		c.add(p+"/max_values", FieldInvalid, "max_values is above the number of options")
	}
}

// parseForm decodes a form definition (Design note 4); maxFields zero skips the field limit.
func parseForm(c *findings, obj object, p string, maxFields int) *formDefinition {
	closed(c, obj, p, "title", "submit_label", "fields")
	d := &formDefinition{SubmitLabel: text(c, obj, "submit_label", p+"/submit_label", false)}
	if s := text(c, obj, "title", p+"/title", true); s != nil {
		d.Title = *s
	}
	items := list(c, obj, "fields", p+"/fields")
	if maxFields > 0 && len(items) > maxFields {
		c.add(p+"/fields", FieldLimitReached, "the form has more fields than component.form_max_fields")
	}
	names := make(map[string]bool, len(items))
	for k, item := range items {
		fp := p + "/fields/" + strconv.Itoa(k)
		if o, ok := decodeObject(c, item, fp); ok {
			d.Fields = append(d.Fields, parseField(c, o, fp, names))
		}
	}
	return d
}

// The field types of a form (Design note 4).
const (
	fieldText     = "text"
	fieldTextarea = "textarea"
	fieldNumber   = "number"
	fieldBoolean  = "boolean"
	fieldChoice   = "choice"
)

// parseField decodes one form field at pointer fp; names holds the names of the fields before it.
func parseField(c *findings, o object, fp string, names map[string]bool) formField {
	closed(c, o, fp, "name", "label", "type", "required", "max_length", "choices")
	var f formField
	if s := text(c, o, "name", fp+"/name", true); s != nil {
		switch {
		case !fieldName.MatchString(*s):
			c.add(fp+"/name", FieldInvalid, "the field name does not match ^[a-z][a-z0-9_]*$")
		case names[*s]:
			c.add(fp+"/name", FieldInvalid, "the field name is not unique within the form")
		}
		names[*s] = true
		f.Name = *s
	}
	if s := text(c, o, "label", fp+"/label", true); s != nil {
		f.Label = *s
	}
	if s := text(c, o, "type", fp+"/type", true); s != nil {
		if oneOf(*s, fieldTypes) {
			f.Type = *s
		} else {
			c.add(fp+"/type", FieldInvalid, "the field type is not one of text, textarea, number, boolean, choice")
		}
	}
	f.Required = boolean(c, o, "required", fp+"/required")
	f.MaxLength = maxLength(c, o, f.Type, fp+"/max_length")
	f.Choices = choices(c, o, f.Type, fp+"/choices")
	return f
}

// maxLength decodes max_length of a field of type typ: an integer of at least one, only for
// text and textarea. A field without a valid type ("") is not judged; its type finding stands.
func maxLength(c *findings, o object, typ, p string) *int {
	n := integer(c, o, "max_length", p)
	switch {
	case n == nil || typ == "":
		return n
	case typ != fieldText && typ != fieldTextarea:
		c.add(p, FieldInvalid, "max_length is only allowed for text and textarea")
	case *n < 1:
		c.add(p, FieldInvalid, "max_length is below one")
	}
	return n
}

// choices decodes the choices of a field of type typ: required for choice and only there, a
// non-empty list of unique, non-empty display texts. An empty entry is refused because it could
// not be drawn as a choice. A field without a valid type ("") is not judged.
func choices(c *findings, o object, typ, p string) []string {
	_, given := o["choices"]
	switch {
	case typ == "":
		return nil
	case typ != fieldChoice:
		if given {
			c.add(p, FieldInvalid, "choices are only allowed for choice")
		}
		return nil
	}
	items := list(c, o, "choices", p)
	out := make([]string, 0, len(items))
	seen := make(map[string]bool, len(items))
	for j, item := range items {
		ep := p + "/" + strconv.Itoa(j)
		s, ok := decodeString(item)
		switch {
		case !ok:
			c.add(ep, FieldInvalid, detailNotString)
		case s == "":
			c.add(ep, FieldInvalid, "the choice is empty")
		case textcheck.DisplayText(s) != nil:
			c.add(ep, FieldInvalid, detailRejected)
		case seen[s]:
			c.add(ep, FieldInvalid, "the choice is not unique within the field")
		default:
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}

// normalizeDefinition returns a copy of a parsed definition with the defaults applied and the
// empty optional texts dropped (no empty string members in the stored form).
func normalizeDefinition(def any) any {
	switch d := def.(type) {
	case *buttonDefinition:
		n := *d
		if n.Style == nil {
			style := defaultButtonStyle
			n.Style = &style
		}
		n.Icon = nonEmpty(n.Icon)
		return &n
	case *selectDefinition:
		n := *d
		n.Placeholder = nonEmpty(n.Placeholder)
		n.Options = make([]selectOption, len(d.Options))
		for i, o := range d.Options {
			o.Description = nonEmpty(o.Description)
			n.Options[i] = o
		}
		if n.MinValues == nil {
			v := defaultMinValues
			n.MinValues = &v
		}
		if n.MaxValues == nil {
			v := defaultMaxValues
			n.MaxValues = &v
		}
		return &n
	case *formDefinition:
		n := *d
		n.SubmitLabel = nonEmpty(n.SubmitLabel)
		n.Fields = make([]formField, len(d.Fields))
		for i, f := range d.Fields {
			if f.Required == nil {
				v := false
				f.Required = &v
			}
			n.Fields[i] = f
		}
		return &n
	}
	return def
}

// nonEmpty returns nil for a nil or empty text, else s.
func nonEmpty(s *string) *string {
	if s == nil || *s == "" {
		return nil
	}
	return s
}

// encodeDefinition encodes a typed definition in its fixed member order. The structs hold only
// strings, integers, booleans and slices of them, so encoding cannot fail.
func encodeDefinition(def any) []byte {
	b, err := json.Marshal(def)
	if err != nil {
		panic("runs: a typed component definition does not encode: " + err.Error())
	}
	return b
}
