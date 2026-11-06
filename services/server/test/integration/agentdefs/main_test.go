// SPDX-License-Identifier: Apache-2.0

// Package agentdefs_test holds the integration tests of the definitions of built-in agents
// (A1-02, docs/spec/26-builtin-agents.md 4) on the shared harness: a real server process with
// PostgreSQL 18 through pgtest, owners with a real step-up, agents with DPoP-bound API tokens and
// the REST consumer of an agent.
package agentdefs_test

import (
	"os"
	"testing"

	"github.com/OWNER/bothy/services/server/test/harness"
)

func TestMain(m *testing.M) { os.Exit(harness.Main(m)) }
