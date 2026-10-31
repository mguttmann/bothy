// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/files/app"
	"github.com/OWNER/bothy/services/server/internal/kernel/eventlog"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/sync"
)

// scanEventGolden is the file.updated the scan pass writes for a clean PNG, as the gateway
// delivers it: before the file is attached (to the uploader) and once it is attached (to the
// thread of its message). The client's sync engine applies exactly these envelopes in its test
// (packages/sync_engine/test/catch_up/scan_event_test.dart), so a change of the server's shape
// fails there, not in a live app (FIX-scan-state-stale).
const scanEventGolden = "../../../../../packages/sync_engine/test/fixtures/file_updated_after_scan.json"

// updateScanGolden rewrites the golden file of TestScanEventGolden.
var updateScanGolden = flag.Bool("update-scan-golden", false, "rewrite the golden file of the scan event")

// The fixed IDs of the golden file; they are the IDs of the Dart test support (id(n) of
// packages/sync_engine/test/catch_up/support.dart).
const (
	goldenFile     = "0192f4a0-7b1c-78f9-8074-000000007001"
	goldenUploader = "0192f4a0-7b1c-78f9-8074-0000000000a1"
	goldenSpace    = "0192f4a0-7b1c-78f9-8074-000000000101"
	goldenChannel  = "0192f4a0-7b1c-78f9-8074-000000000201"
	goldenThread   = "0192f4a0-7b1c-78f9-8074-000000000301"
)

// TestScanEventGolden holds the golden file of the client test: the envelope of the event the
// real scan pass appends for a clean PNG, unattached and attached. Run with
// -update-scan-golden to write the file after a deliberate change of the shape.
func TestScanEventGolden(t *testing.T) {
	t.Parallel()
	content := []byte("bytes of a screenshot of Alex")
	var envelopes []json.RawMessage
	for i, attached := range []bool{false, true} {
		e := newScanEnvTypes(t, scanCfg, fixedType("image/png"))
		s := e.seed(t, content, content, true)
		f := e.repo.files[s.FileID]
		f.OriginalName = "pasted.png"
		e.repo.files[s.FileID] = f
		s.File = f
		e.repo.sessions[s.FileID] = s
		replace := map[string]string{s.FileID.String(): goldenFile, f.UploadedBy.String(): goldenUploader, f.ScopeChannelID.String(): goldenChannel}
		if attached {
			channel, thread := ids.New(), ids.New()
			e.repo.attachments[s.FileID] = app.AttachmentRecord{MessageID: ids.New(), FileID: s.FileID, ThreadID: thread, ChannelID: channel, Position: 1}
			replace[channel.String()], replace[thread.String()] = goldenChannel, goldenThread
		}
		if _, err := e.pass.Run(context.Background(), s.FileID, 0); err != nil {
			t.Fatal(err)
		}
		if len(e.events.list) != 1 {
			t.Fatalf("attached %v: %d events, want one file.updated", attached, len(e.events.list))
		}
		ev := e.events.list[0]
		replace[ev.SpaceID.String()] = goldenSpace
		envelopes = append(envelopes, deliveredEnvelope(t, ev, int64(9001+i), replace))
	}
	got, err := json.MarshalIndent(envelopes, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	got = append(got, '\n')
	path := filepath.FromSlash(scanEventGolden)
	if *updateScanGolden {
		if err := os.WriteFile(path, got, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(path) // #nosec G304 -- a golden file of the repository
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("the scan event changed its shape; update %s with -update-scan-golden and run the sync engine test\ngot:\n%s", path, got)
	}
}

// deliveredEnvelope is the envelope of ev as the gateway delivers it (sync.Event.Envelope) with
// seq, a fixed event ID and time, and the IDs of replace swapped for the golden ones.
func deliveredEnvelope(t *testing.T, ev eventlog.Event, seq int64, replace map[string]string) json.RawMessage {
	t.Helper()
	stored := sync.Event{
		Seq: seq, ID: ids.MustParse(fmt.Sprintf("0192f4a0-7b1c-78f9-8074-%012x", 0x900000+seq)), Type: ev.Type, SchemaVersion: ev.SchemaVersion,
		OccurredAt: time.Date(2026, 10, 31, 10, 0, 1, 123456789, time.UTC), ActorKind: string(ev.ActorKind), ActorPrincipalID: ev.ActorPrincipalID,
		SpaceID: ev.SpaceID, ChannelID: ev.ChannelID, ThreadID: ev.ThreadID, ResourceKind: ev.ResourceKind, ResourceID: ev.ResourceID,
		ResourceVersion: ev.ResourceVersion, AudiencePrincipalID: ev.AudiencePrincipalID, ReceiveAction: ev.ReceiveAction, Payload: ev.Payload,
		Truncated: ev.Truncated, ClientMsgID: ev.ClientMsgID, PermissionVersion: 3,
	}
	raw, err := json.Marshal(stored.Envelope(sync.DeliveryFields{}))
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	for from, to := range replace {
		text = strings.ReplaceAll(text, from, to)
	}
	return json.RawMessage(text)
}
