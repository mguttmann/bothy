// SPDX-License-Identifier: Apache-2.0

package app

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"hash"
	"io"
	"log/slog"
	"time"

	"github.com/OWNER/bothy/services/server/internal/files/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/eventlog"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
)

// ReasonImageMalformed is file_upload_sessions.failure_reason of a JPEG or PNG whose structure the
// filter refused (docs/DECISIONS.md D-M3-06-7): a code without content, next to the reasons of
// the scan pass.
const ReasonImageMalformed = "image_malformed"

// Log messages of the job files.thumbnail; constant text, the only attributes are bothy.file.id
// and an error class (SEC-LOG-4: never a name, metadata value or image byte).
const (
	msgImageNoThumbnails = "file image without thumbnails"
	msgImageUnchanged    = "file image stored unchanged"
	msgImageNoMovieInfo  = "file movie values not readable"
)

// The error classes of the log lines of the job.
const (
	classTooManyPixels = "too_many_pixels"
	classUndecodable   = "undecodable"
	classAnimated      = "animated"
	classMalformed     = "malformed"
	classUnreadable    = "unreadable"
)

// errOriginalChanged reports that the second pass of the filter yielded other bytes than the
// first: the input changed under the job, so the run fails and River retries (design note 3c).
var errOriginalChanged = errors.New("files: the cleaned original changed between the two passes")

// ImageConfig holds the configuration of the job files.thumbnail, bound in internal/wire
// (wire.ImageConfig). Limits are named keys of docs/operations/configuration.md.
type ImageConfig struct {
	// MaxPixels is file.image.max_pixels: no image above it is decoded (12 7.2).
	MaxPixels int64 `config:"file.image.max_pixels"`
	// JPEGQuality is file.image.jpeg_quality, the quality of a still WebP stored as JPEG.
	JPEGQuality int `config:"file.image.jpeg_quality"`
	// ProcessingTimeout is file.image.processing_timeout, the River timeout of one run.
	ProcessingTimeout time.Duration `config:"file.image.processing_timeout"`
	// MaxConcurrent is file.image.max_concurrent, the runs one worker process works at once.
	MaxConcurrent int `config:"file.image.max_concurrent"`
	// SmallPx, MediumPx and LargePx are file.thumbnail.small_px, medium_px and large_px, the
	// longest edges of the variants (12 7.3).
	SmallPx  int `config:"file.thumbnail.small_px"`
	MediumPx int `config:"file.thumbnail.medium_px"`
	LargePx  int `config:"file.thumbnail.large_px"`
	// ThumbnailJPEGQuality is file.thumbnail.jpeg_quality.
	ThumbnailJPEGQuality int `config:"file.thumbnail.jpeg_quality"`
}

// check refuses values the job cannot work with; the configuration schema holds the same bounds.
func (c ImageConfig) check() error {
	if c.MaxPixels < 1 || c.ProcessingTimeout <= 0 || c.MaxConcurrent < 1 || c.SmallPx < 1 || c.MediumPx < 1 || c.LargePx < 1 ||
		c.JPEGQuality < 1 || c.ThumbnailJPEGQuality < 1 {
		return errors.New("files: file.image.* and file.thumbnail.* must be positive")
	}
	return nil
}

// ImageHooks are test seams of the job (ADR 0071); every field is nil in production.
type ImageHooks struct {
	// BeforeRun runs after the run took its slot, before it reads the row; an error ends the run
	// with that error, so River retries it.
	BeforeRun func(ctx context.Context, fileID ids.ID) error
	// AfterRun runs when the run gives its slot back.
	AfterRun func(fileID ids.ID)
	// AfterPut runs after the objects of a phase were written and before its transaction.
	AfterPut func(ctx context.Context, fileID ids.ID)
	// BeforeCommit runs inside T1 and T2 after their checks; an error rolls the unit back.
	BeforeCommit func(ctx context.Context, fileID ids.ID) error
}

