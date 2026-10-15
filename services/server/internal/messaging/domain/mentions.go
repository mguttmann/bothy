// SPDX-License-Identifier: Apache-2.0

package domain

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// ScanMentions returns the distinct lower-case handles of the mention tokens of content in the
// order of their first appearance (03 messages "Inhaltsformat" items 1 and 2): "@" and a handle
// [a-z0-9][a-z0-9._-]* without regard to case, preceded by the start of the text or a character
// that is neither a letter, a digit nor one of _ . - @ /, with trailing ., - and _ cut off; tokens
// in code spans and code blocks do not count. @everyone and @here are handles like any other.
//
// Example:
//
//	domain.ScanMentions("@Alex see `@sam`, alex@example.com") // ["alex"]
func ScanMentions(content string) []string {
	if strings.IndexByte(content, '@') < 0 {
		return nil
	}
	code := CodeMask(content)
	var out []string
	seen := map[string]bool{}
	for i := 0; i < len(content); i++ {
		if content[i] != '@' || code[i] || !tokenStart(content[:i]) {
			continue
		}
		j := i + 1
		if j >= len(content) || !handleStart(content[j]) {
			continue
		}
		k := j + 1
		for k < len(content) && handleByte(content[k]) {
			k++
		}
		h := strings.ToLower(strings.TrimRight(content[j:k], "._-"))
		if !seen[h] {
			seen[h] = true
			out = append(out, h)
		}
		i = k - 1
	}
	return out
}

// tokenStart reports whether a mention may start after before: at the start of the text or after
// a character that is not a letter, a digit, _, ., -, @ or /.
func tokenStart(before string) bool {
	if before == "" {
		return true
	}
	r, _ := utf8.DecodeLastRuneInString(before)
	return !unicode.IsLetter(r) && !unicode.IsDigit(r) && !strings.ContainsRune("_.-@/", r)
}

// handleStart reports the first byte of a handle: an ASCII letter or digit.
func handleStart(b byte) bool { return isASCIILetter(b) || isASCIIDigit(b) }

// handleByte reports a byte that may follow inside a handle.
func handleByte(b byte) bool { return handleStart(b) || b == '.' || b == '_' || b == '-' }
