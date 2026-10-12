// SPDX-License-Identifier: Apache-2.0

package secrets

import (
	"errors"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Sentinel errors of the provider and its drivers. ParseRef, the file and env drivers and
// Provider.Get return a *RefError that wraps one of them or, for a canceled call, the context
// error. Provider.Get passes the error of a caller-supplied driver (Options.Drivers) on unchanged,
// and NewProvider returns plain configuration errors. Callers branch with errors.Is and report
// with ErrorCode, which maps any error to one of five codes.
var (
	// ErrInvalidRef: the reference does not follow the grammar "<scheme>:<path>" of its scheme.
	ErrInvalidRef = errors.New("invalid reference")
	// ErrUnsupportedScheme: the scheme is known (azkv, awssm) or well formed, but no driver of
	// this server reads it.
	ErrUnsupportedScheme = errors.New("unsupported scheme")
	// ErrNotFound: the file or variable does not exist or is empty. An empty secret is never
	// valid, so it fails as early as a missing one.
	ErrNotFound = errors.New("secret not found")
	// ErrInsecureFile: the file is not a regular file, has a mode other than 0400 or 0600, or
	// belongs to another user (SEC-SEC-1).
	ErrInsecureFile = errors.New("insecure secret file")
	// ErrReadFailed: the secret exists but could not be read (permissions, I/O, larger than the
	// size limit, replaced while it was read).
	ErrReadFailed = errors.New("secret could not be read")
)

// RefError is the error type of the package: which reference failed, the sentinel that says how,
// and a detail that explains it. No field ever holds a secret value or file content
// (SEC-SEC-3, SEC-LOG-1).
type RefError struct {
	// Ref is the reference in the form "<scheme>:<path>", or empty when the reference is
	// invalid or its scheme is not file or env: an invalid reference is never repeated, because a
	// secret value may stand where a reference belongs (same rule as the configuration loader),
	// and this package checks the grammar of file and env only.
	Ref string
	// Err is one of the sentinel errors, or the context error for a canceled call.
	Err error
	// Detail says what was observed, for example "mode 0644, want 0400 or 0600"; it may be
	// empty.
	Detail string
}

// Error renders "secrets: <ref>: <err>: <detail>", leaving out the parts that are empty. A
// reference that would not read back as one unambiguous line is Go-quoted.
func (e *RefError) Error() string {
	var b strings.Builder
	b.WriteString("secrets: ")
	if e.Ref != "" {
		b.WriteString(printable(e.Ref))
		b.WriteString(": ")
	}
	if e.Err != nil {
		b.WriteString(e.Err.Error())
	} else {
		b.WriteString("error")
	}
	if e.Detail != "" {
		b.WriteString(": ")
		b.WriteString(e.Detail)
	}
	return b.String()
}

// Unwrap returns the sentinel, so errors.Is(err, ErrNotFound) and the like work.
func (e *RefError) Unwrap() error { return e.Err }

// ErrorCode maps err to the code that logs, metrics and `bothy config check --secrets` show
// instead of a message: "" for nil, else "invalid_ref", "unsupported_scheme", "not_found",
// "insecure_file" or "read_failed". Every other error, including a canceled context, is
// "read_failed", so a caller always has one of five stable values.
func ErrorCode(err error) string {
	switch {
	case err == nil:
		return ""
	case errors.Is(err, ErrInvalidRef):
		return "invalid_ref"
	case errors.Is(err, ErrUnsupportedScheme):
		return "unsupported_scheme"
	case errors.Is(err, ErrNotFound):
		return "not_found"
	case errors.Is(err, ErrInsecureFile):
		return "insecure_file"
	default:
		return "read_failed"
	}
}

// printable returns text as it is, or Go-quoted when it would not read back as one unambiguous
// line: with invalid UTF-8 or a character that is not printable (a newline would forge a log
// line of its own), with leading or trailing white space, or starting with a quote. It is the
// rule of cmd/bothy for configuration values, repeated here because this package may not import
// the command.
func printable(text string) string {
	if !utf8.ValidString(text) || strings.TrimSpace(text) != text || strings.HasPrefix(text, `"`) ||
		strings.IndexFunc(text, func(r rune) bool { return !unicode.IsPrint(r) }) >= 0 {
		return strconv.Quote(text)
	}
	return text
}