// ImageDeps are the collaborators of the job files.thumbnail.
type ImageDeps struct {
	// Repo reads and writes files and file_thumbnails.
	Repo Repository
	// Store reads the originals and writes the cleaned originals and the variants.
	Store ObjectStore
	// UoW runs T1, T2, T3 and the failure transaction.
	UoW txn.UnitOfWork
	// Events appends file.updated.
	Events eventlog.Appender
	// Channels gives the space of the scope channel for file.updated.
	Channels ChannelLocator
	// Codec cleans and renders images; Movies reads MP4 and QuickTime boxes.
	Codec  ImageCodec
	Movies MovieReader
	// Log receives the lines of permanent image errors; nil means slog.Default().
	Log *slog.Logger
	// Hooks are test seams; nil in production.
	Hooks *ImageHooks
}

// ImageProcessor is the use case of the job files.thumbnail (docs/spec/12-files.md 7, ADR 0047,
// brief M3-06 design note 3): the original phase writes the cleaned original orig-<n+1> and makes
// the file deliverable in T1, the thumbnail phase renders small, medium and large, the container
// phase reads the display size and duration of MP4 and QuickTime files. It is safe for concurrent
// use; at most file.image.max_concurrent runs work at once in one process.
type ImageProcessor struct {
	cfg   ImageConfig
	d     ImageDeps
	slots chan struct{}
}

// NewImageProcessor returns the use case. Errors: a missing collaborator or a value ImageConfig
// refuses.
func NewImageProcessor(cfg ImageConfig, d ImageDeps) (*ImageProcessor, error) {
	if d.Repo == nil || d.Store == nil || d.UoW == nil || d.Events == nil || d.Channels == nil || d.Codec == nil || d.Movies == nil {
		return nil, errors.New("files: NewImageProcessor needs every collaborator")
	}
	if err := cfg.check(); err != nil {
		return nil, err
	}
	if d.Log == nil {
		d.Log = slog.Default()
	}
	if d.Hooks == nil {
		d.Hooks = &ImageHooks{}
	}
	return &ImageProcessor{cfg: cfg, d: d, slots: make(chan struct{}, cfg.MaxConcurrent)}, nil
}

// ProcessImage runs files.thumbnail for fileID (design note 3). It returns nil when the file is
// gone, deleted, failed, not clean or skipped, or when nothing is left to do, so every run can
// repeat without double effect (02 12.4 "Rules for jobs" item 1). A transient error of the store or
// the database, a changed original and a cancelled ctx return the error, and River retries.
// Permanent image errors (pixel limit, undecodable image) end a phase without retry.
func (p *ImageProcessor) ProcessImage(ctx context.Context, fileID ids.ID) error {
	select {
	case p.slots <- struct{}{}:
	case <-ctx.Done():
		return ctx.Err()
	}
	defer func() { <-p.slots }()
	if h := p.d.Hooks.AfterRun; h != nil {
		defer h(fileID)
	}
	if h := p.d.Hooks.BeforeRun; h != nil {
		if err := h(ctx, fileID); err != nil {
			return err
		}
	}
	f, found, err := p.d.Repo.ImageFile(ctx, fileID)
	if err != nil || !found || f.Deleted || !scanned(f) {
		return err
	}
	switch f.UploadState {
	case domain.UploadFailed:
		return p.deleteFailedOriginals(ctx, f)
	case domain.UploadPending:
		next, err := p.originalPhase(ctx, f)
		if err != nil || next == nil {
			return err
		}
		f = *next
	}
	if f.UploadState != domain.UploadComplete {
		return nil
	}
	if err := p.deleteOlderRevisions(ctx, f); err != nil {
		return err
	}
	if err := p.thumbnailPhase(ctx, f); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return p.containerPhase(ctx, f)
}

// scanned reports a file whose scan allows delivery (clean or skipped).
func scanned(f FileRecord) bool {
	return f.ScanStatus == domain.ScanClean || f.ScanStatus == domain.ScanSkipped
}

// stripPlan reports the lossless filters, whose failure fails the file (12 7.1).
func stripPlan(plan domain.OriginalPlan) bool {
	return plan == domain.PlanStripJPEG || plan == domain.PlanStripPNG
}

// revision returns the revision of the original key of f.
func revision(f FileRecord) (int, error) {
	_, rev, err := domain.ParseOriginalKey(f.ObjectKey)
	if err != nil {
		return 0, fmt.Errorf("files: the object key of a file is no original key: %w", err)
	}
	return rev, nil
}

