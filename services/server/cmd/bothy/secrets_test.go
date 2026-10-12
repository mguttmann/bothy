// SPDX-License-Identifier: Apache-2.0

//go:build unix

package main

import (
	"archive/tar"
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"flag"
	"io"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"filippo.io/age"
	"filippo.io/age/armor"

	"github.com/OWNER/bothy/services/server/internal/platform/config"
)

// testCanary returns a value that marks a secret in a test: EXAMPLE plus random hex, so a hit in
// any output is never a coincidence.
func testCanary(t *testing.T) string {
	t.Helper()
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		t.Fatal(err)
	}
	return "EXAMPLE-canary-" + hex.EncodeToString(b)
}

// refSchema is a schema with the provider keys, secrets.dir set to dir, and two reference keys:
// database.password_ref names a file below secrets.dir, database.maint_password_ref an env
// variable. The server schema gets its first reference key with M1-12.
func refSchema(dir string) func() (*config.Schema, error) {
	return func() (*config.Schema, error) {
		return config.NewSchema([]config.Key{
			{Name: "secrets.dir", Kind: config.KindString, Default: dir, Doc: "Dir.", Chapter: "16-operations.md"},
			{Name: "secrets.cache_ttl", Kind: config.KindDuration, Default: "5m", Doc: "TTL.", Chapter: "16-operations.md"},
			{Name: "database.password_ref", Kind: config.KindSecretRef, Default: "file:<secrets.dir>/db_app_password", Doc: "Password.", Chapter: "16-operations.md"},
			{Name: "database.maint_password_ref", Kind: config.KindSecretRef, Default: "env:BOTHY_SECRET_EXAMPLE_MAINT", Doc: "Password.", Chapter: "16-operations.md"},
		})
	}
}

// writeSecret writes content as the file name below dir with mode and returns its path.
func writeSecret(t *testing.T, dir, name, content string, mode fs.FileMode) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), mode); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, mode); err != nil { // WriteFile applies the umask
		t.Fatal(err)
	}
	return path
}

// runBothy runs the binary's entry point with env and args and returns exit code, stdout and
// stderr.
func runBothy(t *testing.T, env []string, args ...string) (int, string, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code := run(t.Context(), args, env, &stdout, &stderr)
	return code, stdout.String(), stderr.String()
}

// noCanary fails when a canary appears in any of outs.
func noCanary(t *testing.T, canaries []string, outs ...string) {
	t.Helper()
	for _, out := range outs {
		for _, c := range canaries {
			if strings.Contains(out, c) {
				t.Errorf("output contains a secret:\n%s", out)
			}
		}
	}
}

// TestConfigPrintNeverShowsSecrets is SEC-T12: config print --redacted and config check
// --secrets show references, never the secret behind them, on stdout and stderr.
func TestConfigPrintNeverShowsSecrets(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	fileSecret, envSecret := testCanary(t), testCanary(t)
	path := writeSecret(t, dir, "db_app_password", fileSecret+"\n", 0o600)
	env := []string{"BOTHY_SECRET_EXAMPLE_MAINT=" + envSecret}
	canaries := []string{fileSecret, envSecret}

	code, stdout, stderr := runConfig(t, refSchema(dir), env, "print", "--redacted")
	if code != 0 || stderr != "" ||
		!strings.Contains(stdout, "database.password_ref = file:"+path+"  # default\n") ||
		!strings.Contains(stdout, "database.maint_password_ref = env:BOTHY_SECRET_EXAMPLE_MAINT  # default\n") {
		t.Errorf("print: exit %d, stderr %q, stdout\n%s", code, stderr, stdout)
	}
	noCanary(t, canaries, stdout, stderr)

	code, stdout, stderr = runConfig(t, refSchema(dir), env, "check", "--secrets")
	want := "secret database.maint_password_ref: ok\n" +
		"secret database.password_ref: ok\n" +
		"configuration ok\n"
	if code != 0 || stdout != want {
		t.Errorf("check --secrets: exit %d, stdout\n%s\nwant 0 and\n%s", code, stdout, want)
	}
	noCanary(t, canaries, stdout, stderr)

	// The same references set in the environment, and both commands through the entry point
	// with the server schema, whose secrets.dir holds the file.
	env = append(env, "BOTHY_DATABASE__PASSWORD_REF=file:<secrets.dir>/db_app_password",
		"BOTHY_DATABASE__MAINT_PASSWORD_REF=env:BOTHY_SECRET_EXAMPLE_MAINT")
	code, stdout, stderr = runConfig(t, refSchema(dir), env, "check", "--secrets")
	if code != 0 || stdout != want {
		t.Errorf("check --secrets with references from env: exit %d, stdout\n%s", code, stdout)
	}
	noCanary(t, canaries, stdout, stderr)
	for _, args := range [][]string{{"config", "print", "--redacted"}, {"config", "check", "--secrets"}} {
		code, stdout, stderr = runBothy(t, []string{"BOTHY_SECRETS__DIR=" + dir}, args...)
		if code != 0 || stderr != "" {
			t.Errorf("run %q: exit %d, stderr %q", args, code, stderr)
		}
		noCanary(t, canaries, stdout, stderr)
	}
	if stdout != "configuration ok\n" {
		t.Errorf("run config check --secrets with the server schema: stdout %q", stdout)
	}
}

