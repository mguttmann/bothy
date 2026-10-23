// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/runs"
)

// Compile-time check: Service serves the delivery reads of the gateway.
var _ runs.LiveBox = (*Service)(nil)

// Documents implements runs.LiveBox (M2-10 design note 10): the documents of the active runs of
// threadIDs, each built by the projection getRun answers with. It does not authorize: the gateway
// checked gateway.focus per thread and admits every recipient with run.read before it sends (07
// section 7.4 rule 1, exception "loading a resource that has already been checked"). Reads join
// one snapshot through the unit of work, so a document is never torn between a patch.
func (s *Service) Documents(ctx context.Context, threadIDs []ids.ID) ([]runs.Run, error) {
	if len(threadIDs) == 0 {
		return nil, nil
	}
	var out []runs.Run
	err := s.d.UoW.Do(ctx, func(ctx context.Context) error {
		out = nil
		rows, err := s.d.Repo.ActiveRunsOfThreads(ctx, threadIDs)
		if err != nil {
			return err
		}
		for _, row := range rows {
			doc, err := s.documentOf(ctx, row)
			if err != nil {
				return err
			}
			out = append(out, doc)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// Document implements runs.LiveBox: the document of runID while the run is active; false for an
// unknown or ended run. Like Documents it does not authorize.
func (s *Service) Document(ctx context.Context, runID ids.ID) (runs.Run, bool, error) {
	var doc runs.Run
	var active bool
	err := s.d.UoW.Do(ctx, func(ctx context.Context) error {
		row, found, err := s.d.Repo.Run(ctx, runID)
		if err != nil || !found || !row.State.Active() {
			active = false
			return err
		}
		doc, err = s.documentOf(ctx, row)
		active = err == nil
		return err
	})
	if err != nil {
		return runs.Run{}, false, err
	}
	return doc, active, nil
}
