// SPDX-License-Identifier: Apache-2.0

package buildinfo

// version is the content of services/server/VERSION. A release changes both in the same commit;
// TestVersionMatchesVERSIONFile fails when they differ.
const version = "0.0.0"

// revision is the commit the binary was built from. The build paths set it with
// -ldflags "-X github.com/OWNER/bothy/services/server/internal/platform/buildinfo.revision=<sha>"
// (services/server/Dockerfile, services/server/Makefile release-build); a plain go build leaves it
// empty.
var revision string

// unknownRevision is what Revision returns for a build without the linker flag.
const unknownRevision = "unknown"

// Revision returns the commit of this server build as the build paths embed it, or "unknown" for
// a build without it (a plain go build or go test). It never returns an empty string.
func Revision() string {
	if revision == "" {
		return unknownRevision
	}
	return revision
}

// Version returns the version of this server build: SemVer 2.0.0 without a leading v, equal to
// services/server/VERSION. It never fails and never returns an empty string.
func Version() string { return version }
