// SPDX-License-Identifier: Apache-2.0

//go:build imagetest

// Package postgresimage_test proves the image bothy-postgres with two containers (brief M7-01,
// ADR 0058 "Confirmation"): archive-push over pgBackRest TLS from postgres to the repository
// host backup, a full backup, a point-in-time restore into a fresh volume and pgbackrest verify;
// a client certificate of another CA is rejected; the database container has exactly one
// network, the internal one, and no egress; the image carries its labels and no tag latest.
//
// The tests need a Docker CLI with a reachable daemon and two inputs from the environment, set
// by `make postgres-image-test` after `make postgres-image`:
//
//	BOTHY_TEST_IMAGE      reference of the image loaded for the host platform
//	BOTHY_TEST_IMAGE_OCI  absolute path of the OCI archive with every platform
//
// Secrets come from bothy secrets init (package stack) in a temporary directory. Every network,
// volume and container a test creates carries a random name and is removed in t.Cleanup; no
// container publishes a host port.
package postgresimage_test

import (
	"archive/tar"
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/platform/secrets/stack"
)

const (
	// dockerTimeout bounds one Docker CLI call; a full backup of the small test cluster takes
	// seconds, the slowest call is the start of a container.
	dockerTimeout = 3 * time.Minute
	// waitTimeout bounds every wait for a state (server ready, segment archived).
	waitTimeout = 3 * time.Minute
	// versionFile holds the server version, relative to this package directory.
	versionFile = "../../VERSION"
	// noEgressAddr is in the documentation range 192.0.2.0/24 (RFC 5737), which is never routed.
	noEgressAddr = "192.0.2.1"
	// stanza is the pgBackRest stanza the image configures.
	stanza = "bothy"
)

// env returns the value of the environment variable name and fails the test when it is empty.
func env(t *testing.T, name string) string {
	t.Helper()
	v := os.Getenv(name) //nolint:forbidigo // test input from make postgres-image-test, not server configuration
	if v == "" {
		t.Fatalf("%s is not set; run make postgres-image and then make postgres-image-test", name)
	}
	return v
}

// docker runs the Docker CLI with args under ctx and dockerTimeout and returns stdout, stderr and
// the exit code; only a CLI that cannot start fails the test.
func dockerCtx(ctx context.Context, t *testing.T, args ...string) (stdout, stderr string, code int) {
	t.Helper()
	ctx, cancel := context.WithTimeout(ctx, dockerTimeout)
	defer cancel()
	//nolint:gosec // G204: the arguments are the image under test and fixed flags; running Docker is the test.
	cmd := exec.CommandContext(ctx, "docker", args...)
	var out, errOut bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errOut
	err := cmd.Run()
	var exit *exec.ExitError
	switch {
	case err == nil:
		return out.String(), errOut.String(), 0
	case errors.As(err, &exit):
		return out.String(), errOut.String(), exit.ExitCode()
	default:
		t.Fatalf("docker %s: %v", strings.Join(args, " "), err)
		return "", "", 0
	}
}

// docker is dockerCtx under the test context.
func docker(t *testing.T, args ...string) (stdout, stderr string, code int) {
	t.Helper()
	return dockerCtx(t.Context(), t, args...)
}

// mustDocker is docker that fails the test on a non-zero exit and returns stdout.
func mustDocker(t *testing.T, args ...string) string {
	t.Helper()
	out, errOut, code := docker(t, args...)
	if code != 0 {
		t.Fatalf("docker %s: exit %d: %s%s", strings.Join(args, " "), code, out, errOut)
	}
	return out
}

// randomName returns prefix plus 8 random hex characters.
func randomName(t *testing.T, prefix string) string {
	t.Helper()
	b := make([]byte, 4)
	if _, err := rand.Read(b); err != nil {
		t.Fatal(err)
	}
	return prefix + hex.EncodeToString(b)
}

