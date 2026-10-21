// SPDX-License-Identifier: Apache-2.0

// Package hostpolicy runs TM-T21, TM-T24 and TM-T29 (the rules of the host service on standstill,
// foreign homes and shared locations; brief M2-48 Design note 2) against one stack with real host
// services in Linux containers and the Go test device.
package hostpolicy

import (
	"os"
	"testing"

	"github.com/OWNER/bothy/services/server/test/terminal/suite"
)

// TestMain starts the one stack of the package and removes it after the tests.
func TestMain(m *testing.M) { os.Exit(suite.Main(m, "hostpolicy", "default")) }
