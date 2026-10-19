// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

// TestServeRefusesInconsistentScanLimits holds 12 15 "Startup check" and ADR 0033: with
// file.scan.mode = required and file.max_upload_bytes above file.scan.clamd_stream_max_bytes,
// bothy config check reports the problem (exit 1, the code of a configuration problem in this
// command) and bothy serve --role=worker ends with the configuration error code 2 before any
// listener; both name the keys and no value. Equal limits and mode off pass the check.
func TestServeRefusesInconsistentScanLimits(t *testing.T) {
	t.Parallel()
	env := withPublicURL([]string{"BOTHY_FILE__MAX_UPLOAD_BYTES=104857601"})
	var stdout, stderr bytes.Buffer
	if code := run(context.Background(), []string{"config", "check"}, env, &stdout, &stderr); code != 1 {
		t.Fatalf("config check: exit %d, stdout %q, stderr %q", code, stdout.String(), stderr.String())
	}
	if msg := stderr.String(); !strings.Contains(msg, "scan_stream_limit") || !strings.Contains(msg, "file.scan.clamd_stream_max_bytes") ||
		strings.Contains(msg, "104857601") {
		t.Errorf("config check: stderr %q", msg)
	}
	stdout.Reset()
	stderr.Reset()
	if code := run(context.Background(), []string{"serve", "--role=worker"}, env, &stdout, &stderr); code != 2 {
		t.Fatalf("serve: exit %d, want 2; stderr %q", code, stderr.String())
	}
	if out := stdout.String() + stderr.String(); !strings.Contains(out, "scan_stream_limit") || strings.Contains(out, "listener started") ||
		strings.Contains(out, "104857601") {
		t.Errorf("serve: output %q", out)
	}
	for name, extra := range map[string][]string{
		"equal":    {"BOTHY_FILE__MAX_UPLOAD_BYTES=104857600"},
		"mode off": {"BOTHY_FILE__MAX_UPLOAD_BYTES=104857601", "BOTHY_FILE__SCAN__MODE=off"},
	} {
		var out, errw bytes.Buffer
		if code := run(context.Background(), []string{"config", "check"}, withPublicURL(extra), &out, &errw); code != 0 {
			t.Errorf("%s: exit %d, stderr %q", name, code, errw.String())
		}
	}
}
