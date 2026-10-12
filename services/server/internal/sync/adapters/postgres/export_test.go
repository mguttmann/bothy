// SPDX-License-Identifier: Apache-2.0

package postgres

import (
	"context"

	"github.com/jackc/pgx/v5"
)

// SetDialForTest replaces how the tail opens its LISTEN connection; call it before Run.
func (t *Tail) SetDialForTest(dial func(ctx context.Context) (*pgx.Conn, error)) { t.dial = dial }
