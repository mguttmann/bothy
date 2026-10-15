// SPDX-License-Identifier: Apache-2.0

//go:build unix

package stack_test

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"io"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"syscall"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/platform/secrets/stack"
)

var (
	tokenEntries = []string{
		"db_superuser_password", "db_owner_password", "db_app_password", "db_maint_password",
		"db_monitor_password", "pgbackrest_cipher_pass", "restic_password", "grafana_admin_password",
	}
	keyEntries = []string{"token_pepper", "totp_key", "telemetry_pseudonym_key", "audit_ip_kek", "file_ticket_key"}
)

// freshLines is the report of a run on an empty target and an empty source, in catalog order
// (configuration.md section 5).
var freshLines = []string{
	"generated db_superuser_password",
	"generated db_owner_password",
	"generated db_app_password",
	"generated db_maint_password",
	"generated db_monitor_password",
	"generated token_pepper",
	"generated totp_key",
	"generated telemetry_pseudonym_key",
	"generated audit_ip_kek",
	"generated file_ticket_key",
	"generated pgbackrest_cipher_pass",
	"generated pgbackrest_tls/",
	"generated restic_password",
	"pending s3_access_key",
	"pending s3_secret_key",
	"generated grafana_admin_password",
	"missing apns_key.p8",
	"missing fcm_service_account.json",
	"missing oidc/<provider>_client_secret",
	"missing oidc/<provider>_client_key.pem",
	"missing offsite/",
	"missing alertmanager_bothy_webhook_url",
	"missing alertmanager_external_webhook_url",
	"missing alertmanager_heartbeat_url",
	"missing newrelic_license_key",
	"missing grafana_otlp_authorization",
	"missing datadog_api_key",
	"missing azure_client_secret",
	"missing sentry_key",
}

// freshServiceFiles is the expected content of every service directory after a fresh run: the
// generated entries whose row in configuration.md lists the service.
var freshServiceFiles = map[string][]string{
	"alertmanager": nil,
	"backup":       {"db_superuser_password", "pgbackrest_cipher_pass", "pgbackrest_tls/backup.crt", "pgbackrest_tls/backup.key", "pgbackrest_tls/ca.crt", "pgbackrest_tls/postgres.crt", "pgbackrest_tls/postgres.key", "restic_password"},
	"collector":    {"db_monitor_password", "telemetry_pseudonym_key"},
	"grafana":      {"grafana_admin_password"},
	"migrate":      {"db_app_password", "db_maint_password", "db_monitor_password", "db_owner_password", "db_superuser_password"},
	"postgres":     {"db_superuser_password", "pgbackrest_tls/backup.crt", "pgbackrest_tls/backup.key", "pgbackrest_tls/ca.crt", "pgbackrest_tls/postgres.crt", "pgbackrest_tls/postgres.key"},
	"seaweedfs":    nil,
	"server":       {"audit_ip_kek", "db_app_password", "db_maint_password", "file_ticket_key", "telemetry_pseudonym_key", "token_pepper", "totp_key"},
}

