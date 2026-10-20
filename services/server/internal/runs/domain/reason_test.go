// SPDX-License-Identifier: Apache-2.0

package domain_test

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/runs/domain"
)

// specTable returns the cells of every body row of the first table under the heading of the
// agents chapter (docs/spec/09-*.md) whose text starts with prefix ("### 5.2 ", "## 6. "): the
// heading is matched by its number, so the test reads the chapter in either language. Header and
// separator rows (first cell not a code in backticks) are skipped; backticks are trimmed.
func specTable(t *testing.T, prefix string) [][]string {
	t.Helper()
	files, err := filepath.Glob(filepath.Join(repoRoot(t), "docs", "spec", "09-*.md"))
	if err != nil || len(files) != 1 {
		t.Fatalf("want exactly one agents chapter docs/spec/09-*.md, got %v (%v)", files, err)
	}
	f, err := os.Open(files[0])
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	var rows [][]string
	in := false
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()
		if strings.HasPrefix(line, "## ") || strings.HasPrefix(line, "### ") {
			in = strings.HasPrefix(line, prefix)
			continue
		}
		if !in || !strings.HasPrefix(line, "|") {
			continue
		}
		parts := strings.Split(strings.Trim(line, "|"), "|")
		first := strings.TrimSpace(parts[0])
		if !strings.HasPrefix(first, "`") {
			continue
		}
		cells := make([]string, len(parts))
		for i, p := range parts {
			cells[i] = strings.Trim(strings.TrimSpace(p), "`")
		}
		rows = append(rows, cells)
	}
	if err := sc.Err(); err != nil {
		t.Fatal(err)
	}
	if len(rows) == 0 {
		t.Fatalf("no rows found under heading %q of the agents chapter", prefix)
	}
	return rows
}

// specSetter reads the "set by" cell of 09 5.2 in either language: a cell naming both the adapter
// and the server ("Adapter or Server", "Adapter oder Server") is SetterBoth.
func specSetter(t *testing.T, cell string) domain.Setter {
	t.Helper()
	a, s := strings.Contains(cell, "Adapter"), strings.Contains(cell, "Server")
	switch {
	case a && s:
		return domain.SetterBoth
	case a:
		return domain.SetterAdapter
	case s:
		return domain.SetterServer
	}
	t.Fatalf("unknown setter cell %q in 09 5.2", cell)
	return 0
}

// TestReasonTable checks the reason table against itself and against 09 5.2 of this repository:
// every row has a state of failed or cancelled and a valid setter, codes are unique, ReasonOf
// finds every row, and the table equals the spec table row by row (code, state, setter).
func TestReasonTable(t *testing.T) {
	table := domain.ReasonTable()
	seen := map[string]bool{}
	for _, r := range table {
		if r.State != domain.StateFailed && r.State != domain.StateCancelled {
			t.Errorf("%s: state %s, reasons exist only for failed and cancelled", r.Code, r.State)
		}
		if r.Setter.String() != "server" && r.Setter.String() != "adapter" && r.Setter.String() != "both" {
			t.Errorf("%s: invalid setter %d", r.Code, r.Setter)
		}
		if seen[r.Code] {
			t.Errorf("%s: duplicate row", r.Code)
		}
		seen[r.Code] = true
		if got, ok := domain.ReasonOf(r.Code); !ok || got != r {
			t.Errorf("ReasonOf(%s) = %v, %v; want %v", r.Code, got, ok, r)
		}
	}
	if _, ok := domain.ReasonOf("no_such_reason"); ok {
		t.Error("ReasonOf accepts an unknown code")
	}
	if last := table[len(table)-1]; last != (domain.ReasonRow{Code: domain.RunReasonUserDisabled, State: domain.StateCancelled, Setter: domain.SetterServer}) {
		t.Errorf("last row = %v, want user_disabled, cancelled, server (D-M2-05-7)", last)
	}
	table[0].Code = "mutated"
	if domain.ReasonTable()[0].Code == "mutated" {
		t.Error("ReasonTable returns shared storage")
	}

	spec := specTable(t, "### 5.2 ")
	table = domain.ReasonTable()
	if len(spec) != len(table) {
		t.Fatalf("09 5.2 has %d rows, the reason table %d", len(spec), len(table))
	}
	for i, cells := range spec {
		want := domain.ReasonRow{Code: cells[0], State: domain.State(cells[1]), Setter: specSetter(t, cells[2])}
		if table[i] != want {
			t.Errorf("row %d: table %v, 09 5.2 %v", i, table[i], want)
		}
	}
}

// TestSetterString pins the names of the setters and the zero value.
func TestSetterString(t *testing.T) {
	for s, want := range map[domain.Setter]string{
		domain.SetterServer: "server", domain.SetterAdapter: "adapter", domain.SetterBoth: "both", 0: "invalid",
	} {
		if got := s.String(); got != want {
			t.Errorf("Setter(%d).String() = %q, want %q", s, got, want)
		}
	}
}

