// SPDX-License-Identifier: Apache-2.0

//go:build unix

package importtool

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/cmd/bothy/importtool/discord/discordtest"
	"github.com/OWNER/bothy/services/server/internal/platform/testkit"
)

// TestFollowStopsOnSignal (22 7.4 and 12.2; docs/operations/discord-migration.md section 7 "One
// writing run at a time"): a real SIGINT or SIGTERM to the process of a --follow mirror ends it
// with exit 0 after the current round and frees the lease, so the full import or switch that the
// owner runs next can take it. The binary passes a background context, so only the tool's own
// registration turns the signal into a stop. Not parallel: the signal reaches the whole test
// process; the test's own registration keeps the process alive when the tool registers none.
func TestFollowStopsOnSignal(t *testing.T) {
	for _, sig := range []syscall.Signal{syscall.SIGINT, syscall.SIGTERM} {
		t.Run(sig.String(), func(t *testing.T) {
			guard := make(chan os.Signal, 1)
			signal.Notify(guard, sig)
			defer signal.Stop(guard)
			e := newTestEnv(t, discordtest.BasicFixture(t))
			e.follow = func(ctx context.Context, _ time.Duration) error {
				if err := syscall.Kill(os.Getpid(), sig); err != nil {
					t.Fatalf("kill: %v", err)
				}
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-time.After(testkit.Patience):
					t.Fatalf("%s did not stop the --follow mirror", sig)
					return nil
				}
			}
			src := e.startSource(fixtureMapping)
			if code := e.run("--guild", fxGuild, "--map", e.writeMap(fixtureMapping), "--apply", "--follow"); code != ExitOK {
				t.Fatalf("exit %d after %s, want 0: %s", code, sig, e.stderr.String())
			}
			if _, held := e.bothy.SourceCursor(src)["run"]; held {
				t.Errorf("the lease is still held after %s", sig)
			}
		})
	}
}