// TestConfigCheckSecretsReportsInsecureFile is SEC-T11 seen from config check: a secret file
// with mode 0644 fails with its code on stdout and the reference and mode on stderr, never the
// content.
func TestConfigCheckSecretsReportsInsecureFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	secret, envSecret := testCanary(t), testCanary(t)
	path := writeSecret(t, dir, "db_app_password", secret, 0o644)
	env := []string{"BOTHY_SECRET_EXAMPLE_MAINT=" + envSecret}

	code, stdout, stderr := runConfig(t, refSchema(dir), env, "check", "--secrets")
	want := "secret database.maint_password_ref: ok\n" +
		"secret database.password_ref: insecure_file\n"
	if code != 1 || stdout != want {
		t.Errorf("exit %d, stdout\n%s\nwant 1 and\n%s", code, stdout, want)
	}
	if line := "config: database.password_ref: secrets: file:" + path + ": "; !strings.Contains(stderr, line) || !strings.Contains(stderr, "mode 0644") {
		t.Errorf("stderr does not name the reference and the mode:\n%s", stderr)
	}
	noCanary(t, []string{secret, envSecret}, stdout, stderr)

	// Without --secrets check does not read the file.
	if code, stdout, _ := runConfig(t, refSchema(dir), env, "check"); code != 0 || stdout != "configuration ok\n" {
		t.Errorf("check without --secrets: exit %d, stdout %q", code, stdout)
	}

	// A schema the provider cannot be built from (no secrets.dir) fails before any reference;
	// only the env: warning of the configuration comes first.
	code, stdout, stderr = runConfig(t, secretSchema, []string{"BOTHY_SECRET_TEST=" + secret}, "check", "--secrets")
	if code != 1 || stdout != "" ||
		!strings.HasPrefix(stderr, "warning: database.password_ref uses an env: reference; prefer file: (16-operations.md 5.2)\nbothy config: ") ||
		!strings.Contains(stderr, "secrets.dir") {
		t.Errorf("schema without secrets.dir: exit %d, stdout %q, stderr %q", code, stdout, stderr)
	}
	noCanary(t, []string{secret}, stdout, stderr)
}

