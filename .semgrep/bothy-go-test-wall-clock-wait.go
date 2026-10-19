// SPDX-License-Identifier: Apache-2.0

// Test cases for the rule bothy-go-test-wall-clock-wait (run by `make semgrep`).
package fixtures

import (
	"context"
	"net"
	"net/http"
	"time"

	"github.com/OWNER/bothy/services/server/internal/platform/testkit"
)

func ownSeconds(ctx context.Context, c net.Conn, done chan struct{}, start time.Time) {
	// ruleid: bothy-go-test-wall-clock-wait
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	// ruleid: bothy-go-test-wall-clock-wait
	readyDeadline := time.Now().Add(time.Minute)
	_ = readyDeadline
	// ruleid: bothy-go-test-wall-clock-wait
	_ = c.SetDeadline(time.Now().Add(5 * time.Second))
	// ruleid: bothy-go-test-wall-clock-wait
	_ = c.SetReadDeadline(time.Now().Add(5 * time.Second))
	select {
	case <-done:
	// ruleid: bothy-go-test-wall-clock-wait
	case <-time.After(5 * time.Second):
	}
	// ruleid: bothy-go-test-wall-clock-wait
	_ = time.NewTimer(30 * time.Second)
	select {
	case <-done:
	// ruleid: bothy-go-test-wall-clock-wait
	case <-time.After(time.Minute):
	}
	// ruleid: bothy-go-test-wall-clock-wait
	short, stop := context.WithTimeout(ctx, time.Second)
	defer stop()
	_ = short
	// ruleid: bothy-go-test-wall-clock-wait
	wait, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	_ = wait
	// ruleid: bothy-go-test-wall-clock-wait
	_ = &http.Client{Timeout: 10 * time.Second}
	// ruleid: bothy-go-test-wall-clock-wait
	if time.Since(start) > 5*time.Second {
		return
	}
	// ruleid: bothy-go-test-wall-clock-wait
	if time.Now().Sub(start) > 20*time.Second {
		return
	}
}

func patient(ctx context.Context, c net.Conn, done chan struct{}, start time.Time) {
	// ok: bothy-go-test-wall-clock-wait
	deadline := testkit.Deadline()
	for time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	// ok: bothy-go-test-wall-clock-wait
	_ = c.SetDeadline(testkit.Deadline())
	select {
	case <-done:
	// ok: bothy-go-test-wall-clock-wait
	case <-time.After(testkit.Patience):
	}
	// A wait for something that must not happen: a slow host only makes it pass.
	select {
	case <-done:
	// ok: bothy-go-test-wall-clock-wait
	case <-time.After(200 * time.Millisecond):
	}
	// ok: bothy-go-test-wall-clock-wait
	wait, cancel := context.WithTimeout(ctx, testkit.Patience)
	defer cancel()
	_ = wait
	// ok: bothy-go-test-wall-clock-wait
	_ = &http.Client{Timeout: testkit.Patience}
	// ok: bothy-go-test-wall-clock-wait
	if time.Since(start) > testkit.Patience {
		return
	}
	// A lower bound: a slow host cannot fail it.
	// ok: bothy-go-test-wall-clock-wait
	if time.Since(start) < 2*time.Second {
		return
	}
	// Test data, not a wait.
	// ok: bothy-go-test-wall-clock-wait
	notBefore := time.Now().Add(-30 * time.Minute)
	_ = notBefore
}
