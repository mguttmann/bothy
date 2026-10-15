// SPDX-License-Identifier: Apache-2.0

package s3sink

import (
	"context"
	"errors"
	"net/netip"
	"net/url"
	"strconv"
	"strings"

	"github.com/OWNER/bothy/services/server/internal/audit/domain"
	"github.com/OWNER/bothy/services/server/internal/platform/egress"
)

// The configuration keys the errors of the sink name (19 section 16).
const (
	keySink         = "audit.checkpoint_sink"
	keyEndpoint     = "audit.checkpoint_s3_endpoint"
	keyRegion       = "audit.checkpoint_s3_region"
	keyPathStyle    = "audit.checkpoint_s3_use_path_style"
	keyAccessRef    = "audit.checkpoint_s3_access_key_ref"
	keySecretRef    = "audit.checkpoint_s3_secret_key_ref" // #nosec G101 -- a configuration key name, not a credential
	keyRetention    = "retention.audit"
	keyAllowedHosts = "egress.allowed_hosts"
)

// Config is the object store target of audit.checkpoint_sink = s3://<bucket>[/<prefix>] and the
// keys audit.checkpoint_s3_* (19 section 16). It is separate from file.s3.* and from the backup:
// own endpoint, own bucket and own credentials (runbook backup-restore 8 item 1).
type Config struct {
	// Bucket and Prefix come from audit.checkpoint_sink (domain.ParseSink).
	Bucket string
	// Prefix is "" or segments of [a-z0-9-] joined by "/", put before every object key.
	Prefix string
	// Endpoint is audit.checkpoint_s3_endpoint: an absolute http or https URL with a host and
	// nothing after it but an optional "/". The sink never uses the endpoint resolution of the SDK.
	Endpoint string
	// Region is audit.checkpoint_s3_region, the region of the signature.
	Region string
	// UsePathStyle is audit.checkpoint_s3_use_path_style: requests go to
	// <endpoint>/<bucket>/<object> instead of <bucket>.<endpoint host>.
	UsePathStyle bool
	// AccessKeyRef is audit.checkpoint_s3_access_key_ref, the secret reference of the access key ID.
	AccessKeyRef string
	// SecretKeyRef is audit.checkpoint_s3_secret_key_ref, the secret reference of the secret key.
	SecretKeyRef string
}

// target is the scheme, canonical host and port of every request of the sink.
type target struct {
	scheme string
	host   string // lower case without trailing dot, or an IP literal as net/netip prints it
	port   uint16
	ip     bool
}

// errEndpoint is the error of an endpoint the sink refuses; it names the key, never the value.
var errEndpoint = errors.New(keyEndpoint + " must be an absolute http or https URL with a host and without user info, query, fragment or a path other than / when " + keySink + " is s3://")

// The schemes of audit.checkpoint_s3_endpoint.
const (
	schemeHTTP  = "http"
	schemeHTTPS = "https"
)

// requestTarget parses Endpoint into the target of the requests: the endpoint host with path
// style, else <bucket>.<endpoint host>.
func (c Config) requestTarget() (target, error) {
	if c.Endpoint == "" {
		return target{}, errors.New(keyEndpoint + " must be set when " + keySink + " is s3://")
	}
	u, err := url.Parse(c.Endpoint)
	if err != nil || (u.Scheme != schemeHTTP && u.Scheme != schemeHTTPS) || u.Opaque != "" || u.User != nil ||
		u.RawQuery != "" || u.ForceQuery || strings.Contains(c.Endpoint, "#") ||
		(u.Path != "" && u.Path != "/") || u.Hostname() == "" {
		return target{}, errEndpoint
	}
	t, err := endpointHost(u)
	if err != nil {
		return target{}, err
	}
	if !c.UsePathStyle {
		t.host = c.Bucket + "." + t.host
	}
	return t, nil
}

