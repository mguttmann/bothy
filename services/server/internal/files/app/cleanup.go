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
	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/eventlog"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
)

// The step names of the cleanup in its log lines (brief M3-08 design note 6).
const (
	StepExpiredSessions = "expired_sessions"
	StepUnattached      = "unattached"
	StepFailedRows      = "failed_rows"
	StepCatchUp         = "catch_up"
	StepTempFiles       = "temp_files"
	StepBacklog         = "backlog"
)

// The log keys and messages of the cleanup; lines carry at most bothy.file.id, a step name and
// counts, never a file name, key or digest (12 section 14, SEC-LOG-4).
const (
	logKeyStep        = "bothy.step"
	logKeyCount       = "bothy.count"
	msgCleanupStep    = "file cleanup step failed"
	msgCleanupFile    = "file cleanup of one file failed"
	msgCleanupTemp    = "file cleanup removed temporary files"
	msgCleanupBacklog = "file purge backlog older than privacy.object_delete_delay"
	msgCleanupCatchUp = "file cleanup catch-up stopped at a storage error"
)

// CleanupConfig holds the configuration of the job files.cleanup_pending, bound in internal/wire
// (wire.CleanupConfig). Windows are named keys of docs/operations/configuration.md.
type CleanupConfig struct {
	// UnattachedTTL is file.unattached_ttl, the life of a never attached file.
	UnattachedTTL time.Duration `config:"file.unattached_ttl"`
	// FailedRowTTL is file.failed_row_ttl, the life of the row of a failed or blocked file.
	FailedRowTTL time.Duration `config:"file.failed_row_ttl"`
	// SessionTTL is file.upload_session_ttl, also the age of a stale temporary file.
	SessionTTL time.Duration `config:"file.upload_session_ttl"`
	// ObjectDeleteDelay is privacy.object_delete_delay, the bound of the deletion backlog.
	ObjectDeleteDelay time.Duration `config:"privacy.object_delete_delay"`
	// BatchSize is file.gc_batch_size, the candidates per batch of a step.
	BatchSize int `config:"file.gc_batch_size"`
}

// check refuses values the cleanup cannot work with.
func (c CleanupConfig) check() error {
	if c.UnattachedTTL <= 0 || c.FailedRowTTL <= 0 || c.SessionTTL <= 0 || c.ObjectDeleteDelay <= 0 || c.BatchSize < 1 {
		return errors.New("files: file.unattached_ttl, file.failed_row_ttl, file.upload_session_ttl, privacy.object_delete_delay and file.gc_batch_size must be positive")
	}
	return nil
}

// CleanupDeps are the collaborators of the cleanup.
type CleanupDeps struct {
	// Store reads the candidates and the facts and writes the expiry.
	Store CleanupStore
	// Repo marks files deleted and reads the attachment row for file.updated.
	Repo Repository
	// Scans locks a session with its file in the order of the abort (session, then file).
	Scans ScanRepository
	// UoW runs one unit per candidate.
	UoW txn.UnitOfWork
	// Events appends file.updated of an expired session; Channels gives its scope.
	Events   eventlog.Appender
	Channels ChannelLocator
	// Redactor and Purge are the collaborators of markDeleted.
	Redactor eventlog.Redactor
	Purge    PurgeQueue
	// Purger runs the catch-up inline.
	Purger *Purge
	// Drafts answers whether a draft of the uploader holds a file (the module sync).
	Drafts files.DraftFileSource
	// Temp sweeps the temporary files of the file system driver; nil (S3) sweeps nothing.
	Temp TempSweeper
	// Metrics records bothy.file.purge.backlog; nil records nothing.
	Metrics PurgeMetrics
	// Clock gives now; nil means clock.System().
	Clock clock.Clock
	// Log receives the lines of the steps; nil means slog.Default().
	Log *slog.Logger
}

// Cleanup is the use case of the periodic job files.cleanup_pending (docs/spec/12-files.md
// section 12, brief M3-08 design note 6). It is safe for concurrent use; a repeated run has no
// double effect (02 12.4 rule 1).
type Cleanup struct {
	cfg CleanupConfig
	d   CleanupDeps
}

// NewCleanup returns the use case. Errors: a missing collaborator or a value CleanupConfig
// refuses.
func NewCleanup(cfg CleanupConfig, d CleanupDeps) (*Cleanup, error) {
	if d.Store == nil || d.Repo == nil || d.Scans == nil || d.UoW == nil || d.Events == nil || d.Channels == nil ||
		d.Redactor == nil || d.Purge == nil || d.Purger == nil || d.Drafts == nil {
		return nil, errors.New("files: NewCleanup needs every collaborator")
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
		d.Metrics = noPurgeMetrics{}
	}
	return &Cleanup{cfg: cfg, d: d}, nil
}

