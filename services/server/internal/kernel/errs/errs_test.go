// SPDX-License-Identifier: Apache-2.0

package errs

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

// The generated block of chapter 06 that lists every public problem code.
const (
	beginProblems = "<!-- BEGIN GENERATED: problems -->"
	endProblems   = "<!-- END GENERATED: problems -->"
)

func TestKindStatus(t *testing.T) {
	tests := []struct {
		kind   Kind
		name   string
		status int
	}{
		{KindNone, "none", 0},
		{KindBadRequest, "bad_request", 400},
		{KindUnauthenticated, "unauthenticated", 401},
		{KindStepUpRequired, "step_up_required", 401},
		{KindForbidden, "forbidden", 403},
		{KindNotFound, "not_found", 404},
		{KindMethodNotAllowed, "method_not_allowed", 405},
		{KindConflict, "conflict", 409},
		{KindInvariantViolation, "invariant_violation", 409},
		{KindGone, "gone", 410},
		{KindPreconditionFailed, "precondition_failed", 412},
		{KindTooLarge, "too_large", 413},
		{KindUnsupportedMediaType, "unsupported_media_type", 415},
		{KindRangeNotSatisfiable, "range_not_satisfiable", 416},
		{KindUnprocessable, "unprocessable", 422},
		{KindPreconditionRequired, "precondition_required", 428},
		{KindRateLimited, "rate_limited", 429},
		{KindInternal, "internal", 500},
		{KindUnavailable, "unavailable", 503},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.kind.Status(); got != tt.status {
				t.Errorf("%s.Status() = %d, want %d", tt.name, got, tt.status)
			}
			if got := tt.kind.String(); got != tt.name {
				t.Errorf("Kind(%d).String() = %q, want %q", uint8(tt.kind), got, tt.name)
			}
		})
	}
	if KindNone != 0 {
		t.Errorf("KindNone = %d, want the zero value", uint8(KindNone))
	}
}

func TestKindStringOfUnknownValue(t *testing.T) {
	for _, k := range []Kind{KindUnavailable + 1, Kind(200), Kind(255)} {
		want := "kind(" + strconv.Itoa(int(k)) + ")"
		if got := k.String(); got != want {
			t.Errorf("Kind(%d).String() = %q, want %q", uint8(k), got, want)
		}
		if got := k.Status(); got != 0 {
			t.Errorf("Kind(%d).Status() = %d, want 0", uint8(k), got)
		}
	}
}

// repoRoot walks up from the working directory to the first directory that contains docs/spec,
// so the test reads the real chapter 06 and not a copy.
func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("working directory: %v", err)
	}
	for {
		info, err := os.Stat(filepath.Join(dir, "docs", "spec"))
		if err == nil && info.IsDir() {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("no directory with docs/spec above the working directory")
		}
		dir = parent
	}
}

// chapter06Codes returns code to HTTP status from the generated problems block of 06, in
// document order.
func chapter06Codes(t *testing.T) ([]Code, map[Code]int) {
	t.Helper()
	path := filepath.Join(repoRoot(t), "docs", "spec", "06-api.md")
	data, err := os.ReadFile(path) //nolint:gosec // G304: path is built from the repository root found above, not from input.
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	text := string(data)
	begin := strings.Index(text, beginProblems)
	end := strings.Index(text, endProblems)
	if begin < 0 || end < 0 || end < begin {
		t.Fatalf("%s: markers %q and %q missing or out of order", path, beginProblems, endProblems)
	}
	var order []Code
	status := map[Code]int{}
	sc := bufio.NewScanner(strings.NewReader(text[begin+len(beginProblems) : end]))
	for sc.Scan() {
		line := sc.Text()
		if !strings.HasPrefix(line, "| `") {
			continue
		}
		cells := strings.Split(line, "|")
		if len(cells) < 4 {
			t.Fatalf("%s: row with fewer than two cells: %q", path, line)
		}
		code := Code(strings.Trim(strings.TrimSpace(cells[1]), "`"))
		st, err := strconv.Atoi(strings.TrimSpace(cells[2]))
		if err != nil {
			t.Fatalf("%s: status of %q is not a number: %q", path, code, cells[2])
		}
		if _, dup := status[code]; dup {
			t.Fatalf("%s: code %q listed twice", path, code)
		}
		order = append(order, code)
		status[code] = st
	}
	if len(order) == 0 {
		t.Fatalf("%s: no code rows between the markers", path)
	}
	return order, status
}

