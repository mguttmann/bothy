// SPDX-License-Identifier: Apache-2.0

package secrets_test

import (
	"bytes"
	"context"
	"encoding"
	"encoding/json"
	"fmt"
	"log/slog"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/platform/secrets"
)

const redacted = "[redacted]"

func TestValueNeverPrints(t *testing.T) {
	t.Parallel()
	canary := newCanary(t)
	v := secrets.NewValue([]byte(canary))

	// Renderings that exist must be exactly the placeholder.
	exact := map[string]string{}
	for _, verb := range []string{"%v", "%+v", "%#v", "%s", "%q", "%x", "%X", "%d", "%10s", "%-12v", "% x"} {
		exact["Sprintf "+verb] = fmt.Sprintf(verb, v)
	}
	exact["Sprintf %v of a pointer"] = fmt.Sprintf("%v", &v)
	exact["Sprint"] = fmt.Sprint(v)
	exact["String"] = v.String()
	exact["GoString"] = v.GoString()
	exact["LogValue"] = v.LogValue().String()
	exact["Errorf %v"] = fmt.Errorf("%v", v).Error()
	var tm encoding.TextMarshaler = v
	text, err := tm.MarshalText()
	if err != nil {
		t.Fatal(err)
	}
	exact["MarshalText"] = string(text)
	for label, out := range exact {
		if out != redacted {
			t.Errorf("%s = %q, want %q", label, out, redacted)
		}
	}

	// Renderings that embed the value elsewhere.
	embedded := map[string]struct{ got, want string }{
		"Sprintln":                      {fmt.Sprintln(v), "[redacted]\n"},
		"Sprintf %v of a slice":         {fmt.Sprintf("%v", []secrets.Value{v}), "[[redacted]]"},
		"Sprintf %+v of exported field": {fmt.Sprintf("%+v", struct{ V secrets.Value }{v}), "{V:[redacted]}"},
		"json.Marshal value":            {mustJSON(t, v), `"[redacted]"`},
		"json.Marshal struct":           {mustJSON(t, struct{ Password secrets.Value }{v}), `{"Password":"[redacted]"}`},
		"json.Marshal map key":          {mustJSON(t, map[secrets.Value]int{v: 1}), `{"[redacted]":1}`},
	}
	for label, c := range embedded {
		if c.got != c.want {
			t.Errorf("%s = %q, want %q", label, c.got, c.want)
		}
	}

	// slog handlers, top level and nested in a group.
	for name, newHandler := range map[string]func(*bytes.Buffer) slog.Handler{
		"json": func(b *bytes.Buffer) slog.Handler { return slog.NewJSONHandler(b, nil) },
		"text": func(b *bytes.Buffer) slog.Handler { return slog.NewTextHandler(b, nil) },
	} {
		var buf bytes.Buffer
		logger := slog.New(newHandler(&buf))
		logger.InfoContext(context.Background(), "secret read", slog.Any("secret", v), slog.Group("g", slog.Any("nested", v)))
		out := buf.String()
		assertNoSecret(t, "slog "+name, out, canary)
		if !bytes.Contains(buf.Bytes(), []byte(redacted)) {
			t.Errorf("slog %s output %q lacks %q", name, out, redacted)
		}
	}

	// %w with a non-error has no rendering of its own; it must not leak either. The argument
	// slice keeps vet from rejecting the deliberate misuse.
	args := []any{v}
	wrapped := fmt.Errorf("%w", args...)
	all := []string{wrapped.Error()}
	for _, out := range exact {
		all = append(all, out)
	}
	for _, c := range embedded {
		all = append(all, c.got)
	}
	for _, out := range all {
		assertNoSecret(t, "rendering", out, canary)
	}
}

