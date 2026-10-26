// SPDX-License-Identifier: Apache-2.0

package buildinfo

import (
	"runtime"
	"strings"
	"testing"
)

func TestPlatformOf(t *testing.T) {
	for goos, want := range map[string]string{"darwin": "macos", "linux": "linux", "windows": "windows", "plan9": ""} {
		if got := PlatformOf(goos); got != want {
			t.Errorf("PlatformOf(%q) = %q, want %q", goos, got, want)
		}
	}
	if Platform() != PlatformOf(runtime.GOOS) {
		t.Error("Platform differs from PlatformOf(runtime.GOOS)")
	}
	if Version == "" {
		t.Error("Version is empty")
	}
}

func TestPeerCheckProven(t *testing.T) {
	for _, goos := range []string{"linux", "darwin", "windows"} {
		if PeerCheckProven(goos) {
			t.Errorf("%s proven without a recorded platform run", goos)
		}
	}
	PlatformSuite = "1"
	defer func() { PlatformSuite = "" }()
	if !PeerCheckProven("linux") {
		t.Error("platform suite build")
	}
}

// TestUpdatePath (brief M7-23 requirement 16): every package kind names its own update route
// ("install the newer package of the same kind") and the step that follows; a release binary
// without a package names the release binary and install; an unknown kind falls back to it.
func TestUpdatePath(t *testing.T) {
	t.Parallel()
	for kind, want := range map[string][2]string{
		KindDeb: {".deb package", "bothy-host restart"}, KindRPM: {".rpm package", "bothy-host restart"},
		KindPkg: {".pkg package", "bothy-host restart"}, KindMSI: {"MSI package", "bothy-host install"},
	} {
		got := UpdatePath(kind)
		if !strings.Contains(got, want[0]) || !strings.Contains(got, want[1]) {
			t.Errorf("UpdatePath(%q) = %q", kind, got)
		}
	}
	for _, kind := range []string{KindBinary, "zip"} {
		if got := UpdatePath(kind); !strings.Contains(got, "release binary") || !strings.Contains(got, "bothy-host install") {
			t.Errorf("UpdatePath(%q) = %q", kind, got)
		}
	}
	if PackageKind != KindBinary {
		t.Errorf("a test build carries the package kind %q", PackageKind)
	}
}
