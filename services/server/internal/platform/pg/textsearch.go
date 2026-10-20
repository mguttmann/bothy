// SPDX-License-Identifier: Apache-2.0

package pg

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// The queries of CheckTextSearch. missingConfigQuery returns the first position (1-based) in $1
// whose name is no text search configuration visible on the search path; schema-qualified
// catalog names keep a hostile search_path from shadowing the catalog. caseFoldingQuery is the
// probe of docs/DECISIONS.md D-M3-13-6: U+00C4 and U+00E4 (A with diaeresis, capital and small)
// give the same lexeme only when the character classification of the database knows non-ASCII
// letters.
const (
	missingConfigQuery = `SELECT n.i FROM unnest($1::text[]) WITH ORDINALITY AS n(name, i)
WHERE NOT EXISTS (SELECT 1 FROM pg_catalog.pg_ts_config c
                  WHERE c.cfgname = n.name AND pg_catalog.pg_ts_config_is_visible(c.oid))
ORDER BY n.i LIMIT 1`
	// language-check: off
	caseFoldingQuery = "SELECT pg_catalog.to_tsvector('simple', 'Ä') = pg_catalog.to_tsvector('simple', 'ä')"
	// language-check: on
)

// TextSearchConfigMissingError reports a configuration name that the database behind the pool
// does not have as a text search configuration on its search path: every message insert and every
// search with it would fail with a SQL error. It names the position, never the name, because the
// name is a configured value (02 "Configuration loading" item 4).
type TextSearchConfigMissingError struct {
	// Position is the 1-based position of the first missing name in the list CheckTextSearch got.
	Position int
}

// Error returns a message such as
// "pg: entry 2 of the text search configurations is not in the database".
func (e *TextSearchConfigMissingError) Error() string {
	return fmt.Sprintf("pg: entry %d of the text search configurations is not in the database", e.Position)
}

// ErrTextSearchCaseFolding reports a database whose character classification does not fold the
// case of non-ASCII letters in text search (for example LC_CTYPE C): a search for a word in lower
// case would never find it with a capital umlaut. The database needs a UTF-8 capable classification such as en_US.utf8
// (docs/spec/16-operations.md 7.2).
var ErrTextSearchCaseFolding = errors.New("pg: the database does not fold the case of non-ASCII letters in text search; " +
	"its character classification (LC_CTYPE) must be UTF-8 capable, for example en_US.utf8")

// CheckTextSearch is the start check of the full-text search (docs/spec/02-architecture.md "Start"
// step 4, docs/DECISIONS.md D-M3-13-6): every name in configs must be a text search
// configuration visible on the search path of the role of p, and the database must fold the case
// of non-ASCII letters. It runs two short queries on one connection and changes nothing.
//
// Errors: *TextSearchConfigMissingError for the first missing name, ErrTextSearchCaseFolding,
// the errors of Acquire (kind unavailable) or of a query. None carries a configured name.
//
// Example:
//
//	if err := pg.CheckTextSearch(ctx, pool, langs.Configs()); err != nil {
//		return err // the start ends with exit code 1
//	}
func CheckTextSearch(ctx context.Context, p *Pool, configs []string) error {
	c, err := p.Acquire(ctx)
	if err != nil {
		return err
	}
	defer c.Release()
	var pos int
	switch err := c.QueryRow(ctx, missingConfigQuery, configs).Scan(&pos); {
	case err == nil:
		return &TextSearchConfigMissingError{Position: pos}
	case !errors.Is(err, pgx.ErrNoRows):
		return fmt.Errorf("pg: read the text search configurations: %w", err)
	}
	var folds bool
	if err := c.QueryRow(ctx, caseFoldingQuery).Scan(&folds); err != nil {
		return fmt.Errorf("pg: probe the case folding of text search: %w", err)
	}
	if !folds {
		return ErrTextSearchCaseFolding
	}
	return nil
}
