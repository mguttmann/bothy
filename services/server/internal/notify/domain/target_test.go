// SPDX-License-Identifier: Apache-2.0

package domain_test

import (
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/notify/domain"
)

// TestTargetKind: space, channel and thread are targets of a preference; nothing else is.
func TestTargetKind(t *testing.T) {
	t.Parallel()
	for _, k := range []string{"space", "channel", "thread"} {
		if !domain.TargetKind(k).Valid() {
			t.Errorf("%q invalid", k)
		}
	}
	for _, k := range []string{"dm", "user", "", "Space"} {
		if domain.TargetKind(k).Valid() {
			t.Errorf("%q valid", k)
		}
	}
}

// TestPreferenceUnchanged: the same level and muted_until (both zero, or equal after the cut to
// microseconds) is unchanged; another level, set against zero or another instant is a change.
func TestPreferenceUnchanged(t *testing.T) {
	t.Parallel()
	at := time.Date(2026, 10, 23, 21, 30, 0, 123456000, time.UTC)
	for _, tc := range []struct {
		name       string
		stored, in domain.Preference
		want       bool
	}{
		{"both zero", domain.Preference{Level: domain.LevelAll}, domain.Preference{Level: domain.LevelAll}, true},
		{"equal after the cut", domain.Preference{Level: domain.LevelMute, MutedUntil: at},
			domain.Preference{Level: domain.LevelMute, MutedUntil: at.Add(789 * time.Nanosecond)}, true},
		{"same instant other zone", domain.Preference{Level: domain.LevelMute, MutedUntil: at},
			domain.Preference{Level: domain.LevelMute, MutedUntil: at.In(time.FixedZone("x", 3600))}, true},
		{"other level", domain.Preference{Level: domain.LevelAll}, domain.Preference{Level: domain.LevelMentions}, false},
		{"set against zero", domain.Preference{Level: domain.LevelAll}, domain.Preference{Level: domain.LevelAll, MutedUntil: at}, false},
		{"zero against set", domain.Preference{Level: domain.LevelAll, MutedUntil: at}, domain.Preference{Level: domain.LevelAll}, false},
		{"other instant", domain.Preference{Level: domain.LevelAll, MutedUntil: at},
			domain.Preference{Level: domain.LevelAll, MutedUntil: at.Add(time.Microsecond)}, false},
	} {
		if got := domain.PreferenceUnchanged(tc.stored, tc.in); got != tc.want {
			t.Errorf("%s: %v, want %v", tc.name, got, tc.want)
		}
	}
}