func TestCodesMatchChapter06(t *testing.T) {
	docOrder, docStatus := chapter06Codes(t)
	ours := map[Code]bool{}
	for _, c := range AllCodes() {
		if ours[c] {
			t.Errorf("AllCodes lists %q twice", c)
		}
		ours[c] = true
		st, ok := docStatus[c]
		if !ok {
			t.Errorf("code %q is in AllCodes but not in docs/spec/06-api.md", c)
			continue
		}
		if got := c.Kind().Status(); got != st {
			t.Errorf("code %q: Kind %s has status %d, 06 says %d", c, c.Kind(), got, st)
		}
	}
	for _, c := range docOrder {
		if !ours[c] {
			t.Errorf("code %q is in docs/spec/06-api.md but not in AllCodes", c)
		}
	}
	if got := AllCodes(); len(got) == len(docOrder) {
		for i := range got {
			if got[i] != docOrder[i] {
				t.Errorf("AllCodes()[%d] = %q, 06 lists %q at that position", i, got[i], docOrder[i])
			}
		}
	}
}

func TestEveryCodeHasKind(t *testing.T) {
	for _, c := range AllCodes() {
		if c.Kind() == KindNone {
			t.Errorf("code %q has no kind", c)
		}
	}
	if got := Code("no_such_code").Kind(); got != KindNone {
		t.Errorf("unknown code: Kind() = %s, want none", got)
	}
}

// wantKinds is the expected kind of every public code: the mapping of the brief of M1-05, plus
// docs/DECISIONS.md M1-05/4 for the codes 06 gained later (terminal_denied, i6_violation,
// host_unavailable, fingerprint_mismatch) and M1-05/19 for billing_mode_mismatch and
// tool_class_mismatch, which docs/spec/07-permissions.md section 4.1 maps to invariant_violation.
// It names every code on purpose: several kinds share a status (401, 409), so the status check
// against 06 cannot tell them apart.
var wantKinds = map[Code]Kind{
	CodeValidationFailed:       KindBadRequest,
	CodeMalformedRequest:       KindBadRequest,
	CodeCursorInvalid:          KindBadRequest,
	CodeWebhookHostNotAllowed:  KindBadRequest,
	CodeUnauthenticated:        KindUnauthenticated,
	CodeTokenExpired:           KindUnauthenticated,
	CodeSessionRevoked:         KindUnauthenticated,
	CodeStepUpRequired:         KindStepUpRequired,
	CodeUseDPoPNonce:           KindUnauthenticated,
	CodeInvalidDPoPProof:       KindUnauthenticated,
	CodeForbidden:              KindForbidden,
	CodeScopeMissing:           KindForbidden,
	CodeDevicePending:          KindForbidden,
	CodeChannelReadOnly:        KindForbidden,
	CodeFeatureDisabled:        KindForbidden,
	CodeAgentInvocationDenied:  KindForbidden,
	CodeTerminalDenied:         KindForbidden,
	CodeInvocationTokenInvalid: KindForbidden,
	CodeNotFound:               KindNotFound,
	CodeMethodNotAllowed:       KindMethodNotAllowed,
	CodeConflict:               KindConflict,
	CodeRequestInProgress:      KindConflict,
	CodeI1Violation:            KindInvariantViolation,
	CodeI6Violation:            KindInvariantViolation,
	CodeLastOwner:              KindInvariantViolation,
	CodeAlreadyStarted:         KindConflict,
	CodeBillingModeMismatch:    KindInvariantViolation,
	CodeToolClassMismatch:      KindInvariantViolation,
	CodeHostUnavailable:        KindConflict,
	CodeFingerprintMismatch:    KindConflict,
	CodeInvalidStateTransition: KindConflict,
	CodeBaseVersionConflict:    KindConflict,
	CodeComponentUsed:          KindConflict,
	CodeOperatorMismatch:       KindConflict,
	CodeFileNotReady:           KindConflict,
	CodeFileBlocked:            KindConflict,
	CodeTitleManual:            KindConflict,
	CodeResyncRequired:         KindGone,
	CodeMessageDeleted:         KindGone,
	CodeGone:                   KindGone,
	CodeVersionConflict:        KindPreconditionFailed,
	CodePayloadTooLarge:        KindTooLarge,
	CodeUnsupportedMediaType:   KindUnsupportedMediaType,
	CodeRangeNotSatisfiable:    KindRangeNotSatisfiable,
	CodeIdempotencyKeyReuse:    KindUnprocessable,
	CodePreconditionRequired:   KindPreconditionRequired,
	CodeRateLimited:            KindRateLimited,
	CodeInternalError:          KindInternal,
	CodeUnavailable:            KindUnavailable,
	CodeMaintenance:            KindUnavailable,
	CodeSetupRequired:          KindUnavailable,
}

