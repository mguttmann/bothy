// SPDX-License-Identifier: Apache-2.0

// Test cases for the rule bothy-go-test-null-substring (run by `make semgrep`).
package fixtures

import (
	"bytes"
	"strings"

	"github.com/OWNER/bothy/services/server/internal/platform/testkit"
)

func nullAsText(body []byte, raw string) bool {
	// ruleid: bothy-go-test-null-substring
	if strings.Contains(string(body), "null") {
		return true
	}
	// ruleid: bothy-go-test-null-substring
	if strings.Contains(raw, "null") {
		return true
	}
	// ruleid: bothy-go-test-null-substring
	if bytes.Contains(body, []byte("null")) {
		return true
	}
	for _, absent := range []string{"approved_at", "null"} {
		// ruleid: bothy-go-test-null-substring
		if strings.Contains(raw, absent) {
			return true
		}
	}
	for _, banned := range []string{"hash", "null"} {
		// ruleid: bothy-go-test-null-substring
		if bytes.Contains(body, []byte(banned)) {
			return true
		}
	}
	return false
}

func nullAsJSON(body []byte, raw string) bool {
	// ok: bothy-go-test-null-substring
	if testkit.HasJSONNull(body) {
		return true
	}
	// ok: bothy-go-test-null-substring
	if strings.Contains(raw, `"approved_at"`) {
		return true
	}
	for _, absent := range []string{"approved_at", "revoked_at"} {
		// ok: bothy-go-test-null-substring
		if strings.Contains(raw, absent) {
			return true
		}
	}
	for _, origin := range []string{"https://chat.example.com", "null"} {
		// ok: bothy-go-test-null-substring
		_ = origin == raw
	}
	return false
}
