// SPDX-License-Identifier: Apache-2.0

package buildinfo

// version is the content of services/server/VERSION. A release changes both in the same commit;
// TestVersionMatchesVERSIONFile fails when they differ.
const version = "0.0.0"

// Version returns the version of this server build: SemVer 2.0.0 without a leading v, equal to
// services/server/VERSION. It never fails and never returns an empty string.
func Version() string { return version }
