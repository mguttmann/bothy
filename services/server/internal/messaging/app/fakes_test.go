// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"context"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/messaging/app"
	"github.com/OWNER/bothy/services/server/internal/messaging/domain"
)

// bareStore is the base of the fake stores of the unit tests: no message has reactions or a pin
// (M1-59), every other method is the nil app.Store and panics when called.
type bareStore struct{ app.Store }

func (bareStore) ReactionGroups(context.Context, []ids.ID, []ids.ID) (map[ids.ID][]domain.ReactionGroup, error) {
	return map[ids.ID][]domain.ReactionGroup{}, nil
}

func (bareStore) OwnReactions(context.Context, ids.ID, []ids.ID, []ids.ID) (map[ids.ID]map[string]bool, error) {
	return map[ids.ID]map[string]bool{}, nil
}

func (bareStore) Pinned(context.Context, []ids.ID, []ids.ID) (map[ids.ID]bool, error) {
	return map[ids.ID]bool{}, nil
}

// anySymbol is the symbol normalizer of the unit tests: it accepts every symbol unchanged.
type anySymbol struct{}

func (anySymbol) Normalize(raw string, _ int) (string, error) { return raw, nil }
