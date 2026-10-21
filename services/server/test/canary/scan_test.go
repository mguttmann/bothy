// SPDX-License-Identifier: Apache-2.0

package canary

import (
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strings"
	"testing"
)

// scanOne scans one record of sink with text at location and returns the findings.
func scanOne(reg *Registry, sink Sink, viewer, location, text string, rules AllowRules) []Finding {
	s := NewSinks()
	s.Add(Record{Sink: sink, Viewer: viewer, Fields: []Field{{Location: location, Text: text}}})
	return Scan(reg, s, rules)
}

func TestSEC_T10_ScanFindsPlainCaseInsensitive(t *testing.T) {
	reg := NewRegistry()
	v := reg.New(ClassContent, "", "message")
	if !strings.HasPrefix(v, "cnry") || len(v) != 24 || strings.ToLower(v) != v {
		t.Fatalf("New returned %q, want cnry plus 20 lowercase hex characters", v)
	}
	for _, text := range []string{
		`{"msg":"x","detail":"` + v + `"}`,
		"prefix " + strings.ToUpper(v) + " suffix",
		"/path%2F" + v + "?q=1",
	} {
		got := scanOne(reg, SinkLog, "", "/detail", text, AllowRules{})
		if len(got) != 1 || got[0].Sink != SinkLog || got[0].Class != string(ClassContent) || got[0].Label != "message" {
			t.Errorf("Scan(%q) = %v, want one content finding in the log sink", text, got)
		}
	}
	if got := scanOne(reg, SinkLog, "", "/x", "nothing to see", AllowRules{}); len(got) != 0 {
		t.Errorf("Scan of a clean text = %v, want none", got)
	}
}

func TestSEC_T10_ScanFindsBase64AllAlignments(t *testing.T) {
	reg := NewRegistry()
	v := reg.New(ClassSecret, "", "token")
	encoders := map[string]*base64.Encoding{
		"std": base64.StdEncoding, "raw-std": base64.RawStdEncoding, "url": base64.URLEncoding, "raw-url": base64.RawURLEncoding,
	}
	for name, enc := range encoders {
		for _, prefix := range []string{"", "a", "ab"} {
			text := enc.EncodeToString([]byte(prefix + v + "trailer"))
			if got := scanOne(reg, SinkTrace, "", "/attr", text, AllowRules{}); len(got) != 1 {
				t.Errorf("%s with prefix %q: %d findings, want 1", name, prefix, len(got))
			}
		}
		if got := scanOne(reg, SinkTrace, "", "/attr", enc.EncodeToString([]byte(v)), AllowRules{}); len(got) != 1 {
			t.Errorf("%s of the bare value: %d findings, want 1", name, len(got))
		}
	}
}

func TestSEC_T10_ScanFindsHex(t *testing.T) {
	reg := NewRegistry()
	v := reg.New(ClassSecret, "", "pepper")
	for _, text := range []string{hex.EncodeToString([]byte(v)), strings.ToUpper(hex.EncodeToString([]byte("x" + v)))} {
		if got := scanOne(reg, SinkMetric, "", "/attr", text, AllowRules{}); len(got) != 1 {
			t.Errorf("Scan(%q) = %v, want one finding", text, got)
		}
	}
}

