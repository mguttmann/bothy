// SPDX-License-Identifier: Apache-2.0

package files

import (
	"context"
	"io"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// Register declares the durable event the files module appends (M1-25, docs/spec/05-actions.md
// section 6): file.updated, written by M3-03 when an upload is aborted and by M3-04 to M3-10 for
// the later states. internal/wire lists it in moduleRegistrations.
func Register(r *authorize.Registry) {
	r.Event("files", authorize.EventFileUpdated)
}

// Uploads is the upload half of docs/spec/12-files.md section 3 in mode direct (M3-03). The
// actor comes from kernel/actor in the context; every method authorizes it and answers an unknown
// and a foreign upload with the same 404 not_found. Errors are kernel/errs errors.
type Uploads interface {
	// Create announces an upload on a channel (12 3.1): authorize file.upload on the channel, the
	// size against file.max_upload_bytes, the denied types, the quota, the open sessions and the
	// free space, then the rows of files and file_upload_sessions. It joins the ambient unit of
	// work when one exists (a request with Idempotency-Key, M1-20), else it opens its own. It
	// writes no event and inserts no job.
	//
	// Example:
	//
	//	s, err := uploads.Create(ctx, files.CreateUpload{ChannelID: ch, Name: "report.pdf", SizeBytes: 48213, SHA256: sum})
	Create(ctx context.Context, cmd CreateUpload) (UploadSession, error)
	// PutPart stores one part in mode direct (12 3.2): it checks length and digest while reading
	// and stores nothing on a mismatch; a part that is already confirmed answers without reading
	// the body. It never runs inside a unit of work, because the object storage is called while
	// the body streams (02 8.5).
	PutPart(ctx context.Context, cmd PutPart) (PartRef, error)
	// State returns the confirmed parts of an own session (12 3.2 point 5).
	State(ctx context.Context, fileID ids.ID) (UploadState, error)
	// Complete closes an own session (12 3.4): every part must be confirmed. The file stays
	// pending until the verify pass of M3-04. It joins the ambient unit of work like Create.
	Complete(ctx context.Context, cmd CompleteUpload) (File, error)
	// Abort ends an own open session (abortUpload): the file becomes failed, its parts go and
	// file.updated reaches the uploader. Aborting an aborted or expired session again is no
	// error.
	Abort(ctx context.Context, fileID ids.ID) error
}

// Files reads file metadata (12 13, GET /api/v1/files/{id}) and, from M3-05 on, the bytes of a
// file and its thumbnails, the download tickets and the transcript (12 9). The actor comes from
// kernel/actor in the context; every method calls authorize(file.read) with the rights at call
// time and answers an unknown, a deleted and a foreign file with the same 404 not_found.
type Files interface {
	// Get returns the file with file.read for the actor of ctx; a deleted, unknown or invisible
	// file is 404 not_found.
	Get(ctx context.Context, fileID ids.ID) (File, error)
	// Open opens the bytes of a file or one of its thumbnails for a download (downloadFile,
	// downloadThumbnail; 12 9.1 and 9.3). An anonymous actor with req.Ticket set is replaced by
	// the actor of the ticket; every failure of the ticket path is the unknown-id 404. A file
	// that is not deliverable gives 409 file_not_ready (with Retry-After) or 409 file_blocked, a
	// range outside the variant 416 range_not_satisfiable. The caller closes Delivery.Body.
	//
	// Example:
	//
	//	d, err := f.Open(ctx, files.OpenRequest{FileID: id, Variant: "original", Range: "bytes=0-99"})
	Open(ctx context.Context, req OpenRequest) (Delivery, error)
	// IssueTicket mints a download ticket for variant of a deliverable file (createFileTicket,
	// 12 9.2). The actor needs a login session: a principal without one gets 403 forbidden with
	// reason step_up_not_possible. A variant the file does not have is 404 not_found.
	IssueTicket(ctx context.Context, fileID ids.ID, variant string) (IssuedTicket, error)
	// Transcript returns the transcript of a deliverable file (getTranscript, 12 8.2); it never
	// accepts a ticket.
	Transcript(ctx context.Context, fileID ids.ID) (TranscriptView, error)
	// Delete deletes a file that is not attached (deleteFile, 05 4.8 file.delete; 12 section 12).
	// The caller's own file that is deleted already answers nil (06 7), an unknown, invisible,
	// foreign deleted or purged file 404 not_found, a denial with read 403 forbidden, an attached
	// file 409 conflict with reason file_attached, and a file whose purpose is not attachment 409
	// conflict with reason purpose_not_deletable. The objects and the row go with
	// files.purge_objects within privacy.object_delete_delay (M3-08).
	//
	// Example:
	//
	//	err := f.Delete(ctx, fileID) // nil: 204
	Delete(ctx context.Context, fileID ids.ID) error
}

// CreateUpload is the request of createUpload (schema UploadCreate).
type CreateUpload struct {
	// ChannelID is the channel the file is uploaded for (files.scope_channel_id).
	ChannelID ids.ID
	// Name is the file name as the client sent it; it is cleaned (12 6.2) and never logged.
	Name string
	// SizeBytes is the declared size, at least 1.
	SizeBytes int64
	// SHA256 is the declared digest of the whole file.
	SHA256 [32]byte
	// DeclaredMIMEType is the client's media type, a hint only; "" when absent.
	DeclaredMIMEType string
}

// PutPart is the request of uploadPart.
type PutPart struct {
	// FileID is the upload; PartNumber the number from 1.
	FileID     ids.ID
	PartNumber int
	// ContentDigest is the Content-Digest header (RFC 9530) as received; the use case parses it
	// after the session checks (docs/DECISIONS.md D-M3-03-10).
	ContentDigest string
	// ContentLength is the Content-Length of the request; -1 when unknown.
	ContentLength int64
	// Body streams the part.
	Body io.Reader
}

// CompleteUpload is the request of completeUpload (schema UploadComplete).
type CompleteUpload struct {
	FileID ids.ID
	// Parts is the client's list; it is checked against the confirmed parts.
	Parts []PartRef
}

// PartRef is one part with its ETag (schemas UploadPartResult and UploadState.completed_parts).
type PartRef struct {
	PartNumber int
	ETag       string
}

// UploadSession is the answer of createUpload (schema UploadSession) in mode direct.
type UploadSession struct {
	FileID ids.ID
	// Mode is always "direct" (the mode presigned is not built, 12 3.3).
	Mode          string
	PartSizeBytes int64
	PartCount     int
	ExpiresAt     time.Time
}

// UploadState is the answer of getUpload (schema UploadState).
type UploadState struct {
	FileID ids.ID
	// View is pending, uploading or complete (docs/DECISIONS.md D-M3-03-13).
	View          string
	PartSizeBytes int64
	// Confirmed are the confirmed parts, ascending, with their ETags.
	Confirmed []PartRef
	ExpiresAt time.Time
}

// File mirrors the schema File of docs/api/openapi.yaml; zero values are absent members.
type File struct {
	ID ids.ID
	// SHA256 is the lowercase hex digest of the stored bytes, set only once upload_state is
	// complete (docs/DECISIONS.md D-M3-03-18).
	SHA256    string
	SizeBytes int64
	// MIMEType is the type the server detected; application/octet-stream until the verify pass.
	MIMEType     string
	OriginalName string
	UploadedBy   ids.ID
	// Purpose is attachment, avatar, branding or gdpr_export.
	Purpose string
	// ScopeChannelID is the channel of an attachment; zero for other purposes.
	ScopeChannelID ids.ID
	// UploadState is pending, complete or failed; ScanStatus pending, clean, infected, error or
	// skipped.
	UploadState, ScanStatus string
	// Width, Height and DurationMS are zero when unknown.
	Width, Height int
	DurationMS    int64
	// TranscriptStatus is none, pending, done or failed; TranscriptLanguage and TranscriptSource
	// are empty when absent.
	TranscriptStatus, TranscriptLanguage, TranscriptSource string
	// Thumbnails are the stored variants; empty until M3-06.
	Thumbnails []Thumbnail
	CreatedAt  time.Time
	// TranscriptETag is the derived ETag of the transcript (R-DM-48), which getFile sends in the
	// header ETag and submitTranscript accepts as If-Match (M3-09 design note 8 d); only Get sets
	// it. It is never part of a representation or an event.
	TranscriptETag string
}

// Thumbnail is one stored variant of a file (schema Thumbnail).
type Thumbnail struct {
	Variant, MIMEType string
	Width, Height     int
}
