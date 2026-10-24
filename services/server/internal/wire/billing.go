// SPDX-License-Identifier: Apache-2.0

package wire

import (
	"errors"

	billinghttp "github.com/OWNER/bothy/services/server/internal/billing/adapters/httpapi"
	billingpg "github.com/OWNER/bothy/services/server/internal/billing/adapters/postgres"
	billingapp "github.com/OWNER/bothy/services/server/internal/billing/app"
	"github.com/OWNER/bothy/services/server/internal/kernel/auditing"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
	"github.com/OWNER/bothy/services/server/internal/platform/config"
	"github.com/OWNER/bothy/services/server/internal/platform/pg"
)

// BillingConfig binds the keys of the billing use cases (billing.show_personal_estimates;
// M2-22). The error names the key whose binding failed, never a value.
func BillingConfig(v *config.Values) (billingapp.Config, error) {
	var c billingapp.Config
	if err := config.Bind(v, &c); err != nil {
		return billingapp.Config{}, err
	}
	return c, nil
}

// Billing is the billing module of the process (M2-22): the booking of run costs, which runs
// calls in the unit of work of run.finish, the thread costs and the price table with their REST
// handlers.
type Billing struct {
	// Service implements billing.Costs and billing.Prices.
	Service *billingapp.Service
	// Handlers are the REST handlers.
	Handlers *billinghttp.Billing
}

// NewBilling builds the billing module over pool with the unit of work uow, the Authorizer az,
// the audit recorder rec and the clock (nil means clock.System()). It opens no connection.
// Errors: a missing collaborator.
func NewBilling(cfg billingapp.Config, pool *pg.Pool, uow txn.UnitOfWork, az authorize.Authorizer, rec auditing.Recorder,
	clk clock.Clock) (*Billing, error) {
	if pool == nil {
		return nil, errors.New("wire: the billing module needs the pool")
	}
	svc, err := billingapp.New(cfg, billingapp.Deps{Repo: billingpg.NewRepository(pool), UoW: uow, Authorizer: az, Audit: rec, Clock: clk})
	if err != nil {
		return nil, err
	}
	return &Billing{Service: svc, Handlers: billinghttp.New(svc, svc)}, nil
}

// fillHandlers sets the nil billing handler of h to the one b built; a nil b leaves h unchanged.
func (b *Billing) fillHandlers(h *Handlers) {
	if b != nil && h.Billing == nil {
		h.Billing = b.Handlers
	}
}