// TestConfigCheckSecretsWarnsOnEnv holds 16 5.2 and brief D2: env: references work but are not
// recommended, so config check warns once per key that uses one, with or without --secrets, and
// with --secrets whether the reference resolves or not.
func TestConfigCheckSecretsWarnsOnEnv(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeSecret(t, dir, "db_app_password", testCanary(t), 0o400)
	const warning = "warning: database.maint_password_ref uses an env: reference; prefer file: (16-operations.md 5.2)\n"

	// Without --secrets nothing is read, but the reference is configured all the same.
	for _, env := range [][]string{nil, {"BOTHY_DATABASE__MAINT_PASSWORD_REF=env:BOTHY_SECRET_EXAMPLE_MAINT"}} {
		code, stdout, stderr := runConfig(t, refSchema(dir), env, "check")
		if code != 0 || stdout != "configuration ok\n" || stderr != warning {
			t.Errorf("check with env %q: exit %d, stdout %q, stderr %q; want 0, configuration ok and only the warning", env, code, stdout, stderr)
		}
	}
	// A file: reference set in place of the env: default gives no warning.
	fileRef := []string{"BOTHY_DATABASE__MAINT_PASSWORD_REF=file:<secrets.dir>/db_app_password"}
	if code, stdout, stderr := runConfig(t, refSchema(dir), fileRef, "check"); code != 0 || stdout != "configuration ok\n" || stderr != "" {
		t.Errorf("check with file: references only: exit %d, stdout %q, stderr %q", code, stdout, stderr)
	}

	code, stdout, stderr := runConfig(t, refSchema(dir), []string{"BOTHY_SECRET_EXAMPLE_MAINT=" + testCanary(t)}, "check", "--secrets")
	if code != 0 || !strings.HasSuffix(stdout, "configuration ok\n") || stderr != warning {
		t.Errorf("set: exit %d, stdout %q, stderr %q; want 0 and only the warning", code, stdout, stderr)
	}

	code, stdout, stderr = runConfig(t, refSchema(dir), nil, "check", "--secrets")
	want := "secret database.maint_password_ref: not_found\n" +
		"secret database.password_ref: ok\n"
	if code != 1 || stdout != want || strings.Count(stderr, warning) != 1 ||
		!strings.Contains(stderr, "config: database.maint_password_ref: secrets: env:BOTHY_SECRET_EXAMPLE_MAINT: ") {
		t.Errorf("unset: exit %d, stdout %q, stderr %q", code, stdout, stderr)
	}
	if strings.Contains(stderr, "database.password_ref uses") {
		t.Errorf("a file: reference got a warning: %q", stderr)
	}
}

// openTree makes every directory below root writable again: service directories have mode 0500,
// and the TempDir cleanup must be able to empty them.
func openTree(t *testing.T, root string) {
	t.Helper()
	t.Cleanup(func() {
		_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err == nil && d.IsDir() {
				_ = os.Chmod(path, 0o700) //nolint:gosec // G302: a directory needs the x bit to be emptied.
			}
			return nil
		})
	})
}

// reportLines returns the report lines of secrets init without the reminder, and fails unless
// the last line is the reminder to export the _store of target.
func reportLines(t *testing.T, stdout, target string) []string {
	t.Helper()
	reminder := "back up the secrets: bothy secrets export --from " + filepath.Join(target, "_store") + " --age-recipient <age public key>"
	lines := strings.Split(strings.TrimSuffix(stdout, "\n"), "\n")
	if len(lines) < 2 || lines[len(lines)-1] != reminder || !strings.HasSuffix(stdout, "\n") {
		t.Fatalf("report does not end with %q:\n%s", reminder, stdout)
	}
	return lines[:len(lines)-1]
}

