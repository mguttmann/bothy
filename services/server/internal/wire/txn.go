// SPDX-License-Identifier: Apache-2.0

package wire

import (
	"cmp"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strings"

	appspg "github.com/OWNER/bothy/services/server/internal/apps/adapters/postgres"
	appsapp "github.com/OWNER/bothy/services/server/internal/apps/app"
	auditapp "github.com/OWNER/bothy/services/server/internal/audit/app"
	authzapp "github.com/OWNER/bothy/services/server/internal/authz/app"
	filespg "github.com/OWNER/bothy/services/server/internal/files/adapters/postgres"
	filesapp "github.com/OWNER/bothy/services/server/internal/files/app"
	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
	messagingapp "github.com/OWNER/bothy/services/server/internal/messaging/app"
	notifyapp "github.com/OWNER/bothy/services/server/internal/notify/app"
	"github.com/OWNER/bothy/services/server/internal/platform/config"
	"github.com/OWNER/bothy/services/server/internal/platform/otel"
	"github.com/OWNER/bothy/services/server/internal/platform/pg"
	runsapp "github.com/OWNER/bothy/services/server/internal/runs/app"
	syncapp "github.com/OWNER/bothy/services/server/internal/sync/app"
	terminalpg "github.com/OWNER/bothy/services/server/internal/terminal/adapters/postgres"
	terminalapp "github.com/OWNER/bothy/services/server/internal/terminal/app"
)

// Modules holds the module objects of a process role whose hooks and subscribers the txn
// registry takes: the rights hooks of authz (phases 1 and 3.1, M1-29), the sync hook (M1-16) and
// the audit hook (M1-24). Constructors that fill it must not open network connections, so the
// golden test builds the real registration without a database.
type Modules struct {
	// Authz holds the rights hooks (phases 1 and 3.1); required. It fails every rights change
	// until it is bound to the appender and the recorder (RightsHook.Bind, start step authz).
	Authz *authzapp.RightsHook
	// Sync is the sync phase hook (phase 3.2); required.
	Sync *syncapp.PhaseHook
	// Audit is the audit phase hook (phase 3.3); required.
	Audit *auditapp.Chain
	// SyncSubscribers are the in-transaction subscribers of sync (MessagePosted, M1-61);
	// required. They fail a message with clear_draft until the start binds the appender
	// (Subscribers.Bind).
	SyncSubscribers *syncapp.Subscribers
	// NotifySubscribers are the in-transaction subscribers of notify (ChannelArchived and
	// SpaceArchived, M3-18); required. They fail an archive until the chat step binds them
	// (Subscribers.Bind).
	NotifySubscribers *notifyapp.Subscribers
	// MessagingSubscribers are the in-transaction subscribers of messaging (ThreadMoved moves
	// messages.channel_id, M1-62); required.
	MessagingSubscribers *messagingapp.Subscribers
	// FilesSubscribers are the in-transaction subscribers of files (ThreadMoved moves the copies
	// message_attachments.channel_id, M3-07); required.
	FilesSubscribers *filesapp.Subscribers
	// RunsAcceptor is the in-transaction subscriber of runs (MessagePosted, the acceptance of a
	// triggering message, M2-05); required. It writes nothing until the chat step binds it to the
	// runs use cases (Acceptor.Bind, D-M2-05-19).
	RunsAcceptor *runsapp.Acceptor
	// AppsArchive is the in-transaction subscriber of apps (ChannelArchived revokes the incoming
	// webhooks of the channel, M2-27); required.
	AppsArchive *appsapp.ArchiveSubscriber
	// TerminalSubscribers are the in-transaction subscribers of terminal (AccessChanged,
	// DeviceRevoked and SessionEnded end terminal sessions, M2-46); required. The terminal step
	// binds the use cases (Subscribers.Bind).
	TerminalSubscribers *terminalapp.Subscribers
	// TestSubscribers registers the subscribers of a test after every production one (a veto
	// after files.OnThreadMoved or runs.OnFileFinalized, M3-07; wire.Deps.TxnTestSubscribers);
	// nil in production.
	TestSubscribers func(reg *txn.Registry) error
}

