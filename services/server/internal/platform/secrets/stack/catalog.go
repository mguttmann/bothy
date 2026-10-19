// SPDX-License-Identifier: Apache-2.0

package stack

import (
	"regexp"
	"slices"
	"strings"
)

// Origin says who creates a secret of the catalog.
type Origin string

const (
	// OriginGenerated entries are created by secrets init ("erzeugt" in configuration.md).
	OriginGenerated Origin = "generated"
	// OriginSupplied entries are placed in the source directory by the owner ("geliefert").
	OriginSupplied Origin = "supplied"
)

// Generator names how secrets init creates the value of a generated entry.
type Generator string

const (
	// GeneratorNone is the generator of supplied entries: secrets init never invents them.
	GeneratorNone Generator = ""
	// GeneratorToken is 32 random bytes as base64url without padding (43 ASCII characters), for
	// passwords that pass through configuration files and connection strings unquoted.
	GeneratorToken Generator = "token"
	// GeneratorKey is 32 random bytes as lowercase hex (64 ASCII characters); consumers that need
	// raw key bytes hex-decode it, and the collector's HMAC key gets at least 32 bytes either way.
	GeneratorKey Generator = "key"
	// GeneratorPGBackRestTLS creates the directory entry pgbackrest_tls/: an own CA and one ECDSA
	// P-256 key pair per pgBackRest host, valid for PGBackRestTLSValidity (pgbackrest_tls.go).
	GeneratorPGBackRestTLS Generator = "pgbackrest-tls"
	// GeneratorPending marks an entry whose generator a later ticket builds (Entry.PendingOn);
	// secrets init reports it as pending until then instead of inventing a format.
	GeneratorPending Generator = "pending"
)

// Entry is one row of the table "Geheimnisse" in docs/operations/configuration.md. A name ending
// in "/" is a directory entry (every regular file below it, recursively); an entry with a Pattern
// stands for every file whose relative path matches it.
type Entry struct {
	// Name is the relative path in the source, _store and the service directories; for pattern
	// entries it is the documented placeholder ("<provider>" for "<anbieter>").
	Name string
	// Origin says whether secrets init creates the entry or the owner supplies it.
	Origin Origin
	// Generator creates the value of a generated entry.
	Generator Generator
	// PendingOn names the ticket that builds the generator when Generator is GeneratorPending.
	PendingOn string
	// Services are the service directories (Compose subpath) that get a copy, in table order.
	Services []string
	// Pattern is the anchored regular expression of relative paths for pattern entries, "" for
	// the others.
	Pattern string
}

// isDir reports whether the entry covers a directory tree.
func (e Entry) isDir() bool { return strings.HasSuffix(e.Name, "/") }

// The services of the Compose stack that mount secrets: each name is a service directory below the
// target and the Compose subpath of the service's secrets mount.
const (
	serviceAlertmanager = "alertmanager"
	serviceBackup       = "backup"
	serviceCollector    = "collector"
	serviceGrafana      = "grafana"
	serviceMigrate      = "migrate"
	servicePostgres     = "postgres"
	serviceSeaweedfs    = "seaweedfs"
	serviceServer       = "server"
)

