// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/OWNER/bothy/services/server/internal/files"
	"github.com/OWNER/bothy/services/server/internal/files/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/eventlog"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
)

// Config holds the configuration of the upload use cases, bound in internal/wire
// (wire.FilesConfig). Limits are named keys of docs/operations/configuration.md.
type Config struct {
	// MaxUploadBytes is file.max_upload_bytes, the largest file.
	MaxUploadBytes int64 `config:"file.max_upload_bytes"`
	// PartBytes is file.upload_part_bytes, the size of every part but the last.
	PartBytes int64 `config:"file.upload_part_bytes"`
	// SessionTTL is file.upload_session_ttl, the lifetime of an open session.
	SessionTTL time.Duration `config:"file.upload_session_ttl"`
	// MaxOpenSessions is file.upload_max_open_sessions, the open sessions of one principal.
	MaxOpenSessions int `config:"file.upload_max_open_sessions"`
	// QuotaBytes is file.quota_bytes_per_principal; 0 switches the quota off.
	QuotaBytes int64 `config:"file.quota_bytes_per_principal"`
	// NameMaxBytes is file.name_max_bytes, the longest original_name in UTF-8 bytes.
	NameMaxBytes int `config:"file.name_max_bytes"`
	// DeniedMIMETypes is file.denied_mime_types; empty checks nothing.
	DeniedMIMETypes []string `config:"file.denied_mime_types"`
	// MinFreeBytes is file.fs.min_free_bytes, the reserve of the file system driver.
	MinFreeBytes int64 `config:"file.fs.min_free_bytes"`
}

// check refuses limits the use cases cannot work with.
func (c Config) check() error {
	if c.MaxUploadBytes < 1 || c.PartBytes < 1 || c.SessionTTL <= 0 || c.MaxOpenSessions < 1 {
		return errors.New("files: file.max_upload_bytes, file.upload_part_bytes, file.upload_session_ttl and file.upload_max_open_sessions must be positive")
	}
	if c.QuotaBytes < 0 || c.MinFreeBytes < 0 || c.NameMaxBytes < domain.MinNameBytes {
		return errors.New("files: file.quota_bytes_per_principal and file.fs.min_free_bytes must not be negative, file.name_max_bytes not below its minimum")
	}
	return nil
}

// Deps are the collaborators of the upload use cases.
type Deps struct {
	// Repo reads and writes files, sessions and parts.
	Repo Repository
	// Parts is the part strategy of the driver.
	Parts PartStore
	// UoW runs every write.
	UoW txn.UnitOfWork
	// Authorizer decides file.upload, file.read and file.delete.
	Authorizer authorize.Authorizer
	// Events appends file.updated.
	Events eventlog.Appender
	// Channels gives the space of a channel for the scope of file.updated.
	Channels ChannelLocator
	// Names brings file names into NFC.
	Names NameNormalizer
	// Types gives the extensions of the denied types.
	Types TypeExtensions
	// FreeSpace is the free space of the file system driver; nil for the S3 driver.
	FreeSpace FreeSpace
	// Store opens the variants of avatar and logo files (OpenImageVariant, M3-10); nil refuses it.
	Store ObjectStore
	// ScanQueue inserts the job files.scan when an upload completes (M3-04).
	ScanQueue ScanQueue
	// Purge inserts files.purge_objects when a file is deleted or becomes failed (M3-08).
	Purge PurgeQueue
	// Redactor blanks the stored file.updated payloads of a deleted file (03 6.5, M3-08).
	Redactor eventlog.Redactor
	// Cleanup reads and writes the facts of deleteFile (M3-08).
	Cleanup CleanupStore
	// Metrics records bothy.file.upload.size; nil records nothing.
	Metrics Metrics
	// Clock gives created_at, expires_at and every expiry comparison; nil means clock.System().
	Clock clock.Clock
	// Log receives the warnings of a full storage and of a failed discard; nil means
	// slog.Default(). Lines carry at most bothy.file.id, never a name.
	Log *slog.Logger
}

