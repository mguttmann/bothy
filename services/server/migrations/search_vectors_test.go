// SPDX-License-Identifier: Apache-2.0

package migrations_test

import (
	"context"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/OWNER/bothy/services/server/internal/platform/pg"
	"github.com/OWNER/bothy/services/server/internal/platform/pg/pgtest"
)

// TestSearchColumnsOnPostgres18 holds the search columns of 00035 and the indexes of 00036
// against the catalog of the real PostgreSQL 18 (docs/spec/14-suche.md 3.2, 03 section 5): the
// vectors are stored generated columns, not virtual ones, the languages are regconfig NOT NULL
// DEFAULT 'simple', and both GIN indexes are valid. This holds the row "Migrations-Lint" of 14
// section 12 against the artifact itself.
func TestSearchColumnsOnPostgres18(t *testing.T) {
	t.Parallel()
	conn := pgtest.New(t).SuperConn(t)
	ctx := context.Background()
	var version int
	if err := conn.QueryRow(ctx, "SELECT current_setting('server_version_num')::int").Scan(&version); err != nil {
		t.Fatal(err)
	}
	if version/10000 != 18 {
		t.Fatalf("server_version_num %d is not PostgreSQL 18", version)
	}
	for _, c := range []struct{ table, column string }{{"messages", "search_vector"}, {"threads", "title_search"}} {
		var generated, typ string
		var notNull bool
		err := conn.QueryRow(ctx, `SELECT a.attgenerated::text, a.attnotnull, format_type(a.atttypid, a.atttypmod)
			FROM pg_attribute a WHERE a.attrelid = $1::regclass AND a.attname = $2 AND NOT a.attisdropped`, c.table, c.column).
			Scan(&generated, &notNull, &typ)
		if err != nil {
			t.Fatalf("%s.%s: %v", c.table, c.column, err)
		}
		if generated != "s" || !notNull || typ != "tsvector" {
			t.Errorf("%s.%s: attgenerated %q, not null %v, type %s; want stored, not null, tsvector", c.table, c.column, generated, notNull, typ)
		}
	}
	for _, table := range []string{"messages", "threads"} {
		var typ, def string
		var notNull bool
		err := conn.QueryRow(ctx, `SELECT format_type(a.atttypid, a.atttypmod), a.attnotnull, pg_get_expr(d.adbin, d.adrelid)
			FROM pg_attribute a JOIN pg_attrdef d ON d.adrelid = a.attrelid AND d.adnum = a.attnum
			WHERE a.attrelid = $1::regclass AND a.attname = 'search_language'`, table).Scan(&typ, &notNull, &def)
		if err != nil {
			t.Fatalf("%s.search_language: %v", table, err)
		}
		if typ != "regconfig" || !notNull || def != "'simple'::regconfig" {
			t.Errorf("%s.search_language: type %s, not null %v, default %s", table, typ, notNull, def)
		}
	}
	for _, ix := range []struct{ name, table, column string }{
		{"messages_search_vector_idx", "messages", "search_vector"},
		{"threads_title_search_idx", "threads", "title_search"},
	} {
		var am, table, def string
		var valid bool
		err := conn.QueryRow(ctx, `SELECT am.amname, i.indrelid::regclass::text, i.indisvalid, pg_get_indexdef(i.indexrelid)
			FROM pg_index i JOIN pg_class c ON c.oid = i.indexrelid JOIN pg_am am ON am.oid = c.relam
			WHERE c.relname = $1`, ix.name).Scan(&am, &table, &valid, &def)
		if err != nil {
			t.Fatalf("index %s: %v", ix.name, err)
		}
		if am != "gin" || table != ix.table || !valid || !strings.HasSuffix(def, "USING gin ("+ix.column+")") {
			t.Errorf("index %s: access method %s on %s, valid %v, %s", ix.name, am, table, valid, def)
		}
	}
}

// tsquery returns search_tsquery(langs, q) as text, or nil for NULL, called through q.
func tsquery(t testing.TB, q interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}, langs string, input *string) *string {
	t.Helper()
	var out *string
	if err := q.QueryRow(context.Background(), "SELECT search_tsquery($1::text::regconfig[], $2)::text", langs, input).Scan(&out); err != nil {
		t.Fatalf("search_tsquery(%s, %v): %v", langs, input, err)
	}
	return out
}

func ptr(s string) *string { return &s }

