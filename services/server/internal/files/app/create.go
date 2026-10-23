// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"log/slog"
	"math"
	"mime"
	"slices"
	"strings"
	"time"

	"github.com/OWNER/bothy/services/server/internal/files"
	"github.com/OWNER/bothy/services/server/internal/files/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// The values a new upload starts with (12 3.1, 03 4.7).
const (
	modeDirect        = "direct"
	stateOpenFile     = "pending"
	octetStream       = "application/octet-stream"
	purposeAttachment = "attachment"
	transcriptNone    = "none"
)

// Create implements files.Uploads (brief M3-03 design note 5, docs/DECISIONS.md D-M3-03-4): in
// this order authorize file.upload on the channel, the size, the name and the denied types, then
// in one unit of work under the advisory lock of the principal the quota, the open sessions and
// the free space, and the rows. The unit is the ambient one of a keyed request.
func (s *Service) Create(ctx context.Context, cmd files.CreateUpload) (files.UploadSession, error) {
	a := caller(ctx)
	d, err := s.decide(ctx, a, authorize.ActionFileUpload, authorize.Resource{Kind: authorize.ResourceChannel, ID: cmd.ChannelID})
	if err != nil {
		return files.UploadSession{}, err
	}
	if err := d.Err(); err != nil {
		return files.UploadSession{}, err
	}
	if cmd.SizeBytes > s.cfg.MaxUploadBytes {
		return files.UploadSession{}, errs.New(errs.CodePayloadTooLarge, "size_bytes is above file.max_upload_bytes")
	}
	name := domain.CleanName(s.d.Names.NFC(cmd.Name), s.cfg.NameMaxBytes)
	declared, err := s.checkType(name, cmd.DeclaredMIMEType)
	if err != nil {
		return files.UploadSession{}, err
	}
	now := s.now()
	id := ids.New()
	key, err := domain.OriginalKey(id, 1)
	if err != nil {
		return files.UploadSession{}, err
	}
	count, _ := domain.PlanParts(cmd.SizeBytes, s.cfg.PartBytes)
	up := NewUpload{
		File: FileRecord{
			ID: id, ObjectKey: key, SHA256: cmd.SHA256, SizeBytes: cmd.SizeBytes, MIMEType: octetStream, DeclaredMIMEType: declared,
			OriginalName: name, UploadedBy: a.ID, Purpose: purposeAttachment, ScopeChannelID: cmd.ChannelID,
			UploadState: stateOpenFile, ScanStatus: stateOpenFile, TranscriptStatus: transcriptNone, CreatedAt: now,
		},
		Session: SessionRecord{
			FileID: id, Mode: modeDirect, State: domain.SessionOpen, DeclaredSizeBytes: cmd.SizeBytes, DeclaredSHA256: cmd.SHA256,
			PartSizeBytes: s.cfg.PartBytes, PartCount: count, CreatedAt: now, ExpiresAt: now.Add(s.cfg.SessionTTL),
		},
	}
	err = s.d.UoW.Do(ctx, func(ctx context.Context) error {
		if err := s.checkLimits(ctx, a.ID, cmd.SizeBytes, now); err != nil {
			return err
		}
		return s.d.Repo.InsertUpload(ctx, up)
	})
	if err != nil {
		return files.UploadSession{}, err
	}
	return files.UploadSession{FileID: id, Mode: modeDirect, PartSizeBytes: s.cfg.PartBytes, PartCount: count, ExpiresAt: up.Session.ExpiresAt}, nil
}

// checkType is the early type check of 12 6.1 (D-M3-03-5). It returns the declared media type to
// store: without parameters, in lower case, "" when absent or, with an empty
// file.denied_mime_types, when it does not parse. With denied types an unparsable declared type
// is 400 validation_failed; a denied declared type or an extension of the cleaned name that
// belongs to a denied type is 415 unsupported_media_type with reason file_type_denied.
func (s *Service) checkType(name, declared string) (string, error) {
	mediaType := ""
	if declared != "" {
		mt, _, err := mime.ParseMediaType(declared)
		switch {
		case err == nil:
			mediaType = strings.ToLower(mt)
		case len(s.cfg.DeniedMIMETypes) > 0:
			return "", fieldError(pointerDeclaredType, codeInvalid, "declared_mime_type is not a media type")
		}
	}
	if len(s.cfg.DeniedMIMETypes) == 0 {
		return mediaType, nil
	}
	ext := ""
	if dot := strings.LastIndexByte(name, '.'); dot >= 0 {
		ext = strings.ToLower(name[dot:])
	}
	for _, denied := range s.cfg.DeniedMIMETypes {
		denied = strings.ToLower(strings.TrimSpace(denied))
		if denied == "" {
			continue
		}
		if mediaType == denied || (ext != "" && slices.Contains(s.d.Types.Extensions(denied), ext)) {
			return "", errs.New(errs.CodeUnsupportedMediaType, "the file type is denied by file.denied_mime_types").WithReason(reasonFileTypeDenied)
		}
	}
	return mediaType, nil
}

// checkLimits checks quota, open sessions and free space of the principal inside the unit of work
// under its advisory lock (D-M3-03-6 to -8).
func (s *Service) checkLimits(ctx context.Context, principal ids.ID, size int64, now time.Time) error {
	if err := s.d.Repo.LockPrincipalUploads(ctx, principal); err != nil {
		return err
	}
	if s.cfg.QuotaBytes != 0 {
		used, err := s.d.Repo.QuotaUsedBytes(ctx, principal)
		if err != nil {
			return err
		}
		if used > s.cfg.QuotaBytes-size {
			return conflict(reasonQuotaExceeded, "the upload would exceed file.quota_bytes_per_principal")
		}
	}
	return s.checkOpenAndSpace(ctx, principal, size, now)
}

// checkSessionsAndSpace is checkLimits without the quota, for avatar and logo images, which
// replace the previous one (brief M3-10 design note 2e, docs/decisions.d/M3-10.md D-M3-10-4).
func (s *Service) checkSessionsAndSpace(ctx context.Context, principal ids.ID, size int64, now time.Time) error {
	if err := s.d.Repo.LockPrincipalUploads(ctx, principal); err != nil {
		return err
	}
	return s.checkOpenAndSpace(ctx, principal, size, now)
}

// checkOpenAndSpace checks the open sessions of the principal and the free space of the file
// system driver under the advisory lock the caller took (D-M3-03-7 and -8).
func (s *Service) checkOpenAndSpace(ctx context.Context, principal ids.ID, size int64, now time.Time) error {
	open, earliest, err := s.d.Repo.OpenSessions(ctx, principal, now)
	if err != nil {
		return err
	}
	if open >= s.cfg.MaxOpenSessions {
		wait := time.Duration(math.Ceil(earliest.Sub(now).Seconds())) * time.Second
		return errs.New(errs.CodeRateLimited, "the principal has file.upload_max_open_sessions open upload sessions").
			WithRetryAfter(max(wait, time.Second))
	}
	if s.d.FreeSpace == nil {
		return nil
	}
	free, err := s.d.FreeSpace.FreeBytes(ctx)
	if err != nil {
		return err
	}
	if free-size < s.cfg.MinFreeBytes {
		s.d.Log.LogAttrs(ctx, slog.LevelWarn, "file storage below file.fs.min_free_bytes; upload refused")
		return errs.New(errs.CodeUnavailable, "the file storage is below file.fs.min_free_bytes").WithReason(reasonStorageFull)
	}
	return nil
}
