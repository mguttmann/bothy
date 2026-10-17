// SPDX-License-Identifier: Apache-2.0

package hostos

import "os/exec"

// ownGroup is a no-op on Windows: the default cancel ends the process, and WaitDelay closes the
// pipes a child keeps open.
func ownGroup(*exec.Cmd) {}
