// SPDX-License-Identifier: Apache-2.0

//go:build perf

package statsqueries_test

import (
	"net/http"
	"net/url"
	"os"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/perf/internal/perfkit"
	"github.com/OWNER/bothy/services/server/test/harness"
)

func TestMain(m *testing.M) { os.Exit(harness.Main(m)) }

// The parameters of the fixture (brief M6-05 "Tests").
const (
	channels = 10
	agents   = 10
	messages = 1_000_000
	// runsPerChannel runs, each with one tool call and one cost entry, per channel.
	runsPerChannel = 10_000
	// samples is how many cold calls each operation gets; each uses another range, so none is a
	// hit of the result cache.
	samples = 5
)

// TestPerf_StatsQueries seeds the fixture as rows of the database (the use cases would take hours
// for a million messages) and measures each statistics operation as the Owner over stats.max_range
// on the measured process. It records the slowest and the median answer per operation and the
// plans of the message and run reads.
func TestPerf_StatsQueries(t *testing.T) {
	env := perfkit.Setup(t)
	p := env.StartSeeding(t, nil)
	space := perfkit.Space(t, p, "Statistics")
	chans := perfkit.Channels(t, p, space, "stats", channels)
	maxRange := perfkit.DefaultDuration(t, "stats.max_range")
	db := p.AdminDB()
	ctx := t.Context()
	var owner string
	if err := db.QueryRow(ctx, `SELECT user_id::text FROM owners LIMIT 1`).Scan(&owner); err != nil {
		t.Fatal(err)
	}
	exec := func(sql string, args ...any) {
		t.Helper()
		if _, err := db.Exec(ctx, sql, args...); err != nil {
			t.Fatal(err)
		}
	}
	exec(`UPDATE threads SET created_at = now() - $1::interval`, seconds(maxRange))
	agentIDs := make([]string, agents)
	for i := range agentIDs {
		if err := db.QueryRow(ctx, `INSERT INTO principals (id, kind, status, handle, display_name)
			VALUES (gen_random_uuid(), 'agent', 'active', 'statsagent' || $1, 'statsagent' || $1) RETURNING id::text`, strconv.Itoa(i)).Scan(&agentIDs[i]); err != nil {
			t.Fatal(err)
		}
		exec(`INSERT INTO agents (principal_id, adapter_kind) VALUES ($1, 'custom')`, agentIDs[i])
	}
	step := maxRange / (messages / channels)
	for _, c := range chans {
		exec(`INSERT INTO messages (id, thread_id, channel_id, thread_seq, author_principal_id, client_msg_id, client_payload_sha256, origin, created_at)
			SELECT gen_random_uuid(), $1, $2, 1000 + g, $3, gen_random_uuid(), '\x00', 'human', now() - g * $4::interval
			FROM generate_series(1, $5) g`, c.Thread, c.ID, owner, seconds(step), messages/channels)
		exec(`INSERT INTO runs (id, thread_id, agent_id, trigger_kind, triggered_by, state, state_reason, billing_mode_expected, invocation_jti,
				queued_at, started_at, finished_at)
			SELECT gen_random_uuid(), $1, ($2::uuid[])[1 + g % $3], 'message', $4,
				(ARRAY['completed', 'failed', 'cancelled'])[1 + g % 3], CASE WHEN g % 3 = 1 THEN 'adapter_error' END, 'organization',
				gen_random_uuid(), now() - g * $5::interval, now() - g * $5::interval, now() - g * $5::interval + (g % 60) * interval '1 second'
			FROM generate_series(1, $6) g`, c.Thread, agentIDs, agents, owner, seconds(maxRange/runsPerChannel), runsPerChannel)
	}
	exec(`INSERT INTO run_steps (id, run_id, ordinal, kind, name, status, started_at, finished_at)
		SELECT gen_random_uuid(), r.id, 1, 'tool_call', 'tool' || (abs(hashtext(r.id::text)) % 20), 'completed', r.started_at, r.finished_at FROM runs r`)
	exec(`INSERT INTO cost_entries (id, run_id, agent_id, thread_id, channel_id, space_id, billing_mode, provider, model, amount_micros, source, occurred_at)
		SELECT gen_random_uuid(), r.id, r.agent_id, r.thread_id, t.channel_id, $1, 'organization', 'example', 'model-large', 1000, 'computed', r.finished_at
		FROM runs r JOIN threads t ON t.id = r.thread_id`, space)
	exec(`ANALYZE`)

	env.Measure(t, p, nil)
	to := time.Now().UTC().Truncate(time.Hour)
	extra := map[string]any{"stats_queries.params": map[string]any{"channels": channels, "agents": agents, "messages": messages,
		"runs": channels * runsPerChannel, "max_range_h": maxRange.Hours(), "samples": samples}}
	for _, op := range []struct{ name, path, granularity string }{
		{"usage_day", "/stats/usage", "day"}, {"usage_month", "/stats/usage", "month"}, {"runs_day", "/stats/runs", "day"},
		{"tool_calls_day", "/stats/tool-calls", "day"}, {"errors_day", "/stats/errors", "day"}, {"costs_day", "/stats/costs", "day"},
	} {
		var took []time.Duration
		for i := range samples {
			end := to.AddDate(0, 0, -32*i)
			q := url.Values{"from": {end.Add(-maxRange).Format(time.RFC3339)}, "to": {end.Format(time.RFC3339)}, "granularity": {op.granularity}}
			start := time.Now()
			if _, err := perfkit.Do(ctx, p.Owner, harness.Request{Method: http.MethodGet, Path: op.path + "?" + q.Encode()}, http.StatusOK); err != nil {
				t.Fatal(err)
			}
			took = append(took, time.Since(start))
		}
		slices.Sort(took)
		extra["stats_queries."+op.name+"_ms_max"] = float64(took[len(took)-1].Milliseconds())
		extra["stats_queries."+op.name+"_ms_median"] = float64(took[len(took)/2].Milliseconds())
		t.Logf("%s over %s: median %s, max %s", op.name, maxRange, took[len(took)/2], took[len(took)-1])
	}
	for name, sql := range map[string]string{
		"messages": `EXPLAIN SELECT date_trunc('day', m.created_at, 'UTC'), count(*) FROM messages m
			WHERE m.channel_id = ANY ($1::uuid[]) AND m.deleted_at IS NULL AND m.created_at >= now() - $2::interval AND m.created_at < now() GROUP BY 1`,
		"runs": `EXPLAIN SELECT date_trunc('day', r.finished_at, 'UTC'), r.agent_id, count(*) FROM runs r
			WHERE r.thread_id = ANY ($1::uuid[]) AND r.finished_at >= now() - $2::interval AND r.finished_at < now() GROUP BY 1, 2`,
	} {
		ids := make([]string, len(chans))
		for i, c := range chans {
			ids[i] = c.ID
			if name == "runs" {
				ids[i] = c.Thread
			}
		}
		rows, err := db.Query(ctx, sql, ids, seconds(maxRange))
		if err != nil {
			t.Fatal(err)
		}
		var plan []string
		for rows.Next() {
			var line string
			if err := rows.Scan(&line); err != nil {
				t.Fatal(err)
			}
			plan = append(plan, line)
		}
		rows.Close()
		extra["stats_queries.plan_"+name] = strings.Join(plan, "\n")
		t.Logf("plan of the %s read over stats.max_range:\n%s", name, strings.Join(plan, "\n"))
	}
	env.Record(t, nil, extra)
}

// seconds is d as PostgreSQL interval text.
func seconds(d time.Duration) string {
	return strconv.FormatFloat(d.Seconds(), 'f', -1, 64) + " seconds"
}
