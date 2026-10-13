// SPDX-License-Identifier: Apache-2.0

package wire

import (
	"errors"
	"fmt"
	"net"
	"net/netip"
	"net/url"
	"time"

	identityapp "github.com/OWNER/bothy/services/server/internal/identity/app"
	"github.com/OWNER/bothy/services/server/internal/platform/config"
	"github.com/OWNER/bothy/services/server/internal/platform/httpserver"
	"github.com/OWNER/bothy/services/server/internal/platform/jobs"
)

// configCheck is a cross-check of the configuration schema.
type configCheck struct {
	name string
	keys []string
	fn   func(*config.Values) error
}

// configChecks are the cross-checks of docs/spec/02-architektur.md "Konfigurationsladen" item 4,
// and the check of a key whose entries a platform package parses (egress_allowed_hosts, in
// egress.go). Their errors become problem messages verbatim, so they name keys and never values.
var configChecks = []configCheck{
	{name: "listeners_distinct", keys: []string{"server.listen_addr", "server.internal_listen_addr"}, fn: listenersDistinct},
	{name: "json_limit_above_message", keys: []string{"http.max_json_body_bytes", "message.max_content_bytes"}, fn: jsonLimitAboveMessage},
	{name: "egress_allowed_hosts", keys: []string{keyEgressAllowedHosts}, fn: egressAllowedHosts},
	{name: "public_url_origin", keys: []string{keyPublicURL}, fn: publicURLOrigin},
	{name: "cors_origins", keys: []string{"api.cors_allowed_origins"}, fn: corsOrigins},
	{name: "tls_pair", keys: []string{keyTLSCertRef, keyTLSKeyRef}, fn: tlsPair},
	{name: "tls_https", keys: []string{keyPublicURL, keyTLSCertRef, keyTLSKeyRef}, fn: tlsHTTPS},
	{name: "hsts_whole_seconds", keys: []string{"http.hsts_max_age"}, fn: hstsWholeSeconds},
	{name: "audit_ip_key_retention", keys: []string{"privacy.audit_ip_key_period", "privacy.audit_ip_key_retention"}, fn: auditIPKeyRetention},
	{name: "jobs_schema", keys: []string{"jobs.schema"}, fn: jobsSchema},
	{name: "file_fs_root_absolute", keys: []string{keyStorageDriver, "file.fs.root"}, fn: fileFSRootAbsolute},
	{name: "file_s3", keys: []string{keyStorageDriver, "file.s3.endpoint", "file.s3.region", "file.s3.bucket", "file.s3.prefix",
		"file.s3.use_path_style", "file.s3.checksum_mode", keyEgressAllowedHosts}, fn: fileS3},
	{name: "consumer_alert_below_retention", keys: []string{keyConsumerPendingAlertAfter, keySyncEventRetention}, fn: consumerAlertBelowRetention},
	{name: "dpop_required_for", keys: []string{keyDPoPRequiredFor}, fn: dpopRequiredFor},
	{name: "jwks_max_age_below_rotation", keys: []string{keyJWKSMaxAge, keyInvocationRotation, keyAccessRotation}, fn: jwksMaxAgeBelowRotation},
}

// The keys of the cross-check jwks_max_age_below_rotation.
const (
	keyJWKSMaxAge         = "agent.jwks_max_age"
	keyInvocationRotation = "signing_key.invocation_rotation_interval"
	keyAccessRotation     = "signing_key.access_token_rotation_interval"
)

// jwksMaxAgeBelowRotation requires agent.jwks_max_age to be strictly below both scheduled
// rotation intervals (M1-32, 09 section 4.5): a next key is published that long before it signs,
// so an interval not above it would announce the following key before the last one ever signed.
func jwksMaxAgeBelowRotation(v *config.Values) error {
	maxAge := v.Duration(keyJWKSMaxAge)
	var problems []error
	for _, key := range []string{keyInvocationRotation, keyAccessRotation} {
		if maxAge >= v.Duration(key) {
			problems = append(problems, errors.New(keyJWKSMaxAge+" must be less than "+key))
		}
	}
	return errors.Join(problems...)
}

// dpopRequiredFor refuses a value of auth.dpop_required_for outside agent, app and personal (08
// section 18, M1-44); the error names the key, never the value.
func dpopRequiredFor(v *config.Values) error {
	return identityapp.CheckDPoPRequiredFor(v.Strings(keyDPoPRequiredFor))
}

// consumerAlertBelowRetention requires consumer.pending_alert_after to be strictly below
// sync.event_retention (M2-01, D-M2-01-6): the alarm BothyEventConsumerLag must fire while a
// stalled worker consumer can still catch up, before its events fall out of the retained log.
func consumerAlertBelowRetention(v *config.Values) error {
	if v.Duration(keyConsumerPendingAlertAfter) >= v.Duration(keySyncEventRetention) {
		return errors.New("consumer.pending_alert_after must be less than sync.event_retention")
	}
	return nil
}

