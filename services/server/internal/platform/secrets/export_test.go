// SPDX-License-Identifier: Apache-2.0

package secrets

// NewFileDriverForUID exposes the file driver with an expected owner other than the effective
// uid, so tests can check the owner rule without root.
func NewFileDriverForUID(uid int) Driver { return newFileDriver(uid) }

// CheckSigningKeyDirForUID exposes the start check of the key store with an expected owner other
// than the effective uid.
func CheckSigningKeyDirForUID(dir string, uid int) error { return checkSigningKeyDir(dir, uid) }
