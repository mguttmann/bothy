// SPDX-License-Identifier: Apache-2.0

// Package textextract_test holds the integration tests of the text extraction of brief M3-15
// (section "Tests", Integration): the real server of internal/wire in role all through
// test/harness with the matrix world, uploads through the real HTTP chain, the scan pass with
// file.scan.mode off or the scripted clamd of M3-04, and the jobs run by the job runtime of the
// server; plus tests that build the use cases directly over a pgtest database (direct_test.go).
package textextract_test

import (
	"os"
	"testing"

	"github.com/OWNER/bothy/services/server/test/harness"
)

func TestMain(m *testing.M) { os.Exit(harness.Main(m)) }
