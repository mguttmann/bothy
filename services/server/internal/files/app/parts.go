// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"crypto/sha256"
	"errors"
	"hash"
	"io"
	"log/slog"

	"github.com/OWNER/bothy/services/server/internal/files"
	"github.com/OWNER/bothy/services/server/internal/files/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
)

// The keys of the log lines of the use cases: bothy.file.id of the attribute registry
// (platform/otel/attr.FileID, never on metrics) and the class of an error.
const (
	logKeyFileID    = "bothy.file.id"
	logKeyErrorType = "error.type"
)

// PutPart implements files.Uploads (brief M3-03 design note 9, docs/DECISIONS.md D-M3-03-11 and
// -14): the own session (404 otherwise), file.upload on its channel (a lost read aborts the
// session), the state (410), the number, the digest, a confirmed number answered without reading
// the body, then the body streamed through a reader that checks length and digest into the part
// store, outside any unit of work, and the row in a short unit of work.
func (s *Service) PutPart(ctx context.Context, cmd files.PutPart) (files.PartRef, error) {
	if txn.InUnit(ctx) {
		return files.PartRef{}, errs.New(errs.CodeInternalError, "files: PutPart runs inside a unit of work")
	}
	a := caller(ctx)
	rec, err := s.ownSession(ctx, a, cmd.FileID, authorize.ActionFileUpload)
	if err != nil {
		return files.PartRef{}, err
	}
	if s.expired(rec, s.now()) {
		return files.PartRef{}, gone()
	}
	if cmd.PartNumber < 1 || cmd.PartNumber > rec.PartCount {
		return files.PartRef{}, fieldError(pointerPartNumber, codeOutOfRange, "part_number is outside the part plan of the upload")
	}
	digest, err := domain.ParseContentDigest(cmd.ContentDigest)
	if err != nil {
		return files.PartRef{}, fieldError(pointerContentDigest, codeInvalid, "Content-Digest needs one sha-256 byte sequence")
	}
	if p, found, err := s.d.Repo.Part(ctx, cmd.FileID, cmd.PartNumber); err != nil || found {
		if err != nil {
			return files.PartRef{}, err
		}
		return samePart(p, digest)
	}
	size := domain.PartLength(cmd.PartNumber, rec.PartCount, rec.DeclaredSizeBytes, rec.PartSizeBytes)
	if cmd.ContentLength >= 0 && cmd.ContentLength != size {
		return files.PartRef{}, lengthMismatch()
	}
	body := &verifyingReader{r: cmd.Body, want: size, digest: digest, h: sha256.New()}
	etag, err := s.d.Parts.Put(ctx, rec, cmd.PartNumber, body, size)
	if err != nil {
		return files.PartRef{}, partError(body, err)
	}
	return s.confirmPart(ctx, a, rec, PartRecord{FileID: cmd.FileID, PartNumber: cmd.PartNumber, SizeBytes: size, SHA256: digest,
		ETag: etag, ReceivedAt: s.now()})
}

// confirmPart is step h of design note 9: under FOR SHARE on the session the row of the part
// goes in while the session is open; an ended session answers 410 and loses the object after the
// commit; a completed or conflicting session compares with the confirmed row.
func (s *Service) confirmPart(ctx context.Context, a actor.Actor, rec SessionRecord, p PartRecord) (files.PartRef, error) {
	var out files.PartRef
	var ended bool
	err := s.d.UoW.Do(ctx, func(ctx context.Context) error {
		cur, found, err := s.d.Repo.LockSession(ctx, p.FileID, a.ID, false)
		if err != nil {
			return err
		}
		if !found || cur.State == domain.SessionAborted || cur.State == domain.SessionExpired {
			ended = true
			return gone()
		}
		if cur.State == domain.SessionOpen {
			inserted, err := s.d.Repo.InsertPart(ctx, p)
			if err != nil || inserted {
				out = files.PartRef{PartNumber: p.PartNumber, ETag: partETag(p)}
				return err
			}
		}
		have, found, err := s.d.Repo.Part(ctx, p.FileID, p.PartNumber)
		if err != nil {
			return err
		}
		if !found {
			return errs.New(errs.CodeInternalError, "files: a confirmed part is missing")
		}
		out, err = samePart(have, p.SHA256)
		return err
	})
	if ended {
		if derr := s.d.Parts.DiscardUnconfirmed(ctx, rec, p.PartNumber); derr != nil {
			s.logDiscard(ctx, rec, derr)
		}
	}
	return out, err
}

// samePart answers a part whose number is confirmed: the same digest is the confirmed part (200
// without new work), another one 409 conflict part_conflict (12 3.2 point 3).
func samePart(p PartRecord, digest [32]byte) (files.PartRef, error) {
	if p.SHA256 != digest {
		return files.PartRef{}, conflict(reasonPartConflict, "the part number is confirmed with another digest")
	}
	return files.PartRef{PartNumber: p.PartNumber, ETag: partETag(p)}, nil
}

// partETag is the ETag of a confirmed part: the stored one of an S3 part, else the one derived
// from its digest (D-M3-03-3).
func partETag(p PartRecord) string {
	if p.ETag != "" {
		return p.ETag
	}
	return domain.FSPartETag(p.SHA256)
}

