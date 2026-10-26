// SPDX-License-Identifier: Apache-2.0

package guide

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
)

// placeholderPattern finds a placeholder: {name} or {secret:<file>}.
var placeholderPattern = regexp.MustCompile(`\{(secret:[a-z0-9][a-z0-9._-]*|[a-z][a-z0-9_]*)\}`)

// secretPrefix marks a secret capture.
const secretPrefix = "secret:"

// Bindings holds the values of the placeholders: those the runner binds before the first block
// and those captured from answers and output lines. Secrets are kept apart: they are written to
// their file, never compared and redacted from every error.
type Bindings struct {
	mu        sync.Mutex
	values    map[string]any
	secrets   map[string]string
	secretDir string
	// fresh returns the value of a placeholder that is new at each occurrence ({new_uuid}).
	fresh map[string]func() string
}

// NewBindings returns bindings with the given values; secrets are written into secretDir.
func NewBindings(values map[string]any, secretDir string) *Bindings {
	b := &Bindings{values: map[string]any{}, secrets: map[string]string{}, secretDir: secretDir, fresh: map[string]func() string{}}
	for k, v := range values {
		b.values[k] = v
	}
	return b
}

// Fresh makes name a placeholder whose value gen produces anew at each occurrence.
func (b *Bindings) Fresh(name string, gen func() string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.fresh[name] = gen
}

// Value returns the bound value of name.
func (b *Bindings) Value(name string) (any, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	v, ok := b.values[name]
	return v, ok
}

// Secrets returns the captured secret values (for the final checks of the logs).
func (b *Bindings) Secrets() []string {
	b.mu.Lock()
	defer b.mu.Unlock()
	out := make([]string, 0, len(b.secrets))
	for _, v := range b.secrets {
		out = append(out, v)
	}
	sort.Strings(out)
	return out
}

// Redact replaces every captured secret in s.
func (b *Bindings) Redact(s string) string {
	for _, v := range b.Secrets() {
		if v != "" {
			s = strings.ReplaceAll(s, v, "[redacted]")
		}
	}
	return s
}

// text is the textual form of a bound value.
func text(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case json.Number:
		return x.String()
	default:
		raw, _ := json.Marshal(x)
		return string(raw)
	}
}

// SubstituteText replaces every bound placeholder in s with its textual form; an unbound one is
// an error.
func (b *Bindings) SubstituteText(s string) (string, error) {
	var missing error
	out := placeholderPattern.ReplaceAllStringFunc(s, func(m string) string {
		v, err := b.resolve(m[1 : len(m)-1])
		if err != nil {
			missing = err
			return m
		}
		return text(v)
	})
	return out, missing
}

// resolve returns the value of a placeholder name.
func (b *Bindings) resolve(name string) (any, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if gen, ok := b.fresh[name]; ok {
		return gen(), nil
	}
	if v, ok := b.values[name]; ok {
		return v, nil
	}
	return nil, fmt.Errorf("placeholder {%s} is not bound", name)
}

// SubstituteJSON replaces placeholders in a JSON document: a string that is exactly "{name}"
// becomes the bound value with its JSON type, other strings are substituted textually. It
// returns the compact document.
func (b *Bindings) SubstituteJSON(doc string) ([]byte, error) {
	v, err := decode([]byte(doc))
	if err != nil {
		return nil, err
	}
	out, err := b.substitute(v)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(out); err != nil {
		return nil, err
	}
	return bytes.TrimRight(buf.Bytes(), "\n"), nil
}

func (b *Bindings) substitute(v any) (any, error) {
	switch x := v.(type) {
	case string:
		if name, ok := whole(x); ok && !strings.HasPrefix(name, secretPrefix) {
			return b.resolve(name)
		}
		return b.SubstituteText(x)
	case map[string]any:
		out := map[string]any{}
		for k, e := range x {
			s, err := b.substitute(e)
			if err != nil {
				return nil, err
			}
			out[k] = s
		}
		return out, nil
	case []any:
		out := make([]any, len(x))
		for i, e := range x {
			s, err := b.substitute(e)
			if err != nil {
				return nil, err
			}
			out[i] = s
		}
		return out, nil
	}
	return v, nil
}

// whole reports whether s is exactly one placeholder and returns its name.
func whole(s string) (string, bool) {
	m := placeholderPattern.FindStringSubmatchIndex(s)
	if m == nil || m[0] != 0 || m[1] != len(s) {
		return "", false
	}
	return s[1 : len(s)-1], true
}

// decode parses JSON keeping numbers as json.Number.
func decode(raw []byte) (any, error) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return nil, fmt.Errorf("invalid JSON: %w", err)
	}
	return v, nil
}