// TestSearchTsqueryExamples runs every example of design note 5 of the brief (docs/spec/14-suche.md
// 4.1 and 4.2) as bothy_app on PostgreSQL 18 and compares the exact text form.
func TestSearchTsqueryExamples(t *testing.T) {
	t.Parallel()
	app := pgtest.New(t).Pool(t, pg.RoleApp).PGX()
	for _, c := range []struct {
		langs string
		q     *string
		want  *string
	}{
		{"{german,english}", ptr("Rechnungen"), ptr("'rechnung' | 'rechnungen' | 'rechnungen'")},
		// The English sub-query holds only a stop word and is dropped.
		{"{german,english}", ptr("the"), ptr("'the' | 'the'")},
		{"{german,english}", ptr("!!! ..."), nil},
		{"{german,english}", ptr(""), nil},
		{"{german,english}", ptr(" \t\n "), nil},
		{"{german,english}", nil, nil},
		{"{}", ptr("Rechnungen"), ptr("'rechnungen'")},
		{"{german,english}", ptr("-z"), ptr("!'z' | !'z' | !'z'")},
	} {
		got := tsquery(t, app, c.langs, c.q)
		if (got == nil) != (c.want == nil) || (got != nil && *got != *c.want) {
			t.Errorf("search_tsquery(%s, %s) = %s, want %s", c.langs, show(c.q), show(got), show(c.want))
		}
	}
	// A NULL array behaves like the empty one: only simple.
	var got *string
	if err := app.QueryRow(context.Background(), "SELECT search_tsquery(NULL, 'Rechnungen')::text").Scan(&got); err != nil {
		t.Fatal(err)
	}
	if got == nil || *got != "'rechnungen'" {
		t.Errorf("search_tsquery(NULL, Rechnungen) = %s", show(got))
	}
}

func show(s *string) string {
	if s == nil {
		return "NULL"
	}
	return *s
}

// TestSearchTsqueryUsesIndex holds "die GIN-Abfrage nutzt den Index" of 14 4.2: with sequential
// scans off, the planner answers the search predicate of both vectors with a bitmap index scan on
// their GIN index.
func TestSearchTsqueryUsesIndex(t *testing.T) {
	t.Parallel()
	app := pgtest.New(t).Pool(t, pg.RoleApp).PGX()
	for _, c := range []struct{ query, index string }{
		{"SELECT id FROM messages WHERE search_vector @@ search_tsquery('{german,english}', 'Rechnungen')", "messages_search_vector_idx"},
		{"SELECT id FROM threads WHERE title_search @@ search_tsquery('{german,english}', 'Rechnungen')", "threads_title_search_idx"},
	} {
		plan := explain(t, app, c.query)
		if !strings.Contains(plan, "Bitmap Index Scan on "+c.index) {
			t.Errorf("%s:\n%s\nwant a bitmap index scan on %s", c.query, plan, c.index)
		}
	}
}

// explain returns the plan of query with sequential scans off, in a transaction it always rolls
// back (a test that stops with the transaction open would block the pool).
func explain(t *testing.T, app *pgxpool.Pool, query string) string {
	t.Helper()
	ctx := context.Background()
	tx, err := app.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, "SET LOCAL enable_seqscan = off"); err != nil {
		t.Fatal(err)
	}
	rows, err := tx.Query(ctx, "EXPLAIN "+query)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var plan []string
	for rows.Next() {
		var line string
		if err := rows.Scan(&line); err != nil {
			t.Fatal(err)
		}
		plan = append(plan, line)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return strings.Join(plan, "\n")
}

// TestSearchTsqueryPrivileges holds D-M3-13-9: the owner of search_tsquery is bothy_owner, and
// only bothy_app may execute it, not PUBLIC and not bothy_maint.
func TestSearchTsqueryPrivileges(t *testing.T) {
	t.Parallel()
	conn := pgtest.New(t).SuperConn(t)
	ctx := context.Background()
	const fn = "search_tsquery(regconfig[], text)"
	var owner string
	if err := conn.QueryRow(ctx, "SELECT pg_get_userbyid(proowner) FROM pg_proc WHERE oid = $1::regprocedure", fn).Scan(&owner); err != nil {
		t.Fatal(err)
	}
	if owner != "bothy_owner" {
		t.Errorf("owner %s, want bothy_owner", owner)
	}
	for role, want := range map[string]bool{"bothy_app": true, "public": false, "bothy_maint": false} {
		var got bool
		if err := conn.QueryRow(ctx, "SELECT has_function_privilege($1, $2, 'EXECUTE')", role, fn).Scan(&got); err != nil {
			t.Fatal(err)
		}
		if got != want {
			t.Errorf("EXECUTE for %s = %v, want %v", role, got, want)
		}
	}
	var acl string
	if err := conn.QueryRow(ctx, "SELECT coalesce(proacl::text, '') FROM pg_proc WHERE oid = $1::regprocedure", fn).Scan(&acl); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(acl, "=X/") && !strings.Contains(acl, "bothy_app=X/") && !strings.Contains(acl, "bothy_owner=X/") {
		t.Errorf("acl %s grants EXECUTE beyond bothy_app", acl)
	}
}