// Run runs the steps a to f once with one reading of the clock: expired sessions, never attached
// files, failed and blocked rows, the catch-up of purges, the temporary files and the backlog.
// Every step reads its candidates in batches of file.gc_batch_size and handles each in its own
// short unit of work, so one failing candidate never rolls back the others (02 12.4 rule 6). A
// database error ends only its step with one Warn line; Run returns the first error after the
// last step, so River records it. A cancelled ctx ends the run with ctx.Err().
func (c *Cleanup) Run(ctx context.Context) error {
	now := c.d.Clock.Now().UTC().Truncate(time.Microsecond)
	var first error
	for _, step := range []struct {
		name string
		run  func(context.Context, time.Time) error
	}{
		{StepExpiredSessions, c.expiredSessions},
		{StepUnattached, c.unattached},
		{StepFailedRows, c.failedRows},
		{StepCatchUp, c.catchUp},
		{StepTempFiles, c.tempFiles},
		{StepBacklog, c.backlog},
	} {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := step.run(ctx, now); err != nil {
			c.d.Log.LogAttrs(ctx, slog.LevelWarn, msgCleanupStep, slog.String(logKeyStep, step.name), slog.String(logKeyErrorType, errorType(err)))
			if first == nil {
				first = err
			}
		}
	}
	return first
}

// batches reads candidates with read in pages of file.gc_batch_size until a page is smaller than
// the limit or ctx ends, and hands each to handle. A read error ends the step; an error of handle
// is logged for the file and remembered, and the step goes on.
func (c *Cleanup) batches(ctx context.Context, step string, read func(after Candidate, limit int) ([]Candidate, error),
	handle func(ctx context.Context, id ids.ID) error) error {
	var after Candidate
	var first error
	for ctx.Err() == nil {
		page, err := read(after, c.cfg.BatchSize)
		if err != nil {
			return err
		}
		if err := c.page(ctx, step, page, handle); err != nil && first == nil {
			first = err
		}
		if len(page) < c.cfg.BatchSize {
			break
		}
		after = page[len(page)-1]
	}
	if first == nil {
		first = ctx.Err()
	}
	return first
}

// page hands each candidate of one page to handle until ctx ends; an error is logged with the file
// id and the step, and the first one is returned after the page.
func (c *Cleanup) page(ctx context.Context, step string, page []Candidate, handle func(ctx context.Context, id ids.ID) error) error {
	var first error
	for _, cand := range page {
		if ctx.Err() != nil {
			break
		}
		if err := handle(ctx, cand.FileID); err != nil {
			c.d.Log.LogAttrs(ctx, slog.LevelWarn, msgCleanupFile, slog.String(logKeyStep, step),
				slog.String(logKeyFileID, cand.FileID.String()), slog.String(logKeyErrorType, errorType(err)))
			if first == nil {
				first = err
			}
		}
	}
	return first
}

// expiredSessions is step a (12 12 row "Session open and expired", 12 3.6): an open session past
// expires_at becomes expired, its file failed with file.updated (no actor, a job without
// principal), its part rows go, and recordFileChange inserts the purge, which removes the parts or
// aborts the multipart upload.
func (c *Cleanup) expiredSessions(ctx context.Context, now time.Time) error {
	return c.batches(ctx, StepExpiredSessions, func(after Candidate, limit int) ([]Candidate, error) {
		return c.d.Store.ExpiredOpenSessions(ctx, now, after, limit)
	}, func(ctx context.Context, id ids.ID) error {
		return c.d.UoW.Do(ctx, func(ctx context.Context) error {
			cur, found, err := c.d.Scans.LockScanTarget(ctx, id)
			if err != nil || !found || cur.File.Deleted || !domain.SessionPastExpiry(cur.State, cur.ExpiresAt, now) {
				return err
			}
			if err := c.d.Store.ExpireSession(ctx, id); err != nil {
				return err
			}
			f := cur.File
			f.UploadState = domain.UploadFailed
			return c.changes().recordFileChange(ctx, actor.Actor{}, cur.File, f)
		})
	})
}

// unattached is step b (12 12 row "Never attached", 04 14 row "Drafts"): a deliverable attachment
// file that no message holds and no draft of its uploader holds is deleted once
// domain.UnattachedExpired; a file ID in the draft of another principal protects nothing.
func (c *Cleanup) unattached(ctx context.Context, now time.Time) error {
	cutoff := now.Add(-c.cfg.UnattachedTTL)
	return c.batches(ctx, StepUnattached, func(after Candidate, limit int) ([]Candidate, error) {
		return c.d.Store.UnattachedCandidates(ctx, cutoff, after, limit)
	}, func(ctx context.Context, id ids.ID) error {
		return c.d.UoW.Do(ctx, func(ctx context.Context) error {
			cur, ok, err := c.locked(ctx, id)
			if err != nil || !ok {
				return err
			}
			if cur.Purpose != string(domain.PurposeAttachment) || cur.Deleted || cur.Attached ||
				!domain.IsDeliverable(cur.UploadState, cur.ScanStatus, false) || !domain.UnattachedExpired(cur.Ages, now, c.cfg.UnattachedTTL) {
				return nil
			}
			held, err := c.d.Drafts.HeldByUploader(ctx, []files.UploaderFile{{FileID: id, UploaderID: cur.UploadedBy}})
			if err != nil || held[id] {
				return err
			}
			return c.deletion().mark(ctx, Deletion{FileIDs: []ids.ID{id}}, now)
		})
	})
}