// endpointHost returns the scheme, canonical host and port of a parsed endpoint: an IP literal
// without zone, or a DNS name of labels of [a-z0-9-].
func endpointHost(u *url.URL) (target, error) {
	t := target{scheme: u.Scheme, port: 443}
	if u.Scheme == schemeHTTP {
		t.port = 80
	}
	if p := u.Port(); p != "" {
		n, err := strconv.ParseUint(p, 10, 16)
		if err != nil || n == 0 {
			return target{}, errEndpoint
		}
		t.port = uint16(n)
	}
	if a, err := netip.ParseAddr(u.Hostname()); err == nil {
		if a.Zone() != "" {
			return target{}, errEndpoint
		}
		t.host, t.ip = a.Unmap().String(), true
		return t, nil
	}
	t.host = strings.TrimSuffix(strings.ToLower(u.Hostname()), ".")
	for _, label := range strings.Split(t.host, ".") {
		if label == "" || len(label) > 63 || strings.Trim(label, "abcdefghijklmnopqrstuvwxyz0123456789-") != "" ||
			label[0] == '-' || label[len(label)-1] == '-' {
			return target{}, errEndpoint
		}
	}
	return t, nil
}

// validRegion accepts a region name of [a-z0-9-] (us-east-1, eu-central-1, auto).
func validRegion(r string) bool {
	return r != "" && len(r) <= 63 && strings.Trim(r, "abcdefghijklmnopqrstuvwxyz0123456789-") == ""
}

// Validate checks the target without any request, as the cross-check audit_checkpoint_sink does
// at load and New does at construction, against the entries of egress.allowed_hosts:
//
//   - the bucket follows the S3 naming rules (domain.ValidBucket);
//   - audit.checkpoint_s3_endpoint is an absolute http or https URL with a host, without user
//     info, query, fragment or a path other than "/";
//   - audit.checkpoint_s3_region is a region name;
//   - with audit.checkpoint_s3_use_path_style = false the endpoint host is a DNS name;
//   - an http endpoint needs an entry with allow_private for the request host and port, because
//     the egress client allows http only there (19 SEC-NET-2).
//
// The error names keys, never values.
//
// Example:
//
//	cfg := s3sink.Config{Bucket: "bothy-audit", Endpoint: "https://s3.example.com", Region: "us-east-1", UsePathStyle: true}
//	err := cfg.Validate(nil) // nil
func (c Config) Validate(allowed []egress.Rule) error {
	if !domain.ValidBucket(c.Bucket) {
		return errors.New(keySink + " must name a bucket that follows the S3 bucket naming rules")
	}
	path := c
	path.UsePathStyle = true
	ep, err := path.requestTarget()
	if err != nil {
		return err
	}
	switch {
	case !validRegion(c.Region):
		return errors.New(keyRegion + " must be a region name of lower-case letters, digits and - when " + keySink + " is s3://")
	case !c.UsePathStyle && ep.ip:
		return errors.New(keyPathStyle + " must be true when the host of " + keyEndpoint + " is an IP address")
	}
	if ep.scheme != schemeHTTP {
		return nil
	}
	t, _ := c.requestTarget()
	for _, r := range allowed {
		if r.AllowPrivate && r.Host == t.host && (r.Port == t.port || (r.Port == 0 && t.port == 80)) {
			return nil
		}
	}
	return errors.New("an http " + keyEndpoint + " needs an " + keyAllowedHosts + " entry with allow_private for the request host and port")
}

// HostPolicy returns the consumer policy of the egress client of the sink (19 SEC-NET-2): exactly
// the scheme, host and port of every request. An endpoint that Validate refuses allows nothing.
// The policy never opens http or private addresses; only an entry of egress.allowed_hosts with
// allow_private does.
func (c Config) HostPolicy() egress.HostPolicy {
	t, err := c.requestTarget()
	if err != nil {
		return func(context.Context, string, string, uint16) bool { return false }
	}
	return func(_ context.Context, scheme, host string, port uint16) bool {
		return scheme == t.scheme && host == t.host && port == t.port
	}
}
