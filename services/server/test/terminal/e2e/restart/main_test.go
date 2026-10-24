// SPDX-License-Identifier: Apache-2.0

// Package restart runs TM-T16 on its own stack, because it restarts the server (brief M2-48 Design note 2), against one stack with real host services
// in Linux containers and the Go test device.
package restart

import (
	"os"
	"testing"

	"github.com/OWNER/bothy/services/server/test/terminal/suite"
)

// TestMain starts the one stack of the package and removes it after the tests.
func TestMain(m *testing.M) { os.Exit(suite.Main(m, "restart", "default")) }
