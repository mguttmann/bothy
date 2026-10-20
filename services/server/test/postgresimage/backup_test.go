// SPDX-License-Identifier: Apache-2.0

//go:build imagetest

package postgresimage_test

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"errors"
	"io"
	"math/big"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go"
)

// The test services of the backup test (brief M7-02 "Image test"). SeaweedFS is the image the
// files driver tests pin (internal/files/adapters/s3/seaweed_test.go, docs/DECISIONS.md M3-02/13);
// the collector is the version infra/compose/compose.yaml runs, pinned by the digest of its
// multi-architecture index.
const (
	seaweedImage   = "chrislusf/seaweedfs:4.48@sha256:4e61d15fd35994cb1e43e1e553dff106794841fd9a99ade2fc8c8bfce4d7872d"
	collectorImage = "otel/opentelemetry-collector-contrib:0.162.0@sha256:39923a8e431bd1f57be82411999d389fcfe40857492e4365456d97a4c1f74be6"
)

// The off-site bucket and the prefix of the test instance, and the throwaway S3 identities of
// SeaweedFS: backup may read, write and list (the key of secrets/offsite/), admin is the
// owner's separate account that sets the Object Lock configuration.
const (
	offsiteBucket = "offsite"
	offsitePrefix = "instance-1"
	backupKeyID   = "backupEXAMPLE"
	backupSecret  = "backup-secret-EXAMPLE"
	adminKeyID    = "adminEXAMPLE"
	adminSecret   = "admin-secret-EXAMPLE"
	publicURL     = "https://chat.example.com"
)

// tlsMount is where the SeaweedFS container sees the test's TLS files and identities.
const tlsMount = "/cfg"

// tlsFlag returns the SeaweedFS flag -s3.<name>.file for the PEM file <name>.pem of tlsMount.
func tlsFlag(name string) string { return "-s3." + name + ".file=" + tlsMount + "/" + name + ".pem" }

// collectorConfig receives OTLP over gRPC and writes every metric to the log of the collector
// (exporter debug), where the test reads the instrument names.
const collectorConfig = `receivers:
  otlp:
    protocols:
      grpc:
        endpoint: 0.0.0.0:4317
exporters:
  debug:
    verbosity: detailed
service:
  pipelines:
    metrics:
      receivers: [otlp]
      exporters: [debug]
`

