// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"errors"
	"time"

	"github.com/OWNER/bothy/services/server/internal/identity"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/auditing"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/privacy"
	"github.com/OWNER/bothy/services/server/internal/privacy/domain"
)

// The decisions of GdprDecision.
const (
	decisionApprove = "approve"
	decisionReject  = "reject"
)

// reasonRequestOpen is the reason of the 409 for a second open request (brief M7-06 item 15).
const reasonRequestOpen = "request_open"

// Create implements privacy.Requests (createGdprRequest, 20 section 5.1).
func (s *Service) Create(ctx context.Context, a actor.Actor, cmd privacy.CreateCmd) (privacy.Request, error) {
	ctx = actor.WithActor(ctx, a)
	subject := cmd.SubjectUserID
	if subject.IsZero() {
		subject = a.ID
	}
	kind, err := s.checkCreate(ctx, a, subject, cmd)
	if err != nil {
		return privacy.Request{}, err
	}
	now := s.now()
	r := domain.Request{
		ID: ids.New(), SubjectUserID: subject, Kind: kind, State: domain.StateRequested, RequestedBy: a.ID, Reason: cmd.Reason,
		DueAt: domain.DueAt(now), CreatedAt: now,
	}
	auto := domain.AutoApproved(kind, subject, a.ID, s.cfg.ExportRequiresApproval)
	if auto {
		r.State = domain.StateApproved
	}
	if err := s.d.UoW.Do(ctx, func(ctx context.Context) error { return s.insert(ctx, r, auto) }); err != nil {
		return privacy.Request{}, err
	}
	return view(r), nil
}

// checkCreate authorizes gdpr.request.create on the person subject and checks the input: the
// kind, the reason, the person and, for a deletion, that the person is not the last owner.
func (s *Service) checkCreate(ctx context.Context, a actor.Actor, subject ids.ID, cmd privacy.CreateCmd) (domain.Kind, error) {
	if err := s.check(ctx, a, authorize.ActionGdprRequestCreate,
		authorize.Resource{Kind: authorize.ResourceUser, ID: subject, OwnerID: subject}); err != nil {
		return "", err
	}
	kind := domain.Kind(cmd.Kind)
	if !kind.Valid() {
		return "", invalidEnum("/kind", "unknown kind")
	}
	if err := domain.CheckReason(cmd.Reason, "/reason"); err != nil {
		return "", err
	}
	if _, err := s.d.People.User(ctx, subject); err != nil {
		return "", err
	}
	if kind == domain.KindDeletion {
		return kind, s.checkNotLastOwner(ctx, subject)
	}
	return kind, nil
}

// insert stores the new request r in the unit of work of ctx with its job (auto: the export
// approved by the system), gdpr.request_updated and the audit entry.
func (s *Service) insert(ctx context.Context, r domain.Request, auto bool) error {
	// The open request is named before the insert: after a unique violation the transaction can
	// read nothing more. A concurrent insert still meets the index and gets the 409 without the
	// name.
	open, ok, err := s.d.Store.OpenOf(ctx, r.SubjectUserID, r.Kind)
	if err != nil {
		return err
	}
	if ok {
		return openConflict(&open)
	}
	if err := s.d.Store.Insert(ctx, r); err != nil {
		if errors.Is(err, ErrOpen) {
			return openConflict(nil)
		}
		return err
	}
	if auto {
		if err := s.d.Jobs.EnqueueExport(ctx, r.ID); err != nil {
			return err
		}
	}
	if err := s.appendUpdated(ctx, r); err != nil {
		return err
	}
	return s.d.Audit.Record(ctx, auditing.Entry{
		Action: string(authorize.ActionGdprRequestCreate), Decision: auditing.DecisionAllow, ResourceKind: resourceRequest,
		ResourceID: r.ID, Attrs: auditing.Attrs{
			attrKind: auditing.Code(string(r.Kind)), attrSubject: auditing.ID(r.SubjectUserID), attrReason: auditing.ReasonText(r.Reason),
			attrDueAt: auditing.Time(r.DueAt), attrAutoApproved: auditing.Bool(auto),
		},
	})
}