// cleanup registers "docker <args>" for the end of the test, without the test's cancellation.
func cleanup(t *testing.T, args ...string) {
	t.Helper()
	ctx := context.WithoutCancel(t.Context())
	t.Cleanup(func() {
		if out, errOut, code := dockerCtx(ctx, t, args...); code != 0 && !strings.Contains(errOut, "No such") {
			t.Logf("cleanup docker %s: exit %d: %s%s", strings.Join(args, " "), code, out, errOut)
		}
	})
}

// world is one test installation: an internal network, its volumes and the image.
type world struct {
	t       *testing.T
	image   string
	prefix  string
	network string
}

// newWorld creates the internal network of a test.
func newWorld(t *testing.T) *world {
	t.Helper()
	w := &world{t: t, image: env(t, "BOTHY_TEST_IMAGE"), prefix: randomName(t, "bothy-pgimg-")}
	w.network = w.prefix + "-net"
	mustDocker(t, "network", "create", "--internal", w.network)
	cleanup(t, "network", "rm", w.network)
	return w
}

// volume creates a named volume of the world and returns its name.
func (w *world) volume(name string) string {
	w.t.Helper()
	v := w.prefix + "-" + name
	mustDocker(w.t, "volume", "create", v)
	cleanup(w.t, "volume", "rm", "--force", v)
	return v
}

// secretVolume returns a volume holding the files of dir as the secret volume of a service
// directory holds them after `bothy secrets init --owner <service>=999:999`: owner postgres of the
// image, directories 0500, files 0400 (pgBackRest refuses keys readable by others).
func (w *world) secretVolume(name, dir string) string {
	w.t.Helper()
	v := w.volume(name)
	mustDocker(w.t, "run", "--rm", "--pull=never", "--network", "none", "--user", "0:0", "--entrypoint", "sh",
		"-v", dir+":/in:ro", "-v", v+":/out", w.image, "-c",
		"cp -R /in/. /out/ && chown -R 999:999 /out && find /out -type f -exec chmod 0400 {} + && find /out -type d -exec chmod 0500 {} +")
	return v
}

// start runs a detached container of the image on the internal network under alias and returns
// its name; it is removed with its anonymous volumes at the end.
func (w *world) start(alias string, args ...string) string {
	w.t.Helper()
	name := randomName(w.t, w.prefix+"-"+alias+"-")
	cleanup(w.t, "rm", "--force", "--volumes", name)
	mustDocker(w.t, append([]string{"run", "--detach", "--pull=never", "--name", name,
		"--network", w.network, "--network-alias", alias}, args...)...)
	return name
}

// once runs a container of the image on the internal network to completion and returns its
// combined output and exit code.
func (w *world) once(args ...string) (string, int) {
	w.t.Helper()
	name := randomName(w.t, w.prefix+"-once-")
	cleanup(w.t, "rm", "--force", "--volumes", name)
	out, errOut, code := docker(w.t, append([]string{"run", "--rm", "--pull=never", "--name", name, "--network", w.network}, args...)...)
	return out + errOut, code
}

// eventually polls check every half second until it returns true, failing after waitTimeout with
// what describe says.
func eventually(t *testing.T, what string, check func() bool, describe func() string) {
	t.Helper()
	deadline := time.Now().Add(waitTimeout)
	for !check() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s:\n%s", what, describe())
		}
		time.Sleep(500 * time.Millisecond)
	}
}

// logs returns the combined log of a container.
func logs(t *testing.T, name string) string {
	t.Helper()
	out, errOut, _ := docker(t, "logs", name)
	return out + errOut
}

// psql runs one SQL statement in the database container as postgres and returns the trimmed
// unaligned output.
func psql(t *testing.T, container, sql string) string {
	t.Helper()
	return strings.TrimSpace(mustDocker(t, "exec", container, "psql", "-v", "ON_ERROR_STOP=1", "-U", "postgres", "-d", "bothy", "-Atc", sql))
}

