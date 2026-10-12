// SPDX-License-Identifier: Apache-2.0

package wire_test

import (
	"testing"

	"github.com/OWNER/bothy/services/server/internal/identity/app"
	"github.com/OWNER/bothy/services/server/internal/wire"
)

func TestIdentityConfigBindsDefaults(t *testing.T) {
	t.Parallel()
	cfg, err := wire.IdentityConfig(loadValues(t, schema(t)))
	if err != nil {
		t.Fatal(err)
	}
	if want := (app.Config{HandleMaxChars: 32, DisplayNameMaxChars: 64}); cfg != want {
		t.Fatalf("IdentityConfig = %+v, want %+v", cfg, want)
	}
}

func TestIdentityConfigBindsEnvironment(t *testing.T) {
	t.Parallel()
	v := loadValues(t, schema(t), "BOTHY_PRINCIPAL__HANDLE_MAX_CHARS=20", "BOTHY_PRINCIPAL__DISPLAY_NAME_MAX_CHARS=80")
	cfg, err := wire.IdentityConfig(v)
	if err != nil {
		t.Fatal(err)
	}
	if want := (app.Config{HandleMaxChars: 20, DisplayNameMaxChars: 80}); cfg != want {
		t.Fatalf("IdentityConfig = %+v, want %+v", cfg, want)
	}
}

func TestIdentityLimitsAtLeastOne(t *testing.T) {
	t.Parallel()
	for _, env := range []string{"BOTHY_PRINCIPAL__HANDLE_MAX_CHARS=0", "BOTHY_PRINCIPAL__DISPLAY_NAME_MAX_CHARS=0"} {
		if problems := load(t, env); len(problems) != 1 {
			t.Errorf("%s: problems %v, want one", env, problems)
		}
	}
}
