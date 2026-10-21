# Package `platform/secrets/stack`

Status: built (M1-09). The normative sources are
[16-operations.md](../../../../../../docs/spec/16-operations.md) 5.3 "Secrets in the Compose stack" and
5.4 "Escrow" and the
[configuration reference](../../../../../../docs/operations/configuration.md) section 5
"Secrets"; if they differ, the chapter applies.

## Purpose

Preparation of the volume `secrets` for the Compose stack: `bothy secrets init` generates missing
secrets, takes over the files the owner supplied and creates one directory per service with
exactly the copies the service mounts through `subpath`. `bothy secrets export` writes the
canonical copies and the key store of the signing keys as an age-encrypted bundle for escrow.
Both commands read only their flags and the named directories: no configuration, no environment,
no database (the container `secrets-init` has none of these).

## Code

| File | Content |
|---|---|
| `catalog.go` | `Entry`, `Origin`, `Generator`, `Catalog`, `Services`, mapping of a path to its entry |
| `init.go` | `Init`, `InitOptions`, `Owner`, `ParseOwner`, `Report`, `Line`, `Status`, `ErrUsage`, `ErrConflict` |
| `export.go` | `Export`, `ExportOptions` |
| `fsutil.go` | walking without following links, reading with a check of the opened file, atomic writing, escape sequences in error messages |
| `fsutil_unix.go`, `fsutil_other.go` | reading the owner, directory `fsync`, the running user per platform |
| `lock_flock.go`, `lock_other.go` | lock on `<target>` with `flock`, abort without `flock` |
| `doc.go` | package description |

## Interface

- `Catalog()` returns the entries in the order of the table (a copy), `Services()` the services
  sorted and unique.
- `Init(ctx, InitOptions{Target, Source, Owners, AdoptSource, Rand})` returns a `Report`;
  `Report.WriteTo` writes one line per entry and, as the last line, `Report.Reminder`. On an error
  the report is empty.
- `ParseOwner("<service>=<uid>:<gid>")` checks only the form (a service of lower-case letters,
  digits and `-`, decimal numbers from 0 to `math.MaxInt32`) and never repeats the input; `Init`
  checks whether the service exists.
- `Export(ctx, ExportOptions{From, Recipients, Output, Armor, Stdout, IsTerminal})`.
- Errors that wrap `ErrUsage` are usage errors (exit code 2); all others, `ErrConflict` too, are
  exit code 1. `cmd/bothy` maps them to the exit codes.

## Catalogue

Every row of the table in configuration.md section 5 is an entry with name, origin and services;
`TestCatalogMatchesConfigurationDoc` compares both directions and the order,
`TestCatalogServicesMatchComposeSubpaths` the services with the `subpath` mounts in
`infra/compose/compose.yaml`.

- File entry: exactly one file (`db_app_password`).
- Directory entry, the name ends in `/`: every regular file below it, recursively (`offsite/`,
  `pgbackrest_tls/`).
- Pattern entry: every file whose relative path matches the anchored pattern
  (`oidc/<provider>_client_secret` with `^oidc/[a-z0-9_-]+_client_secret$`,
  `oidc/<provider>_client_key.pem` with `^oidc/[a-z0-9_-]+_client_key\.pem$`); in the report one
  line per file, without a file one line with the placeholder.
- Generators: `token` is 32 random bytes as base64url without padding (43 characters), `key` 32
  random bytes as lower-case hex (64 characters); the source is `crypto/rand`.
  `pgbackrest-tls` (M7-01, `pgbackrest_tls.go`) creates the directory `pgbackrest_tls/`: a
  self-signed CA limited to leaf certificates (`ca.crt`; its key is never written) and per host
  `postgres` and `backup` an ECDSA P-256 key in PKCS#8 (`<host>.key`) with a certificate for the
  host name with server and client use (`<host>.crt`), all valid for `PGBackRestTLSValidity`.
- `db_backup_password` (M7-02) is the password of the role `bothy_backup`: `migrate` sets it with
  `--bootstrap`, `backup` (bothy-backupd) logs in with it to record `backup_runs`.
- `pending`: the generator comes with a later ticket (`Entry.PendingOn`):
  `s3_access_key` and `s3_secret_key` (M3, supplied until then). If
  they are missing, `init` reports them as `pending` and ends with exit code 0; when supplied,
  they are taken over like every other entry.

