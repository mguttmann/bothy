// SPDX-License-Identifier: Apache-2.0

package wire

import (
	"github.com/OWNER/bothy/services/server/internal/notify/domain"
	"github.com/OWNER/bothy/services/server/internal/platform/config"
)

// notifyConfigKeys returns the keys of the notify module that are built (13 "Konfigurationswerte
// dieses Kapitels"): notify.user_default_level, the last step of the effective level (M3-18).
func notifyConfigKeys() []config.Key {
	return []config.Key{
		{Name: "notify.user_default_level", Kind: config.KindEnum, Default: string(domain.LevelMentions), Enum: notifyLevels(), Chapter: chapter13,
			Doc: "Notification level of a person without any own setting."},
	}
}

// notifyLevels returns the levels all, mentions and mute of notify/domain as the enum values.
func notifyLevels() []string {
	var out []string
	for _, l := range domain.Levels() {
		out = append(out, string(l))
	}
	return out
}
