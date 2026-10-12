// SPDX-License-Identifier: Apache-2.0

package migrations_test

import (
	"testing"

	"github.com/OWNER/bothy/services/server/internal/instance/domain"
	"github.com/OWNER/bothy/services/server/internal/platform/buildinfo"
	"github.com/OWNER/bothy/services/server/migrations"
)

// TestMinServerVersionNotAboveOwnVersion: the schema lock bothy migrate writes is valid SemVer
// and never above the version of the binary that writes it, or a fresh installation would never
// become ready (18 "Komponenten" rule 6, upgrade.md "Schema-Prüfung beim Start").
func TestMinServerVersionNotAboveOwnVersion(t *testing.T) {
	t.Parallel()
	c, err := domain.CompareSemVer(migrations.MinServerVersion, buildinfo.Version())
	if err != nil {
		t.Fatalf("MinServerVersion %q: %v", migrations.MinServerVersion, err)
	}
	if c > 0 {
		t.Fatalf("MinServerVersion %s is above the build version %s", migrations.MinServerVersion, buildinfo.Version())
	}
}
