// SPDX-License-Identifier: Apache-2.0

package searchbench

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Index is an index of a search vector: name, definition (pg_get_indexdef) and size in bytes.
type Index struct {
	Name, Def string
	Bytes     int64
}

// Sizes are the sizes of design note 12 (16 section 13 "indexes (the load test measures their
// share)"): the table messages, the indexes on messages.search_vector and threads.title_search,
// the database, and the share of those indexes in the database.
type Sizes struct {
	MessagesTableBytes int64
	DatabaseBytes      int64
	VectorIndexes      []Index
	TitleIndexes       []Index
	IndexShare         float64
}

// indexesOn lists the indexes of table that contain column, by name.
const indexesOn = `SELECT c.relname, pg_get_indexdef(i.indexrelid), pg_relation_size(i.indexrelid)
FROM pg_index i JOIN pg_class c ON c.oid = i.indexrelid
WHERE i.indrelid = $1::regclass
  AND EXISTS (SELECT 1 FROM pg_attribute a WHERE a.attrelid = i.indrelid AND a.attnum = ANY(i.indkey) AND a.attname = $2)
ORDER BY c.relname`

// Querier is the admin pool or a transaction of the measured database.
type Querier interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// indexes returns the indexes of table on column.
func indexes(ctx context.Context, q Querier, table, column string) ([]Index, error) {
	rows, err := q.Query(ctx, indexesOn, table, column)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (Index, error) {
		var x Index
		err := r.Scan(&x.Name, &x.Def, &x.Bytes)
		return x, err
	})
}

// MeasureSizes reads the sizes of design note 12 through the admin pool.
func MeasureSizes(ctx context.Context, q Querier) (Sizes, error) {
	var s Sizes
	if err := q.QueryRow(ctx, `SELECT pg_table_size('messages'), pg_database_size(current_database())`).Scan(&s.MessagesTableBytes, &s.DatabaseBytes); err != nil {
		return s, err
	}
	var err error
	if s.VectorIndexes, err = indexes(ctx, q, "messages", "search_vector"); err != nil {
		return s, err
	}
	if s.TitleIndexes, err = indexes(ctx, q, "threads", "title_search"); err != nil {
		return s, err
	}
	if len(s.VectorIndexes) == 0 || len(s.TitleIndexes) == 0 {
		return s, fmt.Errorf("searchbench: %d indexes on messages.search_vector and %d on threads.title_search", len(s.VectorIndexes), len(s.TitleIndexes))
	}
	var sum int64
	for _, x := range append(slices.Clone(s.VectorIndexes), s.TitleIndexes...) {
		sum += x.Bytes
	}
	if s.DatabaseBytes > 0 {
		s.IndexShare = float64(sum) / float64(s.DatabaseBytes)
	}
	return s, nil
}

// Map returns the sizes for extra.search, absolute and per million of messages messages.
func (s Sizes) Map(messages int) map[string]any {
	perMillion := func(b int64) float64 { return float64(b) / (float64(messages) / 1e6) }
	idx := map[string]any{}
	for _, x := range append(slices.Clone(s.VectorIndexes), s.TitleIndexes...) {
		idx[x.Name] = map[string]any{"bytes": x.Bytes, "bytes_per_million": perMillion(x.Bytes)}
	}
	return map[string]any{"messages_table_bytes": s.MessagesTableBytes, "messages_table_bytes_per_million": perMillion(s.MessagesTableBytes),
		"database_bytes": s.DatabaseBytes, "database_bytes_per_million": perMillion(s.DatabaseBytes), "indexes": idx,
		"search_index_share_of_database": s.IndexShare}
}

// Rewrite is the measurement of a rewrite of messages.search_vector with its indexes (design
// note 12, 14 section 8, RK-38).
type Rewrite struct {
	Rows                                           int64
	RewriteSeconds, IndexBuildSeconds              float64
	CommitSeconds                                  float64
	RewriteSecondsPerMillion, IndexBuildPerMillion float64
	MaintenanceWorkMem                             string
	// PhraseMatches is the number of messages the fixed phrase finds, before and after.
	PhraseMatches int
}

// Map returns the measurement for extra.search.
func (r Rewrite) Map() map[string]any {
	return map[string]any{"rows": r.Rows, "rewrite_seconds": r.RewriteSeconds, "rewrite_seconds_per_million": r.RewriteSecondsPerMillion,
		"index_build_seconds": r.IndexBuildSeconds, "index_build_seconds_per_million": r.IndexBuildPerMillion,
		"commit_seconds": r.CommitSeconds, "maintenance_work_mem": r.MaintenanceWorkMem, "phrase_matches": r.PhraseMatches}
}

// vector is the catalog state of messages.search_vector: expression, nullability, type and indexes.
type vector struct {
	expr, typ string
	notNull   bool
	indexes   []Index
}

// readVector reads the generation expression of messages.search_vector (pg_get_expr over
// pg_attrdef), its nullability and type and the definitions of its indexes, never from a
// migration.
func readVector(ctx context.Context, q Querier) (vector, error) {
	var v vector
	err := q.QueryRow(ctx, `SELECT pg_get_expr(d.adbin, d.adrelid), a.attnotnull, format_type(a.atttypid, a.atttypmod)
		FROM pg_attribute a JOIN pg_attrdef d ON d.adrelid = a.attrelid AND d.adnum = a.attnum
		WHERE a.attrelid = 'messages'::regclass AND a.attname = 'search_vector' AND a.attgenerated = 's' AND NOT a.attisdropped`).Scan(&v.expr, &v.notNull, &v.typ)
	if err != nil {
		return v, fmt.Errorf("searchbench: reading the expression of messages.search_vector: %w", err)
	}
	if v.indexes, err = indexes(ctx, q, "messages", "search_vector"); err != nil {
		return v, err
	}
	if len(v.indexes) == 0 {
		return v, errors.New("searchbench: messages.search_vector has no index")
	}
	return v, nil
}

