// SPDX-License-Identifier: Apache-2.0

package domain_test

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/runs/domain"
)

// cardNow is the request time of every card test.
var cardNow = time.Date(2026, 10, 21, 9, 30, 0, 0, time.UTC)

// cardLimits stand for the configuration keys; the tests only need small values.
var cardLimits = domain.CardLimits{MaxPerMessage: 3, CustomIDMaxChars: 12, FormMaxFields: 2, DefaultTTL: 24 * time.Hour}

const (
	okButton = `{"label":"Allow once","value":"once"}`
	okSelect = `{"options":[{"label":"Small","value":"s"},{"label":"Large","value":"l"}]}`
	okForm   = `{"title":"Contact","fields":[{"name":"note","label":"Note","type":"text"}]}`
)

// comp builds a component input with the given custom_id, type and definition.
func comp(customID, typ, def string) domain.ComponentInput {
	return domain.ComponentInput{CustomID: customID, Type: typ, Definition: json.RawMessage(def)}
}

func boolRef(b bool) *bool { return &b }

func timeRef(t time.Time) *time.Time { return &t }

// fieldsOf returns the field errors of a validation_failed error and fails on any other error.
func fieldsOf(t *testing.T, err error) []errs.FieldError {
	t.Helper()
	e, ok := errs.As(err)
	if !ok || e.Code != errs.CodeValidationFailed {
		t.Fatalf("error = %v, want validation_failed", err)
	}
	for _, f := range e.Fields {
		if f.Detail == "" {
			t.Errorf("field error %s has no detail", f.Pointer)
		}
	}
	return e.Fields
}

// wantFinding checks that err carries exactly the one finding pointer/code.
func wantFinding(t *testing.T, err error, pointer, code string) {
	t.Helper()
	fs := fieldsOf(t, err)
	if len(fs) != 1 || fs[0].Pointer != pointer || fs[0].Code != code {
		t.Fatalf("findings = %+v, want one %s at %s", fs, code, pointer)
	}
}

