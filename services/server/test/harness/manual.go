// SPDX-License-Identifier: Apache-2.0

package harness

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5"

	instancejobs "github.com/OWNER/bothy/services/server/internal/instance/adapters/jobs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	runsjobs "github.com/OWNER/bothy/services/server/internal/runs/adapters/jobs"
	"github.com/OWNER/bothy/services/server/internal/wire"
)

// The job kinds of Options.ManualJobs: River runs no-ops for them, Server.RunDueJobs the real
// handlers. runs.stall_check and instance.key_rotation are periodic and run once per call; the
// others run per scheduled job.
var manualKinds = []string{
	runsjobs.KindQueuedExpiry, runsjobs.KindCancelTimeout, runsjobs.KindStallCheck, instancejobs.KindKeyRotation,
}

// ManualJobKinds returns the job kinds RunDueJobs accepts.
func ManualJobKinds() []string { return append([]string(nil), manualKinds...) }

// manualJobs keeps the real use cases of the manual kinds of the running generation and the IDs
// of the scheduled jobs RunDueJobs already ran (they stay in River's table, completed by the
// no-op, so the set keeps a job from running twice).
type manualJobs struct {
	mu        sync.Mutex
	expiry    runsjobs.Expirer
	lifecycle runsjobs.Lifecycle
	rotation  instancejobs.Rotation
	ran       map[int64]bool
}

// newManualJobs returns an empty set.
func newManualJobs() *manualJobs { return &manualJobs{ran: map[int64]bool{}} }

// take returns the JobModules hook of a generation: it keeps the real use cases of the manual
// kinds, gives River no-ops in their place, then calls the test's own hook (next, may be nil).
func (m *manualJobs) take(next func(*wire.JobModules)) func(*wire.JobModules) {
	return func(jm *wire.JobModules) {
		m.mu.Lock()
		m.expiry, m.lifecycle, m.rotation = jm.QueuedExpiry, jm.RunsLifecycle, jm.KeyRotation
		m.mu.Unlock()
		jm.QueuedExpiry, jm.RunsLifecycle, jm.KeyRotation = noJob{}, noJob{}, noJob{}
		if next != nil {
			next(jm)
		}
	}
}

// withoutStatusProbe returns the JobModules hook without Options.StatusProbe: the test's own
// hook (next, may be nil) runs first and sees the real use case of status.probe, then River gets
// a no-op in its place (PL/46).
func withoutStatusProbe(next func(*wire.JobModules)) func(*wire.JobModules) {
	return func(jm *wire.JobModules) {
		if next != nil {
			next(jm)
		}
		jm.StatusProbe = noJob{}
	}
}

// noJob is the River handler of every manual kind and of status.probe: it does nothing.
type noJob struct{}

func (noJob) ExpireQueued(context.Context, ids.ID) error  { return nil }
func (noJob) CancelTimeout(context.Context, ids.ID) error { return nil }
func (noJob) StallCheck(context.Context) error            { return nil }
func (noJob) RunRotation(context.Context) (int, error)    { return 0, nil }
func (noJob) Run(context.Context) error                   { return nil }

