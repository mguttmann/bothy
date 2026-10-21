// SPDX-License-Identifier: Apache-2.0

package domain_test

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/runs/domain"
)

// valueLimits allow the larger forms of the values tests.
var valueLimits = domain.CardLimits{MaxPerMessage: 3, CustomIDMaxChars: 12, FormMaxFields: 8, DefaultTTL: time.Hour}

// normalized returns the stored form of a definition and fails the test when it is rejected.
func normalized(t *testing.T, typ, def string) json.RawMessage {
	t.Helper()
	norm, err := domain.CheckDefinition(typ, json.RawMessage(def), valueLimits)
	if err != nil {
		t.Fatalf("CheckDefinition(%s): %v", typ, err)
	}
	return norm
}

const (
	valButton = `{"label":"Allow once","value":"once"}`
	valSelect = `{"options":[{"label":"Small","value":"s"},{"label":"Medium","value":"m"},{"label":"Large","value":"l"}],"min_values":1,"max_values":2}`
	valOpen   = `{"options":[{"label":"Small","value":"s"}],"min_values":0,"max_values":1}`
	valForm   = `{"title":"Contact","fields":[` +
		`{"name":"subject","label":"Subject","type":"text","required":true,"max_length":5},` +
		`{"name":"body","label":"Body","type":"textarea"},` +
		`{"name":"count","label":"Count","type":"number"},` +
		`{"name":"urgent","label":"Urgent","type":"boolean"},` +
		`{"name":"colour","label":"Colour","type":"choice","choices":["Red","Blue"]}]}`
)

func TestCheckValues(t *testing.T) {
	cases := []struct {
		name    string
		typ     string
		def     string
		values  string
		pointer string
	}{
		{"button with values", "button", valButton, `{"value":"once"}`, "/values"},
		{"values not an object", "button", valButton, `["once"]`, "/values"},
		{"values with a duplicate key", "form", valForm, `{"subject":"Hi","subject":"Hey"}`, "/values"},
		{"select below min_values", "select", valSelect, `{"selected":[]}`, "/values/selected"},
		{"select above max_values", "select", valSelect, `{"selected":["s","m","l"]}`, "/values/selected"},
		{"select without selected", "select", valSelect, `{}`, "/values/selected"},
		{"select with selected null", "select", valOpen, `{"selected":null}`, "/values/selected"},
		{"select with a number entry", "select", valSelect, `{"selected":["s",1]}`, "/values/selected"},
		{"select with a null entry", "select", valSelect, `{"selected":["s",null]}`, "/values/selected"},
		{"select with an unknown member", "select", valSelect, `{"selected":["s"],"other":true}`, "/values"},
		{"unknown option", "select", valSelect, `{"selected":["xl"]}`, "/values/selected/0"},
		{"duplicate selection", "select", valSelect, `{"selected":["s","s"]}`, "/values/selected/1"},
		{"form with unknown key", "form", valForm, `{"subject":"Hi","password":"Alex"}`, "/values"},
		{"form value with NUL", "form", valForm, `{"subject":"Hi","body":"a\u0000b"}`, "/values"},
		{"form value with an unpaired surrogate", "form", valForm, `{"subject":"Hi","body":"a\ud800"}`, "/values"},
		{"missing required field", "form", valForm, `{"body":"Hello"}`, "/values/subject"},
		{"white-space-only required text", "form", valForm, `{"subject":" \t "}`, "/values/subject"},
		{"line break in text", "form", valForm, `{"subject":"a\nb"}`, "/values/subject"},
		{"carriage return in text", "form", valForm, `{"subject":"a\rb"}`, "/values/subject"},
		{"too long text", "form", valForm, `{"subject":"Hello Sam"}`, "/values/subject"},
		{"number for text", "form", valForm, `{"subject":42}`, "/values/subject"},
		{"null for text", "form", valForm, `{"subject":null}`, "/values/subject"},
		{"string for number", "form", valForm, `{"subject":"Hi","count":"12"}`, "/values/count"},
		{"number outside the range of a 64-bit float", "form", valForm, `{"subject":"Hi","count":1e400}`, "/values/count"},
		{"negative number outside the range", "form", valForm, `{"subject":"Hi","count":-1e400}`, "/values/count"},
		{"number for boolean", "form", valForm, `{"subject":"Hi","urgent":1}`, "/values/urgent"},
		{"string for boolean", "form", valForm, `{"subject":"Hi","urgent":"true"}`, "/values/urgent"},
		{"choice outside choices", "form", valForm, `{"subject":"Hi","colour":"Green"}`, "/values/colour"},
		{"choice of the wrong case", "form", valForm, `{"subject":"Hi","colour":"red"}`, "/values/colour"},
		{"choice as a number", "form", valForm, `{"subject":"Hi","colour":1}`, "/values/colour"},
		{"textarea as a boolean", "form", valForm, `{"subject":"Hi","body":true}`, "/values/body"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := domain.CheckValues(tc.typ, normalized(t, tc.typ, tc.def), json.RawMessage(tc.values))
			fs := fieldsOf(t, err)
			if len(fs) != 1 || fs[0].Pointer != tc.pointer || fs[0].Code != domain.FieldValuesInvalid {
				t.Fatalf("findings = %+v, want one %s at %s", fs, domain.FieldValuesInvalid, tc.pointer)
			}
			for _, v := range []string{"Alex", "Sam", "Green", "xl", "1e400", "12", "Hey"} {
				if strings.Contains(tc.values, v) && strings.Contains(fs[0].Detail, v) {
					t.Errorf("detail %q contains the value %q", fs[0].Detail, v)
				}
			}
		})
	}
}