// TestSecretsInitCommand runs bothy secrets init through the entry point: a fresh target, a
// second run that changes nothing, and the exit codes of usage errors and conflicts.
func TestSecretsInitCommand(t *testing.T) {
	t.Parallel()
	base := t.TempDir()
	target, source := filepath.Join(base, "target"), filepath.Join(base, "source")
	if err := os.Mkdir(source, 0o700); err != nil {
		t.Fatal(err)
	}
	openTree(t, target)
	me := strconv.Itoa(os.Geteuid()) + ":" + strconv.Itoa(os.Getegid())
	owners := []string{"--owner", "server=" + me, "--owner", "migrate=" + me}
	initArgs := append([]string{"secrets", "init", "--target", target, "--source", source}, owners...)
	supplied := testCanary(t)
	writeSecret(t, source, "apns_key.p8", supplied, 0o600)
	// secrets init reads only its flags: a variable config check would reject changes nothing.
	env := []string{"BOTHY_UNKNOWN__EXAMPLE=1", "BOTHY_CONFIG_FILE=/nonexistent/EXAMPLE.yaml"}

	code, first, stderr := runBothy(t, env, initArgs...)
	if code != 0 || stderr != "" {
		t.Fatalf("fresh target: exit %d, stderr %q, stdout\n%s", code, stderr, first)
	}
	lines := reportLines(t, first, target)
	for _, want := range []string{"generated db_app_password sha256:", "adopted apns_key.p8 sha256:", "pending totp_key", "missing sentry_key"} {
		if !slices.ContainsFunc(lines, func(l string) bool { return strings.HasPrefix(l, want) }) {
			t.Errorf("report has no line %q:\n%s", want, first)
		}
	}
	if info, err := os.Stat(filepath.Join(target, "server", "db_app_password")); err != nil || info.Mode().Perm() != 0o400 {
		t.Errorf("server copy of db_app_password: %v, %v", info, err)
	}
	noCanary(t, []string{supplied}, first, stderr)

	// The second run keeps everything, with the same fingerprints.
	code, second, stderr := runBothy(t, nil, initArgs...)
	if code != 0 || stderr != "" {
		t.Fatalf("second run: exit %d, stderr %q", code, stderr)
	}
	for _, l := range reportLines(t, second, target) {
		if !strings.HasPrefix(l, "kept ") && !strings.HasPrefix(l, "missing ") && !strings.HasPrefix(l, "pending ") {
			t.Errorf("second run line %q, want only kept, missing and pending", l)
		}
	}
	kept := strings.NewReplacer("generated ", "kept ", "adopted ", "kept ").Replace(first)
	if second != kept {
		t.Errorf("second run\n%s\nwant\n%s", second, kept)
	}

	// Errors of stack: a usage error (unknown source file) is exit 2, a conflict exit 1; both
	// print one line and write nothing.
	unknown := writeSecret(t, source, "EXAMPLE_unknown.txt", "x", 0o600)
	code, stdout, stderr := runBothy(t, nil, initArgs...)
	if code != 2 || stdout != "" || !strings.HasPrefix(stderr, "bothy secrets init: ") ||
		!strings.Contains(stderr, "EXAMPLE_unknown.txt") || strings.Count(stderr, "\n") != 1 {
		t.Errorf("unknown source file: exit %d, stdout %q, stderr %q", code, stdout, stderr)
	}
	if err := os.Remove(unknown); err != nil {
		t.Fatal(err)
	}
	other := testCanary(t)
	writeSecret(t, source, "db_app_password", other, 0o600)
	code, stdout, stderr = runBothy(t, nil, initArgs...)
	if code != 1 || stdout != "" || !strings.HasPrefix(stderr, "bothy secrets init: ") ||
		!strings.Contains(stderr, "db_app_password") || strings.Count(stderr, "\n") != 1 {
		t.Errorf("conflict: exit %d, stdout %q, stderr %q", code, stdout, stderr)
	}
	// --adopt-source lets the source replace the generated secret.
	code, stdout, stderr = runBothy(t, nil, append(initArgs, "--adopt-source")...)
	if code != 0 || stderr != "" || !strings.Contains(stdout, "\nupdated db_app_password sha256:") {
		t.Errorf("--adopt-source: exit %d, stderr %q, stdout\n%s", code, stderr, stdout)
	}
	noCanary(t, []string{supplied, other}, stdout, stderr)

	// A missing owner of server is a usage error of stack: exit 2, one line.
	code, stdout, stderr = runBothy(t, nil, "secrets", "init", "--target", target, "--owner", "migrate="+me)
	if code != 2 || stdout != "" || !strings.HasPrefix(stderr, "bothy secrets init: ") || !strings.Contains(stderr, "server") {
		t.Errorf("missing server owner: exit %d, stdout %q, stderr %q", code, stdout, stderr)
	}

	// Flag errors print the usage as well.
	for _, args := range [][]string{
		append([]string{"secrets", "init"}, owners...),
		append([]string{"secrets", "init", "--target", ""}, owners...),
		append([]string{"secrets", "init", "--target", target, "--source", ""}, owners...),
		append([]string{"secrets", "init", "--target", target, "--owner", "server=" + me, "--owner", "server=" + me}, owners[2:]...),
		append([]string{"secrets", "init", "--target", target, "--owner", "server"}, owners[2:]...),
		append([]string{"secrets", "init", "--target", target, "--nope"}, owners...),
		append([]string{"secrets", "init", "--target", target, "extra"}, owners...),
		{"secrets", "frobnicate"},
	} {
		code, stdout, stderr := runBothy(t, nil, args...)
		prefix := "bothy secrets init: "
		if args[1] == "frobnicate" {
			prefix = "bothy secrets: "
		}
		if code != 2 || stdout != "" || !strings.HasPrefix(stderr, prefix) || !strings.HasSuffix(stderr, secretsUsage) {
			t.Errorf("%q: exit %d, stdout %q, stderr %q; want 2, error line and usage", args, code, stdout, stderr)
		}
	}

	// The report goes to stdout; when it cannot be written, the command fails.
	var stderrBuf bytes.Buffer
	if code := run(t.Context(), initArgs, nil, failingWriter{}, &stderrBuf); code != 1 ||
		!strings.HasPrefix(stderrBuf.String(), "bothy secrets init: ") {
		t.Errorf("failing stdout: exit %d, stderr %q", code, stderrBuf.String())
	}
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, errors.New("EXAMPLE write failure") }