// opener opens key anew for every pass.
func (p *ImageProcessor) opener(key string) Opener {
	return func(ctx context.Context) (io.ReadCloser, error) { return p.d.Store.Get(ctx, key, nil) }
}

// originalPhase is design note 3c for a pending file: it returns the file after T1, or nil when
// the run ends here (no plan, the file failed, or another change won).
func (p *ImageProcessor) originalPhase(ctx context.Context, f FileRecord) (*FileRecord, error) {
	plan := domain.OriginalPlanFor(f.MIMEType)
	if plan == domain.PlanNone {
		return nil, nil
	}
	rev, err := revision(f)
	if err != nil {
		return nil, err
	}
	next, err := domain.OriginalKey(f.ID, rev+1)
	if err != nil {
		return nil, err
	}
	c, err := p.d.Codec.CleanOriginal(ctx, plan, p.opener(f.ObjectKey), f.SizeBytes,
		CleanLimits{MaxPixels: p.cfg.MaxPixels, JPEGQuality: p.cfg.JPEGQuality})
	switch {
	case err == nil:
	case stripPlan(plan) && errors.Is(err, ErrImageMalformed):
		return nil, p.failImage(ctx, f, ReasonImageMalformed, next)
	case !stripPlan(plan) && permanent(err) && !domain.StoredOriginalAllowed(domain.Purpose(f.Purpose), domain.OriginalUnchanged):
		// An avatar or logo is never stored unchanged (brief M3-10 design note 4).
		return nil, p.failImage(ctx, f, domain.ReasonFileTypeDenied, next)
	case !stripPlan(plan) && permanent(err):
		p.d.Log.InfoContext(ctx, msgImageUnchanged, p.logAttrs(f.ID, err)...)
		return p.keepOriginal(ctx, f)
	default:
		return nil, err
	}
	if !domain.StoredOriginalAllowed(domain.Purpose(f.Purpose), domain.PlannedOriginal(plan)) {
		// The scan pass refuses these already; this keeps the rule where the original is decided.
		return nil, p.failImage(ctx, f, domain.ReasonFileTypeDenied, next)
	}
	if err := p.putCleaned(ctx, next, c); err != nil {
		return nil, err
	}
	if h := p.d.Hooks.AfterPut; h != nil {
		h(ctx, f.ID)
	}
	mimeType := c.MIMEType
	if stripPlan(plan) {
		// The filters keep the type: image/apng stays the detected name of an animated PNG.
		mimeType = f.MIMEType
	}
	return p.commitOriginal(ctx, f, StoredOriginal{ObjectKey: next, SHA256: c.SHA256, SizeBytes: c.Size, MIMEType: mimeType,
		Width: c.Width, Height: c.Height})
}

// permanent reports an image error that no retry changes.
func permanent(err error) bool {
	return errors.Is(err, ErrTooManyPixels) || errors.Is(err, ErrImageUndecodable) || errors.Is(err, ErrImageAnimated) ||
		errors.Is(err, ErrImageMalformed)
}

// errorClass maps an image error to its log class.
func errorClass(err error) string {
	switch {
	case errors.Is(err, ErrTooManyPixels):
		return classTooManyPixels
	case errors.Is(err, ErrImageAnimated):
		return classAnimated
	case errors.Is(err, ErrImageMalformed):
		return classMalformed
	case errors.Is(err, ErrImageUndecodable):
		return classUndecodable
	default:
		return classUnreadable
	}
}

// logAttrs are the only attributes of a log line of the job: the file id and the class of err.
func (p *ImageProcessor) logAttrs(fileID ids.ID, err error) []any {
	return []any{slog.String(logKeyFileID, fileID.String()), slog.String(logKeyErrorType, errorClass(err))}
}

