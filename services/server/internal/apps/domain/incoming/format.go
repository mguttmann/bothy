// SPDX-License-Identifier: Apache-2.0

package incoming

import (
	"slices"

	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
)

// Format is the shape of a request body (11 section 8.2). It is decided per request from the
// body, because incoming_webhooks has no format column (M2-27 design note 4).
type Format string

// The two built-in formats of R-EX-08.
const (
	// FormatNative is the standard: Markdown with optional thread, title and client message ID.
	FormatNative Format = "native"
	// FormatAlertmanager is the unchanged webhook payload of Prometheus Alertmanager.
	FormatAlertmanager Format = "alertmanager"
)

// The codes of errors[].code this package uses. required, empty, not_uuid_v7 and invalid_type
// are the vocabulary of M1-18 and M1-56; the other three are the rulings of M2-27.
const (
	CodeFormatMixed          = "format_mixed"
	CodeFormatDisabled       = "format_disabled"
	CodeRequired             = "required"
	CodeEmpty                = "empty"
	CodeNotUUIDv7            = "not_uuid_v7"
	CodeInvalidType          = "invalid_type"
	CodeRequiredForForumPost = "required_for_forum_post"
)

// The schema codes of M1-18 that ParseAlertmanager uses for checks the HTTP layer already makes.
const (
	codeInvalidEnum = "invalid_enum"
	codeTooFewItems = "too_few_items"
	codeTooSmall    = "too_small"
)

// NativeMembers returns the top-level member names of a native body in schema order:
// content_md, title, thread_id, client_msg_id. The result is a new slice on every call.
func NativeMembers() []string {
	return []string{"content_md", "title", "thread_id", "client_msg_id"}
}

// AlertmanagerMembers returns the top-level member names of an Alertmanager payload in schema
// order. The result is a new slice on every call.
func AlertmanagerMembers() []string {
	return []string{"version", "groupKey", "truncatedAlerts", "status", "receiver", "groupLabels",
		"commonLabels", "commonAnnotations", "externalURL", "alerts"}
}

// fieldError is a validation_failed with one field error; detail names the rule, never a value.
func fieldError(pointer, code, detail string) *errs.Error {
	return errs.New(errs.CodeValidationFailed, detail).WithFields(errs.FieldError{Pointer: pointer, Code: code, Detail: detail})
}

// Classify decides the format from the names of the top-level members present in the body
// (M2-27 design note 4a). present may be in any order; callers pass them in the order of
// NativeMembers then AlertmanagerMembers so the error names the first native member. Names of
// neither list are ignored. A body with at least one Alertmanager member is alertmanager; it must
// contain no native member, else the error is 400 validation_failed with the pointer of the first
// native member in NativeMembers order and code format_mixed. Every other body, the empty one
// included, is native (11 section 8.2: native is the standard).
func Classify(present []string) (Format, error) {
	alertmanager := slices.ContainsFunc(present, func(m string) bool { return slices.Contains(AlertmanagerMembers(), m) })
	if !alertmanager {
		return FormatNative, nil
	}
	for _, m := range NativeMembers() {
		if slices.Contains(present, m) {
			return "", fieldError("/"+m, CodeFormatMixed, "The body mixes members of the formats native and alertmanager (11 section 8.2).")
		}
	}
	return FormatAlertmanager, nil
}

// CheckEnabled refuses a format that enabled, the value of webhook_in.formats_enabled, does not
// list (M2-27 design note 4b): 400 validation_failed with an empty pointer and code
// format_disabled. It returns nil for an enabled format.
func CheckEnabled(f Format, enabled []string) error {
	if slices.Contains(enabled, string(f)) {
		return nil
	}
	return fieldError("", CodeFormatDisabled, "This format is switched off by webhook_in.formats_enabled.")
}
