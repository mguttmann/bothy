// SPDX-License-Identifier: Apache-2.0

package pg

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/pressly/goose/v3"
)

// migrationName is the file name of an embedded migration: a five digit sequential version and
// a lower-case slug (ADR 0007 rule 4; tools/ci/migrations_lint.py holds the same rule for the
// repository). Timestamp versions, which goose would accept, fail it.
var migrationName = regexp.MustCompile(`^(\d{5})_([a-z0-9_]+)\.sql$`)

// envsubAnnotation switches on goose's substitution of ${VAR} from the process environment at
// migrate time. A migration must not depend on the environment (02 "Configuration loading" item
// 5), so Validate refuses it before goose's parser would read a variable. goose strips every
// leading "--" and compares annotations in any case, so this pattern does the same.
var envsubAnnotation = regexp.MustCompile(`(?im)^\s*(?:--\s*)+\+goose\s+envsub\b`)

// migrationFile is one embedded migration.
type migrationFile struct {
	version int64
	name    string // the slug after the version, for example "base_privileges"
	file    string // the file name in the file system
}

// listMigrations returns the *.sql files at the root of fsys in version order and checks their
// names and numbering: the pattern of migrationName, versions 1 to n without gaps or
// duplicates, and at least one migration. Other files are ignored, as goose ignores them.
func listMigrations(fsys fs.FS) ([]migrationFile, error) {
	names, err := fs.Glob(fsys, "*.sql")
	if err != nil {
		return nil, fmt.Errorf("pg: list migrations: %w", err)
	}
	var problems []string
	var files []migrationFile
	for _, n := range names {
		m := migrationName.FindStringSubmatch(n)
		if m == nil {
			problems = append(problems, fmt.Sprintf("%s: the name must match %s", n, migrationName))
			continue
		}
		v, _ := strconv.ParseInt(m[1], 10, 64) // five digits always parse
		if v < 1 {
			problems = append(problems, fmt.Sprintf("%s: versions start at 1", n))
			continue
		}
		files = append(files, migrationFile{version: v, name: m[2], file: n})
	}
	slices.SortFunc(files, func(a, b migrationFile) int { return compareVersions(a.version, b.version) })
	next := int64(1)
	for i, f := range files {
		switch {
		case i > 0 && f.version == files[i-1].version:
			problems = append(problems, fmt.Sprintf("duplicate version %d (%s and %s)", f.version, files[i-1].file, f.file))
		case f.version != next:
			for missing := next; missing < f.version; missing++ {
				problems = append(problems, fmt.Sprintf("version %d is missing", missing))
			}
		}
		next = f.version + 1
	}
	if len(names) == 0 {
		problems = append(problems, "no migrations")
	}
	if len(problems) > 0 {
		return nil, errors.New("pg: invalid migrations: " + strings.Join(problems, "; "))
	}
	return files, nil
}

func compareVersions(a, b int64) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	default:
		return 0
	}
}

// Validate checks the embedded migrations offline, without a database (bothy migrate validate,
// and NewMigrator before it touches one): the file names of migrationName, versions contiguous
// from 1 without duplicates, no goose ENVSUB annotation, and every file parses with goose's own
// SQL parser (annotations, StatementBegin and StatementEnd pairs, a "-- +goose Up" section).
// It does not check the SQL itself; PostgreSQL does that when the migration runs.
//
// The error lists every problem it found, each naming the file.
func Validate(fsys fs.FS) error {
	files, err := listMigrations(fsys)
	if err != nil {
		return err
	}
	var problems []string
	for _, f := range files {
		if err := parseMigration(fsys, f); err != nil {
			problems = append(problems, err.Error())
		}
	}
	if len(problems) > 0 {
		return errors.New("pg: invalid migrations: " + strings.Join(problems, "; "))
	}
	return nil
}

// parseMigration parses one file with goose. goose v3 has no public parser, but a provider parses
// every migration it is about to apply before it runs any of them; over a driver that refuses
// every statement, a parse error is returned as such, and a file that parses ends at the first
// refused statement (stubDriver).
func parseMigration(fsys fs.FS, f migrationFile) error {
	content, err := fs.ReadFile(fsys, f.file)
	if err != nil {
		return fmt.Errorf("%s: %w", f.file, err)
	}
	if envsubAnnotation.Match(content) {
		return fmt.Errorf("%s: the annotation +goose ENVSUB is not allowed: a migration must not read the environment", f.file)
	}
	if !strings.Contains(string(content), "-- +goose Up") {
		return fmt.Errorf("%s: the annotation \"-- +goose Up\" is missing", f.file)
	}
	db := sql.OpenDB(stubConnector{})
	defer func() { _ = db.Close() }()
	p, err := goose.NewProvider(goose.DialectPostgres, db, singleFileFS{fsys: fsys, name: f.file},
		goose.WithDisableVersioning(true), goose.WithDisableGlobalRegistry(true))
	if err != nil {
		return fmt.Errorf("%s: %w", f.file, err)
	}
	_, err = p.Up(context.Background())
	var partial *goose.PartialError
	if err == nil || (errors.As(err, &partial) && errors.Is(partial.Err, errStub)) {
		return nil
	}
	return fmt.Errorf("%s: %w", f.file, err)
}

// singleFileFS is fsys narrowed to one file, so that a provider sees exactly that migration.
type singleFileFS struct {
	fsys fs.FS
	name string
}

// Open opens name when it is the one file, and the root directory.
func (s singleFileFS) Open(name string) (fs.File, error) {
	if name != s.name && name != "." {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrNotExist}
	}
	return s.fsys.Open(name)
}

// Glob returns the one file when it matches pattern.
func (s singleFileFS) Glob(pattern string) ([]string, error) {
	ok, err := path.Match(pattern, s.name)
	if err != nil || !ok {
		return nil, err
	}
	return []string{s.name}, nil
}

// errStub is what every statement over stubDriver returns.
var errStub = errors.New("pg: statement refused by the validation driver")

// stubConnector connects to stubConn, which refuses every statement and transaction, so goose
// can parse but never execute (parseMigration).
type stubConnector struct{}

// Connect returns a stubConn.
func (stubConnector) Connect(context.Context) (driver.Conn, error) { return stubConn{}, nil }

// Driver returns the stub driver.
func (stubConnector) Driver() driver.Driver { return stubDriver{} }

type stubDriver struct{}

// Open returns a stubConn.
func (stubDriver) Open(string) (driver.Conn, error) { return stubConn{}, nil }

type stubConn struct{}

// Prepare refuses.
func (stubConn) Prepare(string) (driver.Stmt, error) { return nil, errStub }

// Close succeeds.
func (stubConn) Close() error { return nil }

// Begin refuses.
func (stubConn) Begin() (driver.Tx, error) { return nil, errStub }
