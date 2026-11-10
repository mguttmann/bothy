// SPDX-License-Identifier: Apache-2.0

package postgres_test

import (
	"context"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/chat"
	"github.com/OWNER/bothy/services/server/internal/chat/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

var (
	// updateThreads finds the start of an UPDATE threads statement in a query file.
	updateThreads = regexp.MustCompile(`(?i)\bUPDATE\s+threads\b`)
	// setsTitle and setsTitleVersion find the assignment of the column in a statement; neither
	// matches title_source or the other one.
	setsTitle        = regexp.MustCompile(`\btitle\s*=`)
	setsTitleVersion = regexp.MustCompile(`\btitle_version\s*=`)
)

// titleWritesWithoutVersion returns the start of every UPDATE threads in sql whose statement (up
// to the next ";") assigns title without assigning title_version.
func titleWritesWithoutVersion(sql string) []string {
	var out []string
	for _, m := range updateThreads.FindAllStringIndex(sql, -1) {
		stmt := sql[m[0]:]
		if end := strings.Index(stmt, ";"); end >= 0 {
			stmt = stmt[:end]
		}
		if setsTitle.MatchString(stmt) && !setsTitleVersion.MatchString(stmt) {
			out = append(out, strings.SplitN(stmt, "\n", 2)[0])
		}
	}
	return out
}

// TestTitleWritesSetTitleVersion holds the rule of threads.title_version for every writer of this
// module (FIX-TITLE-EDIT-412, 03 threads): each UPDATE threads in queries/*.sql that writes title
// writes title_version too, so the precondition of setThreadTitle never accepts a version from
// before a title change.
func TestTitleWritesSetTitleVersion(t *testing.T) {
	t.Parallel()
	if got := titleWritesWithoutVersion("-- name: X :exec\nUPDATE threads SET title = @title, title_source = @s, version = @v\nWHERE id = @id;\n"); len(got) != 1 {
		t.Fatalf("the check accepts a title write without title_version: %v", got)
	}
	if got := titleWritesWithoutVersion("UPDATE threads SET title = '', title_version = version + 1 WHERE id = @id;" +
		"UPDATE threads SET title_source = @s, version = version + 1 WHERE id = @id;"); len(got) != 0 {
		t.Fatalf("the check refuses a statement that follows the rule: %v", got)
	}
	files, err := filepath.Glob(filepath.Join("queries", "*.sql"))
	if err != nil || len(files) == 0 {
		t.Fatalf("no query files: %v", err)
	}
	writes := 0
	for _, f := range files {
		data, err := os.ReadFile(f) // #nosec G304 -- a file of this package's queries directory.
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range updateThreads.FindAllStringIndex(string(data), -1) {
			if stmt, _, _ := strings.Cut(string(data)[m[0]:], ";"); setsTitle.MatchString(stmt) {
				writes++
			}
		}
		for _, stmt := range titleWritesWithoutVersion(string(data)) {
			t.Errorf("%s: %q writes title without title_version", f, stmt)
		}
	}
	if writes == 0 {
		t.Fatal("no title write found; the check reads the wrong files")
	}
}

// TestRepositoryTitleVersion (FIX-TITLE-EDIT-412): a new thread has title_version 1; messages and
// a change of the state raise version and leave it; a title change makes the new version the
// title version; the mirror change of an import names the version the batch ends with, and the
// purge of a deleted thread its own new version.
func TestRepositoryTitleVersion(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	ctx := context.Background()
	alex, _ := e.person(t)
	space, forum := ids.New(), ids.New()
	e.exec(t, `INSERT INTO spaces (id, name, sort_key, created_by) VALUES ($1, 'Support', 'a0', $2)`, space.String(), alex.String())
	e.exec(t, `INSERT INTO channels (id, space_id, kind, name, sort_key, history_backfill_limit, created_by)
		VALUES ($1, $2, 'forum', 'bugs', 'a0', 20, $3)`, forum.String(), space.String(), alex.String())
	stored := func(id ids.ID) (version, titleVersion int64) {
		t.Helper()
		th, found, err := e.repo.Thread(ctx, id)
		if err != nil || !found {
			t.Fatalf("thread %v %v", found, err)
		}
		return th.Version, th.TitleVersion
	}
	var th domain.Thread
	if err := e.write(t, func(ctx context.Context) error {
		var err error
		th, err = e.repo.InsertThread(ctx, domain.Thread{ID: ids.New(), ChannelID: forum, Kind: domain.ThreadKindThread, Title: "Neuer Chat",
			TitleSource: domain.TitleAuto, State: domain.ThreadOpen, CreatedBy: alex, CreatedAt: t0, LastActivityAt: t0, Version: 1,
			SearchLanguage: "simple"})
		if err != nil {
			return err
		}
		for range 2 {
			if th, err = e.repo.AppendToThread(ctx, th.ID, false, t0.Add(time.Minute)); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if th.Version != 3 || th.TitleVersion != 1 {
		t.Fatalf("after two messages: version %d, title version %d", th.Version, th.TitleVersion)
	}
	change := func(fn func(th *domain.Thread)) {
		t.Helper()
		if err := e.write(t, func(ctx context.Context) error {
			locked, found, err := e.repo.LockThread(ctx, th.ID)
			if err != nil || !found {
				t.Errorf("lock %v %v", found, err)
				return err
			}
			fn(&locked)
			return e.repo.SaveThread(ctx, locked)
		}); err != nil {
			t.Fatal(err)
		}
	}
	change(func(th *domain.Thread) { th.SetTitle("Export fails", domain.TitleManual) })
	if version, titleVersion := stored(th.ID); version != 4 || titleVersion != 4 {
		t.Fatalf("after the title change: version %d, title version %d", version, titleVersion)
	}
	change(func(th *domain.Thread) {
		if _, err := th.Close(alex, t0.Add(time.Hour)); err != nil {
			t.Error(err)
		}
	})
	if version, titleVersion := stored(th.ID); version != 5 || titleVersion != 4 {
		t.Fatalf("after the close: version %d, title version %d", version, titleVersion)
	}

	// The mirror change of an import writes no version; the end of its batch raises it once.
	mirror := func(title string) {
		t.Helper()
		if err := e.write(t, func(ctx context.Context) error {
			locked, _, err := e.repo.LockThread(ctx, th.ID)
			if err != nil {
				return err
			}
			locked.Title = title
			if err := e.repo.SaveImportedThread(ctx, locked); err != nil {
				return err
			}
			_, err = e.repo.FinishImportBatch(ctx, th.ID)
			return err
		}); err != nil {
			t.Fatal(err)
		}
	}
	mirror("Export fails")
	if version, titleVersion := stored(th.ID); version != 6 || titleVersion != 4 {
		t.Fatalf("after a mirror change without the title: version %d, title version %d", version, titleVersion)
	}
	mirror("Export fails in the source")
	if version, titleVersion := stored(th.ID); version != 7 || titleVersion != 7 {
		t.Fatalf("after a mirror change of the title: version %d, title version %d", version, titleVersion)
	}

	e.exec(t, `UPDATE threads SET deleted_at = $2, deleted_by = $3 WHERE id = $1`, th.ID.String(), t0, alex.String())
	if err := e.write(t, func(ctx context.Context) error {
		return e.repo.FinishPurge(ctx, chat.TargetThread, th.ID, t0.Add(time.Hour))
	}); err != nil {
		t.Fatal(err)
	}
	if version, titleVersion := stored(th.ID); version != 8 || titleVersion != 8 {
		t.Fatalf("after the purge: version %d, title version %d", version, titleVersion)
	}
}
