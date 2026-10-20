// SPDX-License-Identifier: Apache-2.0

package platform

import (
	"fmt"
	"io"
	"os"
)

// markerName is the file whose presence marks a disposable test machine (brief M2-48 design
// note 11). The host image of the terminal suite carries it; on a disposable macOS or Windows
// machine and on a hosted runner the build session or the workflow creates it before the run.
const markerName = "termsuite-disposable"

// refusal is the line the guard prints on a machine without the marker.
const refusal = "platform: not a disposable test machine (marker file missing)"

// MarkerPath returns where the marker file of a disposable test machine lies on goos (a value of
// runtime.GOOS): /etc/termsuite-disposable on Linux and macOS, <programData>\termsuite-disposable
// on Windows, where programData is the value of the environment variable ProgramData that the
// caller reads (the package reads no environment itself). It returns "" on Windows when
// programData is empty, so the guard refuses instead of looking at a path relative to the working
// directory.
func MarkerPath(goos, programData string) string {
	if goos == "windows" {
		if programData == "" {
			return ""
		}
		// Built by hand, not with filepath.Join, so the path is the Windows one on every build host.
		return programData + `\` + markerName
	}
	return "/etc/" + markerName
}

// Guard decides whether the platform suite may run on this machine. The suite creates and deletes
// operating system users, services and ACLs, so the test binary itself refuses to run unless a
// regular file exists at markerPath; an environment variable alone could be set anywhere by
// mistake, the marker is a property of the machine. markerPath is usually
// MarkerPath(runtime.GOOS, os.Getenv("ProgramData")); an empty path counts as missing. Guard
// returns 0 when the suite may run; otherwise it writes the refusal line to stderr and returns 2,
// the exit code TestMain ends with. It never exits itself, so a test can call it.
func Guard(stderr io.Writer, markerPath string) int {
	if markerPath != "" {
		if fi, err := os.Stat(markerPath); err == nil && fi.Mode().IsRegular() {
			return 0
		}
	}
	_, _ = fmt.Fprintln(stderr, refusal)
	return 2
}
