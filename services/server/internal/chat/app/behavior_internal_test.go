// SPDX-License-Identifier: Apache-2.0

package app

import (
	"encoding/json"
	"slices"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// TestSettingsChangedEvents: the event builder of design note 6 of M2-02. With a configuration
// change and an operator two events in order, without operator or without configuration change
// one; payloads have exactly channel_id and settings_version, no resource version, the audience
// and receive actions of 05 section 6.
func TestSettingsChangedEvents(t *testing.T) {
	t.Parallel()
	space, channel, lotse := ids.New(), ids.New(), ids.New()
	for _, c := range []struct {
		name          string
		operator      ids.ID
		configChanged bool
		want          []string
	}{
		{"configuration change with operator", lotse, true, []string{"channel.settings_changed", "agent.config_changed"}},
		{"configuration change without operator", ids.ID{}, true, []string{"channel.settings_changed"}},
		{"switches only", lotse, false, []string{"channel.settings_changed"}},
	} {
		evs, err := settingsEvents(space, channel, 7, c.operator, c.configChanged)
		if err != nil {
			t.Fatal(err)
		}
		var types []string
		for _, ev := range evs {
			types = append(types, ev.Type)
			if ev.ResourceKind != "channel" || ev.ResourceID != channel || ev.ResourceVersion != 0 || ev.SpaceID != space || ev.ChannelID != channel {
				t.Errorf("%s: %s scope or resource %+v", c.name, ev.Type, ev)
			}
			var payload map[string]any
			if err := json.Unmarshal(ev.Payload, &payload); err != nil {
				t.Fatal(err)
			}
			if len(payload) != 2 || payload["channel_id"] != channel.String() || payload["settings_version"] != float64(7) {
				t.Errorf("%s: %s payload %s, want exactly channel_id and settings_version", c.name, ev.Type, ev.Payload)
			}
			switch ev.Type {
			case "channel.settings_changed":
				if ev.ReceiveAction != "channel.read" || !ev.AudiencePrincipalID.IsZero() {
					t.Errorf("%s: settings_changed receive %q audience %v", c.name, ev.ReceiveAction, ev.AudiencePrincipalID)
				}
			case "agent.config_changed":
				if ev.ReceiveAction != "agent.config.read" || ev.AudiencePrincipalID != lotse {
					t.Errorf("%s: config_changed receive %q audience %v", c.name, ev.ReceiveAction, ev.AudiencePrincipalID)
				}
			}
		}
		if !slices.Equal(types, c.want) {
			t.Errorf("%s: %v, want %v", c.name, types, c.want)
		}
	}
}
