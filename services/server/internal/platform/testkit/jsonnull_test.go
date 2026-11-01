// SPDX-License-Identifier: Apache-2.0

package testkit_test

import (
	"fmt"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/platform/testkit"
)

// TestJSONNullIgnoresTheTextNullInsideAString holds the case of issue #238: a random base64url
// state held the letters null, and a substring check reported a JSON null that was not there.
func TestJSONNullIgnoresTheTextNullInsideAString(t *testing.T) {
	for _, doc := range []string{
		`{"authorization_url":"https://idp.example/authorize?state=o0d5U1VIIudCJv79Zc96lNYHoZN8nZuZnullGnQNyHk","login_attempt_id":"01a1957a-cc01-7437-8c6e-0de9555db231"}`,
		`{"null":"null","list":["null",{"nullable":"annulled"}]}`,
		`"null"`,
		`[]`,
		`{}`,
		`0`,
	} {
		ptr, found, err := testkit.JSONNull([]byte(doc))
		if err != nil || found || ptr != "" {
			t.Errorf("JSONNull(%s) = %q, %v, %v; want no null", doc, ptr, found, err)
		}
		if testkit.HasJSONNull([]byte(doc)) {
			t.Errorf("HasJSONNull(%s) = true", doc)
		}
	}
}

// TestJSONNullFindsANullAtAnyDepth names the JSON pointer (RFC 6901) of the first null.
func TestJSONNullFindsANullAtAnyDepth(t *testing.T) {
	for _, c := range []struct{ doc, want string }{
		{`null`, ""},
		{` null `, ""},
		{`{"a":null}`, "/a"},
		{`{"b":1,"a":{"c":[1,null]}}`, "/a/c/1"},
		{`[{"x":"y"},{"z":null}]`, "/1/z"},
		{`{"a/b":{"m~n":null}}`, "/a~1b/m~0n"},
		{`{"z":null,"a":null}`, "/a"},
		{`{"p":{"q":"null","r":null}}`, "/p/r"},
	} {
		doc, want := c.doc, c.want
		ptr, found, err := testkit.JSONNull([]byte(doc))
		if err != nil || !found || ptr != want {
			t.Errorf("JSONNull(%s) = %q, %v, %v; want %q, true, nil", doc, ptr, found, err, want)
		}
		if !testkit.HasJSONNull([]byte(doc)) {
			t.Errorf("HasJSONNull(%s) = false", doc)
		}
	}
}

// TestJSONNullReadsASequence checks every value of NDJSON; empty text holds no value.
func TestJSONNullReadsASequence(t *testing.T) {
	for doc, want := range map[string]string{
		"{\"a\":1}\n{\"b\":null}\n": "/b",
		`[1] [2,null]`:              "/1",
	} {
		if ptr, found, err := testkit.JSONNull([]byte(doc)); err != nil || !found || ptr != want {
			t.Errorf("JSONNull(%q) = %q, %v, %v; want %q", doc, ptr, found, err, want)
		}
	}
	for _, doc := range []string{``, " \n", "{\"a\":\"null\"}\n{\"b\":2}\n"} {
		if ptr, found, err := testkit.JSONNull([]byte(doc)); err != nil || found {
			t.Errorf("JSONNull(%q) = %q, %v, %v; want no null", doc, ptr, found, err)
		}
	}
}

// TestJSONNullReportsTextThatIsNoJSON: JSONNull returns the error, HasJSONNull finds no JSON
// null in it, and a null before the bad text is still found.
func TestJSONNullReportsTextThatIsNoJSON(t *testing.T) {
	for _, doc := range []string{`{`, `nul`, `404 page not found`, `{"a":1} x`} {
		if _, found, err := testkit.JSONNull([]byte(doc)); err == nil || found {
			t.Errorf("JSONNull(%q): found %v, err %v; want an error", doc, found, err)
		}
		if testkit.HasJSONNull([]byte(doc)) {
			t.Errorf("HasJSONNull(%q) = true", doc)
		}
	}
	if !testkit.HasJSONNull([]byte(`{"a":null} x`)) {
		t.Error("HasJSONNull missed the null before the bad text")
	}
}

// TestNoJSONNullFailsOnlyOnANull reports a null with its pointer and stays quiet on the text null.
func TestNoJSONNullFailsOnlyOnANull(t *testing.T) {
	r := &errRecorder{}
	testkit.NoJSONNull(r, "body", []byte(`{"state":"ZuZnullGnQ"}`))
	if r.msg != "" {
		t.Fatalf("NoJSONNull failed on the text null: %s", r.msg)
	}
	testkit.NoJSONNull(r, "body", []byte(`{"state":"x","expires_at":null}`))
	if r.msg != `body: JSON null at /expires_at in {"state":"x","expires_at":null}` {
		t.Fatalf("NoJSONNull message %q", r.msg)
	}
	r.msg = ""
	testkit.NoJSONNull(r, "body", []byte(`{`))
	if r.msg == "" {
		t.Fatal("NoJSONNull accepted a document that is no JSON")
	}
}

// errRecorder is a testing.TB that records Errorf.
type errRecorder struct {
	testing.TB
	msg string
}

func (r *errRecorder) Helper() {}

func (r *errRecorder) Errorf(format string, args ...any) { r.msg = fmt.Sprintf(format, args...) }