// keepOriginal is T1 for a GIF or WebP that stays revision 1 (design note 7): complete without a
// new object; the size comes from the header when it can be read.
func (p *ImageProcessor) keepOriginal(ctx context.Context, f FileRecord) (*FileRecord, error) {
	o := StoredOriginal{ObjectKey: f.ObjectKey, SHA256: f.SHA256, SizeBytes: f.SizeBytes, MIMEType: f.MIMEType}
	if info, err := p.d.Codec.Inspect(ctx, f.MIMEType, p.opener(f.ObjectKey)); err == nil {
		o.Width, o.Height = info.Width, info.Height
	} else if !permanent(err) {
		return nil, err
	}
	return p.commitOriginal(ctx, f, o)
}

// putCleaned writes the cleaned original to key from a second pass of the filter; the reader
// fails before its last byte when the bytes differ from the first pass, so the store keeps
// nothing (design note 3c).
func (p *ImageProcessor) putCleaned(ctx context.Context, key string, c CleanedOriginal) error {
	body, err := c.Body(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = body.Close() }()
	vr := &cleanedReader{r: body, size: c.Size, want: c.SHA256, h: sha256.New()}
	if err := p.d.Store.Put(ctx, key, vr, c.Size); err != nil {
		if vr.err != nil {
			return vr.err
		}
		return err
	}
	return nil
}

// cleanedReader yields exactly size bytes whose SHA-256 is want; otherwise it fails before it
// hands out the last byte.
type cleanedReader struct {
	r    io.Reader
	size int64
	n    int64
	want [32]byte
	h    hash.Hash
	err  error
}

// Read implements io.Reader.
func (v *cleanedReader) Read(b []byte) (int, error) {
	if v.err != nil {
		return 0, v.err
	}
	n, err := v.r.Read(b)
	if int64(n) > v.size-v.n {
		v.err = errOriginalChanged
		return 0, v.err
	}
	v.h.Write(b[:n])
	v.n += int64(n)
	if v.n == v.size {
		var got [32]byte
		copy(got[:], v.h.Sum(nil))
		if got != v.want {
			v.err = errOriginalChanged
			return 0, v.err
		}
	}
	if errors.Is(err, io.EOF) && v.n != v.size {
		v.err = errOriginalChanged
		return n, v.err
	}
	return n, err
}

// awaitingOriginal reports a locked row the original phase may still change: not deleted,
// pending, clean or skipped, and pointing to the key the run read.
func awaitingOriginal(cur FileRecord, key string) bool {
	return !cur.Deleted && cur.UploadState == domain.UploadPending && scanned(cur) && cur.ObjectKey == key
}

// references reports whether the row refers to key as its original or a variant.
func references(cur FileRecord, key string) bool {
	if cur.ObjectKey == key {
		return true
	}
	for _, t := range cur.Thumbnails {
		if t.ObjectKey == key {
			return true
		}
	}
	return false
}

// commitOriginal is T1 (design note 10a). When a precondition fails it removes the object this
// run wrote, unless the locked row refers to it, and returns nil. After the commit it deletes
// the replaced original.
func (p *ImageProcessor) commitOriginal(ctx context.Context, f FileRecord, o StoredOriginal) (*FileRecord, error) {
	var out *FileRecord
	stale := ""
	err := p.d.UoW.Do(ctx, func(ctx context.Context) error {
		var err error
		out, stale, err = p.storeOriginal(ctx, f, o)
		return err
	})
	if err != nil {
		return nil, err
	}
	if stale != "" {
		return nil, p.deleteKeys(ctx, stale)
	}
	if out != nil && o.ObjectKey != f.ObjectKey {
		if err := p.deleteKeys(ctx, f.ObjectKey); err != nil {
			return nil, err
		}
	}
	return out, nil
}

// storeOriginal is the work of T1: lock and check, the row, file.updated and FileFinalized. It
// returns the file after the change, or the key this run wrote that nobody refers to when a
// precondition failed.
func (p *ImageProcessor) storeOriginal(ctx context.Context, f FileRecord, o StoredOriginal) (*FileRecord, string, error) {
	cur, found, err := p.d.Repo.LockImageFile(ctx, f.ID)
	if err != nil {
		return nil, "", err
	}
	if !found || !awaitingOriginal(cur, f.ObjectKey) {
		if o.ObjectKey != f.ObjectKey && (!found || !references(cur, o.ObjectKey)) {
			return nil, o.ObjectKey, nil
		}
		return nil, "", nil
	}
	if err := p.beforeCommit(ctx, f.ID); err != nil {
		return nil, "", err
	}
	if err := p.d.Repo.StoreImageOriginal(ctx, f.ID, o); err != nil {
		return nil, "", err
	}
	before := cur
	cur.ObjectKey, cur.SHA256, cur.SizeBytes, cur.MIMEType = o.ObjectKey, o.SHA256, o.SizeBytes, o.MIMEType
	cur.Width, cur.Height, cur.UploadState = o.Width, o.Height, domain.UploadComplete
	if err := p.changes().recordFileChange(ctx, actor.Actor{}, before, cur); err != nil {
		return nil, "", err
	}
	return &cur, "", nil
}