// errLength and errDigest are the errors of verifyingReader; the store keeps nothing after
// either.
var (
	errLength = errors.New("files: the part has the wrong length")
	errDigest = errors.New("files: the part does not match its Content-Digest")
)

// lengthMismatch is the 400 of a part of the wrong length: part_digest_mismatch with an empty
// pointer (D-M3-03-10).
func lengthMismatch() error {
	return fieldError("", codePartDigestMismatch, "the part does not have the length of its number")
}

// partError maps the error of the part store: the checks of the reader to 400
// part_digest_mismatch, a size refusal of the store to the length error, anything else unchanged
// (a storage failure answers 503, a cancelled request ends).
func partError(body *verifyingReader, err error) error {
	switch {
	case errors.Is(err, errDigest) || body.failed == errDigest:
		return fieldError(pointerContentDigest, codePartDigestMismatch, "the part does not match its Content-Digest")
	case errors.Is(err, errLength) || errors.Is(err, ErrSizeMismatch) || body.failed == errLength:
		return lengthMismatch()
	}
	if _, ok := errs.As(err); ok {
		return err
	}
	return errs.Wrap(errs.CodeUnavailable, err, "files: the part could not be stored").WithCause(errs.CauseDependencyUnavailable)
}

// verifyingReader hashes and counts the body while the store reads it: it never yields more than
// want bytes, fails with errLength when the body is shorter or longer and with errDigest when the
// bytes do not match the digest. The digest is checked as soon as want bytes are read, so a store
// that stops reading at want bytes also sees the error (design note 9 g).
type verifyingReader struct {
	r      io.Reader
	want   int64
	n      int64
	digest [32]byte
	h      hash.Hash
	failed error
}

func (v *verifyingReader) Read(p []byte) (int, error) {
	if v.failed != nil {
		return 0, v.failed
	}
	if v.n == v.want {
		// The part is complete; one more byte means a longer body.
		var one [1]byte
		n, err := io.ReadFull(v.r, one[:])
		if n > 0 {
			return 0, v.fail(errLength)
		}
		if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
			return 0, io.EOF
		}
		return 0, err
	}
	if rest := v.want - v.n; int64(len(p)) > rest {
		p = p[:rest]
	}
	n, err := v.r.Read(p)
	v.n += int64(n)
	v.h.Write(p[:n])
	if v.n == v.want {
		var sum [32]byte
		v.h.Sum(sum[:0])
		if sum != v.digest {
			// A longer body is a length error, not a digest error (D-M3-03-10).
			if err == nil && v.more() {
				return n, v.fail(errLength)
			}
			return n, v.fail(errDigest)
		}
		if err == io.EOF {
			err = nil
		}
		return n, err
	}
	if errors.Is(err, io.EOF) {
		return n, v.fail(errLength)
	}
	return n, err
}

// more reports whether the body has another byte after the part.
func (v *verifyingReader) more() bool {
	var one [1]byte
	n, _ := io.ReadFull(v.r, one[:])
	return n > 0
}

// fail records err and returns it.
func (v *verifyingReader) fail(err error) error {
	v.failed = err
	return err
}

// ownSession loads the session of fileID of the actor and authorizes act on its channel
// (file.upload) or on the file (file.delete). An unknown, foreign or deleted file is 404; a lost
// read right aborts the open session in its own unit of work and answers 404 (design note 14).
func (s *Service) ownSession(ctx context.Context, a actor.Actor, fileID ids.ID, act authorize.Action) (SessionRecord, error) {
	rec, found, err := s.d.Repo.Session(ctx, fileID, a.ID)
	if err != nil {
		return SessionRecord{}, err
	}
	if !found {
		return SessionRecord{}, notFound()
	}
	r := authorize.Resource{Kind: authorize.ResourceChannel, ID: rec.File.ScopeChannelID}
	if act == authorize.ActionFileDelete {
		r = fileResource(rec.File)
	}
	d, err := s.decide(ctx, a, act, r)
	if err != nil {
		return SessionRecord{}, err
	}
	if d.Outcome == authorize.OutcomeNotFound {
		if !txn.InUnit(ctx) && rec.State == domain.SessionOpen {
			if err := s.abort(ctx, a, rec.FileID); err != nil && errs.KindOf(err) != errs.KindNotFound {
				return SessionRecord{}, err
			}
		}
		return SessionRecord{}, notFound()
	}
	return rec, d.Err()
}

// fileResource describes a file for file.read and file.delete (design note 15).
func fileResource(f FileRecord) authorize.Resource {
	return authorize.Resource{Kind: authorize.ResourceFile, ID: f.ID, ChannelID: f.ScopeChannelID, OwnerID: f.UploadedBy,
		File: &authorize.FileFacts{Purpose: f.Purpose, Attached: false}}
}

// logDiscard logs a failed removal of parts with the file id only; the cleanup of failed files
// (M3-08) removes what is left.
func (s *Service) logDiscard(ctx context.Context, rec SessionRecord, err error) {
	s.d.Log.LogAttrs(ctx, slog.LevelWarn, "upload parts not removed; the cleanup of failed files removes them",
		slog.String(logKeyFileID, rec.FileID.String()), slog.String(logKeyErrorType, errorType(err)))
}

// errorType is a short class of err for a log line, never its text (the text may name a key).
func errorType(err error) string {
	if e, ok := errs.As(err); ok {
		return string(e.PublicCode())
	}
	return "storage"
}
