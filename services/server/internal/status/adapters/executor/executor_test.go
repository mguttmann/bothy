// SPDX-License-Identifier: Apache-2.0

package executor_test

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/status/adapters/executor"
	"github.com/OWNER/bothy/services/server/internal/status/app"
)

// infoJSON is the output of pgbackrest info with two backups; the differential is the newest.
const infoJSON = `[{"name":"bothy","backup":[` +
	`{"label":"20261018-020000F","type":"full","timestamp":{"start":1760752800,"stop":1760753400},"info":{"size":900,"delta":900,"repository":{"size":300,"delta":300}}},` +
	`{"label":"20261018-020000F_20261019-020000D","type":"diff","timestamp":{"start":1760839200,"stop":1760839290},"info":{"size":950,"delta":50,"repository":{"size":320,"delta":20}}}` +
	`]},{"name":"other","backup":[{"label":"20261020-020000F","timestamp":{"stop":1760925600},"info":{"repository":{"delta":7}}}]}]`

// pgbackrestFake answers info with infoJSON and every other command with exit 0.
func pgbackrestFake(t *testing.T, dir string) fake {
	return newFake(t, dir, "pgbackrest", `case "$*" in *" info") printf '%s\n' '`+infoJSON+`';; esac`)
}

// repo2Env is the repo2 environment of exampleTarget with access and secret.
func repo2Env(access, secret string, ca string) map[string]string {
	env := map[string]string{
		"PGBACKREST_REPO2_TYPE": "s3", "PGBACKREST_REPO2_PATH": "/instance-1/pgbackrest",
		"PGBACKREST_REPO2_S3_BUCKET": "bothy-backup", "PGBACKREST_REPO2_S3_ENDPOINT": "s3.example.com",
		"PGBACKREST_REPO2_STORAGE_PORT": "9443", "PGBACKREST_REPO2_S3_REGION": "eu-example-1",
		"PGBACKREST_REPO2_S3_URI_STYLE": "path", "PGBACKREST_REPO2_S3_KEY": access,
		"PGBACKREST_REPO2_S3_KEY_SECRET": secret, "PGBACKREST_REPO2_CIPHER_TYPE": "aes-256-cbc",
		"PGBACKREST_REPO2_CIPHER_PASS": exampleCipherPass, "PGBACKREST_EXPIRE_AUTO": "n",
	}
	if ca != "" {
		env["PGBACKREST_REPO2_STORAGE_CA_FILE"] = ca
	}
	return env
}

