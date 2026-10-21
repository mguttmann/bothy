// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"

	"github.com/OWNER/bothy/services/server/internal/files"
	"github.com/OWNER/bothy/services/server/internal/files/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// State implements files.Uploads (brief M3-03 design note 11, docs/DECISIONS.md D-M3-03-13):
// lookup and authorize as PutPart; an aborted or expired session, or an open one past
// expires_at, is 410 gone; otherwise the view with the confirmed parts in ascending order.
func (s *Service) State(ctx context.Context, fileID ids.ID) (files.UploadState, error) {
	a := caller(ctx)
	rec, err := s.ownSession(ctx, a, fileID, authorize.ActionFileUpload)
	if err != nil {
		return files.UploadState{}, err
	}
	if s.expired(rec, s.now()) {
		return files.UploadState{}, gone()
	}
	parts, err := s.d.Repo.Parts(ctx, fileID)
	if err != nil {
		return files.UploadState{}, err
	}
	view, err := domain.View(rec.State, len(parts))
	if err != nil {
		return files.UploadState{}, gone()
	}
	confirmed := make([]files.PartRef, 0, len(parts))
	for _, p := range parts {
		confirmed = append(confirmed, files.PartRef{PartNumber: p.PartNumber, ETag: partETag(p)})
	}
	return files.UploadState{FileID: fileID, View: view, PartSizeBytes: rec.PartSizeBytes, Confirmed: confirmed, ExpiresAt: rec.ExpiresAt}, nil
}
