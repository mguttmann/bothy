// SPDX-License-Identifier: Apache-2.0

package fakepush_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/notify/adapters/fakepush"
	"github.com/OWNER/bothy/services/server/internal/notify/app"
	"github.com/OWNER/bothy/services/server/internal/notify/domain"
	"github.com/OWNER/bothy/services/server/internal/platform/testkit"
)

// Compile-time check: the fake is a provider of app.
var _ app.PushProvider = (*fakepush.FakeProvider)(nil)

// registration is a registration of Alex's phone with the token secret.
func registration(token string) app.Registration {
	return app.Registration{ID: ids.New(), UserID: ids.New(), DeviceID: ids.New(), Provider: "apns", Environment: "production",
		AppID: "com.example.app", Token: token}
}

func TestFakeProvider_ResultsAndRecord(t *testing.T) {
	ctx := context.Background()
	clk := clock.NewFake(time.Date(2026, 10, 21, 8, 0, 0, 0, time.UTC))
	p := fakepush.New("apns", clk)
	if p.Name() != "apns" {
		t.Fatalf("Name %q, want apns", p.Name())
	}
	alex, sam, robin := registration("token-alex"), registration("token-sam"), registration("token-robin")
	boom := errors.New("network down")
	p.Program(alex.ID,
		fakepush.Outcome{Result: domain.SendResult{Class: domain.ResultRetry, RetryAfter: time.Minute}},
		fakepush.Outcome{Result: domain.SendResult{Class: domain.ResultAccepted}})
	p.Program(sam.ID, fakepush.Outcome{Err: boom})
	p.SetDefault(fakepush.Outcome{Result: domain.SendResult{Class: domain.ResultDisable, DisableReason: domain.DisableTokenInvalid}})

	n := app.Notification{ID: ids.New(), ThreadID: ids.New(), Kind: "message", Count: 1, Badge: 3, HasBadge: true}
	check := func(reg app.Registration, want domain.SendResult, wantErr error) {
		t.Helper()
		got, err := p.Send(ctx, reg, n)
		if got != want || !errors.Is(err, wantErr) {
			t.Errorf("Send(%s) = %+v, %v; want %+v, %v", reg.ID, got, err, want, wantErr)
		}
	}
	check(alex, domain.SendResult{Class: domain.ResultRetry, RetryAfter: time.Minute}, nil)
	clk.Advance(time.Minute)
	check(alex, domain.SendResult{Class: domain.ResultAccepted}, nil)
	// The last programmed outcome repeats.
	check(alex, domain.SendResult{Class: domain.ResultAccepted}, nil)
	check(sam, domain.SendResult{}, boom)
	check(robin, domain.SendResult{Class: domain.ResultDisable, DisableReason: domain.DisableTokenInvalid}, nil)

	calls := p.Calls()
	if len(calls) != 5 {
		t.Fatalf("%d calls, want 5", len(calls))
	}
	first := calls[0]
	if first.Registration.ID != alex.ID || first.Registration.Token != "token-alex" || first.Notification.ID != n.ID ||
		!first.At.Equal(time.Date(2026, 10, 21, 8, 0, 0, 0, time.UTC)) || !calls[1].At.Equal(first.At.Add(time.Minute)) {
		t.Errorf("first calls %+v, %+v", first, calls[1])
	}
	if got := p.CallsFor(alex.ID); len(got) != 3 {
		t.Errorf("%d calls for Alex, want 3", len(got))
	}
	if got := p.CallsFor(ids.New()); len(got) != 0 {
		t.Errorf("%d calls for an unknown registration", len(got))
	}
	// The record is a copy: changing it changes nothing in the fake.
	calls[0].Registration.Token = "changed"
	if p.Calls()[0].Registration.Token != "token-alex" {
		t.Error("Calls returned the fake's own slice")
	}
	for _, format := range []string{"%v", "%+v", "%#v", "%s"} {
		if text := fmt.Sprintf(format, first); strings.Contains(text, "token-alex") {
			t.Errorf("%s of a call shows the token: %s", format, text)
		}
		if text := fmt.Sprintf(format, calls); strings.Contains(text, "token-") {
			t.Errorf("%s of the calls shows a token: %s", format, text)
		}
	}
	if text := first.String(); !strings.Contains(text, alex.ID.String()) || !strings.Contains(text, "apns") {
		t.Errorf("String %q names neither the registration nor the provider", text)
	}
}

func TestFakeProvider_DefaultAccepts(t *testing.T) {
	p := fakepush.New("fcm", nil)
	got, err := p.Send(context.Background(), registration("token-sam"), app.Notification{Kind: app.KindSync})
	if err != nil || got.Class != domain.ResultAccepted {
		t.Errorf("Send = %+v, %v; want accepted", got, err)
	}
	if c := p.Calls(); len(c) != 1 || c[0].At.IsZero() {
		t.Errorf("calls %v, want one with a time", c)
	}
}

// TestFakeProvider_GateBlocksUntilReleased: while the gate is closed every Send waits, the fake
// reports how many wait and the most that ran at once; a release lets them all finish, and a
// cancelled context ends a waiting Send with the context's error.
func TestFakeProvider_GateBlocksUntilReleased(t *testing.T) {
	ctx := context.Background()
	p := fakepush.New("apns", nil)
	release := p.Gate()
	const parallel = 8
	var wg sync.WaitGroup
	errsCh := make(chan error, parallel)
	for range parallel {
		wg.Go(func() {
			_, err := p.Send(ctx, registration("token"), app.Notification{})
			errsCh <- err
		})
	}
	wait, cancel := context.WithTimeout(ctx, testkit.Patience)
	defer cancel()
	if err := p.WaitInFlight(wait, parallel); err != nil {
		t.Fatalf("WaitInFlight(%d): %v with %d in flight", parallel, err, p.InFlight())
	}
	release()
	release() // a second release is harmless
	wg.Wait()
	close(errsCh)
	for err := range errsCh {
		if err != nil {
			t.Errorf("released Send: %v", err)
		}
	}
	if p.InFlight() != 0 || p.MaxInFlight() != parallel {
		t.Errorf("in flight %d max %d, want 0 and %d", p.InFlight(), p.MaxInFlight(), parallel)
	}

	p.Gate()
	cctx, stop := context.WithCancel(ctx)
	done := make(chan error, 1)
	go func() {
		_, err := p.Send(cctx, registration("token"), app.Notification{})
		done <- err
	}()
	if err := p.WaitInFlight(wait, 1); err != nil {
		t.Fatal(err)
	}
	stop()
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Errorf("cancelled Send: %v, want context.Canceled", err)
	}
	// After the gate opens again, Send no longer waits.
	p.Open()
	if _, err := p.Send(ctx, registration("token"), app.Notification{}); err != nil {
		t.Errorf("Send after Open: %v", err)
	}
}

func TestFakeProvider_WaitInFlightHonoursContext(t *testing.T) {
	p := fakepush.New("apns", nil)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := p.WaitInFlight(ctx, 1); !errors.Is(err, context.Canceled) {
		t.Errorf("WaitInFlight on a cancelled context: %v", err)
	}
}
