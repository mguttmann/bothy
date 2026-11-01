// SPDX-License-Identifier: Apache-2.0

package wire_test

import (
	"reflect"
	"strings"
	"testing"
	"time"

	filesjobs "github.com/OWNER/bothy/services/server/internal/files/adapters/jobs"
	"github.com/OWNER/bothy/services/server/internal/platform/jobs"
	"github.com/OWNER/bothy/services/server/internal/wire"
)

// TestConfigScanStreamLimit (12 15 "Startup check", ADR 0033): with file.scan.mode = required,
// file.max_upload_bytes above file.scan.clamd_stream_max_bytes is a problem that names both keys
// and no value; equal values and mode off pass.
func TestConfigScanStreamLimit(t *testing.T) {
	t.Parallel()
	got := load(t, "BOTHY_FILE__MAX_UPLOAD_BYTES=104857601")
	if len(got) != 1 || !strings.Contains(got[0].String(), ": scan_stream_limit: ") ||
		!strings.Contains(got[0].String(), "file.max_upload_bytes") || !strings.Contains(got[0].String(), "file.scan.clamd_stream_max_bytes") {
		t.Fatalf("problems %v, want scan_stream_limit", got)
	}
	noValues(t, got, "104857601")
	noValues(t, got, "104857600")
	for name, env := range map[string][]string{
		"equal":    {"BOTHY_FILE__MAX_UPLOAD_BYTES=2000000", "BOTHY_FILE__SCAN__CLAMD_STREAM_MAX_BYTES=2000000"},
		"mode off": {"BOTHY_FILE__MAX_UPLOAD_BYTES=104857601", "BOTHY_FILE__SCAN__MODE=off"},
	} {
		if got := load(t, env...); got != nil {
			t.Errorf("%s: problems %v, want none", name, got)
		}
	}
}

// TestConfigClamdAddressUnixOnly (19 SEC-TLS-5): only unix:// with an absolute path passes.
func TestConfigClamdAddressUnixOnly(t *testing.T) {
	t.Parallel()
	for _, addr := range []string{"tcp://clamav:3310", "http://clamav.example.com:3310", "unix://relative/path"} {
		got := load(t, "BOTHY_FILE__SCAN__CLAMD_ADDRESS="+addr)
		if len(got) != 1 || !strings.Contains(got[0].String(), ": clamd_address_unix: ") {
			t.Errorf("%s: problems %v, want clamd_address_unix", addr, got)
		}
		noValues(t, got, strings.TrimPrefix(strings.TrimPrefix(addr, "unix://"), "tcp://"))
	}
	if got := load(t, "BOTHY_FILE__SCAN__CLAMD_ADDRESS=unix:///run/clamav/clamd.sock"); got != nil {
		t.Errorf("default socket: problems %v", got)
	}
}

// TestScanJobRegistration: every role registers files.scan on the queue files with arguments of
// IDs and an int64 only.
func TestScanJobRegistration(t *testing.T) {
	t.Parallel()
	reg, err := wire.NewJobRegistry(&wire.JobModules{RetentionInterval: time.Hour, StatusProbeInterval: time.Minute, TerminalSweepInterval: time.Minute, FilesGCInterval: time.Minute, MessagingPurgeInterval: time.Minute, FilesExtractInterval: time.Hour, PartitionInterval: time.Minute, CleanupInterval: time.Hour, ExpiryInterval: time.Minute,
		KeyRotationInterval: time.Minute, AuthCleanupInterval: time.Hour, ChainVerifyInterval: time.Hour, StallInterval: time.Minute})
	if err != nil {
		t.Fatal(err)
	}
	if q, ok := reg.Queue("files.scan"); !ok || q != jobs.QueueFiles {
		t.Fatalf("queue = %q, %v", q, ok)
	}
	if typ := reg.ArgTypes()["files.scan"]; typ != reflect.TypeFor[filesjobs.Scan]() {
		t.Fatalf("argument type %v", typ)
	}
}

// TestScanConfigsBind: the keys of the pass and of the client bind with their chapter defaults.
func TestScanConfigsBind(t *testing.T) {
	t.Parallel()
	pass, client, err := wire.ScanConfigs(filesValues(t))
	if err != nil {
		t.Fatal(err)
	}
	if pass.Mode != "required" || pass.Timeout != 120*time.Second || pass.HealthInterval != 30*time.Second ||
		pass.RetryBackoffCap != 5*time.Minute || pass.SniffBytes != 1048576 {
		t.Errorf("pass %+v", pass)
	}
	if client.Address != "unix:///run/clamav/clamd.sock" || client.ChunkBytes != 65536 || client.MaxConcurrent != 4 || client.Timeout != 120*time.Second {
		t.Errorf("client %+v", client)
	}
}