// untar decrypts an age bundle with id and returns its regular files by name.
func untar(t *testing.T, bundle []byte, armored bool, id age.Identity) map[string]string {
	t.Helper()
	var in io.Reader = bytes.NewReader(bundle)
	if armored {
		in = armor.NewReader(in)
	}
	plain, err := age.Decrypt(in, id)
	if err != nil {
		t.Fatalf("decrypt: %v", err)
	}
	files := map[string]string{}
	tr := tar.NewReader(plain)
	for {
		h, err := tr.Next()
		if errors.Is(err, io.EOF) {
			return files
		}
		if err != nil {
			t.Fatalf("tar: %v", err)
		}
		if h.Typeflag == tar.TypeReg {
			b, err := io.ReadAll(tr)
			if err != nil {
				t.Fatal(err)
			}
			files[h.Name] = string(b)
		}
	}
}

// TestSecretsExportCommand runs bothy secrets export through the entry point: to a new file, to
// stdout (binary and armored), and the exit codes of a missing recipient and an existing file.
func TestSecretsExportCommand(t *testing.T) {
	t.Parallel()
	from := t.TempDir()
	secret := testCanary(t)
	writeSecret(t, from, "db_app_password", secret, 0o400)
	if err := os.Mkdir(filepath.Join(from, "offsite"), 0o700); err != nil {
		t.Fatal(err)
	}
	writeSecret(t, filepath.Join(from, "offsite"), "rclone.conf", "EXAMPLE-offsite", 0o400)
	want := map[string]string{"db_app_password": secret, "offsite/rclone.conf": "EXAMPLE-offsite"}
	id, err := age.GenerateX25519Identity()
	if err != nil {
		t.Fatal(err)
	}
	recipient := id.Recipient().String()
	// secrets export reads only its flags.
	env := []string{"BOTHY_UNKNOWN__EXAMPLE=1"}

	out := filepath.Join(t.TempDir(), "bundle.age")
	code, stdout, stderr := runBothy(t, env, "secrets", "export", "--from", from, "--age-recipient", recipient, "--output", out)
	if code != 0 || stdout != "" || stderr != "" {
		t.Fatalf("export to file: exit %d, stdout %q, stderr %q", code, stdout, stderr)
	}
	info, err := os.Stat(out)
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("bundle file: %v, %v; want mode 0600", info, err)
	}
	bundle, err := os.ReadFile(out) //nolint:gosec // G304: the path is the test's own temp file.
	if err != nil {
		t.Fatal(err)
	}
	if got := untar(t, bundle, false, id); !maps.Equal(got, want) {
		t.Errorf("bundle file holds %v, want %v", slices.Sorted(maps.Keys(got)), slices.Sorted(maps.Keys(want)))
	}

	// The default output is stdout; a buffer is no terminal, so binary output is fine.
	code, stdout, stderr = runBothy(t, nil, "secrets", "export", "--from", from, "--age-recipient", recipient)
	if code != 0 || stderr != "" {
		t.Fatalf("export to stdout: exit %d, stderr %q", code, stderr)
	}
	if got := untar(t, []byte(stdout), false, id); !maps.Equal(got, want) {
		t.Errorf("stdout bundle differs from the store")
	}
	code, stdout, stderr = runBothy(t, nil, "secrets", "export", "--from", from, "--age-recipient", recipient, "--output", "-", "--armor")
	if code != 0 || stderr != "" || !strings.HasPrefix(stdout, "-----BEGIN AGE ENCRYPTED FILE-----\n") {
		t.Fatalf("armored export: exit %d, stderr %q", code, stderr)
	}
	if got := untar(t, []byte(stdout), true, id); !maps.Equal(got, want) {
		t.Errorf("armored bundle differs from the store")
	}

	// An existing output file is exit 1 and stays as it was.
	code, stdout, stderr = runBothy(t, nil, "secrets", "export", "--from", from, "--age-recipient", recipient, "--output", out)
	after, readErr := os.ReadFile(out) //nolint:gosec // G304: the path is the test's own temp file.
	if code != 1 || stdout != "" || !strings.HasPrefix(stderr, "bothy secrets export: ") || strings.Count(stderr, "\n") != 1 ||
		readErr != nil || !bytes.Equal(after, bundle) {
		t.Errorf("existing output: exit %d, stdout %q, stderr %q, file unchanged %v", code, stdout, stderr, bytes.Equal(after, bundle))
	}
	noCanary(t, []string{secret}, stdout, stderr)

	// No recipient and an invalid recipient are usage errors of stack: exit 2, one line.
	for _, args := range [][]string{
		{"secrets", "export", "--from", from},
		{"secrets", "export", "--from", from, "--age-recipient", "AGE-SECRET-KEY-EXAMPLE"},
	} {
		code, stdout, stderr := runBothy(t, nil, args...)
		if code != 2 || stdout != "" || !strings.HasPrefix(stderr, "bothy secrets export: ") || strings.Count(stderr, "\n") != 1 ||
			strings.Contains(stderr, "AGE-SECRET-KEY-EXAMPLE") {
			t.Errorf("%q: exit %d, stdout %q, stderr %q", args, code, stdout, stderr)
		}
		noCanary(t, []string{secret}, stdout, stderr)
	}

	// Flag errors print the usage as well.
	for _, args := range [][]string{
		{"secrets", "export", "--age-recipient", recipient},
		{"secrets", "export", "--from", "", "--age-recipient", recipient},
		{"secrets", "export", "--from", from, "--age-recipient", recipient, "--output", ""},
		{"secrets", "export", "--from", from, "--age-recipient", recipient, "--armor=maybe"},
		{"secrets", "export", "--from", from, "--age-recipient", recipient, "extra"},
	} {
		code, stdout, stderr := runBothy(t, nil, args...)
		if code != 2 || stdout != "" || !strings.HasPrefix(stderr, "bothy secrets export: ") || !strings.HasSuffix(stderr, secretsUsage) {
			t.Errorf("%q: exit %d, stdout %q, stderr %q; want 2, error line and usage", args, code, stdout, stderr)
		}
		noCanary(t, []string{secret}, stdout, stderr)
	}

	// An identity pasted without its flag, also after a bool flag, is named by its position and
	// never repeated: like a rejected --age-recipient it must not reach the terminal or a log.
	for _, c := range []struct {
		pos  string
		args []string
	}{
		{"5", []string{"secrets", "export", "--from", from, "--age-recipient", recipient, "AGE-SECRET-KEY-EXAMPLE"}},
		{"6", []string{"secrets", "export", "--from", from, "--age-recipient", recipient, "--armor", "AGE-SECRET-KEY-EXAMPLE"}},
	} {
		code, stdout, stderr := runBothy(t, nil, c.args...)
		line := "bothy secrets export: unexpected argument " + c.pos + " after export, not shown because it may be a key; every value needs its flag\n"
		if code != 2 || stdout != "" || stderr != line+secretsUsage || strings.Contains(stderr, "AGE-SECRET-KEY-EXAMPLE") {
			t.Errorf("%q: exit %d, stdout %q, stderr %q; want 2 and %q with the usage", c.args, code, stdout, stderr, line)
		}
	}
}

