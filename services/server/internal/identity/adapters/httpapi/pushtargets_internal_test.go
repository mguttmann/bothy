// SPDX-License-Identifier: Apache-2.0

package httpapi

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/api/rest"
	"github.com/OWNER/bothy/services/server/internal/identity"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// fakeTargets returns fixed registrations and counts its calls.
type fakeTargets struct {
	regs  map[ids.ID][]identity.PushTargetView
	calls int
	asked []ids.ID
}

func (f *fakeTargets) ForDevices(_ context.Context, deviceIDs []ids.ID) (map[ids.ID][]identity.PushTargetView, error) {
	f.calls++
	f.asked = append(f.asked, deviceIDs...)
	return f.regs, nil
}

// TestDevicePush_Fill (M3-22 design note 11): one read for all devices of an answer; a device with
// registrations lists them with disabled_at and disabled_reason only for a disabled one; a device
// without has no member; nothing is read before BindPushTargets.
func TestDevicePush_Fill(t *testing.T) {
	t.Parallel()
	phone, tablet := ids.New(), ids.New()
	at := time.Date(2026, 10, 14, 9, 0, 0, 0, time.UTC)
	active := identity.PushTargetView{ID: ids.New(), DeviceID: phone, Provider: "apns", Environment: "production", AppID: "com.example.chat", CreatedAt: at}
	disabled := identity.PushTargetView{ID: ids.New(), DeviceID: phone, Provider: "apns", Environment: "production", AppID: "com.example.chat",
		CreatedAt: at, DisabledAt: at.Add(time.Hour), DisabledReason: "logout"}
	src := &fakeTargets{regs: map[ids.ID][]identity.PushTargetView{phone: {active, disabled}}}
	var d devicePush
	devs := []rest.Device{{Id: rest.Uuid(phone)}, {Id: rest.Uuid(tablet)}}
	if err := d.fill(context.Background(), devs); err != nil || devs[0].PushRegistrations != nil || src.calls != 0 {
		t.Fatalf("unbound: %v, %v, %d calls", err, devs[0].PushRegistrations, src.calls)
	}
	d.BindPushTargets(src)
	if err := d.fill(context.Background(), devs); err != nil {
		t.Fatal(err)
	}
	if src.calls != 1 || len(src.asked) != 2 {
		t.Fatalf("%d calls for %d devices, want one call for both", src.calls, len(src.asked))
	}
	if devs[1].PushRegistrations != nil {
		t.Fatal("a device without registrations has the member")
	}
	raw, err := json.Marshal(devs[0])
	if err != nil {
		t.Fatal(err)
	}
	var got struct {
		PushRegistrations []map[string]any `json:"push_registrations"`
	}
	if err := json.Unmarshal(raw, &got); err != nil || len(got.PushRegistrations) != 2 {
		t.Fatalf("member %s: %v", raw, err)
	}
	if _, ok := got.PushRegistrations[0]["disabled_at"]; ok {
		t.Fatal("an active registration carries disabled_at")
	}
	if got.PushRegistrations[1]["disabled_reason"] != "logout" || got.PushRegistrations[1]["disabled_at"] == nil {
		t.Fatalf("disabled registration: %v", got.PushRegistrations[1])
	}
	if strings.Contains(string(raw), "token") {
		t.Fatal("the member carries a token")
	}
}
