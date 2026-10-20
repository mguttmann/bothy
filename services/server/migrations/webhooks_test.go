// SPDX-License-Identifier: Apache-2.0

package migrations_test

import (
	"reflect"
	"slices"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/platform/pg"
	"github.com/OWNER/bothy/services/server/internal/platform/pg/pgtest"
)

// TestWebhookTablesMatchDataModel holds webhook_subscriptions and webhook_deliveries against
// docs/spec/03-data-model.md section 4.11 column by column, with the checks, the foreign keys
// with their actions, the unique key and the indexes, and the behaviour of the checks for
// bothy_app (M2-26 requirement 11).
func TestWebhookTablesMatchDataModel(t *testing.T) {
	t.Parallel()
	db := pgtest.New(t)
	conn := db.SuperConn(t)
	ts := "timestamp with time zone"
	subs := []principalColumn{
		{"id", "uuid", false, ""},
		{"app_installation_id", "uuid", true, ""},
		{"agent_id", "uuid", true, ""},
		{"url", "text", false, ""},
		{"event_types", "ARRAY", false, ""},
		{"signing_key_kid", "text", false, ""},
		{"hmac_enabled", "boolean", false, "false"},
		{"hmac_generation", "integer", false, "0"},
		{"hmac_rotated_at", ts, true, ""},
		{"state", "text", false, "'active'::text"},
		{"consecutive_failures", "integer", false, "0"},
		{"last_delivered_seq", "bigint", false, "0"},
		{"created_by", "uuid", false, ""},
		{"created_at", ts, false, "now()"},
		{"disabled_at", ts, true, ""},
	}
	deliveries := []principalColumn{
		{"id", "uuid", false, ""},
		{"subscription_id", "uuid", false, ""},
		{"event_seq", "bigint", false, ""},
		{"event_id", "uuid", false, ""},
		{"attempt", "integer", false, "0"},
		{"status", "text", false, "'pending'::text"},
		{"response_code", "smallint", true, ""},
		{"last_error", "text", true, ""},
		{"next_attempt_at", ts, true, ""},
		{"created_at", ts, false, "now()"},
		{"delivered_at", ts, true, ""},
	}
	if got := principalColumns(t, conn, "webhook_subscriptions"); !reflect.DeepEqual(got, subs) {
		t.Errorf("webhook_subscriptions columns:\n got %+v\nwant %+v", got, subs)
	}
	if got := principalColumns(t, conn, "webhook_deliveries"); !reflect.DeepEqual(got, deliveries) {
		t.Errorf("webhook_deliveries columns:\n got %+v\nwant %+v", got, deliveries)
	}
	for table, want := range map[string][]string{
		"webhook_subscriptions": {
			"CHECK (((app_installation_id IS NULL) <> (agent_id IS NULL)))",
			"CHECK ((hmac_enabled OR (hmac_generation = 0)))",
			"CHECK ((state = ANY (ARRAY['active'::text, 'paused'::text, 'disabled'::text])))",
			"FOREIGN KEY (agent_id) REFERENCES agents(principal_id) ON DELETE CASCADE",
			"FOREIGN KEY (app_installation_id) REFERENCES app_installations(id) ON DELETE CASCADE",
			"FOREIGN KEY (created_by) REFERENCES principals(id) ON DELETE RESTRICT",
			"FOREIGN KEY (signing_key_kid) REFERENCES server_signing_keys(kid) ON DELETE RESTRICT",
			"PRIMARY KEY (id)",
		},
		"webhook_deliveries": {
			"CHECK ((status = ANY (ARRAY['pending'::text, 'succeeded'::text, 'failed'::text, 'dead'::text])))",
			"FOREIGN KEY (subscription_id) REFERENCES webhook_subscriptions(id) ON DELETE CASCADE",
			"PRIMARY KEY (id)",
			"UNIQUE (subscription_id, event_seq)",
		},
	} {
		slices.Sort(want)
		got := principalConstraints(t, conn, table)
		slices.Sort(got)
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%s constraints:\n got %q\nwant %q", table, got, want)
		}
	}
	for table, want := range map[string][]string{
		"webhook_subscriptions": {
			"CREATE INDEX webhook_subscriptions_agent_idx ON public.webhook_subscriptions USING btree (agent_id) WHERE (agent_id IS NOT NULL)",
			"CREATE INDEX webhook_subscriptions_state_idx ON public.webhook_subscriptions USING btree (state)",
			"CREATE UNIQUE INDEX webhook_subscriptions_pkey ON public.webhook_subscriptions USING btree (id)",
		},
		"webhook_deliveries": {
			"CREATE INDEX webhook_deliveries_pending_next_idx ON public.webhook_deliveries USING btree (next_attempt_at) WHERE (status = 'pending'::text)",
			"CREATE UNIQUE INDEX webhook_deliveries_pkey ON public.webhook_deliveries USING btree (id)",
			"CREATE UNIQUE INDEX webhook_deliveries_subscription_id_event_seq_key ON public.webhook_deliveries USING btree (subscription_id, event_seq)",
		},
	} {
		if got := indexes(t, conn, table); !reflect.DeepEqual(got, want) {
			t.Errorf("%s indexes:\n got %q\nwant %q", table, got, want)
		}
	}

	app := db.Pool(t, pg.RoleApp)
	const robin = "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f9c01"
	const tracker = "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f9c02"
	const inst = "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f9c03"
	const sub = "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f9c04"
	mustExec(t, app, `INSERT INTO principals (id, kind, handle, display_name) VALUES ($1, 'user', 'robin', 'Robin')`, robin)
	mustExec(t, app, `INSERT INTO principals (id, kind, handle, display_name) VALUES ($1, 'app', 'tracker', 'Tracker')`, tracker)
	mustExec(t, app, `INSERT INTO apps (principal_id, slug, manifest, manifest_version, manifest_sha256) VALUES ($1, 'tracker', '{}', '1.0.0', '\x00')`, tracker)
	mustExec(t, app, `INSERT INTO app_installations (id, app_id, installed_by) VALUES ($1, $2, $3)`, inst, tracker, robin)
	mustExec(t, app, `INSERT INTO server_signing_keys (kid, purpose, alg, public_jwk, private_key_ref, state)
		VALUES ('whk-test', 'webhook_out', 'EdDSA', '{}', 'file:/run/whk', 'active')`)
	ins := `INSERT INTO webhook_subscriptions (id, app_installation_id, agent_id, url, event_types, signing_key_kid, hmac_enabled, hmac_generation, created_by)
		VALUES ($1, $2, $3, 'https://hooks.example.com/chat', '{message.created}', 'whk-test', $4, $5, $6)`
	wantCheckViolation(t, exec(app, ins, sub, nil, nil, false, 0, robin), "neither installation nor agent")
	wantCheckViolation(t, exec(app, ins, sub, inst, tracker, false, 0, robin), "installation and agent")
	wantCheckViolation(t, exec(app, ins, sub, inst, nil, false, 1, robin), "generation without hmac")
	mustExec(t, app, ins, sub, inst, nil, true, 1, robin)
	wantCheckViolation(t, exec(app, `UPDATE webhook_subscriptions SET state = 'gone' WHERE id = $1`, sub), "unknown state")
	dins := `INSERT INTO webhook_deliveries (id, subscription_id, event_seq, event_id, status) VALUES ($1, $2, $3, $4, $5)`
	mustExec(t, app, dins, "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f9d01", sub, 7, "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f9e01", "pending")
	wantState(t, exec(app, dins, "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f9d02", sub, 7, "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f9e01", "pending"), "23505", "second row of one event")
	wantCheckViolation(t, exec(app, dins, "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f9d03", sub, 8, "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f9e02", "lost"), "unknown status")
	mustExec(t, app, `DELETE FROM webhook_subscriptions WHERE id = $1`, sub)
	var left int
	if err := app.QueryRow(t.Context(), `SELECT count(*) FROM webhook_deliveries`).Scan(&left); err != nil || left != 0 {
		t.Fatalf("deliveries after deleting the subscription: %d %v", left, err)
	}
}
