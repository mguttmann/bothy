// SPDX-License-Identifier: Apache-2.0

// Package platform is the platform suite of the host service (brief M2-42 design note 19,
// docs/DECISIONS.md D-M2-42-19). It creates and deletes operating system users, services and ACLs
// with administrator rights and runs only on a disposable machine: a GitHub-hosted runner
// (ubuntu-24.04 with systemd, macos-15, windows-2025) through the manual or scheduled
// workflow host-platform.yml, never on a workstation or a machine with real users. Every file
// carries the build tag platform, so make check never compiles or runs it.
//
// Run: make -C services/host platform-test (sets BOTHY_HOST_BIN, BOTHY_FAKECLAUDE,
// BOTHY_FAKENODE and BOTHY_PLATFORM_RESULT). Each run writes a result file with platform, OS
// version, Go version, commit, test names, pass or fail and durations.
package platform
