// SPDX-License-Identifier: Apache-2.0

package agents_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/OWNER/bothy/services/server/internal/platform/testkit"
)

// checkTable is the table of a check kind, the order of its rows (03 names) and, for a table
// that holds more than a check may read, the JSON object of the readable columns.
type checkTable struct {
	table, order string
	// object is the SQL of the row object; empty means every column (to_jsonb(t)).
	object string
}

// checkTables maps every check kind of brief Design note 4 to its table.
var checkTables = map[string]checkTable{
	"run":         {"runs", "queued_at, id", ""},
	"runs":        {"runs", "queued_at, id", ""},
	"message":     {"messages", "created_at, id", ""},
	"messages":    {"messages", "created_at, id", ""},
	"followup":    {"run_followups", "created_at, message_id", ""},
	"component":   {"message_components", "message_id, position", ""},
	"interaction": {"interactions", "created_at, id", ""},
	"session":     {"agent_sessions", "created_at, id", ""},
	"thread":      {"threads", "id", ""},
	"events":      {"event_log", "seq", ""},
	"audit":       {"audit_events", "seq", ""},
	"consumer":    {"agent_consumer_cursors", "agent_id, consumer_name", ""},
	"commands":    {"commands", "created_at, id", ""},
	// The key table: only kid, purpose, state and the times, never a key reference.
	"signing_keys": {"server_signing_keys", "created_at, kid",
		"jsonb_build_object('kid', t.kid, 'purpose', t.purpose, 'state', t.state, 'created_at', t.created_at, 'not_after', t.not_after)"},
	// The file table: the states of the pipeline and the transcript, never its text or the
	// original name (brief M3-09 Design note 20 a).
	"file": {"files", "created_at, id", fileCheckObject()},
}

// fileCheckColumns are the columns the check file reads besides id.
var fileCheckColumns = []string{"upload_state", "scan_status", "transcript_status", "transcript_source"}

// fileCheckObject is the row object of the check file: id and fileCheckColumns.
func fileCheckObject() string {
	parts := []string{"'id', t.id"}
	for _, c := range fileCheckColumns {
		parts = append(parts, "'"+c+"', t."+c)
	}
	return "jsonb_build_object(" + strings.Join(parts, ", ") + ")"
}

// jtiColumns are compared only by their hash prefix (brief Boundaries: never a token in output).
var jtiColumns = []string{"invocation_jti", "started_jti", "renewed_from_jti"}

// rowQuerier is the read-only database of the checks: AdminDB, or a fake in the unit tests.
type rowQuerier interface {
	rows(ctx context.Context, sql string, args ...any) ([]map[string]any, error)
}

// pgQuerier reads rows through a pgx pool as JSON objects.
type pgQuerier struct {
	q interface {
		Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	}
}