// waitDatabase waits until the PostgreSQL of container accepts queries outside recovery after
// its first start (the base image starts a temporary server during initdb).
func waitDatabase(t *testing.T, container string, fresh bool) {
	t.Helper()
	eventually(t, "the database in "+container, func() bool {
		if fresh && !strings.Contains(logs(t, container), "PostgreSQL init process complete") {
			return false
		}
		out, _, code := docker(t, "exec", container, "psql", "-U", "postgres", "-d", "bothy", "-Atc", "select pg_is_in_recovery()")
		return code == 0 && strings.TrimSpace(out) == "f"
	}, func() string { return logs(t, container) })
}

// archived reports whether the repository holds WAL segment seg of the archive, from the
// repository host.
func archived(t *testing.T, backup, seg string) bool {
	t.Helper()
	out, _, code := docker(t, "exec", backup, "pgbackrest", "--stanza=bothy", "--output=json", "info")
	if code != 0 {
		return false
	}
	var info []struct {
		Archive []struct {
			Max string `json:"max"`
		} `json:"archive"`
	}
	if json.Unmarshal([]byte(out), &info) != nil || len(info) != 1 {
		return false
	}
	for _, a := range info[0].Archive {
		// Segment names of one timeline compare in order as text; a later timeline sorts after.
		if a.Max >= seg {
			return true
		}
	}
	return false
}

// secrets runs bothy secrets init into a temporary directory and returns the volume root.
func secrets(t *testing.T) string {
	t.Helper()
	return secretsWith(t, "")
}

// secretsWith runs bothy secrets init with the supplied files of source ("" for none) into a
// temporary directory and returns the volume root.
func secretsWith(t *testing.T, source string) string {
	t.Helper()
	target := filepath.Join(t.TempDir(), "vol")
	me := stack.Owner{UID: os.Geteuid(), GID: os.Getegid()}
	if _, err := stack.Init(t.Context(), stack.InitOptions{Target: target, Source: source, Owners: map[string]stack.Owner{"server": me, "migrate": me}}); err != nil {
		t.Fatalf("bothy secrets init: %v", err)
	}
	t.Cleanup(func() {
		_ = filepath.WalkDir(target, func(path string, d os.DirEntry, err error) error {
			if err == nil && d.IsDir() {
				_ = os.Chmod(path, 0o700) //nolint:gosec // G302: the directory must be removable.
			}
			return nil
		})
	})
	return target
}

// databaseArgs are the docker run arguments of the database container on the volumes given.
func databaseArgs(secretVol, dataVol string) []string {
	return []string{
		"-e", "POSTGRES_USER=postgres", "-e", "POSTGRES_DB=bothy",
		"-e", "POSTGRES_PASSWORD_FILE=/run/bothy/secrets/db_superuser_password",
		"-e", "BOTHY_PG_ARCHIVE_TIMEOUT=60s", "-e", "BOTHY_PGBACKREST_REPO_HOST=backup",
		"-v", secretVol + ":/run/bothy/secrets:ro", "-v", dataVol + ":/var/lib/postgresql",
	}
}

