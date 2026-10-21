// SPDX-License-Identifier: Apache-2.0

package postgres

import (
	"context"

	"github.com/OWNER/bothy/services/server/internal/identity"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// ActiveDevicesOfUsers implements app.DeviceStore: one read for all people, inside the ambient
// transaction of ctx when there is one. The rows come ordered by person and id, so every list is
// ascending by id without sorting here (uuid order is byte order, the order of ids.ID).
func (r *Repository) ActiveDevicesOfUsers(ctx context.Context, userIDs []ids.ID) (map[ids.ID][]identity.DeviceRef, error) {
	out := map[ids.ID][]identity.DeviceRef{}
	if len(userIDs) == 0 {
		return out, nil
	}
	rows, err := r.read(ctx).ListActiveDevicesOfUsers(ctx, uuids(userIDs))
	if err != nil {
		return nil, mapError(err, "listing the active devices of people")
	}
	for _, row := range rows {
		user := ids.ID(row.UserID.Bytes)
		out[user] = append(out[user], identity.DeviceRef{ID: ids.ID(row.ID.Bytes), Platform: row.Platform})
	}
	return out, nil
}
