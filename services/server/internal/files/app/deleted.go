// SPDX-License-Identifier: Apache-2.0

package app

import "context"

// markDeleted is the one place of the module that writes files.deleted_at (brief M3-07 design
// note 7 c): the files of d that are not deleted yet get the clock time, in the caller's unit of
// work. Callers: DeleteForMessage (a deleted message takes its
// attachments, 12 section 12) and ReleaseImage (a replaced avatar or logo, M3-10). It writes no
// event and touches no object; files.purge_objects removes the objects later (M3-08). M3-08
// (deleteFile, the cleanup jobs) and M3-15 (the extracted text) add their deletions here, so the
// rules of a deletion live in one function.
func (s *Service) markDeleted(ctx context.Context, d Deletion) error {
	if len(d.FileIDs) == 0 {
		return nil
	}
	_, err := s.d.Repo.MarkFilesDeleted(ctx, d, s.now())
	return err
}
