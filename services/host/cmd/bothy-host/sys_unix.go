// SPDX-License-Identifier: Apache-2.0

//go:build linux || darwin

package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"syscall"

	"golang.org/x/sys/unix"

	"github.com/OWNER/bothy/services/host/internal/hostos"
	"github.com/OWNER/bothy/services/host/internal/install"
	"github.com/OWNER/bothy/services/host/internal/localchan"
)

func isAdmin() bool    { return os.Geteuid() == 0 }
func callerID() string { return strconv.Itoa(os.Geteuid()) }
func adminUID() string { return "0" }

// interactive tells that standard input is a terminal (a character device).
func interactive() bool {
	fi, err := os.Stdin.Stat()
	return err == nil && fi.Mode()&os.ModeCharDevice != 0
}

// osVersion returns the kernel name and release.
func osVersion() string {
	var u unix.Utsname
	if unix.Uname(&u) != nil {
		return ""
	}
	return unix.ByteSliceToString(u.Sysname[:]) + " " + unix.ByteSliceToString(u.Release[:])
}

func serviceManager(r hostos.Runner) hostos.ServiceManager {
	if runtime.GOOS == hostos.Darwin {
		return hostos.Launchd{Runner: r}
	}
	return hostos.Systemd{Runner: r}
}

func installOps(r hostos.Runner, users hostos.Users) install.Ops {
	return install.OSOps{Runner: r, Users: users}
}

// asServiceUser runs this program with args as uid and gid, with an empty environment.
func asServiceUser(ctx context.Context, uid, gid int, args []string, stdin []byte, stdout, stderr io.Writer) (int, error) {
	exe, err := os.Executable()
	if err != nil {
		return 0, err
	}
	cmd := exec.CommandContext(ctx, exe, args...) //nolint:gosec // this program
	cmd.Env = []string{}
	cmd.Stdout, cmd.Stderr = stdout, stderr
	if stdin != nil {
		cmd.Stdin = bytes.NewReader(stdin)
	}
	cmd.SysProcAttr = &syscall.SysProcAttr{Credential: &syscall.Credential{Uid: uint32(uid), Gid: uint32(gid)}} //nolint:gosec // IDs of the service user
	err = cmd.Run()
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		return ee.ExitCode(), nil
	}
	return 0, err
}

// inheritedListener returns the socket the host service passed to its child helper (owner
// account mode, ADR 0073).
func inheritedListener() (net.Listener, error) { return localchan.InheritedListener() }

// helperListener returns the socket the service manager passed.
func helperListener(string, string) (net.Listener, error) {
	return localchan.ActivatedListener(runtime.GOOS, sys.getenv("LISTEN_PID"), sys.getenv("LISTEN_FDS"))
}

// profileExtras returns the lines of /etc/shells (a login shell must be listed there, Design note
// 9 of M2-45); there is no environment block on Linux and macOS.
func profileExtras() ([]string, []string, error) {
	b, err := os.ReadFile("/etc/shells")
	if err != nil {
		return nil, nil, err
	}
	var shells []string
	for _, l := range strings.Split(string(b), "\n") {
		if l = strings.TrimSpace(l); l != "" && !strings.HasPrefix(l, "#") {
			shells = append(shells, l)
		}
	}
	return shells, nil, nil
}