// catalog is in the order of configuration.md section 5, which is also the order of the report.
var catalog = []Entry{
	{Name: "db_superuser_password", Origin: OriginGenerated, Generator: GeneratorToken, Services: []string{servicePostgres, serviceMigrate, serviceBackup}},
	{Name: "db_owner_password", Origin: OriginGenerated, Generator: GeneratorToken, Services: []string{serviceMigrate}},
	{Name: "db_app_password", Origin: OriginGenerated, Generator: GeneratorToken, Services: []string{serviceServer, serviceMigrate}},
	{Name: "db_maint_password", Origin: OriginGenerated, Generator: GeneratorToken, Services: []string{serviceServer, serviceMigrate}},
	{Name: "db_monitor_password", Origin: OriginGenerated, Generator: GeneratorToken, Services: []string{serviceCollector, serviceMigrate}},
	// The password of bothy_backup, the role bothy-backupd records backup_runs with (M7-02).
	{Name: "db_backup_password", Origin: OriginGenerated, Generator: GeneratorToken, Services: []string{serviceMigrate, serviceBackup}},
	{Name: "token_pepper", Origin: OriginGenerated, Generator: GeneratorKey, Services: []string{serviceServer}},
	// The AES-256-GCM key of the TOTP secrets in local_credentials (M1-39): 32 bytes as hex.
	{Name: "totp_key", Origin: OriginGenerated, Generator: GeneratorKey, Services: []string{serviceServer}},
	{Name: "telemetry_pseudonym_key", Origin: OriginGenerated, Generator: GeneratorKey, Services: []string{serviceServer, serviceCollector}},
	{Name: "audit_ip_kek", Origin: OriginGenerated, Generator: GeneratorKey, Services: []string{serviceServer}},
	{Name: "file_ticket_key", Origin: OriginGenerated, Generator: GeneratorKey, Services: []string{serviceServer}},
	// The master key of the derived v1 secrets of the outgoing webhooks (M2-26): 32 bytes as hex.
	{Name: "webhook_hmac_master_key", Origin: OriginGenerated, Generator: GeneratorKey, Services: []string{serviceServer}},
	{Name: "pgbackrest_cipher_pass", Origin: OriginGenerated, Generator: GeneratorToken, Services: []string{serviceBackup}},
	// The own CA for mutual TLS between database and repository host (brief M7-01).
	{Name: "pgbackrest_tls/", Origin: OriginGenerated, Generator: GeneratorPGBackRestTLS, Services: []string{servicePostgres, serviceBackup}},
	{Name: "restic_password", Origin: OriginGenerated, Generator: GeneratorToken, Services: []string{serviceBackup}},
	// Generated only with the s3 profile, whose generator M3 builds; until then supplied.
	{Name: "s3_access_key", Origin: OriginSupplied, Generator: GeneratorPending, PendingOn: "M3", Services: []string{serviceServer, serviceSeaweedfs, serviceBackup}},
	{Name: "s3_secret_key", Origin: OriginSupplied, Generator: GeneratorPending, PendingOn: "M3", Services: []string{serviceServer, serviceSeaweedfs, serviceBackup}},
	// The own key of the object store of the audit checkpoints, from a separate account (M7-22);
	// only the server reads it.
	{Name: "audit_checkpoint_s3_access_key", Origin: OriginSupplied, Services: []string{serviceServer}},
	{Name: "audit_checkpoint_s3_secret_key", Origin: OriginSupplied, Services: []string{serviceServer}},
	{Name: "grafana_admin_password", Origin: OriginGenerated, Generator: GeneratorToken, Services: []string{serviceGrafana}},
	{Name: "apns_key.p8", Origin: OriginSupplied, Services: []string{serviceServer}},
	{Name: "fcm_service_account.json", Origin: OriginSupplied, Services: []string{serviceServer}},
	{Name: "oidc/<provider>_client_secret", Origin: OriginSupplied, Services: []string{serviceServer}, Pattern: `^oidc/[a-z0-9_-]+_client_secret$`},
	{Name: "oidc/<provider>_client_key.pem", Origin: OriginSupplied, Services: []string{serviceServer}, Pattern: `^oidc/[a-z0-9_-]+_client_key\.pem$`},
	{Name: "offsite/", Origin: OriginSupplied, Services: []string{serviceBackup}},
	{Name: "alertmanager_bothy_webhook_url", Origin: OriginSupplied, Services: []string{serviceAlertmanager}},
	{Name: "alertmanager_external_webhook_url", Origin: OriginSupplied, Services: []string{serviceAlertmanager}},
	{Name: "alertmanager_heartbeat_url", Origin: OriginSupplied, Services: []string{serviceAlertmanager}},
	{Name: "newrelic_license_key", Origin: OriginSupplied, Services: []string{serviceCollector}},
	{Name: "grafana_otlp_authorization", Origin: OriginSupplied, Services: []string{serviceCollector}},
	{Name: "datadog_api_key", Origin: OriginSupplied, Services: []string{serviceCollector}},
	{Name: "azure_client_secret", Origin: OriginSupplied, Services: []string{serviceCollector}},
	{Name: "sentry_key", Origin: OriginSupplied, Services: []string{serviceCollector}},
}

// patterns holds the compiled Pattern of every pattern entry, by entry name.
var patterns = func() map[string]*regexp.Regexp {
	m := map[string]*regexp.Regexp{}
	for _, e := range catalog {
		if e.Pattern != "" {
			m[e.Name] = regexp.MustCompile(e.Pattern)
		}
	}
	return m
}()

// Catalog returns the secrets of the Compose stack in the order of configuration.md section 5.
// The result is a copy; callers may change it.
func Catalog() []Entry {
	out := make([]Entry, len(catalog))
	for i, e := range catalog {
		e.Services = slices.Clone(e.Services)
		out[i] = e
	}
	return out
}

// Services returns every service named in the catalog, sorted and unique: the directories
// secrets init creates, one per Compose subpath mount.
func Services() []string {
	var out []string
	for _, e := range catalog {
		out = append(out, e.Services...)
	}
	slices.Sort(out)
	return slices.Compact(out)
}

// classify returns the index of the catalog entry a relative slash path belongs to, or -1 when no
// entry claims it (an unknown file).
func classify(rel string) int {
	for i, e := range catalog {
		switch {
		case e.Pattern != "":
			if patterns[e.Name].MatchString(rel) {
				return i
			}
		case e.isDir():
			if strings.HasPrefix(rel, e.Name) && len(rel) > len(e.Name) {
				return i
			}
		case rel == e.Name:
			return i
		}
	}
	return -1
}
