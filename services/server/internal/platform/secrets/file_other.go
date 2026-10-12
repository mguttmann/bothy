// SPDX-License-Identifier: Apache-2.0

//go:build !unix

package secrets

import (
	"io/fs"
	"os"
)

// openFlags opens the file for reading; on this platform Get refuses every file before.
const openFlags = os.O_RDONLY

// platformProblem fails closed: without unix ownership SEC-SEC-1 cannot be enforced, so Get
// refuses every file reference before it touches the file.
const platformProblem = "file ownership cannot be checked on this platform"

// ownerProblem is never reached on this platform (Get stops at platformProblem); it fails
// closed all the same.
func ownerProblem(fs.FileInfo, int) string { return platformProblem }
