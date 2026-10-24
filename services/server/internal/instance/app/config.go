// SPDX-License-Identifier: Apache-2.0

package app

import "time"

// Config holds the configuration values of the settings, branding and discovery use cases
// (M1-46), bound in internal/wire from the keys of docs/spec/03-data-model.md section 9,
// docs/spec/12-files.md and docs/spec/18-release.md section 12 (wire.InstanceConfig). These are
// process configuration, not instance settings (16 section 5.1 item 6): an operator sets them,
// the owner cannot change them through the API.
type Config struct {
	// ProductNameMaxChars is branding.product_name_max_chars, the longest product name in Unicode
	// characters.
	ProductNameMaxChars int `config:"branding.product_name_max_chars"`
	// MessageMaxContentBytes is message.max_content_bytes (InstanceLimits).
	MessageMaxContentBytes int64 `config:"message.max_content_bytes"`
	// MessageMaxAttachments is message.max_attachments (InstanceLimits).
	MessageMaxAttachments int `config:"message.max_attachments"`
	// FileMaxUploadBytes is file.max_upload_bytes (InstanceLimits).
	FileMaxUploadBytes int64 `config:"file.max_upload_bytes"`
	// ThreadTitleMaxChars is thread.title_max_chars (InstanceLimits).
	ThreadTitleMaxChars int `config:"thread.title_max_chars"`
	// VoiceMaxDuration is voice.max_duration; getInstance reports it in milliseconds.
	VoiceMaxDuration time.Duration `config:"voice.max_duration"`
	// TelemetryDashboardURL is telemetry.dashboard_url, the link of the administration to the
	// telemetry backend; "" means none. getInstance sends it to owners only, because it names
	// internal infrastructure (brief M6-06 Design note 4).
	TelemetryDashboardURL string `config:"telemetry.dashboard_url"`
	// MinVersionIOS is client.min_version.ios; "" means no lower bound. The same holds for the
	// other platforms and for the recommended versions.
	MinVersionIOS string `config:"client.min_version.ios"`
	// MinVersionAndroid is client.min_version.android.
	MinVersionAndroid string `config:"client.min_version.android"`
	// MinVersionMacOS is client.min_version.macos.
	MinVersionMacOS string `config:"client.min_version.macos"`
	// MinVersionWindows is client.min_version.windows.
	MinVersionWindows string `config:"client.min_version.windows"`
	// RecommendedVersionIOS is client.recommended_version.ios; "" means no update note.
	RecommendedVersionIOS string `config:"client.recommended_version.ios"`
	// RecommendedVersionAndroid is client.recommended_version.android.
	RecommendedVersionAndroid string `config:"client.recommended_version.android"`
	// RecommendedVersionMacOS is client.recommended_version.macos.
	RecommendedVersionMacOS string `config:"client.recommended_version.macos"`
	// RecommendedVersionWindows is client.recommended_version.windows.
	RecommendedVersionWindows string `config:"client.recommended_version.windows"`
}
