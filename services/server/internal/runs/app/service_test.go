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
	good := app.Config{PageSizeDefault: 3, PageSizeMax: 5, StepDetailMaxBytes: 256, QueuedMaxAge: time.Hour, EventPayloadMaxBytes: 65536,
		CancelGrace: time.Second, PresenceTimeout: time.Second, OrphanTimeout: time.Minute, StallAfter: time.Minute, ReasonMaxChars: 10,
		ComponentDefaultTTL: time.Hour, ComponentMaxPerMessage: 25, ComponentCustomIDMaxChars: 100, ComponentFormMaxFields: 10,
		MaxContentBytes: 4096, ExcerptMaxChars: 8, MaxStepsPersisted: 4, InvocationHoldMax: time.Minute, TranscriptHoldMax: time.Minute}
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
		cfg.CancelGrace, cfg.PresenceTimeout, cfg.OrphanTimeout, cfg.ReasonMaxChars = good.CancelGrace, good.PresenceTimeout, good.OrphanTimeout, good.ReasonMaxChars
		cfg.ExcerptMaxChars, cfg.MaxStepsPersisted, cfg.InvocationHoldMax = good.ExcerptMaxChars, good.MaxStepsPersisted, good.InvocationHoldMax
		cfg.TranscriptHoldMax = good.TranscriptHoldMax
		if _, err := app.New(cfg, full); err == nil {
			t.Errorf("%s: no error", name)
		}
	}
	// M2-09: the limits of start, cancel and the stall check.
	for name, mut := range map[string]func(*app.Config){
		"negative leeway":    func(c *app.Config) { c.ClockLeeway = -time.Second },
		"cancel grace zero":  func(c *app.Config) { c.CancelGrace = 0 },
		"presence zero":      func(c *app.Config) { c.PresenceTimeout = 0 },
		"orphan zero":        func(c *app.Config) { c.OrphanTimeout = 0 },
		"reason length zero": func(c *app.Config) { c.ReasonMaxChars = 0 },
		// M2-10: the limits of patchRun.
		"excerpt length zero": func(c *app.Config) { c.ExcerptMaxChars = 0 },
		"max steps zero":      func(c *app.Config) { c.MaxStepsPersisted = 0 },
		// M3-07: the release time of a held invocation.
		"hold max zero": func(c *app.Config) { c.InvocationHoldMax = 0 },
		// M3-09: the release time of an invocation held for its transcript.
		"transcript hold max zero": func(c *app.Config) { c.TranscriptHoldMax = 0 },
	} {
		cfg := good
		mut(&cfg)
		if _, err := app.New(cfg, full); err == nil {
			t.Errorf("%s: no error", name)
		}
	}
	svc, err := app.New(good, full)
	if err != nil || svc == nil {
		t.Fatalf("New with every collaborator and no clock, log or metrics: %v", err)
	}
}
