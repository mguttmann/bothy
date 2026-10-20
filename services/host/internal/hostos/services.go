// SPDX-License-Identifier: Apache-2.0

package hostos

import (
	"context"
	"errors"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Names of the services of the host service (docs/spec/02-architecture.md "Places that carry the
// product name", D-M2-42-13).
const (
	// HostService is the systemd unit stem, launchd label and Windows service of the host service.
	HostService = "bothy-host"
)

// HostServiceName returns the name the service manager of goos knows the host service by.
func HostServiceName(goos string) string {
	if goos == "linux" {
		return HostService + ".service"
	}
	return HostService
}

// HelperServiceName returns the name of the helper service of sessionUser on goos.
func HelperServiceName(goos, sessionUser string) string {
	switch goos {
	case "linux":
		return HostService + "-helper-" + sessionUser + ".service"
	case "darwin":
		return HostService + ".helper." + sessionUser
	}
	return HostService + "-helper-" + sessionUser
}

// ServiceManager tells the main process of a service.
type ServiceManager interface {
	// MainPID returns the process ID the service manager reports for the service name; an error
	// when the service is unknown or not running.
	MainPID(ctx context.Context, name string) (int, error)
}

// ErrNotRunning is returned by MainPID for a service without a running main process.
var ErrNotRunning = errors.New("hostos: service not running")

// Systemd is the ServiceManager of Linux: systemctl show --property=MainPID --value.
type Systemd struct{ Runner Runner }

// MainPID implements ServiceManager.
func (s Systemd) MainPID(ctx context.Context, name string) (int, error) {
	out, err := s.Runner.Run(ctx, Cmd{Path: "/usr/bin/systemctl", Args: []string{"show", "--property=MainPID", "--value", name}, Timeout: probeTimeoutAll})
	if err != nil || out.ExitCode != 0 {
		return 0, ErrNotRunning
	}
	return positive(strings.TrimSpace(string(out.Stdout)))
}

// Launchd is the ServiceManager of macOS: launchctl print system/<label>.
type Launchd struct{ Runner Runner }

// MainPID implements ServiceManager.
func (l Launchd) MainPID(ctx context.Context, name string) (int, error) {
	out, err := l.Runner.Run(ctx, Cmd{Path: "/bin/launchctl", Args: []string{"print", "system/" + name}, Timeout: probeTimeoutAll})
	if err != nil || out.ExitCode != 0 {
		return 0, ErrNotRunning
	}
	return ParseLaunchctlPID(out.Stdout)
}

var launchctlPID = regexp.MustCompile(`(?m)^\s*pid = (\d+)\s*$`)

// ParseLaunchctlPID returns the "pid = N" line of the output of launchctl print.
func ParseLaunchctlPID(b []byte) (int, error) {
	m := launchctlPID.FindSubmatch(b)
	if m == nil {
		return 0, ErrNotRunning
	}
	return positive(string(m[1]))
}

func positive(s string) (int, error) {
	n, err := strconv.Atoi(s)
	if err != nil || n <= 0 {
		return 0, ErrNotRunning
	}
	return n, nil
}

// probeTimeoutAll bounds a service manager query (protocol constant of the probes).
const probeTimeoutAll = 10 * time.Second
