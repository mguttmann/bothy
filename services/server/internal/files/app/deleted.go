// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"time"

	"github.com/OWNER/bothy/services/server/internal/files/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/eventlog"
)

// deletion are the collaborators of markDeleted; the Service and the cleanup hold the same ones.
type deletion struct {
	repo     Repository
	redactor eventlog.Redactor
	purge    PurgeQueue
}

// deletion returns the collaborators of markDeleted of the use cases.
func (s *Service) deletion() deletion {
	return deletion{repo: s.d.Repo, redactor: s.d.Redactor, purge: s.d.Purge}
}

// markDeleted is the one place of the module that writes files.deleted_at (brief M3-07 design
// note 7 c, brief M3-08 design note 2). See deletion.mark.
func (s *Service) markDeleted(ctx context.Context, d Deletion) error {
	return s.deletion().mark(ctx, d, s.now())
}

// mark sets deleted_at = now on the files of d that are not deleted yet, in the caller's unit of
// work, which locked their rows. Only the files this call marked take the further steps, so a
// second call has no effect: an open upload session becomes aborted (upload_state stays and no
// file.updated is written, because every reader answers 404 for a deleted file), the extracted
// text is deleted (file_text_extracts, M3-15: the text goes when the file goes), the stored
// file.updated events of the file are redacted (domain.RedactFileEvent, R-DM-45), and
// files.purge_objects is inserted, which removes the objects and then the row within
// privacy.object_delete_delay. Mark, redaction and job commit together; a rollback leaves none of
// them (02 8.5). It writes no event (05 6 has none for a deleted file) and touches no object.
//
// Callers: DeleteForMessage (a deleted message takes its attachments), ReleaseImage (a replaced
// avatar or logo, M3-10), Delete (deleteFile) and the cleanup steps of never attached files and of
// failed rows (M3-08).
func (d deletion) mark(ctx context.Context, sel Deletion, now time.Time) error {
	if len(sel.FileIDs) == 0 {
		return nil
	}
	marked, err := d.repo.MarkFilesDeleted(ctx, sel, now)
	if err != nil || len(marked) == 0 {
		return err
	}
	if err := d.repo.AbortOpenSessions(ctx, marked); err != nil {
		return err
	}
	// The extracted text goes when the file goes (file_text_extracts, M3-15, 14 5.3).
	if err := d.repo.DeleteTextExtractsOfDeleted(ctx, marked); err != nil {
		return err
	}
	for _, id := range marked {
		if _, err := d.redactor.Redact(ctx, string(authorize.ResourceFile), id, domain.RedactFileEvent); err != nil {
			return err
		}
		if err := d.purge.Enqueue(ctx, id); err != nil {
			return err
		}
	}
	return nil
}
