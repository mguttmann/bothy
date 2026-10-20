// SPDX-License-Identifier: Apache-2.0

// Package conformance runs the conformance scenarios of the terminal suite end to end (brief M2-48
// Design note 2): TM-T03 to TM-T10, TM-T12, TM-T17, TM-T19, TM-T20, TM-T22, TM-T23, TM-T28, AZ-M54
// to AZ-M61 and ID-T28 against one stack of the profile default with real host services in Linux
// containers and the Go test device, and the tests of requirements 21, 23 and 26.
package conformance

import (
	"os"
	"testing"

	"github.com/OWNER/bothy/services/server/test/terminal/suite"
)

// TestMain starts the one stack of the package and removes it after the tests.
func TestMain(m *testing.M) { os.Exit(suite.Main(m, "conformance", "default")) }