func TestCheckCard(t *testing.T) {
	past := cardNow.Add(-time.Minute)
	cases := []struct {
		name    string
		in      []domain.ComponentInput
		pointer string
		code    string
	}{
		{"unknown definition key", []domain.ComponentInput{comp("a", "button", `{"label":"Go","value":"go","author":"Sam"}`)}, "/components/0/definition", domain.FieldInvalid},
		{"unknown key in an option", []domain.ComponentInput{comp("a", "select", `{"options":[{"label":"A","value":"a","color":"red"}]}`)}, "/components/0/definition/options/0", domain.FieldInvalid},
		{"missing label", []domain.ComponentInput{comp("a", "button", `{"value":"go"}`)}, "/components/0/definition/label", domain.FieldRequired},
		{"empty label", []domain.ComponentInput{comp("a", "button", `{"label":"","value":"go"}`)}, "/components/0/definition/label", domain.FieldRequired},
		{"missing value", []domain.ComponentInput{comp("a", "button", `{"label":"Go"}`)}, "/components/0/definition/value", domain.FieldRequired},
		{"label of the wrong type", []domain.ComponentInput{comp("a", "button", `{"label":7,"value":"go"}`)}, "/components/0/definition/label", domain.FieldInvalid},
		{"label null", []domain.ComponentInput{comp("a", "button", `{"label":null,"value":"go"}`)}, "/components/0/definition/label", domain.FieldInvalid},
		{"style outside the roles", []domain.ComponentInput{comp("a", "button", `{"label":"Go","value":"go","style":"#ff0000"}`)}, "/components/0/definition/style", domain.FieldInvalid},
		{"icon not a glyph name", []domain.ComponentInput{comp("a", "button", `{"label":"Go","value":"go","icon":"warning"}`)}, "/components/0/definition/icon", domain.FieldInvalid},
		{"emoji icon", []domain.ComponentInput{comp("a", "button", `{"label":"Go","value":"go","icon":"🚀"}`)}, "/components/0/definition/icon", domain.FieldInvalid},
		{"free icon name", []domain.ComponentInput{comp("a", "button", `{"label":"Go","value":"go","icon":"rocket"}`)}, "/components/0/definition/icon", domain.FieldInvalid},
		{"definition not an object", []domain.ComponentInput{comp("a", "button", `["Go"]`)}, "/components/0/definition", domain.FieldInvalid},
		{"definition with a duplicate key", []domain.ComponentInput{comp("a", "button", `{"label":"Go","label":"Stop","value":"go"}`)}, "/components/0/definition", domain.FieldInvalid},
		{"definition missing", []domain.ComponentInput{comp("a", "button", ``)}, "/components/0/definition", domain.FieldRequired},
		{"definition null", []domain.ComponentInput{comp("a", "button", `null`)}, "/components/0/definition", domain.FieldRequired},
		{"duplicate option value", []domain.ComponentInput{comp("a", "select", `{"options":[{"label":"A","value":"x"},{"label":"B","value":"x"}]}`)}, "/components/0/definition/options/1/value", domain.FieldInvalid},
		{"options missing", []domain.ComponentInput{comp("a", "select", `{"placeholder":"Pick one"}`)}, "/components/0/definition/options", domain.FieldRequired},
		{"options empty", []domain.ComponentInput{comp("a", "select", `{"options":[]}`)}, "/components/0/definition/options", domain.FieldRequired},
		{"option label missing", []domain.ComponentInput{comp("a", "select", `{"options":[{"value":"x"}]}`)}, "/components/0/definition/options/0/label", domain.FieldRequired},
		{"min_values above max_values", []domain.ComponentInput{comp("a", "select", `{"options":[{"label":"A","value":"a"},{"label":"B","value":"b"},{"label":"C","value":"c"}],"min_values":3,"max_values":2}`)}, "/components/0/definition/min_values", domain.FieldInvalid},
		{"min_values negative", []domain.ComponentInput{comp("a", "select", `{"options":[{"label":"A","value":"a"}],"min_values":-1}`)}, "/components/0/definition/min_values", domain.FieldInvalid},
		{"max_values zero", []domain.ComponentInput{comp("a", "select", `{"options":[{"label":"A","value":"a"}],"min_values":0,"max_values":0}`)}, "/components/0/definition/max_values", domain.FieldInvalid},
		{"max_values above the options", []domain.ComponentInput{comp("a", "select", `{"options":[{"label":"A","value":"a"}],"max_values":2}`)}, "/components/0/definition/max_values", domain.FieldInvalid},
		{"max_values not an integer", []domain.ComponentInput{comp("a", "select", `{"options":[{"label":"A","value":"a"}],"max_values":1.5}`)}, "/components/0/definition/max_values", domain.FieldInvalid},
		{"max_values as a string", []domain.ComponentInput{comp("a", "select", `{"options":[{"label":"A","value":"a"}],"max_values":"1"}`)}, "/components/0/definition/max_values", domain.FieldInvalid},
		{"more fields than component.form_max_fields", []domain.ComponentInput{comp("a", "form", `{"title":"T","fields":[{"name":"a","label":"A","type":"text"},{"name":"b","label":"B","type":"text"},{"name":"c","label":"C","type":"text"}]}`)}, "/components/0/definition/fields", domain.FieldLimitReached},
		{"fields missing", []domain.ComponentInput{comp("a", "form", `{"title":"T"}`)}, "/components/0/definition/fields", domain.FieldRequired},
		{"title missing", []domain.ComponentInput{comp("a", "form", `{"fields":[{"name":"a","label":"A","type":"text"}]}`)}, "/components/0/definition/title", domain.FieldRequired},
		{"field name with an upper case letter", []domain.ComponentInput{comp("a", "form", `{"title":"T","fields":[{"name":"Note","label":"A","type":"text"}]}`)}, "/components/0/definition/fields/0/name", domain.FieldInvalid},
		{"field name starting with a digit", []domain.ComponentInput{comp("a", "form", `{"title":"T","fields":[{"name":"1st","label":"A","type":"text"}]}`)}, "/components/0/definition/fields/0/name", domain.FieldInvalid},
		{"duplicate field name", []domain.ComponentInput{comp("a", "form", `{"title":"T","fields":[{"name":"a","label":"A","type":"text"},{"name":"a","label":"B","type":"number"}]}`)}, "/components/0/definition/fields/1/name", domain.FieldInvalid},
		{"field type unknown", []domain.ComponentInput{comp("a", "form", `{"title":"T","fields":[{"name":"a","label":"A","type":"date"}]}`)}, "/components/0/definition/fields/0/type", domain.FieldInvalid},
		{"field type missing", []domain.ComponentInput{comp("a", "form", `{"title":"T","fields":[{"name":"a","label":"A"}]}`)}, "/components/0/definition/fields/0/type", domain.FieldRequired},
		{"field required not a boolean", []domain.ComponentInput{comp("a", "form", `{"title":"T","fields":[{"name":"a","label":"A","type":"text","required":"yes"}]}`)}, "/components/0/definition/fields/0/required", domain.FieldInvalid},
		{"max_length on a boolean", []domain.ComponentInput{comp("a", "form", `{"title":"T","fields":[{"name":"a","label":"A","type":"boolean","max_length":4}]}`)}, "/components/0/definition/fields/0/max_length", domain.FieldInvalid},
		{"max_length zero", []domain.ComponentInput{comp("a", "form", `{"title":"T","fields":[{"name":"a","label":"A","type":"text","max_length":0}]}`)}, "/components/0/definition/fields/0/max_length", domain.FieldInvalid},
		{"choices missing on a choice", []domain.ComponentInput{comp("a", "form", `{"title":"T","fields":[{"name":"a","label":"A","type":"choice"}]}`)}, "/components/0/definition/fields/0/choices", domain.FieldRequired},
		{"choices empty on a choice", []domain.ComponentInput{comp("a", "form", `{"title":"T","fields":[{"name":"a","label":"A","type":"choice","choices":[]}]}`)}, "/components/0/definition/fields/0/choices", domain.FieldRequired},
		{"choices on a text", []domain.ComponentInput{comp("a", "form", `{"title":"T","fields":[{"name":"a","label":"A","type":"text","choices":["x"]}]}`)}, "/components/0/definition/fields/0/choices", domain.FieldInvalid},
		{"duplicate choice", []domain.ComponentInput{comp("a", "form", `{"title":"T","fields":[{"name":"a","label":"A","type":"choice","choices":["x","x"]}]}`)}, "/components/0/definition/fields/0/choices/1", domain.FieldInvalid},
		{"choice not a string", []domain.ComponentInput{comp("a", "form", `{"title":"T","fields":[{"name":"a","label":"A","type":"choice","choices":["x",2]}]}`)}, "/components/0/definition/fields/0/choices/1", domain.FieldInvalid},
		{"more components than component.max_per_message", []domain.ComponentInput{comp("a", "button", okButton), comp("b", "button", okButton), comp("c", "button", okButton), comp("d", "button", okButton)}, "/components", domain.FieldLimitReached},
		{"empty card", nil, "/components", domain.FieldRequired},
		{"custom_id longer than component.custom_id_max_chars", []domain.ComponentInput{comp("abcdefghijklm", "button", okButton)}, "/components/0/custom_id", domain.FieldTooLong},
		{"custom_id empty", []domain.ComponentInput{comp("", "button", okButton)}, "/components/0/custom_id", domain.FieldRequired},
		{"custom_id with a control character", []domain.ComponentInput{comp("a\tb", "button", okButton)}, "/components/0/custom_id", domain.FieldInvalid},
		{"duplicate custom_id", []domain.ComponentInput{comp("a", "button", okButton), comp("a", "button", okButton)}, "/components/1/custom_id", domain.FieldInvalid},
		{"bidi character in a label", []domain.ComponentInput{comp("a", "button", `{"label":"Allow \u202eonce","value":"once"}`)}, "/components/0/definition/label", domain.FieldInvalid},
		{"bidi character in an option description", []domain.ComponentInput{comp("a", "select", `{"options":[{"label":"A","value":"a","description":"x\u2066y"}]}`)}, "/components/0/definition/options/0/description", domain.FieldInvalid},
		{"line break in a value", []domain.ComponentInput{comp("a", "button", `{"label":"Go","value":"a\nb"}`)}, "/components/0/definition/value", domain.FieldInvalid},
		{"type outside the three", []domain.ComponentInput{comp("a", "slider", `{"anything":true}`)}, "/components/0/type", domain.FieldInvalid},
		{"type missing", []domain.ComponentInput{comp("a", "", okButton)}, "/components/0/type", domain.FieldRequired},
		{"required_capability terminal", []domain.ComponentInput{{CustomID: "a", Type: "button", Definition: json.RawMessage(okButton), RequiredCapability: "terminal"}}, "/components/0/required_capability", domain.FieldInvalid},
		{"restricted_to unknown", []domain.ComponentInput{{CustomID: "a", Type: "button", Definition: json.RawMessage(okButton), RestrictedTo: "everyone"}}, "/components/0/restricted_to", domain.FieldInvalid},
		{"expires_at in the past", []domain.ComponentInput{{CustomID: "a", Type: "button", Definition: json.RawMessage(okButton), ExpiresAt: &past}}, "/components/0/expires_at", domain.FieldInvalid},
		{"expires_at equal to the request time", []domain.ComponentInput{{CustomID: "a", Type: "button", Definition: json.RawMessage(okButton), ExpiresAt: timeRef(cardNow)}}, "/components/0/expires_at", domain.FieldInvalid},
		{"finding in the second component", []domain.ComponentInput{comp("a", "button", okButton), comp("b", "button", `{"label":"Go"}`)}, "/components/1/definition/value", domain.FieldRequired},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, canonical, err := domain.CheckCard(tc.in, cardNow, cardLimits)
			if got != nil || canonical != nil {
				t.Errorf("CheckCard returned components or bytes with an error")
			}
			wantFinding(t, err, tc.pointer, tc.code)
		})
	}
}

