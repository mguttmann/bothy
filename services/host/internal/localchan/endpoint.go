// SPDX-License-Identifier: Apache-2.0

package localchan

import "path/filepath"

// Endpoint folders and names (D-M2-42-11, D-M2-42-13).
const (
	LinuxRunDir  = "/run/bothy-host"
	DarwinRunDir = "/Library/Application Support/bothy-host/run"
	AdminPipe    = `\\.\pipe\bothy-host-admin`
)

// HelperEndpoint returns the endpoint of the helper of sessionUser on goos.
func HelperEndpoint(goos, sessionUser string) string {
	switch goos {
	case "windows":
		return `\\.\pipe\bothy-host-helper-` + sessionUser
	case "darwin":
		return filepath.Join(DarwinRunDir, "helper-"+sessionUser+".sock")
	}
	return filepath.Join(LinuxRunDir, "helper-"+sessionUser+".sock")
}

// EndpointDir returns the folder of the helper endpoints on goos ("" on Windows: pipes have no
// folder).
func EndpointDir(goos string) string {
	switch goos {
	case "windows":
		return ""
	case "darwin":
		return DarwinRunDir
	}
	return LinuxRunDir
}
