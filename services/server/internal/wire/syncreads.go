// SPDX-License-Identifier: Apache-2.0

package wire

import (
	"errors"

	chatapp "github.com/OWNER/bothy/services/server/internal/chat/app"
	"github.com/OWNER/bothy/services/server/internal/instance"
	instancepg "github.com/OWNER/bothy/services/server/internal/instance/adapters/postgres"
	instanceapp "github.com/OWNER/bothy/services/server/internal/instance/app"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
	messagingapp "github.com/OWNER/bothy/services/server/internal/messaging/app"
	"github.com/OWNER/bothy/services/server/internal/platform/config"
	"github.com/OWNER/bothy/services/server/internal/platform/pg"
	"github.com/OWNER/bothy/services/server/internal/runs"
	synchttp "github.com/OWNER/bothy/services/server/internal/sync/adapters/httpapi"
	syncpg "github.com/OWNER/bothy/services/server/internal/sync/adapters/postgres"
	"github.com/OWNER/bothy/services/server/internal/sync/adapters/runsummary"
	syncapp "github.com/OWNER/bothy/services/server/internal/sync/app"
)

// CatchUpConfig binds the keys of catch-up (sync.catchup_page_size, sync.catchup_max_events). The
// error names the key whose binding failed, never a value.
func CatchUpConfig(v *config.Values) (syncapp.CatchUpConfig, error) {
	var c syncapp.CatchUpConfig
	if err := config.Bind(v, &c); err != nil {
		return syncapp.CatchUpConfig{}, err
	}
	return c, nil
}

// SyncReadDeps are the collaborators of catch-up and snapshot: the pool and unit of work, the
// Authorizer, the chat service (structure listings and thread counters), the messaging service
// (unread mentions), the personal data of sync (sidebar, settings, drafts), the agents listing,
// the principal directory of identity and the feature flags of instance.
type SyncReadDeps struct {
	Pool       *pg.Pool
	UoW        txn.UnitOfWork
	Authorizer authorize.Authorizer
	Chat       *chatapp.Service
	Messaging  *messagingapp.Service
	Personal   *syncapp.Personal
	Agents     syncapp.AgentLister
	Directory  syncapp.PrincipalDirectory
	Flags      instance.Flags
	// Runs gives the running runs of the snapshot (M2-05); required.
	Runs runs.Runs
}

// SyncReads is the read part of the sync module (M1-63): the use cases of sync.CatchUp and
// sync.Snapshot, which the gateway reuses (M1-64, M1-65), and their REST handlers.
type SyncReads struct {
	CatchUp  *syncapp.CatchUpService
	Snapshot *syncapp.SnapshotService
	Handlers *synchttp.Sync
}

// NewSyncReads builds catch-up and snapshot over d with the limits cfg. The head is event_seq of
// instance.Versions, the epoch instance.sync_epoch of instance.State, and the running runs come
// from the runs module through adapters/runsummary (M2-05). It opens no connection. Errors: a missing
// collaborator or a limit below 1.
func NewSyncReads(cfg syncapp.CatchUpConfig, d SyncReadDeps) (*SyncReads, error) {
	if d.Pool == nil || d.UoW == nil || d.Chat == nil || d.Messaging == nil || d.Personal == nil || d.Agents == nil || d.Directory == nil ||
		d.Flags == nil || d.Runs == nil {
		return nil, errors.New("wire: catch-up and snapshot need the pool, chat, messaging, the personal data, agents, identity and the flags")
	}
	repo := instancepg.NewRepository(d.Pool)
	head, epoch := instanceapp.NewVersions(repo), instanceapp.NewState(repo, d.UoW)
	c, err := syncapp.NewCatchUp(cfg, syncapp.CatchUpDeps{Log: syncpg.NewRepository(d.Pool, nil), Head: head, Epoch: epoch, Authorizer: d.Authorizer})
	if err != nil {
		return nil, err
	}
	s, err := syncapp.NewSnapshot(syncapp.SnapshotDeps{
		Head: head, Epoch: epoch, Authorizer: d.Authorizer, Directory: d.Directory, Structure: d.Chat, Threads: d.Chat,
		Store: syncpg.NewPersonalRepository(d.Pool), Mentions: d.Messaging.Mentions(), Settings: d.Personal.Settings(),
		Drafts: d.Personal.Drafts(), Agents: d.Agents, Runs: runsummary.New(d.Runs), Flags: d.Flags,
	})
	if err != nil {
		return nil, err
	}
	return &SyncReads{CatchUp: c, Snapshot: s, Handlers: synchttp.NewSync(c, s)}, nil
}

// fillHandlers sets the nil catch-up and snapshot handler of h to the one r built; a nil r leaves
// h unchanged.
func (r *SyncReads) fillHandlers(h *Handlers) {
	if r != nil && h.Sync == nil {
		h.Sync = r.Handlers
	}
}