func TestCheckCard_AllFindings(t *testing.T) {
	in := []domain.ComponentInput{
		comp("", "button", `{"label":"Go"}`),
		{CustomID: "b", Type: "select", Definition: json.RawMessage(`{"options":[]}`), RequiredCapability: "terminal"},
	}
	_, _, err := domain.CheckCard(in, cardNow, cardLimits)
	got := fieldsOf(t, err)
	want := []errs.FieldError{
		{Pointer: "/components/0/custom_id", Code: domain.FieldRequired},
		{Pointer: "/components/0/definition/value", Code: domain.FieldRequired},
		{Pointer: "/components/1/definition/options", Code: domain.FieldRequired},
		{Pointer: "/components/1/required_capability", Code: domain.FieldInvalid},
	}
	if len(got) != len(want) {
		t.Fatalf("findings = %+v, want %+v", got, want)
	}
	for i := range want {
		if got[i].Pointer != want[i].Pointer || got[i].Code != want[i].Code {
			t.Errorf("finding %d = %+v, want %+v", i, got[i], want[i])
		}
	}
}

func TestCheckCard_DetailNamesNoText(t *testing.T) {
	secret := "Robin\u202eexample.com"
	in := []domain.ComponentInput{comp(secret, "button", `{"label":"`+secret+`","value":"`+secret+`","icon":"`+secret+`","style":"`+secret+`"}`)}
	_, _, err := domain.CheckCard(in, cardNow, cardLimits)
	e, _ := errs.As(err)
	if e == nil || len(e.Fields) < 4 {
		t.Fatalf("error = %v, want several findings", err)
	}
	for _, f := range e.Fields {
		if strings.Contains(f.Detail, "Robin") || strings.Contains(f.Detail, "example.com") {
			t.Errorf("detail at %s repeats the text: %q", f.Pointer, f.Detail)
		}
	}
	if strings.Contains(e.Detail, "Robin") {
		t.Errorf("detail repeats the text: %q", e.Detail)
	}
}

