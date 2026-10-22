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

// The field error codes of this package besides the textcheck rules.
const (
	// FieldRequired marks a missing or blank value.
	FieldRequired = "required"
	// FieldInvalid marks a value of the wrong type or outside its enumeration.
	FieldInvalid = "invalid"
	// FieldPattern marks a value outside its pattern.
	FieldPattern = "pattern"
	// FieldUnknown marks a key the schema does not know.
	FieldUnknown = "unknown_key"
	// FieldNotAllowed marks a value the rule forbids in this combination.
	FieldNotAllowed = "not_allowed"
)

// adapterKindPattern is the rule of agents.adapter_kind (docs/DECISIONS.md M1-51): a lower-case
// identifier like hermes, code_runner or custom. Go's regexp runs in linear time (SEC-IN-6).
var adapterKindPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]*$`)

// CheckAdapterKind validates the adapter kind of an agent. Errors: validation_failed on
// /adapter_kind with code required or pattern.
func CheckAdapterKind(s string) error {
	switch {
	case s == "":
		return fieldError("/adapter_kind", FieldRequired, "the adapter kind is required")
	case !adapterKindPattern.MatchString(s):
		return fieldError("/adapter_kind", FieldPattern, "the adapter kind is not a lower-case identifier")
	}
	return nil
}

// CheckDescription validates the description of an agent: it may be empty, and it is a display
// text under SEC-IN-4 (no control or bidi characters). Errors: validation_failed on /description
// with the textcheck rule as code.
func CheckDescription(s string) error {
	return displayText("/description", "description", s, false)
}

// CheckReason validates the reason of an administrative action: not blank and free of control
// and bidi characters (SEC-IN-4, SEC-AUD-5). Errors: validation_failed on /reason.
func CheckReason(s string) error {
	return displayText("/reason", "reason", s, true)
}

// FieldTooLong marks a value above its configured length (the code of platform/httpserver).
const FieldTooLong = "too_long"

// CheckReasonMax is CheckReason plus a length of at most maxChars characters
// (audit.reason_max_chars, SEC-AUD-5; M2-21 design note 2). Errors: validation_failed on /reason
// with code required, a textcheck rule or too_long.
func CheckReasonMax(s string, maxChars int) error {
	if err := CheckReason(s); err != nil {
		return err
	}
	if utf8.RuneCountInString(strings.TrimSpace(s)) > maxChars {
		return fieldError("/reason", FieldTooLong, "the reason is longer than audit.reason_max_chars")
	}
	return nil
}

// displayText checks s under SEC-IN-4 and, with required, that it is not blank. The error never
// repeats the input.
func displayText(pointer, what, s string, required bool) error {
	if err := textcheck.DisplayText(s); err != nil {
		var te *textcheck.Error
		if errors.As(err, &te) {
			return fieldError(pointer, string(te.Rule), "the "+what+" contains a forbidden character")
		}
		return fieldError(pointer, FieldInvalid, "the "+what+" is not valid")
	}
	if required && strings.TrimSpace(s) == "" {
		return fieldError(pointer, FieldRequired, "the "+what+" is required")
	}
	return nil
}

// fieldError returns validation_failed with one field error.
func fieldError(pointer, code, detail string) *errs.Error {
	return errs.New(errs.CodeValidationFailed, detail).WithFields(errs.FieldError{Pointer: pointer, Code: code, Detail: detail})
}
