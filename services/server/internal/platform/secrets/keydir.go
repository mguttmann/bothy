// SPDX-License-Identifier: Apache-2.0

package secrets

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
)

// modeKeyDir is the one allowed mode of secrets.signing_key_dir (ADR 0066 rule 1): the service
// user alone may list, create and remove keys.
const modeKeyDir = 0o700

// CheckSigningKeyDir is the start check of the key store (ADR 0066 rule 1, roles api, worker and
// all): dir must be an absolute clean path to a directory owned by the effective uid of the
// process with mode 0700, and the process must be able to create a file in it (a read-only mount
// fails here). The probe file starts with TempPrefix and is removed at once.
//
// The error names the key secrets.signing_key_dir, the expected mode and what was observed, in
// the style of the file rule SEC-SEC-1; it never names a file content.
func CheckSigningKeyDir(dir string) error { return checkSigningKeyDir(dir, os.Geteuid()) }

// checkSigningKeyDir is CheckSigningKeyDir with the expected owner as a parameter, so tests check
// the owner rule without root.
func checkSigningKeyDir(dir string, uid int) error {
	fail := func(detail string) error {
		return fmt.Errorf("secrets: secrets.signing_key_dir %s: %s; want a directory owned by the service user with mode 0700", printable(dir), detail)
	}
	if !isAbsClean(dir) || dir == "/" {
		return fail("not an absolute clean path other than the root")
	}
	if platformProblem != "" {
		return fail(platformProblem)
	}
	fi, err := os.Stat(dir)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return fail("does not exist")
	case err != nil:
		return fail(osError(dir, err).Detail)
	case !fi.IsDir():
		return fail("not a directory")
	}
	if m := unixMode(fi.Mode()); m != modeKeyDir {
		return fail(fmt.Sprintf("mode 0%03o", m))
	}
	if problem := ownerProblem(fi, uid); problem != "" {
		return fail(problem)
	}
	probe, err := os.CreateTemp(dir, TempPrefix+"probe-*")
	if err != nil {
		return fail("not writable: " + writeError(dir, err).Detail)
	}
	name := probe.Name()
	return errors.Join(probe.Close(), os.Remove(name))
}