func TestCheckCard_DefaultsAndPositions(t *testing.T) {
	later := cardNow.Add(2 * time.Hour)
	in := []domain.ComponentInput{
		comp("approve", "button", ` { "value" : "once", "label" : "Allow once" } `),
		{CustomID: "always", Type: "button", Definition: json.RawMessage(`{"label":"Always allow","value":"always"}`), RequiredCapability: domain.CapabilityApproveActions},
		{CustomID: "pick", Type: "select", Definition: json.RawMessage(`{"placeholder":"","options":[{"label":"A","value":"a","description":"First"}]}`), SingleUse: boolRef(false), RequiredCapability: "upload", RestrictedTo: domain.RestrictOwner, ExpiresAt: &later},
	}
	got, canonical, err := domain.CheckCard(in, cardNow, cardLimits)
	if err != nil {
		t.Fatalf("CheckCard: %v", err)
	}
	if len(canonical) == 0 || len(got) != 3 {
		t.Fatalf("got %d components and %d bytes", len(got), len(canonical))
	}
	ttl := cardNow.Add(cardLimits.DefaultTTL)
	want := []domain.CheckedComponent{
		{CustomID: "approve", Type: "button", Definition: json.RawMessage(`{"label":"Allow once","style":"secondary","value":"once"}`), Position: 1, SingleUse: true, RestrictedTo: domain.RestrictAny, ExpiresAt: ttl},
		{CustomID: "always", Type: "button", Definition: json.RawMessage(`{"label":"Always allow","style":"secondary","value":"always"}`), Position: 2, SingleUse: true, RequiredCapability: domain.CapabilityApproveActions, RestrictedTo: domain.RestrictRunStarter, ExpiresAt: ttl},
		{CustomID: "pick", Type: "select", Definition: json.RawMessage(`{"options":[{"label":"A","value":"a","description":"First"}],"min_values":1,"max_values":1}`), Position: 3, SingleUse: false, RequiredCapability: "upload", RestrictedTo: domain.RestrictOwner, ExpiresAt: later},
	}
	for i := range want {
		g, w := got[i], want[i]
		if g.CustomID != w.CustomID || g.Type != w.Type || g.Position != w.Position || g.SingleUse != w.SingleUse ||
			g.RequiredCapability != w.RequiredCapability || g.RestrictedTo != w.RestrictedTo || !g.ExpiresAt.Equal(w.ExpiresAt) {
			t.Errorf("component %d = %+v, want %+v", i, g, w)
		}
		if string(g.Definition) != string(w.Definition) {
			t.Errorf("component %d definition = %s, want %s", i, g.Definition, w.Definition)
		}
	}
}