func TestInitFreshTarget(t *testing.T) {
	t.Parallel()
	target, source := dirs(t)
	r := mustInit(t, opts(target, source))
	store := filepath.Join(target, "_store")

	if got := perm(t, target); got != fs.ModeDir|0o700 {
		t.Errorf("target mode %v, want drwx------", got)
	}
	if got := perm(t, store); got != fs.ModeDir|0o700 {
		t.Errorf("_store mode %v, want drwx------", got)
	}
	if got := owner(t, store); got != me() {
		t.Errorf("_store owner %v, want %v", got, me())
	}
	for _, name := range tokenEntries {
		value := read(t, store, name)
		raw, err := base64.RawURLEncoding.DecodeString(value)
		if err != nil || len(raw) != 32 || len(value) != 43 {
			t.Errorf("%s is not 32 bytes base64url without padding (len %d, err %v)", name, len(value), err)
		}
	}
	for _, name := range keyEntries {
		value := read(t, store, name)
		raw, err := hex.DecodeString(value)
		if err != nil || len(raw) != 32 || value != strings.ToLower(value) {
			t.Errorf("%s is not 32 bytes lowercase hex (len %d, err %v)", name, len(value), err)
		}
	}
	for _, name := range files(t, store) {
		if got := perm(t, filepath.Join(store, name)); got != 0o400 {
			t.Errorf("_store/%s mode %v, want 0400", name, got)
		}
		if got := owner(t, filepath.Join(store, name)); got != me() {
			t.Errorf("_store/%s owner %v, want %v", name, got, me())
		}
	}
	if got, want := files(t, store), slices.Sorted(slices.Values(slices.Concat(tokenEntries, keyEntries, tlsFiles))); !slices.Equal(got, want) {
		t.Errorf("_store holds %v, want %v", got, want)
	}

	for _, service := range stack.Services() {
		dir := filepath.Join(target, service)
		wantDir, wantFile := fs.ModeDir|0o755, fs.FileMode(0o444)
		if service == "server" || service == "migrate" {
			wantDir, wantFile = fs.ModeDir|0o500, 0o400
		}
		if got := perm(t, dir); got != wantDir {
			t.Errorf("%s/ mode %v, want %v", service, got, wantDir)
		}
		if got := owner(t, dir); got != me() {
			t.Errorf("%s/ owner %v, want %v", service, got, me())
		}
		got := files(t, dir)
		if !slices.Equal(got, freshServiceFiles[service]) {
			t.Errorf("%s/ holds %v, want %v", service, got, freshServiceFiles[service])
		}
		for _, name := range got {
			if m := perm(t, filepath.Join(dir, name)); m != wantFile {
				t.Errorf("%s/%s mode %v, want %v", service, name, m, wantFile)
			}
			if o := owner(t, filepath.Join(dir, name)); o != me() {
				t.Errorf("%s/%s owner %v, want %v", service, name, o, me())
			}
			if read(t, dir, name) != read(t, store, name) {
				t.Errorf("%s/%s differs from _store", service, name)
			}
		}
	}

	var got []string
	for _, l := range r.Lines {
		got = append(got, string(l.Status)+" "+l.Name)
		switch {
		case l.Status == stack.StatusGenerated && strings.HasSuffix(l.Name, "/"):
			// A directory entry's fingerprint covers its files (TestPGBackRestTLSKeptOnSecondRun).
			if !regexp.MustCompile(`^sha256:[0-9a-f]{12}$`).MatchString(l.Fingerprint) {
				t.Errorf("%s fingerprint %q", l.Name, l.Fingerprint)
			}
		case l.Status == stack.StatusGenerated:
			if want := fingerprint(read(t, store, l.Name)); l.Fingerprint != want {
				t.Errorf("%s fingerprint %q, want %q", l.Name, l.Fingerprint, want)
			}
		case l.Fingerprint != "":
			t.Errorf("%s %s has fingerprint %q", l.Status, l.Name, l.Fingerprint)
		}
	}
	if !slices.Equal(got, freshLines) {
		t.Errorf("report lines\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(freshLines, "\n"))
	}
	text := reportText(t, r)
	reminder := "back up the secrets: bothy secrets export --from " + store + " --age-recipient <age public key>"
	if r.Reminder() != reminder || !strings.HasSuffix(text, "\n"+reminder+"\n") {
		t.Errorf("report does not end with %q:\n%s", reminder, text)
	}
}

func TestPseudonymKeyAtLeast32Bytes(t *testing.T) {
	t.Parallel()
	target, _ := dirs(t)
	mustInit(t, stack.InitOptions{Target: target, Owners: owners()})
	for _, path := range []string{"_store/telemetry_pseudonym_key", "server/telemetry_pseudonym_key", "collector/telemetry_pseudonym_key"} {
		raw, err := hex.DecodeString(read(t, target, path))
		if err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		if len(raw) < 32 {
			t.Errorf("%s decodes to %d bytes, want at least 32", path, len(raw))
		}
	}
}

func TestReportWriteTo(t *testing.T) {
	t.Parallel()
	r := stack.Report{Store: "/run/bothy/secrets/_store", Lines: []stack.Line{
		{Status: stack.StatusGenerated, Name: "db_app_password", Fingerprint: "sha256:aaaaaaaaaaaa"},
		{Status: stack.StatusMissing, Name: "apns_key.p8"},
	}}
	var b bytes.Buffer
	n, err := r.WriteTo(&b)
	if err != nil {
		t.Fatal(err)
	}
	want := "generated db_app_password sha256:aaaaaaaaaaaa\nmissing apns_key.p8\n" +
		"back up the secrets: bothy secrets export --from /run/bothy/secrets/_store --age-recipient <age public key>\n"
	if b.String() != want {
		t.Errorf("WriteTo wrote\n%q\nwant\n%q", b.String(), want)
	}
	if n != int64(len(want)) {
		t.Errorf("WriteTo returned %d, want %d", n, len(want))
	}
	if got := r.Lines[1].String(); got != "missing apns_key.p8" {
		t.Errorf("Line.String() = %q", got)
	}
	// The reminder is a command to copy (brief D8 makes --from mandatory): a path that is no
	// plain shell word is single-quoted, an unprintable character escaped, and without a store
	// the line names the place to fill in.
	for store, want := range map[string]string{
		"":                      "--from <volume>/_store ",
		"/srv/my vol/_store":    "--from '/srv/my vol/_store' ",
		"/srv/it's/_store":      `--from '/srv/it'\''s/_store' `,
		"/srv/a\nb/_store":      `--from '/srv/a\nb/_store' `,
		"/srv/plain_1.2/_store": "--from /srv/plain_1.2/_store ",
	} {
		if got := (stack.Report{Store: store}).Reminder(); !strings.Contains(got, want) || strings.Contains(got, "\n") {
			t.Errorf("Reminder with store %q = %q, want it to contain %q on one line", store, got, want)
		}
	}
}

func TestInitSecondRunChangesNothing(t *testing.T) {
	t.Parallel()
	target, source := dirs(t)
	put(t, source, "apns_key.p8", "EXAMPLE-apns", 0o600)
	put(t, source, "oidc/alpha_client_secret", "EXAMPLE-oidc", 0o600)
	put(t, source, "offsite/ssh/id_ed25519", "EXAMPLE-offsite", 0o600)
	mustInit(t, opts(target, source))
	before := snapshot(t, target)

	r := mustInit(t, opts(target, source))
	diffSnapshots(t, before, snapshot(t, target))
	for _, l := range r.Lines {
		if l.Status != stack.StatusKept && l.Status != stack.StatusMissing && l.Status != stack.StatusPending {
			t.Errorf("second run reports %s %s", l.Status, l.Name)
		}
	}
	if got := statuses(r)["offsite/"]; got != "kept "+fingerprint("ssh/id_ed25519 "+sha("EXAMPLE-offsite")+"\n") {
		t.Errorf("offsite/ is %q, want kept with the tree fingerprint", got)
	}
}

func TestInitReconcileTable(t *testing.T) {
	t.Parallel()
	const generated, supplied = "db_app_password", "apns_key.p8"
	type cell struct {
		name    string
		entry   string
		store   string // "" absent, else content placed by a first run
		source  string // "" absent
		adopt   bool
		want    string // status, "" for the conflict
		content string // store content afterwards, "" to compare with the first run
	}
	cells := []cell{
		{name: "generated/absent/absent", entry: generated, want: "generated"},
		{name: "generated/absent/present", entry: generated, source: "EXAMPLE-src", want: "adopted", content: "EXAMPLE-src"},
		{name: "generated/present/absent", entry: generated, store: "EXAMPLE-old", want: "kept", content: "EXAMPLE-old"},
		{name: "generated/present/equal", entry: generated, store: "EXAMPLE-old", source: "EXAMPLE-old", want: "kept", content: "EXAMPLE-old"},
		{name: "generated/present/different", entry: generated, store: "EXAMPLE-old", source: "EXAMPLE-new"},
		{name: "generated/present/different/adopt-source", entry: generated, store: "EXAMPLE-old", source: "EXAMPLE-new", adopt: true, want: "updated", content: "EXAMPLE-new"},
		{name: "supplied/absent/absent", entry: supplied, want: "missing"},
		{name: "supplied/absent/present", entry: supplied, source: "EXAMPLE-src", want: "adopted", content: "EXAMPLE-src"},
		{name: "supplied/present/absent", entry: supplied, store: "EXAMPLE-old", want: "kept", content: "EXAMPLE-old"},
		{name: "supplied/present/equal", entry: supplied, store: "EXAMPLE-old", source: "EXAMPLE-old", want: "kept", content: "EXAMPLE-old"},
		{name: "supplied/present/different", entry: supplied, store: "EXAMPLE-old", source: "EXAMPLE-new", want: "updated", content: "EXAMPLE-new"},
	}
	for _, c := range cells {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			target, source := dirs(t)
			if c.store != "" {
				put(t, source, c.entry, c.store, 0o600)
				mustInit(t, opts(target, source))
				remove(t, source, c.entry)
			}
			if c.source != "" {
				put(t, source, c.entry, c.source, 0o600)
			}
			o := opts(target, source)
			o.AdoptSource = c.adopt
			var before map[string]state
			if exists(t, target) {
				before = snapshot(t, target)
			}
			r, err := stack.Init(t.Context(), o)
			if c.want == "" {
				if !errors.Is(err, stack.ErrConflict) {
					t.Fatalf("Init error %v, want ErrConflict", err)
				}
				if !strings.Contains(err.Error(), c.entry) {
					t.Errorf("error %q does not name %s", err, c.entry)
				}
				if errors.Is(err, stack.ErrUsage) {
					t.Errorf("conflict is a usage error: %v", err)
				}
				diffSnapshots(t, before, snapshot(t, target))
				return
			}
			if err != nil {
				t.Fatalf("Init: %v", err)
			}
			got := statuses(r)[c.entry]
			if c.want == "missing" {
				if got != "missing" {
					t.Errorf("%s: %q, want missing", c.entry, got)
				}
				if exists(t, filepath.Join(target, "_store", c.entry)) {
					t.Errorf("missing entry %s exists in _store", c.entry)
				}
				return
			}
			stored := read(t, target, "_store/"+c.entry)
			if c.content != "" && stored != c.content {
				t.Errorf("_store/%s holds %q, want %q", c.entry, stored, c.content)
			}
			if want := c.want + " " + fingerprint(stored); got != want {
				t.Errorf("%s: %q, want %q", c.entry, got, want)
			}
			if copied := read(t, target, "server/"+c.entry); copied != stored {
				t.Errorf("server/%s differs from _store", c.entry)
			}
		})
	}
}

