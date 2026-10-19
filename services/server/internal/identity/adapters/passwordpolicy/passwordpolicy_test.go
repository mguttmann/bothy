// SPDX-License-Identifier: Apache-2.0

package passwordpolicy_test

import (
	"bufio"
	"os"
	"regexp"
	"testing"
	"unicode/utf8"

	"github.com/OWNER/bothy/services/server/internal/identity/adapters/passwordpolicy"
	"github.com/OWNER/bothy/services/server/internal/identity/app"
	"github.com/OWNER/bothy/services/server/internal/identity/domain"
)

// Compile-time checks: the list is both outbound ports.
var (
	_ app.CommonPasswords    = (*passwordpolicy.List)(nil)
	_ app.PasswordNormalizer = (*passwordpolicy.List)(nil)
)

// knownListed are entries of the source file, taken from it during generation (README): the
// first is long enough to pass the length rule of the standard minimum.
var knownListed = []string{"unbelievable", "password", "qwerty"}

func TestCommonPasswordsFile(t *testing.T) {
	t.Parallel()
	f, err := os.Open("common_passwords.txt")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	line := regexp.MustCompile(`^[0-9a-f]{16}$`)
	sc := bufio.NewScanner(f)
	prev, n := "", 0
	for sc.Scan() {
		s := sc.Text()
		if !line.MatchString(s) {
			t.Fatalf("line %d %q is not 16 lowercase hex characters", n+1, s)
		}
		if s <= prev {
			t.Fatalf("line %d is not sorted and unique", n+1)
		}
		prev = s
		n++
	}
	if err := sc.Err(); err != nil {
		t.Fatal(err)
	}
	if n < 9000 {
		t.Fatalf("only %d entries", n)
	}
	l := passwordpolicy.Default()
	for _, pw := range knownListed {
		if !l.Contains(pw) {
			t.Errorf("listed password not found")
		}
	}
	// The candidate is hashed in lower case: case does not escape the list.
	if !l.Contains("UnBelievable") {
		t.Error("upper case escaped the list")
	}
	if l.Contains("EXAMPLE-unlisted-Robin-2026") {
		t.Error("an unlisted password was found")
	}
}

func TestPasswordRules(t *testing.T) {
	t.Parallel()
	l := passwordpolicy.Default()
	// NFC: e plus a combining acute accent is one code point after normalisation.
	decomposed := "EXAMPLE-café" // 13 code points before, 12 after
	nfc := l.NFC(decomposed)
	if nfc != "EXAMPLE-café" || utf8.RuneCountInString(nfc) != 12 {
		t.Fatalf("NFC %q", nfc)
	}
	if got := domain.CheckPassword(l.NFC("EXAMPLE-café"), 13, false); got != domain.PasswordTooShort {
		t.Fatalf("decomposed form counted before normalisation: %q", got)
	}
	if got := domain.CheckPassword(l.NFC("unbelievable"), 12, l.Contains(l.NFC("unbelievable"))); got != domain.PasswordCommon {
		t.Fatalf("list hit: %q", got)
	}
}