// TestPgBackRestBackup_ArgsEnvAndInfo: backup then info as argument lists, the repo2
// environment with the keys of offsite/ and the CA file, nothing of the parent environment, and
// the newest backup of the stanza.
func TestPgBackRestBackup_ArgsEnvAndInfo(t *testing.T) {
	bin := t.TempDir()
	pg := pgbackrestFake(t, bin)
	secrets := secretsDir(t, true)
	e := newExecutor(t, executor.Config{
		PgBackRest: pg.path, SecretsDir: secrets, Offsite: exampleTarget(), OffsiteRegion: "eu-example-1", OffsitePathStyle: true,
	})
	res, err := e.PgBackRestBackup(context.Background(), app.PgBackRestBackup{Type: "diff", Repo: 2})
	if err != nil {
		t.Fatal(err)
	}
	if res.Label != "20261018-020000F_20261019-020000D" || res.SizeBytes != 20 {
		t.Errorf("result %+v", res)
	}
	calls := pg.calls(t)
	if len(calls) != 2 {
		t.Fatalf("%d calls, want 2", len(calls))
	}
	if want := []string{"--stanza=bothy", "--repo=2", "--type=diff", "backup"}; !slices.Equal(calls[0].args, want) {
		t.Errorf("backup args %q, want %q", calls[0].args, want)
	}
	if want := []string{"--stanza=bothy", "--repo=2", "--output=json", "info"}; !slices.Equal(calls[1].args, want) {
		t.Errorf("info args %q, want %q", calls[1].args, want)
	}
	for _, c := range calls {
		checkEnv(t, c, repo2Env(exampleAccessKey, exampleSecretKey, filepath.Join(secrets, "offsite", "ca.crt")))
	}
	env, err := e.PgBackRestEnv(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(env, "PGBACKREST_REPO2_S3_KEY="+exampleAccessKey) || !slices.Contains(env, "PGBACKREST_EXPIRE_AUTO=n") {
		t.Errorf("PgBackRestEnv %d entries without the repo2 key or expire-auto", len(env))
	}
}

// TestPgBackRest_LocalOnly: without an off-site target the environment holds only
// PGBACKREST_EXPIRE_AUTO=n, and repo 2 is refused without running the binary.
func TestPgBackRest_LocalOnly(t *testing.T) {
	bin := t.TempDir()
	pg := newFake(t, bin, "pgbackrest", `case "$*" in *" info") printf '%s\n' '`+strings.ReplaceAll(infoJSON, `"bothy"`, `"main"`)+`';; esac`)
	e := newExecutor(t, executor.Config{PgBackRest: pg.path, SecretsDir: secretsDir(t, false), Stanza: "main"})
	env, err := e.PgBackRestEnv(context.Background())
	if err != nil || !slices.Equal(env, []string{"PGBACKREST_EXPIRE_AUTO=n"}) {
		t.Fatalf("PgBackRestEnv = %q, %v", env, err)
	}
	if _, err := e.PgBackRestBackup(context.Background(), app.PgBackRestBackup{Type: "full", Repo: 1}); err != nil {
		t.Fatal(err)
	}
	calls := pg.calls(t)
	if len(calls) != 2 || calls[0].args[0] != "--stanza=main" || calls[0].args[2] != "--type=full" {
		t.Fatalf("calls %+v", calls)
	}
	checkEnv(t, calls[0], map[string]string{"PGBACKREST_EXPIRE_AUTO": "n"})
	_, err = e.PgBackRestBackup(context.Background(), app.PgBackRestBackup{Type: "full", Repo: 2})
	wantCode(t, err, "offsite_not_configured")
	if n := len(pg.calls(t)); n != 2 {
		t.Errorf("repo 2 without a target ran the binary (%d calls)", n)
	}
}

// TestPgBackRest_ExitCodeWithoutOutputText: a non-zero exit becomes pgbackrest_exit_<n>; the
// output never enters the error, the tool's stderr goes to the Stderr writer.
func TestPgBackRest_ExitCodeWithoutOutputText(t *testing.T) {
	bin := t.TempDir()
	pg := newFake(t, bin, "pgbackrest", "echo 'stdout EXAMPLE-leak'; echo 'stderr EXAMPLE-diagnostic' >&2; exit 82")
	var stderr syncBuffer
	e := newExecutor(t, executor.Config{PgBackRest: pg.path, SecretsDir: secretsDir(t, false), Stderr: &stderr})
	_, err := e.PgBackRestBackup(context.Background(), app.PgBackRestBackup{Type: "full", Repo: 1})
	wantCode(t, err, "pgbackrest_exit_82")
	if strings.Contains(err.Error(), "EXAMPLE") {
		t.Errorf("error %q carries output text", err)
	}
	if !strings.Contains(stderr.String(), "stderr EXAMPLE-diagnostic") {
		t.Errorf("stderr of the tool did not reach the Stderr writer: %q", stderr.String())
	}
}

// TestPgBackRest_InfoOutputInvalid: info output that is no JSON, or has no backup of the
// stanza, is pgbackrest_output_invalid.
func TestPgBackRest_InfoOutputInvalid(t *testing.T) {
	for name, out := range map[string]string{
		"not json":  "EXAMPLE not json",
		"no stanza": `[{"name":"other","backup":[{"label":"20261020-020000F","timestamp":{"stop":1},"info":{"repository":{"delta":7}}}]}]`,
		"bad label": `[{"name":"bothy","backup":[{"label":"x y","timestamp":{"stop":1},"info":{"repository":{"delta":7}}}]}]`,
		"no delta":  `[{"name":"bothy","backup":[{"label":"20261020-020000F","timestamp":{"stop":1},"info":{}}]}]`,
	} {
		t.Run(name, func(t *testing.T) {
			pg := newFake(t, t.TempDir(), "pgbackrest", `case "$*" in *" info") printf '%s\n' '`+out+`';; esac`)
			e := newExecutor(t, executor.Config{PgBackRest: pg.path, SecretsDir: secretsDir(t, false)})
			_, err := e.PgBackRestBackup(context.Background(), app.PgBackRestBackup{Type: "full", Repo: 1})
			wantCode(t, err, "pgbackrest_output_invalid")
		})
	}
}

// TestPgBackRestExpire: repo1 with its retention; repo2 with the owner's keys from adminDir,
// never the keys of offsite/.
func TestPgBackRestExpire(t *testing.T) {
	bin := t.TempDir()
	pg := pgbackrestFake(t, bin)
	secrets := secretsDir(t, false)
	e := newExecutor(t, executor.Config{
		PgBackRest: pg.path, SecretsDir: secrets, Offsite: exampleTarget(), OffsiteRegion: "eu-example-1", OffsitePathStyle: true,
	})
	if err := e.PgBackRestExpireLocal(context.Background(), 2); err != nil {
		t.Fatal(err)
	}
	admin := adminDir(t)
	if err := e.PgBackRestExpireOffsite(context.Background(), admin, 4); err != nil {
		t.Fatal(err)
	}
	calls := pg.calls(t)
	if len(calls) != 2 {
		t.Fatalf("%d calls", len(calls))
	}
	if want := []string{"--stanza=bothy", "--repo=1", "--repo1-retention-full=2", "expire"}; !slices.Equal(calls[0].args, want) {
		t.Errorf("expire local args %q, want %q", calls[0].args, want)
	}
	if want := []string{"--stanza=bothy", "--repo=2", "--repo2-retention-full=4", "expire"}; !slices.Equal(calls[1].args, want) {
		t.Errorf("expire offsite args %q, want %q", calls[1].args, want)
	}
	checkEnv(t, calls[1], repo2Env(exampleAdminAccess, exampleAdminSecret, ""))

	local := newExecutor(t, executor.Config{PgBackRest: pg.path, SecretsDir: secrets})
	wantCode(t, local.PgBackRestExpireOffsite(context.Background(), admin, 4), "offsite_not_configured")
}

// TestMissingBinaryAndCanceled: a binary that does not exist is <tool>_not_found; a canceled
// context is canceled.
func TestMissingBinaryAndCanceled(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "absent")
	e := newExecutor(t, executor.Config{PgBackRest: missing, Restic: missing, PgDump: missing, SecretsDir: secretsDir(t, false),
		DB: executor.DB{Host: "db", Port: 5432, Name: "bothy", User: "bothy_owner", Password: func(context.Context) ([]byte, error) { return []byte("EXAMPLE"), nil }}})
	_, err := e.PgBackRestBackup(context.Background(), app.PgBackRestBackup{Type: "full", Repo: 1})
	wantCode(t, err, "pgbackrest_not_found")
	_, err = e.ResticBackup(context.Background(), app.ResticBackup{Paths: []string{"/srv/files"}, Tags: []string{"objects"}})
	wantCode(t, err, "restic_not_found")
	_, err = e.PgDump(context.Background(), filepath.Join(t.TempDir(), "dump"))
	wantCode(t, err, "pg_dump_not_found")

	pg := pgbackrestFake(t, t.TempDir())
	e = newExecutor(t, executor.Config{PgBackRest: pg.path, SecretsDir: secretsDir(t, false)})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = e.PgBackRestBackup(ctx, app.PgBackRestBackup{Type: "full", Repo: 1})
	wantCode(t, err, "canceled")
}

