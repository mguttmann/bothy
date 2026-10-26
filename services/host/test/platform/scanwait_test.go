// SPDX-License-Identifier: Apache-2.0

//go:build platform

package platform

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"
	"time"
)

// slowScanEntries is the number of entries that make the entry scan of a home take seconds on a
// test machine, far longer than an opening right after the first report.
const slowScanEntries = 200000

// TestPlatform_OpenAfterTheEntryScan: until the first entry scan of a session user finishes, HD11
// fails for it with entry_scan_running and the gate refuses its openings with shared_location
// (M2-45); the first report after a start may still carry that. waitReady returns only after the
// report that ends every scan of the start, so an opening right after newWorld or restart never
// races the scan. Sam's home of many entries makes the scan of the restart slow.
func TestPlatform_OpenAfterTheEntryScan(t *testing.T) {
	track(t)
	w := newWorld(t, "organization", "", userB)
	big := filepath.Join(home(userB), "many")
	fillEntries(t, big, slowScanEntries)
	chownTree(t, big, userB)
	t.Cleanup(func() { _ = os.RemoveAll(big) })
	start := time.Now()
	w.restart()
	t.Logf("restart ready after %s", time.Since(start).Round(time.Millisecond))
	w.open(shellOf(userB)).closeByClient()
}

// fillEntries creates n entries under dir with the generator of the suite; a platform bundle
// carries it prebuilt, as the test machine may have no Go (brief M2-48 design note 11).
func fillEntries(t *testing.T, dir string, n int) {
	t.Helper()
	gen := exec.Command("go", "run", "-tags", "platform", "./bighome", "-dir", dir, "-entries", strconv.Itoa(n)) //nolint:gosec // the generator of the suite
	if bin := os.Getenv("BOTHY_BIGHOME"); bin != "" {
		gen = exec.Command(bin, "-dir", dir, "-entries", strconv.Itoa(n)) //nolint:gosec // the generator of the bundle
	}
	if out, err := gen.CombinedOutput(); err != nil {
		t.Fatalf("bighome: %v\n%s", err, out)
	}
}
