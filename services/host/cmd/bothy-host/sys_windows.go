// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"errors"
	"io"
	"net"

	"golang.org/x/sys/windows"

	"github.com/OWNER/bothy/services/host/internal/hostos"
	"github.com/OWNER/bothy/services/host/internal/install"
	"github.com/OWNER/bothy/services/host/internal/localchan"
	"github.com/OWNER/bothy/services/host/internal/profiles"
)

func isAdmin() bool { return windows.GetCurrentProcessToken().IsElevated() }

func callerID() string {
	u, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		return ""
	}
	return u.User.Sid.String()
}

func adminUID() string { return "" }

func interactive() bool {
	var mode uint32
	return windows.GetConsoleMode(windows.Stdin, &mode) == nil
}

func osVersion() string {
	maj, min, build := windows.RtlGetNtVersionNumbers()
	return "Windows " + itoa(int(maj)) + "." + itoa(int(min)) + "." + itoa(int(build))
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}

func serviceManager(hostos.Runner) hostos.ServiceManager { return hostos.SCM{} }

func installOps(r hostos.Runner, users hostos.Users) install.Ops {
	return &install.OSOps{Runner: r, Users: users}
}

// asServiceUser has no child process on Windows: doctor starts the service with doctor-once.
func asServiceUser(context.Context, int, int, []string, []byte, io.Writer, io.Writer) (int, error) {
	return 0, errors.New("not on Windows")
}

// helperListener creates the pipe of the helper with FILE_FLAG_FIRST_PIPE_INSTANCE and a DACL for
// the service account only.
func helperListener(name, serviceSID string) (net.Listener, error) {
	return localchan.ListenPipe(localchan.HelperEndpoint(hostos.Windows, name), localchan.HelperPipeSDDL(serviceSID))
}

// profileExtras returns the environment block of the helper's token without inheritance (Design
// note 8 of M2-45); Windows has no /etc/shells.
func profileExtras() ([]string, []string, error) {
	block, err := profiles.TokenEnvironment()
	return nil, block, err
}