// RunDueJobs runs the jobs of kind that are due on the harness clock through their real handler
// and returns how many ran: for runs.queued_expiry and runs.cancel_timeout every job of River's
// table scheduled at or before Clock().Now() that this server did not run yet, in id order; for
// the periodic runs.stall_check and instance.key_rotation one run. It needs Options.ManualJobs;
// an unknown kind or a failing handler fails tb.
//
// Example:
//
//	s.Clock().(*clock.Fake).Advance(grace)
//	if n := s.RunDueJobs(t, "runs.cancel_timeout"); n != 1 { t.Fatalf("ran %d", n) }
func (s *Server) RunDueJobs(tb testing.TB, kind string) int {
	tb.Helper()
	if s.manual == nil {
		tb.Fatal("harness: RunDueJobs needs Options.ManualJobs")
	}
	m := s.manual
	m.mu.Lock()
	expiry, lifecycle, rotation := m.expiry, m.lifecycle, m.rotation
	m.mu.Unlock()
	ctx := context.WithoutCancel(tb.Context())
	switch kind {
	case runsjobs.KindStallCheck:
		if err := lifecycle.StallCheck(ctx); err != nil {
			tb.Fatalf("harness: %s: %v", kind, err)
		}
		return 1
	case instancejobs.KindKeyRotation:
		if _, err := rotation.RunRotation(ctx); err != nil {
			tb.Fatalf("harness: %s: %v", kind, err)
		}
		return 1
	case runsjobs.KindQueuedExpiry, runsjobs.KindCancelTimeout:
	default:
		tb.Fatalf("harness: RunDueJobs of the kind %q (want one of %v)", kind, manualKinds)
	}
	due := s.dueJobs(tb, kind)
	for _, j := range due {
		var err error
		if kind == runsjobs.KindQueuedExpiry {
			err = expiry.ExpireQueued(ctx, j.run)
		} else {
			err = lifecycle.CancelTimeout(ctx, j.run)
		}
		if err != nil {
			tb.Fatalf("harness: %s of job %d: %v", kind, j.id, err)
		}
		m.mu.Lock()
		m.ran[j.id] = true
		m.mu.Unlock()
	}
	return len(due)
}

// dueJob is one scheduled job of a run.
type dueJob struct {
	id  int64
	run ids.ID
}

// dueJobs reads the jobs of kind scheduled at or before the harness clock that did not run yet.
func (s *Server) dueJobs(tb testing.TB, kind string) []dueJob {
	tb.Helper()
	schema := pgx.Identifier{s.ConfigValue("jobs.schema"), "river_job"}.Sanitize()
	rows, err := s.AdminDB().Query(tb.Context(),
		`SELECT id, args FROM `+schema+` WHERE kind = $1 AND scheduled_at <= $2 ORDER BY id`, kind, s.clk.Now())
	if err != nil {
		tb.Fatalf("harness: read the jobs of %s: %v", kind, err)
	}
	defer rows.Close()
	var out []dueJob
	s.manual.mu.Lock()
	defer s.manual.mu.Unlock()
	for rows.Next() {
		var id int64
		var raw []byte
		if err := rows.Scan(&id, &raw); err != nil {
			tb.Fatal(err)
		}
		if s.manual.ran[id] {
			continue
		}
		var args struct {
			RunID ids.ID `json:"run_id"`
		}
		if err := json.Unmarshal(raw, &args); err != nil {
			tb.Fatalf("harness: arguments of job %d of %s: %v", id, kind, err)
		}
		out = append(out, dueJob{id: id, run: args.RunID})
	}
	if err := rows.Err(); err != nil {
		tb.Fatal(err)
	}
	return out
}

// CursorSnapshot is the stored state of agent_consumer_cursors at one moment.
type CursorSnapshot struct {
	rows []cursorRow
}

// cursorRow is one stored cursor.
type cursorRow struct {
	agent, consumer string
	acked           int64
}

// Len returns the number of cursors in the snapshot.
func (c CursorSnapshot) Len() int { return len(c.rows) }

// SnapshotCursors returns the stored rows of agent_consumer_cursors.
func (s *Server) SnapshotCursors(tb testing.TB) CursorSnapshot {
	tb.Helper()
	rows, err := s.AdminDB().Query(tb.Context(), `SELECT agent_id::text, consumer_name, acked_seq FROM agent_consumer_cursors ORDER BY agent_id, consumer_name`)
	if err != nil {
		tb.Fatal(err)
	}
	defer rows.Close()
	var snap CursorSnapshot
	for rows.Next() {
		var r cursorRow
		if err := rows.Scan(&r.agent, &r.consumer, &r.acked); err != nil {
			tb.Fatal(err)
		}
		snap.rows = append(snap.rows, r)
	}
	if err := rows.Err(); err != nil {
		tb.Fatal(err)
	}
	return snap
}