func TestCheckCard_FormDefaults(t *testing.T) {
	in := []domain.ComponentInput{comp("f", "form", `{"fields":[{"type":"choice","choices":["Red","Blue"],"label":"Colour","name":"colour"},{"name":"note","label":"Note","type":"textarea","required":true,"max_length":200}],"title":"Order","submit_label":"Send"}`)}
	got, _, err := domain.CheckCard(in, cardNow, cardLimits)
	if err != nil {
		t.Fatalf("CheckCard: %v", err)
	}
	want := `{"title":"Order","submit_label":"Send","fields":[{"name":"colour","label":"Colour","type":"choice","required":false,"choices":["Red","Blue"]},{"name":"note","label":"Note","type":"textarea","required":true,"max_length":200}]}`
	if string(got[0].Definition) != want {
		t.Fatalf("definition = %s, want %s", got[0].Definition, want)
	}
}

func TestCheckCard_ButtonWithIcon(t *testing.T) {
	in := []domain.ComponentInput{comp("w", "button", `{"label":"Careful","value":"go","style":"danger","icon":"state.warning"}`)}
	got, _, err := domain.CheckCard(in, cardNow, cardLimits)
	if err != nil {
		t.Fatalf("CheckCard: %v", err)
	}
	if want := `{"label":"Careful","style":"danger","icon":"state.warning","value":"go"}`; string(got[0].Definition) != want {
		t.Fatalf("definition = %s, want %s", got[0].Definition, want)
	}
}

