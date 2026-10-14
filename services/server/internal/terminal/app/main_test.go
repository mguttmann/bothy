// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"os"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/platform/pg/pgtest"
)

func TestMain(m *testing.M) { os.Exit(pgtest.Main(m)) }
