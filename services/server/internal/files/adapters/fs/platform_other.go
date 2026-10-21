// SPDX-License-Identifier: Apache-2.0

//go:build !unix

package fs

import (
	"errors"
	iofs "io/fs"
)

// Without a no-follow open flag and a file owner, the driver cannot keep its guarantees, so Open
// refuses every root on systems other than Unix (the server image is Unix).
const (
	oNoFollow = 0
	oNonBlock = 0
)

func fileOwner(iofs.FileInfo) (int, bool) { return 0, false }

func syncUnsupported(err error) bool { return errors.Is(err, errors.ErrUnsupported) }

func notDir(error) bool { return false }

func notEmpty(err error) bool { return errors.Is(err, iofs.ErrExist) }

func renameRaced(error) bool { return false }

func freeBytes(string) (int64, error) { return 0, errors.ErrUnsupported }
