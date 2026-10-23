// SPDX-License-Identifier: Apache-2.0

package jobs

import (
	"reflect"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/notify/app"
	"github.com/OWNER/bothy/services/server/internal/notify/domain"
	"github.com/OWNER/bothy/services/server/internal/platform/jobs"
)

// TestNotificationCarriesNoText walks Notification and the five argument types: only IDs, ID
// lists, int64 and bool, plus the kind string of Notification (ADR 0031, T-PU-01); the job
// runtime accepts the five argument types.
func TestNotificationCarriesNoText(t *testing.T) {
	t.Parallel()
	allowed := map[reflect.Type]bool{
		reflect.TypeFor[ids.ID](): true, reflect.TypeFor[[]ids.ID](): true, reflect.TypeFor[int64](): true, reflect.TypeFor[bool](): true,
	}
	n := reflect.TypeFor[app.Notification]()
	for i := range n.NumField() {
		f := n.Field(i)
		if f.Name == "Kind" && f.Type.Kind() == reflect.String {
			continue
		}
		if !allowed[f.Type] {
			t.Errorf("Notification.%s has type %s", f.Name, f.Type)
		}
	}
	for _, a := range []any{PushSend{}, CoalesceWindow{}, CoalesceFlush{}, BadgeSync{}, BadgeSyncHold{}} {
		typ := reflect.TypeOf(a)
		for i := range typ.NumField() {
			if f := typ.Field(i); !allowed[f.Type] {
				t.Errorf("%s.%s has type %s", typ.Name(), f.Name, f.Type)
			}
		}
		if err := jobs.ValidateArgsType(typ); err != nil {
			t.Errorf("%s: %v", typ.Name(), err)
		}
	}
}

// TestPushArgsRoundTrip: a push job survives the mapping onto the stored arguments, with absent
// IDs left zero, and the codes of the kinds are the fixed ones.
func TestPushArgsRoundTrip(t *testing.T) {
	t.Parallel()
	due := time.Date(2026, 10, 21, 12, 0, 0, 123000, time.UTC)
	for _, j := range []app.PushJob{
		{NotificationID: ids.New(), RegistrationID: ids.New(), RecipientID: ids.New(), ThreadID: ids.New(), MessageID: ids.New(),
			Kind: domain.KindMention, Quiet: true, Count: 1, DueAt: due, Attempt: 1},
		{NotificationID: ids.New(), RegistrationID: ids.New(), RecipientID: ids.New(), TargetID: ids.New(), Kind: domain.KindSecurity,
			Count: 1, DueAt: due, Attempt: 3},
		{NotificationID: ids.New(), RegistrationID: ids.New(), RecipientID: ids.New(), Cleared: []ids.ID{ids.New(), ids.New()},
			Kind: domain.KindSync, DueAt: due, Attempt: 1},
	} {
		a, err := pushArgs(j)
		if err != nil {
			t.Fatal(err)
		}
		back, err := pushJob(a)
		if err != nil || !reflect.DeepEqual(back, j) {
			t.Fatalf("round trip %+v -> %+v (%v)", j, back, err)
		}
	}
	if a, _ := pushArgs(app.PushJob{Kind: domain.KindSync}); a.KindCode != 9 {
		t.Fatalf("sync code %d", a.KindCode)
	}
	if _, err := pushArgs(app.PushJob{Kind: "unknown"}); err == nil {
		t.Fatal("an unknown kind was mapped")
	}
}