func TestInitConflictNamesAllEntriesAndWritesNothing(t *testing.T) {
	t.Parallel()
	target, source := dirs(t)
	mustInit(t, opts(target, source))
	put(t, source, "db_app_password", "EXAMPLE-new-app", 0o600)
	put(t, source, "token_pepper", "EXAMPLE-new-pepper", 0o600)
	put(t, source, "apns_key.p8", "EXAMPLE-apns", 0o600) // would be adopted without the conflict
	before := snapshot(t, target)
	_, err := stack.Init(t.Context(), opts(target, source))
	if !errors.Is(err, stack.ErrConflict) {
		t.Fatalf("Init error %v, want ErrConflict", err)
	}
	for _, name := range []string{"db_app_password", "token_pepper"} {
		if !strings.Contains(err.Error(), name) {
			t.Errorf("error %q does not name %s", err, name)
		}
	}
	diffSnapshots(t, before, snapshot(t, target))
}

func TestInitUnknownSourceFile(t *testing.T) {
	t.Parallel()
	for _, rel := range []string{"db_app_pasword", ".DS_Store", "oidc/Upper_client_secret", "oidc/nested/a_client_secret", "_store/db_app_password"} {
		t.Run(rel, func(t *testing.T) {
			t.Parallel()
			target, source := dirs(t)
			put(t, source, "apns_key.p8", "EXAMPLE-apns", 0o600)
			put(t, source, rel, "EXAMPLE-typo", 0o600)
			_, err := stack.Init(t.Context(), opts(target, source))
			if !errors.Is(err, stack.ErrUsage) {
				t.Fatalf("Init error %v, want ErrUsage", err)
			}
			if !strings.Contains(err.Error(), rel) {
				t.Errorf("error %q does not name %s", err, rel)
			}
			if strings.Contains(err.Error(), "EXAMPLE") {
				t.Errorf("error %q shows file content", err)
			}
			if exists(t, target) {
				t.Error("Init wrote to the target despite the usage error")
			}
		})
	}
}

