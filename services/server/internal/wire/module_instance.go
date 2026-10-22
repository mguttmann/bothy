// SPDX-License-Identifier: Apache-2.0

package wire

import (
	"context"
	"errors"
	"time"

	"github.com/OWNER/bothy/services/server/internal/instance"
)

// instanceState is the part of the server state the instance module fills.
type instanceState struct {
	// instance is the instance row the start step instance loaded (M1-15).
	instance instance.Record
	// blocked is set when a readiness check fails for good (schema_requires_newer_server).
	blocked bool
	// restorePending is set by the start lock of R-DM-50 (restore_not_finalized, M7-04): the
	// process then serves and processes nothing, see whileFinalized.
	restorePending bool
	// settings are the instance settings, branding and discovery (M1-46); identity reads
	// locale.default through them.
	settings *InstanceSettings
	// signingKeys are the server signing keys (M1-32) and keyCheck the interval of their job.
	signingKeys *SigningKeys
	keyCheck    time.Duration
}

// moduleInstance is the name of the module in architecture/modules.yaml and of its start step, where it has one.
const moduleInstance = "instance"

// instanceWiring wires the instance module: the instance row (6), the settings (6d), the signing
// keys (7, M1-32) and the job instance.key_rotation.
func instanceWiring() moduleWiring {
	return moduleWiring{
		module: moduleInstance,
		parts: []part{
			{order: 600, step: moduleInstance, start: (*server).loadInstance},
			{order: 1000, step: "settings", start: (*server).startSettings},
			{order: 1700, step: "signing_keys", start: (*server).startSigningKeys},
		},
		handlers: func(s *server, h *Handlers) {
			if h.Keys == nil && s.signingKeys != nil {
				h.Keys = s.signingKeys.Handlers
			}
			if s.settings != nil {
				s.settings.fillHandlers(h)
			}
		},
		jobs: func(s *server, m *JobModules) error {
			s.mu.Lock()
			keys, keyCheck := s.signingKeys, s.keyCheck
			s.mu.Unlock()
			if keys == nil {
				return errors.New("wire: the jobs step needs the signing keys step")
			}
			m.KeyRotation, m.KeyRotationInterval = keys.Keys, keyCheck
			return nil
		},
	}
}

// startSettings is start step 6d (M1-46): the instance settings,
// branding and discovery over the pool, the unit of work and recorder of audit, the appender of
// sync and the Authorizer. It comes before identity, because createUser reads locale.default
// through the port instance.Settings.
func (s *server) startSettings(context.Context) error {
	cfg, err := InstanceConfig(s.values)
	if err != nil {
		return err
	}
	s.mu.Lock()
	pool, a, az, sy := s.pool, s.audit, s.authorizer, s.sync
	s.mu.Unlock()
	if a == nil || az == nil || sy == nil {
		return errors.New("wire: the settings step needs the unit of work and recorder of audit, the appender of sync and the Authorizer")
	}
	st, err := NewInstanceSettings(cfg, LocalPasswordEnabled(s.values), pool, a.UoW(), az,
		SettingsWrites{Audit: a.Recorder(), Events: sy.Appender()}, s.deps.Clock)
	if err != nil {
		return err
	}
	if st.CSPReports, err = NewCSPReports(az, s.logger(), nil, int(s.values.Int(KeyCSPReportMaxRecords))); err != nil {
		return err
	}
	s.mu.Lock()
	s.settings = st
	s.mu.Unlock()
	return nil
}
