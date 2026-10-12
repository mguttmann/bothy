// SPDX-License-Identifier: Apache-2.0

//go:build !unix

package stack

import (
	"errors"
	"io/fs"
	"os"
)

// readFlags adds nothing where pipes cannot appear in a directory tree.
const readFlags = 0

// ownerOf cannot read ownership on this platform; callers treat the owner as different.
func ownerOf(fs.FileInfo) (Owner, bool) { return Owner{}, false }

// runningUser fails closed: without ownership the service copies could not be protected
// (docs/spec/16-operations.md 5.3 item 4), so init refuses to run instead of writing open copies.
func runningUser() (Owner, error) {
	return Owner{}, errors.New("secrets init needs a unix system: file ownership cannot be set on this platform")
}

// syncDir has nothing to do: directories cannot be synced on this platform.
func syncDir(*os.Root, string) error { return nil }
