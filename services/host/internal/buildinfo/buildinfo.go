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

// The package kinds of docs/spec/18-release.md 2 (row host) and infra/host: the kind a binary was
// built for, set at build time like Version.
const (
	// KindBinary is the release binary without an installation package (bothy-host_<v>_<os>_<arch>).
	KindBinary = ""
	// KindDeb is the binary inside the .deb package (Debian, Ubuntu).
	KindDeb = "deb"
	// KindRPM is the binary inside the .rpm package (Fedora, RHEL and their rebuilds).
	KindRPM = "rpm"
	// KindPkg is the binary inside the signed macOS .pkg package.
	KindPkg = "pkg"
	// KindMSI is the binary inside the signed Windows MSI package.
	KindMSI = "msi"
)

// PackageKind is the installation package this binary was built for (one of the Kind constants).
// make -C services/host release-build sets it per package with -ldflags "-X
// github.com/OWNER/bothy/services/host/internal/buildinfo.PackageKind=<kind>"; the release
// binaries and development builds keep KindBinary.
var PackageKind = KindBinary

// UpdatePath returns the update route of a binary of the package kind kind, the text the host
// service logs when the server refuses its version with 4114 (docs/spec/18-release.md 9.1): the
// host service never updates itself, so the instance operator installs the newer package of the
// same kind and then restarts the service, which does not restart by itself after 4114 (after an
// MSI upgrade with bothy-host install, which restores the service configuration). An unknown kind
// gets the route of the release binary.
//
// Example: UpdatePath(KindDeb) names the .deb package and "bothy-host restart".
func UpdatePath(kind string) string {
	const restart = ", then run bothy-host restart as an administrator"
	switch kind {
	case KindDeb:
		return "install the newer bothy-host .deb package of the release (apt install ./bothy-host_<version>_linux_<arch>.deb)" + restart
	case KindRPM:
		return "install the newer bothy-host .rpm package of the release (dnf install ./bothy-host_<version>_linux_<arch>.rpm)" + restart
	case KindPkg:
		return "install the newer bothy-host .pkg package of the release (installer -pkg bothy-host_<version>_darwin_<arch>.pkg -target /)" + restart
	case KindMSI:
		// The upgrade registers the service again with the start type demand, so install restores
		// its configuration (start type, recovery actions, DACL) and starts it.
		return "install the newer bothy-host MSI package of the release (msiexec /i bothy-host_<version>_windows_amd64.msi), " +
			"then run bothy-host install as an administrator"
	}
	return "download the newer bothy-host release binary, verify it (docs/operations/release.md) and run bothy-host install with it as an administrator"
}
