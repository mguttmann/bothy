// SPDX-License-Identifier: Apache-2.0

package computermcp

import (
	"encoding/json"
	"math"
	"strconv"

	"github.com/OWNER/bothy/services/host/internal/frame"
)

// typeBudget is the number of bytes the JSON form of one text may take inside a cu_action
// frame of the action type: frame.MaxLen less the type byte and the envelope
// {"seq":<largest seq>,"action":"type","args":{"text":""},"quick":true} (27 6.3, 6.4, 7).
var typeBudget = frame.MaxLen - 1 - len(`{"seq":,"action":"type","args":{"text":""},"quick":true}`) - len(strconv.FormatInt(math.MaxInt64, 10))

// splitText splits text into parts whose JSON string form fits typeBudget each, so every part is
// one type action (27 7: "C1 splits a long type text into several actions that each fit one
// frame"). It cuts only between runes, so no part breaks a UTF-8 sequence, and the parts join to
// text. The size of a rune is the size of its JSON form as encoding/json writes it (escapes such
// as < and   included), which is the encoder frame.NewAction uses. An empty text gives
// no part.
//
// Example: splitText("hello") is ["hello"].
func splitText(text string) []string {
	var parts []string
	start, size := 0, 0
	for i, r := range text {
		n := runeJSONLen(r)
		if size+n > typeBudget {
			parts = append(parts, text[start:i])
			start, size = i, 0
		}
		size += n
	}
	if start < len(text) {
		parts = append(parts, text[start:])
	}
	return parts
}

// runeJSONLen returns the length of r inside a JSON string written by encoding/json.
func runeJSONLen(r rune) int {
	if r >= 0x20 && r < 0x7f && r != '"' && r != '\\' && r != '<' && r != '>' && r != '&' {
		return 1
	}
	b, _ := json.Marshal(string(r))
	return len(b) - 2
}
