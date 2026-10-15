// SPDX-License-Identifier: Apache-2.0

package wire

// WarmEnabledValidator builds the request validator of the enabled operations once, before the
// first test of the package starts: its build grows with every enabled operation and takes tens of
// seconds under -race and host load, and the readiness waits of the serve tests (30 s each) must
// not pay it (docs/DECISIONS.md M1-31/18, the class of M1-33/18).
func WarmEnabledValidator() error {
	_, err := enabledValidator()
	return err
}
