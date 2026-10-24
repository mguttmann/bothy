// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"

	"github.com/OWNER/bothy/services/server/internal/files"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/searchkit"
)

// SearchReadStore is the part of Repository that answers the read operations for the search
// coordinator (files.SearchReads, M3-14; docs/spec/14-search.md section 10). Its reads carry the
// visibility predicate over the channels the coordinator passes and authorize nothing (07 7.4
// rule 1); they run in the ambient unit of work of the coordinator.
type SearchReadStore interface {
	// SearchFiles runs the one statement of files.SearchReads.SearchFiles and returns the page with
	// its segments split at q.Markers. Errors: searchkit.ErrTimeout at the statement timeout, the
	// context error when ctx ended, otherwise a kernel/errs error.
	SearchFiles(ctx context.Context, q searchkit.Query) (searchkit.Page, error)
	// HitFiles returns the deliverable files of fileIDs attached in channelIDs with their
	// thumbnails, in the order of fileIDs; the rest is absent.
	HitFiles(ctx context.Context, fileIDs, channelIDs []ids.ID) ([]FileRecord, error)
}

// Compile-time check: the Service is the value the search coordinator reads files through.
var _ files.SearchReads = (*Service)(nil)

// SearchFiles implements files.SearchReads without authorization (the coordinator passes the
// visible channels and checks each delivered hit).
func (s *Service) SearchFiles(ctx context.Context, q searchkit.Query) (searchkit.Page, error) {
	return s.d.Repo.SearchFiles(ctx, q)
}

// HitFiles implements files.SearchReads without authorization: the attached form of getFile
// (attachedView, as Get renders an attached file) of each file the store returns, in the order of
// fileIDs. No id costs no statement.
func (s *Service) HitFiles(ctx context.Context, fileIDs, channelIDs []ids.ID) ([]files.File, error) {
	out := []files.File{}
	if len(fileIDs) == 0 || len(channelIDs) == 0 {
		return out, nil
	}
	recs, err := s.d.Repo.HitFiles(ctx, fileIDs, channelIDs)
	if err != nil {
		return nil, err
	}
	for _, f := range recs {
		out = append(out, attachedView(f))
	}
	return out, nil
}