// beforeCommit runs the test seam BeforeCommit when it is set.
func (p *ImageProcessor) beforeCommit(ctx context.Context, fileID ids.ID) error {
	if h := p.d.Hooks.BeforeCommit; h != nil {
		return h(ctx, fileID)
	}
	return nil
}

// failImage is the failure transaction of an image whose original cannot be stored: a JPEG or PNG
// the filter refused (design note 10b, image_malformed) or an avatar or logo the purpose rule
// refused (brief M3-10 design note 4, file_type_denied): failed with reason, file.updated and
// FileFinalized, no audit entry; after the commit the original and any next revision go (12 12).
func (p *ImageProcessor) failImage(ctx context.Context, f FileRecord, reason, next string) error {
	failed := false
	err := p.d.UoW.Do(ctx, func(ctx context.Context) error {
		cur, found, err := p.d.Repo.LockImageFile(ctx, f.ID)
		if err != nil || !found || !awaitingOriginal(cur, f.ObjectKey) {
			return err
		}
		if err := p.d.Repo.FailImage(ctx, f.ID, reason); err != nil {
			return err
		}
		before := cur
		cur.UploadState = domain.UploadFailed
		failed = true
		return p.changes().recordFileChange(ctx, actor.Actor{}, before, cur)
	})
	if err != nil || !failed {
		return err
	}
	return p.deleteKeys(ctx, f.ObjectKey, next)
}

// deleteFailedOriginals removes the original of a failed image and its next revision again, for
// a retry after a failed deletion (design note 10e); a missing object counts as deleted.
func (p *ImageProcessor) deleteFailedOriginals(ctx context.Context, f FileRecord) error {
	if !stripPlan(domain.OriginalPlanFor(f.MIMEType)) && !domain.Purpose(f.Purpose).Image() {
		return nil
	}
	rev, err := revision(f)
	if err != nil {
		return err
	}
	next, err := domain.OriginalKey(f.ID, rev+1)
	if err != nil {
		return err
	}
	return p.deleteKeys(ctx, f.ObjectKey, next)
}

// deleteOlderRevisions deletes the revisions below the stored original, which a run that died
// after T1 may have left (design note 10e).
func (p *ImageProcessor) deleteOlderRevisions(ctx context.Context, f FileRecord) error {
	rev, err := revision(f)
	if err != nil {
		return err
	}
	keys := make([]string, 0, rev)
	for r := 1; r < rev; r++ {
		k, err := domain.OriginalKey(f.ID, r)
		if err != nil {
			return err
		}
		keys = append(keys, k)
	}
	return p.deleteKeys(ctx, keys...)
}

// deleteKeys deletes keys outside any unit of work; the first error is returned, so River
// retries (a missing object is no error, 12 4.1).
func (p *ImageProcessor) deleteKeys(ctx context.Context, keys ...string) error {
	for _, k := range keys {
		if err := p.d.Store.Delete(ctx, k); err != nil {
			return err
		}
	}
	return nil
}

// variantSpecs returns the variants of RenderedVariants that f has no row of yet, with their
// edges.
func (p *ImageProcessor) variantSpecs(f FileRecord) []VariantSpec {
	edges := map[domain.Variant]int{domain.VariantSmall: p.cfg.SmallPx, domain.VariantMedium: p.cfg.MediumPx, domain.VariantLarge: p.cfg.LargePx}
	have := map[domain.Variant]bool{}
	for _, t := range f.Thumbnails {
		have[t.Variant] = true
	}
	var out []VariantSpec
	for _, v := range domain.RenderedVariants() {
		if !have[v] {
			out = append(out, VariantSpec{Variant: v, LongestEdge: edges[v]})
		}
	}
	return out
}

