// SPDX-License-Identifier: Apache-2.0

package domain_test

import (
	"bytes"
	"encoding/json"
	"regexp"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/textcheck"
	"github.com/OWNER/bothy/services/server/internal/runs/domain"
)

var fuzzTypes = []string{domain.ComponentButton, domain.ComponentSelect, domain.ComponentForm, "slider"}

var fuzzFieldName = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

// stringsOf collects every string of a decoded JSON document, keys excluded.
func stringsOf(v any, out *[]string) {
	switch x := v.(type) {
	case string:
		*out = append(*out, x)
	case []any:
		for _, e := range x {
			stringsOf(e, out)
		}
	case map[string]any:
		for _, e := range x {
			stringsOf(e, out)
		}
	}
}

// checkAcceptedDefinition fails when an accepted, normalized definition breaks a rule of
// component.v1 that the fuzzer can observe from the outside.
func checkAcceptedDefinition(t *testing.T, typ string, norm json.RawMessage) {
	t.Helper()
	again, err := domain.CheckDefinition(typ, norm, cardLimits)
	if err != nil || !bytes.Equal(again, norm) {
		t.Fatalf("normalized %s definition %s does not pass again: %s, %v", typ, norm, again, err)
	}
	var doc map[string]any
	if err := json.Unmarshal(norm, &doc); err != nil {
		t.Fatalf("normalized definition does not decode: %v", err)
	}
	var texts []string
	stringsOf(doc, &texts)
	for _, s := range texts {
		if s == "" || textcheck.DisplayText(s) != nil {
			t.Fatalf("accepted definition %s has an empty or rejected text", norm)
		}
	}
	switch typ {
	case domain.ComponentSelect:
		var d struct {
			Options []struct{ Value string }
			Min     int `json:"min_values"`
			Max     int `json:"max_values"`
		}
		_ = json.Unmarshal(norm, &d)
		seen := map[string]bool{}
		for _, o := range d.Options {
			if seen[o.Value] {
				t.Fatalf("accepted select %s has a duplicate value", norm)
			}
			seen[o.Value] = true
		}
		if len(d.Options) == 0 || d.Min < 0 || d.Max < 1 || d.Min > d.Max || d.Max > len(d.Options) {
			t.Fatalf("accepted select %s breaks the bounds", norm)
		}
	case domain.ComponentForm:
		var d struct {
			Fields []struct {
				Name, Type string
				MaxLength  *int `json:"max_length"`
				Choices    []string
			}
		}
		_ = json.Unmarshal(norm, &d)
		if len(d.Fields) == 0 || len(d.Fields) > cardLimits.FormMaxFields {
			t.Fatalf("accepted form %s has a bad field count", norm)
		}
		seen := map[string]bool{}
		for _, f := range d.Fields {
			if !fuzzFieldName.MatchString(f.Name) || seen[f.Name] {
				t.Fatalf("accepted form %s has a bad or duplicate name", norm)
			}
			seen[f.Name] = true
			if f.MaxLength != nil && f.Type != "text" && f.Type != "textarea" {
				t.Fatalf("accepted form %s has max_length on a %s", norm, f.Type)
			}
			if (len(f.Choices) > 0) != (f.Type == "choice") {
				t.Fatalf("accepted form %s has choices on a %s or none on a choice", norm, f.Type)
			}
		}
	}
}

func FuzzCheckCard(f *testing.F) {
	f.Add(uint8(0), []byte(okButton), "approve", uint8(0), uint8(0), int64(0))
	f.Add(uint8(0), []byte(`{"label":"Go","value":"go","style":"danger","icon":"state.warning"}`), "go", uint8(1), uint8(1), int64(60))
	f.Add(uint8(1), []byte(okSelect), "pick", uint8(2), uint8(2), int64(-60))
	f.Add(uint8(1), []byte(`{"placeholder":"Pick","options":[{"label":"A","value":"a","description":"d"},{"label":"B","value":"b"}],"min_values":0,"max_values":2}`), "p", uint8(3), uint8(3), int64(3600))
	f.Add(uint8(2), []byte(okForm), "form", uint8(4), uint8(0), int64(0))
	f.Add(uint8(2), []byte(`{"title":"T","submit_label":"S","fields":[{"name":"a","label":"A","type":"choice","choices":["x","y"]},{"name":"b","label":"B","type":"text","required":true,"max_length":3}]}`), "f", uint8(0), uint8(0), int64(0))
	f.Add(uint8(3), []byte(`{}`), "", uint8(0), uint8(0), int64(0))
	f.Fuzz(func(t *testing.T, typSel uint8, def []byte, customID string, flags, enums uint8, expSec int64) {
		typ := fuzzTypes[int(typSel)%len(fuzzTypes)]
		in := domain.ComponentInput{CustomID: customID, Type: typ, Definition: def}
		if flags&1 != 0 {
			in.SingleUse = boolRef(flags&2 != 0)
		}
		caps := []string{"", domain.CapabilityApproveActions, "upload", "export", "manage", "terminal"}
		restr := []string{"", domain.RestrictAny, domain.RestrictRunStarter, domain.RestrictOwner, "nobody"}
		in.RequiredCapability = caps[int(enums)%len(caps)]
		in.RestrictedTo = restr[int(enums>>4)%len(restr)]
		if expSec != 0 {
			in.ExpiresAt = timeRef(cardNow.Add(time.Duration(expSec%1e6) * time.Second))
		}
		cards := [][]domain.ComponentInput{{in}, {in, comp("second", "button", okButton)}}
		for _, card := range cards {
			got, canonical, err := domain.CheckCard(card, cardNow, cardLimits)
			if err != nil {
				e, ok := errs.As(err)
				if !ok || e.Code != errs.CodeValidationFailed || len(e.Fields) == 0 {
					t.Fatalf("rejection is not validation_failed with findings: %v", err)
				}
				if got != nil || canonical != nil {
					t.Fatalf("rejection returned components")
				}
				continue
			}
			if len(got) != len(card) || len(canonical) == 0 {
				t.Fatalf("accepted card has %d components and %d bytes", len(got), len(canonical))
			}
			_, again, _ := domain.CheckCard(card, cardNow, cardLimits)
			if !bytes.Equal(again, canonical) {
				t.Fatalf("canonical bytes are not deterministic")
			}
			c := got[0]
			if textcheck.DisplayText(c.CustomID) != nil || c.CustomID == "" || utf8.RuneCountInString(c.CustomID) > cardLimits.CustomIDMaxChars {
				t.Fatalf("accepted custom_id breaks a rule")
			}
			if !c.ExpiresAt.After(cardNow) || c.Position != 1 {
				t.Fatalf("accepted component has expires_at %v and position %d", c.ExpiresAt, c.Position)
			}
			if c.RequiredCapability == "terminal" || (c.RestrictedTo != domain.RestrictAny && c.RestrictedTo != domain.RestrictRunStarter && c.RestrictedTo != domain.RestrictOwner) {
				t.Fatalf("accepted component has capability %q and restriction %q", c.RequiredCapability, c.RestrictedTo)
			}
			if in.RestrictedTo == "" && (c.RestrictedTo == domain.RestrictRunStarter) != (c.RequiredCapability == domain.CapabilityApproveActions) {
				t.Fatalf("default restriction %q for capability %q", c.RestrictedTo, c.RequiredCapability)
			}
			checkAcceptedDefinition(t, typ, c.Definition)
		}
	})
}