// TestBackupKinds runs every backup kind of bothy-backupd against the database container and
// local targets (brief M7-02 "Image test"): repo1 in a volume, the off-site target a SeaweedFS
// bucket with versioning and Object Lock. Each kind leaves one succeeded backup_runs row, its
// manifest is present next to the backup, the key of backup cannot delete a stored version,
// expire runs only on repo1, and the metrics reach an OTLP receiver with the names of
// signals.md.
func TestBackupKinds(t *testing.T) {
	w := newWorld(t)
	bin := buildServerBinary(t)
	ca, serverCert, serverKey := seaweedTLS(t)

	source := t.TempDir()
	writeFile(t, filepath.Join(source, "offsite", "access_key_id"), backupKeyID)
	writeFile(t, filepath.Join(source, "offsite", "secret_access_key"), backupSecret)
	writeFile(t, filepath.Join(source, "offsite", "ca.crt"), string(ca))
	vol := secretsWith(t, source)
	secPostgres := w.secretVolume("sec-postgres", filepath.Join(vol, "postgres"))
	secBackup := w.secretVolume("sec-backup", filepath.Join(vol, "backup"))
	secMigrate := w.secretVolume("sec-migrate", filepath.Join(vol, "migrate"))
	pgdata := w.volume("pgdata")
	files := w.volume("files")

	// SeaweedFS: on the internal network as seaweed, and on a second network whose port is
	// published on 127.0.0.1 so the test can act as the owner and try deletions.
	tlsDir := t.TempDir()
	writeFile(t, filepath.Join(tlsDir, "cert.pem"), string(serverCert))
	writeFile(t, filepath.Join(tlsDir, "key.pem"), string(serverKey))
	writeFile(t, filepath.Join(tlsDir, "s3.json"), s3Identities())
	pub := w.prefix + "-pub"
	mustDocker(t, "network", "create", pub)
	cleanup(t, "network", "rm", pub)
	seaweed := randomName(t, w.prefix+"-seaweed-")
	cleanup(t, "rm", "--force", "--volumes", seaweed)
	mustDocker(t, "run", "--detach", "--name", seaweed, "--network", pub, "-p", "127.0.0.1::8333",
		"-v", tlsDir+":/cfg:ro", seaweedImage, "server", "-s3", "-s3.config=/cfg/s3.json",
		tlsFlag("cert"), tlsFlag("key"), "-volume.max=100", "-master.volumeSizeLimitMB=32",
		// Listen on every network of the container, not only on the first one.
		"-ip.bind=0.0.0.0")
	mustDocker(t, "network", "connect", "--alias", "seaweed", w.network, seaweed)
	admin := s3Client(t, seaweedEndpoint(t, seaweed), ca, adminKeyID, adminSecret)
	eventually(t, "the S3 gateway", func() bool {
		_, err := admin.ListBuckets(t.Context(), &awss3.ListBucketsInput{})
		return err == nil
	}, func() string { return logs(t, seaweed) })
	createLockedBucket(t, seaweed, admin)

	collector := w.start("collector", "--pull=missing", "--entrypoint", "/otelcol-contrib",
		"-v", writeTemp(t, "collector.yaml", collectorConfig)+":/etc/otelcol/test.yaml:ro",
		collectorImage, "--config=/etc/otelcol/test.yaml")

	db := w.start("postgres", append(databaseArgs(secPostgres, pgdata), "-e", "BOTHY_PGBACKREST_REPO2=on", w.image)...)
	waitDatabase(t, db, true)

	// bothy migrate up --bootstrap creates the five roles, the schema with backup_runs and the
	// instance row, as the service migrate of the Compose stack does.
	migrateArgs := append(serverEnv("bothy_owner", "db_owner_password"),
		"-v", secMigrate+":/run/bothy/secrets:ro", "-v", bin+":/opt/bothy:ro", "--entrypoint", "/opt/bothy/bothy",
		w.image, "migrate", "up", "--bootstrap")
	if out, code := w.once(migrateArgs...); code != 0 {
		t.Fatalf("bothy migrate up --bootstrap: exit %d:\n%s", code, out)
	}

	// The volume files with one object as the file system driver of the server leaves it: owner
	// 65532 (the user of bothy-server), directories 0700, files 0600, so only restic's file
	// capability lets the user 999 of backup read it.
	if out, code := w.once("--user", "0:0", "--entrypoint", "sh", "-v", files+":/files", w.image, "-c",
		"mkdir -p /files/f/ab/cd && echo object-EXAMPLE > /files/f/ab/cd/orig-1 && chown -R 65532:65532 /files && "+
			"find /files -type d -exec chmod 0700 {} + && find /files -type f -exec chmod 0600 {} +"); code != 0 {
		t.Fatalf("seed the volume files: exit %d:\n%s", code, out)
	}

	target := "s3:https://seaweed:8333/" + offsiteBucket + "/" + offsitePrefix
	backupArgs := append(serverEnv("bothy_backup", "db_backup_password"),
		"-e", "BOTHY_PGBACKREST_PG_HOST=postgres",
		"-e", "BOTHY_BACKUP__OFFSITE__TARGET="+target,
		"-e", "BOTHY_FILE__STORAGE__DRIVER=fs", "-e", "BOTHY_FILE__FS__ROOT=/var/lib/bothy/files",
		"-e", "OTEL_EXPORTER_OTLP_ENDPOINT=http://collector:4317", "-e", "OTEL_EXPORTER_OTLP_PROTOCOL=grpc",
		"-e", "OTEL_METRIC_EXPORT_INTERVAL=1000",
		"-v", secBackup+":/run/bothy/secrets:ro", "-v", w.volume("repo")+":/var/lib/pgbackrest",
		"-v", w.volume("scratch")+":/var/lib/bothy-backup", "-v", files+":/var/lib/bothy/files:ro",
		"-v", pgdata+":/var/lib/postgresql:ro", "--cap-add", "DAC_READ_SEARCH",
		w.image, "bothy-backupd", "--retry", "1s")
	backup := w.start("backup", backupArgs...)
	eventually(t, "the stanza", func() bool {
		if state := strings.TrimSpace(mustDocker(t, "inspect", "--format", "{{.State.Status}}", backup)); state != "running" {
			t.Fatalf("backup is %s:\n%s", state, logs(t, backup))
		}
		return strings.Contains(logs(t, backup), "stanza "+stanza+" is ready")
	}, func() string { return logs(t, backup) })

	kinds := []string{"pg_full", "pg_diff", "logical", "objects", "pg_full"}
	for _, kind := range kinds {
		out, errOut, code := docker(t, "exec", backup, "bothy-backupd", "backup", kind)
		if code != 0 {
			t.Fatalf("bothy-backupd backup %s: exit %d:\n%s%s\nlog of backup:\n%s", kind, code, out, errOut, logs(t, backup))
		}
	}

	t.Run("one succeeded backup_runs row per run", func(t *testing.T) {
		got := psql(t, db, `select string_agg(kind || ':' || state || ':' || coalesce(error, '-') || ':' ||
			(manifest->>'schema') || ':' || (size_bytes is not null)::text, ',' order by started_at) from backup_runs`)
		var want []string
		for _, k := range kinds {
			want = append(want, k+":succeeded:-:backup_manifest.v1:true")
		}
		if got != strings.Join(want, ",") {
			t.Errorf("backup_runs = %s\nwant      %s", got, strings.Join(want, ","))
		}
		if got := psql(t, db, `select count(*) from backup_runs where kind like 'pg_%' and manifest->'pgbackrest'->>'label' is null`); got != "0" {
			t.Errorf("%s physical runs without pgbackrest.label", got)
		}
		if got := psql(t, db, `select count(*) from backup_runs where kind in ('logical', 'objects') and manifest->'restic'->>'snapshot_id' is null`); got != "0" {
			t.Errorf("%s restic runs without restic.snapshot_id", got)
		}
		rowCounts := psql(t, db, `select manifest->'row_counts'->>'backup_runs' from backup_runs order by started_at limit 1`)
		if rowCounts != "1" {
			t.Errorf("row_counts.backup_runs of the first manifest = %q, want 1 (the running row itself)", rowCounts)
		}
	})

	t.Run("manifest next to the backup", func(t *testing.T) {
		ids := strings.Fields(psql(t, db, `select string_agg(id::text, ' ' order by started_at) from backup_runs`))
		out := mustDocker(t, "exec", "-e", "AWS_ACCESS_KEY_ID="+backupKeyID, "-e", "AWS_SECRET_ACCESS_KEY="+backupSecret,
			"-e", "RESTIC_PASSWORD_FILE=/run/bothy/secrets/restic_password", backup,
			"restic", "--no-cache", "--cacert", "/run/bothy/secrets/offsite/ca.crt", "-o", "s3.bucket-lookup=path",
			"-r", "s3:https://seaweed:8333/"+offsiteBucket+"/"+offsitePrefix+"/restic", "snapshots", "--json", "--tag", "manifest")
		var snaps []struct {
			Tags []string `json:"tags"`
		}
		if err := json.Unmarshal([]byte(out), &snaps); err != nil {
			t.Fatalf("restic snapshots: %v\n%s", err, out)
		}
		tagged := map[string]bool{}
		for _, s := range snaps {
			for _, tag := range s.Tags {
				tagged[tag] = true
			}
		}
		for _, id := range ids {
			if !tagged[id] {
				t.Errorf("no manifest snapshot tagged with run %s (snapshots %s)", id, out)
			}
		}
	})

	t.Run("expire runs only on repo1", func(t *testing.T) {
		out := mustDocker(t, "exec", backup, "pgbackrest", "--stanza=bothy", "--repo=1", "--output=json", "info")
		if n := strings.Count(out, `"type":"full"`); n != 1 {
			t.Errorf("repo1 holds %d full backups, want 1 (backup.pg.local_retention_full):\n%s", n, out)
		}
		labels := map[string]bool{}
		for _, key := range listKeys(t, admin, offsitePrefix+"/pgbackrest/backup/bothy/") {
			if m := regexp.MustCompile(`/backup/bothy/([0-9]{8}-[0-9]{6}F)/`).FindStringSubmatch(key); m != nil {
				labels[m[1]] = true
			}
		}
		if len(labels) != 2 {
			t.Errorf("repo2 holds full backups %v, want both of the two pg_full runs", labels)
		}
	})

	t.Run("the key of backup cannot delete or overwrite", func(t *testing.T) {
		own := s3Client(t, seaweedEndpoint(t, seaweed), ca, backupKeyID, backupSecret)
		checked := 0
		for _, part := range []string{"/pgbackrest/archive/", "/pgbackrest/backup/", "/restic/data/", "/restic/snapshots/"} {
			versions, err := own.ListObjectVersions(t.Context(), &awss3.ListObjectVersionsInput{
				Bucket: aws.String(offsiteBucket), Prefix: aws.String(offsitePrefix + part)})
			if err != nil {
				t.Fatalf("list %s: %v", part, err)
			}
			if len(versions.Versions) == 0 {
				t.Errorf("no object below %s", part)
				continue
			}
			v := versions.Versions[0]
			_, err = own.DeleteObject(t.Context(), &awss3.DeleteObjectInput{Bucket: aws.String(offsiteBucket), Key: v.Key, VersionId: v.VersionId})
			var apiErr smithy.APIError
			if !errors.As(err, &apiErr) || apiErr.ErrorCode() != "AccessDenied" {
				t.Errorf("delete of a version below %s with the key of backup: %v, want AccessDenied", part, err)
			}
			checked++
		}
		// Overwrite: a new version may appear, the stored one stays as it was.
		versions, err := own.ListObjectVersions(t.Context(), &awss3.ListObjectVersionsInput{
			Bucket: aws.String(offsiteBucket), Prefix: aws.String(offsitePrefix + "/restic/snapshots/")})
		if err != nil || len(versions.Versions) == 0 {
			t.Fatalf("list snapshots: %v", err)
		}
		v := versions.Versions[0]
		before := getVersion(t, own, *v.Key, *v.VersionId)
		if _, err := own.PutObject(t.Context(), &awss3.PutObjectInput{Bucket: aws.String(offsiteBucket), Key: v.Key,
			Body: strings.NewReader("overwritten-EXAMPLE"), ChecksumAlgorithm: s3types.ChecksumAlgorithmSha256}); err != nil {
			t.Logf("overwrite refused: %v", err)
		}
		if after := getVersion(t, own, *v.Key, *v.VersionId); !bytes.Equal(before, after) {
			t.Errorf("the stored version of %s changed after an overwrite", *v.Key)
		}
		if checked != 4 {
			t.Errorf("checked %d prefixes, want 4", checked)
		}
	})

	t.Run("metrics reach the OTLP receiver", func(t *testing.T) {
		names := []string{
			"bothy.backup.last_success.timestamp", "bothy.backup.wal.last_archived.timestamp",
			"bothy.backup.duration", "bothy.backup.size", "bothy.storage.volume.free", "bothy.storage.volume.size",
		}
		eventually(t, "the backup metrics in the collector", func() bool {
			l := logs(t, collector)
			for _, n := range names {
				if !strings.Contains(l, "Name: "+n+"\n") && !strings.Contains(l, "-> Name: "+n) {
					return false
				}
			}
			return strings.Contains(l, "service.name: Str(bothy-backupd)")
		}, func() string { return logs(t, collector) })
		l := logs(t, collector)
		for _, bad := range []string{offsiteBucket + "/", "/var/lib/", "seaweed"} {
			if strings.Contains(l, "Str("+bad) {
				t.Errorf("a metric attribute names %q", bad)
			}
		}
	})

	t.Run("bothy migrate up proceeds with a recent backup", func(t *testing.T) {
		migrate := append(serverEnv("bothy_owner", "db_owner_password"),
			"-v", secMigrate+":/run/bothy/secrets:ro", "-v", bin+":/opt/bothy:ro", "--entrypoint", "/opt/bothy/bothy",
			w.image, "migrate", "up")
		if out, code := w.once(migrate...); code != 0 {
			t.Fatalf("bothy migrate up: exit %d:\n%s", code, out)
		}
	})
}

