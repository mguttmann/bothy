// SPDX-License-Identifier: Apache-2.0

package buildinfo

import (
	"runtime"
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
