// SPDX-License-Identifier: Apache-2.0

package domain_test

// Tests of the cuts of agent output: the excerpt keeps its tail, a detail fits its byte limit.

import (
	"encoding/json"
	"math/rand/v2"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/OWNER/bothy/services/server/internal/runs/domain"
)

// TestCutExcerptKeepsTail checks that the excerpt keeps its last runes, because a draft grows at
// the end.
func TestCutExcerptKeepsTail(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in   string
		max  int
		want string
	}{
		{"abcdef", 3, "def"},
		{"äöü🙂x", 3, "ü🙂x"},
		{"äöü", 3, "äöü"},
		{"ab", 3, "ab"},
		{"", 3, ""},
		{"abc", 0, ""},
		{"Hello Alex 🙂🙂", 2, "🙂🙂"},
	}
	for _, c := range cases {
		got := domain.CutExcerpt(c.in, c.max)
		if got != c.want {
			t.Errorf("CutExcerpt(%q, %d) = %q, want %q", c.in, c.max, got, c.want)
		}
		if want := min(utf8.RuneCountInString(c.in), max(c.max, 0)); utf8.RuneCountInString(got) != want {
			t.Errorf("CutExcerpt(%q, %d) kept %d runes, want %d", c.in, c.max, utf8.RuneCountInString(got), want)
		}
	}
}

// decodeObject decodes a cut detail and fails unless it is a JSON object.
func decodeObject(t *testing.T, b []byte) map[string]any {
	t.Helper()
	var m map[string]any
	if !json.Valid(b) || json.Unmarshal(b, &m) != nil || m == nil {
		t.Fatalf("not a JSON object: %s", b)
	}
	return m
}

// TestCutDetail checks the cut of a step detail: valid JSON object, at most maxBytes, the marker
// truncated, longest string first, deterministic, and the fallback {"truncated":true}.
func TestCutDetail(t *testing.T) {
	t.Parallel()
	t.Run("fits and is canonical", func(t *testing.T) {
		got := domain.CutDetail(json.RawMessage(`{ "b": 1, "a": "x<y" }`), 100)
		if string(got) != `{"a":"x<y","b":1}` {
			t.Errorf("got %s", got)
		}
	})
	t.Run("one long string", func(t *testing.T) {
		got := domain.CutDetail(json.RawMessage(`{"log":"`+strings.Repeat("x", 100)+`","n":1}`), 40)
		m := decodeObject(t, got)
		if len(got) > 40 || m["truncated"] != true || m["n"] != 1.0 || !strings.HasPrefix(strings.Repeat("x", 30), m["log"].(string)) {
			t.Errorf("got %s", got)
		}
		if len(got) != 40 {
			t.Errorf("got %d bytes, want exactly 40 for plain ASCII", len(got))
		}
	})
	t.Run("longest first", func(t *testing.T) {
		// Canonical 65 bytes, 82 with the marker: b loses 22 bytes, a stays.
		in := `{"a":"` + strings.Repeat("a", 10) + `","b":"` + strings.Repeat("b", 40) + `"}`
		got := domain.CutDetail(json.RawMessage(in), 60)
		m := decodeObject(t, got)
		if m["a"] != strings.Repeat("a", 10) || m["b"] != strings.Repeat("b", 18) || len(got) != 60 {
			t.Errorf("got %s (%d bytes)", got, len(got))
		}
	})
	t.Run("ties go to the first pointer", func(t *testing.T) {
		// Canonical 75 bytes, 92 with the marker: a (first pointer) loses 22 bytes, b stays.
		in := `{"b":"` + strings.Repeat("b", 30) + `","a":"` + strings.Repeat("a", 30) + `"}`
		got := domain.CutDetail(json.RawMessage(in), 70)
		m := decodeObject(t, got)
		if m["b"] != strings.Repeat("b", 30) || m["a"] != strings.Repeat("a", 8) {
			t.Errorf("got %s", got)
		}
	})
	t.Run("nested and multi-byte", func(t *testing.T) {
		in := `{"x":{"list":["` + strings.Repeat("🙂", 30) + `",1,true],"q":"` + strings.Repeat(`\"\n `, 20) + `"}}`
		full := len(domain.CutDetail(json.RawMessage(in), 1<<20))
		for limit := 18; limit < full; limit += 7 {
			got := domain.CutDetail(json.RawMessage(in), limit)
			m := decodeObject(t, got)
			if len(got) > limit {
				t.Fatalf("limit %d: %d bytes", limit, len(got))
			}
			if m["truncated"] != true {
				t.Errorf("limit %d: no marker in %s", limit, got)
			}
			if !utf8.Valid(got) {
				t.Errorf("limit %d: invalid UTF-8", limit)
			}
		}
	})
	t.Run("an existing truncated member is replaced", func(t *testing.T) {
		got := domain.CutDetail(json.RawMessage(`{"truncated":{"z":"`+strings.Repeat("z", 50)+`"},"a":"`+strings.Repeat("a", 50)+`"}`), 40)
		m := decodeObject(t, got)
		if m["truncated"] != true || len(got) > 40 {
			t.Errorf("got %s", got)
		}
	})
	t.Run("fallback", func(t *testing.T) {
		var b strings.Builder
		b.WriteString(`{`)
		for i := 0; i < 30; i++ {
			if i > 0 {
				b.WriteString(",")
			}
			b.WriteString(`"k` + strings.Repeat("k", i) + `":` + "1")
		}
		b.WriteString(`}`)
		if got := domain.CutDetail(json.RawMessage(b.String()), 40); string(got) != `{"truncated":true}` {
			t.Errorf("got %s", got)
		}
		if got := domain.CutDetail(json.RawMessage(`[1]`), 40); string(got) != `{"truncated":true}` {
			t.Errorf("non-object: got %s", got)
		}
		if got := domain.CutDetail(json.RawMessage(`{`), 40); string(got) != `{"truncated":true}` {
			t.Errorf("invalid JSON: got %s", got)
		}
	})
	t.Run("deterministic and bounded on random input", func(t *testing.T) {
		g := &gen{r: rand.New(rand.NewPCG(11, 13))} //nolint:gosec // a fixed seed makes the property test reproducible
		for i := 0; i < 3000; i++ {
			in := g.detail()
			limit := 18 + g.r.IntN(120)
			got := domain.CutDetail(json.RawMessage(in), limit)
			again := domain.CutDetail(json.RawMessage(in), limit)
			if string(got) != string(again) {
				t.Fatalf("not deterministic for %s", in)
			}
			m := decodeObject(t, got)
			if len(got) > limit {
				t.Fatalf("%d bytes over limit %d for %s", len(got), limit, in)
			}
			if len(in) > limit && m["truncated"] != true {
				canon := domain.CutDetail(json.RawMessage(in), 1<<20)
				if len(canon) > limit {
					t.Fatalf("cut without marker: %s", got)
				}
			}
			// The cut of a cut value with the same limit changes nothing.
			if third := domain.CutDetail(got, limit); string(third) != string(got) {
				t.Fatalf("cut is not stable: %s then %s", got, third)
			}
		}
	})
}

