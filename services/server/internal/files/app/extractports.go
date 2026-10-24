// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"errors"
	"io"
	"time"

	"github.com/OWNER/bothy/services/server/internal/files/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// TextExtractor is the outgoing port of the text extraction (docs/spec/14-search.md 5.1);
// internal/files/adapters/extract implements it in pure Go. Implementations are safe for
// concurrent use.
type TextExtractor interface {
	// Version names the adapter and version that extracts mimeType (the detected media type
	// without parameters), for example "builtin-ooxml/1"; ok is false when no adapter supports
	// the type.
	Version(mimeType string) (version string, ok bool)
	// Extract reads r and returns at most search.extract.max_text_bytes of text. Content problems
	// return *domain.ExtractError with a code of domain.ExtractCode; errors of r come back wrapped
	// and unchanged (errors.Is finds them); a done ctx returns ctx.Err(). An unsupported type is
	// an error without a code; callers ask Version first.
	Extract(ctx context.Context, r io.Reader, mimeType string) (ExtractResult, error)
}

// ExtractResult is the text of one extraction (domain.ExtractResult; the adapter package names it
// there, so it imports no package of app and with it no network package).
type ExtractResult = domain.ExtractResult

// ExtractQueue inserts the jobs of the text extraction (02 12.3 "Job catalogue"); adapters/jobs
// implements it over jobs.Inserter.InsertTx. It must be called inside a unit of work, so the job
// exists exactly when the change that needs it commits.
type ExtractQueue interface {
	// EnqueueExtract inserts files.extract_text for fileID (only the id in the arguments).
	EnqueueExtract(ctx context.Context, fileID ids.ID) error
	// EnqueueExtractRefresh inserts the next run of files.extract_text_refresh after the file id
	// after.
	EnqueueExtractRefresh(ctx context.Context, after ids.ID) error
}

// SearchLanguages answers the text search configuration of the instance (identity.Directory,
// M3-13): file text gets the language of locale.default (14 3.1).
type SearchLanguages interface {
	// InstanceSearchLanguage returns the regconfig name of locale.default.
	InstanceSearchLanguage(ctx context.Context) (string, error)
}

// ExtractTarget is what one run of files.extract_text reads (brief M3-15 design note 4a): the
// extract row and the facts of its file.
type ExtractTarget struct {
	// State is file_text_extracts.state.
	State string
	// File carries purpose, mime_type, size_bytes, object_key, upload_state, scan_status and
	// Deleted; the rest is zero.
	File FileRecord
}

// ExtractWrite is the result one run writes (design note 8). State is done, failed or
// unsupported; Text, Truncated and Version belong to done, Code and Version to failed.
type ExtractWrite struct {
	State     string
	Text      string
	Truncated bool
	// Version is file_text_extracts.extractor; "" writes NULL.
	Version string
	// Code is file_text_extracts.error; "" writes NULL.
	Code string
	// At is extracted_at, from kernel/clock.
	At time.Time
}

// RefreshCandidate is one deliverable attachment of the walk of files.extract_text_refresh with
// its extract row, if any (design note 9).
type RefreshCandidate struct {
	FileID    ids.ID
	MIMEType  string
	SizeBytes int64
	// HasRow reports a row in file_text_extracts; State, Extractor and Error are its columns, ""
	// for NULL.
	HasRow                  bool
	State, Extractor, Error string
}

// ErrVectorTooLarge reports that PostgreSQL refused the search vector of a text (SQLSTATE 54000,
// the tsvector size limit); the use case halves the text and writes again.
var ErrVectorTooLarge = errors.New("files: the search vector of the extracted text is too large")

// ExtractRepository is the outgoing port of the text extraction to file_text_extracts and the
// facts of files it needs (03 4.7); adapters/postgres implements it. The jobs have no principal,
// so every read is keyed by a file id (tools/lint/sql_visibility.yaml, category system_job).
type ExtractRepository interface {
	// InsertTextExtract inserts the pending row of fileID with the regconfig name lang unless a
	// row exists, and reports whether it inserted. It needs a unit of work.
	InsertTextExtract(ctx context.Context, fileID ids.ID, lang string) (bool, error)
	// ExtractTarget reads the row of fileID with its file; found is false without a row.
	ExtractTarget(ctx context.Context, fileID ids.ID) (ExtractTarget, bool, error)
	// WriteExtract writes w to the pending row of fileID whose file is not deleted and reports
	// whether a row changed. ErrVectorTooLarge when the vector of w.Text exceeds the limit. It
	// needs a unit of work.
	WriteExtract(ctx context.Context, fileID ids.ID, w ExtractWrite) (bool, error)
	// DeleteTextExtract deletes the row of fileID. It needs a unit of work.
	DeleteTextExtract(ctx context.Context, fileID ids.ID) error
	// RefreshCandidates returns up to limit deliverable attachments with id > after, ascending
	// by id, each with its extract row if any. It needs a unit of work.
	RefreshCandidates(ctx context.Context, after ids.ID, limit int) ([]RefreshCandidate, error)
	// ResetTextExtract sets the row of fileID pending when its state and extractor still equal
	// the seen values (extractor "" means NULL) and reports whether it did; text_content,
	// truncated, extractor and extracted_at stay. It needs a unit of work.
	ResetTextExtract(ctx context.Context, fileID ids.ID, seenState, seenExtractor string) (bool, error)
	// DeleteOrphanTextExtracts deletes up to limit rows whose file has deleted_at set and returns
	// how many. It needs a unit of work.
	DeleteOrphanTextExtracts(ctx context.Context, limit int) (int, error)
}
