// SPDX-License-Identifier: Apache-2.0

package wire

import (
	"context"
	"time"

	"github.com/OWNER/bothy/services/server/internal/files"
	"github.com/OWNER/bothy/services/server/internal/identity"
	"github.com/OWNER/bothy/services/server/internal/instance"
	instanceapp "github.com/OWNER/bothy/services/server/internal/instance/app"
	"github.com/OWNER/bothy/services/server/internal/kernel/auditing"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/eventlog"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/personaldata"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
	"github.com/OWNER/bothy/services/server/internal/messaging"
	"github.com/OWNER/bothy/services/server/internal/platform/config"
	"github.com/OWNER/bothy/services/server/internal/platform/pg"
	"github.com/OWNER/bothy/services/server/internal/privacy/adapters/archive"
	privacyhttp "github.com/OWNER/bothy/services/server/internal/privacy/adapters/httpapi"
	privacyjobs "github.com/OWNER/bothy/services/server/internal/privacy/adapters/jobs"
	privacypg "github.com/OWNER/bothy/services/server/internal/privacy/adapters/postgres"
	privacyapp "github.com/OWNER/bothy/services/server/internal/privacy/app"
	"github.com/OWNER/bothy/services/server/internal/status"
	statusdomain "github.com/OWNER/bothy/services/server/internal/status/domain"
	"github.com/OWNER/bothy/services/server/internal/sync"
)

// PrivacyConfig holds the keys of the privacy requests (docs/spec/20-privacy.md section 13,
// M7-06). Code and docs name the keys, never the values.
type PrivacyConfig struct {
	ExportTTL              time.Duration `config:"privacy.export_ttl"`
	ExportRequiresApproval bool          `config:"privacy.export_requires_approval"`
	DeletionContent        string        `config:"privacy.account_deletion_content"`
	ReminderLead           time.Duration `config:"privacy.request_reminder_lead"`
	// RetentionPrivacyJob is retention.privacy_job, the period of the job retention.privacy_job.
	RetentionPrivacyJob time.Duration `config:"retention.privacy_job"`
	// DeletionJournalRetention is privacy.deletion_journal_retention, the period of the journal
	// objects in the same job (M7-07).
	DeletionJournalRetention time.Duration `config:"privacy.deletion_journal_retention"`
	// BackupLogical, BackupObjects, BackupFull and BackupLocalFull are the retention rules of the
	// backups the journal retention keeps objects for (16 8.1).
	BackupLogical   time.Duration `config:"backup.logical.retention"`
	BackupObjects   time.Duration `config:"backup.objects.retention"`
	BackupFull      int           `config:"backup.pg.retention_full"`
	BackupLocalFull int           `config:"backup.pg.local_retention_full"`
	// PageSizeDefault and PageSizeMax are api.page_size_default and api.page_size_max.
	PageSizeDefault int `config:"api.page_size_default"`
	PageSizeMax     int `config:"api.page_size_max"`
}

// PrivacyConfigFrom binds PrivacyConfig from v; the error names the key, never a value.
func PrivacyConfigFrom(v *config.Values) (PrivacyConfig, error) {
	var c PrivacyConfig
	if err := config.Bind(v, &c); err != nil {
		return PrivacyConfig{}, err
	}
	return c, nil
}

// appConfig returns the configuration of the use cases.
func (c PrivacyConfig) appConfig() privacyapp.Config {
	return privacyapp.Config{
		ExportTTL: c.ExportTTL, ExportRequiresApproval: c.ExportRequiresApproval,
		DeletionContent: personaldata.ContentMode(c.DeletionContent), ReminderLead: c.ReminderLead,
		PageSizeDefault: c.PageSizeDefault, PageSizeMax: c.PageSizeMax,
	}
}

// PrivacyDeps are the collaborators of the privacy module from other steps.
type PrivacyDeps struct {
	UoW        txn.UnitOfWork
	Authorizer authorize.Authorizer
	Audit      auditing.Recorder
	Events     eventlog.Appender
	// People is the directory of identity.
	People identity.Directory
	// Packages stores the export packages (module files).
	Packages files.ExportPackages
	// Participants are the participants of every module with personal data.
	Participants *personaldata.Registry
	// JournalObjects and JournalFiles are the ports of files for the deletion journal (M7-07).
	JournalObjects files.JournalObjects
	JournalFiles   files.JournalFiles
	// JournalMessages deletes a message again on a restore (messaging.JournalMessages).
	JournalMessages messaging.JournalMessages
	// KeptBackups names the oldest journal object a kept backup needs (module status).
	KeptBackups status.KeptBackups
	// InstanceID is instance.id for manifest.json.
	InstanceID ids.ID
	// TempDir is the parent of the temporary package directories; "" means os.TempDir().
	TempDir string
	// Clock nil means clock.System().
	Clock clock.Clock
}