// pgDumpFake writes 123 bytes to the --file argument and records the passfile, its mode and
// its directory.
func pgDumpFake(t *testing.T, dir string) fake {
	return newFake(t, dir, "pg_dump", `for a in "$@"; do case "$a" in --file=*) f="${a#--file=}";; esac; done
head -c 123 /dev/zero > "$f"
{ printf 'passfile=%s\n' "$(cat "$PGPASSFILE")"; echo "mode=$(ls -l "$PGPASSFILE" | cut -c1-10)"; } >> '`+filepath.Join(dir, "pg_dump.seen")+`'`)
}

// TestPgDump: the argument list, a passfile with the escaped password in a fresh directory
// next to the dump (mode 0600, removed afterwards), no PGPASSWORD, and the size of the file.
func TestPgDump(t *testing.T) {
	bin := t.TempDir()
	pd := pgDumpFake(t, bin)
	e := newExecutor(t, executor.Config{PgDump: pd.path, SecretsDir: secretsDir(t, false), DB: executor.DB{
		Host: "postgres", Port: 5432, Name: "bothy", User: "bothy_owner",
		Password: func(context.Context) ([]byte, error) { return []byte(exampleDBPassword), nil },
	}})
	scratch := t.TempDir()
	path := filepath.Join(scratch, "logical.dump")
	size, err := e.PgDump(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	if size != 123 {
		t.Errorf("size %d, want 123", size)
	}
	calls := pd.calls(t)
	if len(calls) != 1 {
		t.Fatalf("%d calls", len(calls))
	}
	want := []string{"-Fc", "--no-password", "--host=postgres", "--port=5432", "--username=bothy_owner", "--dbname=bothy", "--file=" + path}
	if !slices.Equal(calls[0].args, want) {
		t.Errorf("args %q, want %q", calls[0].args, want)
	}
	passfile := calls[0].env["PGPASSFILE"]
	checkEnv(t, calls[0], map[string]string{"PGPASSFILE": passfile})
	if filepath.Dir(filepath.Dir(passfile)) != scratch {
		t.Errorf("passfile %s is not in a fresh directory under %s", passfile, scratch)
	}
	if _, err := os.Stat(filepath.Dir(passfile)); !os.IsNotExist(err) {
		t.Errorf("the passfile directory was not removed: %v", err)
	}
	seen, err := os.ReadFile(filepath.Join(bin, "pg_dump.seen")) // #nosec G304 -- a file of the test's own temporary directory
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(seen), `passfile=postgres:5432:bothy:bothy_owner:EXAMPLE\:db\\password`) || !strings.Contains(string(seen), "mode=-rw-------") {
		t.Errorf("passfile as the child saw it: %s", seen)
	}
}