func FuzzCheckValues(f *testing.F) {
	f.Add(uint8(0), []byte(valButton), []byte(`{}`))
	f.Add(uint8(0), []byte(valButton), []byte(`{"a":1}`))
	f.Add(uint8(1), []byte(valSelect), []byte(`{"selected":["s","m"]}`))
	f.Add(uint8(1), []byte(valOpen), []byte(`{}`))
	f.Add(uint8(1), []byte(valSelect), []byte(`{"selected":["s","s","x"]}`))
	f.Add(uint8(2), []byte(valForm), []byte(`{"subject":"Hi","count":1e400,"urgent":true,"colour":"Red","body":"a\nb"}`))
	f.Add(uint8(2), []byte(valForm), []byte(`{"subject":" "}`))
	f.Fuzz(func(t *testing.T, typSel uint8, def, values []byte) {
		typ := fuzzTypes[int(typSel)%3]
		norm, err := domain.CheckDefinition(typ, def, valueLimits)
		if err != nil {
			return
		}
		err = domain.CheckValues(typ, norm, values)
		if err != nil {
			e, ok := errs.As(err)
			if !ok || e.Code != errs.CodeValidationFailed || len(e.Fields) == 0 {
				t.Fatalf("rejection is not validation_failed with findings: %v", err)
			}
			for _, fe := range e.Fields {
				if fe.Code != domain.FieldValuesInvalid || (fe.Pointer != "/values" && !bytes.HasPrefix([]byte(fe.Pointer), []byte("/values/"))) {
					t.Fatalf("finding %+v", fe)
				}
			}
			return
		}
		if !domain.SameValues(values, values) {
			t.Fatalf("accepted values are not equal to themselves")
		}
		var doc map[string]json.RawMessage
		if v := bytes.TrimSpace(values); len(v) > 0 && string(v) != "null" {
			if err := json.Unmarshal(v, &doc); err != nil {
				t.Fatalf("accepted values are not an object: %v", err)
			}
		}
		switch typ {
		case domain.ComponentButton:
			if len(doc) != 0 {
				t.Fatalf("accepted button values %s", values)
			}
		case domain.ComponentSelect:
			var d struct {
				Options []struct{ Value string }
				Min     int `json:"min_values"`
				Max     int `json:"max_values"`
			}
			_ = json.Unmarshal(norm, &d)
			var sel []string
			if raw, ok := doc["selected"]; ok {
				if err := json.Unmarshal(raw, &sel); err != nil {
					t.Fatalf("accepted selection is not a list of strings")
				}
			}
			if len(sel) < d.Min || len(sel) > d.Max {
				t.Fatalf("accepted selection of %d outside %d..%d", len(sel), d.Min, d.Max)
			}
			valid, seen := map[string]bool{}, map[string]bool{}
			for _, o := range d.Options {
				valid[o.Value] = true
			}
			for _, s := range sel {
				if !valid[s] || seen[s] {
					t.Fatalf("accepted selection has an unknown or duplicate entry")
				}
				seen[s] = true
			}
		case domain.ComponentForm:
			var d struct {
				Fields []struct {
					Name, Type string
					Required   bool
				}
			}
			_ = json.Unmarshal(norm, &d)
			names := map[string]bool{}
			for _, f := range d.Fields {
				names[f.Name] = true
				if _, ok := doc[f.Name]; f.Required && !ok {
					t.Fatalf("accepted form values lack the required field")
				}
			}
			for k := range doc {
				if !names[k] {
					t.Fatalf("accepted form values have an unknown key")
				}
			}
		}
	})
}
