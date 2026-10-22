// SPDX-License-Identifier: Apache-2.0

// Package importer_test holds the integration tests of the import interface of the server
// (M8-03) on the shared harness of M1-71: the real wire graph with PostgreSQL 18 through pgtest
// and the HTTP chain with request and response validation. Each test starts its own server with
// Robin as owner, Alex as a person without grants and the agent Lotse.
package importer_test

import (
	"os"
	"testing"

	"github.com/OWNER/bothy/services/server/test/harness"
)

func TestMain(m *testing.M) { os.Exit(harness.Main(m)) }
