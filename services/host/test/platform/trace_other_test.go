// SPDX-License-Identifier: Apache-2.0

//go:build platform && !linux

package platform

import "testing"

// traceService has no system call trace on macOS and Windows (the brief names that part of
// TM-T25 as not provable there).
func traceService(*testing.T) (func() string, bool) { return nil, false }
