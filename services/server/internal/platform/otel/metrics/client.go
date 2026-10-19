// SPDX-License-Identifier: Apache-2.0

package metrics

import "strconv"

// The attribute keys of the app metrics (signals.md 4.6a). Every app metric carries the four
// common keys; client.reconnects adds bothy.close.code.
const (
	// KeyClientPlatform is bothy.client.platform.
	KeyClientPlatform = "bothy.client.platform"
	// KeyClientDensity is bothy.client.density; absent when the app sent none.
	KeyClientDensity = "bothy.client.density"
	// KeyClientTheme is bothy.client.theme; absent when the app sent none.
	KeyClientTheme = "bothy.client.theme"
	// KeyClientAppMajor is bothy.client.app_major: the major version of the app, "other" outside
	// ^[0-9]{1,2}$.
	KeyClientAppMajor = "bothy.client.app_major"
	// KeyCloseCode is bothy.close.code.
	KeyCloseCode = "bothy.close.code"
)

// ClientAppMajors are the values of bothy.client.app_major: the decimal numbers 0 to 99 without
// leading zeros (signals.md 4.6a); every other major version records as "other".
func ClientAppMajors() []string {
	out := make([]string, 100)
	for i := range out {
		out[i] = strconv.Itoa(i)
	}
	return out
}

// clientAttrs are the common attributes of every app metric in recording order: platform,
// density, theme, app major version.
func clientAttrs() []Attr {
	return []Attr{
		{Key: KeyClientPlatform, Values: []string{"ios", "android", "macos", "windows", "web"}},
		{Key: KeyClientDensity, Values: []string{"comfortable", "compact", "touch"}, Optional: true},
		{Key: KeyClientTheme, Values: []string{"light", "dark"}, Optional: true},
		{Key: KeyClientAppMajor, Values: ClientAppMajors()},
	}
}

// clientSpec returns the server-side spec of the app metric name (the server name is
// "bothy."+name, signals.md 4.6a).
func clientSpec(name string, kind Kind, unit, description string, extra ...Attr) Spec {
	s := Spec{Name: "bothy." + name, Kind: kind, Unit: unit, Description: description, Attrs: append(clientAttrs(), extra...)}
	switch kind {
	case KindHistogram:
		s.Buckets = BucketsSeconds
	default:
		// The apps send counter increments and gauge values as JSON numbers (ClientMetric.value).
		s.Float = true
	}
	return s
}

// The app metrics of signals.md 4.6a under their server names. The apps aggregate them; the stats
// module checks every entry of a batch and records it here (ADR 0057, brief M4-31). The
// histograms are never created as SDK instruments: their bucket counts reach the exporter as
// pre-aggregated data points through Aggregated (docs/DECISIONS.md D-M4-31-1).
var (
	// ClientColdStartDuration is bothy.client.cold_start.duration.
	ClientColdStartDuration = clientSpec("client.cold_start.duration", KindHistogram, "s",
		"App cold start until a usable sidebar from the cache.")
	// ClientThreadOpenDuration is bothy.client.thread_open.duration.
	ClientThreadOpenDuration = clientSpec("client.thread_open.duration", KindHistogram, "s",
		"Opening a thread from the cache until the first messages are visible.")
	// ClientSendLocalDuration is bothy.client.send_local.duration.
	ClientSendLocalDuration = clientSpec("client.send_local.duration", KindHistogram, "s",
		"Sending a message until its local display.")
	// ClientFrames is bothy.client.frames.
	ClientFrames = clientSpec("client.frames", KindCounter, "{frame}", "Frames the app drew.")
	// ClientFramesOverBudget is bothy.client.frames.over_budget.
	ClientFramesOverBudget = clientSpec("client.frames.over_budget", KindCounter, "{frame}",
		"Frames that took longer than the frame budget of the display.")
	// ClientCatchupDuration is bothy.client.catchup.duration.
	ClientCatchupDuration = clientSpec("client.catchup.duration", KindHistogram, "s",
		"Time the app spent catching up after ready or resumed.")
	// ClientCatchupEvents is bothy.client.catchup.events.
	ClientCatchupEvents = clientSpec("client.catchup.events", KindCounter, "{event}",
		"Events the app received during catch-up.")
	// ClientResync is bothy.client.resync.
	ClientResync = clientSpec("client.resync", KindCounter, unitConnection,
		"Connections of the app that had to resynchronise.")
	// ClientReconnects is bothy.client.reconnects.
	ClientReconnects = clientSpec("client.reconnects", KindCounter, unitConnection,
		"Reconnects of the app by the close code that ended the previous connection.",
		Attr{Key: KeyCloseCode, Source: SourceCloseCodes})
	// ClientOutboxDepth is bothy.client.outbox.depth.
	ClientOutboxDepth = clientSpec("client.outbox.depth", KindGauge, "{entry}",
		"Entries of the outbox of the app that are not final.")
	// ClientPushDisplayDuration is bothy.client.push_display.duration.
	ClientPushDisplayDuration = clientSpec("client.push_display.duration", KindHistogram, "s",
		"Time from a push to the display of its notification.")
	// ClientPushPlaceholder is bothy.client.push.placeholder.
	ClientPushPlaceholder = clientSpec("client.push.placeholder", KindCounter, "{notification}",
		"Notifications that showed a placeholder instead of the text.")

	// ClientMetricsDiscarded is bothy.client_metrics.discarded (4.6, M4-31): entries of a client
	// metrics batch the server discarded, by reason.
	ClientMetricsDiscarded = Spec{
		Name: "bothy.client_metrics.discarded", Kind: KindCounter, Unit: "{entry}",
		Description: "Entries of client metrics batches the server discarded.",
		Attrs: []Attr{{Key: keyReason, Values: []string{
			"unknown_name", "kind_mismatch", "invalid_bounds", "invalid_value"}}},
	}
)

// Client returns the app metrics of signals.md 4.6a under their server names, in table order; a
// fresh slice per call.
func Client() []Spec {
	return []Spec{
		ClientColdStartDuration, ClientThreadOpenDuration, ClientSendLocalDuration, ClientFrames,
		ClientFramesOverBudget, ClientCatchupDuration, ClientCatchupEvents, ClientResync,
		ClientReconnects, ClientOutboxDepth, ClientPushDisplayDuration, ClientPushPlaceholder,
	}
}
