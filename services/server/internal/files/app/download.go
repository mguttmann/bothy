// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/OWNER/bothy/services/server/internal/files"
	"github.com/OWNER/bothy/services/server/internal/files/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// DownloadConfig holds the configuration of downloads, tickets and transcripts (M3-05), bound in
// internal/wire (wire.DownloadConfig). Durations are named keys of
// docs/operations/configuration.md.
type DownloadConfig struct {
	// TicketTTL is file.ticket_ttl, the validity of a download ticket.
	TicketTTL time.Duration `config:"file.ticket_ttl"`
	// NotReadyRetryAfter is file.not_ready_retry_after, the Retry-After of 409 file_not_ready.
	NotReadyRetryAfter time.Duration `config:"file.not_ready_retry_after"`
	// CacheMaxAge is client.cache_max_age, the max-age of an authenticated download.
	CacheMaxAge time.Duration `config:"client.cache_max_age"`
}

// check refuses durations the use cases cannot work with.
func (c DownloadConfig) check() error {
	if c.TicketTTL <= 0 || c.NotReadyRetryAfter <= 0 || c.CacheMaxAge < 0 {
		return errors.New("files: file.ticket_ttl and file.not_ready_retry_after must be positive, client.cache_max_age not negative")
	}
	return nil
}

// DeliveryReader is the outgoing port of the read path of downloads (adapters/postgres, queries
// download.sql). Both reads are keyed by the file id (07 7.4 rule 1).
type DeliveryReader interface {
	// FileForDelivery reads one file, deleted or not (FileRecord.Deleted), with its transcript.
	FileForDelivery(ctx context.Context, fileID ids.ID) (FileRecord, bool, error)
	// Thumbnail reads one thumbnail of fileID by variant.
	Thumbnail(ctx context.Context, fileID ids.ID, variant string) (ThumbnailRecord, bool, error)
}

// SessionActors builds the actor of a download ticket from its principal and login session
// (identity.Auth.SessionActor, M3-05 design note 6): session, device and key state are read per
// call and never refused here, so authorize decides and audits them.
type SessionActors interface {
	SessionActor(ctx context.Context, userID, sessionID ids.ID) (actor.Actor, error)
}

// DownloadDeps are the collaborators of the download use cases.
type DownloadDeps struct {
	// Reader reads files and thumbnails for a delivery.
	Reader DeliveryReader
	// Store is the object storage; downloads use only Get.
	Store ObjectStore
	// Sessions gives the actor of a ticket.
	Sessions SessionActors
	// Keys is the source of the ticket key (file.ticket_key_ref).
	Keys TicketKeySource
}

// download is what EnableDownloads adds to the Service.
type download struct {
	cfg  DownloadConfig
	d    DownloadDeps
	ring *keyRing
}

// EnableDownloads adds downloads, tickets and transcripts to s (M3-05). It is called once while
// the process is built, before s serves requests. Errors: a missing collaborator or a duration
// DownloadConfig refuses.
func (s *Service) EnableDownloads(cfg DownloadConfig, d DownloadDeps) error {
	if d.Reader == nil || d.Store == nil || d.Sessions == nil || d.Keys == nil {
		return errors.New("files: EnableDownloads needs every collaborator")
	}
	if err := cfg.check(); err != nil {
		return err
	}
	s.dl = &download{cfg: cfg, d: d, ring: newKeyRing(d.Keys)}
	return nil
}

// The values of Cache-Control of a delivery (12 9.3).
const (
	cacheTicket = "private, no-store"
	// cacheAuthFormat takes client.cache_max_age in whole seconds.
	cacheAuthFormat = "private, max-age=%d, immutable"
)

// msgObjectMissing is the error line of a deliverable file without its object.
const msgObjectMissing = "deliverable file without its object in the storage"

// errDownloadsOff is the internal error of a Service without EnableDownloads.
var errDownloadsOff = errs.New(errs.CodeInternalError, "files: downloads are not enabled")

