// SPDX-License-Identifier: Apache-2.0

package httpapi_test

import (
	"net/http"
	"testing"

	instanceapp "github.com/OWNER/bothy/services/server/internal/instance/app"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// TestGetInstance_VoiceTranscriptionMode (issue 570): getInstance carries
// voice_transcription_mode, the effective mode of the caller: the default agent, and service
// reported as off while voice.enabled is off (its default); the public discovery call does not
// name it.
func TestGetInstance_VoiceTranscriptionMode(t *testing.T) {
	t.Parallel()
	instanceRow := func(w *world) {
		w.sql(`INSERT INTO instance (instance_id, schema_version, sync_epoch, setup_completed_at) VALUES ($1, 14, $2, now())`,
			ids.New().String(), ids.New().String())
	}
	t.Run("default agent", func(t *testing.T) {
		t.Parallel()
		w := newWorld(t)
		instanceRow(w)
		w.person("sam", "active", false)
		body := w.want(w.do("sam", http.MethodGet, "/api/v1/instance", "", ""), http.StatusOK, "")
		if body["voice_transcription_mode"] != "agent" {
			t.Errorf("voice_transcription_mode %v, want agent", body["voice_transcription_mode"])
		}
		discovery := w.want(w.do("", http.MethodGet, "/api/v1/instance/discovery", "", ""), http.StatusOK, "")
		if v, ok := discovery["voice_transcription_mode"]; ok {
			t.Errorf("discovery names voice_transcription_mode %v, want none", v)
		}
	})
	t.Run("service with voice.enabled off", func(t *testing.T) {
		t.Parallel()
		w := newWorld(t, func(c *instanceapp.Config) { c.VoiceTranscriptionMode = "service" })
		instanceRow(w)
		w.person("sam", "active", false)
		body := w.want(w.do("sam", http.MethodGet, "/api/v1/instance", "", ""), http.StatusOK, "")
		if body["voice_transcription_mode"] != "off" {
			t.Errorf("voice_transcription_mode %v, want off", body["voice_transcription_mode"])
		}
	})
}