// TestInitSkipsKeyStoreOfABundle: a decrypted bundle holds the key store under signing-keys/
// (export with --signing-key-dir, M1-32); init leaves that subtree to the restore of the key store
// volume instead of refusing it, and copies none of it.
func TestInitSkipsKeyStoreOfABundle(t *testing.T) {
	t.Parallel()
	target, source := dirs(t)
	put(t, source, "apns_key.p8", "EXAMPLE-apns", 0o600)
	put(t, source, stack.KeyStorePrefix+"invocation-EXAMPLE_kid.pem", "EXAMPLE-key", 0o600)
	mustInit(t, opts(target, source))
	err := filepath.WalkDir(target, func(path string, d fs.DirEntry, err error) error {
		if err == nil && strings.Contains(path, "signing-keys") {
			t.Errorf("init copied %s", path)
		}
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestInitRejectsSymlinkInSource(t *testing.T) {
	t.Parallel()
	cases := map[string]func(t *testing.T, source, outside string){
		"file link": func(t *testing.T, source, outside string) {
			if err := os.Symlink(filepath.Join(outside, "secret"), filepath.Join(source, "apns_key.p8")); err != nil {
				t.Fatal(err)
			}
		},
		"directory link": func(t *testing.T, source, outside string) {
			if err := os.Symlink(outside, filepath.Join(source, "offsite")); err != nil {
				t.Fatal(err)
			}
		},
		"nested link": func(t *testing.T, source, outside string) {
			if err := os.MkdirAll(filepath.Join(source, "offsite", "ssh"), 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(filepath.Join(outside, "secret"), filepath.Join(source, "offsite", "ssh", "id_ed25519")); err != nil {
				t.Fatal(err)
			}
		},
		"fifo": func(t *testing.T, source, _ string) {
			if err := syscall.Mkfifo(filepath.Join(source, "sentry_key"), 0o600); err != nil {
				t.Fatal(err)
			}
		},
	}
	for name, setup := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			target, source := dirs(t)
			outside := t.TempDir()
			put(t, outside, "secret", "EXAMPLE-outside", 0o644)
			setup(t, source, outside)
			_, err := stack.Init(t.Context(), opts(target, source))
			if !errors.Is(err, stack.ErrUsage) {
				t.Fatalf("Init error %v, want ErrUsage", err)
			}
			if !strings.Contains(err.Error(), "regular file") {
				t.Errorf("error %q does not say why", err)
			}
			if strings.Contains(err.Error(), "EXAMPLE") {
				t.Errorf("error %q shows file content", err)
			}
			if exists(t, target) {
				t.Error("Init wrote to the target")
			}
		})
	}
}

func TestInitRequiresOwnerForServerAndMigrate(t *testing.T) {
	t.Parallel()
	for _, missing := range []string{"server", "migrate"} {
		t.Run(missing, func(t *testing.T) {
			t.Parallel()
			target, source := dirs(t)
			o := opts(target, source)
			delete(o.Owners, missing)
			_, err := stack.Init(t.Context(), o)
			if !errors.Is(err, stack.ErrUsage) {
				t.Fatalf("Init error %v, want ErrUsage", err)
			}
			if !strings.Contains(err.Error(), missing) {
				t.Errorf("error %q does not name %s", err, missing)
			}
			if exists(t, target) {
				t.Error("Init wrote to the target")
			}
		})
	}
}

func TestInitUnknownOwnerService(t *testing.T) {
	t.Parallel()
	for _, service := range []string{"nginx", "_store", ""} {
		t.Run(service, func(t *testing.T) {
			t.Parallel()
			target, source := dirs(t)
			o := opts(target, source)
			o.Owners[service] = me()
			_, err := stack.Init(t.Context(), o)
			if !errors.Is(err, stack.ErrUsage) {
				t.Fatalf("Init error %v, want ErrUsage", err)
			}
			if exists(t, target) {
				t.Error("Init wrote to the target")
			}
		})
	}
}

func TestInitRejectsInvalidOptions(t *testing.T) {
	t.Parallel()
	base := t.TempDir()
	notDir := filepath.Join(base, "file")
	put(t, base, "file", "x", 0o600)
	cases := map[string]stack.InitOptions{
		"empty target":         {Target: "", Owners: owners()},
		"source missing":       {Target: filepath.Join(base, "t1"), Source: filepath.Join(base, "absent"), Owners: owners()},
		"source not directory": {Target: filepath.Join(base, "t2"), Source: notDir, Owners: owners()},
		"negative uid":         {Target: filepath.Join(base, "t3"), Owners: map[string]stack.Owner{"server": {UID: -1, GID: me().GID}, "migrate": me()}},
	}
	for name, o := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			_, err := stack.Init(t.Context(), o)
			if !errors.Is(err, stack.ErrUsage) {
				t.Fatalf("Init error %v, want ErrUsage", err)
			}
			if o.Target != "" && exists(t, o.Target) {
				t.Error("Init wrote to the target")
			}
		})
	}
}

func TestParseOwner(t *testing.T) {
	t.Parallel()
	valid := map[string]struct {
		service string
		owner   stack.Owner
	}{
		"server=65532:65532":       {"server", stack.Owner{UID: 65532, GID: 65532}},
		"backup=0:0":               {"backup", stack.Owner{}},
		"migrate=2147483647:1":     {"migrate", stack.Owner{UID: 2147483647, GID: 1}},
		"postgres=999:2147483647":  {"postgres", stack.Owner{UID: 999, GID: 2147483647}},
		"alertmanager=65534:65534": {"alertmanager", stack.Owner{UID: 65534, GID: 65534}},
	}
	for in, want := range valid {
		service, o, err := stack.ParseOwner(in)
		if err != nil || service != want.service || o != want.owner {
			t.Errorf("ParseOwner(%q) = %q, %v, %v; want %q, %v", in, service, o, err, want.service, want.owner)
		}
	}
	invalid := []string{
		"", "server", "server65532:65532", "=1:1", "server=", "server=1", "server=1:", "server=:1",
		"server=a:1", "server=1:b", "server=-1:1", "server=1:-1", "server=+1:1", "server=2147483648:1",
		"server=1:2147483648", "server=1:1:1", "server= 1:1",
	}
	for _, in := range invalid {
		if _, _, err := stack.ParseOwner(in); !errors.Is(err, stack.ErrUsage) {
			t.Errorf("ParseOwner(%q) error %v, want ErrUsage", in, err)
		}
	}
}

func TestInitForeignOwnerAsNonRoot(t *testing.T) {
	t.Parallel()
	if os.Geteuid() == 0 {
		t.Skip("running as root: every owner is allowed")
	}
	target, source := dirs(t)
	o := opts(target, source)
	o.Owners["server"] = stack.Owner{UID: 65532, GID: 65532}
	_, err := stack.Init(t.Context(), o)
	if err == nil {
		t.Fatal("Init accepted a foreign owner as non-root")
	}
	if errors.Is(err, stack.ErrUsage) {
		t.Errorf("foreign owner is a usage error (exit 2), want a runtime error: %v", err)
	}
	if !strings.Contains(err.Error(), "server") {
		t.Errorf("error %q does not name the service", err)
	}
	if exists(t, target) {
		t.Error("Init wrote to the target")
	}
}

func TestInitRemovesStaleServiceCopies(t *testing.T) {
	t.Parallel()
	target, source := dirs(t)
	put(t, source, "apns_key.p8", "EXAMPLE-apns", 0o600)
	mustInit(t, opts(target, source))
	put(t, target, "server/stale", "EXAMPLE-stale", 0o400)
	put(t, target, "server/old/nested", "EXAMPLE-nested", 0o400)
	put(t, target, "postgres/token_pepper", "EXAMPLE-misplaced", 0o444)
	put(t, target, "server/db_app_password", "EXAMPLE-tampered", 0o400)
	if err := os.Symlink(filepath.Join(target, "_store", "token_pepper"), filepath.Join(target, "grafana", "linked")); err != nil {
		t.Fatal(err)
	}
	remove(t, source, "apns_key.p8") // stays in _store as kept, so the copy stays too

	r := mustInit(t, opts(target, source))
	for _, rel := range []string{"server/stale", "server/old", "postgres/token_pepper", "grafana/linked"} {
		if exists(t, filepath.Join(target, rel)) {
			t.Errorf("%s was not removed", rel)
		}
	}
	if got, want := read(t, target, "server/db_app_password"), read(t, target, "_store/db_app_password"); got != want {
		t.Error("tampered copy server/db_app_password was not restored")
	}
	if got := read(t, target, "server/apns_key.p8"); got != "EXAMPLE-apns" {
		t.Errorf("server/apns_key.p8 holds %q", got)
	}
	if got := statuses(r)["apns_key.p8"]; got != "kept "+fingerprint("EXAMPLE-apns") {
		t.Errorf("apns_key.p8: %q", got)
	}
	if got := perm(t, filepath.Join(target, "server")); got != fs.ModeDir|0o500 {
		t.Errorf("server/ mode %v after the repair, want dr-x------", got)
	}
}

