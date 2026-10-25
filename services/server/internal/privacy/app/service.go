// SPDX-License-Identifier: Apache-2.0

// Package app holds the use cases of the privacy requests (brief M7-06, docs/spec/20-privacy.md
// sections 4.4 and 5): the five operations of the port privacy.Requests, the jobs privacy.export
// and privacy.erase, the expiry of export packages and the reminder gdpr.request_due. The
// participants of all modules (kernel/personaldata) export and erase their own tables; this
// package never runs SQL on them.
package app

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"time"

	"github.com/OWNER/bothy/services/server/internal/files"
	"github.com/OWNER/bothy/services/server/internal/identity"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/auditing"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/eventlog"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/personaldata"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
	"github.com/OWNER/bothy/services/server/internal/privacy"
	"github.com/OWNER/bothy/services/server/internal/privacy/domain"
)

// ErrOpen is returned by Store.Insert when the person already has an open request of the kind
// (the partial unique index of 03 gdpr_requests).
var ErrOpen = errors.New("privacy: an open request of the same person and kind exists")

// ListFilter narrows Store.List.
type ListFilter struct {
	// Subject filters by person; zero for all.
	Subject ids.ID
	// State filters by state; empty for all.
	State domain.State
}

// Key is the keyset position of a list: created_at descending, then id descending.
type Key struct {
	CreatedAt time.Time
	ID        ids.ID
}

// Store reads and writes gdpr_requests; every method joins the ambient unit of work of ctx.
type Store interface {
	// Insert adds r. Errors: ErrOpen for a second open request of the person and kind.
	Insert(ctx context.Context, r domain.Request) error
	// OpenOf returns the open request (requested, approved, running) of subject and kind.
	OpenOf(ctx context.Context, subject ids.ID, kind domain.Kind) (domain.Request, bool, error)
	// Get returns the request id.
	Get(ctx context.Context, id ids.ID) (domain.Request, bool, error)
	// Lock returns the request id locked FOR UPDATE.
	Lock(ctx context.Context, id ids.ID) (domain.Request, bool, error)
	// Update writes every changeable column of r (state, approved_by, due_at, result_file_id,
	// result_expires_at, progress, error, completed_at).
	Update(ctx context.Context, r domain.Request) error
	// List returns at most limit requests of f after the position after (nil for the start),
	// newest first.
	List(ctx context.Context, f ListFilter, after *Key, limit int) ([]domain.Request, error)
	// ReminderCandidates returns at most limit open requests with due_at at or before until
	// whose progress has no reminder yet, earliest deadline first.
	ReminderCandidates(ctx context.Context, until time.Time, limit int) ([]domain.Request, error)
	// ExpiredPackages returns at most limit completed exports whose package is still stored
	// and whose result_expires_at is at or before now.
	ExpiredPackages(ctx context.Context, now time.Time, limit int) ([]domain.Request, error)
}

// Jobs inserts the jobs of a request inside the unit of work of ctx (02 12.3), so a job exists
// exactly when the approval commits.
type Jobs interface {
	// EnqueueExport inserts privacy.export for the request id.
	EnqueueExport(ctx context.Context, id ids.ID) error
	// EnqueueErase inserts privacy.erase for the request id.
	EnqueueErase(ctx context.Context, id ids.ID) error
}

// Built is a finished export archive in temporary storage.
type Built struct {
	// Size and SHA256 describe the archive.
	Size   int64
	SHA256 string
	// Open opens the archive for reading; it may be called more than once.
	Open func() (io.ReadCloser, error)
}

// Archive is one export package under construction (20 section 5.2): the participants write
// into it through personaldata.Writer, Finish adds README.txt and manifest.json last and zips.
type Archive interface {
	personaldata.Writer
	// Finish writes README.txt with readme, then manifest.json from m with the path, size and
	// SHA-256 of every other file filled in, and builds the ZIP archive.
	Finish(readme string, m domain.Manifest) (Built, error)
	// Discard removes the temporary files; it is safe to call after Finish and more than once.
	Discard()
}