// NewModules returns the module objects over pool with log for their reports; pool may be nil
// where no unit of work runs (the golden test). It opens no connection. The rights hook gets the
// empty structure and agent policy sources until M1-54 and M1-51.
func NewModules(pool *pg.Pool, log *slog.Logger) *Modules { return NewModulesWithClock(pool, nil, log) }

// NewModulesWithClock is NewModules with the clock of the rights hook (nil means clock.System()):
// the serve process passes wire.Deps.Clock, so a test on a fake clock folds due expiries on that
// clock (M2-46).
func NewModulesWithClock(pool *pg.Pool, clk clock.Clock, log *slog.Logger) *Modules {
	return &Modules{Authz: mustRightsHook(pool, RightsSources{}, clk, log), Sync: newSyncHook(pool, log), Audit: newAuditChain(pool, log),
		SyncSubscribers: newSyncSubscribers(pool), NotifySubscribers: newNotifySubscribers(pool),
		MessagingSubscribers: newMessagingSubscribers(pool), FilesSubscribers: newFilesSubscribers(pool), RunsAcceptor: runsapp.NewAcceptor(),
		AppsArchive:         appsapp.NewArchiveSubscriber(appspg.NewRepository(pool)),
		TerminalSubscribers: newTerminalSubscribers(pool)}
}

// newFilesSubscribers returns the in-transaction subscribers of files over pool (ThreadMoved moves
// the copies message_attachments.channel_id, M3-07). It opens no connection.
func newFilesSubscribers(pool *pg.Pool) *filesapp.Subscribers {
	subs, err := filesapp.NewSubscribers(filespg.NewRepository(pool))
	if err != nil {
		// The repository is never nil, so this is a programming error.
		panic(err)
	}
	return subs
}

// errNoFilesSubscribers reports Modules without the subscribers of files: a moved thread would
// keep the attachment copies at the old channel (R-DM-43).
var errNoFilesSubscribers = errors.New("wire: the txn registry needs the subscribers of files (Modules.FilesSubscribers)")

// newTerminalSubscribers returns the unbound subscribers of terminal over pool (nil: none read, the
// golden test). It opens no connection.
func newTerminalSubscribers(pool *pg.Pool) *terminalapp.Subscribers {
	if pool == nil {
		return terminalapp.NewSubscribers(nil)
	}
	return terminalapp.NewSubscribers(terminalpg.NewRepository(pool))
}

// errNoTerminalSubscribers reports Modules without the subscribers of terminal: a revocation would
// leave terminal sessions open (24 section 8.3).
var errNoTerminalSubscribers = errors.New("wire: the txn registry needs the subscribers of terminal (Modules.TerminalSubscribers)")

// NewTxnRegistry returns the sealed txn registry of a process role: every role (api, worker,
// all) builds its registry here, so the golden file shows the one list of all roles. The error
// is the first rejected registration, or a missing module object.
//
// Example:
//
//	reg, err := wire.NewTxnRegistry(wire.NewModules(pool, logger))
//	if err != nil {
//		return err
//	}
//	uow := pg.NewUnitOfWork(pool, unitCfg, reg)
func NewTxnRegistry(m *Modules) (*txn.Registry, error) {
	reg := txn.NewRegistry()
	if err := registerTxn(reg, m); err != nil {
		return nil, err
	}
	if m.TestSubscribers != nil {
		if err := m.TestSubscribers(reg); err != nil {
			return nil, err
		}
	}
	reg.Seal()
	return reg, nil
}

// errNoAuthzHook reports Modules without the rights hooks: a rights change without them would
// skip permission_version, I1 and SR8.
var errNoAuthzHook = errors.New("wire: the txn registry needs the rights hooks (Modules.Authz)")

// errNoSyncSubscribers reports Modules without the subscribers of sync: a message with
// clear_draft would keep the draft.
var errNoSyncSubscribers = errors.New("wire: the txn registry needs the subscribers of sync (Modules.SyncSubscribers)")

// errNoNotifySubscribers reports Modules without the subscribers of notify: an archive would keep
// the notification preferences of the archived target (R-DM-16).
var errNoNotifySubscribers = errors.New("wire: the txn registry needs the subscribers of notify (Modules.NotifySubscribers)")