func TestInitRepairsModeDrift(t *testing.T) {
	t.Parallel()
	target, source := dirs(t)
	mustInit(t, opts(target, source))
	chmod(t, filepath.Join(target, "_store", "token_pepper"), 0o600)
	chmod(t, filepath.Join(target, "_store"), 0o750)
	chmod(t, filepath.Join(target, "server", "token_pepper"), 0o440)
	chmod(t, filepath.Join(target, "backup"), 0o700)
	chmod(t, target, 0o755)

	r := mustInit(t, opts(target, source))
	want := map[string]fs.FileMode{
		"":                    fs.ModeDir | 0o700,
		"_store":              fs.ModeDir | 0o700,
		"_store/token_pepper": 0o400,
		"server/token_pepper": 0o400,
		"backup":              fs.ModeDir | 0o755,
	}
	for rel, mode := range want {
		if got := perm(t, filepath.Join(target, rel)); got != mode {
			t.Errorf("%q mode %v, want %v", rel, got, mode)
		}
	}
	if got := statuses(r)["token_pepper"]; !strings.HasPrefix(got, "kept ") {
		t.Errorf("token_pepper: %q, want kept", got)
	}
}

func TestInitRemovesLeftoverTempFiles(t *testing.T) {
	t.Parallel()
	target, source := dirs(t)
	put(t, source, "offsite/ssh/id_ed25519", "EXAMPLE-offsite", 0o600)
	mustInit(t, opts(target, source))
	before := snapshot(t, target)
	leftovers := []string{
		"_store/.bothy-tmp-123456",
		"_store/offsite/ssh/.bothy-tmp-abc",
		"server/.bothy-tmp-987",
		"backup/offsite/.bothy-tmp-x",
		"alertmanager/.bothy-tmp-y",
	}
	for _, rel := range leftovers {
		put(t, target, rel, "EXAMPLE-partial", 0o600)
	}

	r := mustInit(t, opts(target, source))
	for _, rel := range leftovers {
		if exists(t, filepath.Join(target, rel)) {
			t.Errorf("leftover %s was not removed", rel)
		}
	}
	// Planting and removing a file changes the mtime and size of its directory; everything else
	// must be as before.
	after := snapshot(t, target)
	for _, rel := range leftovers {
		dir := filepath.ToSlash(filepath.Dir(rel))
		for _, m := range []map[string]state{before, after} {
			s := m[dir]
			s.mtime, s.size = 0, 0
			m[dir] = s
		}
	}
	diffSnapshots(t, before, after)
	for _, l := range r.Lines {
		if l.Status != stack.StatusKept && l.Status != stack.StatusMissing && l.Status != stack.StatusPending {
			t.Errorf("run after the leftovers reports %s %s", l.Status, l.Name)
		}
	}
}

func TestInitConvergesAfterInterruptedAdoption(t *testing.T) {
	t.Parallel()
	target, source := dirs(t)
	mustInit(t, opts(target, source))
	// A crash between temp file and rename leaves the temp file and no entry.
	put(t, target, "_store/.bothy-tmp-crash", "EXAMPLE-apns", 0o600)
	put(t, source, "apns_key.p8", "EXAMPLE-apns", 0o600)
	r := mustInit(t, opts(target, source))
	if got := statuses(r)["apns_key.p8"]; got != "adopted "+fingerprint("EXAMPLE-apns") {
		t.Errorf("apns_key.p8: %q", got)
	}
	if exists(t, filepath.Join(target, "_store", ".bothy-tmp-crash")) {
		t.Error("leftover was not removed")
	}
	if got := read(t, target, "server/apns_key.p8"); got != "EXAMPLE-apns" {
		t.Errorf("server/apns_key.p8 holds %q", got)
	}
}