// thumbnailPhase is design note 3d: render the missing variants of a processed original, write
// them, then T2. Pixel limit, undecodable image and a profile too large end the phase without
// thumbnails and without retry.
func (p *ImageProcessor) thumbnailPhase(ctx context.Context, f FileRecord) error {
	if domain.OriginalPlanFor(f.MIMEType) == domain.PlanNone {
		return nil
	}
	rev, err := revision(f)
	if err != nil || rev < 2 {
		return err
	}
	specs := p.variantSpecs(f)
	if len(specs) == 0 {
		return nil
	}
	out, err := p.d.Codec.RenderVariants(ctx, p.opener(f.ObjectKey), RenderSpec{MIMEType: f.MIMEType, Variants: specs,
		JPEGQuality: p.cfg.ThumbnailJPEGQuality, MaxPixels: p.cfg.MaxPixels})
	if err != nil {
		if permanent(err) {
			p.d.Log.InfoContext(ctx, msgImageNoThumbnails, p.logAttrs(f.ID, err)...)
			return nil
		}
		return err
	}
	rows := make([]ThumbnailRecord, 0, len(out))
	for _, v := range out {
		key, err := domain.VariantKey(f.ID, v.Variant)
		if err != nil {
			return err
		}
		if err := p.d.Store.Put(ctx, key, bytes.NewReader(v.Data), int64(len(v.Data))); err != nil {
			return err
		}
		rows = append(rows, ThumbnailRecord{Variant: v.Variant, ObjectKey: key, MIMEType: v.MIMEType, Width: v.Width, Height: v.Height,
			SizeBytes: int64(len(v.Data))})
	}
	if h := p.d.Hooks.AfterPut; h != nil {
		h(ctx, f.ID)
	}
	return p.commitThumbnails(ctx, f, rows)
}

// commitThumbnails is T2 (design note 10c). When a precondition fails, the variant objects the
// locked row does not refer to are deleted after the unit.
func (p *ImageProcessor) commitThumbnails(ctx context.Context, f FileRecord, rows []ThumbnailRecord) error {
	var stale []string
	err := p.d.UoW.Do(ctx, func(ctx context.Context) error {
		var err error
		stale, err = p.insertThumbnails(ctx, f, rows)
		return err
	})
	if err != nil {
		return err
	}
	return p.deleteKeys(ctx, stale...)
}

// insertThumbnails is the work of T2: lock and check, the rows and one file.updated when a row
// was inserted. It returns the keys of rows no locked row refers to when a precondition failed.
func (p *ImageProcessor) insertThumbnails(ctx context.Context, f FileRecord, rows []ThumbnailRecord) ([]string, error) {
	cur, found, err := p.d.Repo.LockImageFile(ctx, f.ID)
	if err != nil {
		return nil, err
	}
	if !found || cur.Deleted || cur.UploadState != domain.UploadComplete || cur.ObjectKey != f.ObjectKey {
		var stale []string
		for _, r := range rows {
			if !found || !references(cur, r.ObjectKey) {
				stale = append(stale, r.ObjectKey)
			}
		}
		return stale, nil
	}
	if err := p.beforeCommit(ctx, f.ID); err != nil {
		return nil, err
	}
	n, err := p.d.Repo.InsertThumbnails(ctx, f.ID, rows)
	if err != nil || n == 0 {
		return nil, err
	}
	before := cur
	cur.Thumbnails = mergeThumbnails(cur.Thumbnails, rows)
	return nil, p.changes().recordFileChange(ctx, actor.Actor{}, before, cur)
}

// mergeThumbnails adds the rows of variants have lacks.
func mergeThumbnails(have, rows []ThumbnailRecord) []ThumbnailRecord {
	out := append([]ThumbnailRecord(nil), have...)
	for _, r := range rows {
		dup := false
		for _, h := range have {
			dup = dup || h.Variant == r.Variant
		}
		if !dup {
			out = append(out, r)
		}
	}
	return out
}

