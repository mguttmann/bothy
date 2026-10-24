// SPDX-License-Identifier: Apache-2.0

package runcontext_test

import (
	"reflect"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/platform/config"
	"github.com/OWNER/bothy/services/server/internal/wire"
)

// configValues loads the server's configuration schema with its defaults and the one key
// without a default.
func configValues(t *testing.T) *config.Values {
	t.Helper()
	schema, err := wire.ConfigSchema()
	must(t, err)
	v, err := config.Load(schema, config.Sources{Env: []string{"BOTHY_SERVER__PUBLIC_URL=https://chat.example.com"}})
	must(t, err)
	return v
}

// withDefaults is a module configuration of the world: every key at the default of the server's
// configuration schema (bound by the module's wire binder), then every non-zero field of world, the
// values the tests read. A key that a module makes mandatory later is therefore set here without
// touching the world (messaging principal.handle_max_chars and runs file.invocation_hold_max broke
// this package's setup before).
func withDefaults[T any](t *testing.T, bind func(*config.Values) (T, error), world T) T {
	t.Helper()
	c, err := bind(configValues(t))
	must(t, err)
	dst, src := reflect.ValueOf(&c).Elem(), reflect.ValueOf(world)
	for i := range src.NumField() {
		if f := src.Field(i); dst.Field(i).CanSet() && !f.IsZero() {
			dst.Field(i).Set(f)
		}
	}
	return c
}