// TestPgDump_ExitRemovesPassfile: a failed pg_dump is pg_dump_exit_<n> and leaves no passfile.
func TestPgDump_ExitRemovesPassfile(t *testing.T) {
	pd := newFake(t, t.TempDir(), "pg_dump", "exit 1")
	e := newExecutor(t, executor.Config{PgDump: pd.path, SecretsDir: secretsDir(t, false), DB: executor.DB{
		Host: "postgres", Port: 5432, Name: "bothy", User: "bothy_owner",
		Password: func(context.Context) ([]byte, error) { return []byte("EXAMPLE"), nil },
	}})
	scratch := t.TempDir()
	_, err := e.PgDump(context.Background(), filepath.Join(scratch, "logical.dump"))
	wantCode(t, err, "pg_dump_exit_1")
	entries, _ := os.ReadDir(scratch)
	if len(entries) != 0 {
		t.Errorf("left behind: %v", entries)
	}
}

// summaryLine is a restic backup --json summary.
const summaryLine = `{"message_type":"summary","files_new":3,"data_added":2048,"data_added_packed":1024,` +
	`"snapshot_id":"4a8c2b0f9d3e1a7c6b5d4e3f2a1b0c9d8e7f6a5b4c3d2e1f0a9b8c7d6e5f4a3b"}`

// resticFake answers cat config with exit catExit, init with 0 and backup with a status and a
// summary line; stdin of a backup is saved next to the log.
func resticFake(t *testing.T, dir string, catExit int) fake {
	return newFake(t, dir, "restic", `case "$*" in
*"cat config"*) exit `+strconv.Itoa(catExit)+`;;
*" init"*) exit 0;;
*" backup "*) case "$*" in *--stdin*) cat > '`+filepath.Join(dir, "restic.stdin")+`';; esac
  echo '{"message_type":"status","percent_done":0.5}'; echo '`+summaryLine+`';;
esac`)
}