// errNoMessagingSubscribers reports Modules without the subscribers of messaging: a moved thread
// would keep its messages at the old channel.
var errNoMessagingSubscribers = errors.New("wire: the txn registry needs the subscribers of messaging (Modules.MessagingSubscribers)")

// errNoRunsAcceptor reports Modules without the acceptance of runs: a triggering message would open
// no run.
var errNoRunsAcceptor = errors.New("wire: the txn registry needs the acceptance of runs (Modules.RunsAcceptor)")

// errNoAuditHook reports Modules without the audit hook: every unit of work must chain its audit
// entries, so a registry without it is refused.
var errNoAuditHook = errors.New("wire: the txn registry needs the audit hook (Modules.Audit)")

// registerTxn is the only place with SetHook and Subscribe calls (02 "In-Tx events": one
// list). Hooks and handlers are method values of the module objects in m, never closures here,
// because the registry derives their owner from the Go package. It sets the tracer of subscriber
// spans, the rights hooks of phases 1 and 3.1, the sync hook of phase 3.2 and the audit hook of
// phase 3.3, the subscription messaging.MessagePosted -> sync (drafts with clear_draft, M1-61),
// chat.ThreadMoved -> messaging, then -> sync (the copies of channel_id, M1-62), then -> files (the
// copies message_attachments.channel_id, M3-07) and chat.ChannelArchived and chat.SpaceArchived -> notify (preferences, M3-18) and, after the
// existing subscribers, messaging.MessagePosted -> runs (the acceptance of a triggering message,
// M2-05), chat.ChannelArchived -> apps (M2-27), authorize.AccessChanged, published by authz, and
// agents.AgentDisabled -> runs (M2-09, 02 section 4.3), chat.ChannelReadOnlySet -> runs (the
// cancellation of the active runs of a channel that becomes read-only, M8-03),
// kernel.AccessChanged, identity.DeviceRevoked and identity.SessionEnded -> terminal (the end of
// terminal sessions at every revocation, M2-46) messaging.MessageDeleted -> runs (the
// components of a deleted card, M2-15) and agents.SessionAttached -> runs (M2-12).
func registerTxn(reg *txn.Registry, m *Modules) error {
	reg.SetTracer(otel.SubscriberSpan)
	if m == nil || m.Audit == nil {
		return errNoAuditHook
	}
	if m.Sync == nil {
		return errNoSyncHook
	}
	if m.Authz == nil {
		return errNoAuthzHook
	}
	if err := reg.SetHook(txn.PhaseRightsBegin, m.Authz.Begin); err != nil {
		return err
	}
	if err := reg.SetHook(txn.PhaseRightsCheck, m.Authz.BeforeCommit); err != nil {
		return err
	}
	if err := reg.SetHook(txn.PhaseSync, m.Sync.Run); err != nil {
		return err
	}
	if err := reg.SetHook(txn.PhaseAudit, m.Audit.Hook); err != nil {
		return err
	}
	if m.SyncSubscribers == nil {
		return errNoSyncSubscribers
	}
	if m.MessagingSubscribers == nil {
		return errNoMessagingSubscribers
	}
	if err := txn.Subscribe(reg, m.SyncSubscribers.OnMessagePosted); err != nil {
		return err
	}
	if err := txn.Subscribe(reg, m.MessagingSubscribers.OnThreadMoved); err != nil {
		return err
	}
	if err := txn.Subscribe(reg, m.SyncSubscribers.OnThreadMoved); err != nil {
		return err
	}
	if m.FilesSubscribers == nil {
		return errNoFilesSubscribers
	}
	// After messaging and sync, so the move locks thread, messages, the rows of sync, then
	// message_attachments (03 section 6.3, brief M3-07 note 8).
	if err := txn.Subscribe(reg, m.FilesSubscribers.OnThreadMoved); err != nil {
		return err
	}
	return registerArchiveAndTerminal(reg, m)
}

// registerArchiveAndTerminal registers chat.ChannelArchived and chat.SpaceArchived -> notify
// (M3-18), then kernel.AccessChanged, identity.DeviceRevoked and identity.SessionEnded -> terminal
// (M2-46), in this order after the subscriptions of registerTxn.
func registerArchiveAndTerminal(reg *txn.Registry, m *Modules) error {
	if m.NotifySubscribers == nil {
		return errNoNotifySubscribers
	}
	return subscribeArchivesAndRuns(reg, m)
}

