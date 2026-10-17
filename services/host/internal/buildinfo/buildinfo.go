// SPDX-License-Identifier: Apache-2.0

// Package buildinfo holds the version of the binary bothy-host and the platform it runs on, as the
// API names them (HostPairRequest.host_version and platform, docs/api/openapi.yaml).
package buildinfo

import "runtime"

// Version is the SemVer of this build without "v". make -C services/host release-build sets it
// with -ldflags "-X github.com/OWNER/bothy/services/host/internal/buildinfo.Version=<x.y.z>";
// a development build keeps the default.
var Version = "0.0.0-dev"

// Platform returns the HostPlatform value of the running operating system: "macos", "linux" or
// "windows"; any other GOOS returns "" (no build of bothy-host targets one).
func Platform() string { return PlatformOf(runtime.GOOS) }

// PlatformOf maps a GOOS value to its HostPlatform value ("darwin" is "macos"); unknown values
// give "".
//
// Example: PlatformOf("darwin") == "macos".
func PlatformOf(goos string) string {
	switch goos {
	case "darwin":
		return "macos"
	case "linux", "windows":
		return goos
	}
	return ""
}

// peerCheckProven lists the platforms whose platform run of TM-T26 proved the peer check of the
// local channel (docs/DECISIONS.md D-M2-42-11 and the ruling on the platform suite). It is empty
// until a recorded platform run proves a platform; the commit that records the run adds it. Until
// then HD10 fails with peer_check_unavailable there, and the terminal stays off (RK-23).
var peerCheckProven = map[string]bool{}

// PlatformSuite is set to "1" only by make -C services/host platform-test (-ldflags -X), so the
// platform suite measures the peer check itself on a disposable machine; release-build never sets
// it.
var PlatformSuite = ""

// PeerCheckProven reports whether the peer check counts as proven on goos.
func PeerCheckProven(goos string) bool { return peerCheckProven[goos] || PlatformSuite == "1" }