// TestArchiveBackupRestoreVerify is the main proof of ADR 0058 for M7-01, with the negative cases
// of brief design note 3 on the same installation.
func TestArchiveBackupRestoreVerify(t *testing.T) {
	w := newWorld(t)
	vol := secrets(t)
	secPostgres := w.secretVolume("sec-postgres", filepath.Join(vol, "postgres"))
	secBackup := w.secretVolume("sec-backup", filepath.Join(vol, "backup"))
	pgdata := w.volume("pgdata")

	// bothy-backupd reads the server configuration (M7-02); without migrations its runs would
	// fail, which this test never starts.
	backup := w.start("backup", append(serverEnv("bothy_backup", "db_backup_password"),
		"-e", "BOTHY_PGBACKREST_PG_HOST=postgres",
		"-v", secBackup+":/run/bothy/secrets:ro", "-v", w.volume("repo")+":/var/lib/pgbackrest",
		"-v", w.volume("backup-pg")+":/var/lib/postgresql",
		w.image, "bothy-backupd", "--retry", "1s")...)
	db := w.start("postgres", append(databaseArgs(secPostgres, pgdata), w.image)...)

	waitDatabase(t, db, true)
	eventually(t, "the stanza", func() bool { return strings.Contains(logs(t, backup), "stanza "+stanza+" is ready") },
		func() string { return logs(t, backup) })
	if out, errOut, code := docker(t, "exec", backup, "bothy-backupd", "healthcheck"); code != 0 {
		t.Fatalf("healthcheck of backup: exit %d: %s%s", code, out, errOut)
	}

	t.Run("archive-push over TLS", func(t *testing.T) {
		psql(t, db, "create table t (id int primary key)")
		psql(t, db, "insert into t select generate_series(1, 5)")
		seg := psql(t, db, "select pg_walfile_name(pg_switch_wal())")
		eventually(t, "segment "+seg+" in the repository", func() bool { return archived(t, backup, seg) },
			func() string { return logs(t, db) })
	})

	t.Run("full backup from the repository host", func(t *testing.T) {
		out := mustDocker(t, "exec", backup, "pgbackrest", "--stanza=bothy", "backup", "--type=full", "--start-fast")
		if !strings.Contains(out, "backup command end: completed successfully") {
			t.Errorf("full backup output:\n%s", out)
		}
	})

	// Rows 6 to 10 before the target, 11 to 15 after it; the restore must hold exactly 1 to 10.
	// pgBackRest picks the backup by its stop time in whole seconds, so the target lies at least a
	// second after the end of the backup.
	time.Sleep(1500 * time.Millisecond)
	psql(t, db, "insert into t select generate_series(6, 10)")
	target := psql(t, db, `select to_char(clock_timestamp() at time zone 'UTC', 'YYYY-MM-DD HH24:MI:SS.US') || '+00'`)
	time.Sleep(1500 * time.Millisecond)
	psql(t, db, "insert into t select generate_series(11, 15)")
	seg := psql(t, db, "select pg_walfile_name(pg_switch_wal())")
	eventually(t, "segment "+seg+" in the repository", func() bool { return archived(t, backup, seg) },
		func() string { return logs(t, db) })

	t.Run("database container holds no repository secret", func(t *testing.T) {
		if _, _, code := docker(t, "exec", db, "test", "-e", "/run/bothy/secrets/pgbackrest_cipher_pass"); code == 0 {
			t.Error("postgres sees pgbackrest_cipher_pass")
		}
		if conf := mustDocker(t, "exec", db, "cat", "/etc/pgbackrest/pgbackrest.conf"); strings.Contains(conf, "cipher") {
			t.Errorf("pgbackrest.conf of postgres names a cipher:\n%s", conf)
		}
		env := mustDocker(t, "inspect", "--format", "{{json .Config.Env}}", backup)
		if strings.Contains(strings.ToLower(env), "cipher") {
			t.Errorf("an environment variable of backup names the cipher pass: %s", env)
		}
	})

	t.Run("database container has one internal network and no egress", func(t *testing.T) {
		var networks map[string]json.RawMessage
		if err := json.Unmarshal([]byte(mustDocker(t, "inspect", "--format", "{{json .NetworkSettings.Networks}}", db)), &networks); err != nil {
			t.Fatal(err)
		}
		if names := slices.Sorted(maps.Keys(networks)); !slices.Equal(names, []string{w.network}) {
			t.Errorf("postgres is attached to %v, want only %s", names, w.network)
		}
		if internal := strings.TrimSpace(mustDocker(t, "network", "inspect", "--format", "{{.Internal}}", w.network)); internal != "true" {
			t.Errorf("network %s internal = %s", w.network, internal)
		}
		// Control: the same probe reaches the repository host inside the internal network.
		probe := func(host, port string) []string {
			return []string{"exec", db, "timeout", "10", "bash", "-c", "exec 3<>/dev/tcp/" + host + "/" + port}
		}
		if out, errOut, code := docker(t, probe("backup", "8432")...); code != 0 {
			t.Fatalf("control probe to backup:8432 failed: exit %d %s%s", code, out, errOut)
		}
		if out, errOut, code := docker(t, probe(noEgressAddr, "443")...); code == 0 {
			t.Errorf("postgres opened a TCP connection to %s:443: %s%s", noEgressAddr, out, errOut)
		}
	})

	t.Run("client certificate of another CA is rejected", func(t *testing.T) {
		other := secrets(t)
		// Trust the real CA, present a certificate of another CA: only the server can refuse it.
		foreign := filepath.Join(t.TempDir(), "foreign")
		for _, f := range []struct{ from, name string }{
			{filepath.Join(vol, "postgres", "pgbackrest_tls", "ca.crt"), "ca.crt"},
			{filepath.Join(other, "postgres", "pgbackrest_tls", "postgres.crt"), "postgres.crt"},
			{filepath.Join(other, "postgres", "pgbackrest_tls", "postgres.key"), "postgres.key"},
		} {
			copyFile(t, f.from, filepath.Join(foreign, "pgbackrest_tls", f.name))
		}
		secForeign := w.secretVolume("sec-foreign", foreign)
		scratch := w.volume("scratch")
		// Control: the real client certificate lists the repository.
		if out, code := w.once(append(databaseArgs(secPostgres, scratch), w.image, "pgbackrest", "--stanza=bothy", "repo-ls")...); code != 0 {
			t.Fatalf("control: pgbackrest repo-ls with the stack's certificate failed (%d):\n%s", code, out)
		}
		out, code := w.once(append(databaseArgs(secForeign, scratch), w.image, "pgbackrest", "--stanza=bothy", "repo-ls")...)
		if code == 0 {
			t.Fatalf("pgbackrest repo-ls with a foreign client certificate succeeded:\n%s", out)
		}
		// The repository host's TLS server answers the handshake with an alert for the unknown CA.
		if !regexp.MustCompile(`(?i)alert unknown ca|certificate`).MatchString(out) {
			t.Errorf("the failure does not come from TLS:\n%s", out)
		}
	})

	t.Run("point-in-time restore into a fresh volume", func(t *testing.T) {
		mustDocker(t, "stop", db)
		restored := w.volume("pgdata-restored")
		out, code := w.once(append(databaseArgs(secPostgres, restored), w.image,
			"pgbackrest", "--stanza=bothy", "restore", "--type=time", "--target="+target, "--target-action=promote")...)
		if code != 0 {
			t.Fatalf("restore failed (%d):\n%s", code, out)
		}
		mustDocker(t, "rm", "--force", "--volumes", db)
		db2 := w.start("postgres", append(databaseArgs(secPostgres, restored), w.image)...)
		waitDatabase(t, db2, false)
		if got := psql(t, db2, "select string_agg(id::text, ',' order by id) from t"); got != "1,2,3,4,5,6,7,8,9,10" {
			t.Errorf("restored rows %q, want exactly 1 to 10 (target %s)", got, target)
		}
	})

	t.Run("verify", func(t *testing.T) {
		out := mustDocker(t, "exec", backup, "pgbackrest", "--stanza=bothy", "verify")
		if !strings.Contains(out, "verify command end: completed successfully") {
			t.Errorf("verify output:\n%s", out)
		}
	})
}