func TestSEC_T10_ScanSkipsJWSHeaderSegment(t *testing.T) {
	reg := NewRegistry()
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"ES256","typ":"at+jwt","kid":"k1"}`))
	payload := base64.RawURLEncoding.EncodeToString([]byte(`{"sub":"principal","jti":"0123456789abcdef"}`))
	sig := base64.RawURLEncoding.EncodeToString([]byte("signature-bytes-of-sixty-four-bytes-0123456789-0123456789-abcd"))
	reg.Register(header+"."+payload+"."+sig, ClassSecret, "", "access token")
	if got := scanOne(reg, SinkLog, "", "/h", "header only "+header, AllowRules{}); len(got) != 0 {
		t.Errorf("the header segment alone gave %v, want no finding", got)
	}
	for _, text := range []string{"payload " + payload, "sig " + sig} {
		if got := scanOne(reg, SinkLog, "", "/h", text, AllowRules{}); len(got) != 1 {
			t.Errorf("Scan(%q) = %v, want one finding", text, got)
		}
	}
}

func TestSEC_T10_ScanFindsIPv6Forms(t *testing.T) {
	reg := NewRegistry()
	reg.Register("2001:db8:0:0:1:0:0:2a", ClassNetwork, "", "ipv6")
	reg.Register("198.51.100.77", ClassNetwork, "", "ipv4")
	for _, text := range []string{
		"addr=2001:db8::1:0:0:2a",
		"addr=2001:0db8:0000:0000:0001:0000:0000:002a",
		"addr=2001:DB8::1:0:0:2A",
		"peer 198.51.100.77:443",
	} {
		if got := scanOne(reg, SinkLog, "", "/addr", text, AllowRules{}); len(got) != 1 {
			t.Errorf("Scan(%q) = %v, want one finding", text, got)
		}
	}
	if got := scanOne(reg, SinkLog, "", "/addr", "peer 198.51.100.7", AllowRules{}); len(got) != 0 {
		t.Errorf("a different address gave %v, want none", got)
	}
}

func TestSEC_T10_ScanFindsIDForms(t *testing.T) {
	reg := NewRegistry()
	id := "0199a3b4-5c6d-7e8f-9a0b-1c2d3e4f5a6b"
	reg.Register(id, ClassIdentity, "", "principal alex")
	for _, text := range []string{id, strings.ToUpper(id), strings.ReplaceAll(id, "-", "")} {
		if got := scanOne(reg, SinkLog, "", "/id", text, AllowRules{}); len(got) != 1 {
			t.Errorf("Scan(%q) = %v, want one finding", text, got)
		}
	}
}

func TestSEC_T10_ScanAllowMatrix(t *testing.T) {
	reg := NewRegistry()
	scoped := reg.New(ClassContent, "chan-support", "scoped content")
	open := reg.New(ClassContent, "", "display name")
	reason := reg.New(ClassAdminReason, "", "grant reason")
	secret := reg.New(ClassSecret, "", "token")
	reg.Register("198.51.100.23", ClassNetwork, "", "ipv4")
	reg.Register("0199a3b4-5c6d-7e8f-9a0b-1c2d3e4f5a6b", ClassIdentity, "", "principal")
	rules := AllowRules{Readable: map[string]map[string]bool{"Sam": {"chan-marketing": true}, "Robin": {"chan-support": true}}}
	cases := []struct {
		value, sink, viewer, location string
		allowed                       bool
	}{
		{scoped, SinkLog, "", "/msg", false},
		{scoped, SinkTrace, "", "/attr", false},
		{scoped, SinkMetric, "", "/attr", false},
		{scoped, SinkAudit, "", "/details/reason", false},
		{scoped, SinkError, "", "/body/detail", false},
		{scoped, SinkForeign, "Sam", "/d/payload", false},
		{scoped, SinkForeign, "Robin", "/d/payload", true},
		{open, SinkForeign, "Sam", "/d/payload", true},
		{open, SinkError, "", "/body/detail", false},
		{open, SinkAudit, "", "/details/name", false},
		{reason, SinkAudit, "", "/details/reason", true},
		{reason, SinkAudit, "", "/details/note", false},
		{reason, SinkLog, "", "/msg", false},
		{reason, SinkError, "", "/body/detail", false},
		{secret, SinkForeign, "Sam", "/d", false},
		{secret, SinkAudit, "", "/details/reason", false},
		{secret, SinkError, "", "/header/WWW-Authenticate", false},
		{"198.51.100.23", SinkAudit, "", "/details", false},
		{"198.51.100.23", SinkForeign, "Robin", "/d", false},
		{"198.51.100.23", SinkLog, "", "/msg", false},
		{"0199a3b4-5c6d-7e8f-9a0b-1c2d3e4f5a6b", SinkLog, "", "/attr", false},
		{"0199a3b4-5c6d-7e8f-9a0b-1c2d3e4f5a6b", SinkTrace, "", "/attr", false},
		{"0199a3b4-5c6d-7e8f-9a0b-1c2d3e4f5a6b", SinkMetric, "", "/attr", false},
		{"0199a3b4-5c6d-7e8f-9a0b-1c2d3e4f5a6b", SinkAudit, "", "/actor_id", true},
		{"0199a3b4-5c6d-7e8f-9a0b-1c2d3e4f5a6b", SinkError, "", "/body/instance", true},
		{"0199a3b4-5c6d-7e8f-9a0b-1c2d3e4f5a6b", SinkForeign, "Sam", "/d", true},
	}
	for _, c := range cases {
		got := scanOne(reg, c.sink, c.viewer, c.location, "x "+c.value+" y", rules)
		if c.allowed != (len(got) == 0) {
			t.Errorf("%s in %s (%s) at %s: findings %v, allowed=%v", reg.labelOf(c.value), c.sink, c.viewer, c.location, got, c.allowed)
		}
	}
}

func TestSEC_T10_FindingNeverPrintsValue(t *testing.T) {
	reg := NewRegistry()
	v := reg.New(ClassSecret, "", "refresh token")
	got := scanOne(reg, SinkError, "", "/body/detail", "leaked "+v, AllowRules{})
	if len(got) != 1 {
		t.Fatalf("findings %v, want 1", got)
	}
	if got[0].Prefix != v[:8] {
		t.Errorf("Prefix = %q, want the first eight characters", got[0].Prefix)
	}
	for _, text := range []string{fmt.Sprint(got), fmt.Sprintf("%+v", got[0]), fmt.Sprintf("%#v", got[0]), Report(got)} {
		if strings.Contains(text, v) || strings.Contains(text, v[8:]) {
			t.Errorf("printed finding %q contains the value", text)
		}
	}
}

// TestSEC_T10_ScanEchoFields: content in an echo field of an error response (current of a
// version conflict, the pointer of unknown_member) counts as a success response; a secret there
// is still a finding, and markEcho marks only those fields.
func TestSEC_T10_ScanEchoFields(t *testing.T) {
	reg := NewRegistry()
	content := reg.New(ClassContent, "chan", "message text")
	secret := reg.New(ClassSecret, "", "token")
	body := []byte(`{"code":"version_conflict","detail":"x","current":{"content_md":"` + content + `","note":"` + secret + `"},` +
		`"errors":[{"pointer":"/` + content + `","code":"unknown_member"},{"pointer":"/name","code":"too_long","detail":"` + content + `"}]}`)
	s := NewSinks()
	s.Add(Record{Sink: SinkError, Source: "probe", Fields: markEcho(JSONFields("/body", body), body)})
	got := Scan(reg, s, AllowRules{})
	if len(got) != 2 {
		t.Fatalf("findings:\n%s\nwant the secret in current and the content in errors/1/detail", Report(got))
	}
	for _, f := range got {
		if f.Location != "probe /body/current/note" && f.Location != "probe /body/errors/1/detail" {
			t.Errorf("unexpected finding %s", f)
		}
	}
}

// TestSEC_T10_JSONFieldsKeepsTrailingText: a line that starts with a JSON value but goes on (a
// standard logger line with its date prefix) is scanned as the whole text, never cut to its first
// value.
func TestSEC_T10_JSONFieldsKeepsTrailingText(t *testing.T) {
	reg := NewRegistry()
	v := reg.New(ClassSecret, "", "token")
	for _, line := range []string{"2026/10/18 12:00:00 leaked " + v, "true " + v, `{"a":1} ` + v} {
		s := NewSinks()
		s.AddJSON(SinkLog, "probe", "", "", []byte(line))
		if got := Scan(reg, s, AllowRules{}); len(got) != 1 {
			t.Errorf("%q: %d findings, want 1", line[:10], len(got))
		}
	}
}
