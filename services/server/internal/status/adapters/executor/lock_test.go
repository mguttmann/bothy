// SPDX-License-Identifier: Apache-2.0

package executor_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/status/adapters/executor"
	"github.com/OWNER/bothy/services/server/internal/status/app"
)

// TestResticForgetWaitsForBackup: a prune of the local repository never overlaps a backup of
// this process. restic refuses a prune while another restic holds a lock, so a local forget after
// a logical run would fail an objects backup running at the same time, or the other way round.
// The fake backup blocks until the test releases it; the forget must start only after it ended.
func TestResticForgetWaitsForBackup(t *testing.T) {
	bin := t.TempDir()
	order := filepath.Join(bin, "order")
	release := filepath.Join(bin, "release")
	r := newFake(t, bin, "restic", `case "$*" in
*"cat config"*) exit 0;;
*" backup "*) echo begin-backup >> '`+order+`'
  i=0; while [ ! -e '`+release+`' ] && [ $i -lt 400 ]; do sleep 0.05; i=$((i+1)); done
  echo end-backup >> '`+order+`'
  echo '`+summaryLine+`';;
*" forget "*) echo forget >> '`+order+`';;
esac`)
	e := newExecutor(t, executor.Config{Restic: r.path, SecretsDir: secretsDir(t, false)})
	backupDone := make(chan error, 1)
	go func() {
		_, err := e.ResticBackup(context.Background(), app.ResticBackup{Paths: []string{"/srv/files"}, Tags: []string{"objects"}})
		backupDone <- err
	}()
	waitFor(t, func() bool { return strings.Contains(read(order), "begin-backup") })
	forgetDone := make(chan error, 1)
	go func() {
		forgetDone <- e.ResticForget(context.Background(), app.ResticForget{Tag: "logical", KeepWithin: time.Hour})
	}()
	// Give a forget that does not wait the chance to run before the backup ends.
	time.Sleep(300 * time.Millisecond)
	if err := os.WriteFile(release, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := <-backupDone; err != nil {
		t.Fatal(err)
	}
	if err := <-forgetDone; err != nil {
		t.Fatal(err)
	}
	if got := strings.Fields(read(order)); strings.Join(got, " ") != "begin-backup end-backup forget" {
		t.Errorf("order %q, want the forget after the end of the backup", got)
	}
}

// read returns the content of path, "" when it cannot be read.
func read(path string) string {
	b, err := os.ReadFile(path) //nolint:gosec // G304: a path of t.TempDir.
	if err != nil {
		return ""
	}
	return string(b)
}

// waitFor polls cond until it holds, failing after ten seconds.
func waitFor(t *testing.T, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatal("condition not reached")
		}
		time.Sleep(10 * time.Millisecond)
	}
}