// tlsPair requires server.tls_cert_ref and server.tls_key_ref to be both set or both empty
// (SEC-TLS-2): one of them alone is a half switched on TLS that the start could only guess.
func tlsPair(v *config.Values) error {
	if (v.String(keyTLSCertRef) == "") != (v.String(keyTLSKeyRef) == "") {
		return errors.New("server.tls_cert_ref and server.tls_key_ref must both be set or both be empty")
	}
	return nil
}

// tlsHTTPS requires an https server.public_url when the server terminates TLS itself (SEC-TLS-2):
// the apps reach the server under that URL, and HSTS follows its scheme (SEC-TLS-3).
func tlsHTTPS(v *config.Values) error {
	if v.String(keyTLSCertRef) == "" || v.String(keyTLSKeyRef) == "" {
		return nil
	}
	if u, err := url.Parse(v.String(keyPublicURL)); err != nil || u.Scheme != "https" {
		return errors.New("with server.tls_cert_ref and server.tls_key_ref set, server.public_url must use https")
	}
	return nil
}

// hstsWholeSeconds requires http.hsts_max_age in whole seconds: Strict-Transport-Security
// carries max-age as a whole number of seconds (RFC 6797 6.1.1), and rounding would send another
// value than the operator configured.
func hstsWholeSeconds(v *config.Values) error {
	if v.Duration("http.hsts_max_age")%time.Second != 0 {
		return errors.New("http.hsts_max_age must be a whole number of seconds")
	}
	return nil
}

// auditIPKeyRetention requires privacy.audit_ip_key_retention to be at least
// privacy.audit_ip_key_period (M1-24): a key must outlive its own period, or the destruction job
// of M7-05 would destroy the key that the current entries still use.
func auditIPKeyRetention(v *config.Values) error {
	if v.Duration("privacy.audit_ip_key_retention") < v.Duration("privacy.audit_ip_key_period") {
		return errors.New("privacy.audit_ip_key_retention must be at least privacy.audit_ip_key_period")
	}
	return nil
}

// publicURLOrigin requires server.public_url to be an origin: scheme, host, optional port and
// nothing else (16 "Konfigurationswerte"), because the DPoP htu is this origin plus the path.
func publicURLOrigin(v *config.Values) error {
	if err := httpserver.CheckOrigin(v.String(keyPublicURL)); err != nil {
		return errors.New("server.public_url must be an origin (scheme://host[:port], no path, query or fragment): " + err.Error())
	}
	return nil
}

// corsOrigins requires every entry of api.cors_allowed_origins to be an origin; the chain
// compares the Origin header with the entries exactly.
func corsOrigins(v *config.Values) error {
	for i, o := range v.Strings("api.cors_allowed_origins") {
		if err := httpserver.CheckOrigin(o); err != nil {
			return fmt.Errorf("api.cors_allowed_origins entry %d must be an origin (scheme://host[:port]): %w", i+1, err)
		}
	}
	return nil
}

// jobsSchema holds jobs.schema to an unquoted identifier other than public and pg_* (M1-14):
// River puts the name into its statements as it is, and the domain schema stays free of River's
// tables (R-AR-09).
func jobsSchema(v *config.Values) error { return jobs.ValidateSchema(v.String("jobs.schema")) }

// listenersDistinct rejects a public and an internal listener that would bind the same socket:
// equal ports other than 0 on equal hosts, or when either host is empty, 0.0.0.0 or ::. Port 0
// may repeat because tests bind port 0 (PLAN P-10). IPv4-mapped IPv6 addresses count as their
// IPv4 address, and an unspecified address with an IPv6 zone counts as unspecified.
func listenersDistinct(v *config.Values) error {
	if listenersConflict(v.String("server.listen_addr"), v.String("server.internal_listen_addr")) {
		return errors.New("the public and the internal listener must not share a port on the same or an unspecified address; only port 0 may repeat")
	}
	return nil
}

// listenersConflict compares two canonical listen addresses (config.KindListenAddr).
func listenersConflict(a, b string) bool {
	hostA, portA, errA := net.SplitHostPort(a)
	hostB, portB, errB := net.SplitHostPort(b)
	if errA != nil || errB != nil || portA != portB || portA == "0" {
		return false
	}
	if hostA == "" || hostB == "" {
		return true
	}
	addrA, errA := netip.ParseAddr(hostA)
	addrB, errB := netip.ParseAddr(hostB)
	if errA != nil || errB != nil {
		return false
	}
	addrA, addrB = addrA.Unmap(), addrB.Unmap()
	return unspecified(addrA) || unspecified(addrB) || addrA == addrB
}

// unspecified reports 0.0.0.0 and ::, also with an IPv6 zone: the kernel binds the wildcard for
// "::%eth0" as for "::".
func unspecified(a netip.Addr) bool { return a.WithZone("").IsUnspecified() }

// jsonLimitAboveMessage requires the JSON body limit to be strictly greater than the message
// size: the envelope of a message is small against the gap of the defaults, and a separate value
// for it would be a guess without measurement (02 "Konfigurationsladen" item 4).
func jsonLimitAboveMessage(v *config.Values) error {
	if v.Int("http.max_json_body_bytes") <= v.Int("message.max_content_bytes") {
		return errors.New("http.max_json_body_bytes must be greater than message.max_content_bytes")
	}
	return nil
}
