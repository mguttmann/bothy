// SPDX-License-Identifier: Apache-2.0

package migrations_test

import (
	"reflect"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/platform/pg/pgtest"
)

// TestReactionsPinsMigration_Schema holds reactions and pins against docs/spec/03-data-model.md
// (tables reactions and pins) column by column, with their keys and the index of the pin list
// (M1-59).
func TestReactionsPinsMigration_Schema(t *testing.T) {
	t.Parallel()
	conn := pgtest.New(t).SuperConn(t)
	for _, c := range []struct {
		table string
		cols  []principalColumn
		cons  []string
		idx   []string
	}{
		{
			table: "reactions",
			cols: []principalColumn{
				{"message_id", "uuid", false, ""},
				{"principal_id", "uuid", false, ""},
				{"emoji", "text", false, ""},
				{"created_at", "timestamp with time zone", false, "now()"},
			},
			cons: []string{
				"FOREIGN KEY (message_id) REFERENCES messages(id) ON DELETE CASCADE",
				"FOREIGN KEY (principal_id) REFERENCES principals(id) ON DELETE RESTRICT",
				"PRIMARY KEY (message_id, principal_id, emoji)",
			},
			idx: []string{
				"CREATE UNIQUE INDEX reactions_pkey ON public.reactions USING btree (message_id, principal_id, emoji)",
			},
		},
		{
			table: "pins",
			cols: []principalColumn{
				{"message_id", "uuid", false, ""},
				{"thread_id", "uuid", false, ""},
				{"pinned_by", "uuid", false, ""},
				{"pinned_at", "timestamp with time zone", false, "now()"},
			},
			cons: []string{
				"FOREIGN KEY (message_id) REFERENCES messages(id) ON DELETE CASCADE",
				"FOREIGN KEY (pinned_by) REFERENCES principals(id) ON DELETE RESTRICT",
				"FOREIGN KEY (thread_id) REFERENCES threads(id) ON DELETE CASCADE",
				"PRIMARY KEY (message_id)",
			},
			idx: []string{
				"CREATE INDEX pins_thread_pinned_at_idx ON public.pins USING btree (thread_id, pinned_at DESC)",
				"CREATE UNIQUE INDEX pins_pkey ON public.pins USING btree (message_id)",
			},
		},
	} {
		if got := principalColumns(t, conn, c.table); !reflect.DeepEqual(got, c.cols) {
			t.Errorf("%s columns:\n got %+v\nwant %+v", c.table, got, c.cols)
		}
		if got := principalConstraints(t, conn, c.table); !reflect.DeepEqual(got, c.cons) {
			t.Errorf("%s constraints:\n got %q\nwant %q", c.table, got, c.cons)
		}
		if got := indexes(t, conn, c.table); !reflect.DeepEqual(got, c.idx) {
			t.Errorf("%s indexes:\n got %q\nwant %q", c.table, got, c.idx)
		}
	}
}