func TestCheckCard_CustomIDCountsCodePoints(t *testing.T) {
	// Exactly component.custom_id_max_chars code points but more bytes: accepted.
	id := strings.Repeat("ü", cardLimits.CustomIDMaxChars)
	if _, _, err := domain.CheckCard([]domain.ComponentInput{comp(id, "button", okButton)}, cardNow, cardLimits); err != nil {
		t.Fatalf("CheckCard: %v", err)
	}
}

func TestCheckCard_InvalidLimits(t *testing.T) {
	for _, lim := range []domain.CardLimits{
		{CustomIDMaxChars: 1, FormMaxFields: 1, DefaultTTL: time.Hour},
		{MaxPerMessage: 1, FormMaxFields: 1, DefaultTTL: time.Hour},
		{MaxPerMessage: 1, CustomIDMaxChars: 1, DefaultTTL: time.Hour},
		{MaxPerMessage: 1, CustomIDMaxChars: 1, FormMaxFields: 1},
	} {
		_, _, err := domain.CheckCard([]domain.ComponentInput{comp("a", "button", okButton)}, cardNow, lim)
		if errs.KindOf(err) != errs.KindInternal {
			t.Errorf("limits %+v: error = %v, want internal", lim, err)
		}
	}
}

func canonicalOf(t *testing.T, in ...domain.ComponentInput) []byte {
	t.Helper()
	_, b, err := domain.CheckCard(in, cardNow, cardLimits)
	if err != nil {
		t.Fatalf("CheckCard: %v", err)
	}
	return b
}

