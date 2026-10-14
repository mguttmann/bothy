// SPDX-License-Identifier: Apache-2.0

package filestest_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/files/adapters/filestest"
	"github.com/OWNER/bothy/services/server/internal/files/adapters/fs"
	"github.com/OWNER/bothy/services/server/internal/files/app"
)

// TestConformanceSuiteRunsAgainstTheFileSystemDriver runs the suite once from its own package, so
// its cases are exercised by the package's own tests and not only by the drivers that import it.
func TestConformanceSuiteRunsAgainstTheFileSystemDriver(t *testing.T) {
	t.Parallel()
	filestest.RunObjectStoreConformance(t, func(t *testing.T) app.ObjectStore {
		root := filepath.Join(t.TempDir(), "files")
		if err := os.Mkdir(root, 0o700); err != nil {
			t.Fatal(err)
		}
		s, err := fs.Open(fs.Config{Root: root})
		if err != nil {
			t.Fatalf("Open: %v", err)
		}
		t.Cleanup(func() { _ = s.Close() })
		return s
	}, filestest.Options{PartSize: 4096})
}
