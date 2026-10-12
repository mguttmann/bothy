// SPDX-License-Identifier: Apache-2.0

package domain

import (
	"net/mail"
	"strconv"
	"time"
	_ "time/tzdata" // a host or image without zoneinfo still resolves every IANA name (M1-31)
	"unicode/utf8"

	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/textcheck"
)

// FieldInvalid marks an e-mail address, locale or time zone that is not well-formed.
const FieldInvalid = "invalid"

// The JSON pointers of the profile fields (docs/api/openapi.yaml UserCreate, UserUpdate,
// ProfileUpdate).
const (
	pointerEmail    = "/email"
	pointerLocale   = "/locale"
	pointerTimeZone = "/time_zone"
)

// CheckEmail validates an e-mail address for users.email: one plain address as RFC 5322 reads it
// (no display name, no surrounding space) and free of control and bidi characters (SEC-IN-4).
// The address is for display only; nothing is sent to it.
//
// Errors: validation_failed on /email with code invalid; the text never repeats the input.
func CheckEmail(s string) error {
	invalid := fieldError(pointerEmail, FieldInvalid, "the e-mail address is not one plain address")
	if s == "" || textcheck.DisplayText(s) != nil {
		return invalid
	}
	a, err := mail.ParseAddress(s)
	if err != nil || a.Name != "" || a.Address != s {
		return invalid
	}
	return nil
}

//go:generate go run ../../../tools/zonegen -out zones.gen.go

// CheckTimeZone validates an IANA time zone name for users.time_zone: the name must be one of
// zoneNames, the zones of the tzdata of the Go toolchain without "Factory" (tools/zonegen), and
// time.LoadLocation must load it (the embedded tzdata of this package when the host has none).
// The list makes the rule independent of the host: a case-insensitive file system or the other
// files of a system zoneinfo directory (posixrules, right/...) would otherwise pass names that
// another host refuses (red team M1-31). "Local", which names the zone of the server rather than
// of the person, is never in the list.
//
// Errors: validation_failed on /time_zone with code invalid.
func CheckTimeZone(s string) error {
	invalid := fieldError(pointerTimeZone, FieldInvalid, "the time zone is not an IANA time zone name")
	if !zoneNames[s] {
		return invalid
	}
	if _, err := time.LoadLocation(s); err != nil {
		return invalid
	}
	return nil
}

// LocaleMaxChars bounds a locale before it is parsed: 35 characters is the length RFC 5646
// section 4.4.1 asks every implementation to support, far above any tag a person needs.
const LocaleMaxChars = 35

// CheckLocaleLength refuses a locale above LocaleMaxChars characters, so a private-use tail of
// any length never reaches the parser, the directory or user.updated (red team M1-31).
//
// Errors: validation_failed on /locale with code too_long.
func CheckLocaleLength(s string) error {
	if utf8.RuneCountInString(s) > LocaleMaxChars {
		return fieldError(pointerLocale, FieldTooLong, "the locale has more than "+strconv.Itoa(LocaleMaxChars)+" characters")
	}
	return nil
}

// CheckSearchPrefix validates the search prefix q of the directory: valid UTF-8 without control
// or bidi characters (SEC-IN-4), so it can reach the database and never answers 500.
//
// Errors: validation_failed on #query/q with code invalid.
func CheckSearchPrefix(s string) error {
	if s == "" {
		return nil
	}
	if err := textcheck.DisplayText(s); err != nil {
		return fieldError("#query/q", FieldInvalid, "the search prefix contains a forbidden character")
	}
	return nil
}

// InvalidLocale is the error of a locale that is not a well-formed BCP 47 tag; the use cases
// parse the tag through their locale port and return it on a failure.
func InvalidLocale() error {
	return fieldError(pointerLocale, FieldInvalid, "the locale is not a well-formed BCP 47 language tag")
}

// CheckReason validates the reason of an administrative action (enable a person, grant or revoke
// the owner role): not blank and free of control and bidi characters (SEC-IN-4, SEC-AUD-5).
//
// Errors: validation_failed on /reason with code required or a textcheck rule.
func CheckReason(s string) error {
	return displayText(pointerReason, "reason", s)
}

// EnableTransition decides user.enable for a principal in status s (05 table 4.3, 08 section
// 3.1): pending and disabled become active (change true); an active principal stays as it is
// (change false, a call without effect, 06 section 2 "Ohne Wirkung").
//
// Errors: invalid_state_transition for deleted and any other status.
func EnableTransition(s Status) (change bool, err error) {
	switch s {
	case StatusPending, StatusDisabled:
		return true, nil
	case StatusActive:
		return false, nil
	case StatusDeleted:
	}
	return false, errs.New(errs.CodeInvalidStateTransition, "a deleted person cannot be enabled")
}