// TestResticBackup_OffsiteInit: exit 10 of cat config initialises the off-site repository;
// every call names the repository, the bucket lookup, the region and the CA file, with the keys
// of offsite/ and the password file in the environment.
func TestResticBackup_OffsiteInit(t *testing.T) {
	bin := t.TempDir()
	r := resticFake(t, bin, 10)
	secrets := secretsDir(t, true)
	e := newExecutor(t, executor.Config{Restic: r.path, SecretsDir: secrets, Offsite: exampleTarget(), OffsiteRegion: "eu-example-1", OffsitePathStyle: true})
	res, err := e.ResticBackup(context.Background(), app.ResticBackup{Paths: []string{"/srv/files"}, Tags: []string{"objects", "run-1"}})
	if err != nil {
		t.Fatal(err)
	}
	if res.SnapshotID != "4a8c2b0f9d3e1a7c6b5d4e3f2a1b0c9d8e7f6a5b4c3d2e1f0a9b8c7d6e5f4a3b" || res.SizeBytes != 1024 {
		t.Errorf("result %+v", res)
	}
	calls := r.calls(t)
	if len(calls) != 3 {
		t.Fatalf("%d calls, want cat config, init, backup", len(calls))
	}
	repo := []string{"-r", "s3:https://s3.example.com:9443/bothy-backup/instance-1/restic", "-o", "s3.bucket-lookup=path",
		"-o", "s3.region=eu-example-1", "--cacert", filepath.Join(secrets, "offsite", "ca.crt")}
	for i, tail := range [][]string{{"cat", "config"}, {"init"}, {"backup", "--json", "--quiet", "--tag", "objects", "--tag", "run-1", "--", "/srv/files"}} {
		if want := append(slices.Clone(repo), tail...); !slices.Equal(calls[i].args, want) {
			t.Errorf("call %d args %q, want %q", i, calls[i].args, want)
		}
		checkEnv(t, calls[i], map[string]string{
			"AWS_ACCESS_KEY_ID": exampleAccessKey, "AWS_SECRET_ACCESS_KEY": exampleSecretKey,
			"RESTIC_PASSWORD_FILE": filepath.Join(secrets, "restic_password"),
		})
	}
}

// TestResticBackup_StdinLocal: without an off-site target the local repository; an existing
// repository is not initialised again; stdin reaches the child under its file name.
func TestResticBackup_StdinLocal(t *testing.T) {
	bin := t.TempDir()
	r := resticFake(t, bin, 0)
	secrets := secretsDir(t, false)
	e := newExecutor(t, executor.Config{Restic: r.path, SecretsDir: secrets, LocalResticRepo: "/var/lib/example/restic"})
	_, err := e.ResticBackup(context.Background(), app.ResticBackup{Stdin: []byte(`{"schema":"backup_manifest.v1"}`), StdinName: "manifest-run-1.json", Tags: []string{"manifest"}})
	if err != nil {
		t.Fatal(err)
	}
	calls := r.calls(t)
	if len(calls) != 2 {
		t.Fatalf("%d calls, want cat config and backup", len(calls))
	}
	want := []string{"-r", "/var/lib/example/restic", "backup", "--json", "--quiet", "--tag", "manifest", "--stdin", "--stdin-filename", "manifest-run-1.json"}
	if !slices.Equal(calls[1].args, want) {
		t.Errorf("args %q, want %q", calls[1].args, want)
	}
	checkEnv(t, calls[1], map[string]string{"RESTIC_PASSWORD_FILE": filepath.Join(secrets, "restic_password")})
	got, err := os.ReadFile(filepath.Join(bin, "restic.stdin")) // #nosec G304 -- a file of the test's own temporary directory
	if err != nil || string(got) != `{"schema":"backup_manifest.v1"}` {
		t.Errorf("stdin %q, %v", got, err)
	}
}

// TestResticBackup_Errors: a summary without snapshot id is restic_output_invalid; a failing
// cat config other than 10 is restic_exit_<n> without init.
func TestResticBackup_Errors(t *testing.T) {
	r := newFake(t, t.TempDir(), "restic", `case "$*" in *" backup "*) echo '{"message_type":"summary","data_added_packed":1}';; esac`)
	e := newExecutor(t, executor.Config{Restic: r.path, SecretsDir: secretsDir(t, false)})
	_, err := e.ResticBackup(context.Background(), app.ResticBackup{Paths: []string{"/srv/files"}, Tags: []string{"objects"}})
	wantCode(t, err, "restic_output_invalid")

	r = resticFake(t, t.TempDir(), 12)
	e = newExecutor(t, executor.Config{Restic: r.path, SecretsDir: secretsDir(t, false)})
	_, err = e.ResticBackup(context.Background(), app.ResticBackup{Paths: []string{"/srv/files"}, Tags: []string{"objects"}})
	wantCode(t, err, "restic_exit_12")
	if n := len(r.calls(t)); n != 1 {
		t.Errorf("%d calls after a failing cat config, want 1", n)
	}
}