## Store

- `<target>` has mode `0700` and the running user as its owner (`init` repairs both); if it does
  not exist, `init` creates it. An existing `<target>` must be empty (except for `lost+found`) or
  contain `_store`; otherwise `init` aborts with exit code 2 before it changes anything, so that a
  mistyped `--target` does not take over a foreign directory.
- `<target>/_store/` holds the one canonical copy of every entry: directories `0700`, files
  `0400`, the owner is the running user. Files without a catalogue entry stay untouched and are
  not copied; a file that is neither a directory nor a regular file aborts.
- `<target>/<service>/` for every service from `Services()`, also without a copy, because a
  `subpath` mount needs an existing directory. Expected are all entries in `_store` whose row
  names the service; everything else in the service directory is removed, links too (without
  following them).
- Relative names are the catalogue names everywhere; so a decrypted bundle has the layout of a
  source (plus `signing-keys/`, which `init` skips) ([disaster-recovery.md](../../../../../../docs/operations/disaster-recovery.md) case B).

## Reconciliation

Per entry between `_store` and `--source`:

| `_store` | Source | Generated entry | Supplied entry |
|---|---|---|---|
| missing | missing | generate (`generated`) | `missing`; with a pending generator (`Entry.PendingOn`: `s3_*`) `pending` |
| missing | present | take over (`adopted`) | take over (`adopted`) |
| present | missing | keep (`kept`) | keep (`kept`) |
| present | equal | keep (`kept`) | keep (`kept`) |
| present | different | conflict, exit code 1; with `--adopt-source` `updated` | replace (`updated`) |

A directory entry counts as equal when the same files with the same content are present. When
replacing, `init` first removes files that are missing in the source and directories in whose
place a file now stands, and writes afterwards; so `_store` also follows a path that turns from a
file into a directory in the source or the other way round.

Before the first write, `init` checks everything: options, source, owners, conflicts (all named
at once) and randomness. If a check fails, nothing is written, and `<target>` is not created
either.

- The source may contain only directories and regular files with a catalogue entry. A link, a
  device or a pipe aborts with exit code 2 (`init` runs as root over a directory of the owner and
  follows no link), and so does every unknown file: `.DS_Store` and similar files that a file
  manager creates abort with exit code 2 too. A file of a file or pattern entry without a value
  (empty or only a line end) aborts as well; below a directory entry (`offsite/`) a file may be
  empty. An empty value in `_store` aborts with exit code 1.
- A run holds an exclusive lock (`flock`) on `<target>` from before reading `_store` until the
  last write. If a second run, for example `docker compose run secrets-init` during `up`, finds it
  taken, it aborts with exit code 1 (`another secrets init is running on <target>`) before it
  reads or writes anything: otherwise it would remove the temporary files of the first run as
  leftovers and could replace a secret that was just generated. On systems without `flock`,
  `init` aborts.
- Writing is atomic: a temporary file `.bothy-tmp-<random>` in the same directory, write, `fsync`,
  `chmod`, `chown`, rename, `fsync` of the directory. Such files in the target are leftovers of an
  aborted run and are removed; in the source they are unknown files.
- A copy is written only when it is missing or its content, mode or owner differ; a differing
  mode or owner in `_store` is repaired. A second run without a change writes, renames and changes
  nothing, not even a modification time.
- Every file is read without blocking and only when the opened file is the same as during the
  walk; a link or a pipe swapped in the meantime is rejected.

## Owners and modes

| Service | Directories | Files | Owner |
|---|---|---|---|
| with `--owner <service>=<uid>:<gid>` | `0500` | `0400` | `uid:gid` |
| without `--owner` | `0755` | `0444` | root, without root the running user |

`server` and `migrate` need `--owner` (exit code 2), because the file driver reads only its own
files with `0400` or `0600` (SEC-SEC-1); the Compose stack passes `65532:65532`. An unknown
service or `_store` as a service is exit code 2. Without root, every owner must equal the running
user (otherwise exit code 1, nothing written); so the tests run without root. A directory that
changes is opened to `0700` for this and set to its mode afterwards. On systems without Unix
owners, `init` aborts instead of writing open copies.

## Output and fingerprint