func TestCodeKindsFollowBriefAndRulings(t *testing.T) {
	all := map[Code]bool{}
	for _, c := range AllCodes() {
		all[c] = true
		want, ok := wantKinds[c]
		if !ok {
			t.Errorf("code %q is in AllCodes but has no expected kind in wantKinds", c)
			continue
		}
		if got := c.Kind(); got != want {
			t.Errorf("%q.Kind() = %s, want %s", c, got, want)
		}
	}
	for c := range wantKinds {
		if !all[c] {
			t.Errorf("code %q has an expected kind in wantKinds but is not in AllCodes", c)
		}
	}
}

// declaredCodes parses the non-test Go files of this package and returns the string value of
// every constant declared with type Code, so a constant without a row in codeTable shows up.
func declaredCodes(t *testing.T) map[Code]string {
	t.Helper()
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatalf("list package files: %v", err)
	}
	fset := token.NewFileSet()
	declared := map[Code]string{}
	for _, name := range files {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, name, nil, parser.SkipObjectResolution)
		if err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}
		for _, decl := range f.Decls {
			gen, ok := decl.(*ast.GenDecl)
			if !ok || gen.Tok != token.CONST {
				continue
			}
			for _, spec := range gen.Specs {
				vs, ok := spec.(*ast.ValueSpec)
				if !ok {
					continue
				}
				if typ, ok := vs.Type.(*ast.Ident); !ok || typ.Name != "Code" {
					continue
				}
				for i, ident := range vs.Names {
					if i >= len(vs.Values) {
						t.Fatalf("%s: Code constant %s has no value", fset.Position(ident.Pos()), ident.Name)
					}
					lit, ok := vs.Values[i].(*ast.BasicLit)
					if !ok || lit.Kind != token.STRING {
						t.Fatalf("%s: Code constant %s is not a string literal", fset.Position(ident.Pos()), ident.Name)
					}
					value, err := strconv.Unquote(lit.Value)
					if err != nil {
						t.Fatalf("%s: unquote %s: %v", fset.Position(lit.Pos()), lit.Value, err)
					}
					if prev, dup := declared[Code(value)]; dup {
						t.Errorf("Code constants %s and %s share the value %q", prev, ident.Name, value)
					}
					declared[Code(value)] = ident.Name
				}
			}
		}
	}
	if len(declared) == 0 {
		t.Fatal("no constant of type Code found in the package files")
	}
	return declared
}