// Open implements files.Files (downloadFile, downloadThumbnail; brief M3-05 design note 2): who
// asks (the actor of the request, or of the ticket for an anonymous request with one), the file
// (404 for unknown or deleted), file.read, the readiness, the variant, the range, then the stream.
// Every failure of the ticket path, a refused decision included, is the unknown-id 404.
func (s *Service) Open(ctx context.Context, req files.OpenRequest) (files.Delivery, error) {
	if s.dl == nil {
		return files.Delivery{}, errDownloadsOff
	}
	v, ok := domain.ParseVariant(req.Variant)
	if !ok {
		return files.Delivery{}, notFound()
	}
	ctx, a, ticket, err := s.downloader(ctx, req, v)
	if err != nil {
		return files.Delivery{}, err
	}
	f, err := s.readable(ctx, a, req.FileID, false, ticket)
	if err != nil {
		return files.Delivery{}, err
	}
	src, err := s.variantOf(ctx, f, v)
	if err != nil {
		return files.Delivery{}, err
	}
	return s.deliver(ctx, f, src, req, ticket)
}

// downloader returns the actor of a download: the actor of the chain when it is not anonymous
// (Authorization wins over a ticket, D-M3-05-3), the actor of a valid ticket, or the refusal of an
// anonymous request without one. ctx carries the actor of the ticket, so the audit of a denial
// names its principal.
func (s *Service) downloader(ctx context.Context, req files.OpenRequest, v domain.Variant) (context.Context, actor.Actor, bool, error) {
	a := caller(ctx)
	if !a.IsAnonymous() {
		return ctx, a, false, nil
	}
	if req.Ticket == "" {
		return ctx, a, false, s.anonymous(ctx, a, req.FileID)
	}
	t, err := s.dl.ring.verify(ctx, req.Ticket, s.d.Clock.Now())
	if err != nil || t.FileID != req.FileID || t.Variant != v {
		return ctx, a, true, notFound()
	}
	ta, err := s.dl.d.Sessions.SessionActor(ctx, t.PrincipalID, t.SessionID)
	if err != nil {
		return ctx, a, true, notFound()
	}
	return actor.WithActor(ctx, ta), ta, true, nil
}

// anonymous decides file.read for an anonymous actor on a descriptor that names only the file:
// authorize answers unauthenticated before it resolves anything (07 4.2 step 1), so the caller
// learns nothing about the id (design note 2a). The facts are empty, never read.
func (s *Service) anonymous(ctx context.Context, a actor.Actor, fileID ids.ID) error {
	d, err := s.decide(ctx, a, authorize.ActionFileRead,
		authorize.Resource{Kind: authorize.ResourceFile, ID: fileID, File: &authorize.FileFacts{}})
	if err != nil {
		return err
	}
	if err := d.Err(); err != nil {
		return err
	}
	return errs.New(errs.CodeInternalError, "files: an anonymous file.read was allowed")
}

// readable loads the file and checks file.read and the readiness, in this order (design notes
// 2b to 2d): unknown or deleted is 404 before authorize, a refused decision is its error (404 on
// the ticket path), and only a reader learns the state. issue sets the descriptor fact of
// createFileTicket.
func (s *Service) readable(ctx context.Context, a actor.Actor, fileID ids.ID, issue, ticket bool) (FileRecord, error) {
	f, found, err := s.dl.d.Reader.FileForDelivery(ctx, fileID)
	if err != nil {
		return FileRecord{}, err
	}
	if !found || f.Deleted {
		return FileRecord{}, notFound()
	}
	r := fileResource(f)
	r.IssueTicket = issue
	d, err := s.decide(ctx, a, authorize.ActionFileRead, r)
	if err != nil {
		return FileRecord{}, err
	}
	if !d.Allowed() {
		if ticket {
			return FileRecord{}, notFound()
		}
		return FileRecord{}, d.Err()
	}
	switch domain.ReadinessOf(f.UploadState, f.ScanStatus) {
	case domain.Deliverable:
		return f, nil
	case domain.Blocked:
		return FileRecord{}, errs.New(errs.CodeFileBlocked, "")
	case domain.NotReady, domain.ReadinessInvalidZero:
	}
	return FileRecord{}, errs.New(errs.CodeFileNotReady, "").WithRetryAfter(s.dl.cfg.NotReadyRetryAfter)
}

