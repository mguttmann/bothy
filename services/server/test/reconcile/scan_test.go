// SPDX-License-Identifier: Apache-2.0

package reconcile_test

import (
	"context"
	"crypto/rand"
	"math/big"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

// scanQuery finds the base tables of the schemas $2 whose rows contain the text $1: one static
// statement over pg_class that renders every table with query_to_xml and searches the text with
// strpos (design note 14). No SQL is built in Go; the canary is alphanumeric, so the XML escaping
// cannot hide it.
const scanQuery = `SELECT n.nspname || '.' || c.relname
FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace
WHERE c.relkind = 'r' AND n.nspname = ANY ($2::text[])
  AND strpos(query_to_xml(format('SELECT * FROM %I.%I', n.nspname, c.relname), true, false, '')::text, $1) > 0
ORDER BY 1`

// scanDatabase returns the tables of the domain schema public and of the job schema jobs that hold
// canary in any row.
func scanDatabase(t *testing.T, db *pgxpool.Pool, canary string) []string {
	t.Helper()
	rows, err := db.Query(context.Background(), scanQuery, canary, []string{"public", "jobs"})
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatal(err)
		}
		out = append(out, name)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}

// wantScanFinds is the positive control of the scan: a value the database holds in public.runs
// is found, so an empty scan for a canary cannot pass for the wrong reason.
func wantScanFinds(t *testing.T, db *pgxpool.Pool, known string) {
	t.Helper()
	for _, table := range scanDatabase(t, db, known) {
		if table == "public.runs" {
			return
		}
	}
	t.Fatalf("the scan does not find %s in public.runs", known)
}

// alnum are the characters of a canary.
const alnum = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

// randomAlnum returns n random alphanumeric characters.
func randomAlnum(t *testing.T, n int) string {
	t.Helper()
	var b strings.Builder
	for range n {
		i, err := rand.Int(rand.Reader, big.NewInt(int64(len(alnum))))
		if err != nil {
			t.Fatal(err)
		}
		b.WriteByte(alnum[i.Int64()])
	}
	return b.String()
}
