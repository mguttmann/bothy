// SPDX-License-Identifier: Apache-2.0

package main

import (
	"os"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/platform/pg/pgtest"
)

// TestMain runs the tests with a PostgreSQL container for the test of the scheduler loop that
// reads backup_runs (plan P-09: without a Docker-compatible runtime the binary fails, it never
// skips). The container starts with the first test that asks for a database.
func TestMain(m *testing.M) { os.Exit(pgtest.Main(m)) }
