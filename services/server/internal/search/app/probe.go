// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"errors"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/searchkit"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
	"github.com/OWNER/bothy/services/server/internal/search"
)

// probeTerm is the fixed term of the test query; no content of anyone.
const probeTerm = "status"

// probePageSize is the page size of the test query: one row is enough to run the statement.
const probePageSize = 1

// Compile-time check: the coordinator is the test query of the status page as well.
var _ search.Probe = (*Coordinator)(nil)

// Probe implements search.Probe (M7-09): search_tsquery over search.languages and the message
// search of the index in one unit of work under search.statement_timeout, restricted to the zero
// channel id, which no channel has, so the statement runs against the index and returns nothing.
// It neither authorizes nor records bothy.search.duration, because no principal searches.
func (c *Coordinator) Probe(ctx context.Context) error {
	err := c.d.UoW.Do(ctx, func(ctx context.Context) error {
		tsq, ok, err := c.d.Index.BuildQuery(ctx, probeTerm)
		if err != nil {
			return err
		}
		if !ok {
			return errors.New("search: the test query has no term")
		}
		markers, err := searchkit.NewMarkers(c.d.Random)
		if err != nil {
			return err
		}
		_, err = c.d.Index.Messages(ctx, searchkit.Query{
			TSQuery: tsq, ChannelIDs: []ids.ID{{}}, Sort: searchkit.SortNewest, RefTime: c.d.Clock.Now(),
			Markers: markers, Limits: c.limits(probePageSize),
		})
		return err
	}, txn.StatementTimeout(c.cfg.StatementTimeout))
	return timeoutError(err)
}
