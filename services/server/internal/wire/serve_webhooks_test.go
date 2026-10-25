// SPDX-License-Identifier: Apache-2.0

package wire_test

import (
	"net/http"
	"reflect"
	"syscall"
	"testing"
	"time"

	appsjobs "github.com/OWNER/bothy/services/server/internal/apps/adapters/jobs"
	"github.com/OWNER/bothy/services/server/internal/platform/jobs"
	"github.com/OWNER/bothy/services/server/internal/platform/lifecycle"
	"github.com/OWNER/bothy/services/server/internal/wire"
)

// TestWebhookConsumerRegisteredInWorkerRoles (M2-26): without injected handlers, worker and all
// register the production consumer webhooks, whose cursor row starts at the head; api registers
// none.
func TestWebhookConsumerRegisteredInWorkerRoles(t *testing.T) {
	t.Parallel()
	for _, role := range []wire.Role{wire.RoleAPI, wire.RoleWorker, wire.RoleAll} {
		t.Run(string(role), func(t *testing.T) {
			t.Parallel()
			db := newServeDB(t)
			p := start(t, role, serveEnv(t, db), wire.Deps{})
			waitReadyz(t, p.waitInternal(t), http.StatusOK)
			want := 1
			if role == wire.RoleAPI {
				time.Sleep(300 * time.Millisecond)
				want = 0
			}
			if n := countRows(t, db, "SELECT count(*) FROM event_consumer_cursors WHERE name = 'webhooks'"); n != want {
				t.Fatalf("cursor rows of webhooks = %d, want %d; logs:\n%s", n, want, p.logs)
			}
			p.sigs <- syscall.SIGTERM
			if code := p.exit(t); code != lifecycle.ExitOK {
				t.Fatalf("exit = %d; logs:\n%s", code, p.logs)
			}
		})
	}
}

// TestWebhookJobRegistration (M2-26): the registry of every role holds apps.webhook_deliver on the
// queue delivery, with arguments of IDs only.
func TestWebhookJobRegistration(t *testing.T) {
	t.Parallel()
	reg, err := wire.NewJobRegistry(&wire.JobModules{RetentionInterval: time.Hour, TerminalSweepInterval: time.Minute, FilesGCInterval: time.Minute, FilesExtractInterval: time.Hour, PartitionInterval: time.Minute, CleanupInterval: time.Hour, ExpiryInterval: time.Minute,
		KeyRotationInterval: time.Minute, AuthCleanupInterval: time.Hour, ChainVerifyInterval: time.Hour, StallInterval: time.Minute})
	if err != nil {
		t.Fatal(err)
	}
	if q, ok := reg.Queue(appsjobs.KindWebhookDeliver); !ok || q != jobs.QueueDelivery {
		t.Fatalf("queue = %q, %v", q, ok)
	}
	typ, ok := reg.ArgTypes()[appsjobs.KindWebhookDeliver]
	if !ok || typ != reflect.TypeFor[appsjobs.WebhookDeliver]() {
		t.Fatalf("argument type %v", typ)
	}
	if err := jobs.ValidateArgsType(typ); err != nil {
		t.Fatal(err)
	}
}