// MatchJSON matches an actual body against an expected subset (README.md "Matching"). A string
// that is exactly "{name}" captures the actual value of an unbound name and compares a bound
// one; "{secret:<file>}" captures a string into that file. The error names the JSON Pointer of
// the first difference and never holds a secret.
func (b *Bindings) MatchJSON(expected string, actual []byte) error {
	want, err := decode([]byte(expected))
	if err != nil {
		return fmt.Errorf("the expected body: %w", err)
	}
	got, err := decode(actual)
	if err != nil {
		return fmt.Errorf("the answer: %w", err)
	}
	// Secrets first, so a difference found before them is still redacted.
	if err := b.captureSecrets(want, got); err != nil {
		return err
	}
	if err := b.match(want, got, ""); err != nil {
		return errors.New(b.Redact(err.Error()))
	}
	return nil
}

// captureSecrets binds every {secret:<file>} of the expected objects to the actual string.
func (b *Bindings) captureSecrets(want, got any) error {
	w, ok := want.(map[string]any)
	if !ok {
		return nil
	}
	g, _ := got.(map[string]any)
	for k, wv := range w {
		if s, isString := wv.(string); isString {
			if name, whole := whole(s); whole && strings.HasPrefix(name, secretPrefix) {
				gv, present := g[k].(string)
				if !present || gv == "" {
					return fmt.Errorf("/%s: no secret string in the answer", escape(k))
				}
				if err := b.keepSecret(strings.TrimPrefix(name, secretPrefix), gv); err != nil {
					return err
				}
			}
			continue
		}
		if err := b.captureSecrets(wv, g[k]); err != nil {
			return err
		}
	}
	return nil
}

// keepSecret writes a secret to its file (mode 0600) and remembers it for redaction.
func (b *Bindings) keepSecret(file, value string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if _, dup := b.secrets[file]; dup {
		return fmt.Errorf("secret %s captured twice", file)
	}
	b.secrets[file] = value
	path := filepath.Join(b.secretDir, file)
	if err := os.WriteFile(path, []byte(value), 0o600); err != nil { // #nosec G306 -- the adapter's own file, mode 0600
		return fmt.Errorf("write the secret file %s: %w", file, err)
	}
	return os.Chmod(path, 0o600)
}

// escape escapes a member name for a JSON Pointer (RFC 6901).
func escape(k string) string {
	return strings.ReplaceAll(strings.ReplaceAll(k, "~", "~0"), "/", "~1")
}

// kind is the JSON type of a decoded value.
func kind(v any) string {
	switch v.(type) {
	case nil:
		return "null"
	case string:
		return "string"
	case json.Number:
		return "number"
	case bool:
		return "boolean"
	case []any:
		return "array"
	case map[string]any:
		return "object"
	}
	return fmt.Sprintf("%T", v)
}

// show renders a value for an error, cut short.
func show(v any) string {
	raw, _ := json.Marshal(v)
	s := string(raw)
	if len(s) > 200 {
		s = s[:200] + "..."
	}
	return s
}

func (b *Bindings) match(want, got any, ptr string) error {
	at := ptr
	if at == "" {
		at = "/"
	}
	switch w := want.(type) {
	case string:
		if name, ok := whole(w); ok {
			if strings.HasPrefix(name, secretPrefix) {
				return nil // captured by captureSecrets, never compared
			}
			if v, bound := b.Value(name); bound {
				return b.match(v, got, ptr)
			}
			if _, isFresh := b.fresh[name]; isFresh {
				return fmt.Errorf("%s: {%s} is new at each use and cannot be compared", at, name)
			}
			b.mu.Lock()
			b.values[name] = got
			b.mu.Unlock()
			return nil
		}
		s, err := b.SubstituteText(w)
		if err != nil {
			return fmt.Errorf("%s: %w", at, err)
		}
		if g, ok := got.(string); !ok || g != s {
			return fmt.Errorf("%s: want %s, got %s", at, show(s), show(got))
		}
	case map[string]any:
		g, ok := got.(map[string]any)
		if !ok {
			return fmt.Errorf("%s: want an object, got %s %s", at, kind(got), show(got))
		}
		keys := make([]string, 0, len(w))
		for k := range w {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			gv, present := g[k]
			if !present {
				return fmt.Errorf("%s/%s: missing in the answer", ptr, escape(k))
			}
			if err := b.match(w[k], gv, ptr+"/"+escape(k)); err != nil {
				return err
			}
		}
	case []any:
		g, ok := got.([]any)
		if !ok {
			return fmt.Errorf("%s: want an array, got %s %s", at, kind(got), show(got))
		}
		return b.matchArray(w, g, ptr)
	default:
		if kind(want) != kind(got) || show(want) != show(got) {
			return fmt.Errorf("%s: want %s %s, got %s %s", at, kind(want), show(want), kind(got), show(got))
		}
	}
	return nil
}

// matchArray matches each expected element to exactly one distinct actual element, in any
// order; an element that matches more than one actual element is ambiguous.
func (b *Bindings) matchArray(want, got []any, ptr string) error {
	used := map[int]bool{}
	for i, w := range want {
		var hits []int
		for j, g := range got {
			if b.trial().match(w, g, "") == nil {
				hits = append(hits, j)
			}
		}
		at := fmt.Sprintf("%s/%d", ptr, i)
		switch {
		case len(hits) == 0:
			return fmt.Errorf("%s: no element of the answer matches %s; the answer holds %s", at, show(w), show(got))
		case len(hits) > 1:
			return fmt.Errorf("%s: ambiguous, %d elements of the answer match %s; add a distinguishing member", at, len(hits), show(w))
		case used[hits[0]]:
			return fmt.Errorf("%s: the element of the answer that matches %s is taken by another expected element", ptr, show(w))
		}
		used[hits[0]] = true
		if err := b.match(w, got[hits[0]], at); err != nil {
			return err
		}
	}
	return nil
}

