// SPDX-License-Identifier: Apache-2.0

package smoke

import (
	"strings"
	"testing"
)

// testCode is an enrollment code in the documented display form (M1-37): en_ plus eight groups of
// four Crockford Base32 characters.
const testCode = "en_ABCD-EFGH-JKMN-PQRS-TVWX-YZ01-2345-6789"

// TestReadEnrollmentCode: the code as the only line on stdout passes; empty output, several
// candidates and log noise on stdout fail.
func TestReadEnrollmentCode(t *testing.T) {
	t.Parallel()
	for _, ok := range []string{testCode + "\n", testCode, testCode + "\r\n"} {
		got, err := ReadEnrollmentCode([]byte(ok))
		if err != nil || got != testCode {
			t.Errorf("ReadEnrollmentCode(%q) = %q, %v", ok, got, err)
		}
	}
	for _, bad := range []string{
		"",
		"\n",
		testCode + "\n" + testCode + "\n",
		`{"level":"INFO","msg":"setup"}` + "\n" + testCode + "\n",
		"code: " + testCode + "\n",
		"en_ABCD-EFGH\n",
		"en_ABCD-EFGH-JKMN-PQRS-TVWX-YZ01-2345-678U\n", // U is not in the alphabet
	} {
		if got, err := ReadEnrollmentCode([]byte(bad)); err == nil {
			t.Errorf("ReadEnrollmentCode(%q) = %q, want an error", bad, got)
		}
	}
}

// TestReadEnrollmentCodeErrorHasNoValue: the error never repeats the output, which may hold a
// code.
func TestReadEnrollmentCodeErrorHasNoValue(t *testing.T) {
	t.Parallel()
	_, err := ReadEnrollmentCode([]byte(testCode + "\n" + testCode + "\n"))
	if err == nil || strings.Contains(err.Error(), "ABCD") {
		t.Errorf("error %v", err)
	}
}
