// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"os"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/platform/pg/pgtest"
)

// TestMain starts PostgreSQL for the signing key tests (P-09); the other tests of the package run
// without a database.
func TestMain(m *testing.M) { os.Exit(pgtest.Main(m)) }
