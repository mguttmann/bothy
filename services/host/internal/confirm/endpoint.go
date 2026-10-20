// SPDX-License-Identifier: Apache-2.0

package confirm

// Endpoints of the confirmation channel (brief M2-45 Design note 15 a). The Linux folder comes from
// RuntimeDirectory=bothy-host-confirm of the service unit, the macOS folder from install.
const (
	linuxEndpoint   = "/run/bothy-host-confirm/confirm.sock"
	darwinEndpoint  = "/Library/Application Support/bothy-host/confirm/confirm.sock"
	windowsEndpoint = `\\.\pipe\bothy-host-confirm`
)

// Endpoint returns the endpoint of goos: a Unix socket path on Linux and macOS, the name of the
// named pipe on Windows. Any other goos gets the Linux path.
func Endpoint(goos string) string {
	switch goos {
	case "windows":
		return windowsEndpoint
	case "darwin":
		return darwinEndpoint
	}
	return linuxEndpoint
}