func TestInitPatternAndDirectoryEntries(t *testing.T) {
	t.Parallel()
	target, source := dirs(t)
	put(t, source, "oidc/alpha_client_secret", "EXAMPLE-alpha", 0o600)
	put(t, source, "oidc/beta_client_secret", "EXAMPLE-beta", 0o400)
	put(t, source, "oidc/alpha_client_key.pem", "EXAMPLE-alpha-key", 0o600)
	put(t, source, "offsite/s3_key", "EXAMPLE-s3", 0o600)
	put(t, source, "offsite/ssh/id_ed25519", "EXAMPLE-ssh", 0o600)
	put(t, source, "offsite/legacy/old/key", "EXAMPLE-legacy", 0o600)

	r := mustInit(t, opts(target, source))
	dirFP := fingerprint("legacy/old/key " + sha("EXAMPLE-legacy") + "\ns3_key " + sha("EXAMPLE-s3") +
		"\nssh/id_ed25519 " + sha("EXAMPLE-ssh") + "\n")
	want := map[string]string{
		"oidc/alpha_client_secret":       "adopted " + fingerprint("EXAMPLE-alpha"),
		"oidc/beta_client_secret":        "adopted " + fingerprint("EXAMPLE-beta"),
		"oidc/alpha_client_key.pem":      "adopted " + fingerprint("EXAMPLE-alpha-key"),
		"offsite/":                       "adopted " + dirFP,
		"oidc/<provider>_client_secret":  "",
		"oidc/<provider>_client_key.pem": "",
	}
	got := statuses(r)
	for name, w := range want {
		if got[name] != w {
			t.Errorf("%s: %q, want %q", name, got[name], w)
		}
	}
	var order []string
	for _, l := range r.Lines {
		if strings.HasPrefix(l.Name, "oidc/") {
			order = append(order, l.Name)
		}
	}
	if want := []string{"oidc/alpha_client_secret", "oidc/beta_client_secret", "oidc/alpha_client_key.pem"}; !slices.Equal(order, want) {
		t.Errorf("oidc lines %v, want %v", order, want)
	}
	if got, want := files(t, filepath.Join(target, "server", "oidc")), []string{"alpha_client_key.pem", "alpha_client_secret", "beta_client_secret"}; !slices.Equal(got, want) {
		t.Errorf("server/oidc holds %v, want %v", got, want)
	}
	if got := perm(t, filepath.Join(target, "server", "oidc")); got != fs.ModeDir|0o500 {
		t.Errorf("server/oidc mode %v", got)
	}
	if got, want := files(t, filepath.Join(target, "backup", "offsite")), []string{"legacy/old/key", "s3_key", "ssh/id_ed25519"}; !slices.Equal(got, want) {
		t.Errorf("backup/offsite holds %v, want %v", got, want)
	}
	if got := perm(t, filepath.Join(target, "backup", "offsite", "ssh")); got != fs.ModeDir|0o755 {
		t.Errorf("backup/offsite/ssh mode %v", got)
	}
	if got := perm(t, filepath.Join(target, "_store", "offsite", "ssh")); got != fs.ModeDir|0o700 {
		t.Errorf("_store/offsite/ssh mode %v", got)
	}
	if exists(t, filepath.Join(target, "server", "offsite")) {
		t.Error("offsite/ was copied to server")
	}

	// beta leaves the source: kept. One offsite file changes and two leave, one of them nested:
	// updated, the tree in _store and backup follows the source, emptied directories go too.
	remove(t, source, "oidc/beta_client_secret")
	put(t, source, "offsite/ssh/id_ed25519", "EXAMPLE-ssh-rotated", 0o600)
	remove(t, source, "offsite/s3_key")
	remove(t, source, "offsite/legacy")
	r = mustInit(t, opts(target, source))
	got = statuses(r)
	if got["oidc/beta_client_secret"] != "kept "+fingerprint("EXAMPLE-beta") {
		t.Errorf("oidc/beta_client_secret: %q, want kept", got["oidc/beta_client_secret"])
	}
	newFP := fingerprint("ssh/id_ed25519 " + sha("EXAMPLE-ssh-rotated") + "\n")
	if got["offsite/"] != "updated "+newFP || newFP == dirFP {
		t.Errorf("offsite/: %q, want updated %s", got["offsite/"], newFP)
	}
	for _, dir := range []string{"_store/offsite", "backup/offsite"} {
		if got, want := files(t, filepath.Join(target, dir)), []string{"ssh/id_ed25519"}; !slices.Equal(got, want) {
			t.Errorf("%s holds %v, want %v", dir, got, want)
		}
		if got := read(t, target, dir+"/ssh/id_ed25519"); got != "EXAMPLE-ssh-rotated" {
			t.Errorf("%s/ssh/id_ed25519 holds %q", dir, got)
		}
		if exists(t, filepath.Join(target, dir, "legacy")) {
			t.Errorf("%s/legacy was not removed", dir)
		}
	}
	if got := read(t, target, "server/oidc/beta_client_secret"); got != "EXAMPLE-beta" {
		t.Errorf("server/oidc/beta_client_secret holds %q", got)
	}
}

// TestInitDirectoryEntryChangesFileAndDirectory: a path below a directory entry that turns from a
// file into a directory or back is replaced like any other change (D4 "supplied, present,
// different: updated"), in _store and in the service copy, and the next run changes nothing. An
// empty directory an interrupted run left in _store does not block a file of the same name either.
func TestInitDirectoryEntryChangesFileAndDirectory(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct {
		before, after map[string]string
		plant         []string // empty directories left in _store before the second run
	}{
		"file becomes directory": {
			before: map[string]string{"offsite/ssh": "EXAMPLE-ssh-file", "offsite/s3_key": "EXAMPLE-s3"},
			after:  map[string]string{"offsite/ssh/id_ed25519": "EXAMPLE-ssh-key", "offsite/s3_key": "EXAMPLE-s3"},
		},
		"directory becomes file": {
			before: map[string]string{
				"offsite/ssh/id_ed25519": "EXAMPLE-ssh-key", "offsite/ssh/known/hosts": "EXAMPLE-hosts",
				"offsite/s3_key": "EXAMPLE-s3",
			},
			after: map[string]string{"offsite/ssh": "EXAMPLE-ssh-file", "offsite/s3_key": "EXAMPLE-s3"},
		},
		"empty directory left where a file goes": {
			before: map[string]string{"offsite/s3_key": "EXAMPLE-s3"},
			after:  map[string]string{"offsite/ssh": "EXAMPLE-ssh-file", "offsite/s3_key": "EXAMPLE-s3"},
			plant:  []string{"_store/offsite/ssh/empty"},
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			target, source := dirs(t)
			for rel, content := range tc.before {
				put(t, source, rel, content, 0o600)
			}
			mustInit(t, opts(target, source))
			for _, rel := range tc.plant {
				if err := os.MkdirAll(filepath.Join(target, filepath.FromSlash(rel)), 0o700); err != nil {
					t.Fatal(err)
				}
			}
			remove(t, source, "offsite")
			var lines string
			var want []string
			for _, rel := range slices.Sorted(maps.Keys(tc.after)) {
				put(t, source, rel, tc.after[rel], 0o600)
				inside := strings.TrimPrefix(rel, "offsite/")
				want = append(want, inside)
				lines += inside + " " + sha(tc.after[rel]) + "\n"
			}

			r, err := stack.Init(t.Context(), opts(target, source))
			if err != nil {
				t.Fatalf("Init after the change: %v", err)
			}
			if got := statuses(r)["offsite/"]; got != "updated "+fingerprint(lines) {
				t.Errorf("offsite/: %q, want updated %s", got, fingerprint(lines))
			}
			for _, dir := range []string{"_store/offsite", "backup/offsite"} {
				if got := files(t, filepath.Join(target, dir)); !slices.Equal(got, want) {
					t.Errorf("%s holds %v, want %v", dir, got, want)
				}
				for _, inside := range want {
					if got := read(t, target, dir+"/"+inside); got != tc.after["offsite/"+inside] {
						t.Errorf("%s/%s holds %q", dir, inside, got)
					}
				}
			}

			before := snapshot(t, target)
			r = mustInit(t, opts(target, source))
			if got := statuses(r)["offsite/"]; got != "kept "+fingerprint(lines) {
				t.Errorf("offsite/ on the next run: %q, want kept", got)
			}
			diffSnapshots(t, before, snapshot(t, target))
		})
	}
}

