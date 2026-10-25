// SPDX-License-Identifier: Apache-2.0

package guide

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMatchJSON(t *testing.T) {
	cases := []struct {
		name, expected, actual, want string
	}{
		{"equal scalars", `{"a": 1, "b": "x", "c": true}`, `{"a": 1, "b": "x", "c": true, "d": null}`, ""},
		{"missing member", `{"a": {"b": 1}}`, `{"a": {"c": 1}}`, "/a/b"},
		{"other type", `{"a": "1"}`, `{"a": 1}`, "/a"},
		{"nested", `{"a": {"b": {"c": "x"}}}`, `{"a": {"b": {"c": "y"}}}`, "/a/b/c"},
		{"array unique", `{"items": [{"name": "b"}]}`, `{"items": [{"name": "a"}, {"name": "b"}]}`, ""},
		{"array no match", `{"items": [{"name": "c"}]}`, `{"items": [{"name": "a"}, {"name": "b"}]}`, "/items/0"},
		{"array ambiguous", `{"items": [{"kind": "x"}]}`, `{"items": [{"kind": "x", "n": 1}, {"kind": "x", "n": 2}]}`, "ambiguous"},
		{"array distinct", `{"items": [{"kind": "x"}, {"kind": "x"}]}`, `{"items": [{"kind": "x"}]}`, "/items"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := NewBindings(nil, t.TempDir()).MatchJSON(c.expected, []byte(c.actual))
			if c.want == "" {
				if err != nil {
					t.Fatal(err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("err = %v, want one with %q", err, c.want)
			}
		})
	}

	t.Run("captures keep their JSON type", func(t *testing.T) {
		b := NewBindings(nil, t.TempDir())
		if err := b.MatchJSON(`{"limit": "{limit}", "id": "{id}"}`, []byte(`{"limit": 20, "id": "abc"}`)); err != nil {
			t.Fatal(err)
		}
		body, err := b.SubstituteJSON(`{"history_backfill_limit": "{limit}", "x": "id {id}"}`)
		if err != nil {
			t.Fatal(err)
		}
		if string(body) != `{"history_backfill_limit":20,"x":"id abc"}` {
			t.Fatalf("body = %s", body)
		}
	})
	t.Run("a bound placeholder compares", func(t *testing.T) {
		b := NewBindings(map[string]any{"id": "abc"}, t.TempDir())
		if err := b.MatchJSON(`{"id": "{id}"}`, []byte(`{"id": "abc"}`)); err != nil {
			t.Fatal(err)
		}
		if err := b.MatchJSON(`{"id": "{id}"}`, []byte(`{"id": "abd"}`)); err == nil || !strings.Contains(err.Error(), "/id") {
			t.Fatalf("err = %v, want a mismatch at /id", err)
		}
	})
	t.Run("captures in arrays bind only the unique match", func(t *testing.T) {
		b := NewBindings(nil, t.TempDir())
		expected := `{"items": [{"name": "summarize", "id": "{s}"}, {"name": "version", "id": "{v}"}]}`
		actual := `{"items": [{"name": "version", "id": "2"}, {"name": "summarize", "id": "1"}]}`
		if err := b.MatchJSON(expected, []byte(actual)); err != nil {
			t.Fatal(err)
		}
		if s, _ := b.Value("s"); s != "1" {
			t.Fatalf("s = %v", s)
		}
		if v, _ := b.Value("v"); v != "2" {
			t.Fatalf("v = %v", v)
		}
	})
}

// sliceLines feeds lines and then ends, as a process that exits.
func sliceLines(lines ...string) *Lines {
	l := NewLines()
	for _, s := range lines {
		l.Push(s)
	}
	l.Close(0)
	return l
}

func TestMatchLines(t *testing.T) {
	ctx := context.Background()
	t.Run("exact lines and captures", func(t *testing.T) {
		b := NewBindings(map[string]any{"run_id": "r1"}, t.TempDir())
		src := sliceLines("Step 8: started run r1", "Step 9: card c-17 sent")
		if err := b.MatchLines(ctx, []string{"Step 8: started run {run_id}", "Step 9: card {card} sent"}, src); err != nil {
			t.Fatal(err)
		}
		if v, _ := b.Value("card"); v != "c-17" {
			t.Fatalf("card = %v", v)
		}
		if err := src.Drained(); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("a differing line fails at once", func(t *testing.T) {
		b := NewBindings(nil, t.TempDir())
		err := b.MatchLines(ctx, []string{"Step 4: connected", "Step 5: x"}, sliceLines("Step 4: refused", "Step 5: x"))
		if err == nil || !strings.Contains(err.Error(), "Step 4: connected") || !strings.Contains(err.Error(), "Step 4: refused") {
			t.Fatalf("err = %v, want expected and actual line", err)
		}
	})
	t.Run("a process that ends early fails", func(t *testing.T) {
		b := NewBindings(nil, t.TempDir())
		err := b.MatchLines(ctx, []string{"Step 4: connected", "Step 5: x"}, sliceLines("Step 4: connected"))
		if err == nil || !strings.Contains(err.Error(), "ended before") {
			t.Fatalf("err = %v", err)
		}
	})
	t.Run("extra lines fail at the end", func(t *testing.T) {
		b := NewBindings(nil, t.TempDir())
		src := sliceLines("Step 4: connected", "Step 4: surprise")
		if err := b.MatchLines(ctx, []string{"Step 4: connected"}, src); err != nil {
			t.Fatal(err)
		}
		if err := src.Drained(); err == nil || !strings.Contains(err.Error(), "Step 4: surprise") {
			t.Fatalf("err = %v, want the unexpected line", err)
		}
	})
	t.Run("a waiting read ends with the context", func(t *testing.T) {
		b := NewBindings(nil, t.TempDir())
		cctx, cancel := context.WithCancel(ctx)
		cancel()
		if err := b.MatchLines(cctx, []string{"Step 4: connected"}, NewLines()); err == nil {
			t.Fatal("no error after the context ended")
		}
	})
}

func TestSecretsNeverInErrors(t *testing.T) {
	const secret = "s3cr3t-value-that-must-not-appear"
	dir := t.TempDir()
	b := NewBindings(nil, dir)
	err := b.MatchJSON(`{"secret": "{secret:agent.token}", "token": {"name": "other"}}`,
		[]byte(`{"secret": "`+secret+`", "token": {"name": "adapter"}}`))
	if err == nil {
		t.Fatal("the mismatch was not found")
	}
	if strings.Contains(err.Error(), secret) {
		t.Fatalf("the error shows the secret: %v", err)
	}
	if got := b.Redact("token " + secret + " in a log"); strings.Contains(got, secret) {
		t.Fatalf("Redact kept the secret: %q", got)
	}
	// The secret is captured before the mismatch is found; its file is private.
	info, err := os.Stat(filepath.Join(dir, "agent.token"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("mode = %v, want 0600", info.Mode().Perm())
	}
	if raw, _ := os.ReadFile(filepath.Join(dir, "agent.token")); string(raw) != secret { // #nosec G304 -- the test's own temporary file
		t.Fatal("the secret file does not hold the secret")
	}
	if got := b.Secrets(); len(got) != 1 || got[0] != secret {
		t.Fatalf("Secrets() = %d values", len(got))
	}
	// A secret in an actual value that differs is redacted too.
	b2 := NewBindings(nil, t.TempDir())
	err = b2.MatchJSON(`{"secret": "{secret:agent.token}", "copy": "x"}`, []byte(`{"secret": "`+secret+`", "copy": "`+secret+`"}`))
	if err == nil || strings.Contains(err.Error(), secret) {
		t.Fatalf("err = %v", err)
	}
}
