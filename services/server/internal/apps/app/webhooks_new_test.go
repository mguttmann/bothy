// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/apps/app"
	"github.com/OWNER/bothy/services/server/internal/instance"
	"github.com/OWNER/bothy/services/server/internal/kernel/auditing"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
	"github.com/OWNER/bothy/services/server/internal/sync"
)

// Non-nil collaborators for the construction checks of NewWebhooks; no method is called.
type (
	stubStore      struct{ app.WebhookStore }
	stubRepo       struct{ app.Repository }
	stubUoW        struct{ txn.UnitOfWork }
	stubAuthorizer struct{ authorize.Authorizer }
	stubAudit      struct{ auditing.Recorder }
	stubScheduler  struct{ app.DeliveryScheduler }
	stubSender     struct{ app.WebhookSender }
	stubMaster     struct{ app.HMACMasterKey }
	stubKeys       struct{ keys app.Signers }
	stubState      struct{ app.InstanceState }
	stubLog        struct{ sync.LogReader }
	stubCursors    struct{ app.Cursors }
)

func webhookDeps() app.WebhookDeps {
	return app.WebhookDeps{
		Store: stubStore{}, Repo: stubRepo{}, UoW: stubUoW{}, Authorizer: stubAuthorizer{}, Audit: stubAudit{},
		Scheduler: stubScheduler{}, Sender: stubSender{}, Master: stubMaster{}, Keys: stubKeys{}, State: stubState{},
		Log: stubLog{}, Cursors: stubCursors{},
	}
}

func webhookConfig() app.WebhookConfig {
	return app.WebhookConfig{
		AllowedHosts: []string{"hooks.example.com"}, MaxConsecutiveFailures: 5, RetryBackoffBase: time.Second,
		RetryBackoffCap: time.Minute, RetryMaxAge: time.Hour, MaxPendingPerSubscription: 10,
		SecretRotationOverlap: time.Hour, DeliveryTimeout: 10 * time.Second, PageSizeDefault: 50, PageSizeMax: 200,
	}
}

// TestNewWebhooksChecksItsInputs: every collaborator is required, the limits and durations have
// their ranges, an allowed host must be host[:port], and the optional clock, random source and
// logger get defaults.
func TestNewWebhooksChecksItsInputs(t *testing.T) {
	t.Parallel()
	if _, err := app.NewWebhooks(webhookConfig(), webhookDeps()); err != nil {
		t.Fatalf("valid inputs: %v", err)
	}
	missing := webhookDeps()
	missing.Cursors = nil
	if _, err := app.NewWebhooks(webhookConfig(), missing); err == nil || !strings.Contains(err.Error(), "collaborator") {
		t.Fatalf("missing collaborator: %v", err)
	}
	for name, change := range map[string]func(*app.WebhookConfig){
		"failures":   func(c *app.WebhookConfig) { c.MaxConsecutiveFailures = 0 },
		"pending":    func(c *app.WebhookConfig) { c.MaxPendingPerSubscription = 0 },
		"page max":   func(c *app.WebhookConfig) { c.PageSizeMax = c.PageSizeDefault - 1 },
		"base":       func(c *app.WebhookConfig) { c.RetryBackoffBase = 0 },
		"cap":        func(c *app.WebhookConfig) { c.RetryBackoffCap = c.RetryBackoffBase / 2 },
		"max age":    func(c *app.WebhookConfig) { c.RetryMaxAge = 0 },
		"overlap":    func(c *app.WebhookConfig) { c.SecretRotationOverlap = 0 },
		"timeout":    func(c *app.WebhookConfig) { c.DeliveryTimeout = 0 },
		"host entry": func(c *app.WebhookConfig) { c.AllowedHosts = []string{"https://hooks.example.com/x"} },
	} {
		cfg := webhookConfig()
		change(&cfg)
		if _, err := app.NewWebhooks(cfg, webhookDeps()); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}

// Signers names its only method Signers, so the stub forwards instead of embedding.
func (k stubKeys) Signers(ctx context.Context, p instance.Purpose) ([]instance.Signer, error) {
	return k.keys.Signers(ctx, p)
}

func (k stubKeys) Signer(ctx context.Context, p instance.Purpose) (instance.Signer, error) {
	return k.keys.Signer(ctx, p)
}
