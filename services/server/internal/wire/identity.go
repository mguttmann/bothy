// SPDX-License-Identifier: Apache-2.0

package wire

import (
	"github.com/OWNER/bothy/services/server/internal/identity/app"
	"github.com/OWNER/bothy/services/server/internal/platform/config"
)

// IdentityConfig binds the principal.* keys of v into the configuration of the identity module.
//
// The error names the key whose binding failed (config.Bind), never a value.
func IdentityConfig(v *config.Values) (app.Config, error) {
	var c app.Config
	if err := config.Bind(v, &c); err != nil {
		return app.Config{}, err
	}
	return c, nil
}
