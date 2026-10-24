// SPDX-License-Identifier: Apache-2.0

package wire_test

import (
	"reflect"
	"testing"
	"time"

	filesjobs "github.com/OWNER/bothy/services/server/internal/files/adapters/jobs"
	"github.com/OWNER/bothy/services/server/internal/platform/jobs"
	"github.com/OWNER/bothy/services/server/internal/wire"
)

// TestTranscribeJobRegistration: every role registers files.transcribe on the queue files with an
// argument type of IDs only (M3-09).
func TestTranscribeJobRegistration(t *testing.T) {
	t.Parallel()
	reg, err := wire.NewJobRegistry(&wire.JobModules{TerminalSweepInterval: time.Minute, FilesGCInterval: time.Minute, FilesExtractInterval: time.Hour,
		PartitionInterval: time.Minute, CleanupInterval: time.Hour, ExpiryInterval: time.Minute, KeyRotationInterval: time.Minute,
		AuthCleanupInterval: time.Hour, ChainVerifyInterval: time.Hour, FilesImageTimeout: time.Minute, StallInterval: time.Minute,
		FilesTranscribeAttempts: 5})
	if err != nil {
		t.Fatal(err)
	}
	if q, ok := reg.Queue(filesjobs.KindTranscribe); !ok || q != jobs.QueueFiles {
		t.Fatalf("queue = %q, %v", q, ok)
	}
	if typ := reg.ArgTypes()[filesjobs.KindTranscribe]; typ != reflect.TypeFor[filesjobs.TranscribeArgs]() {
		t.Fatalf("argument type %v", typ)
	}
}
