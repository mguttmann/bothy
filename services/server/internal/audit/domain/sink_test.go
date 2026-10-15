// SPDX-License-Identifier: Apache-2.0

package domain_test

import (
	"strings"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/audit/domain"
)

// TestParseSink: audit.checkpoint_sink is empty, file://<absolute clean path> or
// s3://<bucket>[/<prefix>]; every other form fails with a message naming the supported schemes
// (19 section 16).
func TestParseSink(t *testing.T) {
	t.Parallel()
	if s, err := domain.ParseSink(""); err != nil || s != (domain.Sink{}) {
		t.Errorf("empty: %+v, %v", s, err)
	}
	if s, err := domain.ParseSink("file:///var/lib/bothy-checkpoints"); err != nil ||
		s != (domain.Sink{Kind: domain.SinkFile, Dir: "/var/lib/bothy-checkpoints"}) {
		t.Errorf("file: %+v, %v", s, err)
	}
	for _, bad := range []string{
		"/var/lib/x", "file://relative/x", "file:relative", "file:///var/lib/../x",
		"file:///var/lib/x/", "file:///var/lib/x?q=1", "file:///var/lib/x#f", "file://user@/x", "file://",
		"file:/var/lib/x", "FILE:///var/lib/x", "file:///var/lib/x%00y", "file:///var/lib/x%0Ay", "file:///var/lib/%78",
		"gs://bothy-audit", "https://bothy-audit",
	} {
		_, err := domain.ParseSink(bad)
		if err == nil || !strings.Contains(err.Error(), "file://") || !strings.Contains(err.Error(), "s3://") {
			t.Errorf("%s: %v, want an error naming file:// and s3://", bad, err)
		}
	}
}

// TestParseSinkS3: s3://<bucket>[/<prefix>] names a bucket that follows the S3 naming rules and
// an optional prefix of segments of [a-z0-9-] joined by single slashes (M7-22).
func TestParseSinkS3(t *testing.T) {
	t.Parallel()
	for in, want := range map[string]domain.Sink{
		"s3://bothy-audit":                   {Kind: domain.SinkS3, Bucket: "bothy-audit"},
		"s3://audit-checkpoints.example.com": {Kind: domain.SinkS3, Bucket: "audit-checkpoints.example.com"},
		"s3://bothy-audit/prod":              {Kind: domain.SinkS3, Bucket: "bothy-audit", Prefix: "prod"},
		"s3://bothy-audit/env/prod-1":        {Kind: domain.SinkS3, Bucket: "bothy-audit", Prefix: "env/prod-1"},
	} {
		got, err := domain.ParseSink(in)
		if err != nil || got != want {
			t.Errorf("%s: %+v, %v, want %+v", in, got, err, want)
		}
	}
	for _, bad := range []string{
		"s3://", "s3:///prefix", "S3://bothy-audit", "s3:bothy-audit", "s3://Audit-Store", "s3://ab",
		"s3://bothy_audit", "s3://-audit", "s3://bothy..audit", "s3://192.0.2.1", "s3://xn--audit",
		"s3://bothy-audit/", "s3://bothy-audit//x", "s3://bothy-audit/x/", "s3://bothy-audit/../x",
		"s3://bothy-audit/Prod", "s3://bothy-audit/a_b", "s3://bothy-audit/a.b", "s3://bothy-audit?x=1",
		"s3://bothy-audit#f", "s3://user@bothy-audit", "s3://bothy-audit:9000", "s3://bothy-audit/%78",
		"s3://bothy-audit/x\n", "s3://" + strings.Repeat("a", 64),
	} {
		_, err := domain.ParseSink(bad)
		if err == nil || !strings.Contains(err.Error(), "s3://") {
			t.Errorf("%q: %v, want an error naming s3://", bad, err)
		}
	}
}