// TestTextSearchCaseFolding confirms that the pgtest database folds the case of non-ASCII letters
// in text search: simple turns "Ärger" into 'ärger' (the start check of design note 9 asks the
// same of every production database).
func TestTextSearchCaseFolding(t *testing.T) {
	t.Parallel()
	app := pgtest.New(t).Pool(t, pg.RoleApp).PGX()
	var got string
	if err := app.QueryRow(context.Background(), "SELECT to_tsvector('simple', $1)::text", "\u00c4rger").Scan(&got); err != nil {
		t.Fatal(err)
	}
	if want := "'\u00e4rger':1"; got != want {
		t.Errorf("to_tsvector(simple, Ärger) = %s, want %s", got, want)
	}
}

// FuzzSearchTsquery holds 14 4.1 "jede Eingabe ist sicher" for the query function (PLAN M3 test
// strategy "Fuzzing der Sucheingabe"): for any valid text the call never fails, and the result is
// NULL or a tsquery with nodes. The round trip tq::text to ::tsquery inside the function loses
// nothing: the result equals the OR of the same non-empty sub-queries joined with the tsquery
// operator || (no text in between), and its own text form is stable under a second parse.
//
// tsquery equality compares trees, so the literal property "r::text::tsquery = r" fails for an
// input like "a or b": the text form of a nested OR has no parentheses and parses back
// left-associated, the same query with another tree (reported in the M3-13 report). Invalid UTF-8
// and NUL cannot reach PostgreSQL text; the search coordinator (M3-14) rejects them before SQL.
func FuzzSearchTsquery(f *testing.F) {
	for _, seed := range []string{
		"Rechnungen", `"quoted phrase"`, `it's`, `'`, `''`, `\`, `\\'`, `a\'b`, "a & b", "a | b", "!a", "a <-> b",
		"a:*", "a:*B", "(a | b) & c", "a or b", "or", "a - b", "-a", "--a", "- -", "-", `"-a"`, `" "`, `"a" or -"b c"`,
		"\u00a0word\u2003word\u3000", "\u200fRTL\u202e mark\u2066", "e\u0301cole", "A\u030a", "\u00c4rger \u00fcber \u00dcbel",
		strings.Repeat("Donaudampfschifffahrt", 40), "ERR_CONN_RESET", "a:1", "a:A", "{x}", "a\tb\nc", "\U0001f600 emoji",
		"Rechnung -offen", "'x':1", "1 2 3", "a&!b|c", "<->", ":*", "!", "()", "\"", "x or (y z) or -w", `"a b" "c d" or e`,
	} {
		f.Add(seed)
	}
	app := pgtest.New(f).Pool(f, pg.RoleApp).PGX()
	f.Fuzz(func(t *testing.T, q string) {
		if !utf8.ValidString(q) || strings.ContainsRune(q, 0) {
			t.Skip("PostgreSQL text holds neither invalid UTF-8 nor NUL")
		}
		var (
			isNull, same, stable bool
			nodes                *int
			text                 *string
		)
		err := app.QueryRow(context.Background(), fuzzQuery, q).Scan(&isNull, &nodes, &text, &same, &stable)
		if err != nil {
			t.Fatalf("search_tsquery(%q): %v", q, err)
		}
		if !same {
			t.Errorf("search_tsquery(%q) = %s differs from the OR of its sub-queries", q, show(text))
		}
		if isNull {
			return
		}
		if nodes == nil || *nodes <= 0 {
			t.Errorf("search_tsquery(%q) = %s with %v nodes", q, show(text), nodes)
		}
		if !stable {
			t.Errorf("search_tsquery(%q) = %s: its text form changes under a second parse", q, show(text))
		}
	})
}

// fuzzQuery returns for $1: whether search_tsquery is NULL, its node count and text, whether it
// equals the OR of the non-empty sub-queries folded with || (both NULL counts as equal), and
// whether its text form is stable under a second parse.
const fuzzQuery = `WITH RECURSIVE
  s AS (SELECT ord, websearch_to_tsquery(l, $1) AS tq
        FROM unnest(array_append('{german,english}'::regconfig[], 'simple'::regconfig)) WITH ORDINALITY AS u(l, ord)),
  k AS (SELECT row_number() OVER (ORDER BY ord) AS n, tq FROM s WHERE numnode(tq) > 0),
  f(n, acc) AS (SELECT n, tq FROM k WHERE n = 1 UNION ALL SELECT k.n, f.acc || k.tq FROM f JOIN k ON k.n = f.n + 1),
  folded AS (SELECT (SELECT acc FROM f ORDER BY n DESC LIMIT 1) AS acc),
  r AS (SELECT search_tsquery('{german,english}', $1) AS r)
SELECT r IS NULL, numnode(r), r::text, r IS NOT DISTINCT FROM acc, coalesce(r::text::tsquery::text = r::text, true)
FROM r, folded`
