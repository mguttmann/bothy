// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/terminal"
	"github.com/OWNER/bothy/services/server/internal/terminal/app"
)

// TestNew_Errors (M2-39): New refuses a missing collaborator and a configuration value below its
// minimum; NoConnections reports no host connected (no control connection before M2-43).
func TestNew_Errors(t *testing.T) {
	t.Parallel()
	if _, err := app.New(app.Config{}, app.Deps{}); err == nil {
		t.Fatal("New without collaborators")
	}
	cfg := app.Config{NameMaxChars: 1, OSUserMaxChars: 1, ReasonMaxChars: 1, PairingCodeTTL: time.Minute, PageSizeDefault: 2, PageSizeMax: 1}
	if _, err := app.New(cfg, app.Deps{}); err == nil {
		t.Fatal("New with page sizes the wrong way round")
	}
	if (app.NoConnections{}).Connected(context.Background(), ids.New()) {
		t.Fatal("NoConnections reports a connection")
	}
}

// TestSecret_Redacts (M2-39, SEC-LOG-1): the pairing code of the port renders as [redacted]
// through fmt, JSON and text, and only Reveal hands it out.
func TestSecret_Redacts(t *testing.T) {
	t.Parallel()
	s := terminal.NewSecret("bothy_hpc_secret")
	raw, err := json.Marshal(struct{ C terminal.Secret }{s})
	text, _ := s.MarshalText()
	if out := fmt.Sprintf("%v %s %+v %#v", s, s, s, s); strings.Contains(out, "secret") || err != nil || strings.Contains(string(raw), "secret") ||
		string(text) != "[redacted]" || s.LogValue().String() != "[redacted]" {
		t.Fatalf("leak: %s %s %v", out, raw, err)
	}
	if s.Reveal() != "bothy_hpc_secret" || (terminal.Secret{}).Reveal() != "" {
		t.Fatal("Reveal")
	}
}
