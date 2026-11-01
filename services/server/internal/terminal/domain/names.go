// SPDX-License-Identifier: Apache-2.0

package domain

import (
	"errors"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/textcheck"
)

// The field error codes of this package besides the textcheck rules (06 section 5).
const (
	// FieldRequired marks a missing or blank value.
	FieldRequired = "required"
	// FieldTooLong marks a value above its configured length.
	FieldTooLong = "too_long"
	// FieldPatternMismatch marks a value outside its pattern.
	FieldPatternMismatch = "pattern_mismatch"
	// FieldInvalid marks a value the rule refuses for another reason.
	FieldInvalid = "invalid"
	// FieldUnknownUser marks a person id that is unknown, not a person or deleted (M2-39).
	FieldUnknownUser = "unknown_user"
	// FieldMutuallyExclusive marks two members that must not come together (M2-39).
	FieldMutuallyExclusive = "mutually_exclusive"
	// FieldSessionUserNotOnHost marks a session user name the host does not report (24 2.2).
	FieldSessionUserNotOnHost = "session_user_not_on_host"
	// FieldOwnerAccountOptIn marks an assignment of the owner account without owner_account = true,
	// FieldOwnerAccountNotOnHost owner_account = true for a name the host does not report as its
	// owner account (ADR 0073, D-OA-6).
	FieldOwnerAccountOptIn     = "owner_account_opt_in_required"
	FieldOwnerAccountNotOnHost = "owner_account_not_on_host"
)

// osUserPattern is the check of host_session_users.os_user (03 section 4.14): letters, digits,
// "_", "." and "-", not starting with "." or "-", so never a path.
var osUserPattern = regexp.MustCompile(`^[A-Za-z0-9_][A-Za-z0-9_.-]*$`)

// NewHostName validates the name of a host: trimmed, not blank, a display text under SEC-IN-4
// (no control or bidi character) and at most maxChars characters (host.name_max_chars). It
// returns the trimmed name. Errors: validation_failed on /name with required, too_long or the
// textcheck rule.
func NewHostName(s string, maxChars int) (string, error) {
	return displayText("/name", "name", s, maxChars)
}

// CheckReason validates the mandatory reason of a host.manage write like a host name, at most
// maxChars characters (audit.reason_max_chars), and returns it trimmed. Errors: validation_failed
// on /reason.
func CheckReason(s string, maxChars int) (string, error) {
	return displayText("/reason", "reason", s, maxChars)
}

// NewOSUser validates the name of a session user at pointer: not empty, the pattern of 03 and at
// most maxChars characters (host.os_user_max_chars). Errors: validation_failed at pointer with
// required, pattern_mismatch or too_long.
func NewOSUser(s string, maxChars int, pointer string) (string, error) {
	switch {
	case s == "":
		return "", FieldError(pointer, FieldRequired, "the session user is required")
	case !osUserPattern.MatchString(s):
		return "", FieldError(pointer, FieldPatternMismatch, "the session user is no name of an operating system user")
	case utf8.RuneCountInString(s) > maxChars:
		return "", FieldError(pointer, FieldTooLong, "the session user is too long")
	}
	return s, nil
}

// displayText trims s and checks it: not blank, SEC-IN-4, at most maxChars characters. The error
// never repeats the input.
func displayText(pointer, what, s string, maxChars int) (string, error) {
	s = strings.TrimSpace(s)
	if err := textcheck.DisplayText(s); err != nil {
		var te *textcheck.Error
		if errors.As(err, &te) {
			return "", FieldError(pointer, string(te.Rule), "the "+what+" contains a forbidden character")
		}
		return "", FieldError(pointer, FieldInvalid, "the "+what+" is not valid")
	}
	switch {
	case s == "":
		return "", FieldError(pointer, FieldRequired, "the "+what+" is required")
	case utf8.RuneCountInString(s) > maxChars:
		return "", FieldError(pointer, FieldTooLong, "the "+what+" is too long")
	}
	return s, nil
}

// FieldError returns validation_failed with one field error.
func FieldError(pointer, code, detail string) *errs.Error {
	return errs.New(errs.CodeValidationFailed, "").WithFields(errs.FieldError{Pointer: pointer, Code: code, Detail: detail})
}
