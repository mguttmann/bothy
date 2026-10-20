// SPDX-License-Identifier: Apache-2.0

// Package multinode holds the integration tests of the terminal across two api nodes (M2-47,
// TM-T15 at the server level): two in-process servers of internal/wire in role api with
// cluster.mode multi on one PostgreSQL database of pgtest, an in-process test edge in front of
// them, the Fake-Host of test/terminal/fakehost and the device client of M2-44's relay tests.
package multinode

import (
	"os"
	"testing"

	"github.com/OWNER/bothy/services/server/test/harness"
)

func TestMain(m *testing.M) { os.Exit(harness.Main(m)) }
