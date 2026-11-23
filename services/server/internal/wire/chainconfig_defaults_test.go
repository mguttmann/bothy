// SPDX-License-Identifier: Apache-2.0

package wire_test

import (
	"reflect"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/platform/config"
	"github.com/OWNER/bothy/services/server/internal/platform/httpserver"
)

// TestDefaultChainConfigMatchesSchema holds httpserver.DefaultChainConfig, the start of the chain
// tests and the harness, equal to the defaults of the configuration schema, so a changed default
// cannot leave those tests on the old value.
func TestDefaultChainConfigMatchesSchema(t *testing.T) {
	t.Parallel()
	v, err := config.Load(schema(t), config.Sources{Env: testEnv()})
	if err != nil {
		t.Fatal(err)
	}
	var got httpserver.ChainConfig
	if err := config.Bind(v, &got); err != nil {
		t.Fatal(err)
	}
	want := httpserver.DefaultChainConfig(got.PublicURL)
	// An empty list binds as an empty slice and the literal leaves it nil; both mean no origin.
	if len(got.CORSAllowedOrigins) == 0 && len(want.CORSAllowedOrigins) == 0 {
		got.CORSAllowedOrigins, want.CORSAllowedOrigins = nil, nil
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("schema defaults %+v\nDefaultChainConfig %+v", got, want)
	}
}
