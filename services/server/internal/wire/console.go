// SPDX-License-Identifier: Apache-2.0

package wire

import (
	"github.com/OWNER/bothy/services/server/internal/platform/config"
	"github.com/OWNER/bothy/services/server/internal/platform/httpserver"
)

// ConsoleConfig binds the keys of the web console serving (M6-02, M6-17).
type ConsoleConfig struct {
	// AssetsDir is console.assets_dir: the directory of the built console; empty disables it.
	AssetsDir string `config:"console.assets_dir"`
	// GrafanaEmbedURL is console.grafana_embed_url: the dashboard the console embeds for owners;
	// empty means no embedding and no frame-src (docs/spec/25-visualization.md 9.2).
	GrafanaEmbedURL string `config:"console.grafana_embed_url"`
}

// MountConsole opens the console build of console.assets_dir and mounts it under /console/ on
// mux (httpserver.ConsoleAssets); an empty key mounts nothing, so /console/ stays the 404 of the
// chain. With console.grafana_embed_url set, every console response names its origin in
// frame-src (brief M6-17 Design note 1). The error names the key when the directory cannot be opened or holds no index.html, so
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
	assets, err := httpserver.OpenConsoleAssets(c.AssetsDir, c.GrafanaEmbedURL)
	if err != nil {
		return err
	}
	assets.Mount(mux)
	return nil
}
