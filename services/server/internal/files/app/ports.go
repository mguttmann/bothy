// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"time"

	"github.com/OWNER/bothy/services/server/internal/files/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// FileRecord is a row of files as the use cases read it.
type FileRecord struct {
	ID        ids.ID
	ObjectKey string
	// SHA256 is the declared digest until the verify pass replaces it by the stored one.
	SHA256    [32]byte
	SizeBytes int64
	MIMEType  string
	// DeclaredMIMEType is "" for NULL.
	DeclaredMIMEType string
	OriginalName     string
	UploadedBy       ids.ID
	Purpose          string
	// ScopeChannelID is zero for NULL.
	ScopeChannelID          ids.ID
	UploadState, ScanStatus string
	// Width, Height and DurationMS are zero for NULL.
	Width, Height    int
	DurationMS       int64
	TranscriptStatus string
	// TranscriptLanguage and TranscriptSource are "" for NULL.
	TranscriptLanguage, TranscriptSource string
	CreatedAt                            time.Time
	// Deleted reports deleted_at IS NOT NULL.
	Deleted bool
}

// SessionRecord is a row of file_upload_sessions with its file.
type SessionRecord struct {
	FileID            ids.ID
	Mode              string
	State             domain.SessionState
	DeclaredSizeBytes int64
	DeclaredSHA256    [32]byte
	PartSizeBytes     int64
	PartCount         int
	StorageUploadID   string
	CreatedAt         time.Time
	CompletedAt       time.Time
	ExpiresAt         time.Time
	File              FileRecord
}

// PartRecord is a row of file_upload_parts.
type PartRecord struct {
	FileID     ids.ID
	PartNumber int
	SizeBytes  int64
	SHA256     [32]byte
	// ETag is the stored ETag of an S3 part; "" (NULL) on the file system driver.
	ETag       string
	ReceivedAt time.Time
}

// NewUpload is what Create inserts: the file row and its session.
type NewUpload struct {
	File    FileRecord
	Session SessionRecord
}

// Repository is the outgoing port of the files module to its tables (03 4.7); adapters/postgres
// implements it. Reads outside a unit of work use the pool, reads and writes inside one its
// transaction; the writing methods need a unit of work. Every reading query is keyed by a file id
// or by the uploader (07 7.4 rule 1, the exception list of AZ-P12).
type Repository interface {
	// LockPrincipalUploads takes the transaction-level advisory lock of the uploads of principal,
	// so the quota and session counts of two parallel creates are serialized (D-M3-03-7).
	LockPrincipalUploads(ctx context.Context, principal ids.ID) error
	// QuotaUsedBytes sums size_bytes of the principal's files that are neither deleted nor failed
	// (D-M3-03-6).
	QuotaUsedBytes(ctx context.Context, principal ids.ID) (int64, error)
	// OpenSessions counts the principal's open sessions whose expires_at is after now and returns
	// the earliest of their expiry times (zero without one).
	OpenSessions(ctx context.Context, principal ids.ID, now time.Time) (int, time.Time, error)
	// InsertUpload inserts the file and its session.
	InsertUpload(ctx context.Context, u NewUpload) error
	// Session reads the session of fileID with its file when the file is the principal's and not
	// deleted; found is false otherwise.
	Session(ctx context.Context, fileID, principal ids.ID) (SessionRecord, bool, error)
	// LockSession reads the session as Session and locks its row: FOR UPDATE when update is set,
	// else FOR SHARE. It needs a unit of work.
	LockSession(ctx context.Context, fileID, principal ids.ID, update bool) (SessionRecord, bool, error)
	// SetStorageUploadID stores the multipart upload id of an open session that has none yet and
	// reports whether this call stored it.
	SetStorageUploadID(ctx context.Context, fileID ids.ID, uploadID string) (bool, error)
	// Parts returns the confirmed parts of fileID, ascending.
	Parts(ctx context.Context, fileID ids.ID) ([]PartRecord, error)
	// Part returns one confirmed part.
	Part(ctx context.Context, fileID ids.ID, n int) (PartRecord, bool, error)
	// InsertPart inserts a confirmed part unless its number exists and reports whether it did.
	InsertPart(ctx context.Context, p PartRecord) (bool, error)
	// CompleteSession sets an open session completed with completed_at.
	CompleteSession(ctx context.Context, fileID ids.ID, at time.Time) error
	// AbortSession sets an open session aborted, its file failed and deletes its parts.
	AbortSession(ctx context.Context, fileID ids.ID) error
	// File reads a file that is not deleted.
	File(ctx context.Context, fileID ids.ID) (FileRecord, bool, error)
}

// FreeSpace reports the free bytes of the file system of file.fs.root (adapters/fs); only the
// file system driver has it.
type FreeSpace interface {
	FreeBytes(ctx context.Context) (int64, error)
}

// NameNormalizer brings a file name into NFC (adapters/unicode), because the architecture gate
// allows golang.org/x/text only in adapters.
type NameNormalizer interface {
	NFC(s string) string
}

// TypeExtensions returns the file extensions of a media type, each with its leading dot and in
// lower case (adapters/mimetype); nil for an unknown type.
type TypeExtensions interface {
	Extensions(mediaType string) []string
}

// ChannelLocator returns the space of a channel without a permission check (the chat port), for
// the scope of file.updated; zero for a dm channel.
type ChannelLocator interface {
	SpaceOf(ctx context.Context, channelID ids.ID) (ids.ID, error)
}
