// SPDX-License-Identifier: Apache-2.0

package wire_test

import (
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/platform/jobs"
	terminaljobs "github.com/OWNER/bothy/services/server/internal/terminal/adapters/jobs"
	"github.com/OWNER/bothy/services/server/internal/wire"
)

// TestWire_TerminalSubscriptionsAndSweep (M2-46): the golden registrations list the three
// subscriptions of terminal after the existing ones, and the job registry of every role holds
// terminal.sweep on queue maintenance, periodic every terminal.sweep_interval and run once at the
// start (the runtime runs periodic jobs only in the roles worker and all, M1-14).
func TestWire_TerminalSubscriptionsAndSweep(t *testing.T) {
	t.Parallel()
	golden, err := os.ReadFile("testdata/txn_registrations.golden")
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(golden)), "\n")
	// The terminal subscriptions follow the runs subscribers of M2-09 and M8-03
	// (chat.ChannelReadOnlySet -> runs); later modules (M2-11) may register after them.
	first := slices.IndexFunc(lines, func(l string) bool { return strings.Contains(l, "internal/terminal/app.(*Subscribers).") })
	if first < 1 || first+3 > len(lines) || !strings.HasPrefix(lines[first-1], "subscribe chat.ChannelReadOnlySet -> runs ") ||
		!slices.ContainsFunc(lines[:first], func(l string) bool { return strings.HasPrefix(l, "subscribe agents.AgentDisabled -> runs ") }) {
		t.Fatalf("terminal subscriptions at %d, not after the runs subscribers of M2-09", first)
	}
	tail := lines[first : first+3]
	for i, want := range []string{"subscribe kernel.AccessChanged -> terminal ", "subscribe identity.DeviceRevoked -> terminal ",
		"subscribe identity.SessionEnded -> terminal "} {
		if !strings.HasPrefix(tail[i], want) || !strings.Contains(tail[i], "internal/terminal/app.(*Subscribers).") {
			t.Fatalf("registration %d after the existing ones = %q, want %q...", i, tail[i], want)
		}
	}
	reg, err := wire.NewJobRegistry(&wire.JobModules{RetentionInterval: time.Hour, TerminalSweepInterval: 45 * time.Second, FilesGCInterval: time.Minute, FilesExtractInterval: time.Hour, PartitionInterval: time.Minute,
		CleanupInterval: time.Hour, ExpiryInterval: time.Minute, KeyRotationInterval: time.Minute, AuthCleanupInterval: time.Hour,
		ChainVerifyInterval: time.Hour, StallInterval: time.Minute})
	if err != nil {
		t.Fatal(err)
	}
	if q, ok := reg.Queue(terminaljobs.KindSweep); !ok || q != jobs.QueueMaintenance {
		t.Fatalf("queue of %s = %q, %v", terminaljobs.KindSweep, q, ok)
	}
	found := false
	for _, p := range reg.Periodic() {
		if p.Args.Kind() == terminaljobs.KindSweep {
			found = p.Interval == 45*time.Second && p.RunOnStart
		}
	}
	if !found {
		t.Fatalf("%s not periodic every terminal.sweep_interval with RunOnStart: %+v", terminaljobs.KindSweep, reg.Periodic())
	}
	if _, err := wire.NewJobRegistry(&wire.JobModules{RetentionInterval: time.Hour, FilesGCInterval: time.Minute, FilesExtractInterval: time.Hour, PartitionInterval: time.Minute, CleanupInterval: time.Hour, ExpiryInterval: time.Minute,
		KeyRotationInterval: time.Minute, AuthCleanupInterval: time.Hour, ChainVerifyInterval: time.Hour, StallInterval: time.Minute}); err == nil {
		t.Fatal("a registry without terminal.sweep_interval was built")
	}
}
