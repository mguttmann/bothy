// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"context"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/instance"
	"github.com/OWNER/bothy/services/server/internal/instance/app"
	"github.com/OWNER/bothy/services/server/internal/instance/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// TestInfo_VoiceTranscriptionMode (issue 570): getInstance reports the effective transcription
// mode of the caller (12 8.2): voice.transcription_mode, with service reported as off while
// voice.enabled is off for the caller; agent and off do not depend on the switch.
func TestInfo_VoiceTranscriptionMode(t *testing.T) {
	t.Parallel()
	sam := ids.New()
	for _, c := range []struct {
		name, mode string
		voiceOn    bool
		want       string
	}{
		{"agent with the switch off", "agent", false, "agent"},
		{"agent with the switch on", "agent", true, "agent"},
		{"off with the switch on", "off", true, "off"},
		{"service with the switch on", "service", true, "service"},
		{"service with the switch off", "service", false, "off"},
		{"not configured", "", true, ""},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			cfg := adminCfg
			cfg.VoiceTranscriptionMode = c.mode
			e := newAdminEnvWith(t, cfg, authorize.OutcomeAllow, nil)
			if c.voiceOn {
				// Only sam is in the rollout, so the value is the caller's, not the global one.
				e.flags.rows[instance.FlagVoiceEnabled] = app.FlagRow{Key: instance.FlagVoiceEnabled, Version: 1, Rollout: domain.Rollout{UserIDs: []ids.ID{sam}}}
				e.users.people = []ids.ID{sam, e.owner.ID}
			}
			info, err := e.admin.Info(context.Background(), actor.Actor{ID: sam, Kind: actor.KindUser})
			if err != nil {
				t.Fatal(err)
			}
			if info.VoiceTranscriptionMode != c.want {
				t.Errorf("mode %q, want %q", info.VoiceTranscriptionMode, c.want)
			}
		})
	}
}