// source is one variant of a file: its object and what the headers need.
type source struct {
	variant  domain.Variant
	key      string
	size     int64
	mimeType string
	name     string
	sum      [32]byte
}

// variantOf returns the variant v of f: the file itself, or its thumbnail, 404 when the file has
// none (D-M3-05-5). A thumbnail has no checksum column (03), so its digest comes from one full
// read of the object (D-M3-05-10).
func (s *Service) variantOf(ctx context.Context, f FileRecord, v domain.Variant) (source, error) {
	if v == domain.VariantOriginal {
		return source{variant: v, key: f.ObjectKey, size: f.SizeBytes, mimeType: f.MIMEType, name: f.OriginalName, sum: f.SHA256}, nil
	}
	th, found, err := s.dl.d.Reader.Thumbnail(ctx, f.ID, string(v))
	if err != nil {
		return source{}, err
	}
	if !found {
		return source{}, notFound()
	}
	src := source{variant: v, key: th.ObjectKey, size: th.SizeBytes, mimeType: th.MIMEType,
		name: domain.ThumbnailName(f.OriginalName, v, th.MIMEType)}
	body, err := s.object(ctx, f.ID, src.key, nil)
	if err != nil {
		return source{}, err
	}
	defer func() { _ = body.Close() }()
	h := sha256.New()
	if _, err := io.Copy(h, body); err != nil {
		return source{}, storageError(err)
	}
	copy(src.sum[:], h.Sum(nil))
	return src, nil
}

// deliver decides the range and opens the stream with the headers of 12 9.3.
func (s *Service) deliver(ctx context.Context, f FileRecord, src source, req files.OpenRequest, ticket bool) (files.Delivery, error) {
	etag := domain.FSPartETag(src.sum)
	out := files.Delivery{Status: http.StatusOK, Size: src.size, First: 0, Last: src.size - 1, ETag: etag,
		ReprDigest: "sha-256=:" + base64.StdEncoding.EncodeToString(src.sum[:]) + ":", Ticket: ticket}
	out.ContentType, out.Disposition = presentation(src)
	out.CacheControl = cacheTicket
	if !ticket {
		out.CacheControl = fmt.Sprintf(cacheAuthFormat, int64(s.dl.cfg.CacheMaxAge/time.Second))
	}
	var rng *ByteRange
	switch r := domain.ParseRange(req.Range, req.IfRange, etag, src.size); r.Kind {
	case domain.RangePartial:
		out.Status, out.First, out.Last = http.StatusPartialContent, r.First, r.Last
		rng = &ByteRange{Offset: r.First, Length: r.Last - r.First + 1}
	case domain.RangeUnsatisfiable:
		return files.Delivery{}, errs.New(errs.CodeRangeNotSatisfiable, "").
			WithHeader("Content-Range", "bytes */"+strconv.FormatInt(src.size, 10))
	case domain.RangeFull:
	}
	body, err := s.object(ctx, f.ID, src.key, rng)
	if err != nil {
		return files.Delivery{}, err
	}
	out.Body = body
	return out, nil
}

// presentation is the Content-Type and Content-Disposition of src (domain.Presentation).
func presentation(src source) (string, string) {
	ct, inline := domain.Presentation(src.mimeType, src.variant)
	return ct, domain.ContentDisposition(src.name, inline)
}

// object opens key with rng. A missing object of a deliverable file is a server fault: 500 with
// the file id in the error log, never in the body (D-M3-05-6); any other storage failure is 503.
func (s *Service) object(ctx context.Context, fileID ids.ID, key string, rng *ByteRange) (io.ReadCloser, error) {
	body, err := s.dl.d.Store.Get(ctx, key, rng)
	if errors.Is(err, ErrNotFound) {
		s.d.Log.LogAttrs(ctx, slog.LevelError, msgObjectMissing, slog.String(logKeyFileID, fileID.String()))
		return nil, errs.New(errs.CodeInternalError, "")
	}
	if err != nil {
		return nil, storageError(err)
	}
	return body, nil
}

// storageError is the 503 of a failing object storage (02 14.4); the text of err stays in the log.
func storageError(err error) error {
	return errs.Wrap(errs.CodeUnavailable, err, "files: the file storage failed").WithCause(errs.CauseDependencyUnavailable)
}