func TestCanonicalCard(t *testing.T) {
	later := time.Date(2026, 10, 22, 8, 0, 0, 123456789, time.FixedZone("CEST", 2*60*60))
	base := func() []domain.ComponentInput {
		return []domain.ComponentInput{
			comp("approve", "button", `{"label":"Allow once","value":"once"}`),
			{CustomID: "pick", Type: "select", Definition: json.RawMessage(okSelect), SingleUse: boolRef(true), RequiredCapability: "upload", RestrictedTo: domain.RestrictAny, ExpiresAt: &later},
			comp("form", "form", okForm),
		}
	}
	ref := canonicalOf(t, base()...)

	t.Run("key order and white space give the same bytes", func(t *testing.T) {
		in := base()
		in[0].Definition = json.RawMessage("{\n  \"value\": \"once\",\n  \"label\": \"Allow once\"\n}")
		in[1].Definition = json.RawMessage(`{ "options" : [ { "value":"s", "label":"Small" }, {"value":"l","label":"Large"} ] }`)
		in[2].Definition = json.RawMessage(`{"fields":[{"type":"text","label":"Note","name":"note"}],"title":"Contact"}`)
		if got := canonicalOf(t, in...); !bytes.Equal(got, ref) {
			t.Fatalf("canonical bytes differ")
		}
	})

	t.Run("the same instant in another zone gives the same bytes", func(t *testing.T) {
		in := base()
		utc := later.UTC()
		in[1].ExpiresAt = &utc
		if got := canonicalOf(t, in...); !bytes.Equal(got, ref) {
			t.Fatalf("canonical bytes differ")
		}
	})

	other := later.Add(time.Second)
	changes := map[string]func(in []domain.ComponentInput){
		"custom_id": func(in []domain.ComponentInput) { in[0].CustomID = "approve2" },
		"type":      func(in []domain.ComponentInput) { in[2] = comp("form", "select", okSelect) },
		"label": func(in []domain.ComponentInput) {
			in[0].Definition = json.RawMessage(`{"label":"Allow twice","value":"once"}`)
		},
		"value": func(in []domain.ComponentInput) {
			in[0].Definition = json.RawMessage(`{"label":"Allow once","value":"deny"}`)
		},
		"option order": func(in []domain.ComponentInput) {
			in[1].Definition = json.RawMessage(`{"options":[{"label":"Large","value":"l"},{"label":"Small","value":"s"}]}`)
		},
		"field label": func(in []domain.ComponentInput) {
			in[2].Definition = json.RawMessage(`{"title":"Contact","fields":[{"name":"note","label":"Remark","type":"text"}]}`)
		},
		"single_use given":    func(in []domain.ComponentInput) { in[0].SingleUse = boolRef(true) },
		"single_use false":    func(in []domain.ComponentInput) { in[1].SingleUse = boolRef(false) },
		"required_capability": func(in []domain.ComponentInput) { in[1].RequiredCapability = "export" },
		"restricted_to":       func(in []domain.ComponentInput) { in[1].RestrictedTo = domain.RestrictOwner },
		"expires_at":          func(in []domain.ComponentInput) { in[1].ExpiresAt = &other },
		"expires_at absent":   func(in []domain.ComponentInput) { in[1].ExpiresAt = nil },
		"component order":     func(in []domain.ComponentInput) { in[0], in[2] = in[2], in[0] },
		"component removed":   func(in []domain.ComponentInput) { copy(in[1:], in[2:]); in[2] = comp("x", "button", okButton) },
	}
	for name, change := range changes {
		t.Run("change of "+name+" gives different bytes", func(t *testing.T) {
			in := base()
			change(in)
			if got := canonicalOf(t, in...); bytes.Equal(got, ref) {
				t.Fatalf("canonical bytes did not change")
			}
		})
	}

	t.Run("defaults are not applied", func(t *testing.T) {
		pairs := [][2]domain.ComponentInput{
			{comp("a", "button", okButton), comp("a", "button", `{"label":"Allow once","value":"once","style":"secondary"}`)},
			{comp("a", "select", okSelect), comp("a", "select", `{"options":[{"label":"Small","value":"s"},{"label":"Large","value":"l"}],"min_values":1,"max_values":1}`)},
			{comp("a", "form", okForm), comp("a", "form", `{"title":"Contact","fields":[{"name":"note","label":"Note","type":"text","required":false}]}`)},
			{comp("a", "button", okButton), {CustomID: "a", Type: "button", Definition: json.RawMessage(okButton), SingleUse: boolRef(true)}},
			{comp("a", "button", okButton), {CustomID: "a", Type: "button", Definition: json.RawMessage(okButton), RestrictedTo: domain.RestrictAny}},
		}
		for i, p := range pairs {
			if bytes.Equal(canonicalOf(t, p[0]), canonicalOf(t, p[1])) {
				t.Errorf("pair %d: an explicit default gives the same bytes as its absence", i)
			}
		}
	})

	t.Run("the layout is length prefixed", func(t *testing.T) {
		got := canonicalOf(t, comp("a", "button", okButton))
		var want []byte
		for _, f := range []string{"a", "button", `{"label":"Allow once","value":"once"}`, "", "", "", ""} {
			want = binary.AppendUvarint(want, uint64(len(f)))
			want = append(want, f...)
		}
		if !bytes.Equal(got, want) {
			t.Fatalf("canonical = %q, want %q", got, want)
		}
		withAll := canonicalOf(t, domain.ComponentInput{CustomID: "a", Type: "button", Definition: json.RawMessage(okButton), SingleUse: boolRef(false), RequiredCapability: "manage", RestrictedTo: domain.RestrictOwner, ExpiresAt: &later})
		for _, part := range []string{"\x05false", "\x06manage", "\x05owner", "\x182026-10-22T06:00:00.123Z"} {
			if !bytes.Contains(withAll, []byte(part)) {
				t.Errorf("canonical %q lacks %q", withAll, part)
			}
		}
	})
}