// TestSecretsHelp holds that every spelling of help prints the help on stdout with exit 0, and
// that the help names every flag of init and export.
func TestSecretsHelp(t *testing.T) {
	t.Parallel()
	for _, args := range [][]string{
		{"secrets", "help"}, {"secrets", "-h"}, {"secrets", "--help"},
		{"secrets", "init", "-h"}, {"secrets", "init", "--help"}, {"secrets", "export", "-h"},
	} {
		code, stdout, stderr := runBothy(t, nil, args...)
		if code != 0 || stdout != secretsUsage || stderr != "" {
			t.Errorf("%q: exit %d, stdout %q, stderr %q", args, code, stdout, stderr)
		}
	}
	for _, sub := range []string{"init", "export"} {
		fs, _ := secretsFlags(sub)
		fs.VisitAll(func(f *flag.Flag) {
			if !strings.Contains(secretsUsage, "--"+f.Name+" ") {
				t.Errorf("secretsUsage does not describe --%s of %s", f.Name, sub)
			}
		})
	}
}

// TestSecretsWithoutSubcommandIsUsage holds the idiom of bare bothy and bare bothy config: a call
// that names no subcommand does nothing, so it is a usage error (exit 2, usage on stderr) and a
// script with an empty subcommand variable does not pass as successful.
func TestSecretsWithoutSubcommandIsUsage(t *testing.T) {
	t.Parallel()
	code, stdout, stderr := runBothy(t, nil, "secrets")
	if want := "bothy secrets: missing subcommand\n" + secretsUsage; code != 2 || stdout != "" || stderr != want {
		t.Errorf("exit %d, stdout %q, stderr %q; want 2, nothing and\n%s", code, stdout, stderr, want)
	}
}

