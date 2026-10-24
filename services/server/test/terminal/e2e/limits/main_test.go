// SPDX-License-Identifier: Apache-2.0

// Package limits runs TM-T11 in the stack profile limits (terminal.idle_timeout, terminal.max_session_duration and terminal.detach_grace short), with the manipulated host service (brief M2-48 Design note 2), against one stack with real host services
// in Linux containers and the Go test device.
package limits

import (
	"os"
	"testing"

	"github.com/OWNER/bothy/services/server/test/terminal/suite"
)

// TestMain starts the one stack of the package and removes it after the tests.
func TestMain(m *testing.M) { os.Exit(suite.Main(m, "limits", "limits")) }
