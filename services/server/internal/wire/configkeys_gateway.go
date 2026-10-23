// SPDX-License-Identifier: Apache-2.0

package wire

import "github.com/OWNER/bothy/services/server/internal/platform/config"

// gatewayConfigKeys returns the keys of the WebSocket gateway (M1-64, M1-66): the windows and
// limits of 04 "Configuration values of this chapter", focus and typing, and the write deadline of
// 02. The rate limit of
// identify and resume is the class rl.gateway_connect (configkeys_ratelimit.go).
func gatewayConfigKeys() []config.Key {
	return []config.Key{
		{Name: "sync.ws_max_frame_bytes", Kind: config.KindInt, Default: "1048576", Min: "1024", Max: "16777216", Chapter: chapter04,
			Doc: "Largest WebSocket frame of the gateway; a larger one closes the connection with 4002."},
		{Name: "sync.identify_timeout", Kind: config.KindDuration, Default: "10s", Min: minDeadline, Chapter: chapter04,
			Doc: "Time between hello and identify or resume; without either the gateway closes with 4003."},
		{Name: "sync.heartbeat_interval", Kind: config.KindDuration, Default: thirtySeconds, Min: minDeadline, Chapter: chapter04,
			Doc: "Interval of the client's heartbeats, announced in hello."},
		{Name: "sync.heartbeat_miss_limit", Kind: config.KindInt, Default: "2", Min: "0", Max: "100", Chapter: chapter04,
			Doc: "Missed heartbeats after which the gateway closes with 4009."},
		{Name: "sync.resume_window", Kind: config.KindDuration, Default: "5m", Min: minDeadline, Chapter: chapter04,
			Doc: "How long a gateway session stays resumable after its connection ended."},
		{Name: "sync.reauth_lead_time", Kind: config.KindDuration, Default: sixtySeconds, Min: minDeadline, Chapter: chapter04,
			Doc: "Lead time of reauth_required before a person's access token expires."},
		{Name: "sync.outbound_queue_max", Kind: config.KindInt, Default: "10000", Min: "1", Max: "1000000", Chapter: chapter04,
			Doc: "Frames in the outbound queue of a gateway connection before it closes with 4014."},
		{Name: "sync.max_focused_threads", Kind: config.KindInt, Default: "20", Min: "1", Max: "1000", Chapter: chapter04,
			Doc: "Threads a gateway connection may focus; a focus with more answers validation_failed."},
		// M2-10: the window in which the gateway gathers the patches of a run into one run.updated
		// per connection (04 section 11.3 point 3).
		{Name: "sync.run_patch_coalesce_interval", Kind: config.KindDuration, Default: "250ms", Min: minDeadline, Chapter: chapter04,
			Doc: "Window in which the gateway gathers the live-box patches of a run into one run.updated per connection."},
		{Name: "sync.typing_min_interval", Kind: config.KindDuration, Default: "5s", Min: minDeadline, Chapter: chapter04,
			Doc: "Shortest interval between two typing.started fan-outs of one connection and thread."},
		{Name: "sync.typing_ttl", Kind: config.KindDuration, Default: "8s", Min: minDeadline, Chapter: chapter04,
			Doc: "How long a typing indicator lasts without renewal (expires_at of typing.started)."},
		{Name: "gateway.write_timeout", Kind: config.KindDuration, Default: "10s", Min: minDeadline, Chapter: chapter02,
			Doc: "Deadline of one WebSocket write; a missed deadline closes the connection with 4014."},
	}
}