// RestartOptions change a Restart.
type RestartOptions struct {
	// RestoreCursors, when set, is written back between stop and start: every cursor of the
	// snapshot gets its acked_seq again and no holder, which leaves the state of a process that
	// died before the acknowledgements made after the snapshot were stored (crash emulation,
	// M2-31). Cursors created after the snapshot stay.
	RestoreCursors *CursorSnapshot
	// Config, when set, replaces the entries of Options.Config with the same key for this and
	// every later generation, for example server.maintenance_mode (M7-08); an empty value removes
	// the entry, so the key falls back to its default.
	Config map[string]string
	// Telemetry, when set, replaces Options.Telemetry for this and every later generation, so a
	// generation that a test observes starts with empty sinks; without it the sinks of the
	// previous generation go on (their metrics read the newest generation, MetricSink).
	Telemetry *TelemetrySinks
	// WhileStopped, when set, runs between stop and start, after RestoreCursors: a test takes a
	// snapshot of the database or restores one and finalizes the restore there (M7-07).
	WhileStopped func()
}

// Restart stops the running server gracefully (SIGTERM: gateway sessions close, held
// acknowledgements are stored) and starts it again on the same database, secrets, signing key
// store, file root, configuration and clock, with new listeners on 127.0.0.1; ListenURL changes.
// It needs Options.PublicURL, which stays server.public_url, so the clients of the test keep
// their address through a proxy in front of the server. Owner and every client stay valid.
// Errors fail tb.
func (s *Server) Restart(tb testing.TB, opts RestartOptions) {
	tb.Helper()
	if s.opts.PublicURL == "" {
		tb.Fatal("harness: Restart needs Options.PublicURL (a proxy keeps the address of the clients)")
	}
	s.stopGen()
	if opts.RestoreCursors != nil {
		for _, r := range opts.RestoreCursors.rows {
			if _, err := s.AdminDB().Exec(tb.Context(),
				`UPDATE agent_consumer_cursors SET acked_seq = $3, holder_session_id = NULL WHERE agent_id = $1::uuid AND consumer_name = $2`,
				r.agent, r.consumer, r.acked); err != nil {
				tb.Fatalf("harness: restore cursor: %v", err)
			}
		}
	}
	if opts.WhileStopped != nil {
		opts.WhileStopped()
	}
	next := s.opts
	next.manual = s.manual
	if opts.Telemetry != nil {
		next.Telemetry = opts.Telemetry
	}
	if opts.Config != nil {
		next.Config = maps.Clone(next.Config)
		if next.Config == nil {
			next.Config = map[string]string{}
		}
		for k, v := range opts.Config {
			if v == "" {
				delete(next.Config, k)
			} else {
				next.Config[k] = v
			}
		}
	}
	code := "-"
	n := startRetrying(tb, s.db, next, s.clk, s.SecretsDir, s.SigningKeyDir, s.FilesRoot, &code)
	s.ListenURL, s.InternalURL, s.values, s.env, s.stopGen = n.ListenURL, n.InternalURL, n.values, n.env, n.stopGen
	s.opts = next
}

// AgentCredentials are the API token and the DPoP private key of an agent client. Test-only: a
// test hands them to an adapter process it starts (over stdin) and never logs them.
type AgentCredentials struct {
	// Token is the API token of the agent.
	Token string
	// DPoPPrivateJWK is the private ES256 key the token is bound to, as a JWK.
	DPoPPrivateJWK []byte
}

// AgentCredentials returns the credentials of a client made by NewAgent (or NewTokenClient); a
// client of a person has none and fails the test.
func (c *Client) AgentCredentials() AgentCredentials {
	if !c.apiToken {
		c.tb.Fatal("harness: AgentCredentials of a client without an API token")
	}
	access, _ := c.Tokens()
	return AgentCredentials{Token: access, DPoPPrivateJWK: c.key.privateJWK()}
}

// String never shows the credentials.
func (AgentCredentials) String() string { return "harness.AgentCredentials{redacted}" }

// GoString never shows the credentials.
func (a AgentCredentials) GoString() string { return a.String() }

// Format never shows the credentials, whatever the verb.
func (a AgentCredentials) Format(f fmt.State, _ rune) { _, _ = f.Write([]byte(a.String())) }