// TestCutDetailNumbersInPlainDecimal checks that numbers in a detail are stored in the plain
// decimal form PostgreSQL prints for a jsonb numeric, so the byte limit bounds what every read
// returns: a short literal like 1e-16000 expands to thousands of digits on output.
func TestCutDetailNumbersInPlainDecimal(t *testing.T) {
	t.Parallel()
	cases := []struct{ in, want string }{
		{`{"n":1e2}`, `{"n":100}`},
		{`{"n":1.50}`, `{"n":1.50}`},
		{`{"n":1.5e-3}`, `{"n":0.0015}`},
		{`{"n":-2.5E+1}`, `{"n":-25}`},
		{`{"n":0e-3}`, `{"n":0.000}`},
		{`{"n":-0}`, `{"n":0}`},
		{`{"n":12.345e1}`, `{"n":123.45}`},
		{`{"n":0.05e3}`, `{"n":50}`},
		{`{"n":[7,0.1]}`, `{"n":[7,0.1]}`},
	}
	for _, c := range cases {
		if got := domain.CutDetail(json.RawMessage(c.in), 1000); string(got) != c.want {
			t.Errorf("CutDetail(%s) = %s, want %s", c.in, got, c.want)
		}
	}
	// A literal whose plain form exceeds the limit cannot be cut: the fallback.
	for _, in := range []string{`{"n":1e-16000}`, `{"n":0e-16000}`, `{"n":1e131000}`, `{"n":1e999999999999999999}`, `{"s":"x","n":[1e-200,1e-200]}`} {
		if got := domain.CutDetail(json.RawMessage(in), 300); string(got) != `{"truncated":true}` {
			t.Errorf("CutDetail(%s) = %.80s, want the fallback", in, got)
		}
	}
	// Many expanding numbers below the canonical limit as sent stay within the limit as printed.
	in := `{"n":[` + strings.Repeat(`1e-50,`, 100) + `1]}`
	if got := domain.CutDetail(json.RawMessage(in), 1024); len(got) > 1024 {
		t.Errorf("%d bytes over the limit", len(got))
	}
}
