// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"

	"github.com/OWNER/bothy/services/server/internal/files"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// Get implements files.Files (brief M3-03 design note 15): the file without deleted_at (404
// otherwise), file.read with its facts, then the metadata in every state. Bytes are M3-05's.
func (s *Service) Get(ctx context.Context, fileID ids.ID) (files.File, error) {
	a := caller(ctx)
	f, found, err := s.d.Repo.File(ctx, fileID)
	if err != nil {
		return files.File{}, err
	}
	if !found {
		return files.File{}, notFound()
	}
	d, err := s.decide(ctx, a, authorize.ActionFileRead, fileResource(f))
	if err != nil {
		return files.File{}, err
	}
	if err := d.Err(); err != nil {
		return files.File{}, err
	}
	return FileView(f), nil
}
