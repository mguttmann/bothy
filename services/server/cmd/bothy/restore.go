// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/OWNER/bothy/services/server/internal/wire"
)

// restoreUsage is the help of "bothy restore" (docs/spec/02-architecture.md "Commands of
// the binary", docs/spec/16-operations.md 8.4, docs/operations/backup-restore.md 5.2 step 6).
const restoreUsage = `Usage: bothy restore finalize [--relocation FILE] [--config FILE]

finalize  ends a restore before api and worker start again. It connects as database.user, which
          must be the schema owner (bothy_owner, the role of bothy migrate), needs the
          configuration and secrets of the server (it builds its modules and reads the object
          storage), and refuses while another session of bothy_app is connected to the
          database: stop api and worker first. First it applies the deletion journal: every
          message, file and account deleted after the backup is deleted again through the
          normal deletion; an error stops here and the command can run again. Then, in one
          transaction, it sets a new instance.sync_epoch, moves every agent consumer, webhook
          subscription and worker consumer to head_seq, raises the permission version, stores
          the next number of the journal and the identity of the restored database
          (instance.pg_system_identifier and pg_timeline_id) and writes an audit entry; on an
          error there nothing of it changes. A server whose database identity differs from the
          stored one stays not ready (restore_not_finalized) until this command ran, unless
          database.restore_guard is off.

Output on stdout:
  sync epoch <old> -> <new>, cursors at head_seq <n>
  database identity <system identifier> timeline <timeline id>
  deletion journal: <n> entries read, deleted again <n> messages, <n> files, <n> accounts;
    <n> skipped, <n> refused (last owner), <n> lost; next number <n>
  permission version <n>

--relocation FILE  after a relocation without data loss: FILE is the manifest of the final
                   backup (backup_manifest.v1). Only when event_log.head_seq, audit.last_seq,
                   audit.last_hash and sync_epoch of the restored database equal the manifest,
                   it stores the new identity and keeps the epoch and every cursor
                   ("sync epoch <e> kept, cursors unchanged at head_seq <n>"). Any difference
                   exits 1, names the field and changes nothing; then run the command without
                   --relocation.
--config FILE      configuration file, not empty (default: BOTHY_CONFIG_FILE, else none)

Exit codes: 0 done, 1 error or refused, 2 usage or configuration error.
`

// subcommandFinalize is the only subcommand of bothy restore.
const subcommandFinalize = "finalize"

// restoreOptions holds the flags of restore finalize.
type restoreOptions struct {
	file, relocation string
}

// restoreCommand runs "bothy restore finalize [--relocation FILE] [--config FILE]" with the
// arguments after "restore" and the process environment env, and returns the exit code: 0 done,
// 1 a failure or a refusal (a connected server, a manifest that differs or is invalid, an
// unreachable database), 2 a usage error or a configuration that does not load (02 "Commands of
// the binary"). Every reason goes to stderr without a secret.
func restoreCommand(ctx context.Context, args, env []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		return restoreUsageError(stderr, "")
	}
	if isHelpArg(args[0]) {
		_, _ = io.WriteString(stdout, restoreUsage)
		return exitOK
	}
	if args[0] != subcommandFinalize {
		return restoreUsageError(stderr, fmt.Sprintf("unknown subcommand %q", printable(args[0])))
	}
	o, err := parseRestoreFlags(args[1:])
	if errors.Is(err, flag.ErrHelp) {
		_, _ = io.WriteString(stdout, restoreUsage)
		return exitOK
	}
	if err != nil {
		return restoreUsageError(stderr, err.Error())
	}
	manifest, err := readManifest(o.relocation)
	if err != nil {
		return restoreFailf(stderr, "%v", err)
	}
	s, values, ok := loadConfig(env, wire.ConfigSchema, configOptions{file: o.file}, stderr)
	if !ok {
		return exitUsage
	}
	r, err := wire.NewRestoreFinalize(ctx, values, env, commandLogger(stderr, s, values))
	if err != nil {
		return restoreFailf(stderr, "%v", err)
	}
	defer r.Close(context.WithoutCancel(ctx))
	res, err := r.Finalize(ctx, manifest)
	if err != nil {
		return restoreFailf(stderr, "%v", err)
	}
	printRestoreResult(stdout, res)
	return exitOK
}

