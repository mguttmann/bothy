// SPDX-License-Identifier: Apache-2.0

package s3

import (
	"context"
	"errors"
	"net/netip"
	"net/url"
	"strconv"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"

	"github.com/OWNER/bothy/services/server/internal/files/domain"
	"github.com/OWNER/bothy/services/server/internal/platform/egress"
)

// The values of file.s3.checksum_mode (docs/DECISIONS.md M3-02/10).
const (
	// ChecksumWhenRequired lets the SDK compute and check checksums only where an operation
	// requires them; the default (docs/spec/12-dateien.md "4.3", [Q3]).
	ChecksumWhenRequired = "when_required"
	// ChecksumWhenSupported lets the SDK compute and check checksums wherever an operation
	// supports them, as the SDK does by default.
	ChecksumWhenSupported = "when_supported"
)

// Config is the configuration of the S3 driver, the keys file.s3.* of
// docs/spec/12-dateien.md "18."; internal/wire binds it with config.Bind.
type Config struct {
	// Endpoint is file.s3.endpoint: an absolute http or https URL with a host and nothing
	// after it but an optional "/". The driver never uses the endpoint resolution of the SDK.
	Endpoint string `config:"file.s3.endpoint"`
	// Region is file.s3.region, the region of the signature.
	Region string `config:"file.s3.region"`
	// Bucket is file.s3.bucket.
	Bucket string `config:"file.s3.bucket"`
	// Prefix is file.s3.prefix: empty, or a name in the key grammar put before every key.
	Prefix string `config:"file.s3.prefix"`
	// UsePathStyle is file.s3.use_path_style: requests go to <endpoint>/<bucket>/<object>
	// instead of <bucket>.<endpoint host>.
	UsePathStyle bool `config:"file.s3.use_path_style"`
	// ChecksumMode is file.s3.checksum_mode, ChecksumWhenRequired or ChecksumWhenSupported.
	ChecksumMode string `config:"file.s3.checksum_mode"`
	// AccessKeyRef is file.s3.access_key_ref, the secret reference of the access key ID.
	AccessKeyRef string `config:"file.s3.access_key_ref"`
	// SecretKeyRef is file.s3.secret_key_ref, the secret reference of the secret access key.
	SecretKeyRef string `config:"file.s3.secret_key_ref"`
}

// The schemes of file.s3.endpoint.
const (
	schemeHTTP  = "http"
	schemeHTTPS = "https"
)

// target is the scheme, canonical host and port of a request of the driver.
type target struct {
	scheme string
	host   string // lower case without trailing dot, or an IP literal as net/netip prints it
	port   uint16
	ip     bool
}

// endpoint parses Endpoint into the target of path-style requests, or reports why it is not an
// endpoint the driver accepts. The message names the key, never the value.
func (c Config) endpoint() (target, error) {
	const bad = "file.s3.endpoint must be an absolute http or https URL with a host and without user info, query, fragment or a path other than /"
	if c.Endpoint == "" {
		return target{}, errors.New("file.s3.endpoint must be set when file.storage.driver is s3")
	}
	u, err := url.Parse(c.Endpoint)
	if err != nil || (u.Scheme != schemeHTTP && u.Scheme != schemeHTTPS) || u.Opaque != "" || u.User != nil ||
		u.RawQuery != "" || u.ForceQuery || u.Fragment != "" || strings.Contains(c.Endpoint, "#") ||
		(u.Path != "" && u.Path != "/") || u.Hostname() == "" {
		return target{}, errors.New(bad)
	}
	t := target{scheme: u.Scheme, port: 443}
	if u.Scheme == schemeHTTP {
		t.port = 80
	}
	if p := u.Port(); p != "" {
		n, err := strconv.ParseUint(p, 10, 16)
		if err != nil || n == 0 {
			return target{}, errors.New(bad)
		}
		t.port = uint16(n)
	}
	host := u.Hostname()
	if a, err := netip.ParseAddr(host); err == nil {
		if a.Zone() != "" {
			return target{}, errors.New(bad)
		}
		t.host, t.ip = a.Unmap().String(), true
		return t, nil
	}
	host = strings.TrimSuffix(strings.ToLower(host), ".")
	if !dnsName(host) {
		return target{}, errors.New(bad)
	}
	t.host = host
	return t, nil
}