// Archives creates archives.
type Archives interface {
	// New starts an empty archive. Errors: the temporary storage.
	New() (Archive, error)
}

// Cursors is the codec of the opaque list cursors (platform/httpserver).
type Cursors interface {
	// Encode returns the cursor of key for list and binding.
	Encode(list string, binding []byte, key any) (string, error)
	// Decode checks s against list and binding and fills key. Errors: cursor_invalid.
	Decode(list string, binding []byte, s string, key any) error
}

// Config holds the keys the use cases read (20 section 13, 06). Code and docs name the keys,
// never the values.
type Config struct {
	// ExportTTL is privacy.export_ttl: how long a package is available.
	ExportTTL time.Duration
	// ExportRequiresApproval is privacy.export_requires_approval.
	ExportRequiresApproval bool
	// DeletionContent is privacy.account_deletion_content.
	DeletionContent personaldata.ContentMode
	// ReminderLead is privacy.request_reminder_lead: how long before due_at the owners get
	// gdpr.request_due.
	ReminderLead time.Duration
	// PageSizeDefault and PageSizeMax are api.page_size_default and api.page_size_max.
	PageSizeDefault, PageSizeMax int
}

// Deps are the collaborators of the use cases.
type Deps struct {
	Store      Store
	UoW        txn.UnitOfWork
	Authorizer authorize.Authorizer
	Audit      auditing.Recorder
	Events     eventlog.Appender
	// People reads the person and the active owners.
	People identity.Directory
	// Packages stores, opens and deletes export packages (module files).
	Packages files.ExportPackages
	// Participants are the participants of every module, in the order of their registration.
	Participants *personaldata.Registry
	// InstanceID returns instance.id for manifest.json.
	InstanceID func(ctx context.Context) (ids.ID, error)
	Archives   Archives
	Jobs       Jobs
	Cursors    Cursors
	// Clock nil means clock.System().
	Clock clock.Clock
}

// Service implements privacy.Requests and the jobs of the module. It is safe for concurrent use.
type Service struct {
	cfg Config
	d   Deps
}

// Compile-time check: Service is the inbound port of privacy.
var _ privacy.Requests = (*Service)(nil)

// New returns the use cases over d with cfg. Errors: a missing collaborator or a value of cfg
// that is not allowed.
func New(cfg Config, d Deps) (*Service, error) {
	if d.Store == nil || d.UoW == nil || d.Authorizer == nil || d.Audit == nil || d.Events == nil || d.People == nil ||
		d.Packages == nil || d.Participants == nil || d.InstanceID == nil || d.Archives == nil || d.Jobs == nil || d.Cursors == nil {
		return nil, errors.New("privacy: the use cases need every collaborator")
	}
	if cfg.ExportTTL <= 0 || cfg.ReminderLead <= 0 || !cfg.DeletionContent.Valid() || cfg.PageSizeDefault < 1 ||
		cfg.PageSizeMax < cfg.PageSizeDefault {
		return nil, errors.New("privacy: privacy.export_ttl, privacy.request_reminder_lead, privacy.account_deletion_content or the page sizes are invalid")
	}
	if d.Clock == nil {
		d.Clock = clock.System()
	}
	return &Service{cfg: cfg, d: d}, nil
}

// The names of events, audit attributes and resources of the module (05, 20).
const (
	eventUpdated      = "gdpr.request_updated"
	eventDue          = "gdpr.request_due"
	receiveRead       = "gdpr.request.read"
	receiveSelf       = "self"
	resourceRequest   = "gdpr_request"
	resourcePrincipal = "principal"
	resourceUser      = "user"
	attrKind          = "kind"
	attrRequestID     = "request_id"
	attrReason        = "reason"
	attrDecision      = "decision"
	attrExtendMonths  = "extend_months"
	attrDueAt         = "due_at"
	attrContent       = "content"
	attrSubject       = "subject_user_id"
	attrAutoApproved  = "auto_approved"
	// actionErase is the audit code of step 7 of the account deletion (20 4.4): a system step
	// without authorization, like the other audit codes of system steps (D-M7-04-2).
	actionErase = "privacy.erase"
	// codeLastOwner and codeFailed are the error codes a failed request records (no text).
	codeLastOwner = "last_owner"
	codeFailed    = "job_failed"
)