// serverEnv returns the docker run arguments of the server configuration of a stack process
// that connects to postgres as user with the password file of the secret volume.
func serverEnv(user, passwordFile string) []string {
	return []string{
		"-e", "BOTHY_SERVER__PUBLIC_URL=" + publicURL,
		"-e", "BOTHY_SECRETS__DIR=/run/bothy/secrets",
		"-e", "BOTHY_DATABASE__HOST=postgres", "-e", "BOTHY_DATABASE__NAME=bothy",
		"-e", "BOTHY_DATABASE__USER=" + user,
		"-e", "BOTHY_DATABASE__PASSWORD_REF=file:/run/bothy/secrets/" + passwordFile,
		"-e", "BOTHY_DATABASE__SSLMODE=disable",
	}
}

// buildServerBinary cross compiles the server binary for the platform of the Docker daemon into a
// temporary directory and returns the directory; the test runs bothy migrate from it inside a
// container of the image.
func buildServerBinary(t *testing.T) string {
	t.Helper()
	arch := strings.TrimSpace(mustDocker(t, "info", "--format", "{{.Architecture}}"))
	goarch := map[string]string{"x86_64": "amd64", "amd64": "amd64", "aarch64": "arm64", "arm64": "arm64"}[arch]
	if goarch == "" {
		t.Fatalf("no Go architecture for the Docker daemon architecture %q", arch)
	}
	dir := t.TempDir()
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Minute)
	defer cancel()
	//nolint:gosec // G204: the Go toolchain of this test run builds this repository's command.
	cmd := exec.CommandContext(ctx, "go", "build", "-trimpath", "-o", filepath.Join(dir, "bothy"), "../../cmd/bothy")
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0", "GOOS=linux", "GOARCH="+goarch) //nolint:forbidigo // the toolchain's environment
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("go build ./cmd/bothy: %v\n%s", err, out)
	}
	if err := os.Chmod(dir, 0o755); err != nil { //nolint:gosec // G302: the container user 999 reads the binary.
		t.Fatal(err)
	}
	return dir
}

