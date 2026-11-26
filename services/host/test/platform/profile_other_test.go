// SPDX-License-Identifier: Apache-2.0

//go:build platform && !windows

package platform

// createWindowsProfile and deleteWindowsProfile exist only on Windows (profile_windows_test.go).
func createWindowsProfile(string) error { return nil }

func deleteWindowsProfile(string) {}

// adminOwned is a no-op on Linux and macOS: root owns what the suite creates.
func adminOwned(string) error { return nil }
