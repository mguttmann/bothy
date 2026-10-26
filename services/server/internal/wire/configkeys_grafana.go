// SPDX-License-Identifier: Apache-2.0

package wire

import (
	"errors"
	"net/url"
	"strings"

	"github.com/OWNER/bothy/services/server/internal/platform/config"
	"github.com/OWNER/bothy/services/server/internal/platform/httpserver"
)

// keyGrafanaEmbedURL is the dashboard the web console embeds for owners
// (docs/spec/25-visualization.md 9.2, R-VZ-12, brief M6-17).
const keyGrafanaEmbedURL = "console.grafana_embed_url"

// grafanaConfigKeys returns console.grafana_embed_url. It is a string key, because the schema
// allows the empty default ("no embedding") only for string kinds (as telemetry.dashboard_url,
// D-M6-06-4); the cross-check grafana_embed_url requires an https URL whose origin differs from
// the console's.
func grafanaConfigKeys() []config.Key {
	return []config.Key{
		{
			Name: keyGrafanaEmbedURL, Kind: config.KindString, Default: "", Chapter: chapter25,
			Doc: "Dashboard URL the web console embeds for owners; empty means no embedding and no frame-src.",
		},
	}
}

// grafanaEmbedURL is the cross-check grafana_embed_url (25 9.2 items 4 and 5, SEC-VZ-2,
// VZ-T12): a set console.grafana_embed_url is an https URL with a host and without user
// information (httpserver.FrameOrigin), and its host is not the host of server.public_url. The
// same origin would lift the sandbox: with sandbox="allow-scripts allow-same-origin" Grafana's
// scripts would reach the console and its terminal. Another port of the same host is refused as
// well, because cookies are not separated by port (RFC 6265 section 8.5): the browser would send
// the console's session cookie to Grafana with every request of the iframe. The error names both
// keys and never a value.
func grafanaEmbedURL(v *config.Values) error {
	s := v.String(keyGrafanaEmbedURL)
	origin, err := httpserver.FrameOrigin(s)
	if err != nil {
		return errors.New(keyGrafanaEmbedURL + " must be empty or an https URL with a host and without user information: " + err.Error())
	}
	if origin == "" {
		return nil
	}
	public, err := url.Parse(v.String(keyPublicURL))
	if err != nil || public.Hostname() == "" {
		// public_url_origin reports the broken console origin.
		return nil
	}
	embed, _ := url.Parse(origin)
	if strings.EqualFold(strings.TrimSuffix(embed.Hostname(), "."), strings.TrimSuffix(public.Hostname(), ".")) {
		return errors.New(keyGrafanaEmbedURL + " must not be on the host of " + keyPublicURL +
			": the same origin lifts the iframe's sandbox and every port of the host gets the console's cookie; give Grafana its own subdomain")
	}
	return nil
}

// GrafanaEmbedCrossSite reports whether a set console.grafana_embed_url is on another site than
// server.public_url (25 9.2 item 4, httpserver.SameSite): another scheme, or another registrable
// domain by the public suffix list. Browsers may then not send the cookie of the auth proxy in the
// iframe, so bothy config check warns; it is no error, because a deployment may still work. It
// reports false for an empty value.
//
// Example:
//
//	if wire.GrafanaEmbedCrossSite(values) {
//		fmt.Fprintln(stderr, "warning: ...")
//	}
func GrafanaEmbedCrossSite(v *config.Values) bool {
	embed := v.String(keyGrafanaEmbedURL)
	if embed == "" {
		return false
	}
	return !httpserver.SameSite(v.String(keyPublicURL), embed)
}
