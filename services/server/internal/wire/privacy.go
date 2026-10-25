// SPDX-License-Identifier: Apache-2.0

package wire

import (
	"context"
	"time"

	"github.com/OWNER/bothy/services/server/internal/files"
	"github.com/OWNER/bothy/services/server/internal/identity"
	"github.com/OWNER/bothy/services/server/internal/kernel/auditing"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/eventlog"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/personaldata"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
	"github.com/OWNER/bothy/services/server/internal/platform/config"
	"github.com/OWNER/bothy/services/server/internal/platform/pg"
	"github.com/OWNER/bothy/services/server/internal/privacy/adapters/archive"
	privacyhttp "github.com/OWNER/bothy/services/server/internal/privacy/adapters/httpapi"
	privacyjobs "github.com/OWNER/bothy/services/server/internal/privacy/adapters/jobs"
	privacypg "github.com/OWNER/bothy/services/server/internal/privacy/adapters/postgres"
	privacyapp "github.com/OWNER/bothy/services/server/internal/privacy/app"
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
	return &Privacy{Service: svc, Handlers: privacyhttp.New(svc), Queue: q, Retention: privacypg.NewRetention(pool)}, nil
}

// fillHandlers sets the nil privacy handlers of h; a nil p leaves h unchanged.
func (p *Privacy) fillHandlers(h *Handlers) {
	if p == nil {
		return
	}
	fill(&h.Privacy, p.Handlers)
}
