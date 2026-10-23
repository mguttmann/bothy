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
	// TranscriptText is "" for NULL; only FileForDelivery reads it (M3-05).
	TranscriptText string
	CreatedAt      time.Time
	// Deleted reports deleted_at IS NOT NULL.
	Deleted bool
	// Thumbnails are the rows of file_thumbnails (M3-06), in any order; FileView orders them.
	// Only File and the reads of the job files.thumbnail fill them.
	Thumbnails []ThumbnailRecord
}

// ThumbnailRecord is a row of file_thumbnails (docs/spec/03-data-model.md 4.7). FileID is set by
// the read of a single thumbnail for the download (M3-05); the rows of FileRecord.Thumbnails and of
// the job files.thumbnail leave it zero.
type ThumbnailRecord struct {
	FileID    ids.ID
	Variant   domain.Variant
	ObjectKey string
	MIMEType  string
	Width     int
	Height    int
	SizeBytes int64
}

// StoredOriginal is what T1 of the job files.thumbnail writes to the files row (brief M3-06 design
// note 10a): the stored original after the removal of the metadata.
type StoredOriginal struct {
	ObjectKey string
	SHA256    [32]byte
	SizeBytes int64
	MIMEType  string
	// Width and Height are the display size; zero writes NULL.
	Width, Height int
}

// MediaInfo is what T3 of the job files.thumbnail writes (design note 10d); a zero value keeps
// the column.
type MediaInfo struct {
	Width, Height int
	DurationMS    int64
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
	// File reads a file that is not deleted, with its thumbnails.
	File(ctx context.Context, fileID ids.ID) (FileRecord, bool, error)

	// ImageFile reads a file by id with its thumbnails, deleted or not (FileRecord.Deleted), for
	// the job files.thumbnail, which has no principal (M3-06).
	ImageFile(ctx context.Context, fileID ids.ID) (FileRecord, bool, error)
	// LockImageFile reads as ImageFile and locks the files row FOR UPDATE (03 6.3: only this
	// row). It needs a unit of work.
	LockImageFile(ctx context.Context, fileID ids.ID) (FileRecord, bool, error)
	// StoreImageOriginal writes the stored original and upload_state complete to a pending file
	// that is not deleted. It needs a unit of work.
	StoreImageOriginal(ctx context.Context, fileID ids.ID, o StoredOriginal) error
	// FailImage sets a pending file failed and the failure_reason of its session. It needs a
	// unit of work.
	FailImage(ctx context.Context, fileID ids.ID, reason string) error
	// InsertThumbnails inserts the rows of fileID unless a row of the variant exists and
	// returns how many it inserted. It needs a unit of work.
	InsertThumbnails(ctx context.Context, fileID ids.ID, rows []ThumbnailRecord) (int, error)
	// SetMediaInfo writes the non-zero values of m to a file that is not deleted. It needs a
	// unit of work.
	SetMediaInfo(ctx context.Context, fileID ids.ID, m MediaInfo) error

	// ReleaseFile sets deleted_at = at on the file fileID when its purpose is purpose, it is not
	// deleted and, unless uploadedBy is zero, uploadedBy uploaded it (M3-10); released reports
	// whether it did. It needs a unit of work.
	ReleaseFile(ctx context.Context, fileID ids.ID, purpose string, uploadedBy ids.ID, at time.Time) (released bool, err error)
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
