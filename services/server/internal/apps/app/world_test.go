// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"os"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/platform/pg/pgtest"
	"github.com/OWNER/bothy/services/server/test/appsworld"
)

// TestMain starts the PostgreSQL container of the integration tests and removes it at the end.
func TestMain(m *testing.M) { os.Exit(pgtest.Main(m)) }

// The helpers of the shared world under the names the tests use.
var (
	newWorld        = appsworld.NewWorld
	idem            = appsworld.Idem
	manifest        = appsworld.Manifest
	installBody     = appsworld.InstallBody
	trackerManifest = appsworld.TrackerManifest
	equalList       = appsworld.EqualList
	scopesBody      = appsworld.ScopesBody
	threadOf        = appsworld.ThreadOf
	messageIn       = appsworld.MessageIn
	targetIn        = appsworld.TargetIn
	trackerWithG6   = appsworld.TrackerWithG6
)

// The allowed hosts of the world.
const (
	allowedHost = appsworld.AllowedHost
	ipHost      = appsworld.IPHost
	longHost    = appsworld.LongHost
)