func TestCheckValues_AllFindings(t *testing.T) {
	err := domain.CheckValues("form", normalized(t, "form", valForm), json.RawMessage(`{"other":1,"count":"x","urgent":"no"}`))
	fs := fieldsOf(t, err)
	want := []string{"/values", "/values/subject", "/values/count", "/values/urgent"}
	if len(fs) != len(want) {
		t.Fatalf("findings = %+v, want pointers %v", fs, want)
	}
	for i, p := range want {
		if fs[i].Pointer != p || fs[i].Code != domain.FieldValuesInvalid {
			t.Errorf("finding %d = %+v, want %s", i, fs[i], p)
		}
	}
}

func TestCheckValues_Accepts(t *testing.T) {
	cases := []struct {
		name   string
		typ    string
		def    string
		values string
	}{
		{"button with nil values", "button", valButton, ``},
		{"button with null", "button", valButton, `null`},
		{"button with empty object", "button", valButton, ` {} `},
		{"select within bounds", "select", valSelect, `{"selected":["l","s"]}`},
		{"select at min_values", "select", valSelect, `{"selected":["m"]}`},
		{"select with min_values zero and no selected", "select", valOpen, `{}`},
		{"select with min_values zero and an empty list", "select", valOpen, `{"selected":[]}`},
		{"form minimal", "form", valForm, `{"subject":"Hi"}`},
		{"form full", "form", valForm, `{"colour":"Blue","urgent":false,"count":-3.5e2,"body":"Line one\nLine two","subject":"Héllo"}`},
		{"form with max_length in code points", "form", valForm, `{"subject":"üüüüü"}`},
		{"form optional empty textarea", "form", valForm, `{"subject":"Hi","body":""}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := domain.CheckValues(tc.typ, normalized(t, tc.typ, tc.def), json.RawMessage(tc.values)); err != nil {
				t.Fatalf("CheckValues: %v", err)
			}
		})
	}
}

func TestCheckValues_BrokenDefinitionIsInternal(t *testing.T) {
	for _, tc := range []struct{ typ, def string }{
		{"button", `{"label":"Go"}`},
		{"select", `not json`},
		{"slider", valButton},
	} {
		err := domain.CheckValues(tc.typ, json.RawMessage(tc.def), nil)
		if errs.KindOf(err) != errs.KindInternal {
			t.Errorf("%s %s: error = %v, want internal", tc.typ, tc.def, err)
		}
	}
}

func TestSameValues(t *testing.T) {
	cases := []struct {
		a, b string
		same bool
	}{
		{`{"a":1,"b":"x"}`, `{"b":"x","a":1}`, true},
		{``, `{}`, true},
		{`null`, ``, true},
		{` { } `, `null`, true},
		{`{"selected":["a","b"]}`, `{"selected":["b","a"]}`, false},
		{`{"count":100}`, `{"count":1e2}`, true},
		{`{"count":1}`, `{"count":2}`, false},
		{`{"name":"A"}`, `{"name":"A"}`, true},
		{`{"a":true}`, `{"a":"true"}`, false},
		{`{"a":1}`, `{"a":1,"b":2}`, false},
		{`{}`, `[]`, false},
		{`{"a":1}`, `{"a":1} x`, false},
	}
	for i, tc := range cases {
		if got := domain.SameValues(json.RawMessage(tc.a), json.RawMessage(tc.b)); got != tc.same {
			t.Errorf("case %d: SameValues(%s, %s) = %v, want %v", i, tc.a, tc.b, got, tc.same)
		}
	}
}
