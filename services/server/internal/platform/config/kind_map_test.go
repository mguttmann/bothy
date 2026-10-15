// SPDX-License-Identifier: Apache-2.0

package config

import "testing"

func TestKindStringMap(t *testing.T) {
	t.Parallel()
	if got := KindStringMap.String(); got != "map(string)" {
		t.Errorf("KindStringMap.String() = %q", got)
	}
	runParseCases(t, KindStringMap, nil, []parseCase{
		{in: `{"en":"english","de":"german"}`, ok: true, want: map[string]string{"de": "german", "en": "english"}, canon: `{"de":"german","en":"english"}`},
		{in: ` { "de" : "german" } `, ok: true, want: map[string]string{"de": "german"}, canon: `{"de":"german"}`},
		{in: `{}`, ok: true, want: map[string]string{}, canon: `{}`},
		{in: `{"x":"<&>"}`, ok: true, want: map[string]string{"x": "<&>"}, canon: `{"x":"<&>"}`},
		{in: `{"x":""}`, ok: true, want: map[string]string{"x": ""}, canon: `{"x":""}`},
		{in: `["german","english"]`},
		{in: `{"de":1}`},
		{in: `{"de":true}`},
		{in: `{"de":null}`},
		{in: `{"de":{"x":"y"}}`},
		{in: `{"de":["german"]}`},
		{in: `{"de":"german","de":"deutsch"}`},
		{in: `{"":"german"}`},
		{in: `{"de":"german"} trailing`},
		{in: `{"de":"german"}{}`},
		{in: `{"de":"german"`},
		{in: `"german"`},
		{in: `german`},
		{in: ``},
		{in: `null`},
	})
}

func TestKindStringMapErrorsNamePositions(t *testing.T) {
	t.Parallel()
	_, _, err := parseValue(KindStringMap, nil, `{"de":"german","secretkey":7}`)
	if err == nil || err.Error() != "entry 2: the value must be a string" {
		t.Errorf("error = %v", err)
	}
	_, _, err = parseValue(KindStringMap, nil, `{"de":"german","de":"secretvalue"}`)
	if err == nil || err.Error() != "entry 2 repeats the key of an earlier entry" {
		t.Errorf("error = %v", err)
	}
}
