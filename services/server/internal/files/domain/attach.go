// SPDX-License-Identifier: Apache-2.0

package domain

import "github.com/OWNER/bothy/services/server/internal/kernel/ids"

// AttachFacts are the facts Attachable decides on (docs/spec/12-files.md section 10 point 1,
// SEC-UP-9, R-DM-22): the locked files row with the state of its upload session, the attachment
// row of the file and the message the file should hang on. A zero value is an unknown file.
type AttachFacts struct {
	// Found reports that the files row exists.
	Found bool
	// Deleted reports files.deleted_at IS NOT NULL.
	Deleted bool
	// UploadedBy is files.uploaded_by; AuthorID the author of the message.
	UploadedBy, AuthorID ids.ID
	// Purpose is files.purpose.
	Purpose string
	// ScopeChannelID is files.scope_channel_id (zero for NULL); ChannelID the channel of the
	// message.
	ScopeChannelID, ChannelID ids.ID
	// SessionState is file_upload_sessions.state; "" when the file has no session.
	SessionState SessionState
	// UploadState and ScanStatus are files.upload_state and files.scan_status.
	UploadState, ScanStatus string
	// Attached reports that a row of message_attachments exists for the file, or that the same
	// request listed the file before.
	Attached bool
}

// Attachable reports whether a file may hang on the message (12 10 point 1): the row exists, is
// not deleted, was uploaded by the author for the channel of the message as an attachment, its
// upload session is completed, its upload did not fail and it hangs on no message yet. The scan
// does not matter: blocked files (infected, error) and files whose scan still runs are
// attachable (12 10 point 1, 12 3.4 step 3), because a not final attachment holds the agent
// invocation instead (R-FI-09).
//
// Example:
//
//	ok := domain.Attachable(domain.AttachFacts{Found: true, UploadedBy: alex, AuthorID: alex,
//		Purpose: "attachment", ScopeChannelID: ch, ChannelID: ch, SessionState: domain.SessionCompleted,
//		UploadState: domain.UploadPending, ScanStatus: domain.ScanPending}) // true
func Attachable(f AttachFacts) bool {
	return f.Found && !f.Deleted && !f.AuthorID.IsZero() && f.UploadedBy == f.AuthorID &&
		f.Purpose == string(PurposeAttachment) && !f.ChannelID.IsZero() && f.ScopeChannelID == f.ChannelID &&
		f.SessionState == SessionCompleted && f.UploadState != UploadFailed && !f.Attached
}

// Target is where file.updated of a file goes (05 section 6 row file.updated, brief M3-03 design
// note 16).
type Target struct {
	// AudienceUploader reports that the event reaches the uploader only (AudiencePrincipalID),
	// because before attaching the scope alone would show the file to every reader of the
	// channel.
	AudienceUploader bool
	// ThreadScope reports that the scope carries the thread of the message besides its space
	// and channel, so every reader of the message receives the event.
	ThreadScope bool
}

// EventTarget returns the target of file.updated of a file that is attached or not: unattached
// goes to the uploader in the scope of files.scope_channel_id, attached goes without audience to
// the scope of its message (the copies of message_attachments, R-DM-43).
//
// Example:
//
//	t := domain.EventTarget(true) // Target{ThreadScope: true}
func EventTarget(attached bool) Target {
	return Target{AudienceUploader: !attached, ThreadScope: attached}
}

// StateFacts are the columns that decide whether a file is final (12 section 2).
type StateFacts struct {
	UploadState, ScanStatus string
	Deleted                 bool
}

// BecameFinal reports a change from a state that is not final to a final one (12 section 2,
// Final): exactly the writes that publish files.FileFinalized, so every file announces it once.
// A change of a file that was final already (thumbnails, media info) and a change that leaves it
// open (an image whose cleaned original is still missing) report false.
//
// Example:
//
//	pending := domain.StateFacts{UploadState: domain.UploadPending, ScanStatus: domain.ScanPending}
//	clean := domain.StateFacts{UploadState: domain.UploadComplete, ScanStatus: domain.ScanClean}
//	domain.BecameFinal(pending, clean) // true
func BecameFinal(before, after StateFacts) bool {
	return !Final(before.UploadState, before.ScanStatus, before.Deleted) && Final(after.UploadState, after.ScanStatus, after.Deleted)
}
