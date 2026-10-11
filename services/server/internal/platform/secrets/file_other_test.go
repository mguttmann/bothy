// SPDX-License-Identifier: Apache-2.0

//go:build !unix

package secrets_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/platform/secrets"
)

// TestFileDriverFailsClosedOnThisPlatform: without unix ownership the driver cannot enforce
// SEC-SEC-1, so it refuses every file reference, whether the file exists or not.
func TestFileDriverFailsClosedOnThisPlatform(t *testing.T) {
	t.Parallel()
	ref := secrets.Ref{Scheme: "file", Path: "/EXAMPLE/secret"}
	v, _, err := secrets.NewFileDriver().Get(context.Background(), ref)
	if !errors.Is(err, secrets.ErrInsecureFile) {
		t.Fatalf("Get error = %v, want ErrInsecureFile", err)
	}
	if !strings.Contains(err.Error(), "file ownership cannot be checked on this platform") {
		t.Fatalf("message %q does not say why", err)
	}
	if v.Len() != 0 {
		t.Fatal("Get returned a value on a platform without ownership checks")
	}
}