// containerPhase is design note 3e: the display size and duration of an MP4 or QuickTime file,
// then T3 when a value is new. A box error leaves the values absent (D-M3-06-9); a read error of
// the store is transient.
func (p *ImageProcessor) containerPhase(ctx context.Context, f FileRecord) error {
	if !domain.HasContainerMetadata(f.MIMEType) {
		return nil
	}
	ra := &objectReaderAt{ctx: ctx, store: p.d.Store, key: f.ObjectKey, size: f.SizeBytes}
	info, err := p.d.Movies.ReadMovieInfo(ra, f.SizeBytes)
	if err != nil {
		if ra.err != nil {
			return ra.err
		}
		p.d.Log.InfoContext(ctx, msgImageNoMovieInfo, slog.String(logKeyFileID, f.ID.String()), slog.String(logKeyErrorType, classMalformed))
		return nil
	}
	if (mediaChange(f, info) == MediaInfo{}) {
		return nil
	}
	return p.d.UoW.Do(ctx, func(ctx context.Context) error { return p.storeMediaInfo(ctx, f.ID, info) })
}

// storeMediaInfo is T3 (design note 10d): lock, write the values that are new and one
// file.updated.
func (p *ImageProcessor) storeMediaInfo(ctx context.Context, fileID ids.ID, info MovieInfo) error {
	cur, found, err := p.d.Repo.LockImageFile(ctx, fileID)
	if err != nil || !found || cur.Deleted {
		return err
	}
	m := mediaChange(cur, info)
	if m == (MediaInfo{}) {
		return nil
	}
	if err := p.d.Repo.SetMediaInfo(ctx, fileID, m); err != nil {
		return err
	}
	before := cur
	if m.Width != 0 {
		cur.Width, cur.Height = m.Width, m.Height
	}
	if m.DurationMS != 0 {
		cur.DurationMS = m.DurationMS
	}
	return p.changes().recordFileChange(ctx, actor.Actor{}, before, cur)
}

// mediaChange returns the values of info that differ from the row; zero fields stay.
func mediaChange(f FileRecord, info MovieInfo) MediaInfo {
	var m MediaInfo
	if info.HasSize && info.Width > 0 && info.Height > 0 && (info.Width != f.Width || info.Height != f.Height) {
		m.Width, m.Height = info.Width, info.Height
	}
	if info.HasDuration && info.DurationMS > 0 && info.DurationMS != f.DurationMS {
		m.DurationMS = info.DurationMS
	}
	return m
}

// changes returns the collaborators of recordFileChange of the image job: file.updated without
// actor (a job without principal) and FileFinalized when T1 or the failure transaction makes the
// file final (D-M3-06-1).
func (p *ImageProcessor) changes() fileChanges {
	return fileChanges{attachments: p.d.Repo, channels: p.d.Channels, events: p.d.Events}
}

// objectReaderAt reads an object through ranged Get calls for the box reader (design note 2). It
// keeps the first store error, so the caller tells a store failure from a box error.
type objectReaderAt struct {
	ctx   context.Context //nolint:containedctx // the reader lives only inside one phase of one run
	store ObjectStore
	key   string
	size  int64
	err   error
}

// ReadAt implements io.ReaderAt.
func (o *objectReaderAt) ReadAt(b []byte, off int64) (int, error) {
	if off < 0 {
		return 0, errors.New("files: negative offset")
	}
	if off >= o.size {
		return 0, io.EOF
	}
	want := int64(len(b))
	short := false
	if want > o.size-off {
		want, short = o.size-off, true
	}
	if want == 0 {
		return 0, nil
	}
	r, err := o.store.Get(o.ctx, o.key, &ByteRange{Offset: off, Length: want})
	if err != nil {
		o.keep(err)
		return 0, err
	}
	defer func() { _ = r.Close() }()
	n, err := io.ReadFull(r, b[:want])
	if err != nil {
		o.keep(err)
		return n, err
	}
	if short {
		return n, io.EOF
	}
	return n, nil
}

// keep records the first store error.
func (o *objectReaderAt) keep(err error) {
	if o.err == nil {
		o.err = err
	}
}
