// SPDX-License-Identifier: Apache-2.0

package domain

import "unicode/utf8"

// Field error codes of a rejected password (setMyLocalCredential, pointer /password).
const (
	// PasswordTooShort is a password below auth.password_min_length code points.
	PasswordTooShort = "too_short"
	// PasswordCommon is a password on the local list of common passwords.
	PasswordCommon = "common_password"
)

// CheckPassword applies the password rules of docs/spec/08-identitaet.md section 2.5 to nfc, the
// password already normalised to NFC (the normalisation needs golang.org/x/text and lives in the
// adapter passwordpolicy): at least minLength Unicode code points, and not listed among the common
// passwords (listed is the answer of that list). There are no composition rules. It returns the
// field error code, PasswordTooShort before PasswordCommon, or "" for an acceptable password. The
// password itself is never part of a result.
func CheckPassword(nfc string, minLength int, listed bool) string {
	if utf8.RuneCountInString(nfc) < minLength {
		return PasswordTooShort
	}
	if listed {
		return PasswordCommon
	}
	return ""
}