// writeFile writes content to path with mode 0644, creating the parent directories.
func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil { //nolint:gosec // G301: test input read by containers.
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil { //nolint:gosec // G306: test input read by containers.
		t.Fatal(err)
	}
}

// writeTemp writes content into a new file name below a temporary directory and returns its
// path.
func writeTemp(t *testing.T, name, content string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), name)
	writeFile(t, p, content)
	return p
}

// s3Identities is the identity file of SeaweedFS: backup with Read, Write and List, admin with
// every action.
func s3Identities() string {
	type cred struct {
		AccessKey string `json:"accessKey"`
		SecretKey string `json:"secretKey"`
	}
	type identity struct {
		Name        string   `json:"name"`
		Credentials []cred   `json:"credentials"`
		Actions     []string `json:"actions"`
	}
	b, _ := json.Marshal(map[string][]identity{"identities": {
		{Name: "backup", Credentials: []cred{{backupKeyID, backupSecret}}, Actions: []string{"Read", "Write", "List"}},
		{Name: "admin", Credentials: []cred{{adminKeyID, adminSecret}}, Actions: []string{"Admin", "Read", "Write", "List", "Tagging"}},
	}})
	return string(b)
}

// seaweedTLS returns a throwaway CA and a server certificate and key for the names seaweed and
// 127.0.0.1, all PEM.
func seaweedTLS(t *testing.T) (caPEM, certPEM, keyPEM []byte) {
	t.Helper()
	caKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	caTmpl := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "test CA EXAMPLE"},
		NotBefore: now.Add(-time.Hour), NotAfter: now.Add(24 * time.Hour), IsCA: true, BasicConstraintsValid: true,
		KeyUsage: x509.KeyUsageCertSign}
	caDER, err := x509.CreateCertificate(rand.Reader, caTmpl, caTmpl, &caKey.PublicKey, caKey)
	if err != nil {
		t.Fatal(err)
	}
	caCert, err := x509.ParseCertificate(caDER)
	if err != nil {
		t.Fatal(err)
	}
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{SerialNumber: big.NewInt(2), Subject: pkix.Name{CommonName: "seaweed"},
		NotBefore: now.Add(-time.Hour), NotAfter: now.Add(24 * time.Hour), DNSNames: []string{"seaweed", "localhost"},
		IPAddresses: []net.IP{net.ParseIP("127.0.0.1")}, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		KeyUsage: x509.KeyUsageDigitalSignature}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, caCert, &key.PublicKey, caKey)
	if err != nil {
		t.Fatal(err)
	}
	keyDER, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caDER}),
		pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}),
		pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER})
}