// phraseIDs returns the ids of the messages the phrase finds through search_tsquery over langs,
// as the query construction of 14 4.2 builds it.
func phraseIDs(ctx context.Context, q Querier, langs []string, phrase string) ([]string, error) {
	rows, err := q.Query(ctx, `SELECT id::text FROM messages WHERE search_vector @@ search_tsquery($1::text[]::regconfig[], $2) ORDER BY id`, langs, phrase)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowTo[string])
}

// MeasureRewrite times the rewrite of messages.search_vector as a release that changes its
// expression would run it in a maintenance window (design note 12), through the admin pool of a
// database no server uses any more: it saves expression, nullability and index definitions from
// the catalog, then in one transaction drops the column (catalog only, removes its indexes), adds
// it again GENERATED ALWAYS AS the saved expression STORED (one rewrite of the table), and builds
// each saved index without CONCURRENTLY (no writers in a window). Afterwards expression and index
// definitions must equal the saved ones and phrase, queried with search_tsquery over langs, must
// find the same messages as before. The error reports a failed step or check without content.
func MeasureRewrite(ctx context.Context, db *pgxpool.Pool, langs []string, phrase string) (Rewrite, error) {
	var out Rewrite
	before, err := readVector(ctx, db)
	if err != nil {
		return out, err
	}
	idsBefore, err := phraseIDs(ctx, db, langs, phrase)
	if err != nil {
		return out, err
	}
	if len(idsBefore) == 0 {
		return out, errors.New("searchbench: the fixed phrase finds no message before the rewrite")
	}
	if err := db.QueryRow(ctx, `SELECT count(*), current_setting('maintenance_work_mem') FROM messages`).Scan(&out.Rows, &out.MaintenanceWorkMem); err != nil {
		return out, err
	}
	if err := rewriteVector(ctx, db, before, &out); err != nil {
		return out, err
	}
	millions := float64(out.Rows) / 1e6
	out.RewriteSecondsPerMillion, out.IndexBuildPerMillion = out.RewriteSeconds/millions, out.IndexBuildSeconds/millions
	after, err := readVector(ctx, db)
	if err != nil {
		return out, err
	}
	if err := sameVector(before, after); err != nil {
		return out, err
	}
	idsAfter, err := phraseIDs(ctx, db, langs, phrase)
	if err != nil {
		return out, err
	}
	if !slices.Equal(idsBefore, idsAfter) {
		return out, fmt.Errorf("searchbench: the fixed phrase finds %d messages after the rewrite, %d before", len(idsAfter), len(idsBefore))
	}
	out.PhraseMatches = len(idsAfter)
	return out, nil
}

// rewriteVector drops and adds messages.search_vector as v describes it and builds its indexes in
// one transaction, and records the times in out.
func rewriteVector(ctx context.Context, db *pgxpool.Pool, v vector, out *Rewrite) error {
	tx, err := db.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	notNull := ""
	if v.notNull {
		notNull = " NOT NULL"
	}
	t0 := time.Now()
	if _, err := tx.Exec(ctx, `ALTER TABLE messages DROP COLUMN search_vector`); err != nil {
		return fmt.Errorf("searchbench: dropping messages.search_vector: %w", err)
	}
	// DDL takes no parameters: type and expression are what the catalog of this throwaway
	// database printed for the column (format_type, pg_get_expr), never input of a request.
	if _, err := tx.Exec(ctx, fmt.Sprintf(`ALTER TABLE messages ADD COLUMN search_vector %s%s GENERATED ALWAYS AS (%s) STORED`, v.typ, notNull, v.expr)); err != nil { // nosemgrep: bothy-go-sql-string-building -- DDL of the load test from the catalog of its throwaway database
		return fmt.Errorf("searchbench: adding messages.search_vector again: %w", err)
	}
	out.RewriteSeconds = time.Since(t0).Seconds()
	t0 = time.Now()
	for _, x := range v.indexes {
		if _, err := tx.Exec(ctx, x.Def); err != nil {
			return fmt.Errorf("searchbench: building %s: %w", x.Name, err)
		}
	}
	out.IndexBuildSeconds = time.Since(t0).Seconds()
	t0 = time.Now()
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	out.CommitSeconds = time.Since(t0).Seconds()
	return nil
}

// sameVector reports a difference of expression, nullability, type or index definitions between
// the catalog state before and after the rewrite.
func sameVector(before, after vector) error {
	if after.expr != before.expr || after.notNull != before.notNull || after.typ != before.typ {
		return errors.New("searchbench: the expression, nullability or type of messages.search_vector changed in the rewrite")
	}
	if len(after.indexes) != len(before.indexes) {
		return fmt.Errorf("searchbench: %d indexes on messages.search_vector after the rewrite, %d before", len(after.indexes), len(before.indexes))
	}
	for k := range after.indexes {
		if after.indexes[k].Name != before.indexes[k].Name || after.indexes[k].Def != before.indexes[k].Def {
			return fmt.Errorf("searchbench: index %s changed in the rewrite", before.indexes[k].Name)
		}
	}
	return nil
}

// FixedPhrase returns the phrase of the rewrite check: the first two words of message 0 of the
// corpus, which opens the first post and is never deleted.
func (c *Corpus) FixedPhrase() string {
	w := Words(c.Message(0).Text)
	return fmt.Sprintf("%q", w[0]+" "+w[1])
}