func TestCheckDefinition(t *testing.T) {
	t.Run("normalizes and is idempotent", func(t *testing.T) {
		for typ, def := range map[string]string{"button": okButton, "select": okSelect, "form": okForm} {
			norm, err := domain.CheckDefinition(typ, json.RawMessage(def), cardLimits)
			if err != nil {
				t.Fatalf("%s: %v", typ, err)
			}
			again, err := domain.CheckDefinition(typ, norm, cardLimits)
			if err != nil || !bytes.Equal(again, norm) {
				t.Fatalf("%s: second pass = %s, %v; want %s", typ, again, err, norm)
			}
		}
	})
	t.Run("pointers below /definition", func(t *testing.T) {
		_, err := domain.CheckDefinition("button", json.RawMessage(`{"label":"Go","value":"go","style":"pink"}`), cardLimits)
		wantFinding(t, err, "/definition/style", domain.FieldInvalid)
		_, err = domain.CheckDefinition("form", json.RawMessage(`{"title":"T","fields":[{"name":"a","label":"A","type":"text"},{"name":"b","label":"B","type":"text"},{"name":"c","label":"C","type":"text"}]}`), cardLimits)
		wantFinding(t, err, "/definition/fields", domain.FieldLimitReached)
		_, err = domain.CheckDefinition("select", nil, cardLimits)
		wantFinding(t, err, "/definition", domain.FieldRequired)
		_, err = domain.CheckDefinition("button", json.RawMessage(`{"label":"Go","value":"go","required_capability":"manage"}`), cardLimits)
		wantFinding(t, err, "/definition", domain.FieldInvalid)
	})
	t.Run("a type outside the three is internal", func(t *testing.T) {
		_, err := domain.CheckDefinition("slider", json.RawMessage(okButton), cardLimits)
		if errs.KindOf(err) != errs.KindInternal {
			t.Fatalf("error = %v, want internal", err)
		}
	})
}

func TestSameDefinition(t *testing.T) {
	a := json.RawMessage(`{"label":"Go","style":"secondary","value":"go"}`)
	cases := []struct {
		b    string
		same bool
	}{
		{`{"value":"go","label":"Go","style":"secondary"}`, true},
		{" {\"label\" : \"Go\",\n \"style\":\"secondary\", \"value\":\"go\"} ", true},
		{`{"label":"Go","style":"primary","value":"go"}`, false},
		{`{"label":"Go","value":"go"}`, false},
		{`[]`, false},
		{`not json`, false},
	}
	for i, tc := range cases {
		if got := domain.SameDefinition(a, json.RawMessage(tc.b)); got != tc.same {
			t.Errorf("case %d: SameDefinition = %v, want %v", i, got, tc.same)
		}
	}
	if !domain.SameDefinition(json.RawMessage(`{"min_values":1}`), json.RawMessage(`{"min_values":1.0}`)) {
		t.Errorf("equal numbers in another notation are not the same")
	}
}

func TestCheckCard_ErrorIsValidationFailed(t *testing.T) {
	_, _, err := domain.CheckCard(nil, cardNow, cardLimits)
	var e *errs.Error
	if !errors.As(err, &e) || e.Kind() != errs.KindBadRequest {
		t.Fatalf("error = %v, want a bad request", err)
	}
}

// manyFields builds a form definition with n text fields.
func manyFields(n int) string {
	var b strings.Builder
	b.WriteString(`{"title":"T","fields":[`)
	for i := range n {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteString(`{"name":"f` + strconv.Itoa(i) + `","label":"L","type":"text"}`)
	}
	b.WriteString(`]}`)
	return b.String()
}

func TestCheckCard_FormAtTheLimit(t *testing.T) {
	if _, _, err := domain.CheckCard([]domain.ComponentInput{comp("f", "form", manyFields(cardLimits.FormMaxFields))}, cardNow, cardLimits); err != nil {
		t.Fatalf("CheckCard at the limit: %v", err)
	}
}