// TestUsageListsSecrets holds that the command table registers secrets with its summary.
func TestUsageListsSecrets(t *testing.T) {
	t.Parallel()
	code, stdout, _ := runBothy(t, nil, "--help")
	if line := "  secrets  Create the secrets of the Compose stack or export them encrypted\n"; code != 0 || !strings.Contains(stdout, line) {
		t.Errorf("usage does not list secrets:\n%s", stdout)
	}
}

// TestSecretsNeverEchoArguments holds that no error of bothy secrets repeats text from the
// command line (ruling M1-09/21): an age identity pasted into the wrong place, as a subcommand,
// as a flag name, as the value of a flag or as an owner, must not reach the terminal or a log.
// The message names the position of the argument instead.
func TestSecretsNeverEchoArguments(t *testing.T) {
	t.Parallel()
	const key = "AGE-SECRET-KEY-1EXAMPLEEXAMPLEEXAMPLE"
	dir := t.TempDir()
	for _, c := range []struct {
		args []string
		want string // the first line of stderr
	}{
		{[]string{"secrets", key}, "bothy secrets: unknown subcommand; want init or export"},
		{[]string{"secrets", "init", "--target", dir, "--" + key}, "bothy secrets init: argument 3 is not a flag of init or has an invalid value; it is not shown because it may be a key"},
		{[]string{"secrets", "init", "--target", dir, "--owner", key}, "bothy secrets init: invalid --owner at argument 4: want <service>=<uid>:<gid>, the service in lower-case letters, digits and -"},
		{[]string{"secrets", "init", "--target", dir, "--owner=server=" + key}, "bothy secrets init: invalid --owner at argument 3: want <service>=<uid>:<gid>, the service in lower-case letters, digits and -"},
		{[]string{"secrets", "init", "--target", dir, "--owner", key + "=1:1"}, "bothy secrets init: invalid --owner at argument 4: want <service>=<uid>:<gid>, the service in lower-case letters, digits and -"},
		{[]string{"secrets", "init", "--target", dir, "--adopt-source=" + key}, "bothy secrets init: argument 3 is not a flag of init or has an invalid value; it is not shown because it may be a key"},
		{[]string{"secrets", "export", "--from", dir, "--armor=" + key}, "bothy secrets export: argument 3 is not a flag of export or has an invalid value; it is not shown because it may be a key"},
		{[]string{"secrets", "export", "--from", dir, "-" + key}, "bothy secrets export: argument 3 is not a flag of export or has an invalid value; it is not shown because it may be a key"},
	} {
		code, stdout, stderr := runBothy(t, nil, c.args...)
		first, _, _ := strings.Cut(stderr, "\n")
		if code != 2 || stdout != "" || first != c.want || !strings.HasSuffix(stderr, secretsUsage) {
			t.Errorf("%d args: exit %d, stdout %q, first line %q; want 2 and %q with the usage", len(c.args), code, stdout, first, c.want)
		}
		if strings.Contains(stdout+stderr, "EXAMPLEEXAMPLE") {
			t.Errorf("%d args: the output repeats the key", len(c.args))
		}
	}
}

// TestSecretsInitAdoptSourceNeedsSource holds that --adopt-source without --source is a usage
// error (ruling M1-09/18): without a source the flag does nothing, and a restore would generate
// new secrets instead of adopting the bundle.
func TestSecretsInitAdoptSourceNeedsSource(t *testing.T) {
	t.Parallel()
	target := filepath.Join(t.TempDir(), "volume")
	code, stdout, stderr := runBothy(t, nil, "secrets", "init", "--target", target, "--adopt-source")
	want := "bothy secrets init: --adopt-source needs --source\n" + secretsUsage
	if code != 2 || stdout != "" || stderr != want {
		t.Errorf("exit %d, stdout %q, stderr %q; want 2 and %q", code, stdout, stderr, want)
	}
	if _, err := os.Stat(target); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("the target was created: %v", err)
	}
}
