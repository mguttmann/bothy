// SPDX-License-Identifier: Apache-2.0

// Package platform is the platform suite of the host service (brief M2-42 design note 19,
// docs/DECISIONS.md D-M2-42-19). It creates and deletes operating system users, services and ACLs
// with administrator rights and runs only on a disposable machine: a GitHub-hosted runner
// (ubuntu-24.04 with systemd, macos-15, windows-2025) through the manual or scheduled
// workflow host-platform.yml or a host container of the terminal suite (M2-48), never on a
// workstation or a machine with real users. Every file except doc.go, guard.go and guard_test.go
// carries the build tag platform, so make check runs only the guard's own test. The test binary
// refuses to run without the marker file /etc/termsuite-disposable (Windows
// %ProgramData%\termsuite-disposable) and exits 2 (Guard, brief M2-48 design note 11).
//
// Run: make -C services/host platform-test (builds the bundle of the build host and runs it), or
// copy a bundle (make -C services/host platform-bundle OUT=<dir> [GOOS=<os> GOARCH=<arch>]) to the
// disposable machine and run run.sh suite there (run.ps1 on Windows), which sets BOTHY_HOST_BIN,
// BOTHY_FAKECLAUDE, BOTHY_FAKENODE and BOTHY_PLATFORM_RESULT. Each run writes a result file with
// platform, OS version, Go version, commit, test names, pass or fail and durations.
package platform
