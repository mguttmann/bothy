// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"errors"

	"github.com/OWNER/bothy/services/server/internal/chat"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// Guard implements chat.ReadOnlyGuard over the store (22 12.1, M8-03 design note 4): a channel is
// controlled by an import lead while it is the target of a posted channel item of a source that
// is neither completed nor failed. It needs only the store, so internal/wire builds it before
// chat, whose use cases it serves (the direction of 02 R-AR-04).
type Guard struct {
	store Store
}

// Compile-time check: the source port of chat.
var _ chat.ReadOnlyGuard = (*Guard)(nil)

// NewGuard returns the guard over store. Errors: a nil store.
func NewGuard(store Store) (*Guard, error) {
	if store == nil {
		return nil, errors.New("importer: NewGuard needs the store")
	}
	return &Guard{store: store}, nil
}

// ImportControlled implements chat.ReadOnlyGuard.
func (g *Guard) ImportControlled(ctx context.Context, channelID ids.ID) (bool, error) {
	return g.store.ControlledChannel(ctx, channelID)
}
