// SPDX-License-Identifier: Apache-2.0

package wire

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestFileCredentials: the off-site key of the mirror comes from access_key_id and
// secret_access_key, trimmed, read on every call; a missing file is an error that names the
// file below offsite/ and not the directory.
func TestFileCredentials(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	creds := fileCredentials(dir)
	if _, _, err := creds(context.Background()); err == nil || !strings.Contains(err.Error(), "offsite/access_key_id") || strings.Contains(err.Error(), dir) {
		t.Errorf("without files: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "access_key_id"), []byte(" EXAMPLEKEYID\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := creds(context.Background()); err == nil || !strings.Contains(err.Error(), "offsite/secret_access_key") || strings.Contains(err.Error(), dir) {
		t.Errorf("without the secret: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "secret_access_key"), []byte("EXAMPLESECRET\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	id, key, err := creds(context.Background())
	if err != nil || id != "EXAMPLEKEYID" || key != "EXAMPLESECRET" {
		t.Errorf("credentials %q %q, %v", id, key, err)
	}
}
