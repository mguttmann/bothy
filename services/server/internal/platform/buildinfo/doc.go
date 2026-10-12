// SPDX-License-Identifier: Apache-2.0

// Package buildinfo tells the server its own version (docs/spec/18-release.md "Components" rule
// 5). The version is a constant equal to services/server/VERSION, held by a unit test, and not
// set by linker flags: a plain `go build` then reports the same version as the release build.
// It feeds the resource attribute service.version (17), the comparison with
// instance.min_server_version (16) and the image labels (18).
//
// Example:
//
//	fmt.Println(buildinfo.Version()) // "0.0.0" before the first release
package buildinfo