// requestTarget is the target of every request: the endpoint with path style, else
// <bucket>.<endpoint host>.
func (c Config) requestTarget() (target, error) {
	t, err := c.endpoint()
	if err != nil {
		return target{}, err
	}
	if !c.UsePathStyle {
		t.host = c.Bucket + "." + t.host
	}
	return t, nil
}

// dnsName reports a host name of labels of [a-z0-9-], 1 to 63 bytes each, not starting or ending
// with "-", at most 253 bytes.
func dnsName(name string) bool {
	if name == "" || len(name) > 253 {
		return false
	}
	for _, l := range strings.Split(name, ".") {
		if l == "" || len(l) > 63 || l[0] == '-' || l[len(l)-1] == '-' {
			return false
		}
		for i := 0; i < len(l); i++ {
			c := l[i]
			if (c < 'a' || c > 'z') && (c < '0' || c > '9') && c != '-' {
				return false
			}
		}
	}
	return true
}

// validBucket checks the S3 bucket naming rules for general purpose buckets: 3 to 63 bytes of
// lower-case letters, digits, "." and "-", beginning and ending with a letter or digit, no "..",
// not an IPv4 address, and none of the prefixes and suffixes S3 reserves (directory buckets end
// with "--x-s3", which would make the SDK open a session with CreateSession).
func validBucket(b string) bool {
	if len(b) < 3 || len(b) > 63 || strings.Contains(b, "..") || !alnum(b[0]) || !alnum(b[len(b)-1]) {
		return false
	}
	for i := 0; i < len(b); i++ {
		if c := b[i]; !alnum(c) && c != '.' && c != '-' {
			return false
		}
	}
	if a, err := netip.ParseAddr(b); err == nil && a.Is4() {
		return false
	}
	return !reservedBucket(b)
}

// reservedBucket reports the prefixes and suffixes S3 reserves for other bucket types.
func reservedBucket(b string) bool {
	for _, p := range []string{"xn--", "sthree-", "amzn-s3-demo-"} {
		if strings.HasPrefix(b, p) {
			return true
		}
	}
	for _, s := range []string{"-s3alias", "--ol-s3", "--x-s3", "--table-s3", ".mrap"} {
		if strings.HasSuffix(b, s) {
			return true
		}
	}
	return false
}

func alnum(c byte) bool { return ('a' <= c && c <= 'z') || ('0' <= c && c <= '9') }

// validRegion accepts a region name of [a-z0-9-] (us-east-1, eu-central-1, auto), which the
// signature carries in the credential scope.
func validRegion(r string) bool {
	if r == "" || len(r) > 63 {
		return false
	}
	for i := 0; i < len(r); i++ {
		c := r[i]
		if !alnum(c) && c != '-' {
			return false
		}
	}
	return true
}