// invalidEnum is the 422 of a value outside its enum at pointer.
func invalidEnum(pointer, detail string) error {
	return errs.New(errs.CodeValidationFailed, detail).WithFields(errs.FieldError{Pointer: pointer, Code: codeInvalidEnum, Detail: detail})
}

// codeInvalidEnum is the field error code of a value outside its enum.
const codeInvalidEnum = "invalid_enum"

// openConflict is the 409 of a second open request; the detail and current name the open request
// so the client can follow it (brief M7-06 item 15). open is nil when it is not known.
func openConflict(open *domain.Request) error {
	detail := "an open request of this kind exists"
	if open == nil {
		return errs.New(errs.CodeConflict, detail).WithReason(reasonRequestOpen)
	}
	return errs.New(errs.CodeConflict, detail+": "+open.ID.String()).WithReason(reasonRequestOpen).WithCurrent(requestJSON(view(*open)))
}

// checkNotLastOwner refuses the deletion of the last active owner with 409 last_owner (20 4.4
// item 1, 08 SR8): at least one active owner must remain.
func (s *Service) checkNotLastOwner(ctx context.Context, subject ids.ID) error {
	owners, err := s.d.People.ActiveOwners(ctx)
	if err != nil {
		return err
	}
	isOwner, others := false, 0
	for _, o := range owners {
		if o.User.Status != identity.StatusActive {
			continue
		}
		if o.User.ID == subject {
			isOwner = true
			continue
		}
		others++
	}
	if isOwner && others == 0 {
		return errs.New(errs.CodeLastOwner, "the last active owner cannot be deleted")
	}
	return nil
}

// Get implements privacy.Requests (getGdprRequest).
func (s *Service) Get(ctx context.Context, a actor.Actor, id ids.ID) (privacy.Request, error) {
	r, ok, err := s.d.Store.Get(ctx, id)
	if err != nil {
		return privacy.Request{}, err
	}
	if !ok {
		return privacy.Request{}, notFound()
	}
	if err := s.check(ctx, a, authorize.ActionGdprRequestRead, requestResource(r)); err != nil {
		return privacy.Request{}, err
	}
	return view(r), nil
}

// listName is the name of the list cursor of listGdprRequests.
const listName = "gdpr_requests"

// listCursor is the keyset position of listGdprRequests.
type listCursor struct {
	At string `json:"at"`
	ID string `json:"id"`
}

// cursorTime keeps the microseconds of created_at in a cursor.
const cursorTime = "2006-01-02T15:04:05.000000Z07:00"

// List implements privacy.Requests (listGdprRequests): the requests the caller may read
// (gdpr.request.read in filter mode per request, so an owner sees all and a person their own),
// newest first. A page can hold fewer than limit items while further ones follow.
func (s *Service) List(ctx context.Context, a actor.Actor, q privacy.ListQuery) (privacy.Page, error) {
	f := ListFilter{Subject: q.SubjectUserID, State: domain.State(q.State)}
	if q.State != "" && !f.State.Valid() {
		return privacy.Page{}, invalidEnum("/state", "unknown state")
	}
	// The caller must be signed in and active: gdpr.request.read on its own requests, decided
	// with the liveness checks (an anonymous or disabled caller gets 401 or 403), before any
	// decision in filter mode, which leaves those checks to the request.
	if err := s.check(ctx, a, authorize.ActionGdprRequestRead, authorize.Resource{Kind: authorize.ResourceGdprRequest, OwnerID: a.ID}); err != nil {
		return privacy.Page{}, err
	}
	// A caller who may not read the requests of others (no owner) lists only their own: without
	// subject_user_id the list is narrowed to the caller, and a foreign subject_user_id is an
	// empty page without a cursor, so neither the items nor the paging reveal another person's
	// requests.
	others := s.check(authorize.Filter(ctx), a, authorize.ActionGdprRequestRead, authorize.Resource{Kind: authorize.ResourceGdprRequest}) == nil
	switch {
	case others:
	case f.Subject.IsZero():
		f.Subject = a.ID
	case f.Subject != a.ID:
		return privacy.Page{Items: []privacy.Request{}}, nil
	}
	binding := []byte(a.ID.String() + "\x00" + f.Subject.String() + "\x00" + string(f.State))
	after, err := s.listAfter(binding, q.Cursor)
	if err != nil {
		return privacy.Page{}, err
	}
	n := s.pageSize(q.Limit)
	rows, err := s.d.Store.List(ctx, f, after, n+1)
	if err != nil {
		return privacy.Page{}, err
	}
	more := len(rows) > n
	if more {
		rows = rows[:n]
	}
	out := privacy.Page{Items: []privacy.Request{}}
	for _, r := range rows {
		d, err := s.d.Authorizer.Authorize(authorize.Filter(ctx), a, authorize.ActionGdprRequestRead, requestResource(r))
		if err != nil {
			return privacy.Page{}, err
		}
		if d.Allowed() {
			out.Items = append(out.Items, view(r))
		}
	}
	if more {
		last := rows[n-1]
		out.NextCursor, err = s.d.Cursors.Encode(listName, binding, listCursor{At: last.CreatedAt.UTC().Format(cursorTime), ID: last.ID.String()})
	}
	return out, err
}