func TestEveryCodeConstantIsInTable(t *testing.T) {
	declared := declaredCodes(t)
	all := map[Code]bool{}
	for _, c := range AllCodes() {
		all[c] = true
		if _, ok := declared[c]; !ok {
			t.Errorf("code %q is in codeTable but no Code constant declares it", c)
		}
	}
	for c, name := range declared {
		if !all[c] {
			t.Errorf("Code constant %s = %q has no row in codeTable", name, c)
		}
	}
}

func TestAllCodesReturnsCopy(t *testing.T) {
	first := AllCodes()
	if len(first) == 0 {
		t.Fatal("AllCodes is empty")
	}
	want := first[0]
	first[0] = "tampered"
	if got := AllCodes()[0]; got != want {
		t.Errorf("AllCodes()[0] = %q after changing an earlier result, want %q", got, want)
	}
}

func mustPanic(t *testing.T, name string, f func()) {
	t.Helper()
	defer func() {
		if recover() == nil {
			t.Errorf("%s did not panic", name)
		}
	}()
	f()
}

func TestNewPanicsOnUnknownCode(t *testing.T) {
	for _, c := range []Code{"", "no_such_code", "Version_Conflict"} {
		mustPanic(t, fmt.Sprintf("New(%q)", c), func() { _ = New(c, "detail") })
		mustPanic(t, fmt.Sprintf("Wrap(%q)", c), func() { _ = Wrap(c, io.EOF, "detail") })
	}
}

func TestNewAndWrapCarryCodeAndKind(t *testing.T) {
	e := New(CodeVersionConflict, "stale version")
	if e.Code != CodeVersionConflict || e.Detail != "stale version" || e.Kind() != KindPreconditionFailed {
		t.Errorf("New = %+v, kind %s", e, e.Kind())
	}
	if e.Unwrap() != nil {
		t.Errorf("New: Unwrap() = %v, want nil", e.Unwrap())
	}
	w := Wrap(CodeUnavailable, io.EOF, "")
	if !errors.Is(w, io.EOF) {
		t.Error("errors.Is(Wrap(..., io.EOF), io.EOF) = false")
	}
	if w.Kind() != KindUnavailable {
		t.Errorf("Wrap kind = %s, want unavailable", w.Kind())
	}
	n := Wrap(CodeConflict, nil, "taken")
	if n.Unwrap() != nil {
		t.Errorf("Wrap with nil error: Unwrap() = %v, want nil", n.Unwrap())
	}
	if got := n.Error(); got != "conflict: taken" {
		t.Errorf("Wrap with nil error: Error() = %q", got)
	}
}

func TestSetters(t *testing.T) {
	e := New(CodeValidationFailed, "bad input")
	f1 := FieldError{Pointer: "/name", Code: "required", Detail: "name is missing"}
	f2 := FieldError{Pointer: "/email", Code: "format", Detail: "not an address at example.com"}
	f3 := FieldError{Pointer: "/age", Code: "range"}
	current := map[string]int{"version": 7}
	chain := []struct {
		name string
		got  *Error
	}{
		{"WithReason", e.WithReason("channel_archived")},
		{"WithHint", e.WithHint("switch to organization first")},
		{"WithCause", e.WithCause(CauseDBPoolExhausted)},
		{"WithFields", e.WithFields(f1, f2)},
		{"WithFields again", e.WithFields(f3)},
		{"WithCurrent", e.WithCurrent(current)},
		{"WithRetryAfter", e.WithRetryAfter(30 * time.Second)},
	}
	for _, c := range chain {
		if c.got != e {
			t.Errorf("%s returned another pointer", c.name)
		}
	}
	if e.Reason != "channel_archived" || e.Hint != "switch to organization first" || e.Cause != CauseDBPoolExhausted {
		t.Errorf("reason/hint/cause = %q/%q/%q", e.Reason, e.Hint, e.Cause)
	}
	if len(e.Fields) != 3 || e.Fields[0] != f1 || e.Fields[1] != f2 || e.Fields[2] != f3 {
		t.Errorf("Fields = %+v, want appended [f1 f2 f3]", e.Fields)
	}
	if got, ok := e.Current.(map[string]int); !ok || got["version"] != 7 {
		t.Errorf("Current = %#v", e.Current)
	}
	if e.RetryAfter != 30*time.Second {
		t.Errorf("RetryAfter = %v", e.RetryAfter)
	}
}

