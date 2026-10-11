// SPDX-License-Identifier: Apache-2.0

package wire

import "github.com/OWNER/bothy/services/server/internal/platform/config"

// minPositiveDuration is the lower bound of a time limit that must not be zero: a zero would
// switch the limit off in net/http or leave a shutdown step without its own budget.
const minPositiveDuration = "1ms"

// httpConfigKeys returns the keys of the HTTP chain from docs/spec/02-architektur.md
// "Konfigurationswerte dieses Kapitels": the JSON body limit, which the cross-check
// json_limit_above_message reads, and the server limits that platform/httpserver applies to both
// listeners (M1-11). The HTTP chain (M1-18) adds its other keys here.
func httpConfigKeys() []config.Key {
	return []config.Key{
		{
			Name: "http.max_json_body_bytes", Kind: config.KindInt, Default: "1048576", Min: "1",
			Doc:     "Largest JSON request body in bytes; it must be greater than message.max_content_bytes.",
			Chapter: chapter02,
		},
		{
			Name: "http.read_header_timeout", Kind: config.KindDuration, Default: "5s", Min: minPositiveDuration,
			Doc:     "Time a client has to send the request headers.",
			Chapter: chapter02,
		},
		{
			Name: "http.idle_timeout", Kind: config.KindDuration, Default: "120s", Min: minPositiveDuration,
			Doc:     "How long a keep-alive connection may stay idle before the server closes it.",
			Chapter: chapter02,
		},
		{
			// At least 4 KiB: below that ordinary requests with a bearer token and DPoP proof no
			// longer fit; at most 16 MiB, which bounds the memory one connection can bind.
			Name: "http.max_header_bytes", Kind: config.KindInt, Default: "65536", Min: "4096", Max: "16777216",
			Doc:     "Largest request header block in bytes.",
			Chapter: chapter02,
		},
	}
}