// subscribeArchivesAndRuns registers chat.ChannelArchived and chat.SpaceArchived -> notify, then
// messaging.MessagePosted -> runs after the existing subscribers (M2-05) and -> notify (the
// automatic following, M3-19), then chat.ChannelArchived -> apps (the incoming webhooks of the
// channel, M2-27), then authorize.AccessChanged and agents.AgentDisabled -> runs (M2-09), then
// kernel.AccessChanged, identity.DeviceRevoked and identity.SessionEnded -> terminal (M2-46), then
// messaging.MessageDeleted -> runs (the open follow-up of a deleted message, M2-11, and the
// components of a deleted card, M2-15, in one subscriber), then files.FileFinalized -> runs (the
// release of held agent invocations, M3-07), then agents.SessionAttached -> runs (M2-12).
func subscribeArchivesAndRuns(reg *txn.Registry, m *Modules) error {
	if err := txn.Subscribe(reg, m.NotifySubscribers.OnChannelArchived); err != nil {
		return err
	}
	if err := txn.Subscribe(reg, m.NotifySubscribers.OnSpaceArchived); err != nil {
		return err
	}
	if m.RunsAcceptor == nil {
		return errNoRunsAcceptor
	}
	if err := txn.Subscribe(reg, m.RunsAcceptor.OnMessagePosted); err != nil {
		return err
	}
	// The automatic following of notify, after the subscribers of sync and runs (13 3.2, M3-19).
	if err := txn.Subscribe(reg, m.NotifySubscribers.OnMessagePosted); err != nil {
		return err
	}
	if m.AppsArchive == nil {
		return errNoAppsArchive
	}
	if err := txn.Subscribe(reg, m.AppsArchive.OnChannelArchived); err != nil {
		return err
	}
	// The revocation and the disabled agent end or stop the affected runs in the same
	// transaction (R-SY-11, 09 2.1; M2-09).
	if err := txn.Subscribe(reg, m.RunsAcceptor.OnAccessChanged); err != nil {
		return err
	}
	if err := txn.Subscribe(reg, m.RunsAcceptor.OnAgentDisabled); err != nil {
		return err
	}
	// A channel that becomes read-only cancels its active runs in the same transaction (05
	// channel.read_only.set, 22 13.4; M8-03).
	if err := txn.Subscribe(reg, m.RunsAcceptor.OnChannelReadOnlySet); err != nil {
		return err
	}
	return subscribeTerminalAndRunEnds(reg, m)
}

// subscribeTerminalAndRunEnds continues subscribeArchivesAndRuns in the same order:
// kernel.AccessChanged, identity.DeviceRevoked and identity.SessionEnded -> terminal (M2-46),
// then messaging.MessageDeleted, files.FileFinalized and agents.SessionAttached -> runs.
func subscribeTerminalAndRunEnds(reg *txn.Registry, m *Modules) error {
	if m.TerminalSubscribers == nil {
		return errNoTerminalSubscribers
	}
	if err := txn.Subscribe(reg, m.TerminalSubscribers.OnAccessChanged); err != nil {
		return err
	}
	if err := txn.Subscribe(reg, m.TerminalSubscribers.OnDeviceRevoked); err != nil {
		return err
	}
	if err := txn.Subscribe(reg, m.TerminalSubscribers.OnSessionEnded); err != nil {
		return err
	}
	if err := txn.Subscribe(reg, m.RunsAcceptor.OnMessageDeleted); err != nil {
		return err
	}
	// The transaction that makes the last attachment final writes the held agent.invocation
	// (04 section 15.3 point 1, D-K-G6).
	if err := txn.Subscribe(reg, m.RunsAcceptor.OnFileFinalized); err != nil {
		return err
	}
	// A report that creates or replaces the session row of a run attaches it to the run and, on
	// fail open, writes the system line (M2-12).
	return txn.Subscribe(reg, m.RunsAcceptor.OnSessionAttached)
}

// errNoAppsArchive reports Modules without the archive subscriber of apps: an archived channel
// would keep incoming webhooks that can never post again.
var errNoAppsArchive = errors.New("wire: the txn registry needs the archive subscriber of apps (Modules.AppsArchive)")

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
