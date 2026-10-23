// SPDX-License-Identifier: Apache-2.0

// Package multinode holds the integration tests of the gateway across several api nodes (M7-17):
// two in-process servers of internal/wire in role api and one in role worker with cluster.mode
// multi on one PostgreSQL database of pgtest, an in-process test edge in front of the api nodes
// that routes each request and each gateway connection to the node the test names, and the
// matrix world of the harness.
package multinode

import (
	"os"
	"testing"

	"github.com/OWNER/bothy/services/server/test/harness"
)

func TestMain(m *testing.M) { os.Exit(harness.Main(m)) }
