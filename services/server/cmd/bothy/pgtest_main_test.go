// SPDX-License-Identifier: Apache-2.0

package main

import (
	"os"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/platform/pg/pgtest"
)

// TestMain runs the tests of the command with a PostgreSQL container for the tests of bothy
// migrate (plan P-09: without a Docker-compatible runtime the binary fails, it never skips).
func TestMain(m *testing.M) { os.Exit(pgtest.Main(m)) }