// TestResticForget: the local repository without AdminDir; with an off-site target the service
// refuses to forget without AdminDir and runs nothing; with AdminDir the owner's keys. Keep-within
// is whole hours, rounded up.
func TestResticForget(t *testing.T) {
	bin := t.TempDir()
	r := resticFake(t, bin, 0)
	secrets := secretsDir(t, false)
	local := newExecutor(t, executor.Config{Restic: r.path, SecretsDir: secrets})
	if err := local.ResticForget(context.Background(), app.ResticForget{Tag: "logical", KeepWithin: 30*24*time.Hour + time.Minute}); err != nil {
		t.Fatal(err)
	}
	calls := r.calls(t)
	want := []string{"-r", "/var/lib/pgbackrest/restic", "forget", "--tag", "logical", "--group-by=", "--keep-within", "721h", "--prune"}
	if len(calls) != 1 || !slices.Equal(calls[0].args, want) {
		t.Fatalf("calls %+v, want args %q", calls, want)
	}

	off := newExecutor(t, executor.Config{Restic: r.path, SecretsDir: secrets, Offsite: exampleTarget(), OffsitePathStyle: false})
	wantCode(t, off.ResticForget(context.Background(), app.ResticForget{Tag: "logical", KeepWithin: time.Hour}), "restic_offsite_forget_refused")
	if n := len(r.calls(t)); n != 1 {
		t.Fatalf("the refused forget ran the binary (%d calls)", n)
	}
	if err := off.ResticForget(context.Background(), app.ResticForget{Tag: "logical", KeepWithin: 2 * time.Hour, AdminDir: adminDir(t)}); err != nil {
		t.Fatal(err)
	}
	calls = r.calls(t)
	last := calls[len(calls)-1]
	want = []string{"-r", "s3:https://s3.example.com:9443/bothy-backup/instance-1/restic", "-o", "s3.bucket-lookup=dns", "-o", "s3.region=us-east-1",
		"forget", "--tag", "logical", "--group-by=", "--keep-within", "2h", "--prune"}
	if !slices.Equal(last.args, want) {
		t.Errorf("args %q, want %q", last.args, want)
	}
	checkEnv(t, last, map[string]string{"AWS_ACCESS_KEY_ID": exampleAdminAccess, "AWS_SECRET_ACCESS_KEY": exampleAdminSecret,
		"RESTIC_PASSWORD_FILE": filepath.Join(secrets, "restic_password")})

	wantCode(t, local.ResticForget(context.Background(), app.ResticForget{KeepWithin: time.Hour}), "restic_request_invalid")
	wantCode(t, local.ResticForget(context.Background(), app.ResticForget{Tag: "logical"}), "restic_request_invalid")
}

// mirrorFunc is a mirror of the tests.
type mirrorFunc func(context.Context) (app.MirrorResult, error)

func (f mirrorFunc) S3Mirror(ctx context.Context) (app.MirrorResult, error) { return f(ctx) }

// TestS3Mirror: without a mirror the step is s3_mirror_not_configured; with one it delegates.
func TestS3Mirror(t *testing.T) {
	e := newExecutor(t, executor.Config{SecretsDir: secretsDir(t, false)})
	_, err := e.S3Mirror(context.Background())
	wantCode(t, err, "s3_mirror_not_configured")
	e = newExecutor(t, executor.Config{SecretsDir: secretsDir(t, false), Mirror: mirrorFunc(func(context.Context) (app.MirrorResult, error) {
		return app.MirrorResult{Copied: 2, SizeBytes: 10}, nil
	})})
	res, err := e.S3Mirror(context.Background())
	if err != nil || res.Copied != 2 || res.SizeBytes != 10 {
		t.Errorf("S3Mirror = %+v, %v", res, err)
	}
}

// TestVolumeUsage: the file system of a temporary directory has a size and at most that much
// free space; a missing path is an error code.
func TestVolumeUsage(t *testing.T) {
	e := newExecutor(t, executor.Config{SecretsDir: secretsDir(t, false)})
	u, err := e.VolumeUsage(context.Background(), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if u.SizeBytes == 0 || u.FreeBytes > u.SizeBytes {
		t.Errorf("usage %+v", u)
	}
	_, err = e.VolumeUsage(context.Background(), filepath.Join(t.TempDir(), "absent"))
	wantCode(t, err, "volume_usage_failed")
}

// TestNewRequiresSecretsDir: the secret directory is required.
func TestNewRequiresSecretsDir(t *testing.T) {
	if _, err := executor.New(executor.Config{}); err == nil {
		t.Error("New without SecretsDir succeeded")
	}
}
