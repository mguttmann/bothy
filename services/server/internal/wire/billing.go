// SPDX-License-Identifier: Apache-2.0

package wire

import (
	"errors"
	"log/slog"

	billinghttp "github.com/OWNER/bothy/services/server/internal/billing/adapters/httpapi"
	billingjobs "github.com/OWNER/bothy/services/server/internal/billing/adapters/jobs"
	billingpg "github.com/OWNER/bothy/services/server/internal/billing/adapters/postgres"
	billingapp "github.com/OWNER/bothy/services/server/internal/billing/app"
	"github.com/OWNER/bothy/services/server/internal/kernel/auditing"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
	"github.com/OWNER/bothy/services/server/internal/platform/config"
	"github.com/OWNER/bothy/services/server/internal/platform/jobs"
	"github.com/OWNER/bothy/services/server/internal/platform/pg"
)

// BillingConfig binds the keys of the billing use cases (billing.show_personal_estimates, M2-22;
// api.page_size_default and api.page_size_max of listCostEstimates, M2-28). The error names the key
// whose binding failed, never a value.
func BillingConfig(v *config.Values) (billingapp.Config, error) {
	var c billingapp.Config
	if err := config.Bind(v, &c); err != nil {
		return billingapp.Config{}, err
	}
	return c, nil
}

// Billing is the billing module of the process (M2-22): the booking of run costs, which runs
// calls in the unit of work of run.finish, the thread costs, the price table and the cost
// reconciliation (M2-28) with their REST handlers and the job billing.reconcile.
type Billing struct {
	// Service implements billing.Costs and billing.Prices and the use case of billing.reconcile.
	Service *billingapp.Service
	// Handlers are the REST handlers.
	Handlers *billinghttp.Billing
	// Scheduler inserts billing.reconcile; BindJobs gives it the job runtime.
	Scheduler *billingjobs.Scheduler
}

// NewBilling builds the billing module over pool with the unit of work uow, the Authorizer az,
// the audit recorder rec, the clock (nil means clock.System()) and the logger of the job (nil
// discards). The scheduler of billing.reconcile is bound later (BindJobs). It opens no connection.
// Errors: a missing collaborator.
func NewBilling(cfg billingapp.Config, pool *pg.Pool, uow txn.UnitOfWork, az authorize.Authorizer, rec auditing.Recorder,
	clk clock.Clock, log *slog.Logger) (*Billing, error) {
	if pool == nil {
		return nil, errors.New("wire: the billing module needs the pool")
	}
	sched := &billingjobs.Scheduler{}
	svc, err := billingapp.New(cfg, billingapp.Deps{Repo: billingpg.NewRepository(pool), UoW: uow, Authorizer: az, Audit: rec, Clock: clk,
		Scheduler: sched, Cursors: billinghttp.Cursors{}, Logger: log})
	if err != nil {
		return nil, err
	}
	return &Billing{Service: svc, Handlers: billinghttp.New(svc, svc), Scheduler: sched}, nil
}

// BindJobs binds the inserter of the job runtime of the role to the scheduler of billing.reconcile.
func (b *Billing) BindJobs(ins jobs.Inserter) { b.Scheduler.Bind(ins) }

// fillHandlers sets the nil billing handler of h to the one b built; a nil b leaves h unchanged.
func (b *Billing) fillHandlers(h *Handlers) {
	if b != nil && h.Billing == nil {
		h.Billing = b.Handlers
	}
}
