// SPDX-License-Identifier: Apache-2.0

//go:build linux || darwin

package localchan

import (
	"errors"
	"net"
	"os"
	"strconv"
)

// ActivatedListener returns the listening socket the service manager passed: on Linux fd 3 when
// listenPID (LISTEN_PID) is this process and listenFDs (LISTEN_FDS) is 1 (systemd socket
// activation); on macOS standard input (launchd Sockets with inetdCompatibility Wait = true). The
// caller reads the variables, because only cmd/bothy-host reads the environment.
func ActivatedListener(goos, listenPID, listenFDs string) (net.Listener, error) {
	fd := 0
	if goos == "linux" {
		if listenPID != strconv.Itoa(os.Getpid()) || listenFDs != "1" {
			return nil, errors.New("localchan: no socket from the service manager")
		}
		fd = 3
	}
	f := os.NewFile(uintptr(fd), "activated")
	defer func() { _ = f.Close() }()
	l, err := net.FileListener(f)
	if err != nil {
		return nil, errors.New("localchan: no socket from the service manager")
	}
	return l, nil
}

// InheritedListener returns the listening socket a parent passed as fd 3 (os/exec ExtraFiles): the
// PTY helper of the owner account mode is a child of the host service (ADR 0073, D-OA-4), which
// creates the socket in its state directory.
func InheritedListener() (net.Listener, error) {
	f := os.NewFile(3, "inherited")
	if f == nil {
		return nil, errors.New("localchan: no inherited socket")
	}
	defer func() { _ = f.Close() }()
	l, err := net.FileListener(f)
	if err != nil {
		return nil, errors.New("localchan: no inherited socket")
	}
	return l, nil
}
