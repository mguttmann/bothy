// SPDX-License-Identifier: Apache-2.0

package wire_test

import (
	"reflect"
	"testing"

	filesjobs "github.com/OWNER/bothy/services/server/internal/files/adapters/jobs"
	"github.com/OWNER/bothy/services/server/internal/platform/jobs"
	"github.com/OWNER/bothy/services/server/internal/wire"
)

// TestTranscribeJobRegistration: every role registers files.transcribe on the queue files with an
// argument type of IDs only (M3-09).
func TestTranscribeJobRegistration(t *testing.T) {
	t.Parallel()
	m := everyIntervalSet()
	m.FilesTranscribeAttempts = 5
	reg, err := wire.NewJobRegistry(m)
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