// failedRows is step c (12 12 row "Failed or blocked"): the row of a failed or blocked attachment
// file that hangs on no message is deleted once domain.FailedRowExpired. Drafts do not protect it,
// and avatar and branding rows are left to their release (docs/decisions.d/M3-08.md D-M3-08-7).
func (c *Cleanup) failedRows(ctx context.Context, now time.Time) error {
	cutoff := now.Add(-c.cfg.FailedRowTTL)
	return c.batches(ctx, StepFailedRows, func(after Candidate, limit int) ([]Candidate, error) {
		return c.d.Store.FailedRowCandidates(ctx, cutoff, after, limit)
	}, func(ctx context.Context, id ids.ID) error {
		return c.d.UoW.Do(ctx, func(ctx context.Context) error {
			cur, ok, err := c.locked(ctx, id)
			if err != nil || !ok {
				return err
			}
			blocked := domain.Final(cur.UploadState, cur.ScanStatus, false) && !domain.IsDeliverable(cur.UploadState, cur.ScanStatus, false)
			if cur.Purpose != string(domain.PurposeAttachment) || cur.Deleted || cur.Attached || !blocked ||
				!domain.FailedRowExpired(cur.Ages, now, c.cfg.FailedRowTTL) {
				return nil
			}
			return c.deletion().mark(ctx, Deletion{FileIDs: []ids.ID{id}}, now)
		})
	})
}

// locked locks the files row of id and reads its facts in a new statement; ok is false when the
// row is gone.
func (c *Cleanup) locked(ctx context.Context, id ids.ID) (CleanupRecord, bool, error) {
	found, err := c.d.Store.LockFile(ctx, id)
	if err != nil || !found {
		return CleanupRecord{}, false, err
	}
	return c.d.Store.CleanupRecord(ctx, id)
}

// catchUp is step d (12 12 "catches up on missed deletions"): every row marked deleted is purged
// inline instead of inserting a further job, so a long store outage piles up no jobs. The first
// error ends the step for this run with one Warn line that names the backlog; the next run tries
// again. A row a running purge job removes meanwhile is skipped (PurgeNothing).
func (c *Cleanup) catchUp(ctx context.Context, _ time.Time) error {
	var after Candidate
	for ctx.Err() == nil {
		page, err := c.d.Store.DeletedRows(ctx, after, c.cfg.BatchSize)
		if err != nil {
			return err
		}
		for _, cand := range page {
			if err := c.d.Purger.Run(ctx, cand.FileID); err != nil {
				left, _, _ := c.d.Store.PurgeBacklog(ctx)
				c.d.Log.LogAttrs(ctx, slog.LevelWarn, msgCleanupCatchUp, slog.String(logKeyStep, StepCatchUp),
					slog.Int64(logKeyCount, left), slog.String(logKeyErrorType, errorType(err)))
				return err
			}
		}
		if len(page) < c.cfg.BatchSize {
			return nil
		}
		after = page[len(page)-1]
	}
	return ctx.Err()
}

// tempFiles is step e (12 4.2): the temporary files of the file system driver older than
// file.upload_session_ttl belong to a crashed process and go; one Info line with the count.
func (c *Cleanup) tempFiles(ctx context.Context, now time.Time) error {
	if c.d.Temp == nil {
		return nil
	}
	n, err := c.d.Temp.SweepTemp(ctx, now.Add(-c.cfg.SessionTTL))
	if n > 0 {
		c.d.Log.LogAttrs(ctx, slog.LevelInfo, msgCleanupTemp, slog.String(logKeyStep, StepTempFiles), slog.Int(logKeyCount, n))
	}
	return err
}

// backlog is step f (12 14 "deletion backlog"): the number of rows marked deleted goes to
// bothy.file.purge.backlog, and when the oldest mark is older than privacy.object_delete_delay one
// Warn line names the count, never an id.
func (c *Cleanup) backlog(ctx context.Context, now time.Time) error {
	n, oldest, err := c.d.Store.PurgeBacklog(ctx)
	if err != nil {
		return err
	}
	c.d.Metrics.PurgeBacklog(ctx, n)
	if n > 0 && now.Sub(oldest) > c.cfg.ObjectDeleteDelay {
		c.d.Log.LogAttrs(ctx, slog.LevelWarn, msgCleanupBacklog, slog.String(logKeyStep, StepBacklog), slog.Int64(logKeyCount, n))
	}
	return nil
}

// changes returns the collaborators of recordFileChange of the cleanup: file.updated without
// actor and the purge of the expired file.
func (c *Cleanup) changes() fileChanges {
	return fileChanges{attachments: c.d.Repo, channels: c.d.Channels, events: c.d.Events, purge: c.d.Purge}
}

// deletion returns the collaborators of markDeleted of the cleanup.
func (c *Cleanup) deletion() deletion {
	return deletion{repo: c.d.Repo, redactor: c.d.Redactor, purge: c.d.Purge}
}
