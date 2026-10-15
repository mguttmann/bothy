// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/identity"
	"github.com/OWNER/bothy/services/server/internal/identity/app"
	"github.com/OWNER/bothy/services/server/internal/kernel/auditing"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
)

// Placeholders that are non-nil collaborators; the constructors only check their presence.
type (
	stubChallenges    struct{ app.ChallengeStore }
	stubSessions      struct{ app.SessionStore }
	stubDevices       struct{ app.DeviceStore }
	stubAuth          struct{ identity.Auth }
	stubUoW           struct{ txn.UnitOfWork }
	stubAuthorizer    struct{ authorize.Authorizer }
	stubAudit         struct{ auditing.Recorder }
	stubDeviceExpirer struct{ app.DeviceExpirer }
	stubCleanup       struct{ app.CleanupStore }
	stubExpiry        struct{ app.Expiry }
)

// stepUpConfig returns the step-up and cleanup values the constructors accept.
func stepUpConfig() app.Config {
	return app.Config{StepUpChallengeTTL: time.Minute, StepUpMaxAge: 5 * time.Minute, LoginFailWindow: 15 * time.Minute,
		StepUpFailLimit: 5, CleanupInterval: time.Hour}
}

// TestNewStepUps_Refuses: a missing collaborator, an empty pepper and every value outside its
// range are errors; a nil clock is the system clock.
func TestNewStepUps_Refuses(t *testing.T) {
	t.Parallel()
	deps := app.StepUpDeps{Challenges: stubChallenges{}, Sessions: stubSessions{}, Devices: stubDevices{}, Auth: stubAuth{},
		UoW: stubUoW{}, Authorizer: stubAuthorizer{}, Audit: stubAudit{}}
	pepper := []byte("EXAMPLE-pepper")
	if _, err := app.NewStepUps(stepUpConfig(), pepper, deps); err != nil {
		t.Fatalf("NewStepUps with valid values: %v", err)
	}
	noAudit := deps
	noAudit.Audit = nil
	if _, err := app.NewStepUps(stepUpConfig(), pepper, noAudit); err == nil {
		t.Error("NewStepUps without the audit recorder passed")
	}
	if _, err := app.NewStepUps(stepUpConfig(), nil, deps); err == nil {
		t.Error("NewStepUps without a pepper passed")
	}
	for name, mutate := range map[string]func(*app.Config){
		"challenge ttl": func(c *app.Config) { c.StepUpChallengeTTL = 0 },
		"max age":       func(c *app.Config) { c.StepUpMaxAge = 0 },
		"fail window":   func(c *app.Config) { c.LoginFailWindow = 0 },
		"fail limit":    func(c *app.Config) { c.StepUpFailLimit = 0 },
		"leeway":        func(c *app.Config) { c.ClockLeeway = -time.Second },
	} {
		cfg := stepUpConfig()
		mutate(&cfg)
		if _, err := app.NewStepUps(cfg, pepper, deps); err == nil {
			t.Errorf("%s: NewStepUps passed", name)
		}
	}
}

// TestNewAuthCleanup_Refuses: a missing collaborator and a duration that is not positive are
// errors; a nil clock is the system clock.
func TestNewAuthCleanup_Refuses(t *testing.T) {
	t.Parallel()
	if _, err := app.NewAuthCleanup(stepUpConfig(), stubUoW{}, stubCleanup{}, stubExpiry{}, stubDeviceExpirer{}, nil); err != nil {
		t.Fatalf("NewAuthCleanup with valid values: %v", err)
	}
	if _, err := app.NewAuthCleanup(stepUpConfig(), nil, stubCleanup{}, stubExpiry{}, stubDeviceExpirer{}, nil); err == nil {
		t.Error("NewAuthCleanup without a unit of work passed")
	}
	cfg := stepUpConfig()
	cfg.CleanupInterval = 0
	if _, err := app.NewAuthCleanup(cfg, stubUoW{}, stubCleanup{}, stubExpiry{}, stubDeviceExpirer{}, nil); err == nil {
		t.Error("NewAuthCleanup with auth.cleanup_interval 0 passed")
	}
}
