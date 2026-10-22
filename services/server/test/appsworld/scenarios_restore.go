// SPDX-License-Identifier: Apache-2.0

package appsworld

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	auditapp "github.com/OWNER/bothy/services/server/internal/audit/app"
	"github.com/OWNER/bothy/services/server/internal/instance"
	instancepg "github.com/OWNER/bothy/services/server/internal/instance/adapters/postgres"
	instanceapp "github.com/OWNER/bothy/services/server/internal/instance/app"
	"github.com/OWNER/bothy/services/server/internal/platform/pg"
	"github.com/OWNER/bothy/services/server/internal/platform/testkit"
	"github.com/OWNER/bothy/services/server/internal/wire"
)

// worldSessions is the instance repository of the world's finalize with no other bothy_app
// session: the world keeps its bothy_app pool open for the worker it runs, so the refusal of a
// connected server (tested in internal/wire and cmd/bothy) would stop the scenario.
type worldSessions struct{ *instancepg.Repository }

// OtherSessionsOfRole reports no other session.
func (worldSessions) OtherSessionsOfRole(context.Context, string) (int64, error) { return 0, nil }

// noAuditHead is the audit head of a finalize without a relocation; it is never read.
type noAuditHead struct{}

func (noAuditHead) AuditHead(context.Context) (int64, string, error) { return 0, "", nil }

// FinalizeRestore runs bothy restore finalize on the world's database as the schema owner with
// the cursor ports of internal/wire, and returns its result.
func (w *World) FinalizeRestore() instance.FinalizeResult {
	w.t.Helper()
	a, err := wire.NewAudit(w.owner, pg.UnitConfig{StatementTimeout: 30 * time.Second, LockTimeout: 20 * time.Second, IdleInTxTimeout: time.Minute},
		auditapp.Config{DenyAggregateWindow: time.Hour, DenyAggregateMaxKeys: 100, IPKeyPeriod: 30 * 24 * time.Hour,
			IPKeyRetention: 90 * 24 * time.Hour, IPKEKRef: "file:<secrets.dir>/audit_ip_kek"}, testKEK{}, nil)
	Must(w.t, err)
	rs, err := instanceapp.NewRestore(instanceapp.RestoreDeps{
		Repo: worldSessions{instancepg.NewRepository(w.owner)}, UoW: a.UoW(), Audit: a.Recorder(),
		Cursors: wire.RestoreCursors(w.owner), AuditHead: noAuditHead{}, ServerRole: string(pg.RoleApp),
	})
	Must(w.t, err)
	res, err := rs.Finalize(context.Background(), instance.FinalizeRestore{})
	Must(w.t, err)
	return res
}

// ScenarioWebhookCarriesEpochAfterRestore (M7-04, ADR 0059 item 4, 04 2.2): a delivery before
// bothy restore finalize carries the old sync_epoch; finalize sets the subscription cursor to the
// new head, and the first delivery after it carries the new epoch.
func ScenarioWebhookCarriesEpochAfterRestore(t *testing.T) {
	t.Parallel()
	w := NewWorld(t)
	_, _, created := subscribed(t, w, false)
	sub := SubscriptionID(created)
	w.StartWorker()
	esk(w, "Before the restore")
	w.Round()
	waitFor(t, testkit.Patience, "the first delivery", func() bool {
		d := w.Deliveries(sub)
		return len(d) == 1 && d[0].Status == "succeeded"
	})
	epochOf := func(i int) string {
		got := w.Hooks().Got()
		if len(got) <= i {
			t.Fatalf("%d requests, want %d", len(got), i+1)
		}
		var env map[string]any
		Must(t, json.Unmarshal(got[i].Body, &env))
		s, _ := env["sync_epoch"].(string)
		return s
	}
	old := epochOf(0)

	res := w.FinalizeRestore()
	if res.PreviousSyncEpoch.String() != old || res.SyncEpoch.String() == old {
		t.Fatalf("finalize %+v after the epoch %s of the first delivery", res, old)
	}
	if last := w.SubscriptionRow(sub).LastSeq; last != res.HeadSeq {
		t.Fatalf("subscription cursor %d after finalize, want head_seq %d", last, res.HeadSeq)
	}

	msg := esk(w, "After the restore")
	w.Round()
	waitFor(t, testkit.Patience, "the delivery after the restore", func() bool {
		d := w.Deliveries(sub)
		return len(d) == 2 && d[0].Status == "succeeded" && d[1].Status == "succeeded"
	})
	if got := epochOf(1); got != res.SyncEpoch.String() {
		t.Fatalf("delivery of seq %d carries sync_epoch %q, want the new epoch %s", msg.Seq, got, res.SyncEpoch)
	}
}