// trial returns a copy whose captures do not reach b.
func (b *Bindings) trial() *Bindings {
	b.mu.Lock()
	defer b.mu.Unlock()
	c := &Bindings{values: map[string]any{}, secrets: map[string]string{}, secretDir: b.secretDir, fresh: b.fresh}
	for k, v := range b.values {
		c.values[k] = v
	}
	for k, v := range b.secrets {
		c.secrets[k] = v
	}
	return c
}

// lineCapture is the pattern of an unbound placeholder in an output line.
const lineCapture = `(\S+)`

// MatchLine matches one actual output line against an expected line: bound placeholders are
// substituted, an unbound one captures a run of non-space characters.
func (b *Bindings) MatchLine(expected, actual string) error {
	var names []string
	var pattern strings.Builder
	pattern.WriteString("^")
	rest := expected
	for {
		loc := placeholderPattern.FindStringIndex(rest)
		if loc == nil {
			pattern.WriteString(regexp.QuoteMeta(rest))
			break
		}
		pattern.WriteString(regexp.QuoteMeta(rest[:loc[0]]))
		name := rest[loc[0]+1 : loc[1]-1]
		if v, err := b.resolve(name); err == nil {
			pattern.WriteString(regexp.QuoteMeta(text(v)))
		} else {
			pattern.WriteString(lineCapture)
			names = append(names, name)
		}
		rest = rest[loc[1]:]
	}
	pattern.WriteString("$")
	m := regexp.MustCompile(pattern.String()).FindStringSubmatch(actual)
	if m == nil {
		return fmt.Errorf("output line differs:\n  want %s\n  got  %s", expected, b.Redact(actual))
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	for i, name := range names {
		if v, dup := b.values[name]; dup && v != m[i+1] {
			return fmt.Errorf("output line binds {%s} twice with different values", name)
		}
		b.values[name] = m[i+1]
	}
	return nil
}

// MatchLines takes the next lines of src and matches them in order; it fails at the first
// differing line, and when src ends before every expected line arrived.
func (b *Bindings) MatchLines(ctx context.Context, expected []string, src *Lines) error {
	for _, want := range expected {
		got, ok, err := src.Next(ctx)
		if err != nil {
			return fmt.Errorf("waiting for %q: %w", want, err)
		}
		if !ok {
			return fmt.Errorf("the process ended before the line %q (exit code %d)", want, src.ExitCode())
		}
		if err := b.MatchLine(want, got); err != nil {
			return err
		}
	}
	return nil
}

// Lines is the stdout of a process line by line, as it arrives.
type Lines struct {
	mu     sync.Mutex
	lines  []string
	next   int
	ended  bool
	exit   int
	notify chan struct{}
}

// NewLines returns an empty line buffer.
func NewLines() *Lines { return &Lines{notify: make(chan struct{})} }

// wake wakes every waiting reader; callers hold mu.
func (l *Lines) wake() {
	close(l.notify)
	l.notify = make(chan struct{})
}

// Push appends a line.
func (l *Lines) Push(line string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.lines = append(l.lines, line)
	l.wake()
}

// Close marks the end of the stream with the process's exit code.
func (l *Lines) Close(exit int) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.ended, l.exit = true, exit
	l.wake()
}

// Ended reports whether the stream ended.
func (l *Lines) Ended() bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.ended
}

// ExitCode is the exit code after the end.
func (l *Lines) ExitCode() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.exit
}

// All returns every line received so far.
func (l *Lines) All() []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]string(nil), l.lines...)
}

// Next returns the next line, waiting for it; ok is false when the stream ended first.
func (l *Lines) Next(ctx context.Context) (line string, ok bool, err error) {
	for {
		l.mu.Lock()
		if l.next < len(l.lines) {
			line = l.lines[l.next]
			l.next++
			l.mu.Unlock()
			return line, true, nil
		}
		if l.ended {
			l.mu.Unlock()
			return "", false, nil
		}
		wait := l.notify
		l.mu.Unlock()
		select {
		case <-wait:
		case <-ctx.Done():
			return "", false, ctx.Err()
		}
	}
}

// Wait blocks until the stream ended.
func (l *Lines) Wait(ctx context.Context) error {
	for {
		l.mu.Lock()
		if l.ended {
			l.mu.Unlock()
			return nil
		}
		wait := l.notify
		l.mu.Unlock()
		select {
		case <-wait:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// Drained fails when lines arrived that no output block consumed.
func (l *Lines) Drained() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.next < len(l.lines) {
		return fmt.Errorf("%d output line(s) no output block expects, first %q", len(l.lines)-l.next, l.lines[l.next])
	}
	return nil
}
