// SPDX-License-Identifier: Apache-2.0

package postgres

import (
	"testing"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
)

// TestMapError_InvalidText: invalid UTF-8 or NUL in a text parameter (SQLSTATE 22021) is 400,
// never 500 (red team M1-31).
func TestMapError_InvalidText(t *testing.T) {
	t.Parallel()
	e, ok := errs.As(mapError(&pgconn.PgError{Code: "22021"}, "listing people"))
	if !ok || e.Code != errs.CodeValidationFailed {
		t.Fatalf("mapError(22021) = %v", e)
	}
}
