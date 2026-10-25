// SPDX-License-Identifier: Apache-2.0

package wire

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"
)

// TestStartLog_DPT13 (20 DP-T13, 4.1): the start log warns for retention.content = none without a
// reason, and otherwise names the effective value at info level; the reason itself is never
// logged.
func TestStartLog_DPT13(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name, content, reason string
		warn                  bool
		value                 string
	}{
		{"none without a reason", "none", "", true, ""},
		{"none with a reason", "none", "canary-reason-text", false, "none"},
		{"a period", "730d", "", false, "730d"},
	} {
		var buf bytes.Buffer
		log := slog.New(slog.NewJSONHandler(&buf, nil))
		logRetentionContent(t.Context(), log, RetentionConfig{Content: c.content, ContentNoneReason: c.reason})
		out := buf.String()
		if got := strings.Contains(out, `"level":"WARN"`) && strings.Contains(out, msgRetentionContentNone); got != c.warn {
			t.Errorf("%s: warning %v, want %v: %s", c.name, got, c.warn, out)
		}
		if c.value != "" && !strings.Contains(out, `"retention.content":"`+c.value+`"`) {
			t.Errorf("%s: the log does not name the effective value: %s", c.name, out)
		}
		if strings.Contains(out, "canary-reason-text") {
			t.Errorf("%s: the reason is logged: %s", c.name, out)
		}
	}
}
