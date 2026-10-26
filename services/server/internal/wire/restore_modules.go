// SPDX-License-Identifier: Apache-2.0

package wire

import (
	"context"
	"errors"
	"log/slog"

	"github.com/OWNER/bothy/services/server/internal/instance"
	instancepg "github.com/OWNER/bothy/services/server/internal/instance/adapters/postgres"
	"github.com/OWNER/bothy/services/server/internal/platform/config"
	"github.com/OWNER/bothy/services/server/internal/platform/jobs"
	"github.com/OWNER/bothy/services/server/internal/platform/lifecycle"
	"github.com/OWNER/bothy/services/server/internal/platform/pg"
)

// roleRestore is the internal role of bothy restore finalize (M7-07): the process builds the
// modules of the server on the restored database, so the deletion journal deletes again through
// the normal deletion use cases (SEC-BAK-3, 20 sections 4.2 to 4.4), but it opens no listener,
// runs no job, no consumer and no log tail, and measures nothing. bothy serve refuses the role.
const roleRestore Role = "restore"

// The order keys that bound the parts of roleRestore: from the secrets to the privacy step. The
// telemetry and the internal listener before them and the routes, the log tail, River, the
// consumers, the public listener and readiness after them belong to a serving process.
const (
	restoreFirstOrder = 200
	restoreLastOrder  = 1950
	// restoreSkipOrder is the internal listener, the one part inside the range that serves.
	restoreSkipOrder = 300
)

// restorePart reports whether p runs in roleRestore. Parts with a role condition (api, worker)
// also check it, and roleRestore is neither.
func restorePart(p part) bool {
	return p.order >= restoreFirstOrder && p.order <= restoreLastOrder && p.order != restoreSkipOrder
}

// restoreModules builds the modules of a server of roleRestore over the loaded values v (M7-07):
// the start parts of restorePart in their order, then a job runtime of the role api that only
// inserts (the deletions queue files.purge_objects and other jobs, which the worker runs after the
// restore), bound to the modules. The pool connects as database.user, the schema owner of bothy
// restore finalize. The start lock of R-DM-50 is not checked in this role. On an error the stop
// steps of what started run before it returns.
//
// Errors: a configuration that does not bind, a missing secret, an unreachable or too old
// database, the error of a module start.
func restoreModules(ctx context.Context, v *config.Values, env []string, log *slog.Logger) (*server, error) {
	s, err := newServer(v, roleRestore, Deps{Env: env, Logger: log})
	if err != nil {
		return nil, err
	}
	// The command reports its own outcome; the start and shutdown lines of a server would only
	// mix into it.
	s.runner = lifecycle.New(lifecycle.Options{Logger: slog.New(slog.DiscardHandler), TotalTimeout: s.cfg.TotalTimeout})
	s.runner.OnStop(lifecycle.StopStep{Name: "pool", Phase: lifecycle.PhaseDatabase, Run: s.stopDatabase})
	for _, c := range s.components() {
		if err := c.Start(ctx); err != nil {
			s.close(ctx)
			return nil, err
		}
		if c.Name() == stepSchema {
			// No module starts while a server is connected: the refusal comes before any read or
			// write of a module (M7-04 requirement 16).
			if err := s.refuseConnectedServers(ctx); err != nil {
				s.close(ctx)
				return nil, err
			}
		}
	}
	if err := s.bindInsertOnlyJobs(ctx); err != nil {
		s.close(ctx)
		return nil, err
	}
	return s, nil
}

// stepSchema is the start step that checks the schema; the database is open after it.
const stepSchema = "schema"

// refuseConnectedServers returns instance.ErrServerConnected while another session of bothy_app
// is connected to the database.
func (s *server) refuseConnectedServers(ctx context.Context) error {
	s.mu.Lock()
	pool := s.pool
	s.mu.Unlock()
	n, err := instancepg.NewRepository(pool).OtherSessionsOfRole(ctx, string(pg.RoleApp))
	if err != nil {
		return err
	}
	if n > 0 {
		return instance.ErrServerConnected
	}
	return nil
}

// bindInsertOnlyJobs builds the job registry of the modules and a runtime of the role api, which
// inserts and never works, and hands its inserter to the modules (bindModuleJobs).
func (s *server) bindInsertOnlyJobs(ctx context.Context) error {
	s.mu.Lock()
	pool := s.pool
	s.mu.Unlock()
	if err := jobs.ValidateMigrations(ctx, pool, s.jobsCfg.Schema); err != nil {
		return err
	}
	modules, err := s.retentionModules(ctx)
	if err != nil {
		return err
	}
	if err := s.moduleJobs(&modules); err != nil {
		return err
	}
	if modules.Retention, err = NewRetentionJobs(modules.RetentionParticipants, modules.RetentionConfig, s.deps.Clock); err != nil {
		return err
	}
	reg, err := NewJobRegistry(&modules)
	if err != nil {
		return err
	}
	rt, err := jobs.New(pool, s.jobsCfg, JobRole(s.role), reg, nil)
	if err != nil {
		return err
	}
	s.bindModuleJobs(rt.Inserter())
	return nil
}

// close runs the stop steps the parts of roleRestore registered (the audit denials, the sweepers,
// the pool and the object store) within shutdown.total_timeout.
func (s *server) close(ctx context.Context) {
	s.runner.Shutdown(context.WithoutCancel(ctx))
}

// errNoPrivacy is the error of a restore server whose privacy step did not run.
var errNoPrivacy = errors.New("wire: bothy restore finalize needs the privacy module for the deletion journal")
