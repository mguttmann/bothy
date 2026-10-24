// SPDX-License-Identifier: Apache-2.0

//go:build platform

package platform

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// entryScanMeasurement holds the measured duration of the entry scan of a synthetic home of one
// million entries (design note 19); the result file carries it.
var entryScanMeasurement map[string]float64

// TestPlatform_EntryScanMeasurement fills the home of a session user with go run ./bighome and
// measures the full run of doctor (dominated by the entry scan).
func TestPlatform_EntryScanMeasurement(t *testing.T) {
	track(t)
	installed(t, "  entry_scan_timeout: 1h\n", userA)
	big := filepath.Join(home(userA), "bighome")
	gen := exec.Command("go", "run", "-tags", "platform", "./bighome", "-dir", big, "-entries", "1000000") //nolint:gosec // the generator of the suite
	if bin := os.Getenv("BOTHY_BIGHOME"); bin != "" {
		// A platform bundle carries bighome prebuilt: the test machine may have no Go (brief M2-48
		// design note 11).
		gen = exec.Command(bin, "-dir", big, "-entries", "1000000") //nolint:gosec // the generator of the bundle
	}
	if out, err := gen.CombinedOutput(); err != nil {
		t.Fatalf("bighome: %v\n%s", err, out)
	}
	chownTree(t, big, userA)
	start := time.Now()
	out, _ := doctor(t)
	d := time.Since(start)
	if strings.Contains(out, "entry_scan_timeout") {
		t.Errorf("the scan ran into the bound:\n%s", out)
	}
	entryScanMeasurement = map[string]float64{"seconds": d.Seconds()}
	_ = os.RemoveAll(big)
}

func chownTree(t *testing.T, root, user string) {
	t.Helper()
	if _, err := os.Stat("/bin/chown"); err == nil {
		sh(t, "/bin/chown", "-R", user, root)
		return
	}
	if _, err := os.Stat("/usr/sbin/chown"); err == nil {
		sh(t, "/usr/sbin/chown", "-R", user, root)
		return
	}
	sh(t, "icacls", root, "/setowner", user, "/T")
}