// seaweedEndpoint returns https://127.0.0.1:<published port> of the S3 gateway.
func seaweedEndpoint(t *testing.T, container string) string {
	t.Helper()
	out := strings.TrimSpace(mustDocker(t, "port", container, "8333/tcp"))
	for _, line := range strings.Split(out, "\n") {
		if strings.HasPrefix(line, "127.0.0.1:") {
			return "https://" + line
		}
	}
	t.Fatalf("no 127.0.0.1 port of %s: %s", container, out)
	return ""
}

// staticCredentials is an aws.CredentialsProvider of one key.
type staticCredentials struct{ id, secret string }

// Retrieve implements aws.CredentialsProvider.
func (c staticCredentials) Retrieve(context.Context) (aws.Credentials, error) {
	return aws.Credentials{AccessKeyID: c.id, SecretAccessKey: c.secret, Source: "test"}, nil
}

// s3Client returns a path-style client of endpoint that trusts ca.
func s3Client(t *testing.T, endpoint string, ca []byte, id, secret string) *awss3.Client {
	t.Helper()
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(ca) {
		t.Fatal("test CA does not parse")
	}
	return awss3.New(awss3.Options{
		Region: "us-east-1", BaseEndpoint: aws.String(endpoint), UsePathStyle: true,
		Credentials: staticCredentials{id, secret},
		HTTPClient:  &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{RootCAs: pool, MinVersion: tls.VersionTLS12}}},
	})
}

