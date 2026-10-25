// SPDX-License-Identifier: Apache-2.0

package wire

import (
	"context"
	"errors"

	billingpg "github.com/OWNER/bothy/services/server/internal/billing/adapters/postgres"
	"github.com/OWNER/bothy/services/server/internal/platform/jobs"
)

// billingState is the part of the server state the billing module fills.
type billingState struct {
	// billing is the billing module (M2-22), built in the chat step before runs, which books the
	// costs of a finish through it.
	billing *Billing
}

// moduleBilling is the name of the module in architecture/modules.yaml.
const moduleBilling = "billing"

// billingWiring wires the billing module into the start step chat, before runs, mounts the REST
// handlers of getThreadCosts, listPrices, createPrice, closePrice, reconcileCosts,
// submitCostReconciliation and listCostEstimates, and wires the job billing.reconcile (M2-28) and
// the inserter of its scheduler.
func billingWiring() moduleWiring {
	return moduleWiring{
		module:   moduleBilling,
		parts:    []part{{order: 1315, step: moduleChat, start: (*server).startBilling}},
		handlers: func(s *server, h *Handlers) { s.billing.fillHandlers(h) },
		jobs: func(s *server, m *JobModules) error {
			s.mu.Lock()
			bl := s.billing
			s.mu.Unlock()
			if bl == nil {
				return errors.New("wire: the jobs step needs the billing module")
			}
			m.Reconciliation = bl.Service
			// The job retention.cost deletes expired cost entries and closed prices (M7-05).
			return m.registerRetention(billingpg.NewRetention(s.currentPool()), m.RetentionConfig.Cost)
		},
		bindJobs: func(s *server, ins jobs.Inserter) {
			s.mu.Lock()
			bl := s.billing
			s.mu.Unlock()
			bl.BindJobs(ins)
		},
	}
}

// startBilling builds the billing module (M2-22) over the pool, the unit of work and the recorder
// of audit and the Authorizer.
func (s *server) startBilling(context.Context) error {
	cfg, err := BillingConfig(s.values)
	if err != nil {
		return err
	}
	s.mu.Lock()
	pool, a, az := s.pool, s.audit, s.authorizer
	s.mu.Unlock()
	if a == nil || az == nil {
		return errors.New("wire: the billing module needs audit and the Authorizer")
	}
	bl, err := NewBilling(cfg, pool, a.UoW(), az, a.Recorder(), s.deps.Clock, s.logger())
	if err != nil {
		return err
	}
	s.mu.Lock()
	s.billing = bl
	s.mu.Unlock()
	return nil
}