// copyFile copies one file, creating the parent directories.
func copyFile(t *testing.T, from, to string) {
	t.Helper()
	raw, err := os.ReadFile(from) //nolint:gosec // G304: paths from t.TempDir
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(to), 0o755); err != nil { //nolint:gosec // G301: the copy container reads it.
		t.Fatal(err)
	}
	if err := os.WriteFile(to, raw, 0o644); err != nil { //nolint:gosec // G306: the copy container reads it.
		t.Fatal(err)
	}
}

// imageConfig is the part of docker image inspect these tests read.
type imageConfig struct {
	RepoTags []string `json:"RepoTags"`
	Config   struct {
		User   string            `json:"User"`
		Labels map[string]string `json:"Labels"`
	} `json:"Config"`
}

// TestImageLabelsTagsAndVersion checks the OCI labels of 18 "Server-Images" rule 6, that the image
// has no tag latest, runs as postgres and ships pgBackRest 2.59.3 or later.
func TestImageLabelsTagsAndVersion(t *testing.T) {
	image := env(t, "BOTHY_TEST_IMAGE")
	var configs []imageConfig
	if err := json.Unmarshal([]byte(mustDocker(t, "image", "inspect", image)), &configs); err != nil || len(configs) != 1 {
		t.Fatalf("docker image inspect: %v (%d images)", err, len(configs))
	}
	c := configs[0]
	raw, err := os.ReadFile(versionFile)
	if err != nil {
		t.Fatal(err)
	}
	want := strings.TrimSpace(string(raw))
	labels := c.Config.Labels
	if labels["org.opencontainers.image.version"] != want {
		t.Errorf("label version = %q, want %q", labels["org.opencontainers.image.version"], want)
	}
	if !regexp.MustCompile(`^[0-9a-f]{40}$`).MatchString(labels["org.opencontainers.image.revision"]) {
		t.Errorf("label revision = %q, want a commit SHA", labels["org.opencontainers.image.revision"])
	}
	if !strings.HasPrefix(labels["org.opencontainers.image.source"], "https://") {
		t.Errorf("label source = %q", labels["org.opencontainers.image.source"])
	}
	if labels["org.opencontainers.image.title"] != "bothy-postgres" || labels["org.opencontainers.image.licenses"] != "Apache-2.0" {
		t.Errorf("labels title %q licenses %q", labels["org.opencontainers.image.title"], labels["org.opencontainers.image.licenses"])
	}
	for _, tag := range c.RepoTags {
		if strings.HasSuffix(tag, ":latest") {
			t.Errorf("image has tag %s (18 \"Kanäle\" rule 3)", tag)
		}
	}
	if c.Config.User != "999:999" {
		t.Errorf("user = %q, want 999:999 (postgres of the base image)", c.Config.User)
	}
	out, errOut, code := docker(t, "run", "--rm", "--pull=never", "--network", "none", "--entrypoint", "check-pgbackrest-version.sh", image)
	if code != 0 {
		t.Errorf("version guard in the image failed: %s%s", out, errOut)
	}
	if out, _, _ := docker(t, "run", "--rm", "--pull=never", "--network", "none", "--entrypoint", "bothy-backupd", image, "version"); strings.TrimSpace(out) != want {
		t.Errorf("bothy-backupd version = %q, want %q", strings.TrimSpace(out), want)
	}
}