func TestInitOutputHasNoValues(t *testing.T) {
	t.Parallel()
	target, source := dirs(t)
	const canary = "EXAMPLE-canary-5f1c0de"
	put(t, source, "apns_key.p8", canary, 0o600)
	put(t, source, "oidc/alpha_client_secret", canary+"-oidc", 0o600)
	r := mustInit(t, opts(target, source))
	text := reportText(t, r)
	if strings.Contains(text, canary) {
		t.Errorf("report shows a supplied value:\n%s", text)
	}
	for _, name := range append(slices.Clone(tokenEntries), keyEntries...) {
		value := read(t, target, "_store/"+name)
		if strings.Contains(text, value) || strings.Contains(text, value[:12]) {
			t.Errorf("report shows the value of %s", name)
		}
	}
	for _, line := range strings.Split(strings.TrimSuffix(text, "\n"), "\n") {
		if line == r.Reminder() {
			continue
		}
		if !regexp.MustCompile(`^(generated|adopted|kept|updated) \S+ sha256:[0-9a-f]{12}$|^(missing|pending) \S+$`).MatchString(line) {
			t.Errorf("unexpected report line %q", line)
		}
	}
}

type failingReader struct{}

func (failingReader) Read([]byte) (int, error) { return 0, errors.New("EXAMPLE entropy failure") }

// halfReader returns some bytes and then fails, like an entropy source that breaks mid-read.
type halfReader struct{ n int }

func (h *halfReader) Read(p []byte) (int, error) {
	if h.n > 0 {
		n := min(h.n, len(p))
		h.n -= n
		return n, nil
	}
	return 0, io.ErrUnexpectedEOF
}

func TestInitRandFailure(t *testing.T) {
	t.Parallel()
	for name, rnd := range map[string]io.Reader{"fails": failingReader{}, "fails mid-read": &halfReader{n: 40}} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			target, source := dirs(t)
			o := opts(target, source)
			o.Rand = rnd
			if _, err := stack.Init(t.Context(), o); err == nil {
				t.Fatal("Init succeeded with a failing random source")
			}
			if exists(t, target) {
				if got := files(t, target); len(got) != 0 {
					t.Errorf("target holds files after the failure: %v", got)
				}
			}
		})
	}
}

func TestInitHonoursCanceledContext(t *testing.T) {
	t.Parallel()
	target, source := dirs(t)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := stack.Init(ctx, opts(target, source)); err == nil {
		t.Fatal("Init ran with a canceled context")
	}
	if exists(t, target) {
		t.Error("Init wrote to the target")
	}
}

func TestInitLeavesUnknownStoreFilesAlone(t *testing.T) {
	t.Parallel()
	target, source := dirs(t)
	mustInit(t, opts(target, source))
	put(t, target, "_store/notes.txt", "EXAMPLE-notes", 0o640)
	before := snapshot(t, target)
	mustInit(t, opts(target, source))
	diffSnapshots(t, before, snapshot(t, target))
	for _, service := range stack.Services() {
		if exists(t, filepath.Join(target, service, "notes.txt")) {
			t.Errorf("unknown store file copied to %s", service)
		}
	}
}

func TestInitRejectsNonRegularInStore(t *testing.T) {
	t.Parallel()
	target, source := dirs(t)
	mustInit(t, opts(target, source))
	outside := t.TempDir()
	put(t, outside, "secret", "EXAMPLE-outside", 0o600)
	remove(t, target, "_store/token_pepper")
	if err := os.Symlink(filepath.Join(outside, "secret"), filepath.Join(target, "_store", "token_pepper")); err != nil {
		t.Fatal(err)
	}
	before := snapshot(t, target)
	_, err := stack.Init(t.Context(), opts(target, source))
	if err == nil {
		t.Fatal("Init accepted a link in _store")
	}
	if !strings.Contains(err.Error(), "_store/token_pepper is not a regular file") || strings.Contains(err.Error(), "EXAMPLE") {
		t.Errorf("error %q does not name the entry and the reason, or shows content", err)
	}
	diffSnapshots(t, before, snapshot(t, target))
}

func TestInitReplacesLinkedServiceDirectory(t *testing.T) {
	t.Parallel()
	target, source := dirs(t)
	mustInit(t, opts(target, source))
	outside := t.TempDir()
	put(t, outside, "keep", "EXAMPLE-outside", 0o600)
	remove(t, target, "grafana")
	if err := os.Symlink(outside, filepath.Join(target, "grafana")); err != nil {
		t.Fatal(err)
	}
	put(t, target, "seaweedfs-file", "x", 0o600) // not a service: left alone
	mustInit(t, opts(target, source))
	if got := perm(t, filepath.Join(target, "grafana")); got != fs.ModeDir|0o755 {
		t.Errorf("grafana is %v, want a directory with mode 0755", got)
	}
	if got := files(t, outside); !slices.Equal(got, []string{"keep"}) {
		t.Errorf("Init wrote through the link: %v", got)
	}
	if got := files(t, filepath.Join(target, "grafana")); !slices.Equal(got, []string{"grafana_admin_password"}) {
		t.Errorf("grafana/ holds %v", got)
	}
}

func TestInitTargetNotDirectory(t *testing.T) {
	t.Parallel()
	base := t.TempDir()
	put(t, base, "target", "EXAMPLE", 0o600)
	_, err := stack.Init(t.Context(), stack.InitOptions{Target: filepath.Join(base, "target"), Owners: owners()})
	if err == nil {
		t.Fatal("Init accepted a file as target")
	}
	if got := read(t, base, "target"); got != "EXAMPLE" {
		t.Errorf("target file changed to %q", got)
	}
}

// TestInitErrorsEscapeControlCharacters: a file name from the owner's source can reach an error
// through an operating system message; a newline in it must not forge a line in a terminal or log.
func TestInitErrorsEscapeControlCharacters(t *testing.T) {
	t.Parallel()
	if os.Geteuid() == 0 {
		t.Skip("running as root: an unreadable file cannot be simulated")
	}
	target, source := dirs(t)
	put(t, source, "offsite/forged\nwarning: all good", "EXAMPLE-offsite", 0o000)
	_, err := stack.Init(t.Context(), opts(target, source))
	if err == nil {
		t.Fatal("Init read an unreadable source file")
	}
	if strings.ContainsAny(err.Error(), "\n\r") {
		t.Errorf("error message contains a raw control character: %q", err.Error())
	}
	if !strings.Contains(err.Error(), `forged\nwarning`) {
		t.Errorf("error message %q does not name the file in escaped form", err.Error())
	}
	if !errors.Is(err, fs.ErrPermission) {
		t.Errorf("error %v does not wrap the cause", err)
	}
}

