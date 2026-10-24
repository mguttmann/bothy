// SPDX-License-Identifier: Apache-2.0

// Package twonode runs TM-T15 on M2-47's two-node overlay with the example edge (profile twonode) (brief M2-48 Design note 2), against one stack with real host services
// in Linux containers and the Go test device.
package twonode

import (
	"os"
	"testing"

	"github.com/OWNER/bothy/services/server/test/terminal/suite"
)

// TestMain starts the one stack of the package and removes it after the tests.
func TestMain(m *testing.M) { os.Exit(suite.Main(m, "twonode", "twonode")) }
