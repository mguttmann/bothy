// SPDX-License-Identifier: Apache-2.0

package config_test

import (
	"strings"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/platform/config"
)

// key returns a valid key of the given kind and default; tests change single fields from there.
func key(name string, kind config.Kind, def string) config.Key {
	return config.Key{Name: name, Kind: kind, Default: def, Doc: "Test key.", Chapter: "16-betrieb.md"}
}

// enumKey returns server.role as chapter 16 defines it.
func enumKey() config.Key {
	k := key("server.role", config.KindEnum, "all")
	k.Enum = []string{"api", "worker", "all"}
	return k
}

// mustSchema builds a schema or stops the test.
func mustSchema(t *testing.T, keys ...config.Key) *config.Schema {
	t.Helper()
	s, err := config.NewSchema(keys)
	if err != nil {
		t.Fatalf("NewSchema: %v", err)
	}
	return s
}

// wantSchemaError checks that err is not nil and contains every fragment.
func wantSchemaError(t *testing.T, err error, fragments ...string) {
	t.Helper()
	if err == nil {
		t.Fatalf("got no error, want one containing %q", fragments)
	}
	for _, f := range fragments {
		if !strings.Contains(err.Error(), f) {
			t.Errorf("error %q does not contain %q", err, f)
		}
	}
}
