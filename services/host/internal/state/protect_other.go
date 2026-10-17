// SPDX-License-Identifier: Apache-2.0

//go:build !windows

package state

// PlatformProtector returns the Protector of the running platform: Plain on Linux and macOS, where
// the mode 0600 of the key file in the 0700 directory of the service user protects the keys.
func PlatformProtector() Protector { return Plain{} }