// TestMultiArchArchive checks that the OCI archive of make postgres-image holds exactly
// linux/amd64 and linux/arm64 with the labels of the loaded image.
func TestMultiArchArchive(t *testing.T) {
	var configs []imageConfig
	if err := json.Unmarshal([]byte(mustDocker(t, "image", "inspect", env(t, "BOTHY_TEST_IMAGE"))), &configs); err != nil || len(configs) != 1 {
		t.Fatalf("docker image inspect: %v", err)
	}
	loaded := configs[0].Config.Labels
	files := readOCIArchive(t, env(t, "BOTHY_TEST_IMAGE_OCI"))
	var platforms []string
	for _, img := range ociImages(t, files, "index.json", 0) {
		platforms = append(platforms, img.platform)
		labels := ociLabels(t, files, img.digest)
		for _, k := range []string{"org.opencontainers.image.version", "org.opencontainers.image.revision"} {
			if labels[k] != loaded[k] {
				t.Errorf("%s: label %s = %q, loaded image has %q (rerun make postgres-image)", img.platform, k, labels[k], loaded[k])
			}
		}
	}
	slices.Sort(platforms)
	if want := []string{"linux/amd64", "linux/arm64"}; !slices.Equal(platforms, want) {
		t.Errorf("platforms = %v, want %v", platforms, want)
	}
}

