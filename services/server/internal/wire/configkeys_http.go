// SPDX-License-Identifier: Apache-2.0

package wire

import "github.com/OWNER/bothy/services/server/internal/platform/config"

// minPositiveDuration is the lower bound of a time limit that must not be zero: a zero would
// switch the limit off in net/http or leave a shutdown step without its own budget.
const minPositiveDuration = "1ms"

// httpConfigKeys returns the keys of the HTTP chain from docs/spec/02-architecture.md
// "Configuration values of this chapter": the JSON body limit, which the cross-check
// json_limit_above_message reads, the server limits that platform/httpserver applies to both
// listeners (M1-11), the limits, deadlines and Retry-After of the HTTP chain (M1-18), and the
// HSTS max-age of both listeners (M1-21).
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
		{
			// At most 1000: deeper than any contract schema, and the guard keeps one map per level.
			Name: "http.max_json_depth", Kind: config.KindInt, Default: "32", Min: "1", Max: "1000",
			Doc:     "Deepest nesting of a JSON request body; deeper bodies are refused with 400 malformed_request.",
			Chapter: chapter02,
		},
		{
			Name: "http.request_timeout", Kind: config.KindDuration, Default: "15s", Min: minPositiveDuration,
			Doc:     "Context deadline of a normal request.",
			Chapter: chapter02,
		},
		{
			Name: "http.transfer_timeout", Kind: config.KindDuration, Default: "10m", Min: minPositiveDuration,
			Doc:     "Deadline of upload and download operations, applied to the connection.",
			Chapter: chapter02,
		},
		{
			// Retry-After is sent in whole seconds, so less than one second cannot be expressed.
			Name: "http.unavailable_retry_after", Kind: config.KindDuration, Default: "5s", Min: "1s", Max: "1h",
			Doc:     "Retry-After of every 503 unavailable response, in whole seconds.",
			Chapter: chapter02,
		},
		{
			// Whole seconds, checked by hsts_whole_seconds: max-age has no fractions (SEC-TLS-3).
			Name: "http.hsts_max_age", Kind: config.KindDuration, Default: "31536000s", Min: "0s",
			Doc:     "max-age of Strict-Transport-Security in whole seconds, sent while server.public_url uses https.",
			Chapter: chapter19,
		},
		{
			Name: "file.max_upload_bytes", Kind: config.KindInt, Default: "104857600", Min: "1",
			Doc:     "Largest file, and the stream limit of multipart and octet-stream request bodies.",
			Chapter: chapter03,
		},
		{
			Name: "api.cors_allowed_origins", Kind: config.KindStringList, Default: "",
			Doc:     "Additional origins that get CORS headers; each entry is an origin such as https://app.example.com.",
			Chapter: chapter06,
		},
	}
}
