// SPDX-License-Identifier: Apache-2.0

package wire

import (
	"log/slog"

	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/platform/config"
	"github.com/OWNER/bothy/services/server/internal/platform/secrets"
)

// NewSecretsProvider builds the secret provider of a process from the loaded configuration. It
// is the one place that builds it (brief M1-09 D10): every role and bothy config check --secrets
// call it once, and modules receive values through their Config or a narrow interface, never a
// provider of their own.
//
// v supplies secrets.dir and secrets.cache_ttl (bound into secrets.Config). env is the process
// environment as cmd/bothy read it once ("NAME=value" entries); the env driver looks up env:
// references there, because no other package reads the environment (02 "Konfigurationsladen"
// item 5). logger receives the error line of a failed refresh; nil means slog.Default(). The
// clock is the system clock.
//
// The error reports a schema without the two keys (config.Bind) or values secrets.NewProvider
// refuses (a secrets.dir that is not absolute and clean, a negative secrets.cache_ttl); it names
// keys, never values.
func NewSecretsProvider(v *config.Values, env []string, logger *slog.Logger) (*secrets.Provider, error) {
	var c secrets.Config
	if err := config.Bind(v, &c); err != nil {
		return nil, err
	}
	return secrets.NewProvider(secrets.Options{
		Dir:      c.Dir,
		CacheTTL: c.CacheTTL,
		Clock:    clock.System(),
		Logger:   logger,
		Env:      env,
	})
}
