// SPDX-License-Identifier: Apache-2.0

package postgres

import (
	"testing"
	"time"
)

// TestNewTail_ConfigFallbacks: zero fields of a hand-built configuration get the defaults of 02,
// and a reconnect cap below the poll interval is raised to the poll interval, never to the
// default cap.
func TestNewTail_ConfigFallbacks(t *testing.T) {
	t.Parallel()
	got := NewTail(nil, nil, TailConfig{}, nil, nil).cfg
	if got.PollInterval != time.Second || got.BatchSize != 1000 || got.ReconnectBackoffCap != 10*time.Second {
		t.Errorf("zero config = %+v", got)
	}
	got = NewTail(nil, nil, TailConfig{PollInterval: time.Second, BatchSize: 5, ReconnectBackoffCap: 500 * time.Millisecond}, nil, nil).cfg
	if got.ReconnectBackoffCap != time.Second || got.BatchSize != 5 {
		t.Errorf("cap below the poll interval = %+v, want the cap raised to the poll interval", got)
	}
}
