// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"time"

	"github.com/OWNER/bothy/services/server/internal/files/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
)

// ExtractConfig holds the configuration of the text extraction use cases, bound in internal/wire
// (wire.ExtractConfigs). Limits are named keys of docs/operations/configuration.md; the archive
// and text limits belong to the adapter (adapters/extract.Config).
type ExtractConfig struct {
	// MaxInputBytes is search.extract.max_input_bytes: larger files end failed input_too_large
	// without a read, and a run never reads more of an object.
	MaxInputBytes int64 `config:"search.extract.max_input_bytes"`
	// Timeout is search.extract.timeout, the deadline of reading and parsing one file.
	Timeout time.Duration `config:"search.extract.timeout"`
	// RefreshBatchSize is search.extract.refresh_batch_size, the files per run of the walk.
	RefreshBatchSize int `config:"search.extract.refresh_batch_size"`
}

// check refuses values the use cases cannot work with.
func (c ExtractConfig) check() error {
	if c.MaxInputBytes < 1 || c.Timeout <= 0 || c.RefreshBatchSize < 1 {
		return errors.New("files: search.extract.max_input_bytes, search.extract.timeout and search.extract.refresh_batch_size must be positive")
	}
	return nil
}

// ExtractDeps are the collaborators of the text extraction.
type ExtractDeps struct {
	// Repo reads and writes file_text_extracts.
	Repo ExtractRepository
	// Store reads the object of a file once per run.
	Store ObjectStore
	// UoW runs every write.
	UoW txn.UnitOfWork
	// Extractor is the outgoing port to the adapters (adapters/extract).
	Extractor TextExtractor
	// Queue inserts files.extract_text and the next run of the walk.
	Queue ExtractQueue
	// Languages gives the search language of the instance (identity.Directory).
	Languages SearchLanguages
	// Clock gives extracted_at; nil means clock.System().
	Clock clock.Clock
	// Log receives the Warn of a failed extraction and the Error of a missing object; nil means
	// slog.Default(). Lines carry only bothy.file.id and bothy.reason.
	Log *slog.Logger
}

// Extraction holds the use cases of the text extraction (docs/spec/14-search.md 5): the trigger
// that the writes making a file deliverable call, the job files.extract_text (Text) and the walk
// files.extract_text_refresh (Refresh). It is safe for concurrent use.
type Extraction struct {
	// Text is the use case of files.extract_text.
	Text *ExtractText
	// Refresh is the use case of files.extract_text_refresh.
	Refresh *ExtractRefresh
	trigger *ExtractTrigger
}

