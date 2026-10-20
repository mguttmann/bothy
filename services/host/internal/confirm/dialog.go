// SPDX-License-Identifier: Apache-2.0

package confirm

import (
	"fmt"
	"time"
)

// dialogTitle is the title of every dialog.
const dialogTitle = "bothy-host"

// dialogText is the question of the dialog: open or attach, the profile and the session user,
// nothing else (brief M2-45 Design note 15 c).
func dialogText(r Request) string {
	if r.Kind == KindAttach {
		return fmt.Sprintf("Attach to the terminal session with the profile %s of the session user %s?", r.Profile, r.SessionUser)
	}
	return fmt.Sprintf("Open a terminal session with the profile %s as the session user %s?", r.Profile, r.SessionUser)
}

// dialogSeconds rounds timeout up to whole seconds, at least one, for the dialog programs that
// count in seconds; the context of the request still ends the dialog at the exact timeout.
func dialogSeconds(timeout time.Duration) int {
	return max(1, int((timeout+time.Second-1)/time.Second))
}
