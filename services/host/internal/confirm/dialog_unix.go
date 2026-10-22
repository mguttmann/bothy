// SPDX-License-Identifier: Apache-2.0

//go:build linux || darwin

package confirm

import "os"

// toolExists reports whether path is an executable regular file.
func toolExists(path string) bool {
	fi, err := os.Stat(path)
	return err == nil && fi.Mode().IsRegular() && fi.Mode().Perm()&0o111 != 0
}
