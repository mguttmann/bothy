// SPDX-License-Identifier: Apache-2.0

package pg

import (
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// UUID converts id into a parameter of a uuid column.
func UUID(id ids.ID) pgtype.UUID { return pgtype.UUID{Bytes: id, Valid: true} }

// NullableUUID converts an optional id into a parameter: the zero ID becomes NULL.
func NullableUUID(id ids.ID) pgtype.UUID { return pgtype.UUID{Bytes: id, Valid: !id.IsZero()} }

// IDOf converts a uuid column; NULL becomes the zero ID.
func IDOf(u pgtype.UUID) ids.ID {
	if u.Valid {
		return ids.ID(u.Bytes)
	}
	return ids.ID{}
}

// TimeOf converts a timestamptz column into UTC; NULL becomes the zero time.
func TimeOf(t pgtype.Timestamptz) time.Time {
	if t.Valid {
		return t.Time.UTC()
	}
	return time.Time{}
}
