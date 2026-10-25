// SPDX-License-Identifier: Apache-2.0

package wire

import (
	"context"
	"errors"
	"maps"
	"slices"
	"time"

	authzdomain "github.com/OWNER/bothy/services/server/internal/authz/domain"
	"github.com/OWNER/bothy/services/server/internal/chat"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
	"github.com/OWNER/bothy/services/server/internal/platform/config"
	"github.com/OWNER/bothy/services/server/internal/platform/httpserver"
	"github.com/OWNER/bothy/services/server/internal/platform/jobs"
	"github.com/OWNER/bothy/services/server/internal/platform/pg"
	"github.com/OWNER/bothy/services/server/internal/runs"
	runsjobs "github.com/OWNER/bothy/services/server/internal/runs/adapters/jobs"
)

// Maintenance is maintenance mode of the process (server.maintenance_mode, ADR 0065, M7-08),
// read once at the start of api and worker.
type Maintenance struct {
	// ReadOnly is server.maintenance_mode = read_only.
	ReadOnly bool
	// RetryAfter is server.maintenance_retry_after: the Retry-After of 503 maintenance and the
	// delay of a paused job.
	RetryAfter time.Duration
}

// maintenanceKeys are the two keys of maintenance mode.
type maintenanceKeys struct {
	Mode       string        `config:"server.maintenance_mode"`
	RetryAfter time.Duration `config:"server.maintenance_retry_after"`
}

// MaintenanceFrom binds maintenance mode from v. The error names the key whose binding failed.
func MaintenanceFrom(v *config.Values) (Maintenance, error) {
	var k maintenanceKeys
	if err := config.Bind(v, &k); err != nil {
		return Maintenance{}, err
	}
	return Maintenance{ReadOnly: k.Mode == maintenanceReadOnly, RetryAfter: k.RetryAfter}, nil
}

// writesInCatalogue reports whether action has effect write in the catalogue (05 section 2.7);
// an unknown action counts as write, so it is blocked in maintenance mode (fail closed).
func writesInCatalogue(action string) bool {
	spec, ok := authzdomain.LookupAction(action)
	return !ok || spec.Effect != authzdomain.EffectRead
}

// maintenanceRuns implements httpserver.RunningRuns over the port of runs and the threads of
// chat, which give each running run the channel of its thread (runs has no channel_id,
// D-M2-05-9).
type maintenanceRuns struct {
	runs    runs.RunningRuns
	threads chat.Threads
}

// Running implements httpserver.RunningRuns.
func (m maintenanceRuns) Running(ctx context.Context, agentID ids.ID) ([]httpserver.RunningRun, error) {
	rs, err := m.runs.Running(ctx, agentID)
	if err != nil || len(rs) == 0 {
		return nil, err
	}
	threadIDs := make([]ids.ID, len(rs))
	for i, r := range rs {
		threadIDs[i] = r.ThreadID
	}
	infos, err := m.threads.Locate(ctx, threadIDs)
	if err != nil {
		return nil, err
	}
	channels := make(map[ids.ID]ids.ID, len(infos))
	for _, info := range infos {
		channels[info.ID] = info.ChannelID
	}
	out := make([]httpserver.RunningRun, len(rs))
	for i, r := range rs {
		out[i] = httpserver.RunningRun{RunID: r.ID, ThreadID: r.ThreadID, ChannelID: channels[r.ThreadID]}
	}
	return out, nil
}

// InstallMaintenance installs the stage of maintenance mode at httpserver.StageMaintenance of
// chain when m is read_only (05 rule 1.9, ADR 0065 item 3), over the effect of the catalogue, the
// running runs of rs and the threads of chat; with mode off it installs nothing, so the slot
// passes through. writes replaces the effect of the catalogue when not nil (Deps.MaintenanceWrites,
// tests only). The error is the one of httpserver.NewMaintenanceStage.
func InstallMaintenance(chain *httpserver.Chain, m Maintenance, rs runs.RunningRuns, threads chat.Threads, writes func(string) bool) error {
	if !m.ReadOnly {
		return nil
	}
	if writes == nil {
		writes = writesInCatalogue
	}
	stage, err := httpserver.NewMaintenanceStage(httpserver.MaintenanceConfig{
		RetryAfter: m.RetryAfter, Writes: writes, Runs: maintenanceRuns{runs: rs, threads: threads},
		Actions: authzdomain.ActionIDs(),
	})
	if err != nil {
		return err
	}
	chain.Use(httpserver.StageMaintenance, stage)
	return nil
}

// installMaintenance installs the stage of maintenance mode in chain over the runs and chat of
// the process (mountAPI, after the rate limits).
func (s *server) installMaintenance(chain *httpserver.Chain) error {
	if !s.maint.ReadOnly {
		return nil
	}
	s.mu.Lock()
	r, c := s.runs, s.chat
	s.mu.Unlock()
	if r == nil || c == nil {
		return errors.New("wire: maintenance mode needs the runs and the chat module")
	}
	return InstallMaintenance(chain, s.maint, r.Service, c.Service, s.deps.MaintenanceWrites)
}

// unitConfig is UnitConfig of the process with maintenance mode: in read_only every unit of work
// without txn.WithWriteInMaintenance runs READ ONLY (ADR 0065 item 5). Only bothy serve reads it;
// the commands keep UnitConfig.
func (s *server) unitConfig() (pg.UnitConfig, error) {
	c, err := UnitConfig(s.values)
	if err != nil {
		return pg.UnitConfig{}, err
	}
	c.ReadOnly, c.ReadOnlyRetryAfter = s.maint.ReadOnly, s.maint.RetryAfter
	return c, nil
}

// MaintenanceJobs returns the job runtime of maintenance mode over the kinds of reg (02 12.2,
// ADR 0065 item 4): the worker fetches from the queues delivery and maintenance and runs every
// kind of the queue delivery and runs.cancel_timeout; every other job of the queue maintenance
// waits pause and is looked at again, and the other queues and periodic jobs pause until a
// restart without the mode.
func MaintenanceJobs(reg *jobs.Registry, pause time.Duration) *jobs.Maintenance {
	m := &jobs.Maintenance{Queues: []string{jobs.QueueDelivery, jobs.QueueMaintenance}, Kinds: []string{runsjobs.KindCancelTimeout}, Pause: pause}
	for _, kind := range slices.Sorted(maps.Keys(reg.ArgTypes())) {
		if q, _ := reg.Queue(kind); q == jobs.QueueDelivery {
			m.Kinds = append(m.Kinds, kind)
		}
	}
	return m
}

// maintenancePatcher is the run patcher of the gateway op run.update in maintenance mode (ADR 0065
// item 4): a patch of the agent's own running run runs with txn.WithWriteInMaintenance, like
// patchRun through the stage; every other patch runs read-only and is refused there, unless the
// use case refuses it first (another agent's run, an ended run).
type maintenancePatcher struct {
	LiveBoxRuns
	running runs.RunningRuns
}

// Patch implements gatewayapp.RunPatcher.
func (m maintenancePatcher) Patch(ctx context.Context, in runs.PatchRun) (runs.PatchResult, error) {
	if a, ok := actor.FromContext(ctx); ok && a.Kind == actor.KindAgent {
		rs, err := m.running.Running(ctx, a.ID)
		if err != nil {
			return runs.PatchResult{}, err
		}
		if slices.ContainsFunc(rs, func(r runs.RunningRun) bool { return r.ID == in.RunID }) {
			ctx = txn.WithWriteInMaintenance(ctx)
		}
	}
	return m.LiveBoxRuns.Patch(ctx, in)
}
