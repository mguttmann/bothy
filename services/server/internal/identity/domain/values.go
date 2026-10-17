// SPDX-License-Identifier: Apache-2.0

package domain

import (
	"errors"
	"regexp"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/textcheck"
)

// Kind is the kind of a principal, the column principals.kind. It never changes after creation.
type Kind string

const (
	// KindUser is a person; only this kind has a row in users.
	KindUser Kind = "user"
	// KindAgent is an agent.
	KindAgent Kind = "agent"
	// KindApp is an app.
	KindApp Kind = "app"
)

// Valid reports whether k is a value of the check on principals.kind.
func (k Kind) Valid() bool {
	return k == KindUser || k == KindAgent || k == KindApp
}

// Status is the lifecycle state of a principal, the column principals.status.
type Status string

const (
	// StatusPending is a provisioned person waiting for approval (docs/spec/08-identity.md).
	StatusPending Status = "pending"
	// StatusActive is the normal state and the column default.
	StatusActive Status = "active"
	// StatusDisabled is a principal without rights (SR7).
	StatusDisabled Status = "disabled"
	// StatusDeleted is a deleted principal; its row stays for authorship (03 "Principles" item 7).
	StatusDeleted Status = "deleted"
)

// Valid reports whether s is a value of the check on principals.status.
func (s Status) Valid() bool {
	return s == StatusPending || s == StatusActive || s == StatusDisabled || s == StatusDeleted
}

// The field error codes of this package besides the textcheck rules.
const (
	// FieldRequired marks an empty or blank value.
	FieldRequired = "required"
	// FieldTooLong marks a value above its configured length.
	FieldTooLong = "too_long"
	// FieldPattern marks a handle outside ^[a-z0-9][a-z0-9._-]*$.
	FieldPattern = "pattern"
)

// The JSON pointers of the fields this package validates (docs/api/openapi.yaml UserCreate).
const (
	pointerHandle      = "/handle"
	pointerDisplayName = "/display_name"
	pointerReason      = "/reason"
)

// handlePattern is the check of principals.handle and the pattern of UserCreate.handle. Go's
// regexp runs in linear time (SEC-IN-6).
var handlePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]*$`)

// Handle is a validated handle for mentions: lower case only, at most
// principal.handle_max_chars characters. The zero value is no valid handle.
type Handle struct{ s string }

// NewHandle validates s against ^[a-z0-9][a-z0-9._-]*$ and the length 1 to maxChars
// (principal.handle_max_chars). Upper case is rejected, not folded, because the database check
// and the API pattern accept lower case only.
//
// Errors: validation_failed on /handle with code required (empty), too_long or pattern.
func NewHandle(s string, maxChars int) (Handle, error) {
	switch {
	case s == "":
		return Handle{}, fieldError(pointerHandle, FieldRequired, "the handle is required")
	case utf8.RuneCountInString(s) > maxChars:
		return Handle{}, fieldError(pointerHandle, FieldTooLong, "the handle has more than "+strconv.Itoa(maxChars)+" characters")
	case !handlePattern.MatchString(s):
		return Handle{}, fieldError(pointerHandle, FieldPattern,
			"the handle must start with a lower-case letter or digit and contain only a-z, 0-9, '.', '_' and '-'")
	}
	return Handle{s: s}, nil
}

// HandleFromStore wraps a handle read back from the database, which its check already holds.
// Only the repository of this module uses it.
func HandleFromStore(s string) Handle { return Handle{s: s} }

// String returns the handle.
func (h Handle) String() string { return h.s }

// IsZero reports whether h is the zero value.
func (h Handle) IsZero() bool { return h.s == "" }

// DisplayName is a validated display name: no control or bidi characters (SEC-IN-4), not blank,
// at most principal.display_name_max_chars runes. The zero value is no valid name.
type DisplayName struct{ s string }

// NewDisplayName validates s with textcheck.DisplayText, requires at least one rune that is
// neither Unicode white space nor an invisible format character (category Cf) and at most maxChars runes (principal.display_name_max_chars). The text is
// kept as given, without trimming.
//
// Errors: validation_failed on /display_name with code invalid_utf8, control_character,
// bidi_control, required or too_long.
func NewDisplayName(s string, maxChars int) (DisplayName, error) {
	if err := displayText(pointerDisplayName, "display name", s); err != nil {
		return DisplayName{}, err
	}
	if utf8.RuneCountInString(s) > maxChars {
		return DisplayName{}, fieldError(pointerDisplayName, FieldTooLong,
			"the display name has more than "+strconv.Itoa(maxChars)+" characters")
	}
	return DisplayName{s: s}, nil
}

// DisplayNameFromStore wraps a display name read back from the database. Only the repository of
// this module uses it.
func DisplayNameFromStore(s string) DisplayName { return DisplayName{s: s} }

// String returns the display name.
func (d DisplayName) String() string { return d.s }

// IsZero reports whether d is the zero value.
func (d DisplayName) IsZero() bool { return d.s == "" }

// displayText checks a display text for SEC-IN-4 and blankness and returns the field error of
// pointer; what names the field in the detail.
func displayText(pointer, what, s string) error {
	if err := textcheck.DisplayText(s); err != nil {
		var te *textcheck.Error
		if errors.As(err, &te) {
			return fieldError(pointer, string(te.Rule),
				"the "+what+" contains a forbidden character at position "+strconv.Itoa(te.Offset))
		}
		return fieldError(pointer, string(textcheck.RuleInvalidUTF8), "the "+what+" is not valid text")
	}
	if strings.TrimFunc(s, invisible) == "" {
		return fieldError(pointer, FieldRequired, "the "+what+" must not be empty or blank")
	}
	return nil
}

// invisible reports runes that show nothing on their own: Unicode white space and format
// characters (category Cf, such as U+200B, U+200D and U+FEFF). A text of only such runes would
// make a person unidentifiable in mentions, member lists and the audit view.
func invisible(r rune) bool {
	return unicode.IsSpace(r) || unicode.Is(unicode.Cf, r)
}

// fieldError is the validation_failed error with one field error.
func fieldError(pointer, code, detail string) *errs.Error {
	return errs.New(errs.CodeValidationFailed, detail).
		WithFields(errs.FieldError{Pointer: pointer, Code: code, Detail: detail})
}

// CheckPasskeyName validates the name a person gives a passkey (03 passkey_credentials.name,
// M1-40) with the rules of a display name: SEC-IN-4, not blank, at most maxChars runes
// (principal.display_name_max_chars). pointer is the field of the request (/name or
// /passkey_name). Errors: validation_failed on pointer.
func CheckPasskeyName(pointer, s string, maxChars int) error {
	if err := displayText(pointer, "passkey name", s); err != nil {
		return err
	}
	if utf8.RuneCountInString(s) > maxChars {
		return fieldError(pointer, FieldTooLong, "the passkey name has more than "+strconv.Itoa(maxChars)+" characters")
	}
	return nil
}
