// SPDX-License-Identifier: Apache-2.0

package executor

import (
	"context"
	"path/filepath"
)

// PgBackRestRestore runs "pgbackrest restore" of the backup set label from repo1 into pgPath for
// the restore test (docs/operations/backup-restore.md 4 step 2, brief M7-03): recovery target
// immediate, so the copy ends at the consistency point of the backup, then promoted, so it
// accepts queries outside recovery; archive_mode off, so the copy never pushes a segment of its
// own timeline into the repository. The repository host restores as if it were the database host
// (--reset-pg1-host with --pg1-path); restore_command names the same options, so the copy fetches
// the WAL of the backup from the local repository. pgBackRest checks the checksum of every file it
// restores. The error is an *app.ExecError (pgbackrest_exit_<n>, pgbackrest_request_invalid).
func (e *Executor) PgBackRestRestore(ctx context.Context, label, pgPath string) error {
	if !labelPattern.MatchString(label) || pgPath == "" || !filepath.IsAbs(pgPath) {
		return execError(codePgBackRestRequestInvalid)
	}
	env, err := e.PgBackRestEnv(ctx)
	if err != nil {
		return err
	}
	scratch := []string{"--repo=1", "--reset-pg1-host", "--pg1-path=" + pgPath}
	restoreCommand := "pgbackrest --stanza=" + e.cfg.Stanza + " " + scratch[0] + " " + scratch[1] + " " + scratch[2] + ` archive-get %f "%p"`
	_, err = e.pgbackrest(ctx, env, append(scratch,
		"--set="+label, "--type=immediate", "--target-action=promote", "--archive-mode=off",
		"--recovery-option=restore_command="+restoreCommand, "restore")...)
	return err
}

// PgBackRestVerify runs "pgbackrest verify" on repo1: every file of the repository against its
// checksum (16 8.3 item 1). The error is an *app.ExecError.
func (e *Executor) PgBackRestVerify(ctx context.Context) error {
	env, err := e.PgBackRestEnv(ctx)
	if err != nil {
		return err
	}
	_, err = e.pgbackrest(ctx, env, "--repo=1", "verify")
	return err
}

// ResticRestore restores the files includes (absolute paths as stored) of the newest restic
// snapshot with tag into target for the file sample of the restore test (16 8.3 item 6). It reads
// the repository of the instance like ResticBackup and never changes it. The error is an
// *app.ExecError (restic_exit_<n>, restic_request_invalid).
func (e *Executor) ResticRestore(ctx context.Context, tag string, includes []string, target string) error {
	if tag == "" || len(includes) == 0 || target == "" || !filepath.IsAbs(target) {
		return execError(codeResticRequestInvalid)
	}
	repo, env, err := e.resticRepo(e.serviceKeysDir())
	if err != nil {
		return err
	}
	e.resticMu.RLock()
	defer e.resticMu.RUnlock()
	args := []string{"restore", "latest", "--tag", tag, "--target", target}
	for _, p := range includes {
		args = append(args, "--include", p)
	}
	_, err = e.restic(ctx, repo, env, nil, args...)
	return err
}
