// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"errors"
	"fmt"

	"github.com/OWNER/bothy/services/server/internal/files"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/personaldata"
	"github.com/OWNER/bothy/services/server/internal/privacy/domain"
)

// begin moves an approved request of kind to running (with gdpr.request_updated) and returns it;
// a running request (a retry after a crash) is returned as it is. done is true for a request the
// job has nothing to do for any more: unknown, of another kind, or in any other state.
func (s *Service) begin(ctx context.Context, id ids.ID, kind domain.Kind, start func(*domain.Request)) (domain.Request, bool, error) {
	var r domain.Request
	done := false
	err := s.d.UoW.Do(ctx, func(ctx context.Context) error {
		var ok bool
		var err error
		if r, ok, err = s.d.Store.Lock(ctx, id); err != nil {
			return err
		}
		if !ok || r.Kind != kind || (r.State != domain.StateApproved && r.State != domain.StateRunning) {
			done = true
			return nil
		}
		if r.State == domain.StateRunning {
			return nil
		}
		r.State = domain.StateRunning
		if start != nil {
			start(&r)
		}
		if err := s.d.Store.Update(ctx, r); err != nil {
			return err
		}
		return s.appendUpdated(ctx, r)
	})
	return r, done, err
}

// finish writes the final state of r in one unit with gdpr.request_updated.
func (s *Service) finish(ctx context.Context, id ids.ID, change func(context.Context, *domain.Request) error) error {
	return s.d.UoW.Do(ctx, func(ctx context.Context) error {
		r, ok, err := s.d.Store.Lock(ctx, id)
		if err != nil {
			return err
		}
		if !ok || r.State != domain.StateRunning {
			return nil
		}
		if err := change(ctx, &r); err != nil {
			return err
		}
		if err := s.d.Store.Update(ctx, r); err != nil {
			return err
		}
		return s.appendUpdated(ctx, r)
	})
}

// Fail ends the running request id as failed with the error code code (no text, no content); the
// job calls it after its last attempt. A request that is not running is left alone.
func (s *Service) Fail(ctx context.Context, id ids.ID, code string) error {
	return s.finish(ctx, id, func(_ context.Context, r *domain.Request) error {
		r.State, r.Error, r.CompletedAt = domain.StateFailed, code, s.now()
		return nil
	})
}

// RunExport is the job privacy.export (20 section 5.2, brief M7-06 design note 2): it moves the
// approved request to running, streams every participant into the package in temporary storage,
// writes README.txt in the language of the person and manifest.json last, stores the ZIP through
// the files port with purpose gdpr_export and completes the request with result_expires_at =
// now + privacy.export_ttl. A retry builds the package again from the start; a package stored by
// a failed attempt is deleted again.
//
// Errors: those of the participants, the archive and the storage; the job retries.
func (s *Service) RunExport(ctx context.Context, id ids.ID) error {
	r, done, err := s.begin(ctx, id, domain.KindExport, nil)
	if err != nil || done {
		return err
	}
	built, arc, err := s.build(ctx, r)
	if arc != nil {
		defer arc.Discard()
	}
	if err != nil {
		return err
	}
	body, err := built.Open()
	if err != nil {
		return err
	}
	now := s.now()
	fileID, err := s.d.Packages.Store(ctx, files.StoreExport{
		SubjectID: r.SubjectUserID, Name: domain.PackageName(r.SubjectUserID, now), SizeBytes: built.Size, SHA256: built.SHA256, Body: body,
	})
	_ = body.Close()
	if err != nil {
		return err
	}
	err = s.finish(ctx, id, func(_ context.Context, r *domain.Request) error {
		r.State, r.CompletedAt = domain.StateCompleted, now
		r.ResultFileID, r.ResultExpiresAt = fileID, now.Add(s.cfg.ExportTTL)
		r.Progress.Bytes = built.Size
		return nil
	})
	if err != nil {
		// The request did not take the package: nobody can download it, so it goes at once.
		if derr := s.d.UoW.Do(ctx, func(ctx context.Context) error { return s.d.Packages.Delete(ctx, fileID) }); derr != nil {
			return errors.Join(err, derr)
		}
	}
	return err
}

// build writes the package of r into a new archive; arc is returned for Discard also on error.
func (s *Service) build(ctx context.Context, r domain.Request) (Built, Archive, error) {
	arc, err := s.d.Archives.New()
	if err != nil {
		return Built{}, nil, err
	}
	e := personaldata.Export{Subject: r.SubjectUserID, Readable: func(ctx context.Context, channelID ids.ID) (bool, error) {
		if s.d.Readable == nil {
			return false, nil
		}
		return s.d.Readable(ctx, r.SubjectUserID, channelID)
	}}
	for _, p := range s.d.Participants.All() {
		if err := p.Export(ctx, e, arc); err != nil {
			return Built{}, arc, fmt.Errorf("privacy: export of module %s: %w", p.Module(), err)
		}
	}
	user, err := s.d.People.User(ctx, r.SubjectUserID)
	if err != nil {
		return Built{}, arc, err
	}
	inst, err := s.d.InstanceID(ctx)
	if err != nil {
		return Built{}, arc, err
	}
	built, err := arc.Finish(domain.Readme(domain.ReadmeLanguage(user.Locale)), domain.Manifest{
		SchemaVersion: domain.SchemaVersion, InstanceID: inst, SubjectUserID: r.SubjectUserID, RequestID: r.ID, CreatedAt: s.now(),
	})
	return built, arc, err
}

// ExpirePackages is the job privacy.export_expiry (DP-T04): every completed export whose
// result_expires_at has passed loses its package: the file is deleted through the files port
// (its object goes with files.purge_objects within privacy.object_delete_delay) and
// result_file_id is cleared in the same unit, so downloadGdprExport answers 410 gone. It works
// in batches of batch requests and returns the number of packages deleted.
//
// Errors: the storage; the job retries.
func (s *Service) ExpirePackages(ctx context.Context, batch int) (int, error) {
	if batch < 1 {
		return 0, errs.New(errs.CodeInternalError, "privacy: the batch size must be positive")
	}
	n := 0
	for {
		rows, err := s.d.Store.ExpiredPackages(ctx, s.now(), batch)
		if err != nil || len(rows) == 0 {
			return n, err
		}
		for _, r := range rows {
			if err := s.d.UoW.Do(ctx, func(ctx context.Context) error { return s.expireOne(ctx, r.ID) }); err != nil {
				return n, err
			}
			n++
		}
		if len(rows) < batch {
			return n, nil
		}
	}
}

// expireOne deletes the package of the request id in the unit of work of ctx, if it still has one.
func (s *Service) expireOne(ctx context.Context, id ids.ID) error {
	cur, ok, err := s.d.Store.Lock(ctx, id)
	if err != nil || !ok || cur.ResultFileID.IsZero() {
		return err
	}
	if err := s.d.Packages.Delete(ctx, cur.ResultFileID); err != nil {
		return err
	}
	cur.ResultFileID = ids.ID{}
	if err := s.d.Store.Update(ctx, cur); err != nil {
		return err
	}
	return s.appendUpdated(ctx, cur)
}
