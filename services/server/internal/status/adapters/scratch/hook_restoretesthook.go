// SPDX-License-Identifier: Apache-2.0

//go:build restoretesthook

package scratch

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/OWNER/bothy/services/server/internal/platform/pg"
)

// The test hook of the image test (brief M7-03 design note 3), compiled only with the build tag
// restoretesthook, which no release build sets: services/server/test/postgresimage builds its own
// bothy-backupd with it. The variable RESTORE_TEST_HOOK names one manipulation of the copy before
// the checks; it has no BOTHY_ prefix, so the configuration never sees it. Every run also writes
// /proc/net/tcp and /proc/net/tcp6 while the copy runs to probeFile, outside the scratch
// directory, for the check that the copy has no TCP listener.
const (
	hookVariable = "RESTORE_TEST_HOOK"
	probeFile    = "/tmp/restore-test-tcp"
	// The manipulations.
	hookEventGap   = "event_log_gap"
	hookAuditHash  = "audit_hash"
	hookObjectByte = "object_bytes"
)

// hook returns the manipulation of this run.
func hook() string {
	return os.Getenv(hookVariable) //nolint:forbidigo // test-only build: the image test selects the manipulation
}

// afterStart records the TCP sockets of the container and applies the database manipulations:
// one event_log row in the middle of its partition deleted, or the stored hash of one
// audit_events row in the middle of the chain changed (with the protection trigger off, as an
// attacker with the superuser would).
func afterStart(ctx context.Context, _ string, pool *pg.Pool) error {
	var probe []byte
	for _, f := range []string{"/proc/net/tcp", "/proc/net/tcp6"} {
		b, err := os.ReadFile(f) // #nosec G304 -- fixed kernel files
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			return err
		}
		probe = append(probe, b...)
	}
	if err := os.WriteFile(probeFile, probe, 0o600); err != nil { // #nosec G703 -- a fixed path of the test build
		return err
	}
	switch hook() {
	case hookEventGap:
		_, err := pool.Exec(ctx, `DELETE FROM event_log WHERE seq = (SELECT (min(seq) + max(seq)) / 2 FROM event_log)`)
		return err
	case hookAuditHash:
		_, err := pool.Exec(ctx, `ALTER TABLE audit_events DISABLE TRIGGER audit_events_no_update_delete;
			UPDATE audit_events SET hash = sha256(hash) WHERE seq = (SELECT (min(seq) + max(seq)) / 2 FROM audit_events);
			ALTER TABLE audit_events ENABLE TRIGGER audit_events_no_update_delete`)
		return err
	}
	return nil
}

// afterFetch replaces the bytes of the first restored object (in name order) with other bytes.
func afterFetch(_ context.Context, target string) error {
	if hook() != hookObjectByte {
		return nil
	}
	var first string
	err := filepath.WalkDir(target, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if first == "" && d.Type().IsRegular() {
			first = p
		}
		return nil
	})
	if err != nil || first == "" {
		return errors.New("scratch: no restored object to change")
	}
	return os.WriteFile(first, []byte("changed-EXAMPLE"), 0o600)
}
