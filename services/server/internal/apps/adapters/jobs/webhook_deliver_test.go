// SPDX-License-Identifier: Apache-2.0

package jobs_test

import (
	"context"
	"reflect"
	"testing"
	"time"

	appsjobs "github.com/OWNER/bothy/services/server/internal/apps/adapters/jobs"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/jobs"
)

// deliverer records the subscriptions it was asked for.
type deliverer struct{ got []ids.ID }

func (d *deliverer) Deliver(_ context.Context, id ids.ID) error {
	d.got = append(d.got, id)
	return nil
}

// TestWebhookDeliverJob: the kind apps.webhook_deliver on the queue delivery with arguments of IDs
// only; the handler runs the attempt of its subscription; an unbound scheduler refuses.
func TestWebhookDeliverJob(t *testing.T) {
	t.Parallel()
	reg := jobs.NewRegistry()
	d := &deliverer{}
	if err := appsjobs.Register(reg, d); err != nil {
		t.Fatal(err)
	}
	if q, ok := reg.Queue(appsjobs.KindWebhookDeliver); !ok || q != jobs.QueueDelivery {
		t.Fatalf("queue %q %v", q, ok)
	}
	if err := jobs.ValidateArgsType(reflect.TypeFor[appsjobs.WebhookDeliver]()); err != nil {
		t.Fatal(err)
	}
	sub := ids.New()
	if err := (appsjobs.Handler{Deliverer: d}).Work(context.Background(), jobs.Job[appsjobs.WebhookDeliver]{Args: appsjobs.WebhookDeliver{SubscriptionID: sub}}); err != nil ||
		len(d.got) != 1 || d.got[0] != sub {
		t.Fatalf("Work: %v %v", err, d.got)
	}
	var s appsjobs.Scheduler
	if err := s.Schedule(context.Background(), sub, time.Now()); errs.KindOf(err) != errs.KindInternal {
		t.Fatalf("unbound scheduler: %v", err)
	}
}