// TestValueNeverPrintsThroughReflection covers the paths where fmt does not call Format: an
// unexported struct field and the verb %p, which fmt renders through reflection (Review Focus 5).
func TestValueNeverPrintsThroughReflection(t *testing.T) {
	t.Parallel()
	canary := newCanary(t)
	v := secrets.NewValue([]byte(canary))
	holder := struct{ v secrets.Value }{v}
	// The argument slices keep vet from rejecting the verbs it knows do not fit (%p of a struct,
	// %x of a struct with an unexported field); those misuses are what this test is about.
	sprintf := func(format string, arg any) string {
		args := []any{arg}
		return fmt.Sprintf(format, args...)
	}
	outputs := map[string]string{
		"%+v of unexported field": sprintf("%+v", holder),
		"%#v of unexported field": sprintf("%#v", holder),
		"%v of unexported field":  sprintf("%v", holder),
		"%x of unexported field":  sprintf("%x", holder),
		"%p of a Value":           sprintf("%p", v),
	}
	for label, out := range outputs {
		assertNoSecret(t, label, out, canary)
	}
}

func TestValueBytesCopyLenEqual(t *testing.T) {
	t.Parallel()
	input := []byte("EXAMPLE-secret")
	v := secrets.NewValue(input)
	input[0] = 'X'
	if got := string(v.Bytes()); got != "EXAMPLE-secret" {
		t.Fatalf("NewValue kept the caller's slice: Bytes = %q", got)
	}
	b := v.Bytes()
	b[0] = 'Y'
	if got := string(v.Bytes()); got != "EXAMPLE-secret" {
		t.Fatalf("Bytes returned the internal slice: after a change Bytes = %q", got)
	}
	if v.Len() != len("EXAMPLE-secret") {
		t.Fatalf("Len = %d, want %d", v.Len(), len("EXAMPLE-secret"))
	}
	if !v.Equal(secrets.NewValue([]byte("EXAMPLE-secret"))) {
		t.Fatal("Equal = false for equal content")
	}
	if v.Equal(secrets.NewValue([]byte("EXAMPLE-secreT"))) {
		t.Fatal("Equal = true for different content of the same length")
	}
	if v.Equal(secrets.NewValue([]byte("EXAMPLE"))) {
		t.Fatal("Equal = true for a prefix")
	}

	var zero secrets.Value
	if zero.Len() != 0 || zero.Bytes() != nil {
		t.Fatalf("zero Value: Len = %d, Bytes = %v, want 0 and nil", zero.Len(), zero.Bytes())
	}
	if zero.Equal(v) || v.Equal(zero) {
		t.Fatal("zero Value equals a non-empty Value")
	}
	if !zero.Equal(secrets.Value{}) {
		t.Fatal("zero Value does not equal itself")
	}
}

func TestVersionIsSHA256Prefix(t *testing.T) {
	t.Parallel()
	// The literal is the first 16 hex characters of the SHA-256 of "EXAMPLE", computed outside
	// the code under test (shasum -a 256).
	const want = secrets.Version("f2e8378c30d317ab")
	d := secrets.NewEnvDriver([]string{
		"BOTHY_SECRET_A=EXAMPLE", "BOTHY_SECRET_B=EXAMPLE", "BOTHY_SECRET_C=EXAMPLE2",
	})
	ctx := context.Background()
	version := func(name string) secrets.Version {
		t.Helper()
		_, ver, err := d.Get(ctx, secrets.Ref{Scheme: "env", Path: name})
		if err != nil {
			t.Fatal(err)
		}
		return ver
	}
	if got := version("BOTHY_SECRET_A"); got != want {
		t.Fatalf("version of EXAMPLE = %q, want %q", got, want)
	}
	if got := version("BOTHY_SECRET_B"); got != want {
		t.Fatalf("equal content gave version %q, want %q", got, want)
	}
	if got := version("BOTHY_SECRET_C"); got != "e0600a843840328a" {
		t.Fatalf("version of EXAMPLE2 = %q, want e0600a843840328a", got)
	}
}

func mustJSON(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