// now is the clock at the precision of PostgreSQL, so answers equal stored values.
func (s *Service) now() time.Time { return s.d.Clock.Now().UTC().Truncate(time.Microsecond) }

// check authorizes act on r for a.
func (s *Service) check(ctx context.Context, a actor.Actor, act authorize.Action, r authorize.Resource) error {
	d, err := s.d.Authorizer.Authorize(ctx, a, act, r)
	if err != nil {
		return err
	}
	return d.Err()
}

// requestResource is the resource of the request r for the actions on it: the person owns it.
func requestResource(r domain.Request) authorize.Resource {
	return authorize.Resource{Kind: authorize.ResourceGdprRequest, ID: r.ID, OwnerID: r.SubjectUserID}
}

// notFound is the 404 of an unknown or invisible request.
func notFound() error { return errs.New(errs.CodeNotFound, "") }

// view converts a request into the port type.
func view(r domain.Request) privacy.Request {
	out := privacy.Request{
		ID: r.ID, SubjectUserID: r.SubjectUserID, Kind: string(r.Kind), State: string(r.State), RequestedBy: r.RequestedBy,
		ApprovedBy: r.ApprovedBy, Reason: r.Reason, DueAt: r.DueAt, Error: r.Error, CreatedAt: r.CreatedAt,
		CompletedAt: r.CompletedAt, Progress: progressMap(r.Progress),
	}
	out.ResultExpiresAt = r.ResultExpiresAt
	return out
}

// progressMap returns the progress as a JSON object of the API.
func progressMap(p domain.Progress) map[string]any {
	b, err := domain.MarshalProgress(p)
	if err != nil {
		return map[string]any{}
	}
	out := map[string]any{}
	if err := json.Unmarshal(b, &out); err != nil {
		return map[string]any{}
	}
	return out
}

// appendUpdated appends gdpr.request_updated with the request (04 2.2) by the actor of ctx, or
// the system outside a request.
func (s *Service) appendUpdated(ctx context.Context, r domain.Request) error {
	payload, err := json.Marshal(map[string]any{"request": requestJSON(view(r))})
	if err != nil {
		return errs.Wrap(errs.CodeInternalError, err, "privacy: gdpr.request_updated payload")
	}
	ev := eventlog.Event{
		Type: eventUpdated, SchemaVersion: 1, ResourceKind: resourceRequest, ResourceID: r.ID, ReceiveAction: receiveRead,
		Payload: payload,
	}
	if a, ok := actor.FromContext(ctx); ok {
		ev.ActorKind, ev.ActorPrincipalID = eventlog.ActorFor(string(a.Kind), a.ID)
	} else {
		ev.ActorKind = eventlog.ActorSystem
	}
	return s.d.Events.Append(ctx, ev)
}

// requestJSON is the schema GdprRequest of a request: optional members only when set.
func requestJSON(r privacy.Request) map[string]any {
	out := map[string]any{
		"id": r.ID.String(), "subject_user_id": r.SubjectUserID.String(), "kind": r.Kind, "state": r.State,
		"requested_by": r.RequestedBy.String(), "reason": r.Reason, "due_at": stamp(r.DueAt), "created_at": stamp(r.CreatedAt),
		"progress": r.Progress,
	}
	if !r.ApprovedBy.IsZero() {
		out["approved_by"] = r.ApprovedBy.String()
	}
	if !r.ResultExpiresAt.IsZero() {
		out["result_expires_at"] = stamp(r.ResultExpiresAt)
	}
	if r.Error != "" {
		out["error"] = r.Error
	}
	if !r.CompletedAt.IsZero() {
		out["completed_at"] = stamp(r.CompletedAt)
	}
	return out
}

// stamp is the time form of the API: UTC with milliseconds.
func stamp(t time.Time) string { return t.UTC().Format("2006-01-02T15:04:05.000Z07:00") }