// NewExtraction returns the use cases. Errors: a missing collaborator or a value ExtractConfig
// refuses.
func NewExtraction(cfg ExtractConfig, d ExtractDeps) (*Extraction, error) {
	if d.Repo == nil || d.Store == nil || d.UoW == nil || d.Extractor == nil || d.Queue == nil || d.Languages == nil {
		return nil, errors.New("files: NewExtraction needs every collaborator")
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
	tr := &ExtractTrigger{d: d}
	return &Extraction{Text: &ExtractText{cfg: cfg, d: d}, Refresh: &ExtractRefresh{cfg: cfg, d: d, trigger: tr}, trigger: tr}, nil
}

// Trigger returns the trigger the scan pass and the image job take (ScanDeps.Extracts,
// ImageDeps.Extracts).
func (e *Extraction) Trigger() *ExtractTrigger { return e.trigger }

// ExtractTrigger creates the extract row and the job of a file that became deliverable (14 5.1
// item 1, brief M3-15 design note 2). recordFileChange calls it in the unit of work of the write
// that made the file deliverable, so the row and the job commit with that write.
type ExtractTrigger struct {
	d ExtractDeps
}

// schedule runs inside the caller's unit of work for a file f in its new state: when
// domain.ExtractApplies holds it inserts the row with the instance language and, only when it
// inserted one, the job files.extract_text. An unsupported type gets no row.
func (t *ExtractTrigger) schedule(ctx context.Context, f FileRecord) error {
	_, supported := t.d.Extractor.Version(f.MIMEType)
	if !domain.ExtractApplies(f.Purpose, domain.IsDeliverable(f.UploadState, f.ScanStatus, f.Deleted), supported) {
		return nil
	}
	lang, err := t.d.Languages.InstanceSearchLanguage(ctx)
	if err != nil {
		return err
	}
	return t.create(ctx, f.ID, lang)
}

// create inserts the pending row of fileID with lang and, when it inserted one, the job.
func (t *ExtractTrigger) create(ctx context.Context, fileID ids.ID, lang string) error {
	inserted, err := t.d.Repo.InsertTextExtract(ctx, fileID, lang)
	if err != nil || !inserted {
		return err
	}
	return t.d.Queue.EnqueueExtract(ctx, fileID)
}

// Log messages of the extraction; constant text, the only attributes are bothy.file.id and
// bothy.reason (SEC-LOG-4: never text, a part name or a file name).
const (
	msgExtractFailed  = "file text extraction failed"
	msgExtractMissing = "file text extraction found no object"
)

// ExtractText is the use case of the job files.extract_text (14 5.1 items 2 and 3, brief M3-15
// design note 4).
type ExtractText struct {
	cfg ExtractConfig
	d   ExtractDeps
}

// Run extracts the text of fileID. It reads the object outside every unit of work (02 8.5) under
// search.extract.timeout and writes the result in its own unit; a repeated run finds the row no
// longer pending and does nothing (02 12.4 rule 1). A content problem, the deadline and a missing
// object end failed and return nil. Errors: ctx.Err() when the job context ends (shutdown); a
// transient error of the object store or the database, so River retries with the row still
// pending.
func (u *ExtractText) Run(ctx context.Context, fileID ids.ID) error {
	t, found, err := u.d.Repo.ExtractTarget(ctx, fileID)
	if err != nil {
		return err
	}
	if !found || t.State != domain.ExtractPending {
		return nil
	}
	f := t.File
	if !domain.IsDeliverable(f.UploadState, f.ScanStatus, f.Deleted) {
		return u.d.UoW.Do(ctx, func(ctx context.Context) error { return u.d.Repo.DeleteTextExtract(ctx, fileID) })
	}
	version, ok := u.d.Extractor.Version(f.MIMEType)
	if !ok {
		return u.write(ctx, fileID, ExtractWrite{State: domain.ExtractUnsupported})
	}
	if f.SizeBytes > u.cfg.MaxInputBytes {
		return u.fail(ctx, fileID, version, domain.CodeInputTooLarge)
	}
	res, err := u.extract(ctx, f)
	if ctx.Err() != nil {
		return ctx.Err()
	}
	var xe *domain.ExtractError
	switch {
	case err == nil:
		return u.write(ctx, fileID, ExtractWrite{State: domain.ExtractDone, Text: res.Text, Truncated: res.Truncated, Version: res.Version})
	case errors.As(err, &xe):
		return u.fail(ctx, fileID, version, xe.Code)
	case errors.Is(err, ErrNotFound):
		u.d.Log.ErrorContext(ctx, msgExtractMissing, slog.String(logKeyFileID, fileID.String()),
			slog.String(logKeyReason, string(domain.CodeObjectMissing)))
		return u.write(ctx, fileID, ExtractWrite{State: domain.ExtractFailed, Version: version, Code: string(domain.CodeObjectMissing)})
	case errors.Is(err, context.DeadlineExceeded):
		return u.fail(ctx, fileID, version, domain.CodeTimeout)
	}
	return err
}

// extract opens the object and runs the extractor under the deadline search.extract.timeout,
// over a reader that fails with input_too_large after search.extract.max_input_bytes. The
// deadline of the extraction comes back as context.DeadlineExceeded.
func (u *ExtractText) extract(ctx context.Context, f FileRecord) (ExtractResult, error) {
	xctx, cancel := context.WithTimeout(ctx, u.cfg.Timeout)
	defer cancel()
	body, err := u.d.Store.Get(xctx, f.ObjectKey, nil)
	if err != nil {
		if xctx.Err() != nil {
			return ExtractResult{}, xctx.Err()
		}
		return ExtractResult{}, err
	}
	defer func() { _ = body.Close() }()
	res, err := u.d.Extractor.Extract(xctx, &inputLimit{r: body, left: u.cfg.MaxInputBytes}, f.MIMEType)
	if err != nil && xctx.Err() != nil {
		return ExtractResult{}, xctx.Err()
	}
	return res, err
}

// inputLimit reads at most left bytes and fails with input_too_large when the object has more:
// the stored size said it fits, but the bytes disagree.
type inputLimit struct {
	r    io.Reader
	left int64
}

// Read implements io.Reader.
func (l *inputLimit) Read(p []byte) (int, error) {
	if int64(len(p)) > l.left+1 {
		p = p[:l.left+1]
	}
	n, err := l.r.Read(p)
	if int64(n) > l.left {
		return int(l.left), domain.NewExtractError(domain.CodeInputTooLarge)
	}
	l.left -= int64(n)
	return n, err
}

// fail writes failed with code and logs it.
func (u *ExtractText) fail(ctx context.Context, fileID ids.ID, version string, code domain.ExtractCode) error {
	u.d.Log.WarnContext(ctx, msgExtractFailed, slog.String(logKeyFileID, fileID.String()), slog.String(logKeyReason, string(code)))
	return u.write(ctx, fileID, ExtractWrite{State: domain.ExtractFailed, Version: version, Code: string(code)})
}

// write stores w in its own unit of work with the clock time (design note 8). When the vector of
// the text exceeds the size limit of PostgreSQL the text is halved at a rune boundary, marked
// truncated and written again in a new unit, until it fits; the empty text always fits. No row
// changed (deleted file, row gone, another run first) is no error.
func (u *ExtractText) write(ctx context.Context, fileID ids.ID, w ExtractWrite) error {
	w.At = u.d.Clock.Now().UTC().Truncate(time.Microsecond)
	for {
		err := u.d.UoW.Do(ctx, func(ctx context.Context) error {
			_, err := u.d.Repo.WriteExtract(ctx, fileID, w)
			return err
		})
		if !errors.Is(err, ErrVectorTooLarge) {
			return err
		}
		if w.State != domain.ExtractDone || w.Text == "" {
			return errs.Wrap(errs.CodeInternalError, err, "files: an empty extract text was refused")
		}
		w.Text, w.Truncated = domain.HalveText(w.Text), true
	}
}
