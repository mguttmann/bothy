// SPDX-License-Identifier: Apache-2.0

package wire_test

import (
	"encoding/json"
	"testing"
)

// withoutChatTitles returns body (a live frame or a timeline) without the thread_title of the chat
// of each run, and the titles it removed. The chat of a run names its thread for a person who may
// read it (owner request 2026-11-01, docs/spec/25-visualization.md 4 rule 6); the content canaries
// of VZ-T07 must appear nowhere else. A body that is no JSON object stays as it is.
func withoutChatTitles(t *testing.T, body []byte) ([]byte, []string) {
	t.Helper()
	var doc map[string]any
	if err := json.Unmarshal(body, &doc); err != nil {
		return body, nil
	}
	runs, _ := doc["runs"].([]any)
	var titles []string
	for _, r := range runs {
		run, _ := r.(map[string]any)
		chat, _ := run["chat"].(map[string]any)
		if title, ok := chat["thread_title"].(string); ok {
			titles = append(titles, title)
			delete(chat, "thread_title")
		}
	}
	out, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	return out, titles
}
