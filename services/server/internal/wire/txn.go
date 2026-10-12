// SPDX-License-Identifier: Apache-2.0

package wire

import (
	"cmp"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
	"github.com/OWNER/bothy/services/server/internal/platform/config"
	"github.com/OWNER/bothy/services/server/internal/platform/otel"
	"github.com/OWNER/bothy/services/server/internal/platform/pg"
)

// Modules holds the module objects of a process role whose hooks and subscribers the txn
// registry takes. It is empty until the first module with a hook or subscriber exists (authz
// M1-29, sync M1-16, audit M1-24); those tickets add their fields. Constructors that fill it
// must not open network connections, so the golden test builds the real registration without a
// database.
type Modules struct{}

// NewTxnRegistry returns the sealed txn registry of a process role: every role (api, worker,
// all) builds its registry here, so the golden file shows the one list of all roles. The error
// is the first rejected registration.
//
// Example:
//
//	reg, err := wire.NewTxnRegistry(&wire.Modules{})
//	if err != nil {
//		return err
//	}
//	uow := pg.NewUnitOfWork(pool, unitCfg, reg)
func NewTxnRegistry(m *Modules) (*txn.Registry, error) {
	reg := txn.NewRegistry()
	if err := registerTxn(reg, m); err != nil {
		return nil, err
	}
	reg.Seal()
	return reg, nil
}

// registerTxn is the only place with SetHook and Subscribe calls (02 "In-Tx events": one
// list). Hooks and handlers are method values of the module objects in m, never closures here,
// because the registry derives their owner from the Go package. Today no module has a hook or a
// subscriber; only the tracer of subscriber spans is set.
//
//nolint:unparam // the module tickets return the errors of SetHook and Subscribe from here.
func registerTxn(reg *txn.Registry, _ *Modules) error {
	reg.SetTracer(otel.SubscriberSpan)
	return nil
}

// UnitConfig binds the configuration of the unit of work (database.tx_retry_max and the three
// transaction deadlines) from v. The error names the key whose binding failed, never a value.
func UnitConfig(v *config.Values) (pg.UnitConfig, error) {
	var c pg.UnitConfig
	if err := config.Bind(v, &c); err != nil {
		return pg.UnitConfig{}, err
	}
	return c, nil
}

// txnGoldenHeader is the first line of testdata/txn_registrations.golden.
const txnGoldenHeader = "# txn registrations; regenerate with: go test ./internal/wire -run TestTxnRegistrationsGolden -update"

// TxnRegistrations renders the registrations of reg for the golden file: the header line, then
// one line per registration, hooks before subscriptions, hooks in phase order, subscriptions
// in registration order:
//
//	hook <phase> <module> <handler>
//	subscribe <publisher>.<Event> -> <subscriber> <handler>
func TxnRegistrations(reg *txn.Registry) string {
	return renderTxnRegistrations(reg.Registrations())
}

// renderTxnRegistrations renders regs as TxnRegistrations describes.
func renderTxnRegistrations(regs []txn.Registration) string {
	sorted := slices.Clone(regs)
	slices.SortStableFunc(sorted, func(a, b txn.Registration) int {
		return cmp.Or(cmp.Compare(a.Kind, b.Kind), cmp.Compare(a.Phase, b.Phase))
	})
	var b strings.Builder
	b.WriteString(txnGoldenHeader + "\n")
	for _, r := range sorted {
		if r.Kind == txn.KindHook {
			fmt.Fprintf(&b, "hook %s %s %s\n", r.Phase, r.Subscriber, r.Handler)
			continue
		}
		fmt.Fprintf(&b, "%s %s -> %s %s\n", r.Kind, r.Event, r.Subscriber, r.Handler)
	}
	return b.String()
}

// ModuleDeps maps every module of architecture/modules.yaml to its depends_on list.
type ModuleDeps map[string][]string

// hookOwners names the module that owns the hook of each phase (02 "Flow").
var hookOwners = map[txn.Phase]string{
	txn.PhaseRightsBegin: "authz", txn.PhaseRightsCheck: "authz", txn.PhaseSync: "sync", txn.PhaseAudit: "audit",
}

// kernelPublisher is the publisher of kernel events (authorize.AccessChanged).
const kernelPublisher = "kernel"

// CheckTxnDirections checks regs against the module register deps (02 "Other checks in
// the gate", R-AR-04): a subscription is allowed when its publisher is kernel or is in depends_on of
// the subscriber, that is a higher module subscribes to a lower one; a hook belongs to the module
// of its phase (authz for both rights phases, sync, audit). The error lists every violation
// and names the modules involved.
func CheckTxnDirections(regs []txn.Registration, deps ModuleDeps) error {
	var problems []error
	for _, r := range regs {
		if err := checkRegistration(r, deps); err != nil {
			problems = append(problems, err)
		}
	}
	return errors.Join(problems...)
}

// checkRegistration checks one registration.
func checkRegistration(r txn.Registration, deps ModuleDeps) error {
	// The registry takes the segment after the last /internal/ as the module, so that test
	// fixtures nested below a package act as modules. In the product a nested internal package
	// (internal/wire/internal/sync) could pass a closure of wire off as one of sync; the real
	// registration therefore allows exactly one /internal/ in the handler's path.
	if strings.Count(r.Handler, "/internal/") > 1 {
		return fmt.Errorf("registration %s of %s: the handler lies in a nested internal package, so its owner is not the module of its path", r.Handler, r.Subscriber)
	}
	switch r.Kind {
	case txn.KindHook:
		if owner, ok := hookOwners[r.Phase]; !ok || owner != r.Subscriber {
			return fmt.Errorf("hook %s of phase %s: module %s does not own this phase (02 section 8.1)", r.Handler, r.Phase, r.Subscriber)
		}
		return nil
	case txn.KindSubscribe:
		subDeps, known := deps[r.Subscriber]
		switch {
		case !known:
			return fmt.Errorf("subscription %s by %s: %s is not a module of architecture/modules.yaml", r.Handler, r.Subscriber, r.Subscriber)
		case r.Publisher == kernelPublisher:
			return nil
		case !slices.Contains(subDeps, r.Publisher):
			return fmt.Errorf("subscription %s: %s subscribes to %s of %s, but %s is not in depends_on of %s (a higher module subscribes to a lower one, R-AR-04)",
				r.Handler, r.Subscriber, r.Event, r.Publisher, r.Publisher, r.Subscriber)
		default:
			return nil
		}
	default:
		return fmt.Errorf("registration %s of %s has the unknown kind %q", r.Handler, r.Subscriber, r.Kind)
	}
}