// listAfter decodes the cursor of listGdprRequests; nil for the first page. Errors: 400
// cursor_invalid.
func (s *Service) listAfter(binding []byte, cursor string) (*Key, error) {
	if cursor == "" {
		return nil, nil
	}
	var k listCursor
	if err := s.d.Cursors.Decode(listName, binding, cursor, &k); err != nil {
		return nil, err
	}
	at, errT := time.Parse(cursorTime, k.At)
	id, errID := ids.Parse(k.ID)
	if errT != nil || errID != nil {
		return nil, errs.New(errs.CodeCursorInvalid, "")
	}
	return &Key{CreatedAt: at, ID: id}, nil
}

// pageSize returns the page size of a request: 0 is api.page_size_default, larger values are
// capped at api.page_size_max.
func (s *Service) pageSize(n int) int {
	switch {
	case n <= 0:
		return s.cfg.PageSizeDefault
	case n > s.cfg.PageSizeMax:
		return s.cfg.PageSizeMax
	}
	return n
}

// Decide implements privacy.Requests (decideGdprRequest, 20 section 5.1 item 2): approve or
// reject a requested request; an approved export or deletion gets its job, an approved
// rectification is completed by the decision (20 section 5.3). ExtendMonths moves due_at once.
func (s *Service) Decide(ctx context.Context, a actor.Actor, cmd privacy.DecideCmd) (privacy.Request, error) {
	ctx = actor.WithActor(ctx, a)
	if cmd.Decision != decisionApprove && cmd.Decision != decisionReject {
		return privacy.Request{}, invalidEnum("/decision", "unknown decision")
	}
	var out domain.Request
	err := s.d.UoW.Do(ctx, func(ctx context.Context) error {
		var err error
		out, err = s.decideLocked(ctx, a, cmd)
		return err
	})
	if err != nil {
		return privacy.Request{}, err
	}
	return view(out), nil
}

// decideLocked runs a decision in the unit of work of ctx: lock, authorize, check, apply, store,
// gdpr.request_updated and the audit entry with the extension, if any.
func (s *Service) decideLocked(ctx context.Context, a actor.Actor, cmd privacy.DecideCmd) (domain.Request, error) {
	r, ok, err := s.d.Store.Lock(ctx, cmd.RequestID)
	if err != nil {
		return r, err
	}
	if !ok {
		return r, notFound()
	}
	if err := s.check(ctx, a, authorize.ActionGdprRequestDecide, requestResource(r)); err != nil {
		return r, err
	}
	if err := domain.CheckReason(cmd.Reason, "/reason"); err != nil {
		return r, err
	}
	now := s.now()
	attrs := auditing.Attrs{
		attrKind: auditing.Code(string(r.Kind)), attrSubject: auditing.ID(r.SubjectUserID),
		attrDecision: auditing.Code(cmd.Decision), attrReason: auditing.ReasonText(cmd.Reason),
	}
	if cmd.ExtendMonths != 0 {
		due, err := domain.Extend(r, cmd.ExtendMonths, now)
		if err != nil {
			return r, err
		}
		r.DueAt, r.Progress.ExtendedMonths = due, cmd.ExtendMonths
		attrs[attrExtendMonths], attrs[attrDueAt] = auditing.Int(int64(cmd.ExtendMonths)), auditing.Time(due)
	}
	if r, err = s.decide(ctx, r, a.ID, cmd.Decision, now); err != nil {
		return r, err
	}
	if err := s.d.Store.Update(ctx, r); err != nil {
		return r, err
	}
	if err := s.appendUpdated(ctx, r); err != nil {
		return r, err
	}
	return r, s.d.Audit.Record(ctx, auditing.Entry{
		Action: string(authorize.ActionGdprRequestDecide), Decision: auditing.DecisionAllow, ResourceKind: resourceRequest,
		ResourceID: r.ID, Attrs: attrs,
	})
}