// createLockedBucket creates the off-site bucket with versioning and Object Lock and a default
// retention in mode COMPLIANCE of one day, the setup of docs/operations/backup-restore.md 2.
func createLockedBucket(t *testing.T, seaweed string, admin *awss3.Client) {
	t.Helper()
	out, errOut, code := docker(t, "exec", "-i", seaweed, "sh", "-c", "echo 's3.bucket.create -name "+offsiteBucket+" -withLock' | weed shell")
	if code != 0 || !strings.Contains(out+errOut, "Object Lock: enabled") {
		t.Fatalf("create the locked bucket: exit %d:\n%s%s", code, out, errOut)
	}
	if _, err := admin.PutObjectLockConfiguration(t.Context(), &awss3.PutObjectLockConfigurationInput{
		Bucket: aws.String(offsiteBucket),
		ObjectLockConfiguration: &s3types.ObjectLockConfiguration{
			ObjectLockEnabled: s3types.ObjectLockEnabledEnabled,
			Rule: &s3types.ObjectLockRule{DefaultRetention: &s3types.DefaultRetention{
				Mode: s3types.ObjectLockRetentionModeCompliance, Days: aws.Int32(1)}},
		},
	}); err != nil {
		t.Fatalf("default retention of the bucket: %v", err)
	}
}

// listKeys lists every key below prefix of the off-site bucket.
func listKeys(t *testing.T, c *awss3.Client, prefix string) []string {
	t.Helper()
	var keys []string
	p := awss3.NewListObjectsV2Paginator(c, &awss3.ListObjectsV2Input{Bucket: aws.String(offsiteBucket), Prefix: aws.String(prefix)})
	for p.HasMorePages() {
		page, err := p.NextPage(t.Context())
		if err != nil {
			t.Fatalf("list %s: %v", prefix, err)
		}
		for _, o := range page.Contents {
			keys = append(keys, *o.Key)
		}
	}
	return keys
}

// getVersion returns the bytes of one version of key.
func getVersion(t *testing.T, c *awss3.Client, key, version string) []byte {
	t.Helper()
	out, err := c.GetObject(t.Context(), &awss3.GetObjectInput{Bucket: aws.String(offsiteBucket), Key: aws.String(key), VersionId: aws.String(version)})
	if err != nil {
		t.Fatalf("get %s version %s: %v", key, version, err)
	}
	defer func() { _ = out.Body.Close() }()
	b, err := io.ReadAll(out.Body)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
