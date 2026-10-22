// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/runs/app"
)

// TestNew_RefusesMissingCollaboratorsAndBadLimits: New refuses a missing collaborator and the
// limits Config.check refuses, and fills clock, log and metrics when the caller leaves them out.
func TestNew_RefusesMissingCollaboratorsAndBadLimits(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	full := app.Deps{
		Repo: w.repo, UoW: uow{}, Authorizer: w.authz, Events: events{w.j}, Messages: msgs{w.j, w.msgFacts}, Threads: w.threads, Channels: w.channels,
		Agents: w.agents, Cursors: cursors{},
	}
	good := app.Config{PageSizeDefault: 3, PageSizeMax: 5, StepDetailMaxBytes: 256, QueuedMaxAge: time.Hour, EventPayloadMaxBytes: 65536}
	if _, err := app.New(good, app.Deps{}); err == nil {
		t.Fatal("New without collaborators: no error")
	}
	missing := full
	missing.Cursors = nil
	if _, err := app.New(good, missing); err == nil {
		t.Fatal("New without Cursors: no error")
	}
	for name, cfg := range map[string]app.Config{
		"default below 1":     {PageSizeDefault: 0, PageSizeMax: 5, StepDetailMaxBytes: 256, QueuedMaxAge: time.Hour, EventPayloadMaxBytes: 65536},
		"default above max":   {PageSizeDefault: 6, PageSizeMax: 5, StepDetailMaxBytes: 256, QueuedMaxAge: time.Hour, EventPayloadMaxBytes: 65536},
		"step detail below 1": {PageSizeDefault: 3, PageSizeMax: 5, StepDetailMaxBytes: 0, QueuedMaxAge: time.Hour, EventPayloadMaxBytes: 65536},
		"queued age zero":     {PageSizeDefault: 3, PageSizeMax: 5, StepDetailMaxBytes: 256, EventPayloadMaxBytes: 65536},
		"payload below 1":     {PageSizeDefault: 3, PageSizeMax: 5, StepDetailMaxBytes: 256, QueuedMaxAge: time.Hour},
	} {
		if _, err := app.New(cfg, full); err == nil {
			t.Errorf("%s: no error", name)
		}
	}
	svc, err := app.New(good, full)
	if err != nil || svc == nil {
		t.Fatalf("New with every collaborator and no clock, log or metrics: %v", err)
	}
}