func TestCauseConstants(t *testing.T) {
	tests := map[string]string{
		CauseAuthzNotChecked:       "authz_not_checked",
		CauseUnknownAction:         "unknown_action",
		CauseDBPoolExhausted:       "db_pool_exhausted",
		CauseShuttingDown:          "shutting_down",
		CauseDeadlineExceeded:      "deadline_exceeded",
		CauseDependencyUnavailable: "dependency_unavailable",
	}
	// A map literal with two equal constant keys does not compile, so the causes are distinct.
	for got, want := range tests {
		if got != want {
			t.Errorf("cause %q, want %q", got, want)
		}
	}
}

func TestAs(t *testing.T) {
	e := New(CodeNotFound, "")
	var typedNil *Error
	tests := []struct {
		name   string
		err    error
		want   *Error
		wantOK bool
	}{
		{"nil", nil, nil, false},
		{"plain", errors.New("boom"), nil, false},
		{"direct", e, e, true},
		{"wrapped", fmt.Errorf("load: %w", e), e, true},
		{"joined", errors.Join(errors.New("boom"), e), e, true},
		{"typed nil", typedNil, nil, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := As(tt.err)
			if got != tt.want || ok != tt.wantOK {
				t.Errorf("As = (%p, %v), want (%p, %v)", got, ok, tt.want, tt.wantOK)
			}
		})
	}
}

func TestKindOf(t *testing.T) {
	var typedNil *Error
	tests := []struct {
		name string
		err  error
		want Kind
	}{
		{"nil", nil, KindNone},
		{"plain error", errors.New("boom"), KindInternal},
		{"direct", New(CodeLastOwner, ""), KindInvariantViolation},
		{"wrapped with %w", fmt.Errorf("remove member: %w", New(CodeVersionConflict, "")), KindPreconditionFailed},
		{"twice wrapped", fmt.Errorf("a: %w", fmt.Errorf("b: %w", New(CodeRateLimited, ""))), KindRateLimited},
		{"errors.Join plain first", errors.Join(errors.New("boom"), New(CodeNotFound, "")), KindNotFound},
		{"typed nil *Error", typedNil, KindInternal},
		{"typed nil *Error joined", errors.Join(errors.New("boom"), typedNil), KindInternal},
		{"wrapped typed nil", fmt.Errorf("x: %w", typedNil), KindInternal},
		{"zero *Error", &Error{}, KindInternal},
		{"unknown code literal", &Error{Code: "bogus"}, KindInternal},
		{"wrapped unknown code", fmt.Errorf("x: %w", &Error{Code: "bogus"}), KindInternal},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := KindOf(tt.err); got != tt.want {
				t.Errorf("KindOf = %s, want %s", got, tt.want)
			}
		})
	}
}

func TestErrorKindNeverNone(t *testing.T) {
	e := New(CodeNotFound, "")
	e.Code = "bogus"
	if got := e.Kind(); got != KindInternal {
		t.Errorf("Kind() after setting an unknown code = %s, want internal", got)
	}
	if got := (&Error{}).Kind(); got != KindInternal {
		t.Errorf("zero Error Kind() = %s, want internal", got)
	}
	if got := (&Error{}).Kind().Status(); got != 500 {
		t.Errorf("zero Error status = %d, want 500", got)
	}
}

