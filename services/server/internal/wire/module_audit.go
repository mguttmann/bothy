// SPDX-License-Identifier: Apache-2.0

package wire

import (
	"context"
	"errors"
)

// auditState is the part of the server state the audit module fills.
type auditState struct {
	// audit is the audit module (M1-24); later tickets hand its recorder to the modules.
	audit *Audit
}

// moduleAudit is the name of the module in architecture/modules.yaml and of its start step, where it has one.
const moduleAudit = "audit"

// auditWiring wires the audit module: its start step (6a), the audit log after the signing keys
// (M1-50) and the job audit.chain_verify.
func auditWiring() moduleWiring {
	return moduleWiring{
		module: moduleAudit,
		parts: []part{
			{order: 700, step: moduleAudit, start: (*server).startAudit},
			{order: 1800, step: "audit_log", start: (*server).startAuditLog},
		},
		handlers: func(s *server, h *Handlers) {
			if h.Log == nil && s.audit != nil {
				h.Log = s.audit.Handlers()
			}
		},
		jobs: func(s *server, m *JobModules) error {
			s.mu.Lock()
			a := s.audit
			s.mu.Unlock()
			if a.Log() == nil {
				return errors.New("wire: the jobs step needs the audit log step")
			}
			m.ChainVerify, m.ChainVerifyInterval = a.Log(), a.cfg.CheckpointInterval
			return nil
		},
	}
}

// startAudit is start step 6a: it checks the key-encryption key, builds the audit module over the
// pool of the role and starts its aggregator with the stop step that flushes it (Audit.Start;
// the pool step is registered earlier, so the flush runs before the pool closes).
func (s *server) startAudit(ctx context.Context) error {
	cfg, err := AuditConfig(s.values)
	if err != nil {
		return err
	}
	unitCfg, err := UnitConfig(s.values)
	if err != nil {
		return err
	}
	kek := SecretsKEK{Provider: s.sp}
	if err := checkAuditKEK(ctx, kek, cfg.IPKEKRef); err != nil {
		return err
	}
	s.mu.Lock()
	pool := s.pool
	s.mu.Unlock()
	a, err := NewAudit(pool, unitCfg, cfg, kek, s.logger())
	if err != nil {
		return err
	}
	a.Start(s.runner)
	s.mu.Lock()
	s.audit = a
	s.mu.Unlock()
	return nil
}

// startAuditLog is the second extension point of start step 7 (M1-50): the query, export and
// verification of the audit log and the job audit.chain_verify over the signing keys of the
// previous step, the Authorizer and the instance ID. The sink of audit.checkpoint_sink is built
// here; an object store target without an effective retention lock stops the start (M7-22).
// Processes that run jobs write the warning audit_checkpoint_sink_not_configured once here when
// no sink is configured (19 SEC-AUD-2).
func (s *server) startAuditLog(ctx context.Context) error {
	s.mu.Lock()
	a, keys, az, rec := s.audit, s.signingKeys, s.authorizer, s.instance
	s.mu.Unlock()
	if a == nil || keys == nil || az == nil {
		return errors.New("wire: the audit log needs the audit step, the signing keys and the Authorizer")
	}
	sink, err := CheckpointSink(ctx, s.values, rec.InstanceID, CheckpointSinkDeps{Secrets: s.sp, Egress: s.egressDeps()})
	if err != nil {
		return err
	}
	if err := a.BindLog(keys.Keys, az, rec.InstanceID, s.deps.Clock, sink); err != nil {
		return err
	}
	if s.worker() {
		a.WarnWithoutSink(ctx)
	}
	return nil
}
