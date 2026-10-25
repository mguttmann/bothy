// SPDX-License-Identifier: Apache-2.0

package wire

import (
	"context"
	"errors"

	agentspg "github.com/OWNER/bothy/services/server/internal/agents/adapters/postgres"
	appspg "github.com/OWNER/bothy/services/server/internal/apps/adapters/postgres"
	auditpg "github.com/OWNER/bothy/services/server/internal/audit/adapters/postgres"
	"github.com/OWNER/bothy/services/server/internal/authz"
	authzpg "github.com/OWNER/bothy/services/server/internal/authz/adapters/postgres"
	authzapp "github.com/OWNER/bothy/services/server/internal/authz/app"
	billingpg "github.com/OWNER/bothy/services/server/internal/billing/adapters/postgres"
	chatpg "github.com/OWNER/bothy/services/server/internal/chat/adapters/postgres"
	importerpg "github.com/OWNER/bothy/services/server/internal/importer/adapters/postgres"
	instancepg "github.com/OWNER/bothy/services/server/internal/instance/adapters/postgres"
	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/eventlog"
	"github.com/OWNER/bothy/services/server/internal/kernel/personaldata"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
	messagingpg "github.com/OWNER/bothy/services/server/internal/messaging/adapters/postgres"
	messagingapp "github.com/OWNER/bothy/services/server/internal/messaging/app"
	notifypg "github.com/OWNER/bothy/services/server/internal/notify/adapters/postgres"
	notifyapp "github.com/OWNER/bothy/services/server/internal/notify/app"
	"github.com/OWNER/bothy/services/server/internal/platform/jobs"
	"github.com/OWNER/bothy/services/server/internal/platform/pg"
	privacypg "github.com/OWNER/bothy/services/server/internal/privacy/adapters/postgres"
	runspg "github.com/OWNER/bothy/services/server/internal/runs/adapters/postgres"
	syncpg "github.com/OWNER/bothy/services/server/internal/sync/adapters/postgres"
	syncapp "github.com/OWNER/bothy/services/server/internal/sync/app"
	terminalpg "github.com/OWNER/bothy/services/server/internal/terminal/adapters/postgres"
)

// privacyState is the part of the server state the privacy module fills.
type privacyState struct {
	// privacy is the privacy module (M7-06), built in the step privacy after the sessions step,
	// which builds the revocation use cases the participant of identity needs.
	privacy *Privacy
	// privacyCfg is its configuration; the jobs step reads retention.privacy_job from it.
	privacyCfg PrivacyConfig
}

// modulePrivacy is the name of the module in architecture/modules.yaml and of its start step.
const modulePrivacy = "privacy"

// privacyWiring wires the privacy module (M7-06): its start step after the sessions step, the REST handlers
// of the five operations, the jobs privacy.export, privacy.erase, privacy.export_expiry and
// privacy.request_reminder and the participant of retention.privacy_job.
func privacyWiring() moduleWiring {
	return moduleWiring{
		module:   modulePrivacy,
		parts:    []part{{order: 1950, step: modulePrivacy, start: (*server).startPrivacy}},
		handlers: func(s *server, h *Handlers) { s.privacy.fillHandlers(h) },
		jobs: func(s *server, m *JobModules) error {
			s.mu.Lock()
			p, cfg, pool := s.privacy, s.privacyCfg, s.pool
			s.mu.Unlock()
			if p == nil {
				return errors.New("wire: the jobs step needs the privacy step")
			}
			m.Privacy = p.Service
			return m.registerRetention(privacypg.NewRetention(pool), cfg.RetentionPrivacyJob)
		},
		bindJobs: func(s *server, ins jobs.Inserter) {
			s.mu.Lock()
			p := s.privacy
			s.mu.Unlock()
			if p != nil {
				p.Queue.Bind(ins)
			}
		},
	}
}

