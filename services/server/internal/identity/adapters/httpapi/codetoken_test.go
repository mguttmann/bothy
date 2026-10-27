// SPDX-License-Identifier: Apache-2.0

package httpapi_test

import (
	"strings"
	"testing"
)

// containsCode reports whether code occurs in text as a whole token: neither the character
// before nor the one after is an ASCII letter or digit. A short code such as the six digits of
// a verification code also occurs by chance inside the random hex of key IDs, HMACs and UUIDs
// that audit details and log lines carry; such an occurrence is no leak of the code.
func containsCode(text, code string) bool {
	for i := 0; code != ""; {
		j := strings.Index(text[i:], code)
		if j < 0 {
			return false
		}
		start, end := i+j, i+j+len(code)
		if (start == 0 || !alnum(text[start-1])) && (end == len(text) || !alnum(text[end])) {
			return true
		}
		i = start + 1
	}
	return false
}

func alnum(c byte) bool {
	return c >= '0' && c <= '9' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z'
}

// TestContainsCode: a code inside a run of hex (main run 40491299995: "137420" in an audit
// entry of TestPairingCodeNeverLogged) is no match; the code as a value, an attribute or the
// whole text is, also after an earlier chance occurrence.
func TestContainsCode(t *testing.T) {
	t.Parallel()
	const code = "137420"
	kid := `{"kid": "7703fd96fec9150137420b6cc51dbbad"}`
	if !strings.Contains(kid, code) {
		t.Fatal("the hex sample does not hold the code")
	}
	for text, want := range map[string]bool{
		kid:                         false,
		"ab137420":                  false,
		"137420cd":                  false,
		"":                          false,
		code:                        true,
		`{"code": "137420"}`:        true,
		"verification_code=137420":  true,
		"approve 137420.":           true,
		kid + ` {"code": "137420"}`: true,
		"a137420 137420":            true,
		"ABCD-EFGH":                 false,
	} {
		if got := containsCode(text, code); got != want {
			t.Errorf("containsCode(%q) = %v, want %v", text, got, want)
		}
	}
	if !containsCode("pairing ABCD-EFGH", "ABCD-EFGH") || containsCode("xABCDEFGH", "ABCDEFGH") || containsCode("anything", "") {
		t.Error("pairing code forms")
	}
}