// Privacy is the privacy module of a process (M7-06): the use cases of privacy.Requests and of
// its jobs, the REST handlers, the job queue and the participant of retention.privacy_job.
type Privacy struct {
	// Service implements privacy.Requests and the jobs.
	Service *privacyapp.Service
	// Handlers are the REST handlers of the five operations.
	Handlers *privacyhttp.Privacy
	// Queue inserts privacy.export and privacy.erase; BindJobs gives it the inserter.
	Queue *privacyjobs.Queue
	// Retention is the participant of retention.privacy_job.
	Retention *privacypg.Retention
	// Journal is the deletion journal (M7-07): the consumer privacy.journal, ApplyJournal of
	// bothy restore finalize and the participant of the journal objects.
	Journal *privacyapp.Journal
}

// NewPrivacy builds the privacy module over pool with cfg and d. It opens no connection and
// starts nothing. Errors: a missing collaborator or a value the use cases refuse.
func NewPrivacy(cfg PrivacyConfig, pool *pg.Pool, d PrivacyDeps) (*Privacy, error) {
	q := privacyjobs.NewQueue()
	svc, err := privacyapp.New(cfg.appConfig(), privacyapp.Deps{
		Store: privacypg.NewRequests(pool), UoW: d.UoW, Authorizer: d.Authorizer, Audit: d.Audit, Events: d.Events, People: d.People,
		Packages: d.Packages, Participants: d.Participants,
		InstanceID: func(context.Context) (ids.ID, error) { return d.InstanceID, nil },
		Archives:   archive.New(d.TempDir), Jobs: q, Cursors: privacyhttp.Cursors{}, Clock: d.Clock,
	})
	if err != nil {
		return nil, err
	}
	requests := privacypg.NewRequests(pool)
	j, err := privacyapp.NewJournal(privacyapp.JournalDeps{
		State: privacypg.NewJournalState(pool), Requests: requests, Store: requests, Objects: d.JournalObjects,
		Messages: d.JournalMessages, Files: d.JournalFiles, People: d.People, Eraser: svc, Floor: d.KeptBackups,
		DeletionContent: personaldata.ContentMode(cfg.DeletionContent), Clock: d.Clock,
	})
	if err != nil {
		return nil, err
	}
	return &Privacy{Service: svc, Handlers: privacyhttp.New(svc), Queue: q, Retention: privacypg.NewRetention(pool), Journal: j}, nil
}

// keep returns the retention rules of the backups for the journal floor (status.KeptBackups).
func (c PrivacyConfig) keep() statusdomain.Keep {
	return statusdomain.Keep{Logical: c.BackupLogical, Objects: c.BackupObjects, FullCount: max(c.BackupFull, c.BackupLocalFull)}
}

// journalConsumer adapts the deletion journal to the worker consumer privacy.journal
// (sync.ConsumerHandler): privacy does not depend on sync, so the event is mapped here.
type journalConsumer struct {
	j *privacyapp.Journal
}

// HandleEvent implements sync.ConsumerHandler.
func (c journalConsumer) HandleEvent(ctx context.Context, ev sync.Event) error {
	return c.j.HandleEvent(ctx, privacyapp.JournalEvent{
		Seq: ev.Seq, Type: ev.Type, OccurredAt: ev.OccurredAt, ResourceID: ev.ResourceID, Payload: ev.Payload,
	})
}

// Consumer returns the handler of the worker consumer privacy.journal; nil for a nil p.
func (p *Privacy) Consumer() sync.ConsumerHandler {
	if p == nil {
		return nil
	}
	return journalConsumer{j: p.Journal}
}

// RestoreJournal adapts the deletion journal to the port of bothy restore finalize
// (instanceapp.DeletionJournal).
type RestoreJournal struct {
	j *privacyapp.Journal
}

// RestoreJournal returns the port of bothy restore finalize over the deletion journal of p.
func (p *Privacy) RestoreJournal() RestoreJournal { return RestoreJournal{j: p.Journal} }

// Compile-time check.
var _ instanceapp.DeletionJournal = RestoreJournal{}

// Apply implements instanceapp.DeletionJournal.
func (r RestoreJournal) Apply(ctx context.Context) (instance.JournalApplied, error) {
	res, err := r.j.ApplyJournal(ctx)
	if err != nil {
		return instance.JournalApplied{}, err
	}
	return instance.JournalApplied{
		Entries: res.Entries, Messages: res.Messages, Files: res.Files, Accounts: res.Accounts, Skipped: res.Skipped,
		Refused: res.Refused, Next: res.Next,
	}, nil
}

// StoreNext implements instanceapp.DeletionJournal.
func (r RestoreJournal) StoreNext(ctx context.Context, next int64) error { return r.j.StoreNext(ctx, next) }

// fillHandlers sets the nil privacy handlers of h; a nil p leaves h unchanged.
func (p *Privacy) fillHandlers(h *Handlers) {
	if p == nil {
		return
	}
	fill(&h.Privacy, p.Handlers)
}
