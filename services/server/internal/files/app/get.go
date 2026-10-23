// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"

	"github.com/OWNER/bothy/services/server/internal/files"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// Get implements files.Files (brief M3-03 design note 15): the file without deleted_at (404
// otherwise), its attachment row, file.read with its facts (on the path of the message once the
// file is attached, brief M3-07 design note 9), then the metadata in every state, without
// scope_channel_id once attached. Bytes are M3-05's.
func (s *Service) Get(ctx context.Context, fileID ids.ID) (files.File, error) {
	a := caller(ctx)
	f, found, err := s.d.Repo.File(ctx, fileID)
	if err != nil {
		return files.File{}, err
	}
	if !found {
		return files.File{}, notFound()
	}
	ref, err := s.attachmentRef(ctx, fileID)
	if err != nil {
		return files.File{}, err
	}
	d, err := s.decide(ctx, a, authorize.ActionFileRead, fileResource(f, ref))
	if err != nil {
		return files.File{}, err
	}
	if err := d.Err(); err != nil {
		return files.File{}, err
	}
	if ref != nil {
		return attachedView(f), nil
	}
	return FileView(f), nil
}
