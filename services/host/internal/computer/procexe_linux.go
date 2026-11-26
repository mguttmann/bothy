// SPDX-License-Identifier: Apache-2.0

package computer

import (
	"context"
	"errors"
	"os"
	"strconv"
)

// errNoSignature is the signature check of a platform without a helper.
var errNoSignature = errors.New("computer: no signature check on this platform")

// DefaultExePath returns the executable of process pid from /proc. Linux has no helper; the path
// part of the peer rule does not apply there (docs/spec/27-computer-use.md 9.4).
func DefaultExePath(pid int) (string, error) {
	return os.Readlink("/proc/" + strconv.Itoa(pid) + "/exe")
}

// DefaultVerifySignature refuses: Linux has no signed helper.
func DefaultVerifySignature(context.Context, string, string) error { return errNoSignature }
