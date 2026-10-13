// SPDX-License-Identifier: Apache-2.0

package domain_test

import (
	"strings"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/audit/domain"
)

// TestParseSink: audit.checkpoint_sink is empty or file://<absolute clean path>; every other form
// fails with a message naming the supported scheme (19 section 16).
func TestParseSink(t *testing.T) {
	t.Parallel()
	if p, err := domain.ParseSink(""); err != nil || p != "" {
		t.Errorf("empty: %q, %v", p, err)
	}
	if p, err := domain.ParseSink("file:///var/lib/bothy-checkpoints"); err != nil || p != "/var/lib/bothy-checkpoints" {
		t.Errorf("file: %q, %v", p, err)
	}
	for _, bad := range []string{
		"s3://bucket/prefix", "/var/lib/x", "file://relative/x", "file:relative", "file:///var/lib/../x",
		"file:///var/lib/x/", "file:///var/lib/x?q=1", "file:///var/lib/x#f", "file://user@/x", "file://",
		"file:/var/lib/x", "FILE:///var/lib/x", "file:///var/lib/x%00y", "file:///var/lib/x%0Ay", "file:///var/lib/%78",
	} {
		_, err := domain.ParseSink(bad)
		if err == nil || !strings.Contains(err.Error(), "file://") {
			t.Errorf("%s: %v, want an error naming file://", bad, err)
		}
	}
}