// printRestoreResult prints the outcome of a finalize on stdout: epoch and cursors, the database
// identity, the counts of the deletion journal (never an ID, M7-07) and the permission version.
func printRestoreResult(stdout io.Writer, res wire.RestoreResult) {
	if res.Relocation {
		_, _ = fmt.Fprintf(stdout, "sync epoch %s kept, cursors unchanged at head_seq %d\n", res.SyncEpoch, res.HeadSeq)
	} else {
		_, _ = fmt.Fprintf(stdout, "sync epoch %s -> %s, cursors at head_seq %d\n", res.PreviousSyncEpoch, res.SyncEpoch, res.HeadSeq)
	}
	_, _ = fmt.Fprintf(stdout, "database identity %s timeline %d\n", res.Identity.SystemIdentifier, res.Identity.TimelineID)
	j := res.Journal
	_, _ = fmt.Fprintf(stdout, "deletion journal: %d entries read, deleted again %d messages, %d files, %d accounts; %d skipped, %d refused (last owner), %d lost; next number %d\n",
		j.Entries, j.Messages, j.Files, j.Accounts, j.Skipped, j.Refused, j.Lost, j.Next)
	_, _ = fmt.Fprintf(stdout, "permission version %d\n", res.PermissionVersion)
}

// readManifest reads and checks the manifest of --relocation before the database is touched, so a
// wrong path or an invalid manifest changes nothing and starts no module (M7-07); nil without the
// option.
func readManifest(path string) ([]byte, error) {
	if path == "" {
		return nil, nil
	}
	manifest, err := os.ReadFile(path) // #nosec G304 -- the operator names the file
	if err != nil {
		return nil, fmt.Errorf("read the manifest: %w", err)
	}
	if _, err := wire.ParseRelocationManifest(manifest); err != nil {
		return nil, err
	}
	return manifest, nil
}

// parseRestoreFlags parses the flags of restore finalize. It returns flag.ErrHelp for -h and
// --help; the message of any other error is the line restoreUsageError prints.
func parseRestoreFlags(args []string) (restoreOptions, error) {
	fs := flag.NewFlagSet("bothy restore finalize", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var o restoreOptions
	fs.StringVar(&o.file, "config", "", "configuration file")
	fs.StringVar(&o.relocation, "relocation", "", "manifest of the final backup")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return restoreOptions{}, err
		}
		return restoreOptions{}, errors.New(printable(err.Error()))
	}
	if fs.NArg() > 0 {
		return restoreOptions{}, fmt.Errorf("unexpected argument %q", printable(fs.Arg(0)))
	}
	if o.file == "" && flagGiven(fs, "config") {
		return restoreOptions{}, errors.New("--config needs a file path")
	}
	if o.relocation == "" && flagGiven(fs, "relocation") {
		return restoreOptions{}, errors.New("--relocation needs a manifest file")
	}
	return o, nil
}

// restoreUsageError prints msg (when set) and the usage on stderr and returns the usage exit
// code.
func restoreUsageError(stderr io.Writer, msg string) int {
	if msg != "" {
		_, _ = fmt.Fprintf(stderr, "bothy restore: %s\n", msg)
	}
	_, _ = io.WriteString(stderr, restoreUsage)
	return exitUsage
}

// restoreFailf prints one error line on stderr and returns exit code 1.
func restoreFailf(stderr io.Writer, format string, args ...any) int {
	_, _ = fmt.Fprintf(stderr, "bothy restore: "+format+"\n", args...)
	return exitFailure
}