// Service implements files.Uploads and files.Files (M3-03). It is safe for concurrent use.
type Service struct {
	cfg Config
	d   Deps
	// dl are downloads, tickets and transcripts (M3-05); nil until EnableDownloads.
	dl *download
}

// Compile-time checks: the incoming ports.
var (
	_ files.Uploads = (*Service)(nil)
	_ files.Files   = (*Service)(nil)
)

// New returns the use cases. Errors: a missing collaborator or a limit Config refuses.
func New(cfg Config, d Deps) (*Service, error) {
	if d.Repo == nil || d.Parts == nil || d.UoW == nil || d.Authorizer == nil || d.Events == nil || d.Channels == nil ||
		d.Names == nil || d.Types == nil || d.ScanQueue == nil || d.Purge == nil || d.Redactor == nil || d.Cleanup == nil {
		return nil, errors.New("files: New needs every collaborator")
	}
	if err := cfg.check(); err != nil {
		return nil, err
	}
	if d.Clock == nil {
		d.Clock = clock.System()
	}
	if d.Log == nil {
		d.Log = slog.Default()
	}
	if d.Metrics == nil {
		d.Metrics = noMetrics{}
	}
	return &Service{cfg: cfg, d: d}, nil
}

// now is the clock time in the precision of the columns.
func (s *Service) now() time.Time { return s.d.Clock.Now().UTC().Truncate(time.Microsecond) }

// caller returns the actor of the request; anonymous when the chain set none, which authorize
// refuses.
func caller(ctx context.Context) actor.Actor {
	if a, ok := actor.FromContext(ctx); ok {
		return a
	}
	return actor.Anonymous()
}

// decide authorizes act on r for a and returns the decision.
func (s *Service) decide(ctx context.Context, a actor.Actor, act authorize.Action, r authorize.Resource) (authorize.Decision, error) {
	return s.d.Authorizer.Authorize(ctx, a, act, r)
}

// notFound is the 404 of an unknown, foreign or deleted file: the same body for all of them.
func notFound() error { return errs.New(errs.CodeNotFound, "") }

// gone is the 410 of an aborted or expired session.
func gone() error { return errs.New(errs.CodeGone, "the upload session has ended") }

// fieldError is a 400 validation_failed with one field error.
func fieldError(pointer, code, detail string) error {
	return errs.New(errs.CodeValidationFailed, detail).WithFields(errs.FieldError{Pointer: pointer, Code: code, Detail: detail})
}

// The field error codes of the upload use cases (docs/DECISIONS.md D-M3-03-10 and -12).
const (
	codeInvalid            = "invalid"
	codeOutOfRange         = "out_of_range"
	codeDuplicate          = "duplicate"
	codePartDigestMismatch = "part_digest_mismatch"
)

// The pointers of the field errors.
const (
	pointerPartNumber    = "#path/part_number"
	pointerContentDigest = "#header/Content-Digest"
	pointerDeclaredType  = "/declared_mime_type"
	pointerParts         = "/parts"
)

// The reasons of 409 conflict, 415 and 503 (12 3.6, D-W-20).
const (
	reasonQuotaExceeded    = "quota_exceeded"
	reasonPartConflict     = "part_conflict"
	reasonUploadIncomplete = "upload_incomplete"
	reasonFileTypeDenied   = "file_type_denied"
	reasonStorageFull      = "storage_full"
)

// conflict is a 409 conflict with reason.
func conflict(reason, detail string) error {
	return errs.New(errs.CodeConflict, detail).WithReason(reason)
}

// expired reports a session that no longer accepts work: aborted, expired, or open with
// expires_at reached.
func (s *Service) expired(rec SessionRecord, now time.Time) bool {
	switch rec.State {
	case domain.SessionAborted, domain.SessionExpired:
		return true
	case domain.SessionOpen:
		return !now.Before(rec.ExpiresAt)
	case domain.SessionCompleted:
	}
	return false
}