// Validate checks the configuration as the cross-check file_s3 does at load and New does at
// construction (docs/DECISIONS.md M3-02/2 and M3-02/11), against the entries of
// egress.allowed_hosts:
//
//   - file.s3.endpoint is an absolute http or https URL with a host, without user info, query,
//     fragment or a path other than "/";
//   - file.s3.bucket follows the S3 bucket naming rules, file.s3.region is set;
//   - file.s3.prefix is empty or follows the key grammar;
//   - with file.s3.use_path_style = false the endpoint host is a DNS name, not an IP literal;
//   - file.s3.checksum_mode is when_required or when_supported, and when_supported only with an
//     https endpoint (the SDK sends the checksum of a streamed body as a trailer, over TLS only);
//   - an http endpoint needs an entry with allow_private for the request host and port, because
//     the egress client allows http only there (19 SEC-NET-2).
//
// The error names keys, never values.
//
// Example:
//
//	cfg := s3.Config{Endpoint: "https://s3.example.com", Region: "us-east-1", Bucket: "bothy-files",
//		UsePathStyle: true, ChecksumMode: s3.ChecksumWhenRequired}
//	err := cfg.Validate(nil) // nil
func (c Config) Validate(allowed []egress.Rule) error {
	ep, err := c.endpoint()
	if err != nil {
		return err
	}
	switch {
	case !validBucket(c.Bucket):
		return errors.New("file.s3.bucket must be set and follow the S3 bucket naming rules when file.storage.driver is s3")
	case !validRegion(c.Region):
		return errors.New("file.s3.region must be set to a region name of lower-case letters, digits and - when file.storage.driver is s3")
	case !c.UsePathStyle && ep.ip:
		return errors.New("file.s3.use_path_style must be true when the host of file.s3.endpoint is an IP address")
	}
	if c.Prefix != "" && domain.ValidateKey(c.Prefix) != nil {
		return errors.New("file.s3.prefix must be empty or segments of [a-z0-9-] joined by single /, without a leading or trailing /")
	}
	if _, _, err := checksumModes(c.ChecksumMode); err != nil {
		return err
	}
	if ep.scheme == schemeHTTP && c.ChecksumMode == ChecksumWhenSupported {
		return errors.New("file.s3.checksum_mode when_supported needs an https file.s3.endpoint: the SDK sends the checksum of a streamed body as a trailer, which it allows over TLS only")
	}
	if ep.scheme == schemeHTTP {
		t, _ := c.requestTarget()
		if !privateEntry(allowed, t) {
			return errors.New("an http file.s3.endpoint needs an egress.allowed_hosts entry with allow_private for the request host and port (with file.s3.use_path_style false: <file.s3.bucket>.<host>)")
		}
	}
	return nil
}

// privateEntry reports whether an entry with allow_private matches the host and port of t; an
// entry without port matches the default port of the scheme, as in the egress client.
func privateEntry(allowed []egress.Rule, t target) bool {
	for _, r := range allowed {
		if r.AllowPrivate && r.Host == t.host && (r.Port == t.port || (r.Port == 0 && t.port == 80)) {
			return true
		}
	}
	return false
}

// HostPolicy returns the consumer policy of the egress client of the driver (19 SEC-NET-2, a
// consumer names its own targets): exactly the scheme, host and port of every request, the
// endpoint with path style or <bucket>.<endpoint host> with virtual-hosted style, and the port of
// the endpoint or the default port of its scheme (docs/DECISIONS.md M3-02/1). An endpoint that
// Validate refuses allows nothing. The policy never opens http or private addresses; only an
// entry of egress.allowed_hosts with allow_private does.
func (c Config) HostPolicy() egress.HostPolicy {
	t, err := c.requestTarget()
	if err != nil {
		return func(context.Context, string, string, uint16) bool { return false }
	}
	return func(_ context.Context, scheme, host string, port uint16) bool {
		return scheme == t.scheme && host == t.host && port == t.port
	}
}

// checksumModes maps file.s3.checksum_mode to the request and response settings of the SDK
// (docs/DECISIONS.md M3-02/10).
func checksumModes(mode string) (aws.RequestChecksumCalculation, aws.ResponseChecksumValidation, error) {
	switch mode {
	case ChecksumWhenRequired:
		return aws.RequestChecksumCalculationWhenRequired, aws.ResponseChecksumValidationWhenRequired, nil
	case ChecksumWhenSupported:
		return aws.RequestChecksumCalculationWhenSupported, aws.ResponseChecksumValidationWhenSupported, nil
	}
	return 0, 0, errors.New("file.s3.checksum_mode must be when_required or when_supported")
}