// TestInitRejectsEmptySourceValues holds that a supplied file without a value (empty, or only
// the line end the file driver strips) is a usage error naming every such file: an empty
// password or key is never valid (docs/DECISIONS.md M1-09/2), and init must not hand it to the
// services that read these files directly. Files below a directory entry may be empty.
func TestInitRejectsEmptySourceValues(t *testing.T) {
	t.Parallel()
	target, source := dirs(t)
	put(t, source, "grafana_admin_password", "", 0o600)
	put(t, source, "db_superuser_password", "\n", 0o600)
	put(t, source, "oidc/alpha_client_secret", "\r\n", 0o600)
	put(t, source, "apns_key.p8", "EXAMPLE-apns\n", 0o600)
	put(t, source, "offsite/known_hosts", "", 0o600)
	_, err := stack.Init(t.Context(), opts(target, source))
	if !errors.Is(err, stack.ErrUsage) {
		t.Fatalf("Init error %v, want ErrUsage", err)
	}
	for _, name := range []string{"db_superuser_password", "grafana_admin_password", "oidc/alpha_client_secret"} {
		if !strings.Contains(err.Error(), name) {
			t.Errorf("error %q does not name %s", err, name)
		}
	}
	for _, name := range []string{"apns_key.p8", "offsite"} {
		if strings.Contains(err.Error(), name) {
			t.Errorf("error %q names %s, which has a value", err, name)
		}
	}
	if _, statErr := os.Stat(target); !errors.Is(statErr, fs.ErrNotExist) {
		t.Errorf("the target was created: %v", statErr)
	}

	// Without the empty files the same source is adopted, the empty file below offsite/ too.
	remove(t, source, "grafana_admin_password")
	remove(t, source, "db_superuser_password")
	remove(t, source, "oidc")
	r := mustInit(t, opts(target, source))
	if got := statuses(r); !strings.HasPrefix(got["offsite/"], "adopted ") || !strings.HasPrefix(got["apns_key.p8"], "adopted ") {
		t.Errorf("statuses %v, want offsite/ and apns_key.p8 adopted", got)
	}
}

// TestInitRefusesEmptyStoredValue holds that a value in _store that is empty (only possible by
// a change outside init) stops the run with an error naming it instead of being kept and
// copied forever.
func TestInitRefusesEmptyStoredValue(t *testing.T) {
	t.Parallel()
	target, source := dirs(t)
	mustInit(t, opts(target, source))
	store := filepath.Join(target, "_store")
	restore := openDir(t, store)
	chmod(t, filepath.Join(store, "token_pepper"), 0o600)
	if err := os.WriteFile(filepath.Join(store, "token_pepper"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	chmod(t, filepath.Join(store, "token_pepper"), 0o400)
	restore()
	before := snapshot(t, target)
	_, err := stack.Init(t.Context(), opts(target, source))
	if err == nil || errors.Is(err, stack.ErrUsage) || !strings.Contains(err.Error(), "token_pepper") {
		t.Fatalf("Init error %v, want a runtime error naming token_pepper", err)
	}
	diffSnapshots(t, before, snapshot(t, target))
}

// TestInitAsRoot runs the production path of the Compose stack, which runs secrets-init as root
// with the server user 65532:65532 (docs/spec/16-betrieb.md 5.3 item 4): copies of server and
// migrate belong to that user, the others to root, the volume root to root with mode 0700, and
// a second run changes nothing. It needs root, so it runs only there (CI runs it as root; a
// developer machine skips it).
func TestInitAsRoot(t *testing.T) {
	t.Parallel()
	if os.Geteuid() != 0 {
		t.Skip("needs root: only root can give copies to uid 65532 and to root")
	}
	target, source := dirs(t)
	if err := os.Mkdir(target, 0o750); err != nil {
		t.Fatal(err)
	}
	// A volume root that belongs to another user gets root as owner, not only mode 0700.
	if err := os.Chown(target, 65533, 65533); err != nil {
		t.Fatal(err)
	}
	server := stack.Owner{UID: 65532, GID: 65532}
	o := stack.InitOptions{Target: target, Source: source, Owners: map[string]stack.Owner{"server": server, "migrate": server}}
	mustInit(t, o)
	if got := owner(t, target); got != (stack.Owner{}) || perm(t, target) != fs.ModeDir|0o700 {
		t.Errorf("volume root owner %v mode %v, want root and drwx------", got, perm(t, target))
	}
	for _, service := range stack.Services() {
		want := stack.Owner{}
		if service == "server" || service == "migrate" {
			want = server
		}
		dir := filepath.Join(target, service)
		if got := owner(t, dir); got != want {
			t.Errorf("%s/ owner %v, want %v", service, got, want)
		}
		for _, name := range files(t, dir) {
			if got := owner(t, filepath.Join(dir, name)); got != want {
				t.Errorf("%s/%s owner %v, want %v", service, name, got, want)
			}
		}
	}
	before := snapshot(t, target)
	mustInit(t, o)
	diffSnapshots(t, before, snapshot(t, target))
}

// TestInitRefusesForeignTarget holds that init prepares only an empty directory or an existing
// secrets volume (docs/DECISIONS.md M1-09/29): a mistyped --target such as /var/lib, which holds
// files but no _store, is a usage error before anything changes, because the run would otherwise
// take the directory over and empty every subdirectory named like a service. A volume root that
// holds only lost+found (a dedicated file system) counts as empty.
func TestInitRefusesForeignTarget(t *testing.T) {
	t.Parallel()
	target, source := dirs(t)
	put(t, target, "backup/EXAMPLE-data", "EXAMPLE-not-a-secret", 0o600)
	put(t, target, "notes", "EXAMPLE-notes", 0o600)
	before := snapshot(t, target)
	_, err := stack.Init(t.Context(), opts(target, source))
	if !errors.Is(err, stack.ErrUsage) || !strings.Contains(err.Error(), "_store") {
		t.Fatalf("Init error %v, want ErrUsage naming the missing _store", err)
	}
	diffSnapshots(t, before, snapshot(t, target))

	fresh, _ := dirs(t)
	if err := os.MkdirAll(filepath.Join(fresh, "lost+found"), 0o700); err != nil {
		t.Fatal(err)
	}
	mustInit(t, opts(fresh, source))
}