// startPrivacy builds the privacy module over the pool, the unit of work and recorder of audit,
// the Authorizer (and its readers of a channel), the appender of sync, the directory of identity,
// the export packages of files and the participants of every module with personal data.
func (s *server) startPrivacy(context.Context) error {
	cfg, err := PrivacyConfigFrom(s.values)
	if err != nil {
		return err
	}
	s.mu.Lock()
	pool, a, az, sy, id, f, rec := s.pool, s.audit, s.authorizer, s.sync, s.identity, s.files, s.instance
	s.mu.Unlock()
	if a == nil || az == nil || sy == nil || id == nil || f == nil || rec.InstanceID.IsZero() {
		return errors.New("wire: privacy needs audit, the Authorizer, sync, identity, files and the instance")
	}
	readers, ok := az.(authz.Recipients)
	if !ok {
		return errors.New("wire: privacy needs the Authorizer of authz (authz.Recipients)")
	}
	reg, err := s.personalDataParticipants()
	if err != nil {
		return err
	}
	pkgs, err := f.ExportPackages()
	if err != nil {
		return err
	}
	p, err := NewPrivacy(cfg, pool, PrivacyDeps{
		UoW: a.UoW(), Authorizer: az, Audit: a.Recorder(), Events: sy.Appender(), People: id.Directory,
		Packages: pkgs, Participants: reg, Readers: readers.Readers, InstanceID: rec.InstanceID,
		Clock: s.deps.Clock,
	})
	if err != nil {
		return err
	}
	s.mu.Lock()
	s.privacy, s.privacyCfg = p, cfg
	s.mu.Unlock()
	return nil
}

// personalDataParticipants returns the participants of every module with personal data of
// docs/spec/20-privacy.md section 2 in the order the steps of 20 section 4.4 call them (each step
// calls all of them before the next step begins): notify before identity in step 2, because push
// registrations hold the devices identity deletes; the content modules before identity, so
// contents go before the account is pseudonymized in step 6; identity last.
func (s *server) personalDataParticipants() (*personaldata.Registry, error) {
	s.mu.Lock()
	pool, a, sy, c, m, f, id := s.pool, s.audit, s.sync, s.chat, s.messaging, s.files, s.identity
	s.mu.Unlock()
	if a == nil || sy == nil || c == nil || m == nil || f == nil || id == nil {
		return nil, errors.New("wire: the participants need audit, sync, chat, messaging, files and identity")
	}
	list, err := personalDataList(personalDataModules{
		pool: pool, uow: a.UoW(), events: sy.Appender(), clock: s.deps.Clock, chat: c, messaging: m, files: f, identity: id,
	})
	if err != nil {
		return nil, err
	}
	if s.deps.PersonalData != nil {
		list = s.deps.PersonalData(list)
	}
	reg := personaldata.NewRegistry()
	for _, p := range list {
		if err := reg.Register(p); err != nil {
			return nil, err
		}
	}
	return reg, nil
}

// personalDataModules are the module objects the participants are built over.
type personalDataModules struct {
	pool      *pg.Pool
	uow       txn.UnitOfWork
	events    eventlog.Appender
	clock     clock.Clock
	chat      *Chat
	messaging *Messaging
	files     *Files
	identity  *Identity
}

// personalDataList builds the participants of m in the order of personalDataParticipants.
func personalDataList(m personalDataModules) ([]personaldata.Participant, error) {
	clk := m.clock
	if clk == nil {
		clk = clock.System()
	}
	notify, err := notifyapp.NewPersonalData(notifyapp.PersonalDataDeps{
		Store: notifypg.NewRepository(m.pool), Registrations: notifypg.NewRegistrations(m.pool), UoW: m.uow, Events: m.events,
	})
	if err != nil {
		return nil, err
	}
	personal, err := syncapp.NewPersonalData(syncapp.PersonalDataDeps{
		Rows: syncpg.NewPersonalRepository(m.pool), UoW: m.uow, Events: m.events, Clock: clk,
	})
	if err != nil {
		return nil, err
	}
	messages, err := messagingapp.NewPersonalData(m.messaging.Service, messagingpg.NewRepository(m.pool), m.chat.Service, m.files.Service)
	if err != nil {
		return nil, err
	}
	uploads, err := m.files.PersonalData()
	if err != nil {
		return nil, err
	}
	authzRepo := authzpg.NewRepository(m.pool)
	access, err := authzapp.NewPersonalData(authzapp.PersonalDataDeps{
		UoW: m.uow, Store: authzRepo, History: authzRepo, Structure: NewStructureSource(m.pool), Events: m.events, Clock: clk,
	})
	if err != nil {
		return nil, err
	}
	people, err := m.identity.PersonalData()
	if err != nil {
		return nil, err
	}
	return []personaldata.Participant{
		notify, personal, chatpg.NewPersonalData(m.pool), messages, uploads, access, runspg.NewPersonalData(m.pool),
		billingpg.NewPersonalData(), agentspg.NewPersonalData(m.pool), terminalpg.NewPersonalData(m.pool),
		appspg.NewPersonalData(m.pool), importerpg.NewPersonalData(), instancepg.NewPersonalData(), auditpg.NewPersonalData(m.pool),
		privacypg.NewPersonalData(m.pool), people,
	}, nil
}