func TestPublicCodeStaysInClosedList(t *testing.T) {
	var typedNil *Error
	changed := New(CodeNotFound, "gone away")
	changed.Code = "bogus"
	tests := []struct {
		name      string
		err       *Error
		wantCode  Code
		wantError string
	}{
		{"known code", New(CodeNotFound, "no such channel"), CodeNotFound, "not_found: no such channel"},
		{"known code without detail", New(CodeRateLimited, ""), CodeRateLimited, "rate_limited"},
		{"unknown code literal", &Error{Code: "bogus", Detail: "d"}, CodeInternalError, "internal_error: d"},
		{"code changed after New", changed, CodeInternalError, "internal_error: gone away"},
		{"zero Error", &Error{}, CodeInternalError, "internal_error"},
		{"nil *Error", typedNil, CodeInternalError, "<nil>"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.err.PublicCode()
			if got != tt.wantCode {
				t.Errorf("PublicCode() = %q, want %q", got, tt.wantCode)
			}
			if got.Kind() != tt.err.Kind() {
				t.Errorf("PublicCode() %q has kind %s, Kind() = %s", got, got.Kind(), tt.err.Kind())
			}
			if msg := tt.err.Error(); msg != tt.wantError {
				t.Errorf("Error() = %q, want %q", msg, tt.wantError)
			}
		})
	}
}

func TestLogValueKeepsRawCode(t *testing.T) {
	var buf bytes.Buffer
	slog.New(slog.NewTextHandler(&buf, nil)).Info("request failed", "err", &Error{Code: "bogus"})
	out := buf.String()
	for _, want := range []string{"err.code=bogus", "err.kind=internal"} {
		if !strings.Contains(out, want) {
			t.Errorf("log line %q lacks %q", out, want)
		}
	}
}

func TestNilErrorIsSafe(t *testing.T) {
	var e *Error
	if got := e.Kind(); got != KindInternal {
		t.Errorf("nil Kind() = %s, want internal", got)
	}
	if e.Unwrap() != nil {
		t.Error("nil Unwrap() != nil")
	}
	if got := e.Error(); got != "<nil>" {
		t.Errorf("nil Error() = %q, want <nil>", got)
	}
	if got := e.LogValue(); got.Kind() != slog.KindGroup || len(got.Group()) != 0 {
		t.Errorf("nil LogValue() = %v, want an empty group", got)
	}
}

func TestErrorStringHidesCauseAndWrapped(t *testing.T) {
	inner := errors.New("pq: connection refused to db.example.com")
	e := Wrap(CodeUnavailable, inner, "database not reachable").
		WithCause(CauseDBPoolExhausted).
		WithReason("secret_reason").
		WithHint("secret_hint")
	if got, want := e.Error(), "unavailable: database not reachable"; got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
	if got := New(CodeNotFound, "").Error(); got != "not_found" {
		t.Errorf("Error() without detail = %q, want not_found", got)
	}
	// Without a detail Error() takes another branch; it must leak as little as the one above.
	bare := Wrap(CodeUnavailable, inner, "").
		WithCause(CauseDBPoolExhausted).
		WithReason("secret_reason").
		WithHint("secret_hint")
	if got, want := bare.Error(), "unavailable"; got != want {
		t.Errorf("Error() without detail = %q, want %q", got, want)
	}
	for _, leaky := range []*Error{e, bare} {
		outer := fmt.Errorf("handler: %w", leaky).Error()
		for _, secret := range []string{CauseDBPoolExhausted, "secret_reason", "secret_hint", "connection refused", "db.example.com"} {
			if strings.Contains(leaky.Error(), secret) || strings.Contains(outer, secret) {
				t.Errorf("error text %q / %q contains %q", leaky.Error(), outer, secret)
			}
		}
	}
}

