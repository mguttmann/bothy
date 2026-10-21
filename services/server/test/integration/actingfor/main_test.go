// SPDX-License-Identifier: Apache-2.0

// Package actingfor_test holds the cross-module integration tests of the acting-for binding
// (07 guard rule SR9, brief M2-14 "Tests", block "Integration"): the real internal/wire graph on
// the shared harness (M1-71) with PostgreSQL 18 through pgtest, the matrix world of
// harness.BuildMatrixWorld, agents with DPoP-bound API tokens, and invocation tokens delivered
// through the REST poll of the agent consumers (M2-07, M2-08) and started with startRun (M2-09).
//
// A harness server with its world takes minutes to start under load, so the tests share a few
// servers: each top-level test starts one and runs the cases of the brief as ordered subtests
// named after them. The top-level names keep the prefixes TestAZ and TestSR9, so the run
// patterns of the brief select them.
package actingfor_test

import (
	"os"
	"testing"

	"github.com/OWNER/bothy/services/server/test/harness"
)

func TestMain(m *testing.M) { os.Exit(harness.Main(m)) }
