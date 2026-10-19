// SPDX-License-Identifier: Apache-2.0

package smoke

import (
	"errors"
	"regexp"
	"strings"
)

// enrollmentCode is the display form of an enrollment code (M1-37, docs/spec/08-identity.md 3.2):
// en_ plus eight groups of four Crockford Base32 characters (no I, L, O, U), joined by '-'.
var enrollmentCode = regexp.MustCompile(`^en_[0-9A-HJKMNP-TV-Z]{4}(-[0-9A-HJKMNP-TV-Z]{4}){7}$`)

// ReadEnrollmentCode returns the enrollment code that `bothy setup bootstrap-owner` printed on
// stdout. The command prints the code as the only line and writes its logs to stderr (08 4,
// 02 3.3), so anything else on stdout is an error: empty output, several lines or a line that is
// not a code. The error never repeats the output, because it may hold a code.
func ReadEnrollmentCode(stdout []byte) (string, error) {
	lines := strings.Split(strings.TrimRight(string(stdout), "\r\n"), "\n")
	if len(lines) != 1 {
		return "", errors.New("smoke: the setup output has more than one line; only the enrollment code may be on stdout")
	}
	code := strings.TrimSuffix(lines[0], "\r")
	if !enrollmentCode.MatchString(code) {
		return "", errors.New("smoke: the setup output is not one enrollment code in the display form en_XXXX")
	}
	return code, nil
}

// canonicalCode returns the code without the prefix and the separators, the form the server
// hashes; the scan looks for it as well.
func canonicalCode(code string) string {
	return strings.ReplaceAll(strings.TrimPrefix(code, "en_"), "-", "")
}
