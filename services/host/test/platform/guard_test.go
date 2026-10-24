// SPDX-License-Identifier: Apache-2.0

package platform

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// TestMainRefusesWithoutMarker: the guard the tagged TestMain calls first refuses a machine
// without the marker file with exit 2 and the fixed message, and lets a marked machine pass
// (brief M2-48 design note 11). It runs untagged, so make check proves the guard.
func TestMainRefusesWithoutMarker(t *testing.T) {
	dir := t.TempDir()
	marker := filepath.Join(dir, "termsuite-disposable")

	var stderr bytes.Buffer
	if code := Guard(&stderr, marker); code != 2 {
		t.Errorf("Guard without the marker = %d, want 2", code)
	}
	if got, want := stderr.String(), "platform: not a disposable test machine (marker file missing)\n"; got != want {
		t.Errorf("stderr = %q, want %q", got, want)
	}

	stderr.Reset()
	if code := Guard(&stderr, ""); code != 2 || stderr.Len() == 0 {
		t.Errorf("Guard with an unknown marker path = %d (%q), want 2 and the message", code, stderr.String())
	}

	stderr.Reset()
	if err := os.Mkdir(marker, 0o700); err != nil {
		t.Fatal(err)
	}
	if code := Guard(&stderr, marker); code != 2 {
		t.Errorf("Guard with a folder at the marker path = %d, want 2", code)
	}
	if err := os.Remove(marker); err != nil {
		t.Fatal(err)
	}

	stderr.Reset()
	if err := os.WriteFile(marker, []byte("termsuite: disposable test machine\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if code := Guard(&stderr, marker); code != 0 {
		t.Errorf("Guard with the marker = %d, want 0", code)
	}
	if stderr.Len() != 0 {
		t.Errorf("Guard with the marker wrote %q", stderr.String())
	}
}

// TestMarkerPath: the marker lies in /etc on Linux and macOS and in the ProgramData folder on
// Windows; without ProgramData there is no marker path, so the guard refuses.
func TestMarkerPath(t *testing.T) {
	for _, goos := range []string{"linux", "darwin"} {
		if got := MarkerPath(goos, `C:\ProgramData`); got != "/etc/termsuite-disposable" {
			t.Errorf("MarkerPath(%q) = %q, want /etc/termsuite-disposable", goos, got)
		}
	}
	if got := MarkerPath("windows", `C:\ProgramData`); got != `C:\ProgramData\termsuite-disposable` {
		t.Errorf(`MarkerPath("windows") = %q, want C:\ProgramData\termsuite-disposable`, got)
	}
	if got := MarkerPath("windows", ""); got != "" {
		t.Errorf(`MarkerPath("windows") without ProgramData = %q, want ""`, got)
	}
}