// readOCIArchive returns the regular files of the OCI archive at path by name.
func readOCIArchive(t *testing.T, path string) map[string][]byte {
	t.Helper()
	f, err := os.Open(path) //nolint:gosec // path from make postgres-image-test
	if err != nil {
		t.Fatalf("open the OCI archive: %v (run make postgres-image)", err)
	}
	defer func() { _ = f.Close() }()
	files := map[string][]byte{}
	tr := tar.NewReader(f)
	for {
		h, err := tr.Next()
		if errors.Is(err, io.EOF) {
			return files
		}
		if err != nil {
			t.Fatalf("read the OCI archive: %v", err)
		}
		if h.Typeflag != tar.TypeReg {
			continue
		}
		if files[strings.TrimPrefix(h.Name, "./")], err = io.ReadAll(tr); err != nil {
			t.Fatalf("read %s: %v", h.Name, err)
		}
	}
}

// ociImage is one image manifest of the archive.
type ociImage struct{ platform, digest string }

// ociDescriptor is an entry of an OCI index.
type ociDescriptor struct {
	MediaType string `json:"mediaType"`
	Digest    string `json:"digest"`
	Platform  *struct {
		OS           string `json:"os"`
		Architecture string `json:"architecture"`
	} `json:"platform"`
}

// ociImages follows the index in files[name] down to the image manifests; attestation
// manifests (platform unknown/unknown) are left out.
func ociImages(t *testing.T, files map[string][]byte, name string, depth int) []ociImage {
	t.Helper()
	if depth > 3 {
		t.Fatalf("OCI archive: indexes nested deeper than 3 levels at %s", name)
	}
	var idx struct {
		Manifests []ociDescriptor `json:"manifests"`
	}
	readJSON(t, files, name, &idx)
	var images []ociImage
	for _, d := range idx.Manifests {
		switch {
		case strings.HasSuffix(d.MediaType, "image.index.v1+json"), strings.HasSuffix(d.MediaType, "manifest.list.v2+json"):
			images = append(images, ociImages(t, files, blobPath(d.Digest), depth+1)...)
		case d.Platform == nil:
			t.Errorf("OCI archive: manifest %s has no platform", d.Digest)
		case d.Platform.OS+"/"+d.Platform.Architecture != "unknown/unknown":
			images = append(images, ociImage{platform: d.Platform.OS + "/" + d.Platform.Architecture, digest: d.Digest})
		}
	}
	return images
}

// ociLabels returns the labels of the image configuration that the manifest with digest names.
func ociLabels(t *testing.T, files map[string][]byte, digest string) map[string]string {
	t.Helper()
	var manifest struct {
		Config struct {
			Digest string `json:"digest"`
		} `json:"config"`
	}
	readJSON(t, files, blobPath(digest), &manifest)
	var config struct {
		Config struct {
			Labels map[string]string `json:"Labels"`
		} `json:"config"`
	}
	readJSON(t, files, blobPath(manifest.Config.Digest), &config)
	return config.Config.Labels
}

// blobPath returns the archive path of the blob with digest ("sha256:<hex>").
func blobPath(digest string) string { return "blobs/" + strings.Replace(digest, ":", "/", 1) }

// readJSON decodes files[name] into v and fails the test when it is missing or invalid.
func readJSON(t *testing.T, files map[string][]byte, name string, v any) {
	t.Helper()
	raw, ok := files[name]
	if !ok {
		t.Fatalf("OCI archive: %s is missing", name)
	}
	if err := json.Unmarshal(raw, v); err != nil {
		t.Fatalf("OCI archive: %s: %v", name, err)
	}
}
