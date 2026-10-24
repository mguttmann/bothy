// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"

	"github.com/OWNER/bothy/services/server/internal/files/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// ExtractRefresh is the use case of the periodic job files.extract_text_refresh (14 5.1 item 4,
// brief M3-15 design note 9): it walks the deliverable attachments by id in batches and re-queues
// what an older extractor version produced, what failed only for its size, what has no row and
// what is pending without a job.
type ExtractRefresh struct {
	cfg     ExtractConfig
	d       ExtractDeps
	trigger *ExtractTrigger
}

// Run decides the next search.extract.refresh_batch_size files after the id after in one unit of
// work (domain.RefreshDecision), deletes as many rows of deleted files, and inserts the next run
// when the batch was full. A reset is a compare-and-set that keeps the old text, so a file stays
// findable while it is re-extracted. Errors: the unit of work failed; River retries the run.
func (u *ExtractRefresh) Run(ctx context.Context, after ids.ID) error {
	return u.d.UoW.Do(ctx, func(ctx context.Context) error {
		cands, err := u.d.Repo.RefreshCandidates(ctx, after, u.cfg.RefreshBatchSize)
		if err != nil {
			return err
		}
		lang := ""
		for _, c := range cands {
			current, supported := u.d.Extractor.Version(c.MIMEType)
			row := domain.RefreshRow{Exists: c.HasRow, State: c.State, Extractor: c.Extractor, Error: c.Error, SizeBytes: c.SizeBytes}
			switch domain.RefreshDecision(row, current, supported, u.cfg.MaxInputBytes) {
			case domain.RefreshCreate:
				if lang == "" {
					if lang, err = u.d.Languages.InstanceSearchLanguage(ctx); err != nil {
						return err
					}
				}
				err = u.trigger.create(ctx, c.FileID, lang)
			case domain.RefreshEnqueue:
				err = u.d.Queue.EnqueueExtract(ctx, c.FileID)
			case domain.RefreshReset:
				err = u.reset(ctx, c)
			case domain.RefreshNothing:
			}
			if err != nil {
				return err
			}
		}
		if _, err := u.d.Repo.DeleteOrphanTextExtracts(ctx, u.cfg.RefreshBatchSize); err != nil {
			return err
		}
		if len(cands) == u.cfg.RefreshBatchSize {
			return u.d.Queue.EnqueueExtractRefresh(ctx, cands[len(cands)-1].FileID)
		}
		return nil
	})
}

// reset sets the row of c pending when it is unchanged since the read and enqueues the job.
func (u *ExtractRefresh) reset(ctx context.Context, c RefreshCandidate) error {
	ok, err := u.d.Repo.ResetTextExtract(ctx, c.FileID, c.State, c.Extractor)
	if err != nil || !ok {
		return err
	}
	return u.d.Queue.EnqueueExtract(ctx, c.FileID)
}
