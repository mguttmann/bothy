// SPDX-License-Identifier: Apache-2.0

// Package behavior_test holds the integration tests of channel behaviour and agent configuration
// (M2-02) on the shared harness of M1-71: a real server process with PostgreSQL 18 through
// pgtest, the matrix world of BuildMatrixWorld and the HTTP chain with response validation. It
// lives in its own package because test/integration runs on pgtest.Main without the harness
// (docs/DECISIONS.md D-M2-02-12).
package behavior_test

import (
	"os"
	"testing"

	"github.com/OWNER/bothy/services/server/test/harness"
)

func TestMain(m *testing.M) { os.Exit(harness.Main(m)) }
