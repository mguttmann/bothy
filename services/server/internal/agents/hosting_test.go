// SPDX-License-Identifier: Apache-2.0

package agents_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/agents"
)

// TestSecret_Redacted: the secret of a hosted token renders as "[redacted]" through fmt, slog,
// encoding/json and encoding.TextMarshaler (SEC-LOG-1); only Reveal hands out the value, and the
// zero Secret reveals "" and reports IsZero.
func TestSecret_Redacted(t *testing.T) {
	t.Parallel()
	const canary = "bothy_agt_canary-value"
	s := agents.NewSecret(canary)
	if s.Reveal() != canary || s.IsZero() {
		t.Fatalf("Reveal %q, IsZero %v", s.Reveal(), s.IsZero())
	}
	var zero agents.Secret
	if zero.Reveal() != "" || !zero.IsZero() {
		t.Fatalf("zero Secret: Reveal %q, IsZero %v", zero.Reveal(), zero.IsZero())
	}
	var logged bytes.Buffer
	slog.New(slog.NewJSONHandler(&logged, nil)).Info("issued", "secret", s)
	js, err := json.Marshal(struct{ S agents.Secret }{s})
	if err != nil {
		t.Fatal(err)
	}
	text, err := s.MarshalText()
	if err != nil {
		t.Fatal(err)
	}
	for _, out := range []string{fmt.Sprint(s), fmt.Sprintf("%v %+v %#v %s %q", s, s, s, s, s), s.String(), s.GoString(),
		logged.String(), string(js), string(text)} {
		if strings.Contains(out, canary) || !strings.Contains(out, "[redacted]") {
			t.Errorf("rendering %q", out)
		}
	}
}
