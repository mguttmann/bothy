// SPDX-License-Identifier: Apache-2.0

package wire

import (
	"github.com/OWNER/bothy/services/server/internal/notify/domain"
	"github.com/OWNER/bothy/services/server/internal/platform/config"
)

// notifyConfigKeys returns the keys of the notify module that are built (13 "Configuration values
// of this chapter"): notify.user_default_level, the last step of the effective level (M3-18), and
// the keys of the push path (M3-19): the bundling window, the mobile delay, the defaults of the
// personal switches and the interval of the badge sync.
// notifyIntervalDefault is the default of the bundling window, the mobile delay and the smallest
// badge sync interval (13 "Configuration values of this chapter").
const notifyIntervalDefault = "60s"

func notifyConfigKeys() []config.Key {
	return []config.Key{
		{Name: "notify.user_default_level", Kind: config.KindEnum, Default: string(domain.LevelMentions), Enum: notifyLevels(), Chapter: chapter13,
			Doc: "Notification level of a person without any own setting."},
		{Name: "notify.coalesce_window", Kind: config.KindDuration, Default: notifyIntervalDefault, Min: "0s", Chapter: chapter13,
			Doc: "Bundling window per recipient and thread; zero opens no window."},
		{Name: "notify.mobile_delay_when_desktop_active", Kind: config.KindDuration, Default: notifyIntervalDefault, Min: "0s", Chapter: chapter13,
			Doc: "Delay of mobile pushes while a desktop of the person is active; zero delays nothing."},
		{Name: "notify.default_admin_enabled", Kind: config.KindBool, Default: defaultTrue, Chapter: chapter13,
			Doc: "Default of the personal switch notify.admin_enabled."},
		{Name: "notify.default_input_needed_breaks_quiet_hours", Kind: config.KindBool, Default: "false", Chapter: chapter13,
			Doc: "Default of the personal switch notify.input_needed_breaks_quiet_hours."},
		{Name: "notify.badge_sync_min_interval", Kind: config.KindDuration, Default: notifyIntervalDefault, Min: "0s", Chapter: chapter13,
			Doc: "Smallest interval between two badge syncs of one person."},
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