// TestNormalizeAdapterReason checks ruling D-M2-05-8: an adapter failed keeps only the codes 09
// 5.2 lists for the adapter; every other code becomes adapter_error with the original.
func TestNormalizeAdapterReason(t *testing.T) {
	for _, code := range []string{
		"billing_mode_mismatch", "tool_class_mismatch", "invalid_invocation_token",
		"runtime_error", "adapter_error", "runtime_not_authenticated",
	} {
		reason, original, err := domain.NormalizeAdapterReason(code)
		if err != nil || reason != code || original != "" {
			t.Errorf("NormalizeAdapterReason(%s) = %q, %q, %v; want kept", code, reason, original, err)
		}
	}
	for _, code := range []string{
		"quota_exhausted", "stale", "interrupted", "agent_unavailable",
		"billing_mode_personal_not_holder", "user_cancelled", "user_disabled", "Runtime_Error", " runtime_error",
	} {
		reason, original, err := domain.NormalizeAdapterReason(code)
		if err != nil || reason != domain.RunReasonAdapterError || original != code {
			t.Errorf("NormalizeAdapterReason(%s) = %q, %q, %v; want adapter_error with original", code, reason, original, err)
		}
	}
	if _, _, err := domain.NormalizeAdapterReason(""); !errors.Is(err, domain.ErrReasonRequired) {
		t.Errorf("empty code: err = %v, want ErrReasonRequired", err)
	}
}

// TestValidateServerReason checks the server path: failed and cancelled need a server or both
// row of the same state; the other states take no reason; every violation is internal_error.
func TestValidateServerReason(t *testing.T) {
	ok := []struct {
		to   domain.State
		code string
	}{
		{domain.StateCancelled, domain.RunReasonUserCancelled},
		{domain.StateCancelled, domain.RunReasonUserDisabled},
		{domain.StateCancelled, "access_revoked"},
		{domain.StateFailed, domain.RunReasonStale},
		{domain.StateFailed, domain.RunReasonInterrupted},
		{domain.StateFailed, "billing_mode_mismatch"}, // both
		{domain.StateFailed, "tool_class_mismatch"},   // both
		{domain.StateQueued, ""},
		{domain.StateRunning, ""},
		{domain.StateWaitingInput, ""},
		{domain.StateCompleted, ""},
	}
	for _, c := range ok {
		if err := domain.ValidateServerReason(c.to, c.code); err != nil {
			t.Errorf("ValidateServerReason(%s, %q) = %v, want nil", c.to, c.code, err)
		}
	}
	bad := []struct {
		to   domain.State
		code string
	}{
		{domain.StateFailed, ""},                            // the server always knows its reason
		{domain.StateCancelled, ""},                         // same
		{domain.StateFailed, domain.RunReasonRuntimeError},  // adapter only
		{domain.StateFailed, domain.RunReasonAdapterError},  // adapter only
		{domain.StateFailed, domain.RunReasonUserCancelled}, // state mismatch
		{domain.StateCancelled, domain.RunReasonStale},      // state mismatch
		{domain.StateFailed, "quota_exhausted"},             // unknown
		{domain.StateRunning, domain.RunReasonStale},        // no reason outside failed, cancelled
		{domain.StateCompleted, domain.RunReasonUserCancelled},
		{domain.StateQueued, domain.RunReasonInterrupted},
		{domain.StateWaitingInput, domain.RunReasonInterrupted},
		{"bogus", ""},
	}
	for _, c := range bad {
		err := domain.ValidateServerReason(c.to, c.code)
		e, ok := errs.As(err)
		if !ok || e.Code != errs.CodeInternalError || errors.Is(err, domain.ErrReasonRequired) {
			t.Errorf("ValidateServerReason(%s, %q) = %v, want internal_error", c.to, c.code, err)
		}
	}
}

// encodedLen is the length of v encoded like OriginalReasonDetail measures it.
func encodedLen(t *testing.T, v map[string]any) int {
	t.Helper()
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		t.Fatal(err)
	}
	return len(bytes.TrimSuffix(b.Bytes(), []byte("\n")))
}

// TestOriginalReasonDetail checks the cut on a UTF-8 boundary within run.step_detail_max_bytes,
// the rejection of control and bidi characters and a limit too small for any detail.
func TestOriginalReasonDetail(t *testing.T) {
	const empty = len(`{"original_reason":""}`)
	if got := domain.OriginalReasonDetail("quota_exhausted", 1024); got["original_reason"] != "quota_exhausted" || len(got) != 1 {
		t.Errorf("short code: %v", got)
	}
	code := "quota_ü€😀_<&>_\"x\""
	full := encodedLen(t, map[string]any{"original_reason": code})
	for limit := empty; limit <= full+2; limit++ {
		got := domain.OriginalReasonDetail(code, limit)
		s, ok := got["original_reason"].(string)
		if !ok || len(got) != 1 {
			t.Fatalf("limit %d: %v", limit, got)
		}
		if !strings.HasPrefix(code, s) || !utf8.ValidString(s) {
			t.Errorf("limit %d: %q is not a valid prefix of the code", limit, s)
		}
		if n := encodedLen(t, got); n > limit {
			t.Errorf("limit %d: encoded detail has %d bytes", limit, n)
		}
		if s != code {
			// The longest prefix: one more rune would not fit.
			_, size := utf8.DecodeRuneInString(code[len(s):])
			if n := encodedLen(t, map[string]any{"original_reason": code[:len(s)+size]}); n <= limit {
				t.Errorf("limit %d: %q is not the longest prefix", limit, s)
			}
		}
	}
	if got := domain.OriginalReasonDetail("abc", empty); got["original_reason"] != "" {
		t.Errorf("limit of the empty detail: %v", got)
	}
	for _, c := range []struct {
		code  string
		limit int
	}{
		{"evil\u202Ecode", 1024},
		{"line\nbreak", 1024},
		{"bad\xffutf8", 1024},
		{"quota_exhausted", empty - 1},
		{"quota_exhausted", 0},
	} {
		if got := domain.OriginalReasonDetail(c.code, c.limit); got == nil || len(got) != 0 {
			t.Errorf("OriginalReasonDetail(%q, %d) = %v, want {}", c.code, c.limit, got)
		}
	}
}
