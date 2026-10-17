// SPDX-License-Identifier: Apache-2.0

package hostos

import (
	"io/fs"
	"os"
)

// OSLinkFS is the LinkFS of the operating system, seen with the rights of the calling process.
type OSLinkFS struct{}

// IsLink implements LinkFS: a symbolic link, or on Windows also a junction (a reparse point that
// os.Readlink can read).
func (OSLinkFS) IsLink(path string) (bool, error) {
	fi, err := os.Lstat(path)
	if err != nil {
		return false, err
	}
	if fi.Mode()&fs.ModeSymlink != 0 {
		return true, nil
	}
	if fi.Mode()&fs.ModeIrregular != 0 && fi.IsDir() {
		if _, err := os.Readlink(path); err == nil {
			return true, nil
		}
	}
	return false, nil
}

// Readlink implements LinkFS.
func (OSLinkFS) Readlink(path string) (string, error) { return os.Readlink(path) }

// OSResolver is the Resolver of the operating system with the rights of the calling process: the
// helper resolves the places of its session user, the service user the places for all.
type OSResolver struct{}

// Resolve implements Resolver.
func (OSResolver) Resolve(path string) (Resolved, error) { return Resolve(OSLinkFS{}, path) }

// Identity implements Resolver.
func (OSResolver) Identity(path string) (string, error) { return identity(path) }
