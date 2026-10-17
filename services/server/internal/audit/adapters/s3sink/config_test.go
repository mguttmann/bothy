// SPDX-License-Identifier: Apache-2.0

package s3sink_test

import (
	"strings"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/audit/adapters/s3sink"
	"github.com/OWNER/bothy/services/server/internal/platform/egress"
)

// TestConfigValidate: the endpoint, region and path style of the object store target follow the
// rules of the files S3 driver, with the keys audit.checkpoint_s3_* in every message and never a
// value (M7-22, docs/DECISIONS.md M7-22/4).
func TestConfigValidate(t *testing.T) {
	t.Parallel()
	good := s3sink.Config{Bucket: "bothy-audit", Endpoint: "https://s3.example.com", Region: "eu-central-1", UsePathStyle: true}
	if err := good.Validate(nil); err != nil {
		t.Fatalf("good: %v", err)
	}
	vh := good
	vh.UsePathStyle = false
	if err := vh.Validate(nil); err != nil {
		t.Fatalf("virtual-hosted: %v", err)
	}
	private, err := egress.ParseRule("127.0.0.1:9000;allow_private")
	if err != nil {
		t.Fatal(err)
	}
	local := good
	local.Endpoint = "http://127.0.0.1:9000"
	if err := local.Validate([]egress.Rule{private}); err != nil {
		t.Fatalf("http with allow_private: %v", err)
	}
	for _, c := range []struct {
		name string
		mod  func(*s3sink.Config)
		key  string
	}{
		{"no endpoint", func(c *s3sink.Config) { c.Endpoint = "" }, "audit.checkpoint_s3_endpoint"},
		{"endpoint with path", func(c *s3sink.Config) { c.Endpoint = "https://s3.example.com/x" }, "audit.checkpoint_s3_endpoint"},
		{"endpoint with user", func(c *s3sink.Config) { c.Endpoint = "https://alex@s3.example.com" }, "audit.checkpoint_s3_endpoint"},
		{"endpoint with query", func(c *s3sink.Config) { c.Endpoint = "https://s3.example.com?x=1" }, "audit.checkpoint_s3_endpoint"},
		{"endpoint ftp", func(c *s3sink.Config) { c.Endpoint = "ftp://s3.example.com" }, "audit.checkpoint_s3_endpoint"},
		{"endpoint port 0", func(c *s3sink.Config) { c.Endpoint = "https://s3.example.com:0" }, "audit.checkpoint_s3_endpoint"},
		{"no region", func(c *s3sink.Config) { c.Region = "" }, "audit.checkpoint_s3_region"},
		{"bad region", func(c *s3sink.Config) { c.Region = "EU Central" }, "audit.checkpoint_s3_region"},
		{"ip without path style", func(c *s3sink.Config) { c.Endpoint, c.UsePathStyle = "https://192.0.2.1", false }, "audit.checkpoint_s3_use_path_style"},
		{"http without allow_private", func(c *s3sink.Config) { c.Endpoint = "http://s3.example.com" }, "egress.allowed_hosts"},
		{"bad bucket", func(c *s3sink.Config) { c.Bucket = "Audit-Store" }, "audit.checkpoint_sink"},
	} {
		cfg := good
		c.mod(&cfg)
		err := cfg.Validate(nil)
		if err == nil || !strings.Contains(err.Error(), c.key) {
			t.Errorf("%s: %v, want an error naming %s", c.name, err, c.key)
			continue
		}
		if cfg.Endpoint != "" && strings.Contains(err.Error(), cfg.Endpoint) {
			t.Errorf("%s: the error names the value: %v", c.name, err)
		}
	}
}

// TestHostPolicy: the egress client of the sink may reach exactly the request host and port.
func TestHostPolicy(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	path := s3sink.Config{Bucket: "bothy-audit", Endpoint: "https://s3.example.com", Region: "us-east-1", UsePathStyle: true}.HostPolicy()
	if !path(ctx, "https", "s3.example.com", 443) || path(ctx, "https", "bothy-audit.s3.example.com", 443) ||
		path(ctx, "http", "s3.example.com", 443) || path(ctx, "https", "s3.example.com", 8443) {
		t.Error("path style policy")
	}
	vh := s3sink.Config{Bucket: "bothy-audit", Endpoint: "https://S3.Example.com:8443/", Region: "us-east-1"}.HostPolicy()
	if !vh(ctx, "https", "bothy-audit.s3.example.com", 8443) || vh(ctx, "https", "s3.example.com", 8443) {
		t.Error("virtual-hosted policy")
	}
	if (s3sink.Config{Endpoint: "::"}).HostPolicy()(ctx, "https", "s3.example.com", 443) {
		t.Error("a refused endpoint allows a host")
	}
}