func TestJSONOmitsCause(t *testing.T) {
	// Nothing encodes an *Error directly today; if a writer ever does, the cause must stay out.
	e := Wrap(CodeUnavailable, errors.New("pq: connection refused"), "database not reachable").
		WithCause(CauseDBPoolExhausted)
	data, err := json.Marshal(e)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	for _, secret := range []string{`"Cause"`, CauseDBPoolExhausted, "connection refused"} {
		if bytes.Contains(data, []byte(secret)) {
			t.Errorf("json.Marshal = %s, contains %s", data, secret)
		}
	}
	if !bytes.Contains(data, []byte(`"Detail":"database not reachable"`)) {
		t.Errorf("json.Marshal = %s, want the detail as a sanity check", data)
	}
}

func TestJSONWritesPublicCode(t *testing.T) {
	// Like Error() and PublicCode(), JSON never carries a code outside the closed list of 06,
	// whether an encoder meets an *Error, an Error value or either one behind an interface.
	changed := New(CodeNotFound, "gone away")
	changed.Code = "bogus"
	tests := []struct {
		name string
		v    any
		want string
	}{
		{"unknown code literal", &Error{Code: "bogus", Detail: "d"}, `"Code":"internal_error"`},
		{"code changed after New", changed, `"Code":"internal_error"`},
		{"Error value", Error{Code: "bogus"}, `"Code":"internal_error"`},
		{"Error value in a field", struct{ E any }{E: Error{Code: "bogus"}}, `"Code":"internal_error"`},
		{"zero Error", &Error{}, `"Code":"internal_error"`},
		{"known code", New(CodeNotFound, "no such channel"), `"Code":"not_found","Detail":"no such channel"`},
		{"nil *Error", (*Error)(nil), `null`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.v)
			if err != nil {
				t.Fatalf("json.Marshal: %v", err)
			}
			if !bytes.Contains(data, []byte(tt.want)) || bytes.Contains(data, []byte("bogus")) {
				t.Errorf("json.Marshal = %s, want it to contain %s and no raw code", data, tt.want)
			}
		})
	}
}

func TestLogValueContainsCause(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, nil))
	inner := errors.New("pq: connection refused")
	e := Wrap(CodeUnavailable, inner, "database not reachable").
		WithCause(CauseDBPoolExhausted).
		WithReason("pool").
		WithHint("retry later")
	logger.Info("request failed", "err", e)
	out := buf.String()
	for _, want := range []string{
		"err.code=unavailable",
		"err.kind=unavailable",
		`err.detail="database not reachable"`,
		"err.reason=pool",
		`err.hint="retry later"`,
		"err.cause=db_pool_exhausted",
		`err.error="pq: connection refused"`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("log line %q lacks %q", out, want)
		}
	}

	buf.Reset()
	logger.Info("request failed", "err", New(CodeNotFound, ""))
	out = buf.String()
	if !strings.Contains(out, "err.code=not_found") || !strings.Contains(out, "err.kind=not_found") {
		t.Errorf("log line %q lacks code or kind", out)
	}
	for _, unset := range []string{"err.detail", "err.reason", "err.hint", "err.cause", "err.error"} {
		if strings.Contains(out, unset) {
			t.Errorf("log line %q contains unset attribute %q", out, unset)
		}
	}
}

// TestFingerprintMismatch_AttemptsRemaining (M2-41): the error is fingerprint_mismatch and
// carries the member attempts_remaining, the value 0 included; it never names a fingerprint.
func TestFingerprintMismatch_AttemptsRemaining(t *testing.T) {
	t.Parallel()
	for _, n := range []int{0, 1, 2} {
		err := FingerprintMismatch(n)
		e, ok := As(err)
		if !ok || e.Code != CodeFingerprintMismatch || KindOf(err) != KindConflict {
			t.Fatalf("%d: %v", n, err)
		}
		var ext ProblemExtender
		if !errors.As(err, &ext) || ext.ProblemExtensions()["attempts_remaining"] != n {
			t.Fatalf("%d: extensions %v", n, ext)
		}
	}
}
