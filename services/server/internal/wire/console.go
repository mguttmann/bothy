// SPDX-License-Identifier: Apache-2.0

package wire

import (
	"github.com/OWNER/bothy/services/server/internal/platform/config"
	"github.com/OWNER/bothy/services/server/internal/platform/httpserver"
)

// ConsoleConfig binds the key of the web console serving (M6-02).
type ConsoleConfig struct {
	// AssetsDir is console.assets_dir: the directory of the built console; empty disables it.
	AssetsDir string `config:"console.assets_dir"`
}

// MountConsole opens the console build of console.assets_dir and mounts it under /console/ on
// mux (httpserver.ConsoleAssets); an empty key mounts nothing, so /console/ stays the 404 of the
// chain. The error names the key when the directory cannot be opened or holds no index.html, so
// the start fails instead of serving a broken console.
//
// Example:
//
//	if err := wire.MountConsole(publicMux, values); err != nil {
//		return err // exit code 1, never ready
//	}
func MountConsole(mux Mux, v *config.Values) error {
	var c ConsoleConfig
	if err := config.Bind(v, &c); err != nil {
		return err
	}
	assets, err := httpserver.OpenConsoleAssets(c.AssetsDir)
	if err != nil {
		return err
	}
	assets.Mount(mux)
	return nil
}
