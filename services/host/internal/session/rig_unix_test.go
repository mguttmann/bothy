// SPDX-License-Identifier: Apache-2.0

//go:build linux || darwin

package session

import (
	"runtime"
	"syscall"
)

var sigZero = syscall.Signal(0)

func currentGOOS() string { return runtime.GOOS }
