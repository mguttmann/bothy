// SPDX-License-Identifier: Apache-2.0

package s3_test

import (
	"context"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"

	"github.com/OWNER/bothy/services/server/internal/files/adapters/s3"
	"github.com/OWNER/bothy/services/server/internal/platform/egress"
)

// validConfig is an https configuration that Validate accepts.
func validConfig() s3.Config {
	return s3.Config{
		Endpoint:     "https://s3.example.com",
		Region:       testRegion,
		Bucket:       "bothy-files",
		UsePathStyle: true,
		ChecksumMode: "when_required",
		AccessKeyRef: "file:<secrets.dir>/s3_access_key",
		SecretKeyRef: "file:<secrets.dir>/s3_secret_key",
	}
}

func rule(t *testing.T, s string) egress.Rule {
	t.Helper()
	r, err := egress.ParseRule(s)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

// TestConfigValidate holds the rules a to e of the cross-check file_s3 (docs/DECISIONS.md
// M3-02/2 and M3-02/11): each violation names its key, and a canary in the value never appears in
// the message.
func TestConfigValidate(t *testing.T) {
	t.Parallel()
	const canary = "canary7q"
	cases := []struct {
		name    string
		mutate  func(*s3.Config)
		allowed []string
		key     string
	}{
		{"endpoint empty", func(c *s3.Config) { c.Endpoint = "" }, nil, "file.s3.endpoint"},
		{"endpoint relative", func(c *s3.Config) { c.Endpoint = canary + ".example.com" }, nil, "file.s3.endpoint"},
		{"endpoint ftp", func(c *s3.Config) { c.Endpoint = "ftp://" + canary + ".example.com" }, nil, "file.s3.endpoint"},
		{"endpoint user info", func(c *s3.Config) { c.Endpoint = "https://" + canary + ":pw@s3.example.com" }, nil, "file.s3.endpoint"},
		{"endpoint query", func(c *s3.Config) { c.Endpoint = "https://s3.example.com/?x=" + canary }, nil, "file.s3.endpoint"},
		{"endpoint fragment", func(c *s3.Config) { c.Endpoint = "https://s3.example.com/#" + canary }, nil, "file.s3.endpoint"},
		{"endpoint path", func(c *s3.Config) { c.Endpoint = "https://s3.example.com/" + canary }, nil, "file.s3.endpoint"},
		{"endpoint without host", func(c *s3.Config) { c.Endpoint = "https:///" + canary }, nil, "file.s3.endpoint"},
		{"endpoint port zero", func(c *s3.Config) { c.Endpoint = "https://" + canary + ".example.com:0" }, nil, "file.s3.endpoint"},
		{"endpoint bad host", func(c *s3.Config) { c.Endpoint = "https://" + canary + "_x.example.com" }, nil, "file.s3.endpoint"},
		{"bucket empty", func(c *s3.Config) { c.Bucket = "" }, nil, "file.s3.bucket"},
		{"bucket upper case", func(c *s3.Config) { c.Bucket = "Upper-" + canary }, nil, "file.s3.bucket"},
		{"bucket too short", func(c *s3.Config) { c.Bucket = "ab" }, nil, "file.s3.bucket"},
		{"bucket too long", func(c *s3.Config) { c.Bucket = strings.Repeat("a", 64) }, nil, "file.s3.bucket"},
		{"bucket ip", func(c *s3.Config) { c.Bucket = "192.0.2.1" }, nil, "file.s3.bucket"},
		{"bucket double dot", func(c *s3.Config) { c.Bucket = canary + "..x" }, nil, "file.s3.bucket"},
		{"bucket leading hyphen", func(c *s3.Config) { c.Bucket = "-" + canary }, nil, "file.s3.bucket"},
		{"bucket directory suffix", func(c *s3.Config) { c.Bucket = canary + "--x-s3" }, nil, "file.s3.bucket"},
		{"bucket reserved prefix", func(c *s3.Config) { c.Bucket = "xn--" + canary }, nil, "file.s3.bucket"},
		{"region empty", func(c *s3.Config) { c.Region = "" }, nil, "file.s3.region"},
		{"region with slash", func(c *s3.Config) { c.Region = canary + "/x" }, nil, "file.s3.region"},
		{"prefix slash", func(c *s3.Config) { c.Prefix = "/" + canary }, nil, "file.s3.prefix"},
		{"prefix dots", func(c *s3.Config) { c.Prefix = canary + "/.." }, nil, "file.s3.prefix"},
		{"virtual hosted on ip", func(c *s3.Config) { c.UsePathStyle = false; c.Endpoint = "https://192.0.2.1" }, nil, "file.s3.use_path_style"},
		{"checksum mode", func(c *s3.Config) { c.ChecksumMode = canary }, nil, "file.s3.checksum_mode"},
		{"http without entry", func(c *s3.Config) { c.Endpoint = "http://" + canary + ".example.com:8333" }, nil, "egress.allowed_hosts"},
		{"http entry without allow_private", func(c *s3.Config) { c.Endpoint = "http://" + canary + ".example.com:8333" }, []string{canary + ".example.com:8333"}, "egress.allowed_hosts"},
		{"http entry on another port", func(c *s3.Config) { c.Endpoint = "http://" + canary + ".example.com:8333" }, []string{canary + ".example.com:9333;allow_private"}, "egress.allowed_hosts"},
		{"http entry for the endpoint, not the virtual host", func(c *s3.Config) {
			c.UsePathStyle = false
			c.Endpoint = "http://" + canary + ".example.com:8333"
		}, []string{canary + ".example.com:8333;allow_private"}, "egress.allowed_hosts"},
		{"when_supported over http", func(c *s3.Config) {
			c.Endpoint = "http://" + canary + ".example.com:8333"
			c.ChecksumMode = "when_supported"
		}, []string{canary + ".example.com:8333;allow_private"}, "file.s3.checksum_mode"},
		{"http default port, entry with other port", func(c *s3.Config) { c.Endpoint = "http://" + canary + ".example.com" }, []string{canary + ".example.com:8080;allow_private"}, "egress.allowed_hosts"},
	}
	for _, c := range cases {
		cfg := validConfig()
		c.mutate(&cfg)
		var rules []egress.Rule
		for _, a := range c.allowed {
			rules = append(rules, rule(t, a))
		}
		err := cfg.Validate(rules)
		if err == nil {
			t.Errorf("%s: Validate accepted %+v", c.name, cfg)
			continue
		}
		if !strings.Contains(err.Error(), c.key) {
			t.Errorf("%s: error %q does not name %s", c.name, err, c.key)
		}
		if strings.Contains(err.Error(), canary) {
			t.Errorf("%s: error %q echoes the value", c.name, err)
		}
	}
	ok := []struct {
		name    string
		mutate  func(*s3.Config)
		allowed []string
	}{
		{"https", func(*s3.Config) {}, nil},
		{"https with slash and port", func(c *s3.Config) { c.Endpoint = "https://s3.example.com:9000/" }, nil},
		{"virtual hosted", func(c *s3.Config) { c.UsePathStyle = false }, nil},
		{"prefix", func(c *s3.Config) { c.Prefix = "env-a/prod" }, nil},
		{"when_supported", func(c *s3.Config) { c.ChecksumMode = "when_supported" }, nil},
		{"http with entry", func(c *s3.Config) { c.Endpoint = "http://seaweedfs:8333" }, []string{"seaweedfs:8333;allow_private"}},
		{"http ip with entry", func(c *s3.Config) { c.Endpoint = "http://192.0.2.1:8333" }, []string{"192.0.2.1:8333;allow_private"}},
		{"http default port with portless entry", func(c *s3.Config) { c.Endpoint = "http://s3.example.com" }, []string{"s3.example.com;allow_private"}},
		{"http virtual hosted with entry", func(c *s3.Config) {
			c.UsePathStyle = false
			c.Endpoint = "http://s3.example.com:8333"
		}, []string{"bothy-files.s3.example.com:8333;allow_private"}},
		{"bucket with dots", func(c *s3.Config) { c.Bucket = "files.example.com" }, nil},
	}
	for _, c := range ok {
		cfg := validConfig()
		c.mutate(&cfg)
		var rules []egress.Rule
		for _, a := range c.allowed {
			rules = append(rules, rule(t, a))
		}
		if err := cfg.Validate(rules); err != nil {
			t.Errorf("%s: Validate = %v", c.name, err)
		}
	}
}

// TestChecksumModeMapping holds docs/DECISIONS.md M3-02/10: both values of file.s3.checksum_mode
// map to their SDK pair for request and response.
func TestChecksumModeMapping(t *testing.T) {
	t.Parallel()
	req, resp, err := s3.ChecksumModes("when_required")
	if err != nil || req != aws.RequestChecksumCalculationWhenRequired || resp != aws.ResponseChecksumValidationWhenRequired {
		t.Errorf("when_required = %v, %v, %v", req, resp, err)
	}
	req, resp, err = s3.ChecksumModes("when_supported")
	if err != nil || req != aws.RequestChecksumCalculationWhenSupported || resp != aws.ResponseChecksumValidationWhenSupported {
		t.Errorf("when_supported = %v, %v, %v", req, resp, err)
	}
	if _, _, err := s3.ChecksumModes("always"); err == nil {
		t.Error("an unknown mode mapped")
	}
}

// TestHostPolicy holds docs/DECISIONS.md M3-02/1: the consumer policy allows exactly the request
// host and port of the endpoint.
func TestHostPolicy(t *testing.T) {
	t.Parallel()
	type req struct {
		scheme, host string
		port         uint16
	}
	cases := []struct {
		name    string
		mutate  func(*s3.Config)
		allowed []req
		refused []req
	}{
		{"path style", func(*s3.Config) {}, []req{{"https", "s3.example.com", 443}},
			[]req{{"https", "bothy-files.s3.example.com", 443}, {"https", "s3.example.com", 444}, {"http", "s3.example.com", 443}, {"https", "example.com", 443}, {"https", "evil.example.com", 443}}},
		{"path style with port", func(c *s3.Config) { c.Endpoint = "https://s3.example.com:9000" }, []req{{"https", "s3.example.com", 9000}},
			[]req{{"https", "s3.example.com", 443}}},
		{"virtual hosted", func(c *s3.Config) { c.UsePathStyle = false }, []req{{"https", "bothy-files.s3.example.com", 443}},
			[]req{{"https", "s3.example.com", 443}, {"https", "other.s3.example.com", 443}}},
		{"upper case endpoint", func(c *s3.Config) { c.Endpoint = "https://S3.Example.COM." }, []req{{"https", "s3.example.com", 443}}, nil},
		{"ip", func(c *s3.Config) { c.Endpoint = "http://192.0.2.1:8333" }, []req{{"http", "192.0.2.1", 8333}},
			[]req{{"http", "192.0.2.2", 8333}}},
		{"ipv6", func(c *s3.Config) { c.Endpoint = "https://[2001:db8::1]:9000" }, []req{{"https", "2001:db8::1", 9000}}, nil},
	}
	for _, c := range cases {
		cfg := validConfig()
		c.mutate(&cfg)
		p := cfg.HostPolicy()
		for _, r := range c.allowed {
			if !p(context.Background(), r.scheme, r.host, r.port) {
				t.Errorf("%s: %+v refused", c.name, r)
			}
		}
		for _, r := range c.refused {
			if p(context.Background(), r.scheme, r.host, r.port) {
				t.Errorf("%s: %+v allowed", c.name, r)
			}
		}
	}
	if p := (s3.Config{Endpoint: "::not a url"}).HostPolicy(); p(context.Background(), "https", "", 443) {
		t.Error("the policy of an invalid endpoint allows something")
	}
}

// coreOperations are the eight SDK operations of the narrow core interface (12 "4.1",
// ADR 0010): no listing, copying, batch deletion, bucket calls or versioning.
var coreOperations = []string{
	"AbortMultipartUpload", "CompleteMultipartUpload", "CreateMultipartUpload", "DeleteObject",
	"GetObject", "HeadObject", "PutObject", "UploadPart",
}

func methodNames(t reflect.Type) []string {
	var names []string
	for i := range t.NumMethod() {
		names = append(names, t.Method(i).Name)
	}
	slices.Sort(names)
	return names
}

// TestCoreAPIMethodSet holds 12 "15" (the S3 driver depends only on an interface with the port
// operations) and ADR 0048: exactly the eight core operations, which *s3.Client satisfies.
func TestCoreAPIMethodSet(t *testing.T) {
	t.Parallel()
	if got := methodNames(s3.CoreAPI()); !slices.Equal(got, coreOperations) {
		t.Errorf("coreAPI methods = %v, want %v", got, coreOperations)
	}
	if !reflect.TypeFor[*awss3.Client]().Implements(s3.CoreAPI()) {
		t.Error("*s3.Client does not implement coreAPI")
	}
}

// TestPresignAPIMethodSet: the presign interface has exactly PresignUploadPart; no presigned GET,
// HEAD, DELETE or whole-object PUT (R-FI-03, ADR 0048).
func TestPresignAPIMethodSet(t *testing.T) {
	t.Parallel()
	if got := methodNames(s3.PresignAPI()); !slices.Equal(got, []string{"PresignUploadPart"}) {
		t.Errorf("presignAPI methods = %v", got)
	}
	if !reflect.TypeFor[*awss3.PresignClient]().Implements(s3.PresignAPI()) {
		t.Error("*s3.PresignClient does not implement presignAPI")
	}
}
