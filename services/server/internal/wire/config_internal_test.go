// SPDX-License-Identifier: Apache-2.0

package wire

import (
	"strings"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/platform/config"
)

func TestBuildConfigSchemaReportsInvalidKeysAndChecks(t *testing.T) {
	t.Parallel()
	bad := func() []config.Key {
		return []config.Key{{Name: "Server.Role", Kind: config.KindString, Doc: "Bad.", Chapter: "16-betrieb.md"}}
	}
	if _, err := buildConfigSchema([]func() []config.Key{serverConfigKeys, bad}, nil); err == nil || !strings.Contains(err.Error(), "Server.Role") {
		t.Errorf("invalid key: error = %v", err)
	}
	checks := []configCheck{{name: "dangling", keys: []string{"server.missing"}, fn: listenersDistinct}}
	if _, err := buildConfigSchema(configKeyGroups, checks); err == nil || !strings.Contains(err.Error(), `unknown key "server.missing"`) {
		t.Errorf("invalid check: error = %v", err)
	}
}

func TestListenersConflictNeedsListenAddresses(t *testing.T) {
	t.Parallel()
	// Values of kind listen address always parse; text of another form never conflicts.
	for _, pair := range [][2]string{{"no-port", ":8080"}, {":8080", "no-port"}, {"name:8080", "127.0.0.1:8080"}, {"127.0.0.1:8080", "name:8080"}} {
		if listenersConflict(pair[0], pair[1]) {
			t.Errorf("listenersConflict(%q, %q) = true", pair[0], pair[1])
		}
	}
}
