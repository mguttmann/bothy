// SPDX-License-Identifier: Apache-2.0

package files

import (
	"context"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// ImportReserve is the file an import reserves before its bytes (createImportFile, 22 6.5 and 7.2
// item 3, M8-11 design note 6 b).
type ImportReserve struct {
	// ChannelID is the channel of the attachment (files.scope_channel_id).
	ChannelID ids.ID
	// UploadedBy is the mapped author of the message (files.uploaded_by).
	UploadedBy ids.ID
	// Name is the file name of the source; it is cleaned like every upload name and never logged.
	Name string
	// SizeBytes is the declared size, at least 1.
	SizeBytes int64
	// SHA256 is the declared digest of the whole file (file_upload_sessions.declared_sha256).
	SHA256 [32]byte
	// DeclaredMIMEType is the media type of the source, a hint only; "" when absent.
	DeclaredMIMEType string
}

// ImportSlot is the state of a reserved import file.
type ImportSlot struct {
	// FileID is the reserved file.
	FileID ids.ID
	// Completed reports that every part is confirmed and the session completed; Upload is then the
	// zero value.
	Completed bool
	// Upload is the open session of mode direct.
	Upload UploadSession
}

// ImportFiles is the incoming port of files for the attachments of an import (M8-11 design note
// 6). It authorizes nothing: importer authorized import.apply on the source and checked that the
// file is the target of a file item of the source. Reserve, Slot and Complete run only inside the
// caller's unit of work (internal otherwise); UploadPart never runs inside one, because the object
// store is called while the body streams (02 8.5). The rules of the parts and of the completion
// are those of uploadPart and completeUpload (12 3.2 and 3.4) without file.upload, so a read-only
// mirror channel accepts them (05 rule 1.8, 4.12). The scan and processing jobs of an import file
// run with the lowest job priority on the queue files (12 11). Errors are kernel/errs errors.
type ImportFiles interface {
	// Reserve inserts the pending file and its open session of mode direct without the quota
	// check of createUpload (12 11); the size against file.max_upload_bytes (413), the name and
	// the denied types (415), the open sessions of the author and the free space hold. Errors:
	// those of createUpload but quota_exceeded.
	Reserve(ctx context.Context, r ImportReserve) (ImportSlot, error)
	// Slot returns the state of a reserved file: completed, its open session, or for an aborted or
	// expired session a new open session for the same file ID (22 7.2 item 3). Errors: not_found
	// for an unknown or deleted file and for one that is no attachment.
	Slot(ctx context.Context, fileID ids.ID) (ImportSlot, error)
	// UploadPart stores one part like uploadPart. Errors: those of uploadPart.
	UploadPart(ctx context.Context, cmd PutPart) (PartRef, error)
	// Complete closes the session like completeUpload and enqueues files.scan with the lowest job
	// priority; a completed session answers the file again. Errors: those of completeUpload.
	Complete(ctx context.Context, cmd CompleteUpload) (File, error)
}
