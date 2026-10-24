// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"

	"github.com/OWNER/bothy/services/server/internal/files"
	"github.com/OWNER/bothy/services/server/internal/files/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/eventlog"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
)

// attachmentReader reads the attachment row of a file by its unique key (Repository.AttachmentOf).
type attachmentReader interface {
	AttachmentOf(ctx context.Context, fileID ids.ID) (AttachmentRecord, bool, error)
}

// fileChanges are the collaborators of recordFileChange; every use case that changes the state of
// a file holds one (the abort of M3-03, the scan pass of M3-04, the image job of M3-06).
type fileChanges struct {
	attachments attachmentReader
	channels    ChannelLocator
	events      eventlog.Appender
	// extracts is the trigger of the text extraction (M3-15); nil in the use cases that never
	// make a file deliverable (the upload use cases of Service).
	extracts *ExtractTrigger
}

// changes returns the collaborators of recordFileChange of the upload use cases.
func (s *Service) changes() fileChanges {
	return fileChanges{attachments: s.d.Repo, channels: s.d.Channels, events: s.d.Events}
}

// recordFileChange is the one place that appends file.updated for a change of upload_state,
// scan_status, the thumbnails, the media values or the transcript of a file (12 section 2: every
// state change writes file.updated; brief M3-07 design note 10), inside the caller's unit of work,
// which holds the files row locked. It reads the attachment row of the file: an unattached file
// writes to the uploader in the scope of its scope channel, an attached one without audience in
// the scope of its message (the copies of message_attachments, the space of the copy channel) and
// in its attached form (domain.EventTarget). a is the actor of the caller, the zero actor for a
// job. In the same step it publishes files.FileFinalized exactly when the change makes the file
// final (domain.BecameFinal), so every finalizing write announces it once and runs.OnFileFinalized
// commits with it (02 8.3). When the change makes the file deliverable it also runs the trigger of
// the text extraction (docs/spec/12-files.md 3.4 last paragraph, 14 5.1 item 1), so every write
// that makes a file deliverable (the scan pass, the image job) creates the extract row and the job
// files.extract_text in its unit of work.
func (c fileChanges) recordFileChange(ctx context.Context, a actor.Actor, before, after FileRecord) error {
	row, attached, err := c.attachments.AttachmentOf(ctx, after.ID)
	if err != nil {
		return err
	}
	target := domain.EventTarget(attached)
	sc := EventScope{ChannelID: after.ScopeChannelID, Uploader: target.AudienceUploader}
	view := FileView(after)
	if target.ThreadScope {
		sc.ChannelID, sc.ThreadID = row.ChannelID, row.ThreadID
		view = attachedView(after)
	}
	if !sc.ChannelID.IsZero() {
		if sc.SpaceID, err = c.channels.SpaceOf(ctx, sc.ChannelID); err != nil {
			return err
		}
	}
	ev, err := FileUpdated(a, sc, view)
	if err != nil {
		return err
	}
	if err := c.events.Append(ctx, ev); err != nil {
		return err
	}
	if becameDeliverable(before, after) {
		if c.extracts == nil {
			return errs.New(errs.CodeInternalError, "files: a file became deliverable without the text extraction trigger")
		}
		if err := c.extracts.schedule(ctx, after); err != nil {
			return err
		}
	}
	if domain.BecameFinal(stateOf(before), stateOf(after)) {
		return txn.Publish(ctx, files.FileFinalized{FileID: after.ID})
	}
	return nil
}

// becameDeliverable reports a change that makes the file deliverable (12 section 2).
func becameDeliverable(before, after FileRecord) bool {
	return !domain.IsDeliverable(before.UploadState, before.ScanStatus, before.Deleted) &&
		domain.IsDeliverable(after.UploadState, after.ScanStatus, after.Deleted)
}

// stateOf returns the columns of f that decide whether it is final.
func stateOf(f FileRecord) domain.StateFacts {
	return domain.StateFacts{UploadState: f.UploadState, ScanStatus: f.ScanStatus, Deleted: f.Deleted}
}