// decide applies the decision to the locked request r and inserts the job of an approval.
func (s *Service) decide(ctx context.Context, r domain.Request, by ids.ID, decision string, now time.Time) (domain.Request, error) {
	var err error
	if decision == decisionReject {
		if r.State, err = domain.Move(r.State, domain.StateRejected); err != nil {
			return r, err
		}
		r.CompletedAt = now
		return r, nil
	}
	if r.State, err = domain.Move(r.State, domain.StateApproved); err != nil {
		return r, err
	}
	r.ApprovedBy = by
	switch r.Kind {
	case domain.KindExport:
		return r, s.d.Jobs.EnqueueExport(ctx, r.ID)
	case domain.KindDeletion:
		if err := s.checkNotLastOwner(ctx, r.SubjectUserID); err != nil {
			return r, err
		}
		return r, s.d.Jobs.EnqueueErase(ctx, r.ID)
	case domain.KindRectification:
		// The owner rectified the data; the request documents it (20 5.3).
		r.State, r.CompletedAt = domain.StateCompleted, now
	}
	return r, nil
}

// Download implements privacy.Requests (downloadGdprExport, 20 section 5.1 item 5): the package
// of a completed export while privacy.export_ttl lasts, 410 gone afterwards (DP-T04).
func (s *Service) Download(ctx context.Context, a actor.Actor, id ids.ID) (privacy.Package, error) {
	ctx = actor.WithActor(ctx, a)
	var r domain.Request
	err := s.d.UoW.Do(ctx, func(ctx context.Context) error {
		var err error
		r, err = s.downloadable(ctx, a, id)
		return err
	})
	if err != nil {
		return privacy.Package{}, err
	}
	c, err := s.d.Packages.Open(ctx, r.ResultFileID)
	if e, ok := errs.As(err); ok && e.Code == errs.CodeNotFound {
		// The purge of an expired package may already have removed the file.
		return privacy.Package{}, gone()
	}
	if err != nil {
		return privacy.Package{}, err
	}
	return privacy.Package{Name: c.Name, Size: c.SizeBytes, SHA256: c.SHA256, Body: c.Body}, nil
}

// downloadable reads the request id in the unit of work of ctx, authorizes gdpr.export.download,
// checks that its package is ready and not expired, and records the audit entry.
func (s *Service) downloadable(ctx context.Context, a actor.Actor, id ids.ID) (domain.Request, error) {
	r, ok, err := s.d.Store.Get(ctx, id)
	if err != nil {
		return r, err
	}
	if !ok {
		return r, notFound()
	}
	if err := s.check(ctx, a, authorize.ActionGdprExportDownload, requestResource(r)); err != nil {
		return r, err
	}
	if r.Kind != domain.KindExport || r.State != domain.StateCompleted {
		return r, errs.New(errs.CodeConflict, "the request has no export package").WithReason("export_not_ready")
	}
	if r.ResultFileID.IsZero() || !s.now().Before(r.ResultExpiresAt) {
		return r, gone()
	}
	return r, s.d.Audit.Record(ctx, auditing.Entry{
		Action: string(authorize.ActionGdprExportDownload), Decision: auditing.DecisionAllow, ResourceKind: resourceRequest,
		ResourceID: r.ID, Attrs: auditing.Attrs{attrSubject: auditing.ID(r.SubjectUserID)},
	})
}

// gone is the 410 of an expired export package (DP-T04).
func gone() error { return errs.New(errs.CodeGone, "the export package has expired") }
