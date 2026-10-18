// SPDX-License-Identifier: Apache-2.0

package pg

import "testing"

// TestOperationOf: the operation and the table of a statement come from its first keyword (the
// main statement after a WITH) and the table it reads or writes, with comments, schemas and
// quotes removed; a statement without a table has none.
func TestOperationOf(t *testing.T) {
	t.Parallel()
	for sql, want := range map[string][2]string{
		"SELECT * FROM instance":                                                  {"SELECT", "instance"},
		"-- name: X :one\n/* c */ select a from public.Users u":                   {"SELECT", "users"},
		"INSERT INTO messages (a) VALUES ($1)":                                    {"INSERT", "messages"},
		"UPDATE ONLY sequence_counters SET value = value + $1":                    {"UPDATE", "sequence_counters"},
		"DELETE FROM \"event_log\" WHERE seq < $1":                                {"DELETE", "event_log"},
		"WITH a AS (SELECT 1 FROM x), b AS (DELETE FROM y) UPDATE z SET":          {"UPDATE", "z"},
		"WITH RECURSIVE t(n) AS (SELECT 1 UNION SELECT n FROM t) SELECT n FROM t": {"SELECT", "t"},
		"SELECT (SELECT 1 FROM inner_t), 'FROM no' FROM outer_t":                  {"SELECT", "outer_t"},
		"SELECT pg_notify('event_log', $1)":                                       {"SELECT", ""},
		"begin":                                                                   {"BEGIN", ""},
		"SET LOCAL statement_timeout = 5":                                         {"SET", ""},
		"COPY files FROM STDIN":                                                   {"COPY", "files"},
		"VACUUM":                                                                  {"VACUUM", ""},
		"":                                                                        {"", ""},
	} {
		op, table := operationOf(sql)
		if op != want[0] || table != want[1] {
			t.Errorf("operationOf(%q) = %q, %q; want %q, %q", sql, op, table, want[0], want[1])
		}
	}
}