func (p pgQuerier) rows(ctx context.Context, sql string, args ...any) ([]map[string]any, error) {
	rs, err := p.q.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rs.Close()
	var out []map[string]any
	for rs.Next() {
		var raw []byte
		if err := rs.Scan(&raw); err != nil {
			return nil, err
		}
		var m map[string]any
		if err := json.Unmarshal(raw, &m); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rs.Err()
}

// jtiHash is the hash prefix the Fake-Agent reports for a token (first 16 hex characters of the
// SHA-256 of the jti).
func jtiHash(jti string) string {
	s := sha256.Sum256([]byte(jti))
	return hex.EncodeToString(s[:])[:16]
}

// checkQuery builds the read-only SQL of a check: every row of the table as a JSON object,
// filtered by id (single checks) or by the members of where (dotted paths into JSON columns).
func checkQuery(kind, id string, where map[string]any) (string, []any, error) {
	ct, ok := checkTables[kind]
	if !ok {
		return "", nil, fmt.Errorf("unknown check %s", kind)
	}
	var conds []string
	var args []any
	if id != "" {
		args = append(args, id)
		conds = append(conds, fmt.Sprintf("(to_jsonb(t)->>'id') = $%d", len(args)))
	}
	keys := make([]string, 0, len(where))
	for k := range where {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		path := "'{" + strings.ReplaceAll(k, ".", ",") + "}'"
		if strings.ContainsAny(k, "'\\{}") {
			return "", nil, fmt.Errorf("bad where key %q", k)
		}
		v := where[k]
		if s, ok := v.(string); ok && s == "<absent>" {
			conds = append(conds, fmt.Sprintf("(to_jsonb(t) #>> %s) IS NULL", path))
			continue
		}
		args = append(args, scalarText(v))
		conds = append(conds, fmt.Sprintf("(to_jsonb(t) #>> %s) = $%d", path, len(args)))
	}
	object := ct.object
	if object == "" {
		object = "to_jsonb(t)"
	}
	sql := "SELECT " + object + " FROM " + ct.table + " t"
	if len(conds) > 0 {
		sql += " WHERE " + strings.Join(conds, " AND ")
	}
	return sql + " ORDER BY " + ct.order, args, nil
}

// readCheck reads the rows of a check and replaces the jti columns by their hash prefix.
func readCheck(ctx context.Context, q rowQuerier, kind, id string, where map[string]any) ([]map[string]any, error) {
	sql, args, err := checkQuery(kind, id, where)
	if err != nil {
		return nil, err
	}
	rows, err := q.rows(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	for _, r := range rows {
		for _, c := range jtiColumns {
			if s, ok := r[c].(string); ok {
				r[c] = jtiHash(s)
			}
		}
	}
	return rows, nil
}

// evalCheck compares the rows of a check with its expectation: a single check (an id) compares
// the members of the one row; a set check compares count, none and the rows (partial, in order).
func (c *conductor) evalCheck(kind string, single bool, rows []map[string]any, e Expect) error {
	if single {
		if len(rows) != 1 {
			return &stepError{fmt.Sprintf("check %s: expected one row, observed %d", kind, len(rows))}
		}
		return c.compare(e, answer(rows[0]))
	}
	if e.None && len(rows) != 0 {
		return &stepError{fmt.Sprintf("check %s: expected no row, observed %d", kind, len(rows))}
	}
	if e.Count != nil && len(rows) != *e.Count {
		return &stepError{fmt.Sprintf("check %s: expected %d rows, observed %d %s", kind, *e.Count, len(rows), rowsBrief(rows))}
	}
	if len(e.Rows) > 0 {
		if len(rows) < len(e.Rows) {
			return &stepError{fmt.Sprintf("check %s: expected at least %d rows, observed %d", kind, len(e.Rows), len(rows))}
		}
		for i, want := range e.Rows {
			resolved, err := c.resolveMap(want)
			if err != nil {
				return err
			}
			for k, v := range resolved {
				got, ok := lookup(rows[i], k)
				if err := matchValue(fmt.Sprintf("row %d %s", i+1, k), v, got, ok); err != nil {
					return fmt.Errorf("check %s: %w; observed %s", kind, err, rowsBrief(rows[i:i+1]))
				}
			}
		}
	}
	if len(e.Fields) > 0 && len(rows) > 0 {
		return c.compare(Expect{Fields: e.Fields}, answer(rows[len(rows)-1]))
	}
	return nil
}

// check runs a check step through AdminDB; with eventually it polls until it holds.
func (c *conductor) check(st *Step) error {
	q := c.db
	id := ""
	if st.ID != "" {
		v, err := c.resolve(st.ID)
		if err != nil {
			return err
		}
		id = fmt.Sprint(v)
	}
	where, err := c.resolveMap(st.Where)
	if err != nil {
		return err
	}
	ctx := testkit.Context(c.t)
	for {
		// The reads run on the test's context, so the last comparison survives the deadline.
		rows, err := readCheck(c.t.Context(), q, st.Check, id, where)
		if err != nil {
			return err
		}
		err = c.evalCheck(st.Check, id != "", rows, *st.Expect)
		if err == nil {
			return c.keep(st.Keep, firstRow(rows))
		}
		if !st.Eventually {
			return err
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("never held: %w", err)
		case <-time.After(20 * time.Millisecond):
		}
	}
}

// firstRow returns the first row of a check as an answer for keep.
func firstRow(rows []map[string]any) answer {
	if len(rows) == 0 {
		return answer{}
	}
	return answer(rows[0])
}

// rowsBrief renders the first rows of a failed set check (scalars only, shortened, redacted).
func rowsBrief(rows []map[string]any) string {
	var parts []string
	for i, r := range rows {
		if i == 3 {
			parts = append(parts, "...")
			break
		}
		keep := map[string]any{}
		for k, v := range r {
			switch v.(type) {
			case map[string]any, []any:
				if k != "details" && k != "payload" {
					continue
				}
			}
			if strings.HasSuffix(k, "search") || strings.HasSuffix(k, "vector") || k == "content_md" {
				continue
			}
			keep[k] = v
		}
		parts = append(parts, short(keep))
	}
	return strings.Join(parts, " ")
}