One line per entry in catalogue order: `<status> <name> <fingerprint>`, for `missing` and
`pending` without a fingerprint; the last line (`Report.Reminder`) is the export command with the
`_store` of the run, for example
`back up the secrets: bothy secrets export --from /run/bothy/secrets/_store --age-recipient <age public key>`
(a path that is not a simple shell word in single quotes). The fingerprint is `sha256:` and the
first 12 hex characters of the SHA-256 of the content in `_store`; for a directory entry of the
text made of the lines `<path in the entry> <hex sha256 of the file>\n`, sorted by path. The
fingerprint suffices to compare two hosts; like every unsalted hash, though, it confirms a
guessed value with little entropy (a weak supplied password, a webhook URL), so the report, which
the log of the container `secrets-init` keeps as well, does not belong in public places. Values
never stand in the output and never in errors. Errors name paths; every non-printable character
of an error message of `Init` and `Export` appears as a Go escape sequence (`\n`), so a file name
of the owner below `offsite/` cannot fake a line in the terminal or the log.

## Exit codes

| Code | Case |
|---|---|
| 0 | success, also with `missing` and `pending` |
| 1 | runtime error, conflict of a generated entry, a foreign owner without root, another `init` on the same target, an existing output file on export |
| 2 | usage error (`ErrUsage`): no target, source not a directory, unknown or non-regular file in the source, invalid or missing owner, on export no or an invalid recipient, binary output to a terminal, `--from` not a directory |

## Export

- `--from` is a `_store` directory. The bundle is a POSIX tar (PAX) with relative names in sorted
  order, directories with a trailing `/` and included, mode bits as in `_store`, uid and gid 0
  without names, modification time 0. Leftovers `.bothy-tmp-*` are missing from the bundle; a
  link, device or pipe aborts. A file without a catalogue entry (for example when exporting the
  root of the volume instead of `_store`) is a usage error that names all of them before the
  output is created: `init --source` would otherwise reject the bundle at restore time.
- Encrypted with `filippo.io/age` to every `--age-recipient` (X25519, `age1...`); every recipient
  can decrypt it alone. With `--armor` in the ASCII armor of age.
- Recipients are checked before anything else. An invalid recipient is named only by its
  position, never by its text: a private key pasted by mistake (`AGE-SECRET-KEY-...`) must not
  end up in the terminal or a log.
- `--output -` (the default) writes to standard output and refuses binary output to a terminal
  without `--armor` (terminal means a character device). `--output <file>` creates the file
  exclusively with mode `0600`, never overwrites an existing one (exit code 1, file unchanged) and
  removes it again when the export fails.
- Plaintext exists only in memory; there are no temporary files.
- The key store of the signing keys (`--signing-key-dir`, default
  `/var/lib/bothy/signing-keys`, `secrets.signing_key_dir`, ADR 0066 rule 5) follows under the
  prefix `signing-keys/` (`KeyStorePrefix`): the directory and every regular file
  `<purpose>-<kid>.pem` with its mode. Leftovers `.bothy-tmp-*` of an interrupted write are
  skipped; a subdirectory, a link or another name stops the export (usage error, exit code 2),
  because a restore must not install a file the server never wrote. A key store named with
  `--signing-key-dir` that does not exist is a usage error; the missing default is skipped with
  one line on stderr (`signing key store <dir> not found; the bundle holds _store only`).

Decrypt and unpack for a restore:
`age --decrypt -i <identity> bundle.age | tar -x -C <source>`, then
`secrets init --adopt-source` (case B). `init` skips the subtree `signing-keys/` of the source;
its files belong to the volume of `secrets.signing_key_dir` (disaster-recovery.md case B).

## What is missing

- Generators for the generated form of
  `s3_access_key` and `s3_secret_key` (M3).

## Dependencies

The standard library, `filippo.io/age` and `filippo.io/age/armor` (register: `platform.secrets`
with the vendor group `age`). No other project packages, not `internal/platform/secrets` either;
the package computes the fingerprints itself.

## Chapters

[16-operations.md](../../../../../../docs/spec/16-operations.md),
[19-security.md](../../../../../../docs/spec/19-security.md),
[configuration.md](../../../../../../docs/operations/configuration.md),
[disaster-recovery.md](../../../../../../docs/operations/disaster-recovery.md)
