// SPDX-License-Identifier: Apache-2.0

package migrations_test

import (
	"reflect"
	"slices"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/platform/pg/pgtest"
)

// TestAgentConsumerCursorsMigration_Schema holds agent_consumer_cursors against
// docs/spec/03-data-model.md column by column, with the primary key and the foreign key to agents
// with ON DELETE CASCADE (M2-07).
func TestAgentConsumerCursorsMigration_Schema(t *testing.T) {
	t.Parallel()
	conn := pgtest.New(t).SuperConn(t)
	ts := "timestamp with time zone"
	columns := []principalColumn{
		{"agent_id", "uuid", false, ""},
		{"consumer_name", "text", false, "'default'::text"},
		{"acked_seq", "bigint", false, ""},
		{"holder_session_id", "uuid", true, ""},
		{"created_at", ts, false, "now()"},
		{"updated_at", ts, false, "now()"},
	}
	constraints := []string{
		"FOREIGN KEY (agent_id) REFERENCES agents(principal_id) ON DELETE CASCADE",
		"PRIMARY KEY (agent_id, consumer_name)",
	}
	if got := principalColumns(t, conn, "agent_consumer_cursors"); !reflect.DeepEqual(got, columns) {
		t.Errorf("columns:\n got %+v\nwant %+v", got, columns)
	}
	got := principalConstraints(t, conn, "agent_consumer_cursors")
	slices.Sort(got)
	if !reflect.DeepEqual(got, constraints) {
		t.Errorf("constraints:\n got %q\nwant %q", got, constraints)
	}
}
