// SPDX-License-Identifier: Apache-2.0

package wire_test

import (
	"strings"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/wire"
)

// TestFilesCleanupConfigCheck (brief M3-08 design note 11): file.gc_interval equal to or longer
// than privacy.object_delete_delay fails the cross-check object_delete_delay_gc with a message that
// names both keys and no value; a shorter interval passes, and CleanupConfig binds the keys.
func TestFilesCleanupConfigCheck(t *testing.T) {
	t.Parallel()
	for name, env := range map[string][]string{
		"equal":  {"BOTHY_FILE__GC_INTERVAL=24h"},
		"longer": {"BOTHY_FILE__GC_INTERVAL=25h"},
		"delay":  {"BOTHY_PRIVACY__OBJECT_DELETE_DELAY=5m"},
	} {
		got := load(t, env...)
		if len(got) != 1 || !strings.Contains(got[0].String(), ": object_delete_delay_gc: ") ||
			!strings.Contains(got[0].String(), "file.gc_interval") || !strings.Contains(got[0].String(), "privacy.object_delete_delay") {
			t.Errorf("%s: problems %v, want object_delete_delay_gc", name, got)
		}
		noValues(t, got, "24h", "25h", "5m", "10m")
	}
	if got := load(t, "BOTHY_FILE__GC_INTERVAL=23h"); len(got) != 0 {
		t.Errorf("a shorter interval: problems %v", got)
	}
	cfg, err := wire.CleanupConfig(filesValues(t))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.UnattachedTTL != 24*time.Hour || cfg.FailedRowTTL != 7*24*time.Hour || cfg.SessionTTL != 24*time.Hour ||
		cfg.ObjectDeleteDelay != 24*time.Hour || cfg.BatchSize != 500 {
		t.Errorf("CleanupConfig %+v", cfg)
	}
}
